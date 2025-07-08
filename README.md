# ask-human-cli

A stateless CLI tool to post questions to Slack and wait for human replies. Designed for AI agents and developers who need human-in-the-loop interactions.

## Installation

```bash
go install github.com/hogelog/ask-human-cli@latest
```

Or build from source:

```bash
git clone https://github.com/hogelog/ask-human-cli.git
cd ask-human-cli
go build
```

## Configuration

### 1. Create a Slack App

1. Go to [api.slack.com/apps](https://api.slack.com/apps)
2. Create a new app
3. **Enable Socket Mode (Required)**:
   - Go to "Socket Mode" in the sidebar
   - Enable Socket Mode
   - Generate an App-Level Token with `connections:write` scope
4. Subscribe to events:
   - Go to "Event Subscriptions"
   - Enable Events
   - Subscribe to bot events:
     - `message.channels` - for public channels
     - `message.groups` - for private channels
   - Save changes
5. Add OAuth scopes under "OAuth & Permissions":
   - `chat:write` - Post messages
   - `channels:read` - Resolve channel names
   - `users:read` - Resolve user mentions
   - `channels:history` - Read message history
   - `groups:history` - Read private channel history (if using private channels)
6. Install the app to your workspace

### 2. Configure the CLI

Run the setup command to create your configuration:

```bash
ask-human-cli setup
```

This will prompt you for:
- Your Slack Bot Token (xoxb-...)
- Your Slack App Token (xapp-...)
- Default channel (e.g., #ask-human)
- Default timeout in seconds (default: 300)

The configuration will be saved to `~/.config/ask-human-cli/config.json`.

Alternatively, you can manually create the config file:

```json
{
  "slack_token": "xoxb-your-token-here",
  "app_token": "xapp-your-app-token-here",
  "default_channel": "#ask-human",
  "default_timeout": 300
}
```

Or use environment variables:

```bash
export SLACK_BOT_TOKEN="xoxb-your-token-here"
export SLACK_APP_TOKEN="xapp-your-app-token-here"
```

## Usage

The `ask` command posts a question to Slack and waits for a human reply. It will monitor the thread for responses until either a reply is received or the timeout is reached, then post a status message to the thread.

### Post a new question

```bash
ask-human-cli ask \
  --question "What were your main tasks in 2014-2016?" \
  --title "Career history question" \
  --channel "#ask-human"
```

### Post with mention

```bash
ask-human-cli ask \
  --question "What were your main tasks in 2014-2016?" \
  --title "Career history question" \
  --mention "@john_doe"
```

### Reply to an existing thread

```bash
ask-human-cli ask \
  --question "Any specific projects that stood out?" \
  --thread-ts "1720172735.001100"
```

### Specify custom timeout

```bash
ask-human-cli ask \
  --question "What is your preferred approach?" \
  --title "Technical question" \
  --timeout 600  # Wait up to 10 minutes
```

## Command Options

| Option | Required | Description |
|--------|----------|-------------|
| `--question` | ✅ | The question content |
| `--title` | Conditional | Session title. Required for new posts (when `--thread-ts` is not specified) |
| `--thread-ts` | Optional | Thread timestamp for replies |
| `--mention` | Optional | User to notify - @username or @displayname (e.g., @hogelog) |
| `--channel` | Conditional | Target channel (can be omitted if configured default_channel in config.json) |
| `--timeout` | Optional | Timeout in seconds (uses config default if not specified) |


## Use Cases

- AI agents requiring human input
- Automated workflows with human approval steps
- Development processes needing human verification
- Research tools integrating human feedback

## License

MIT
