#!/bin/bash

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Tool paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK_DIR="$SCRIPT_DIR/tools"
COOKIE_EXTRACTOR="$WORK_DIR/firefox-login-cookie-extractor.py"
GO_AGENT="$WORK_DIR/web-integration-agent"

# Track what we've installed
INSTALLED_PYTHON=false
INSTALLED_GO=false
INSTALLED_DEPENDENCIES=false
EXTRACTED_COOKIES=false
BUILT_AGENT=false

# Config
CONFIG_FILE="$SCRIPT_DIR/config.json"
COOKIES_DIR="$SCRIPT_DIR/cookies"
SCREENSHOTS_DIR="$SCRIPT_DIR/screenshots"
LOG_DIR="$SCRIPT_DIR/logs"

# Print banner
echo -e "${BLUE}============================================================${NC}"
echo -e "${BLUE}    Auto-Setup for Browser Integration Agent with Cookies    ${NC}"
echo -e "${BLUE}============================================================${NC}"

# Check for required utilities
check_requirements() {
    echo -e "\n${BLUE}Checking system requirements...${NC}"
    
    # Check for git
    if ! command -v git &> /dev/null; then
        echo -e "${RED}Error: git is not installed${NC}"
        echo -e "Please install git and try again"
        exit 1
    else
        echo -e "${GREEN}✓ git is installed${NC}"
    fi
    
    # Check for Python
    if ! command -v python3 &> /dev/null; then
        echo -e "${YELLOW}Python 3 is not installed. Installing...${NC}"
        if [[ "$OSTYPE" == "linux-gnu"* ]]; then
            sudo apt-get update && sudo apt-get install -y python3 python3-pip
        elif [[ "$OSTYPE" == "darwin"* ]]; then
            brew install python3
        else
            echo -e "${RED}Error: Automatic Python installation not supported on this OS${NC}"
            echo -e "Please install Python 3 manually and try again"
            exit 1
        fi
        INSTALLED_PYTHON=true
        echo -e "${GREEN}✓ Python 3 installed${NC}"
    else
        echo -e "${GREEN}✓ Python 3 is installed${NC}"
    fi
    
    # Check for Go
    if ! command -v go &> /dev/null; then
        echo -e "${YELLOW}Go is not installed. Installing...${NC}"
        if [[ "$OSTYPE" == "linux-gnu"* ]]; then
            wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
            sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
            echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
            source ~/.profile
            rm go1.21.0.linux-amd64.tar.gz
        elif [[ "$OSTYPE" == "darwin"* ]]; then
            brew install go
        else
            echo -e "${RED}Error: Automatic Go installation not supported on this OS${NC}"
            echo -e "Please install Go manually and try again"
            exit 1
        fi
        INSTALLED_GO=true
        echo -e "${GREEN}✓ Go installed${NC}"
    else
        echo -e "${GREEN}✓ Go is installed${NC}"
    fi
}

# Create necessary directories
create_directories() {
    echo -e "\n${BLUE}Creating necessary directories...${NC}"
    
    mkdir -p "$WORK_DIR"
    mkdir -p "$COOKIES_DIR"
    mkdir -p "$SCREENSHOTS_DIR" 
    mkdir -p "$LOG_DIR"
    
    echo -e "${GREEN}✓ Directories created${NC}"
}

# Install necessary Python packages
install_dependencies() {
    echo -e "\n${BLUE}Installing dependencies...${NC}"
    
    # Python dependencies
    pip3 install requests sqlite3 pathlib

    # Go dependencies
    go get -u github.com/chromedp/chromedp
    go get -u github.com/chromedp/cdproto
    
    INSTALLED_DEPENDENCIES=true
    echo -e "${GREEN}✓ Dependencies installed${NC}"
}

