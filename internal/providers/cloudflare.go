package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CloudflareProvider implements DnsProvider for the Cloudflare API v4.
type CloudflareProvider struct {
	token  string
	zone   string
	client *http.Client
}

// NewCloudflareProvider creates a new Cloudflare DNS provider.
func NewCloudflareProvider(token, zone string) *CloudflareProvider {
	return &CloudflareProvider{
		token: token,
		zone:  zone,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *CloudflareProvider) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
}

func isHex32(s string) bool {
	if len(s) != 32 {
		return false
	}
	for i := 0; i < len(s); i++ {
		b := s[i]
		if !((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')) {
			return false
		}
	}
	return true
}

func (c *CloudflareProvider) getZoneID(ctx context.Context) (string, error) {
	if isHex32(c.zone) {
		return c.zone, nil
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?name=%s", c.zone)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("cloudflare get zone ID for '%s' failed: %s", c.zone, string(bodyBytes))
	}

	var data struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return "", err
	}

	if len(data.Result) > 0 && data.Result[0].ID != "" {
		return data.Result[0].ID, nil
	}

	return "", fmt.Errorf("cloudflare zone '%s' not found or credentials invalid", c.zone)
}

// ListRecords queries Cloudflare for A and AAAA records matching recordName.
func (c *CloudflareProvider) ListRecords(ctx context.Context, recordName string) ([]string, error) {
	zoneID, err := c.getZoneID(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?name=%s&per_page=100", zoneID, recordName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cloudflare API returned error: %s", string(bodyBytes))
	}

	var data struct {
		Result []struct {
			Type    string `json:"type"`
			Content string `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, err
	}

	var ips []string
	for _, rec := range data.Result {
		if rec.Type == "A" || rec.Type == "AAAA" {
			ips = append(ips, rec.Content)
		}
	}
	return ips, nil
}

// AddRecord adds an A or AAAA record to Cloudflare.
func (c *CloudflareProvider) AddRecord(ctx context.Context, recordName, ip, recordType string) error {
	zoneID, err := c.getZoneID(ctx)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneID)
	payload := map[string]interface{}{
		"type":    recordType,
		"name":    recordName,
		"content": ip,
		"proxied": false,
		"ttl":     60,
	}
	bodyJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cloudflare add record failed: %s", string(bodyBytes))
	}

	return nil
}

// DeleteRecord searches for and removes any matching DNS records in Cloudflare.
func (c *CloudflareProvider) DeleteRecord(ctx context.Context, recordName, ip, recordType string) error {
	zoneID, err := c.getZoneID(ctx)
	if err != nil {
		return err
	}

	queryURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?name=%s&type=%s&content=%s", zoneID, recordName, recordType, ip)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("cloudflare query record failed: %s", string(bodyBytes))
	}

	var data struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return err
	}

	for _, rec := range data.Result {
		delURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneID, rec.ID)
		delReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, delURL, nil)
		if err != nil {
			return err
		}
		c.setHeaders(delReq)

		delResp, err := c.client.Do(delReq)
		if err != nil {
			return err
		}
		defer delResp.Body.Close()

		if delResp.StatusCode < 200 || delResp.StatusCode >= 300 {
			delBody, _ := io.ReadAll(delResp.Body)
			return fmt.Errorf("cloudflare delete record ID %s failed: %s", rec.ID, string(delBody))
		}
	}

	return nil
}
