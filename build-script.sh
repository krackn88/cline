#!/bin/bash

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print banner
echo -e "${BLUE}=======================================${NC}"
echo -e "${BLUE}       AI Web Integration Agent       ${NC}"
echo -e "${BLUE}=======================================${NC}"
echo ""

# Check for Go
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed. Please install Go 1.18 or newer.${NC}"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | grep -o "go[0-9]\.[0-9]*" | cut -c 3-)
echo -e "${GREEN}✓ Go version ${GO_VERSION} installed${NC}"

# Create directories
echo -e "${BLUE}Creating directories...${NC}"
mkdir -p screenshots

# Install dependencies
echo -e "${BLUE}Installing dependencies...${NC}"
go mod tidy

# Build the application
echo -e "${BLUE}Building application...${NC}"
go build -o ai-agent main.go

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Build successful${NC}"
    echo -e "${GREEN}✓ Binary created: ai-agent${NC}"
else
    echo -e "${RED}✗ Build failed${NC}"
    exit 1
fi

# Make it executable
chmod +x ai-agent

# Check if config.json exists
if [ ! -f "config.json" ]; then
    echo -e "${YELLOW}Warning: config.json not found, creating default config${NC}"
    cat > config.json << EOF
{
  "claude_url": "https://claude.ai/chat",
  "github_copilot_url": "https://github.com/features/copilot",
  "browser_user_data_dir": "~/.browser-agent",
  "screenshot_dir": "./screenshots",
  "log_file": "./agent.log",
  "headless": false,
  "debug_mode": true,
  "claude_login_required": true,
  "github_login_required": true
}
EOF
    echo -e "${GREEN}✓ Default config.json created${NC}"
fi

echo ""
echo -e "${GREEN}=======================================${NC}"
echo -e "${GREEN}      Build completed successfully     ${NC}"
echo -e "${GREEN}=======================================${NC}"
echo ""
echo -e "Run the agent with: ${BLUE}./ai-agent${NC}"
echo ""
echo -e "${YELLOW}Note: On first run, you will need to log in to Claude and GitHub Copilot in the browser window that opens.${NC}"
echo ""