# Create the Firefox cookie extractor script
create_cookie_extractor() {
    echo -e "\n${BLUE}Creating Firefox cookie extractor...${NC}"
    
    cat > "$COOKIE_EXTRACTOR" << 'EOF'
#!/usr/bin/env python3
"""
Firefox Login Cookie Extractor
Extract only login/authentication cookies from Firefox for automated sessions.
"""

import os
import sys
import json
import sqlite3
import shutil
import tempfile
import argparse
from pathlib import Path
from datetime import datetime

# Common authentication cookie names
AUTH_COOKIE_KEYWORDS = [
    'auth', 'login', 'token', 'session', 'sid', 'user', 'account', 
    'jwt', 'bearer', 'access', 'refresh', 'id', 'identity', 'oauth',
    'remember', 'credential', 'logged', 'authenticated'
]

# Well-known sites and their auth cookie patterns
KNOWN_AUTH_COOKIES = {
    'github.com': ['user_session', 'dotcom_user', 'logged_in', 'tz'],
    'claude.ai': ['__Secure-next-auth.session-token', 'sessionKey'],
    'anthropic.com': ['__Secure-next-auth.session-token', 'sessionKey']
}

def get_firefox_profile_dirs():
    """Find Firefox profile directories on the current system."""
    profiles = []
    
    if sys.platform.startswith('win'):
        base_path = os.path.join(os.environ.get('APPDATA', ''), 'Mozilla', 'Firefox', 'Profiles')
    elif sys.platform.startswith('darwin'):
        base_path = os.path.expanduser('~/Library/Application Support/Firefox/Profiles')
    else:  # Linux and others
        base_path = os.path.expanduser('~/.mozilla/firefox')
    
    if not os.path.exists(base_path):
        return profiles
    
    # Handle direct profiles directory
    if os.path.isdir(base_path):
        for item in os.listdir(base_path):
            profile_path = os.path.join(base_path, item)
            if os.path.isdir(profile_path) and (item.endswith('.default') or '.default-' in item or 'default-release' in item):
                profiles.append((item, profile_path))
    
    # Handle profiles.ini approach
    profiles_ini = os.path.join(os.path.dirname(base_path), 'profiles.ini')
    if os.path.exists(profiles_ini):
        with open(profiles_ini, 'r') as f:
            current_profile = None
            current_path = None
            
            for line in f:
                line = line.strip()
                if line.startswith('[Profile'):
                    if current_profile and current_path:
                        profiles.append((current_profile, current_path))
                    current_profile = None
                    current_path = None
                elif '=' in line:
                    key, value = line.split('=', 1)
                    if key == 'Name':
                        current_profile = value
                    elif key == 'Path':
                        if os.path.isabs(value):
                            current_path = value
                        else:
                            current_path = os.path.join(os.path.dirname(profiles_ini), value)
            
            if current_profile and current_path:
                profiles.append((current_profile, current_path))
    
    return profiles

def copy_cookie_db(profile_path):
    """Create a temporary copy of the cookies database to avoid lock issues."""
    cookies_db = os.path.join(profile_path, 'cookies.sqlite')
    
    if not os.path.exists(cookies_db):
        return None
    
    temp_dir = tempfile.mkdtemp()
    temp_db = os.path.join(temp_dir, 'cookies.sqlite')
    
    try:
        shutil.copy2(cookies_db, temp_db)
        return temp_db
    except Exception as e:
        print(f"Error copying cookies database: {e}")
        return None

def is_login_cookie(domain, name, secure, http_only):
    """Determine if a cookie is likely related to authentication/login."""
    # Check if it's a known auth cookie for a specific site
    domain_stripped = domain.lstrip('.')
    for site, cookies in KNOWN_AUTH_COOKIES.items():
        if site in domain_stripped and name in cookies:
            return True
    
    # Check if the cookie name contains auth-related keywords
    name_lower = name.lower()
    for keyword in AUTH_COOKIE_KEYWORDS:
        if keyword in name_lower:
            return True
    
    # Security attributes common for auth cookies
    if secure and http_only:
        # Cookies with both secure and httpOnly flags are often auth-related
        return True
    
    # Check for auth subdomains
    if domain.startswith(('.auth.', '.login.', '.account.', '.id.')):
        return True
    
    return False

def extract_login_cookies(db_path, domain_filter=None):
    """Extract likely login cookies from the Firefox SQLite database."""
    cookies = []
    
    try:
        conn = sqlite3.connect(db_path)
        cursor = conn.cursor()
        
        query = '''
            SELECT host, name, value, path, expiry, isSecure, isHttpOnly, sameSite
            FROM moz_cookies
        '''
        
        if domain_filter:
            query += " WHERE host LIKE ?"
            cursor.execute(query, (f"%{domain_filter}%",))
        else:
            cursor.execute(query)
        
        for row in cursor.fetchall():
            host, name, value, path, expiry, is_secure, is_http_only, same_site = row
            
            # Check if this is a login-related cookie
            if is_login_cookie(host, name, bool(is_secure), bool(is_http_only)):
                # Convert expiry to datetime
                if expiry:
                    # Firefox stores expiry as seconds since epoch
                    expiry_date = datetime.fromtimestamp(expiry)
                else:
                    expiry_date = None
                    
                cookie = {
                    'domain': host,
                    'name': name,
                    'value': value,
                    'path': path,
                    'expiry': expiry_date.isoformat() if expiry_date else None,
                    'secure': bool(is_secure),
                    'httpOnly': bool(is_http_only),
                    'sameSite': same_site
                }
                
                cookies.append(cookie)
        
        conn.close()
        return cookies
    
    except Exception as e:
        print(f"Error extracting cookies: {e}")
        return []

def save_as_json(cookies, output_path):
    """Save cookies in JSON format."""
    with open(output_path, 'w') as f:
        json.dump(cookies, f, indent=2)
    
    print(f"Saved {len(cookies)} login cookies to {output_path}")

def extract_cookies_for_domain(domain, output_file, profile_index=None):
    """Extract cookies for a specific domain and save to a file."""
    profiles = get_firefox_profile_dirs()
    
    if not profiles:
        print("No Firefox profiles found.")
        return False
    
    # Select profile
    selected_profile = None
    
    if profile_index is not None and 0 <= profile_index < len(profiles):
        selected_profile = profiles[profile_index][1]
    elif len(profiles) == 1:
        selected_profile = profiles[0][1]
    else:
        print("Available Firefox profiles:")
        for i, (name, path) in enumerate(profiles):
            print(f"{i+1}. {name} ({path})")
        
        choice = input("Select profile (number): ")
        try:
            index = int(choice) - 1
            if 0 <= index < len(profiles):
                selected_profile = profiles[index][1]
            else:
                print("Invalid selection.")
                return False
        except ValueError:
            print("Invalid input.")
            return False
    
    # Create a temporary copy of the cookies database
    temp_db = copy_cookie_db(selected_profile)
    
    if not temp_db:
        print("Could not access cookies database.")
        return False
    
    try:
        # Extract login cookies
        cookies = extract_login_cookies(temp_db, domain)
        
        if not cookies:
            print(f"No login cookies found for domain '{domain}'.")
            return False
        
        # Save cookies
        save_as_json(cookies, output_file)
        return True
        
    finally:
        # Clean up the temporary directory
        temp_dir = os.path.dirname(temp_db)
        if os.path.exists(temp_dir):
            shutil.rmtree(temp_dir)

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Extract login cookies from Firefox browser')
    parser.add_argument('-d', '--domain', required=True, help='Domain to extract cookies for')
    parser.add_argument('-o', '--output', required=True, help='Output file path')
    parser.add_argument('-p', '--profile', type=int, help='Profile index (0-based)')
    
    args = parser.parse_args()
    
    success = extract_cookies_for_domain(args.domain, args.output, args.profile)
    sys.exit(0 if success else 1)
EOF
    
    chmod +x "$COOKIE_EXTRACTOR"
    echo -e "${GREEN}✓ Firefox cookie extractor created${NC}"
}

