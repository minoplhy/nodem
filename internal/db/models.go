package db

import (
	"time"
)

// User represents an administrator or tenant account.
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// Session represents an active authenticated user session.
type Session struct {
	SessionID  string     `json:"session_id"`
	PublicID   string     `json:"public_id"`
	UserID     int64      `json:"user_id"`
	ExpiresAt  time.Time  `json:"expires_at"`
	IPAddress  *string    `json:"ip_address,omitempty"`
	UserAgent  *string    `json:"user_agent,omitempty"`
	LastActive *time.Time `json:"last_active,omitempty"`
}

// DnsProviderConfig holds credentials and settings for DNS providers.
type DnsProviderConfig struct {
	ID           int64  `json:"id"`
	TenantID     int64  `json:"tenant_id"`
	Name         string `json:"name"`
	ProviderType string `json:"provider_type"` // "Cloudflare", "Technitium"
	APIURL       string `json:"api_url"`
	Token        string `json:"token"`
	Zone         string `json:"zone"`
}

// TargetGroup represents a group of nodes monitored for a specific DNS record.
type TargetGroup struct {
	ID                int64     `json:"id"`
	TenantID          int64     `json:"tenant_id"`
	Name              string    `json:"name"`
	DnsRecord         string    `json:"dns_record"`
	DnsProviderID     int64     `json:"dns_provider_id"`
	CheckIntervalSecs int64     `json:"check_interval_secs"`
	Enabled           bool      `json:"enabled"`
	CreatedAt         time.Time `json:"created_at"`
}

// TargetIp represents an individual IP belonging to a TargetGroup.
type TargetIp struct {
	ID           int64      `json:"id"`
	GroupID      int64      `json:"group_id"`
	IP           string     `json:"ip"`
	DnsAdded     bool       `json:"dns_added"`
	LastChecked  *time.Time `json:"last_checked,omitempty"`
	Status       string     `json:"status"` // "UP", "DOWN", "UNKNOWN"
	DisplayOrder int64      `json:"display_order"`
	Enabled      bool       `json:"enabled"`
}

// TargetIpInput represents batch IP input for synchronization.
type TargetIpInput struct {
	IP      string `json:"ip"`
	Enabled bool   `json:"enabled"`
}

// CheckConfig defines a health check for a TargetGroup.
type CheckConfig struct {
	ID                     int64   `json:"id"`
	GroupID                int64   `json:"group_id"`
	Name                   string  `json:"name"`
	Protocol               string  `json:"protocol"` // "TCP", "UDP", "HTTP", "HTTPS"
	Domain                 *string `json:"domain,omitempty"`
	Port                   uint16  `json:"port"`
	Path                   *string `json:"path,omitempty"`
	DownThreshold          int64   `json:"down_threshold"`
	UpThreshold            int64   `json:"up_threshold"`
	BypassOnGlobalFailure bool    `json:"bypass_on_global_failure"`
}

// CheckState tracks consecutive check successes and failures per IP and check.
type CheckState struct {
	IPID            int64   `json:"ip_id"`
	CheckID         int64   `json:"check_id"`
	ConsecutiveUp   int64   `json:"consecutive_up"`
	ConsecutiveDown int64   `json:"consecutive_down"`
	Status          string  `json:"status"` // "UP", "DOWN", "UNKNOWN"
	Message         *string `json:"message,omitempty"`
}

// CheckLog represents an audit log entry for a check execution.
type CheckLog struct {
	ID        int64     `json:"id"`
	IPID      int64     `json:"ip_id"`
	CheckID   int64     `json:"check_id"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
	Message   *string   `json:"message"`
	IPAddress *string   `json:"ip_address,omitempty"`
}

// GroupRule specifies custom logic for adding or removing an IP from DNS.
type GroupRule struct {
	ID             int64  `json:"id"`
	GroupID        int64  `json:"group_id"`
	ExpressionJSON string `json:"expression_json"`
	Action         string `json:"action"` // "AddToDns", "RemoveFromDns"
}

// NotificationChannel defines an alerting endpoint (Discord, Slack, Telegram).
type NotificationChannel struct {
	ID          int64  `json:"id"`
	TenantID    int64  `json:"tenant_id"`
	Name        string `json:"name"`
	ChannelType string `json:"channel_type"` // "DISCORD", "SLACK", "TELEGRAM"
	ConfigJSON  string `json:"config_json"`
}

// GroupNotification defines the linkage between a TargetGroup and a NotificationChannel.
type GroupNotification struct {
	GroupID      int64 `json:"group_id"`
	ChannelID    int64 `json:"channel_id"`
	NotifyOnUp   bool  `json:"notify_on_up"`
	NotifyOnDown bool  `json:"notify_on_down"`
}
