# AI Web Integration Agent

An agent that integrates Claude AI web interface and GitHub Copilot web to power your computing tasks without using their APIs directly.

## Features

- Browser automation to interact with Claude and GitHub Copilot web interfaces
- Multi-step reasoning using both AI assistants
- Code completion and generation without API access
- Session persistence with browser user data
- Screenshot capturing for debugging

## Prerequisites

- Go 1.18+
- Chrome/Chromium browser
- Active Claude access
- GitHub account with Copilot access

## Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/ai-agent.git
cd ai-agent

# Build the agent
chmod +x build.sh
./build.sh
```

## Usage

```bash
# Run the agent
./ai-agent
```

On first run, the browser will open and you'll need to:
1. Log in to Claude AI
2. Log in to GitHub with Copilot access

After logging in, the agent will save your session, so you don't need to log in again.

## Example Tasks

Once the agent is running, you can enter tasks like:

```
> Write a simple Flask app to display current weather
> Create a Python script to extract data from a CSV file
> Generate a bash script to backup my home directory
```

The agent will:
1. Ask Claude for guidance
2. Extract code snippets from Claude's response
3. Send them to GitHub Copilot for completion
4. Have Claude review and refine the final solution

## Configuration

Edit `config.json` to configure the agent:

```json
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
```

## Troubleshooting

If you encounter issues:

1. Check `agent.log` for detailed logs
2. Review screenshots in the `screenshots` directory
3. Set `debug_mode` to `true` in `config.json`
4. Ensure your Chrome/Chromium browser is up to date

## License

MIT