# Extract cookies from Firefox
extract_cookies() {
    echo -e "\n${BLUE}Extracting cookies from Firefox browser...${NC}"
    
    echo -e "${YELLOW}Extracting Claude cookies...${NC}"
    python3 "$COOKIE_EXTRACTOR" --domain claude.ai --output "$COOKIES_DIR/claude_cookies.json"
    
    echo -e "${YELLOW}Extracting GitHub cookies...${NC}"
    python3 "$COOKIE_EXTRACTOR" --domain github.com --output "$COOKIES_DIR/github_cookies.json"
    
    EXTRACTED_COOKIES=true
    echo -e "${GREEN}✓ Cookies extracted and saved to:${NC}"
    echo -e "  - $COOKIES_DIR/claude_cookies.json"
    echo -e "  - $COOKIES_DIR/github_cookies.json"
}

# Create web integration agent
create_web_agent() {
    echo -e "\n${BLUE}Creating web integration agent...${NC}"
    
    # Create agent directory
    mkdir -p "$GO_AGENT"
    
    # Create go.mod file for the agent
    cd "$GO_AGENT"
    go mod init github.com/yourusername/web-integration-agent
    
    # Create main.go file for the agent
    cat > "$GO_AGENT/main.go" << 'EOF'
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// Configuration for the agent
type Config struct {
	ClaudeURL            string   `json:"claude_url"`
	GithubCopilotURL     string   `json:"github_copilot_url"`
	BrowserUserDataDir   string   `json:"browser_user_data_dir"`
	ScreenshotDir        string   `json:"screenshot_dir"`
	LogFile              string   `json:"log_file"`
	CookiesFiles         []string `json:"cookies_files"`
	Headless             bool     `json:"headless"`
	DebugMode            bool     `json:"debug_mode"`
}

// Cookie represents a browser cookie
type Cookie struct {
	Domain   string `json:"domain"`
	Name     string `json:"name"`
	Value    string `json:"value"`
	Path     string `json:"path"`
	Expiry   string `json:"expiry,omitempty"`
	Secure   bool   `json:"secure"`
	HttpOnly bool   `json:"httpOnly"`
	SameSite int    `json:"sameSite"`
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

// Load cookies from file
func (s *Session) LoadCookies() error {
	for _, cookiesFile := range s.config.CookiesFiles {
		s.logger.Printf("Loading cookies from %s", cookiesFile)
		
		cookiesData, err := os.ReadFile(cookiesFile)
		if err != nil {
			s.logger.Printf("Error reading cookies file: %v", err)
			continue
		}
		
		var cookies []Cookie
		if err := json.Unmarshal(cookiesData, &cookies); err != nil {
			s.logger.Printf("Error parsing cookies JSON: %v", err)
			continue
		}
		
		// Set cookies in browser
		for _, cookie := range cookies {
			// Convert expiry to timestamp if present
			var expires *cdp.TimeSinceEpoch
			if cookie.Expiry != "" {
				// Parse ISO datetime
				t, err := time.Parse(time.RFC3339, cookie.Expiry)
				if err == nil {
					e := cdp.TimeSinceEpoch(t.Unix())
					expires = &e
				}
			}
			
			err := chromedp.Run(s.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
				// Create the CDP SetCookie action
				expr := network.SetCookie(cookie.Name, cookie.Value).
					WithDomain(cookie.Domain).
					WithPath(cookie.Path).
					WithSecure(cookie.Secure).
					WithHTTPOnly(cookie.HttpOnly)
					
				if expires != nil {
					expr = expr.WithExpires(*expires)
				}
				
				return expr.Do(ctx)
			}))
			
			if err != nil {
				s.logger.Printf("Error setting cookie %s: %v", cookie.Name, err)
			}
		}
		
		s.logger.Printf("Loaded %d cookies from %s", len(cookies), cookiesFile)
	}
	
	return nil
}

