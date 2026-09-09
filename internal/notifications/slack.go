package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/minoplhy/nodem/internal/db"
)

// SlackNotifier sends webhook notifications to Slack.
type SlackNotifier struct {
	webhookURL string
	client     *http.Client
}

// NewSlackNotifier creates a new Slack notifier.
func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// SendNotification dispatches a message payload to Slack.
func (s *SlackNotifier) SendNotification(ctx context.Context, message, _ string) error {
	payload := map[string]interface{}{
		"text": message,
	}

	bodyJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack webhook failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// SendDnsUpdateNotification formats and sends batched DNS update alerts to Slack.
func (s *SlackNotifier) SendDnsUpdateNotification(ctx context.Context, group *db.TargetGroup, transitionsUp []TransitionUp, transitionsDown []string, ipsStatus []IPStatus) error {
	var ipsStatusLines []string
	for _, item := range ipsStatus {
		ipsStatusLines = append(ipsStatusLines, fmt.Sprintf("- `%s`: **%s**", item.IP, item.Status))
	}
	ipsStatusFormatted := strings.Join(ipsStatusLines, "\n")

	var parts []string
	var header string
	if len(transitionsUp) > 0 && len(transitionsDown) > 0 {
		header = fmt.Sprintf("🔄 **DNS Record Status Update**: `%s`", group.DnsRecord)
	} else if len(transitionsUp) > 0 {
		header = fmt.Sprintf("✅ **Host UP (Batched)**: `%s`", group.DnsRecord)
	} else {
		header = fmt.Sprintf("❌ **Host DOWN (Batched)**: `%s`", group.DnsRecord)
	}
	parts = append(parts, header)

	nowStr := time.Now().UTC().Format(time.RFC3339)

	if len(transitionsUp) > 0 {
		var upLines []string
		for _, item := range transitionsUp {
			details := []string{fmt.Sprintf("- `%s`", item.IP)}
			if item.DownSince != nil {
				details = append(details, fmt.Sprintf("  - Down since: %s", item.DownSince.Format(time.RFC3339)))
				diff := time.Now().UTC().Sub(*item.DownSince)
				secs := int64(diff.Seconds())
				if secs > 0 {
					details = append(details, fmt.Sprintf("  - Downtime duration: %dm %ds", secs/60, secs%60))
				}
			}
			details = append(details, fmt.Sprintf("  - Up since: %s", nowStr))
			upLines = append(upLines, strings.Join(details, "\n"))
		}
		parts = append(parts, fmt.Sprintf("Recovered (UP):\n%s", strings.Join(upLines, "\n")))
	}

	if len(transitionsDown) > 0 {
		var downLines []string
		for _, ip := range transitionsDown {
			details := []string{
				fmt.Sprintf("- `%s`", ip),
				fmt.Sprintf("  - Down since: %s", nowStr),
			}
			downLines = append(downLines, strings.Join(details, "\n"))
		}
		parts = append(parts, fmt.Sprintf("Failed (DOWN):\n%s", strings.Join(downLines, "\n")))
	}

	parts = append(parts, fmt.Sprintf("Group IPs Status:\n%s", ipsStatusFormatted))

	message := strings.Join(parts, "\n\n")
	level := "success"
	if len(transitionsDown) > 0 {
		level = "error"
	}

	return s.SendNotification(ctx, message, level)
}

// SendUnmanagedIpsNotification sends unmanaged anomaly notifications to Slack.
func (s *SlackNotifier) SendUnmanagedIpsNotification(ctx context.Context, group *db.TargetGroup, newlyDetected, newlyCleared []string) error {
	var parts []string
	parts = append(parts, fmt.Sprintf("⚠️ **Unmanaged IPs Update** for DNS Record `%s`", group.DnsRecord))

	if len(newlyDetected) > 0 {
		var lines []string
		for _, ip := range newlyDetected {
			lines = append(lines, fmt.Sprintf("- `%s`", ip))
		}
		parts = append(parts, fmt.Sprintf("Newly Detected Anomalies:\n%s", strings.Join(lines, "\n")))
	}

	if len(newlyCleared) > 0 {
		var lines []string
		for _, ip := range newlyCleared {
			lines = append(lines, fmt.Sprintf("- `%s`", ip))
		}
		parts = append(parts, fmt.Sprintf("Resolved Anomalies:\n%s", strings.Join(lines, "\n")))
	}

	message := strings.Join(parts, "\n\n")
	level := "info"
	if len(newlyDetected) > 0 {
		level = "warn"
	}

	return s.SendNotification(ctx, message, level)
}
