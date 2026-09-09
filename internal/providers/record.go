package providers

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// HTTPSRecordParams defines parameters for creating or updating an RFC 9460 type 65 HTTPS resource record.
type HTTPSRecordParams struct {
	Domain     string
	Base64ECH  string
	PublicName string
	TTL        int
	ALPN       []string
	IPv4Hint   []string
	IPv6Hint   []string
	Priority   uint16
	Target     string
}

// GetDomain returns the target domain for the record.
func (p *HTTPSRecordParams) GetDomain() string {
	return p.Domain
}

// GetTTL returns the Time To Live.
func (p *HTTPSRecordParams) GetTTL() int {
	if p.TTL <= 0 {
		return 300
	}
	return p.TTL
}

// GetPriority returns the SVCB/HTTPS priority (defaults to 1).
func (p *HTTPSRecordParams) GetPriority() uint16 {
	if p.Priority == 0 {
		return 1
	}
	return p.Priority
}

// GetTarget returns the target name (defaults to ".").
func (p *HTTPSRecordParams) GetTarget() string {
	if p.Target == "" {
		return "."
	}
	return p.Target
}

// ToRFC9460String formats the record in RFC 9460 presentation format:
// e.g. 1 . alpn="h2,h3" ech="<BASE64>"
func (p *HTTPSRecordParams) ToRFC9460String() string {
	parts := []string{
		fmt.Sprintf("%d", p.GetPriority()),
		p.GetTarget(),
	}

	if len(p.ALPN) > 0 {
		parts = append(parts, fmt.Sprintf(`alpn="%s"`, strings.Join(p.ALPN, ",")))
	} else {
		parts = append(parts, `alpn="h2,h3"`)
	}

	if len(p.IPv4Hint) > 0 {
		parts = append(parts, fmt.Sprintf(`ipv4hint="%s"`, strings.Join(p.IPv4Hint, ",")))
	}

	if len(p.IPv6Hint) > 0 {
		parts = append(parts, fmt.Sprintf(`ipv6hint="%s"`, strings.Join(p.IPv6Hint, ",")))
	}

	parts = append(parts, fmt.Sprintf(`ech="%s"`, p.Base64ECH))
	return strings.Join(parts, " ")
}

// ECHHex converts the Base64-encoded ECHConfigList to a lowercase hexadecimal string for Technitium.
func (p *HTTPSRecordParams) ECHHex() string {
	trimmed := strings.TrimSpace(p.Base64ECH)
	if trimmed == "" {
		return ""
	}

	if isHexString(trimmed) {
		return strings.ToLower(trimmed)
	}

	if b, err := base64.StdEncoding.DecodeString(trimmed); err == nil && len(b) > 0 {
		return hex.EncodeToString(b)
	}
	if b, err := base64.RawStdEncoding.DecodeString(trimmed); err == nil && len(b) > 0 {
		return hex.EncodeToString(b)
	}
	if b, err := base64.URLEncoding.DecodeString(trimmed); err == nil && len(b) > 0 {
		return hex.EncodeToString(b)
	}

	return trimmed
}

func isHexString(s string) bool {
	if len(s) == 0 || len(s)%2 != 0 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// ToTechnitiumParams formats the record into Technitium's pipe-separated parameter syntax:
// e.g. alpn|h2,h3|ech|<HEX>
func (p *HTTPSRecordParams) ToTechnitiumParams() string {
	var pairs []string

	if len(p.ALPN) > 0 {
		pairs = append(pairs, "alpn", strings.Join(p.ALPN, ","))
	} else {
		pairs = append(pairs, "alpn", "h2,h3")
	}

	if len(p.IPv4Hint) > 0 {
		pairs = append(pairs, "ipv4hint", strings.Join(p.IPv4Hint, ","))
	}

	if len(p.IPv6Hint) > 0 {
		pairs = append(pairs, "ipv6hint", strings.Join(p.IPv6Hint, ","))
	}

	echHex := p.ECHHex()
	if echHex != "" {
		pairs = append(pairs, "ech", echHex)
	}
	return strings.Join(pairs, "|")
}