// Navigate to Claude
func (s *Session) NavigateToClaude() error {
	s.logger.Println("Navigating to Claude")
	if err := chromedp.Run(s.ctx, chromedp.Navigate(s.config.ClaudeURL)); err != nil {
		return fmt.Errorf("failed to navigate to Claude: %v", err)
	}
	
	// Wait for Claude to load
	if err := chromedp.Run(s.ctx, 
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("failed waiting for Claude page: %v", err)
	}
	
	// Take screenshot to see login state
	if err := s.TakeScreenshot("claude_page.png"); err != nil {
		s.logger.Printf("Warning: Failed to take screenshot: %v", err)
	}
	
	return nil
}

// Navigate to GitHub Copilot
func (s *Session) NavigateToGitHubCopilot() error {
	s.logger.Println("Navigating to GitHub Copilot")
	if err := chromedp.Run(s.ctx, chromedp.Navigate(s.config.GithubCopilotURL)); err != nil {
		return fmt.Errorf("failed to navigate to GitHub Copilot: %v", err)
	}
	
	// Wait for GitHub page to load
	if err := chromedp.Run(s.ctx, 
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("failed waiting for GitHub page: %v", err)
	}
	
	// Take screenshot
	if err := s.TakeScreenshot("github_page.png"); err != nil {
		s.logger.Printf("Warning: Failed to take screenshot: %v", err)
	}
	
	return nil
}

