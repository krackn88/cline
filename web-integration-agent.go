package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime/enable"
	"github.com/chromedp/chromedp"
)

// Configuration for the agent
type Config struct {
	ClaudeURL           string `json:"claude_url"`
	GithubCopilotURL    string `json:"github_copilot_url"`
	BrowserUserDataDir  string `json:"browser_user_data_dir"`
	ScreenshotDir       string `json:"screenshot_dir"`
	LogFile             string `json:"log_file"`
	Headless            bool   `json:"headless"`
	DebugMode           bool   `json:"debug_mode"`
	ClaudeLoginRequired bool   `json:"claude_login_required"`
	GithubLoginRequired bool   `json:"github_login_required"`
}

// Session represents a browser session
type Session struct {
	ctx    context.Context
	cancel context.CancelFunc
	config Config
	logger *log.Logger
}

// Initialize a new session
func NewSession(config Config) (*Session, error) {
	// Setup logging
	logFile, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	logger := log.New(logFile, "AGENT: ", log.LstdFlags|log.Lshortfile)
	logger.Println("Initializing new session")

	// Create screenshots directory if it doesn't exist
	if err := os.MkdirAll(config.ScreenshotDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create screenshots directory: %v", err)
	}

	// Initialize Chrome options
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.DisableGPU,
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-background-networking", false),
		chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
	}

	// Add user data directory if specified
	if config.BrowserUserDataDir != "" {
		// Expand ~ to home directory if present
		if strings.HasPrefix(config.BrowserUserDataDir, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get user home directory: %v", err)
			}
			config.BrowserUserDataDir = filepath.Join(home, config.BrowserUserDataDir[1:])
		}
		
		opts = append(opts, chromedp.UserDataDir(config.BrowserUserDataDir))
	}

	// Set headless mode based on config
	if config.Headless {
		opts = append(opts, chromedp.Headless)
	}

	// Create context with options
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(logger.Printf))

	if config.DebugMode {
		// Enable debug protocol
		chromedp.Run(ctx, enable.Enable())
	}

	return &Session{
		ctx:    ctx,
		cancel: cancel,
		config: config,
		logger: logger,
	}, nil
}

// Close the session
func (s *Session) Close() {
	s.logger.Println("Closing session")
	s.cancel()
}

// Take a screenshot
func (s *Session) TakeScreenshot(filename string) error {
	var buf []byte
	if err := chromedp.Run(s.ctx, chromedp.FullScreenshot(&buf, 90)); err != nil {
		return err
	}

	path := filepath.Join(s.config.ScreenshotDir, filename)
	return os.WriteFile(path, buf, 0644)
}

// Log in to Claude if needed
func (s *Session) LoginToClaude() error {
	if !s.config.ClaudeLoginRequired {
		s.logger.Println("Claude login not required, skipping")
		return nil
	}

	s.logger.Println("Opening Claude login page")
	if err := chromedp.Run(s.ctx, chromedp.Navigate(s.config.ClaudeURL)); err != nil {
		return fmt.Errorf("failed to navigate to Claude: %v", err)
	}

	// Wait for login page to load completely
	if err := chromedp.Run(s.ctx, 
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("failed waiting for Claude page: %v", err)
	}

	// Take screenshot to see login state
	if err := s.TakeScreenshot("claude_login.png"); err != nil {
		s.logger.Printf("Warning: Failed to take screenshot: %v", err)
	}

	// Check if login is needed by looking for a login button or form
	var loginNeeded bool
	err := chromedp.Run(s.ctx, chromedp.Evaluate(`
		document.querySelector('button[type="submit"]') !== null || 
		document.querySelector('input[type="password"]') !== null
	`, &loginNeeded))
	
	if err != nil {
		return fmt.Errorf("failed to check login state: %v", err)
	}

	if loginNeeded {
		s.logger.Println("Claude login appears to be required")
		// Wait for user to login manually since we can't automate Anthropic login
		// due to security measures
		fmt.Println("Please log in to Claude in the browser window")
		fmt.Println("Press Enter when done...")
		fmt.Scanln()
	} else {
		s.logger.Println("Already logged into Claude")
	}

	return nil
}

