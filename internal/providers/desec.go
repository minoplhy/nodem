package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DesecProvider manages DNS records via the deSEC.io REST API.
type DesecProvider struct {
	BaseURL string
	Token   string
	Zone    string
	Client  *http.Client
}

type desecRRSet struct {
	Subname string   `json:"subname"`
	Type    string   `json:"type"`
	TTL     int      `json:"ttl"`
	Records []string `json:"records"`
}

// NewDesecProvider initializes a deSEC DNS provider.
func NewDesecProvider(token, zone string) *DesecProvider {
	return &DesecProvider{
		BaseURL: "https://desec.io/api/v1",
		Token:   token,
		Zone:    zone,
		Client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (d *DesecProvider) getSubnameAndZone(domain string) (string, string) {
	zone := d.Zone
	subname := ""
	if zone != "" && strings.HasSuffix(domain, "."+zone) {
		subname = strings.TrimSuffix(domain, "."+zone)
	} else if zone == "" {
		zone = domain
		subname = ""
	}
	return subname, zone
}

func (d *DesecProvider) ListRecords(ctx context.Context, recordName string) ([]string, error) {
	subname, zone := d.getSubnameAndZone(recordName)
	path := fmt.Sprintf("%s/domains/%s/rrsets/", d.BaseURL, zone)
	if subname != "" {
		path = fmt.Sprintf("%s/domains/%s/rrsets/?subname=%s", d.BaseURL, zone, subname)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+d.Token)

	resp, err := d.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("desec list rrsets failed (%d): %s", resp.StatusCode, string(b))
	}

	var rrsets []desecRRSet
	if err := json.NewDecoder(resp.Body).Decode(&rrsets); err != nil {
		return nil, err
	}

	var ips []string
	for _, rr := range rrsets {
		if rr.Type == "A" || rr.Type == "AAAA" {
			ips = append(ips, rr.Records...)
		}
	}
	return ips, nil
}

func (d *DesecProvider) AddRecord(ctx context.Context, recordName, ip, recordType string) error {
	subname, zone := d.getSubnameAndZone(recordName)

	existing, _ := d.ListRecords(ctx, recordName)
	records := append(existing, ip)

	rrset := desecRRSet{
		Subname: subname,
		Type:    recordType,
		TTL:     300,
		Records: records,
	}

	body, err := json.Marshal(rrset)
	if err != nil {
		return err
	}

	putPath := fmt.Sprintf("%s/domains/%s/rrsets/%s/%s/", d.BaseURL, zone, subname, recordType)
	if subname == "" {
		putPath = fmt.Sprintf("%s/domains/%s/rrsets/@/%s/", d.BaseURL, zone, recordType)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, putPath, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+d.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("desec add record failed: %s", string(b))
	}
	return nil
}

func (d *DesecProvider) DeleteRecord(ctx context.Context, recordName, ip, recordType string) error {
	subname, zone := d.getSubnameAndZone(recordName)

	existing, err := d.ListRecords(ctx, recordName)
	if err != nil {
		return err
	}

	var remaining []string
	for _, existingIP := range existing {
		if existingIP != ip {
			remaining = append(remaining, existingIP)
		}
	}

	if len(remaining) == 0 {
		delPath := fmt.Sprintf("%s/domains/%s/rrsets/%s/%s/", d.BaseURL, zone, subname, recordType)
		if subname == "" {
			delPath = fmt.Sprintf("%s/domains/%s/rrsets/@/%s/", d.BaseURL, zone, recordType)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, delPath, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Token "+d.Token)
		resp, err := d.Client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		return nil
	}

	rrset := desecRRSet{
		Subname: subname,
		Type:    recordType,
		TTL:     300,
		Records: remaining,
	}
	body, _ := json.Marshal(rrset)
	putPath := fmt.Sprintf("%s/domains/%s/rrsets/%s/%s/", d.BaseURL, zone, subname, recordType)
	if subname == "" {
		putPath = fmt.Sprintf("%s/domains/%s/rrsets/@/%s/", d.BaseURL, zone, recordType)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, putPath, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+d.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (d *DesecProvider) UpdateHTTPSRecord(ctx context.Context, params HTTPSRecordParams) error {
	subname, zone := d.getSubnameAndZone(params.Domain)
	formattedValue := params.ToRFC9460String()

	rrset := desecRRSet{
		Subname: subname,
		Type:    "HTTPS",
		TTL:     params.GetTTL(),
		Records: []string{formattedValue},
	}

	body, err := json.Marshal(rrset)
	if err != nil {
		return err
	}

	putPath := fmt.Sprintf("%s/domains/%s/rrsets/%s/HTTPS/", d.BaseURL, zone, subname)
	if subname == "" {
		putPath = fmt.Sprintf("%s/domains/%s/rrsets/@/HTTPS/", d.BaseURL, zone)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, putPath, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+d.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Try POST
		postPath := fmt.Sprintf("%s/domains/%s/rrsets/", d.BaseURL, zone)
		postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, postPath, bytes.NewReader(body))
		if err != nil {
			return err
		}
		postReq.Header.Set("Authorization", "Token "+d.Token)
		postReq.Header.Set("Content-Type", "application/json")

		postResp, err := d.Client.Do(postReq)
		if err != nil {
			return err
		}
		defer postResp.Body.Close()

		if postResp.StatusCode < 200 || postResp.StatusCode >= 300 {
			b, _ := io.ReadAll(postResp.Body)
			return fmt.Errorf("desec create HTTPS record failed: %s", string(b))
		}
		return nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("desec update HTTPS record failed: %s", string(b))
	}

	return nil
}
