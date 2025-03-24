package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Configuration for the service
type Config struct {
	Host           string            `json:"host"`
	Port           int               `json:"port"`
	Providers      map[string]string `json:"providers"`
	MaxConcurrent  int               `json:"max_concurrent"`
	LogFile        string            `json:"log_file"`
	CostThreshold  float64           `json:"cost_threshold"`
	AutoScaling    bool              `json:"auto_scaling"`
	MemorySettings MemoryConfig      `json:"memory_settings"`
}

// Memory configuration
type MemoryConfig struct {
	Strategy          string `json:"strategy"`
	MinPerInstance    string `json:"min_per_instance"`
	PreferredMemory   string `json:"preferred_memory"`
	RetentionMinutes  int    `json:"retention_minutes"`
}

// Server represents our HTTP server
type Server struct {
	config     *Config
	router     *http.ServeMux
	taskQueue  chan Task
	wg         sync.WaitGroup
	cancelFunc context.CancelFunc
}

// Task represents a unit of work
type Task struct {
	ID          string
	Provider    string
	Payload     map[string]interface{}
	ResultChan  chan interface{}
	ErrorChan   chan error
	CreatedAt   time.Time
}

// CompletionRequest for API
type CompletionRequest struct {
	Model       string                 `json:"model"`
	Provider    string                 `json:"provider"`
	Content     string                 `json:"content"`
	Options     map[string]interface{} `json:"options,omitempty"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
}

// CompletionResponse from the API
type CompletionResponse struct {
	ID        string      `json:"id"`
	Provider  string      `json:"provider"`
	Model     string      `json:"model"`
	Content   interface{} `json:"content"`
	CreatedAt int64       `json:"created_at"`
	Usage     struct {
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		TotalTokens      int     `json:"total_tokens"`
		Cost             float64 `json:"cost"`
	} `json:"usage"`
}

// Provider interface for AI providers
type Provider interface {
	ProcessRequest(payload map[string]interface{}) (interface{}, error)
	GetName() string
	GetCost(payload map[string]interface{}) float64
}

// Load configuration from file or environment
func loadConfig(path string) (*Config, error) {
	// Default configuration
	cfg := &Config{
		Host:          "localhost",
		Port:          8080,
		MaxConcurrent: 10,
		CostThreshold: 5.0,
		AutoScaling:   true,
		MemorySettings: MemoryConfig{
			Strategy:         "dynamic",
			MinPerInstance:   "4GB",
			PreferredMemory:  "8GB",
			RetentionMinutes: 60,
		},
		Providers: map[string]string{
			"default": "local",
		},
	}

	// If path provided, load from file
	if path != "" {
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open config file: %v", err)
		}
		defer file.Close()

		decoder := json.NewDecoder(file)
		if err := decoder.Decode(cfg); err != nil {
			return nil, fmt.Errorf("failed to decode config: %v", err)
		}
	}

	// Override with environment variables
	if host := os.Getenv("SERVICE_HOST"); host != "" {
		cfg.Host = host
	}
	
	if portStr := os.Getenv("SERVICE_PORT"); portStr != "" {
		if port, err := fmt.Sscanf(portStr, "%d", &cfg.Port); err != nil {
			log.Printf("Warning: Invalid port in environment: %s", portStr)
		} else {
			cfg.Port = port
		}
	}

	return cfg, nil
}

// Create a new server
func newServer(cfg *Config) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	
	server := &Server{
		config:     cfg,
		router:     http.NewServeMux(),
		taskQueue:  make(chan Task, cfg.MaxConcurrent),
		cancelFunc: cancel,
	}

	// Set up routes
	server.setupRoutes()
	
	return server
}

// Set up HTTP routes
func (s *Server) setupRoutes() {
	s.router.HandleFunc("/", s.handleIndex)
	s.router.HandleFunc("/v1/completions", s.handleCompletions)
	s.router.HandleFunc("/v1/models", s.handleListModels)
	s.router.HandleFunc("/health", s.handleHealth)
}

// Handle index route
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<html><body><h1>AI Service Gateway</h1><p>API documentation available at <a href='/docs'>/docs</a></p></body></html>")
}

// Handle completions API
func (s *Server) handleCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.MaxTokens == 0 {
		req.MaxTokens = 1024
	}
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}

	// Create task
	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
	resultChan := make(chan interface{}, 1)
	errChan := make(chan error, 1)

	// Create payload
	payload := map[string]interface{}{
		"model":       req.Model,
		"content":     req.Content,
		"max_tokens":  req.MaxTokens,
		"temperature": req.Temperature,
	}
	if req.Options != nil {
		for k, v := range req.Options {
			payload[k] = v
		}
	}

	task := Task{
		ID:         taskID,
		Provider:   req.Provider,
		Payload:    payload,
		ResultChan: resultChan,
		ErrorChan:  errChan,
		CreatedAt:  time.Now(),
	}

	// Submit task
	select {
	case s.taskQueue <- task:
		// Task submitted successfully
	default:
		// Queue is full
		http.Error(w, "Server is busy, try again later", http.StatusServiceUnavailable)
		return
	}

	// Wait for result with timeout
	select {
	case result := <-resultChan:
		response := CompletionResponse{
			ID:        taskID,
			Provider:  req.Provider,
			Model:     req.Model,
			Content:   result,
			CreatedAt: time.Now().Unix(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

	case err := <-errChan:
		http.Error(w, fmt.Sprintf("Error processing request: %v", err), http.StatusInternalServerError)

	case <-time.After(60 * time.Second):
		http.Error(w, "Request timed out", http.StatusGatewayTimeout)
	}
}

// Handle models listing
func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Mock response for now
	models := map[string]interface{}{
		"models": []map[string]interface{}{
			{"id": "gpt-4", "provider": "openai"},
			{"id": "claude-3", "provider": "anthropic"},
			{"id": "llama2", "provider": "local"},
			{"id": "gemma", "provider": "local"},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models)
}

// Handle health check
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// Start the server
func (s *Server) start() error {
	// Start worker goroutines
	for i := 0; i < s.config.MaxConcurrent; i++ {
		s.wg.Add(1)
		go s.taskWorker(i)
	}

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	// Run the server in a goroutine
	go func() {
		log.Printf("Starting server on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down server...")
	
	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	// Cancel all workers and wait for them to finish
	s.cancelFunc()
	close(s.taskQueue)
	s.wg.Wait()

	log.Println("Server stopped")
	return nil
}

// Task worker processes tasks from the queue
func (s *Server) taskWorker(id int) {
	defer s.wg.Done()
	log.Printf("Starting worker %d", id)

	for task := range s.taskQueue {
		// Process task (mock implementation)
		time.Sleep(100 * time.Millisecond)
		
		// Generate mock response
		result := map[string]interface{}{
			"text": fmt.Sprintf("This is a mock response from worker %d for task %s", id, task.ID),
		}
		
		select {
		case task.ResultChan <- result:
			// Result sent successfully
		default:
			// No one is waiting for the result anymore
		}
	}

	log.Printf("Worker %d stopped", id)
}

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to configuration file")
	port := flag.Int("port", 0, "HTTP server port (overrides config)")
	flag.Parse()

	// Load configuration
	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override port if specified
	if *port > 0 {
		cfg.Port = *port
	}

	// Create and start server
	server := newServer(cfg)
	if err := server.start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