// Log in to GitHub if needed
func (s *Session) LoginToGitHub() error {
	if !s.config.GithubLoginRequired {
		s.logger.Println("GitHub login not required, skipping")
		return nil
	}

	s.logger.Println("Opening GitHub login page")
	if err := chromedp.Run(s.ctx, chromedp.Navigate("https://github.com/login")); err != nil {
		return fmt.Errorf("failed to navigate to GitHub login: %v", err)
	}

	// Wait for login page to load completely
	if err := chromedp.Run(s.ctx, 
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("failed waiting for GitHub login page: %v", err)
	}

	// Take screenshot
	if err := s.TakeScreenshot("github_login.png"); err != nil {
		s.logger.Printf("Warning: Failed to take screenshot: %v", err)
	}

	// Check if we're already logged in by looking for avatar
	var loggedIn bool
	err := chromedp.Run(s.ctx, chromedp.Evaluate(`
		document.querySelector('.avatar') !== null || 
		document.querySelector('.Header-item.position-relative.mr-0 .avatar') !== null
	`, &loggedIn))
	
	if err != nil {
		return fmt.Errorf("failed to check GitHub login state: %v", err)
	}

	if !loggedIn {
		s.logger.Println("GitHub login appears to be required")
		fmt.Println("Please log in to GitHub in the browser window")
		fmt.Println("Press Enter when done...")
		fmt.Scanln()
	} else {
		s.logger.Println("Already logged into GitHub")
	}

	return nil
}

// Navigate to Claude and send a prompt
func (s *Session) AskClaude(prompt string) (string, error) {
	s.logger.Println("Navigating to Claude")
	if err := chromedp.Run(s.ctx, chromedp.Navigate(s.config.ClaudeURL)); err != nil {
		return "", fmt.Errorf("failed to navigate to Claude: %v", err)
	}

	// Wait for Claude to load
	if err := chromedp.Run(s.ctx, 
		chromedp.WaitVisible(`textarea`, chromedp.ByQuery),
	); err != nil {
		return "", fmt.Errorf("failed waiting for Claude input: %v", err)
	}

	s.logger.Println("Sending prompt to Claude")
	// Clear existing text and type new prompt
	if err := chromedp.Run(s.ctx,
		chromedp.Click(`textarea`, chromedp.ByQuery),
		chromedp.KeyEvent(input.Esc), // Ensure clean state
		chromedp.KeyEvent("Control+a"), // Select all
		chromedp.KeyEvent("Delete"), // Delete selected
		chromedp.SendKeys(`textarea`, prompt, chromedp.ByQuery),
	); err != nil {
		return "", fmt.Errorf("failed to input prompt: %v", err)
	}

	// Send the prompt (press Enter)
	if err := chromedp.Run(s.ctx,
		chromedp.KeyEvent(input.Enter),
	); err != nil {
		return "", fmt.Errorf("failed to send prompt: %v", err)
	}

	// Wait for response to appear
	// Claude's response usually appears in a div with role="article"
	time.Sleep(2 * time.Second) // Brief pause to let Claude start generating
	if err := chromedp.Run(s.ctx, 
		chromedp.WaitVisible(`div[role="article"]`, chromedp.ByQuery),
	); err != nil {
		s.logger.Printf("Warning: Couldn't detect Claude's response element: %v", err)
	}

	// Wait for Claude to finish typing (stop animation)
	// We'll wait up to 60 seconds for the response
	timeout := 60 * time.Second
	start := time.Now()
	
	for {
		if time.Since(start) > timeout {
			s.logger.Println("Timeout waiting for Claude to finish responding")
			break
		}
		
		// Check if Claude is still generating by looking for typing indicators
		var isGenerating bool
		err := chromedp.Run(s.ctx, chromedp.Evaluate(`
			document.querySelector('.typing-indicator') !== null || 
			document.querySelector('.animate-pulse') !== null
		`, &isGenerating))
		
		if err != nil {
			s.logger.Printf("Warning: Failed to check if Claude is still generating: %v", err)
			break
		}
		
		if !isGenerating {
			// If Claude is no longer generating, wait a bit more and confirm
			time.Sleep(2 * time.Second)
			
			err := chromedp.Run(s.ctx, chromedp.Evaluate(`
				document.querySelector('.typing-indicator') !== null || 
				document.querySelector('.animate-pulse') !== null
			`, &isGenerating))
			
			if err != nil || !isGenerating {
				break // Claude has finished responding
			}
		}
		
		time.Sleep(1 * time.Second) // Wait before checking again
	}

	// Take screenshot of the response
	if err := s.TakeScreenshot(fmt.Sprintf("claude_response_%d.png", time.Now().Unix())); err != nil {
		s.logger.Printf("Warning: Failed to take screenshot: %v", err)
	}

	// Extract Claude's response text
	var response string
	err := chromedp.Run(s.ctx, chromedp.Evaluate(`
		// Get all message containers
		const messages = document.querySelectorAll('div[role="article"]');
		// Get the latest message (Claude's response)
		const lastMessage = messages[messages.length - 1];
		return lastMessage ? lastMessage.innerText : "Couldn't extract Claude's response";
	`, &response))
	
	if err != nil {
		return "", fmt.Errorf("failed to extract Claude's response: %v", err)
	}

	s.logger.Println("Successfully received response from Claude")
	return response, nil
}

