package checkers

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"node_monitor_go/internal/db"
)

func checkHTTPHTTPS(ctx context.Context, ip net.IP, config *db.CheckConfig) CheckResult {
	port := config.Port
	path := "/"
	if config.Path != nil && *config.Path != "" {
		path = *config.Path
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}

	isHTTPS := strings.ToUpper(config.Protocol) == "HTTPS"
	domain := ""
	if config.Domain != nil {
		domain = strings.TrimSpace(*config.Domain)
	}

	targetAddr := net.JoinHostPort(ip.String(), fmt.Sprintf("%d", port))

	var transport *http.Transport
	var reqURL string

	if domain != "" && isHTTPS {
		slog.Debug("Checker (HTTP/HTTPS): manual resolution", "domain", domain, "targetAddr", targetAddr)
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				ServerName:         domain,
				InsecureSkipVerify: true,
			},
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, p, err := net.SplitHostPort(addr)
				if err == nil && (strings.EqualFold(host, domain) || strings.EqualFold(host, ip.String())) {
					dialPort := p
					if dialPort == "443" && port != 443 {
						dialPort = fmt.Sprintf("%d", port)
					}
					addr = net.JoinHostPort(ip.String(), dialPort)
				}
				dialer := net.Dialer{Timeout: 5 * time.Second}
				return dialer.DialContext(ctx, network, addr)
			},
		}
		if port != 443 {
			reqURL = fmt.Sprintf("https://%s:%d%s", domain, port, path)
		} else {
			reqURL = fmt.Sprintf("https://%s%s", domain, path)
		}
	} else {
		scheme := "http"
		if isHTTPS {
			scheme = "https"
		}
		ipFormatted := ip.String()
		if ip.To4() == nil {
			ipFormatted = fmt.Sprintf("[%s]", ip.String())
		}
		reqURL = fmt.Sprintf("%s://%s:%d%s", scheme, ipFormatted, port, path)
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	slog.Debug("Checker (HTTP/HTTPS): sending request", "url", reqURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		msg := fmt.Sprintf("HTTP client build failed: %v", err)
		return CheckResult{Success: false, Message: &msg}
	}

	if domain != "" {
		req.Host = domain
	}

	resp, err := client.Do(req)
	if err != nil {
		msg := fmt.Sprintf("HTTP request error: %v", err)
		return CheckResult{Success: false, Message: &msg}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		msg := fmt.Sprintf("HTTP request success: status %d %s", resp.StatusCode, resp.Status)
		return CheckResult{Success: true, Message: &msg}
	}

	msg := fmt.Sprintf("HTTP request failed: status %d %s", resp.StatusCode, resp.Status)
	return CheckResult{Success: false, Message: &msg}
}