func main() {
	// Check for config file
	if len(os.Args) < 2 {
		fmt.Println("Usage: web-integration-agent config.json")
		os.Exit(1)
	}
	
	configFile := os.Args[1]
	
	// Load configuration
	configData, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}
	
	var config Config
	if err := json.Unmarshal(configData, &config); err != nil {
		log.Fatalf("Failed to parse config file: %v", err)
	}
	
	// Create the session
	session, err := NewSession(config)
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}
	defer session.Close()
	
	// Load cookies
	if err := session.LoadCookies(); err != nil {
		log.Fatalf("Failed to load cookies: %v", err)
	}
	
	// Test navigation
	fmt.Println("Testing navigation to Claude...")
	if err := session.NavigateToClaude(); err != nil {
		log.Fatalf("Failed to navigate to Claude: %v", err)
	}
	fmt.Printf("Successfully navigated to Claude (screenshot saved to %s/claude_page.png)\n", config.ScreenshotDir)
	
	fmt.Println("Testing navigation to GitHub Copilot...")
	if err := session.NavigateToGitHubCopilot(); err != nil {
		log.Fatalf("Failed to navigate to GitHub Copilot: %v", err)
	}
	fmt.Printf("Successfully navigated to GitHub Copilot (screenshot saved to %s/github_page.png)\n", config.ScreenshotDir)
	
	fmt.Println("Browser test completed successfully!")
	fmt.Println("The browser is configured with your login cookies.")
	fmt.Println("You can now use the agent for automated tasks.")
}
EOF
    
    echo -e "${GREEN}✓ Web integration agent created${NC}"
}

# Build the web agent
build_web_agent() {
    echo -e "\n${BLUE}Building web integration agent...${NC}"
    
    cd "$GO_AGENT"
    go mod tidy
    go build -o web-integration-agent main.go
    
    BUILT_AGENT=true
    echo -e "${GREEN}✓ Web integration agent built successfully${NC}"
}

# Create configuration file
create_config() {
    echo -e "\n${BLUE}Creating configuration file...${NC}"
    
    cat > "$CONFIG_FILE" << EOF
{
    "claude_url": "https://claude.ai/chat",
    "github_copilot_url": "https://github.com/features/copilot",
    "browser_user_data_dir": "$WORK_DIR/browser_data",
    "screenshot_dir": "$SCREENSHOTS_DIR",
    "log_file": "$LOG_DIR/agent.log",
    "cookies_files": [
        "$COOKIES_DIR/claude_cookies.json",
        "$COOKIES_DIR/github_cookies.json"
    ],
    "headless": false,
    "debug_mode": true
}
EOF
    
    echo -e "${GREEN}✓ Configuration file created at $CONFIG_FILE${NC}"
}

# Create the run script
create_run_script() {
    echo -e "\n${BLUE}Creating run script...${NC}"
    
    cat > "$SCRIPT_DIR/run-agent.sh" << EOF
#!/bin/bash

# Run the web integration agent with cookies
"$GO_AGENT/web-integration-agent" "$CONFIG_FILE"
EOF
    
    chmod +x "$SCRIPT_DIR/run-agent.sh"
    echo -e "${GREEN}✓ Run script created at $SCRIPT_DIR/run-agent.sh${NC}"
}

# Main function
main() {
    check_requirements
    create_directories
    install_dependencies
    create_cookie_extractor
    extract_cookies
    create_web_agent
    build_web_agent
    create_config
    create_run_script
    
    # Print summary
    echo -e "\n${GREEN}============================================================${NC}"
    echo -e "${GREEN}                 Setup completed successfully                  ${NC}"
    echo -e "${GREEN}============================================================${NC}"
    echo -e "\n${BLUE}Summary:${NC}"
    echo -e "  - ${GREEN}Cookie extractor created${NC}"
    echo -e "  - ${GREEN}Cookies extracted from Firefox${NC}"
    echo -e "  - ${GREEN}Web integration agent built${NC}"
    echo -e "  - ${GREEN}Configuration created${NC}"
    
    echo -e "\n${BLUE}To run the agent:${NC}"
    echo -e "  ${YELLOW}./run-agent.sh${NC}"
    
    # Warn about missing cookies
    if [ ! -f "$COOKIES_DIR/claude_cookies.json" ] || [ ! -f "$COOKIES_DIR/github_cookies.json" ]; then
        echo -e "\n${YELLOW}WARNING: Some cookie files are missing. The agent may not work correctly.${NC}"
        echo -e "${YELLOW}Make sure you're logged in to Claude and GitHub in Firefox.${NC}"
    fi
    
    echo -e "\n${GREEN}Good luck with your computer use agent!${NC}"
}

# Run the script
main
