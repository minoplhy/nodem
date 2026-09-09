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

// UpdateHTTPSRecord creates or updates an RFC 9460 Type 65 HTTPS resource record on Technitium.
func (t *TechnitiumProvider) UpdateHTTPSRecord(ctx context.Context, params HTTPSRecordParams) error {
	zone := t.zone
	if zone == "" {
		zone = params.GetDomain()
	}

	ttl := params.GetTTL()
	svcParams := params.ToTechnitiumParams()

	// 1. Delete existing HTTPS record to avoid duplicates
	delURL := fmt.Sprintf(
		"%s/api/zones/records/delete?token=%s&domain=%s&zone=%s&type=HTTPS",
		t.apiURL,
		url.QueryEscape(t.token),
		url.QueryEscape(params.GetDomain()),
		url.QueryEscape(zone),
	)
	if delReq, err := http.NewRequestWithContext(ctx, http.MethodGet, delURL, nil); err == nil {
		if delResp, err := t.client.Do(delReq); err == nil {
			_ = delResp.Body.Close()
		}
	}

	// 2. Add the updated HTTPS record
	addURL, err := url.Parse(fmt.Sprintf("%s/api/zones/records/add", t.apiURL))
	if err != nil {
		return fmt.Errorf("invalid Technitium API URL: %w", err)
	}

	q := addURL.Query()
	q.Set("token", t.token)
	q.Set("domain", params.GetDomain())
	q.Set("zone", zone)
	q.Set("type", "HTTPS")
	q.Set("svcPriority", fmt.Sprintf("%d", params.GetPriority()))
	q.Set("svcTargetName", params.GetTarget())
	q.Set("svcParams", svcParams)
	q.Set("ttl", fmt.Sprintf("%d", ttl))
	q.Set("overwrite", "true")
	addURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addURL.String(), nil)
	if err != nil {
		return err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("technitium add HTTPS record failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var res struct {
		Status       string `json:"status"`
		ErrorMessage string `json:"errorMessage,omitempty"`
	}
	if err := json.Unmarshal(bodyBytes, &res); err == nil && strings.EqualFold(res.Status, "error") {
		return fmt.Errorf("technitium error: %s", res.ErrorMessage)
	}

	return nil
}