// Navigate to GitHub Copilot and use it
func (s *Session) UseGitHubCopilot(codeContext string) (string, error) {
	s.logger.Println("Navigating to GitHub Copilot")
	if err := chromedp.Run(s.ctx, chromedp.Navigate(s.config.GithubCopilotURL)); err != nil {
		return "", fmt.Errorf("failed to navigate to GitHub Copilot: %v", err)
	}

	// Wait for the code editor to load
	// This selector will need to be updated based on the actual GitHub Copilot Web UI
	if err := chromedp.Run(s.ctx, 
		chromedp.WaitVisible(`.monaco-editor`, chromedp.ByQuery),
	); err != nil {
		return "", fmt.Errorf("failed waiting for code editor: %v", err)
	}

	// Clear existing code and input the context
	if err := chromedp.Run(s.ctx,
		chromedp.Click(`.monaco-editor`, chromedp.ByQuery),
		chromedp.KeyEvent("Control+a"), // Select all
		chromedp.KeyEvent("Delete"), // Delete selected
		chromedp.SendKeys(`.monaco-editor`, codeContext, chromedp.ByQuery),
	); err != nil {
		return "", fmt.Errorf("failed to input code context: %v", err)
	}

	// Trigger Copilot suggestions
	if err := chromedp.Run(s.ctx,
		chromedp.KeyEvent("Control+Enter"), // This may vary based on the actual trigger
	); err != nil {
		return "", fmt.Errorf("failed to trigger Copilot suggestions: %v", err)
	}

	// Wait for suggestions to appear
	time.Sleep(3 * time.Second)

	// Take screenshot
	if err := s.TakeScreenshot(fmt.Sprintf("github_copilot_%d.png", time.Now().Unix())); err != nil {
		s.logger.Printf("Warning: Failed to take screenshot: %v", err)
	}

	// Extract suggested code
	var suggestedCode string
	err := chromedp.Run(s.ctx, chromedp.Evaluate(`
		// This selector needs to be updated based on the actual GitHub Copilot Web UI
		const suggestion = document.querySelector('.copilot-suggestion');
		return suggestion ? suggestion.innerText : "Couldn't extract Copilot's suggestion";
	`, &suggestedCode))
	
	if err != nil {
		return "", fmt.Errorf("failed to extract Copilot suggestion: %v", err)
	}

	s.logger.Println("Successfully received suggestion from GitHub Copilot")
	return suggestedCode, nil
}

