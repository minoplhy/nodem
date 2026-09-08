package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"node_monitor_go/internal/db"
)

// TransitionUp records a recovered IP and when it originally went down.
type TransitionUp struct {
	IP        string
	DownSince *time.Time
}

// IPStatus holds the display status of a target IP.
type IPStatus struct {
	IP     string
	Status string
}

// Notifier defines the interface for dispatching system and monitoring alerts.
type Notifier interface {
	SendNotification(ctx context.Context, message, level string) error
	SendDnsUpdateNotification(ctx context.Context, group *db.TargetGroup, transitionsUp []TransitionUp, transitionsDown []string, ipsStatus []IPStatus) error
	SendUnmanagedIpsNotification(ctx context.Context, group *db.TargetGroup, newlyDetected, newlyCleared []string) error
}

// CreateNotifier instantiates a Notifier based on the channel's configuration.
func CreateNotifier(channel *db.NotificationChannel) (Notifier, error) {
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(channel.ConfigJSON), &cfg); err != nil {
		return nil, fmt.Errorf("invalid channel config JSON: %w", err)
	}

	switch strings.ToUpper(channel.ChannelType) {
	case "DISCORD":
		urlVal, ok := cfg["url"].(string)
		if !ok || urlVal == "" {
			return nil, fmt.Errorf("missing 'url' parameter in Discord config")
		}
		return NewDiscordNotifier(urlVal), nil

	case "SLACK":
		urlVal, ok := cfg["url"].(string)
		if !ok || urlVal == "" {
			return nil, fmt.Errorf("missing 'url' parameter in Slack config")
		}
		return NewSlackNotifier(urlVal), nil

	case "TELEGRAM":
		botToken, ok1 := cfg["bot_token"].(string)
		chatID, ok2 := cfg["chat_id"].(string)
		if !ok1 || botToken == "" {
			return nil, fmt.Errorf("missing 'bot_token' parameter in Telegram config")
		}
		if !ok2 || chatID == "" {
			return nil, fmt.Errorf("missing 'chat_id' parameter in Telegram config")
		}
		return NewTelegramNotifier(botToken, chatID), nil

	default:
		return nil, fmt.Errorf("unsupported notification channel type: %s", channel.ChannelType)
	}
}
