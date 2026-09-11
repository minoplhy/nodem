package dns

import (
	"fmt"
	"strings"

	"github.com/minoplhy/nodem/internal/db"
	"github.com/minoplhy/nodem/internal/provider"
)

// CreateProviderClient creates an appropriate provider.DNSProvider instance for the given configuration.
func CreateProviderClient(config *db.DnsProviderConfig) (provider.DNSProvider, error) {
	switch strings.ToUpper(config.ProviderType) {
	case "CLOUDFLARE":
		return NewCloudflareProvider(config.Token, config.Zone), nil
	case "TECHNITIUM":
		return NewTechnitiumProvider(config.APIURL, config.Token, config.Zone), nil
	case "DESEC":
		return NewDesecProvider(config.Token, config.Zone), nil
	case "HOOK":
		return NewHookProvider(config.APIURL), nil
	default:
		return nil, fmt.Errorf("unsupported DNS provider type: %s", config.ProviderType)
	}
}
