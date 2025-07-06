package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/spf13/cobra"
)

type Config struct {
	SlackToken     string `json:"slack_token"`
	AppToken       string `json:"app_token"`      // App-level token for Socket Mode
	DefaultChannel string `json:"default_channel"`
	DefaultTimeout int    `json:"default_timeout"` // Default timeout in seconds
}

var (
	question string
	title    string
	mention  string
	channel  string
	threadTS string
	timeout  int
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "ask-human-cli",
		Short: "A stateless CLI tool to post questions to Slack",
		Long:  `ask-human-cli is a stateless CLI tool that posts questions to Slack as a bot.`,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	var askCmd = &cobra.Command{
		Use:   "ask",
		Short: "Post a question to Slack",
		RunE:  runAsk,
	}

	askCmd.Flags().StringVar(&question, "question", "", "Question content (required)")
	askCmd.Flags().StringVar(&title, "title", "", "Title for current session (required when --thread-ts is not specified)")
	askCmd.Flags().StringVar(&mention, "mention", "", "User to notify - @username or @displayname (e.g., @hogelog)")
	askCmd.Flags().StringVar(&channel, "channel", "", "Channel to post to")
	askCmd.Flags().StringVar(&threadTS, "thread-ts", "", "Thread timestamp for replies")
	askCmd.Flags().IntVar(&timeout, "timeout", 0, "Timeout in seconds (0 means use default from config)")

	askCmd.MarkFlagRequired("question")

	var setupCmd = &cobra.Command{
		Use:   "setup",
		Short: "Generate configuration file",
		Long:  `Generate configuration file at ~/.config/ask-human-cli/config.json`,
		RunE:  runSetup,
	}

	var descriptionCmd = &cobra.Command{
		Use:   "description",
		Short: "Show usage description",
		Run:   runDescription,
	}

	rootCmd.AddCommand(askCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(descriptionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runAsk(cmd *cobra.Command, args []string) error {
	if threadTS == "" && title == "" {
		return fmt.Errorf("--title is required when --thread-ts is not specified")
	}

	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	slackToken := getSlackToken(config)
	if slackToken == "" {
		return fmt.Errorf("Slack token not found. Please set SLACK_BOT_TOKEN or configure in ~/.config/ask-human-cli/config.json")
	}

	appToken := getAppToken(config)
	if appToken == "" {
		return fmt.Errorf("App token not found. Please set SLACK_APP_TOKEN or configure app_token in ~/.config/ask-human-cli/config.json")
	}

	channelName := getChannel(config)
	if channelName == "" {
		return fmt.Errorf("Channel not found. Please provide --channel or configure default_channel in ~/.config/ask-human-cli/config.json")
	}

	api := slack.New(slackToken, slack.OptionAppLevelToken(appToken))
	
	client := socketmode.New(
		api,
		socketmode.OptionDebug(false),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		if err := client.RunContext(ctx); err != nil {
			errChan <- err
		}
	}()

	time.Sleep(2 * time.Second)

	channelID, err := resolveChannelID(api, channelName)
	if err != nil {
		return fmt.Errorf("failed to resolve channel ID: %w", err)
	}

	var userID string
	if mention != "" {
		if !strings.HasPrefix(mention, "@") {
			return fmt.Errorf("mention must start with @ (e.g., @hogelog)")
		}
		
		username := strings.TrimPrefix(mention, "@")
		users, err := api.GetUsers()
		if err != nil {
			return fmt.Errorf("failed to get users for mention resolution: %w", err)
		}
		
		found := false
		for _, user := range users {
			if user.Name == username || user.Profile.DisplayName == username {
				userID = user.ID
				found = true
				break
			}
		}
		
		if !found {
			return fmt.Errorf("user @%s not found", username)
		}
	}

	if threadTS == "" {
		var message = "📝 " + title

	  options := []slack.MsgOption{
	  	slack.MsgOptionText(message, false),
	  	slack.MsgOptionAsUser(false),
	  }

	  _, timestamp, err := api.PostMessage(channelID, options...)
	  if err != nil {
	  	return fmt.Errorf("failed to post message: %w", err)
	  }

	  fmt.Printf("Message posted successfully. Timestamp: %s\n", timestamp)

		threadTS = timestamp
	}

	var sb strings.Builder
	if userID != "" {
		sb.WriteString("<@" + userID + "> ")
	}
	sb.WriteString(question)
	message := sb.String()

	instructionOptions := []slack.MsgOption{
		slack.MsgOptionText(message, false),
		slack.MsgOptionAsUser(false),
		slack.MsgOptionTS(threadTS),
	}
	
	_, _, err = api.PostMessage(channelID, instructionOptions...)
	if err != nil {
		return fmt.Errorf("failed to post instruction message: %w", err)
	}

	waitTimeout := getTimeout(config)
	if waitTimeout <= 0 {
		waitTimeout = 300
	}

	fmt.Printf("Waiting for reply (timeout: %d seconds)...\n", waitTimeout)

	waitThreadTS := threadTS
	reply, err := waitForReplySocketMode(client, api, channelID, waitThreadTS, waitTimeout)
	if err != nil {
		return fmt.Errorf("error waiting for reply: %w", err)
	}

	var responseMessage string
	
	if reply == nil {
		fmt.Println("Timeout: No reply received.")
		responseMessage = "⏱️ Timed out waiting for response."
	} else {
		userInfo, err := api.GetUserInfo(reply.User)
		var fromDisplay string
		if err == nil {
			if userInfo.Profile.DisplayName != "" {
				fromDisplay = "@" + userInfo.Profile.DisplayName
			} else {
				fromDisplay = "@" + userInfo.Name
			}
		} else {
			fromDisplay = reply.User
		}
		
		fmt.Println("\nReply received:")
		fmt.Printf("From: %s\n", fromDisplay)
		fmt.Printf("Text: %s\n", reply.Text)
		fmt.Printf("Thread TS: %s\n", reply.ThreadTimestamp)
		
		if ts, err := parseSlackTimestamp(reply.Timestamp); err == nil {
			fmt.Printf("Timestamp: %s\n", ts.Format("2006-01-02 15:04"))
		} else {
			fmt.Printf("Timestamp: %s\n", reply.Timestamp)
		}
		
		responseMessage = fmt.Sprintf("✅ Response received from %s", fromDisplay)
	}
	
	responseOptions := []slack.MsgOption{
		slack.MsgOptionText(responseMessage, false),
		slack.MsgOptionAsUser(false),
		slack.MsgOptionTS(threadTS),
	}
	
	_, _, err = api.PostMessage(channelID, responseOptions...)
	if err != nil {
		fmt.Printf("Warning: Failed to post response status: %v\n", err)
	}

	cancel()

	return nil
}

func loadConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(home, ".config", "ask-human-cli", "config.json")
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func getSlackToken(config *Config) string {
	if envToken := os.Getenv("SLACK_BOT_TOKEN"); envToken != "" {
		return envToken
	}
	return config.SlackToken
}

func getAppToken(config *Config) string {
	if envToken := os.Getenv("SLACK_APP_TOKEN"); envToken != "" {
		return envToken
	}
	return config.AppToken
}

func getChannel(config *Config) string {
	if channel != "" {
		return channel
	}
	return config.DefaultChannel
}

func resolveChannelID(api *slack.Client, channelName string) (string, error) {
	if !strings.HasPrefix(channelName, "#") {
		channelName = "#" + channelName
	}
	
	channels, _, err := api.GetConversations(&slack.GetConversationsParameters{
		Types: []string{"public_channel", "private_channel"},
	})
	if err != nil {
		return "", err
	}

	targetName := strings.TrimPrefix(channelName, "#")
	for _, channel := range channels {
		if channel.Name == targetName {
			return channel.ID, nil
		}
	}

	return "", fmt.Errorf("channel %s not found", channelName)
}

func getTimeout(config *Config) int {
	if timeout > 0 {
		return timeout
	}
	if config.DefaultTimeout > 0 {
		return config.DefaultTimeout
	}
	return 300 // Default 5 minutes
}


func waitForReplySocketMode(client *socketmode.Client, api *slack.Client, channelID, threadTS string, timeoutSeconds int) (*slack.Message, error) {
	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	
	authTest, err := api.AuthTest()
	if err != nil {
		return nil, fmt.Errorf("failed to get bot user ID: %w", err)
	}
	botUserID := authTest.UserID
	
	eventCh := make(chan *slack.Message, 1)
	
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case evt := <-client.Events:
				switch evt.Type {
				case socketmode.EventTypeEventsAPI:
					eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
					if !ok {
						continue
					}
					
					switch eventsAPIEvent.Type {
					case slackevents.CallbackEvent:
						innerEvent := eventsAPIEvent.InnerEvent
						
						switch ev := innerEvent.Data.(type) {
						case *slackevents.MessageEvent:
							if ev.ThreadTimeStamp == threadTS && ev.Channel == channelID {
								if ev.User != botUserID && ev.SubType != "bot_message" {
									msg := &slack.Message{
										Msg: slack.Msg{
											Type:      ev.Type,
											Channel:   ev.Channel,
											User:      ev.User,
											Text:      ev.Text,
											Timestamp: ev.TimeStamp,
											ThreadTimestamp: ev.ThreadTimeStamp,
										},
									}
									eventCh <- msg
									client.Ack(*evt.Request)
									return
								}
							}
						}
					}
					
					client.Ack(*evt.Request)
				}
			}
		}
	}()
	
	select {
	case msg := <-eventCh:
		return msg, nil
	case <-ctx.Done():
		return nil, nil
	}
}

