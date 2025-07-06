# ask-human-cli

A stateless CLI tool to post questions to Slack and wait for human replies using Socket Mode. Designed for AI agents and developers who need human-in-the-loop interactions without rate limits.

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
   - Copy the token (starts with `xapp-`)
4. Subscribe to events:
   - Go to "Event Subscriptions"
   - Enable Events
   - Subscribe to bot events:
     - `message.channels` - for public channels
     - `message.groups` - for private channels
     - `message.im` - for direct messages (optional)
   - Save changes
5. Add OAuth scopes under "OAuth & Permissions":
   - `chat:write` - Post messages
   - `channels:read` - Resolve channel names
   - `users:read` - Resolve user mentions
   - `channels:history` - Read message history
   - `groups:history` - Read private channel history (if using private channels)
6. Install the app to your workspace
7. Copy the Bot User OAuth Token (starts with `xoxb-`)

### 2. Configure the CLI

Run the setup command to create your configuration:

```bash
ask-human-cli setup
```

This will prompt you for:
- Your Slack Bot Token (xoxb-...)
- Your Slack App Token (xapp-...)
- Default channel (e.g., #hitl)
- Default timeout in seconds (default: 300)

The configuration will be saved to `~/.config/ask-human-cli/config.json`.

Alternatively, you can manually create the config file:

```json
{
  "slack_token": "xoxb-your-token-here",
  "app_token": "xapp-your-app-token-here",
  "default_channel": "#hitl",
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
  --channel "#hitl"
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
| `--title` | Conditional | Required for new posts (when `--thread-ts` is not specified) |
| `--mention` | Optional | User to notify - @username or @displayname (e.g., @hogelog) |
| `--channel` | Conditional | Target channel (can be omitted if configured) |
| `--thread-ts` | Optional | Thread timestamp for replies |
| `--timeout` | Optional | Timeout in seconds (uses config default if not specified) |

## Priority Order

### Slack Bot Token
1. `SLACK_BOT_TOKEN` environment variable
2. `config.json` → `slack_token`

### Slack App Token
1. `SLACK_APP_TOKEN` environment variable
2. `config.json` → `app_token`

### Channel
1. `--channel` flag
2. `config.json` → `default_channel`

### Timeout
1. `--timeout` flag
2. `config.json` → `default_timeout`
3. Default: 300 seconds (5 minutes)

## Message Format

### New post
Main message:
```
📝 Career history question
```

Thread replies:
```
@john_doe What were your main tasks in 2014-2016?
```

After receiving a response or timeout:
```
✅ Response received from @john_doe
```
or
```
⏱️ Timed out waiting for response.
```

### Thread reply
```
@jane_smith Any specific projects that stood out?
```

Note: The mention will appear as a clickable Slack mention in the actual message.

## Reply Output

When a reply is received, the tool outputs:
```
Reply received:
From: @hogelog
Text: The main tasks were developing the payment system and API integration.
Thread TS: 1720172735.001100
Timestamp: 2024-07-05 14:32
```

If no reply is received within the timeout period:
```
Timeout: No reply received.
```

## Use Cases

- AI agents requiring human input
- Automated workflows with human approval steps
- Development processes needing human verification
- Research tools integrating human feedback

## Socket Mode (Required)

This tool exclusively uses Slack's Socket Mode for all operations:
- WebSocket connections for real-time events
- No rate limits
- Instant message reception
- Requires both Bot Token (xoxb-) and App Token (xapp-)

## License

MIT