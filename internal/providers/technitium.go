package providers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TechnitiumProvider implements DnsProvider for the Technitium DNS Server HTTP API.
type TechnitiumProvider struct {
	apiURL string
	token  string
	zone   string
	client *http.Client
}

// NewTechnitiumProvider creates a new Technitium DNS provider.
func NewTechnitiumProvider(apiURL, token, zone string) *TechnitiumProvider {
	return &TechnitiumProvider{
		apiURL: strings.TrimRight(apiURL, "/"),
		token:  token,
		zone:   zone,
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // Self-signed certs commonly used
				},
			},
		},
	}
}

// ListRecords queries Technitium for A and AAAA records matching recordName.
func (t *TechnitiumProvider) ListRecords(ctx context.Context, recordName string) ([]string, error) {
	reqURL := fmt.Sprintf(
		"%s/api/zones/records/get?token=%s&zone=%s&domain=%s&listZone=false",
		t.apiURL,
		url.QueryEscape(t.token),
		url.QueryEscape(t.zone),
		url.QueryEscape(recordName),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("technitium API error status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var data struct {
		Response struct {
			Records []struct {
				Type  string `json:"type"`
				RData struct {
					IPAddress string `json:"ipAddress"`
				} `json:"rData"`
			} `json:"records"`
		} `json:"response"`
	}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, err
	}

	var ips []string
	for _, rec := range data.Response.Records {
		if (rec.Type == "A" || rec.Type == "AAAA") && rec.RData.IPAddress != "" {
			ips = append(ips, rec.RData.IPAddress)
		}
	}
	return ips, nil
}

// AddRecord registers an A or AAAA record with Technitium.
func (t *TechnitiumProvider) AddRecord(ctx context.Context, recordName, ip, recordType string) error {
	reqURL := fmt.Sprintf(
		"%s/api/zones/records/add?token=%s&domain=%s&zone=%s&type=%s&ipAddress=%s",
		t.apiURL,
		url.QueryEscape(t.token),
		url.QueryEscape(recordName),
		url.QueryEscape(t.zone),
		url.QueryEscape(recordType),
		url.QueryEscape(ip),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("technitium add record failed: %s", string(bodyBytes))
	}

	return nil
}

// DeleteRecord deletes an A or AAAA record from Technitium.
func (t *TechnitiumProvider) DeleteRecord(ctx context.Context, recordName, ip, recordType string) error {
	reqURL := fmt.Sprintf(
		"%s/api/zones/records/delete?token=%s&domain=%s&zone=%s&type=%s&value=%s",
		t.apiURL,
		url.QueryEscape(t.token),
		url.QueryEscape(recordName),
		url.QueryEscape(t.zone),
		url.QueryEscape(recordType),
		url.QueryEscape(ip),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("technitium delete record failed: %s", string(bodyBytes))
	}

	return nil
}
