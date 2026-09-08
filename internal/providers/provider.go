package providers

import (
	"context"
	"fmt"
	"strings"

	"node_monitor_go/internal/db"
)

// DnsProvider defines operations for interacting with dynamic DNS providers.
type DnsProvider interface {
	ListRecords(ctx context.Context, recordName string) ([]string, error)
	AddRecord(ctx context.Context, recordName, ip, recordType string) error
	DeleteRecord(ctx context.Context, recordName, ip, recordType string) error
}

// CreateProviderClient creates an appropriate DnsProvider instance for the given configuration.
func CreateProviderClient(config *db.DnsProviderConfig) (DnsProvider, error) {
	switch strings.ToUpper(config.ProviderType) {
	case "CLOUDFLARE":
		return NewCloudflareProvider(config.Token, config.Zone), nil
	case "TECHNITIUM":
		return NewTechnitiumProvider(config.APIURL, config.Token, config.Zone), nil
	default:
		return nil, fmt.Errorf("unsupported DNS provider type: %s", config.ProviderType)
	}
}