func parseSlackTimestamp(ts string) (time.Time, error) {
	parts := strings.Split(ts, ".")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid timestamp format")
	}
	
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	
	return time.Unix(sec, 0), nil
}

func runSetup(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".config", "ask-human-cli")
	configPath := filepath.Join(configDir, "config.json")

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("Configuration file already exists at: %s\n", configPath)
		fmt.Print("Do you want to overwrite it? (y/N): ")
		
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		
		if answer != "y" && answer != "yes" {
			fmt.Println("Setup cancelled.")
			return nil
		}
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter your Slack Bot Token (xoxb-...): ")
	token, _ := reader.ReadString('\n')
	token = strings.TrimSpace(token)

	fmt.Print("Enter your Slack App Token (xapp-...): ")
	appToken, _ := reader.ReadString('\n')
	appToken = strings.TrimSpace(appToken)

	fmt.Print("Enter default channel (e.g., #general): ")
	channel, _ := reader.ReadString('\n')
	channel = strings.TrimSpace(channel)

	if channel != "" && !strings.HasPrefix(channel, "#") {
		channel = "#" + channel
	}

	fmt.Print("Enter default timeout in seconds (default: 300): ")
	timeoutStr, _ := reader.ReadString('\n')
	timeoutStr = strings.TrimSpace(timeoutStr)
	
	defaultTimeout := 300
	if timeoutStr != "" {
		if t, err := fmt.Sscanf(timeoutStr, "%d", &defaultTimeout); err != nil || t != 1 {
			fmt.Println("Invalid timeout, using default 300 seconds")
			defaultTimeout = 300
		}
	}

	config := Config{
		SlackToken:     token,
		AppToken:       appToken,
		DefaultChannel: channel,
		DefaultTimeout: defaultTimeout,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("\nConfiguration saved to: %s\n", configPath)
	fmt.Println("\nYou can now use ask-human-cli with:")
	fmt.Println("  ask-human-cli ask --question \"Your question here\" --title \"Question title\"")
	
	return nil
}

func runDescription(cmd *cobra.Command, args []string) {
	description := `ask-human-cli is a Slack bot that posts questions and waits for human responses using Socket Mode.

How it works:
1. Posts your question to a Slack channel
2. Waits for a human to reply in the thread
3. Returns the reply or times out

Quick start:
Start a new conversation:
   ask-human-cli ask --question "What's the deployment process?" --title "Session title"

The bot will:
- Create a new thread with your question
- Wait for a human response (default: 5 minutes)
- Display the reply with Thread TS for follow-up questions
- Post a status message to the thread

Continue in the same thread:
   ask-human-cli ask --question "Can you explain step 3 in detail?" --thread-ts "1234567890.123456"

Using --thread-ts allows you to:
- Continue conversations in the same Slack thread
- Maintain context across multiple questions
- Keep related discussions organized in one place

Perfect for:
- AI agents needing human input
- Automated workflows requiring manual approval
- Multi-turn conversations with humans
- Session-based interactions

For detailed command options, run:
  ask-human-cli ask --help`

	fmt.Println(description)
}
