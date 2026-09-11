package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/minoplhy/nodem/internal/ech/transport"
	"github.com/minoplhy/nodem/internal/version"
	"golang.org/x/crypto/ssh"
)

// ResolveEndpointURL ensures the server URL has an appropriate scheme and appends the requested endpoint cleanly.
func ResolveEndpointURL(serverURL, endpoint string) string {
	serverURL = strings.TrimSpace(serverURL)
	if !strings.HasPrefix(serverURL, "http://") && !strings.HasPrefix(serverURL, "https://") {
		if strings.HasPrefix(serverURL, "localhost") || strings.HasPrefix(serverURL, "127.0.0.1") {
			serverURL = "http://" + serverURL
		} else {
			serverURL = "https://" + serverURL
		}
	}
	serverURL = strings.TrimRight(serverURL, "/")
	serverURL = strings.TrimSuffix(serverURL, "/api")
	serverURL = strings.TrimRight(serverURL, "/")

	endpoint = "/" + strings.TrimLeft(endpoint, "/")
	return serverURL + endpoint
}

// PullViaHTTPS queries the central server over HTTPS for assigned cluster ECH updates.
func PullViaHTTPS(ctx context.Context, cfg Config, clusters map[string]int64) (*transport.SyncResponse, error) {
	reqURL := ResolveEndpointURL(cfg.ServerURL, "/api/v1/agent/ech/sync")
	reqBody, _ := json.Marshal(transport.SyncRequest{
		ProxyType: cfg.ProxyType,
		Version:   version.Full(),
		Clusters:  clusters,
	})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Agent-Token", cfg.Token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var syncResp transport.SyncResponse
	if err := json.Unmarshal(bodyBytes, &syncResp); err != nil {
		return nil, fmt.Errorf("failed decoding server json: %w", err)
	}

	return &syncResp, nil
}

// AckViaHTTPS transmits deployment acknowledgment items back to the central server via HTTPS.
func AckViaHTTPS(ctx context.Context, cfg Config, acks []transport.ClusterAckItem) error {
	reqURL := ResolveEndpointURL(cfg.ServerURL, "/api/v1/agent/ech/ack")
	reqBody, _ := json.Marshal(transport.AckRequest{
		Acks: acks,
	})

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Agent-Token", cfg.Token)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// PullViaSSH invokes the remote sync command on the central server via SSH.
func PullViaSSH(ctx context.Context, cfg Config, clusters map[string]int64) (*transport.SyncResponse, error) {
	client, err := DialSSH(cfg)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	versJSON, _ := json.Marshal(clusters)
	cmd := fmt.Sprintf("sync --proxy %s --versions %q", cfg.ProxyType, string(versJSON))
	out, err := session.CombinedOutput(cmd)
	if err != nil {
		return nil, fmt.Errorf("ssh command failed: %s: %w", string(out), err)
	}

	var syncResp transport.SyncResponse
	if err := json.Unmarshal(out, &syncResp); err != nil {
		return nil, fmt.Errorf("failed unmarshaling SSH json response: %w (raw: %s)", err, string(out))
	}

	return &syncResp, nil
}

// AckViaSSH sends deployment acknowledgments to the central server via SSH.
func AckViaSSH(ctx context.Context, cfg Config, acks []transport.ClusterAckItem) error {
	client, err := DialSSH(cfg)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	acksJSON, _ := json.Marshal(acks)
	cmd := fmt.Sprintf("ack --acks %q", string(acksJSON))
	return session.Run(cmd)
}

// DialSSH establishes an authenticated SSH connection to the server.
func DialSSH(cfg Config) (*ssh.Client, error) {
	host := cfg.ServerURL
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	if strings.Contains(host, "/") {
		host = strings.Split(host, "/")[0]
	}
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	addr := fmt.Sprintf("%s:%d", host, cfg.SSHPort)
	var authMethods []ssh.AuthMethod

	if cfg.SSHKeyPath != "" {
		keyBytes, err := os.ReadFile(cfg.SSHKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed reading ssh key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed parsing ssh key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if cfg.Token != "" {
		authMethods = append(authMethods, ssh.Password(cfg.Token))
	}

	user := "sync"
	if cfg.Token != "" {
		user = cfg.Token
	}

	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	return ssh.Dial("tcp", addr, sshConfig)
}