// Integrate Claude and GitHub Copilot
func (s *Session) ExecuteTask(task string) (string, error) {
	s.logger.Printf("Executing task: %s", task)

	// First, ask Claude for guidance
	claudePrompt := fmt.Sprintf(
		"I need to %s. Please provide detailed instructions and any code structure I should start with.", 
		task,
	)
	
	claudeResponse, err := s.AskClaude(claudePrompt)
	if err != nil {
		return "", fmt.Errorf("Claude interaction failed: %v", err)
	}

	// Extract code from Claude's response
	codeContext := extractCodeFromText(claudeResponse)
	
	if codeContext == "" {
		// If no code was found, use the entire response as context
		codeContext = claudeResponse
	}

	// Use GitHub Copilot to generate/complete the code
	copilotSuggestion, err := s.UseGitHubCopilot(codeContext)
	if err != nil {
		return "", fmt.Errorf("GitHub Copilot interaction failed: %v", err)
	}

	// Ask Claude to review and refine the Copilot's suggestion
	reviewPrompt := fmt.Sprintf(
		"I'm working on a task: %s\n\nClaude (you) gave me this guidance:\n%s\n\nGitHub Copilot suggested this code:\n%s\n\nPlease review the Copilot suggestion and provide a final version of the code with any necessary improvements or corrections. Explain any significant changes you make.",
		task,
		claudeResponse,
		copilotSuggestion,
	)

	finalResponse, err := s.AskClaude(reviewPrompt)
	if err != nil {
		return "", fmt.Errorf("Claude review failed: %v", err)
	}

	return finalResponse, nil
}

// Extract code blocks from text
func extractCodeFromText(text string) string {
	var codeBlocks []string

	// Simple extraction of code blocks based on markdown code fences
	lines := strings.Split(text, "\n")
	inCodeBlock := false
	currentBlock := ""

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End of a code block
				inCodeBlock = false
				codeBlocks = append(codeBlocks, currentBlock)
				currentBlock = ""
			} else {
				// Start of a code block
				inCodeBlock = true
			}
		} else if inCodeBlock {
			currentBlock += line + "\n"
		}
	}

	// Combine all found code blocks
	return strings.Join(codeBlocks, "\n\n")
}

// Open the default browser to a URL
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // "linux", "freebsd", etc.
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Start()
}

// Load configuration from file
func loadConfig(path string) (Config, error) {
	// Default configuration
	config := Config{
		ClaudeURL:           "https://claude.ai/chat",
		GithubCopilotURL:    "https://github.com/features/copilot",
		BrowserUserDataDir:  "~/.browser-agent",
		ScreenshotDir:       "./screenshots",
		LogFile:             "./agent.log",
		Headless:            false,
		DebugMode:           true,
		ClaudeLoginRequired: true,
		GithubLoginRequired: true,
	}

	// If no config file specified, return defaults
	if path == "" {
		return config, nil
	}

	// Read the configuration file
	data, err := os.ReadFile(path)
	if err != nil {
		return config, fmt.Errorf("failed to read config file: %v", err)
	}

	// Parse the JSON
	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to parse config file: %v", err)
	}

	return config, nil
}

func main() {
	// Load configuration
	config, err := loadConfig("config.json")
	if err != nil {
		log.Printf("Warning: Failed to load config file: %v", err)
		log.Println("Using default configuration")
	}

	// Create the session
	session, err := NewSession(config)
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}
	defer session.Close()

	// Login to services if needed
	if err := session.LoginToClaude(); err != nil {
		log.Fatalf("Claude login failed: %v", err)
	}

	if err := session.LoginToGitHub(); err != nil {
		log.Fatalf("GitHub login failed: %v", err)
	}

	// Main interaction loop
	fmt.Println("==== AI Agent Ready ====")
	fmt.Println("Enter tasks or commands (type 'exit' to quit):")

	for {
		fmt.Print("> ")
		var input string
		fmt.Scanln(&input)

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			break
		}

		// Execute the task
		result, err := session.ExecuteTask(input)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Println("=== Result ===")
		fmt.Println(result)
		fmt.Println("==============")
	}

	fmt.Println("Exiting AI Agent")
}
