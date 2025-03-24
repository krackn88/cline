# Go for Python Developers: Learning Golang from Real Projects

This guide will help you learn Go (Golang) from a Python developer's perspective, using real examples from the Synergy Workbench and Agent Evolution Platform projects.

## Table of Contents

1. [Go Basics](#go-basics)
2. [Project Structure](#project-structure)
3. [Types and Structs](#types-and-structs)
4. [Error Handling](#error-handling)
5. [Interfaces](#interfaces)
6. [Concurrency](#concurrency)
7. [Building and Running](#building-and-running)
8. [Integration with Other Languages](#integration-with-other-languages)
9. [Real Project Examples](#real-project-examples)
10. [From Python to Go: Key Differences](#from-python-to-go-key-differences)

## Go Basics

Go is a statically typed, compiled language with garbage collection. Here's how it differs from Python:

| Feature | Python | Go |
|---------|--------|-----|
| Typing | Dynamic | Static |
| Execution | Interpreted | Compiled |
| Concurrency | GIL-limited | Built-in goroutines |
| Error Handling | Exceptions | Return values |
| OOP | Classes | Structs and interfaces |

### Hello World Comparison

**Python:**
```python
def main():
    print("Hello, World!")

if __name__ == "__main__":
    main()
```

**Go:**
```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
```

## Project Structure

Go projects typically follow a standard structure:

```
project/
├── cmd/                  # Command applications
├── pkg/                  # Library code
├── internal/             # Private code
├── go.mod                # Module definition
├── go.sum                # Dependencies checksum
└── main.go               # Entry point (if not in cmd/)
```

From your Synergy Workbench:

```
synergy-workbench/
├── bin/                  # Compiled binaries
├── src/                  # Source code
│   ├── ai-integrations/  # Integration with AI services  
│   └── core/             # Core functionality
├── go.mod                # Module definition
└── config.json           # Configuration
```

### Module Definition

The `go.mod` file defines the module and its dependencies:

```go
module github.com/krackn/synergy-workbench

go 1.21
```

## Types and Structs

Go uses structs instead of classes. Here's an example from `server.go`:

```go
// Server represents a Synergy Workbench server
type Server struct {
    wb        *workbench.Workbench
    providers *providers.AIProviderSet
    cfg       *config.Config
}

// CompletionRequest represents an API request
type CompletionRequest struct {
    Model       string   `json:"model"`
    Prompt      string   `json:"prompt"`
    Provider    string   `json:"provider,omitempty"`
    Freeload    bool     `json:"freeload,omitempty"`
    MaxTokens   int      `json:"max_tokens,omitempty"`
    Temperature float64  `json:"temperature,omitempty"`
    Stop        []string `json:"stop,omitempty"`
}
```

### JSON Tags

The \`json:"field_name,omitempty"\` syntax defines how struct fields are marshaled to/from JSON. The `omitempty` option omits empty fields.

### Equivalent in Python

```python
@dataclass
class CompletionRequest:
    model: str
    prompt: str
    provider: Optional[str] = None
    freeload: bool = False
    max_tokens: Optional[int] = None
    temperature: Optional[float] = None
    stop: Optional[List[str]] = None
```

## Error Handling

Go uses explicit error returns instead of exceptions:

```go
// Python
def load_config(path):
    try:
        with open(path) as f:
            return json.load(f)
    except Exception as e:
        raise ConfigError(f"Failed to load config: {e}")

# Go
func loadConfig(configPath string) (Config, error) {
    defaultConfig := Config{
        Host:  "localhost",
        Port:  8082,
        // ...other defaults
    }
    
    // If no config path specified, return default config
    if configPath == "" {
        return defaultConfig, nil
    }
    
    // Read config file
    configData, err := os.ReadFile(configPath)
    if err != nil {
        return defaultConfig, err
    }
    
    // Parse config file
    var config Config
    err = json.Unmarshal(configData, &config)
    if err != nil {
        return defaultConfig, err
    }
    
    return config, nil
}
```

### Error Checking Pattern

Go uses a common pattern for error checking:

```go
result, err := someFunction()
if err != nil {
    // Handle error
    return nil, fmt.Errorf("error in someFunction: %v", err)
}
// Use result...
```

## Interfaces

Go uses interfaces for polymorphism. Here's an example from the AIProvider interface:

```go
// AIProvider defines the interface for AI service providers
type AIProvider interface {
    ExecuteTask(payload interface{}) (interface{}, error)
    GetCostEstimate(taskType string) (float64, error)
}
```

Example implementation:

```go
// AnthropicProvider implements the AIProvider interface
type AnthropicProvider struct {
    apiKey      string
    client      *http.Client
    maxTokens   int
    temperature float64
}

// ExecuteTask implements the AIProvider interface for Anthropic
func (p *AnthropicProvider) ExecuteTask(payload interface{}) (interface{}, error) {
    // Implementation here...
    return response, nil
}

// GetCostEstimate implements the AIProvider interface
func (p *AnthropicProvider) GetCostEstimate(taskType string) (float64, error) {
    // Implementation here...
    return 0.01, nil
}
```

### Python Equivalent

```python
class AIProvider(ABC):
    @abstractmethod
    def execute_task(self, payload):
        pass
        
    @abstractmethod
    def get_cost_estimate(self, task_type):
        pass

class AnthropicProvider(AIProvider):
    def __init__(self, api_key, max_tokens=2048, temperature=0.7):
        self.api_key = api_key
        self.client = requests.Session()
        self.max_tokens = max_tokens
        self.temperature = temperature
        
    def execute_task(self, payload):
        # Implementation
        return response
        
    def get_cost_estimate(self, task_type):
        return 0.01
```

## Concurrency

Go's concurrency model is based on goroutines and channels, which are more lightweight than Python's threads.

### Goroutines

```go
// Start task processors
for i := 0; i < wb.config.ComputeResources.MaxConcurrentTasks; i++ {
    wb.wg.Add(1)
    go wb.taskProcessor(i)  // Launches a goroutine
}
```

### Channels

```go
// Create a task with freeload mode enabled
resultChan := make(chan interface{})
errChan := make(chan error)

// Wait for result or error with timeout
select {
case result := <-resultChan:
    // Process result
case err := <-errChan:
    // Handle error
case <-time.After(2 * time.Minute):
    // Handle timeout
}
```

### Python Equivalent

Using async/await:

```python
async def process_task(task):
    try:
        result = await execute_task(task)
        return result
    except Exception as e:
        return None, e

# Create tasks
tasks = [process_task(task) for task in all_tasks]

# Wait with timeout
done, pending = await asyncio.wait(
    tasks, 
    timeout=120,  # 2 minutes
    return_when=asyncio.FIRST_COMPLETED
)
```

## Building and Running

Go programs are compiled into a single binary:

```bash
# Build
go build -o bin/synergy-workbench src/core/main.go

# Run
./bin/synergy-workbench -port 8080 -config ./config.json
```

### Makefile Example

```makefile
BINARY_NAME=synergy-workbench
GO_VERSION=1.21

build:
    go mod tidy
    go build -o bin/$(BINARY_NAME) src/core/main.go

test:
    go test ./... -v

run: build
    ./bin/$(BINARY_NAME)
```

## Integration with Other Languages

Go can integrate with other languages like Python:

```go
// Go code to execute a Python script
func executePythonScript(scriptPath string, args []string) (string, error) {
    cmd := exec.Command("python3", append([]string{scriptPath}, args...)...)
    var out bytes.Buffer
    cmd.Stdout = &out
    err := cmd.Run()
    if err != nil {
        return "", err
    }
    return out.String(), nil
}
```

From the Agent Evolution Platform, we see an example of Python-Go integration:

```bash
create_integration_code() {
    log_info "Creating Python-Go integration code..."
    
    # Create Python integration module
    create_dir "$AGENT_SRC_DIR/plugins/go_integration" || exit 1
    
    # Create Go integration plugin
    cat > "$AGENT_SRC_DIR/plugins/go_integration/plugin.py" << 'EOF'
"""
Go Integration Plugin for Agent Evolution Framework

This plugin enables the agent to communicate with Go components.
"""
import requests

# Knowledge Base Service endpoint
KB_SERVICE_HOST = os.environ.get("KB_SERVICE_HOST", "localhost")
KB_SERVICE_PORT = int(os.environ.get("KB_SERVICE_PORT", "8081"))
KB_SERVICE_URL = f"http://{KB_SERVICE_HOST}:{KB_SERVICE_PORT}"

def search_kb_service(query: str, limit: int = 5) -> Dict[str, Any]:
    """
    Search the Knowledge Base Service.
    """
    try:
        payload = {
            "query": query,
            "limit": limit
        }
        
        response = requests.post(
            f"{KB_SERVICE_URL}/search",
            json=payload,
            timeout=5
        )
        
        return response.json()
    except Exception as e:
        return {"error": str(e)}
EOF
```

## Real Project Examples

### HTTP Server in Go

From `server.go`:

```go
func main() {
    // Parse command-line arguments
    port := flag.Int("port", 8080, "HTTP server port")
    configPath := flag.String("config", "", "Path to config file")
    flag.Parse()
    
    // Create HTTP server
    mux := http.NewServeMux()
    mux.HandleFunc("/", server.handleRoot)
    mux.HandleFunc("/v1/completions", server.handleCompletions)
    mux.HandleFunc("/v1/models", server.handleListModels)
    mux.HandleFunc("/health", server.handleHealth)

    // Start HTTP server
    addr := fmt.Sprintf(":%d", *port)
    srv := &http.Server{
        Addr:    addr,
        Handler: mux,
    }
    
    log.Printf("Starting server on http://localhost%s", addr)
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("Failed to start HTTP server: %v", err)
    }
}
```

### Handler Function

```go
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
    
    // Process request...
    
    // Return response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}
```

### Python Equivalent

Using Flask:

```python
from flask import Flask, request, jsonify

app = Flask(__name__)

@app.route('/v1/completions', methods=['POST'])
def handle_completions():
    if request.method != 'POST':
        return jsonify({"error": "Method not allowed"}), 405
        
    try:
        req = request.get_json()
    except:
        return jsonify({"error": "Invalid request"}), 400
        
    # Process request...
    
    # Return response
    return jsonify(resp)

if __name__ == '__main__':
    app.run(host='localhost', port=8080)
```

## From Python to Go: Key Differences

### 1. Variable Declaration

**Python:**
```python
name = "Claude"
age = 30
```

**Go:**
```go
var name string = "Claude"
age := 30  // Type inferred
```

### 2. Function Declaration

**Python:**
```python
def add(a, b):
    return a + b
```

**Go:**
```go
func add(a int, b int) int {
    return a + b
}
```

### 3. Error Handling

**Python:**
```python
try:
    result = some_function()
except Exception as e:
    print(f"Error: {e}")
```

**Go:**
```go
result, err := someFunction()
if err != nil {
    fmt.Printf("Error: %v", err)
}
```

### 4. Loops

**Python:**
```python
for i in range(10):
    print(i)
    
for item in items:
    print(item)
```

**Go:**
```go
for i := 0; i < 10; i++ {
    fmt.Println(i)
}

for _, item := range items {
    fmt.Println(item)
}
```

### 5. Data Structures

**Python:**
```python
# List
items = [1, 2, 3]
items.append(4)

# Dict
config = {
    "host": "localhost",
    "port": 8080
}
```

**Go:**
```go
// Slice
items := []int{1, 2, 3}
items = append(items, 4)

// Map
config := map[string]interface{}{
    "host": "localhost",
    "port": 8080,
}
```

### 6. Imports

**Python:**
```python
import os
import json
from typing import Dict, Any
```

**Go:**
```go
import (
    "os"
    "encoding/json"
    "fmt"
)
```

### 7. Dependency Management

**Python:**
```
# requirements.txt
requests==2.28.1
flask==2.2.2
```

**Go:**
```
# go.mod
module github.com/username/project

go 1.21

require (
    github.com/gorilla/mux v1.8.0
)
```

## Conclusion

Go offers several advantages for Python developers:

1. **Performance**: Compiled, statically typed language with excellent performance
2. **Concurrency**: Built-in goroutines and channels for easy concurrent programming
3. **Simplicity**: Small language with a clean syntax
4. **Deployment**: Single binary deployments with no dependencies

The key to learning Go as a Python developer is understanding the differences in paradigms (static vs. dynamic typing, explicit error handling, structs vs. classes) while leveraging your existing programming knowledge.

Use the real-world examples from the Synergy Workbench and Agent Evolution Platform to see how Go is used in production applications.
