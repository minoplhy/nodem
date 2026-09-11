package provider

import "context"

// DNSProvider defines operations for interacting with dynamic DNS providers.
type DNSProvider interface {
	ListRecords(ctx context.Context, recordName string) ([]string, error)
	AddRecord(ctx context.Context, recordName, ip, recordType string) error
	DeleteRecord(ctx context.Context, recordName, ip, recordType string) error
	UpdateHTTPSRecord(ctx context.Context, params HTTPSRecordParams) error
}

// DnsProvider is a type alias for DNSProvider for backward compatibility.
type DnsProvider = DNSProvider
