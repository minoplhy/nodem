package providers

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// HookProvider executes an external shell script or binary to handle DNS operations.
type HookProvider struct {
	ScriptPath string
}

// NewHookProvider creates a new HookProvider.
func NewHookProvider(scriptPath string) *HookProvider {
	return &HookProvider{ScriptPath: scriptPath}
}

func (h *HookProvider) runHook(ctx context.Context, action string, extraEnv map[string]string, args ...string) (string, error) {
	if h.ScriptPath == "" {
		return "", fmt.Errorf("hook script path is empty")
	}

	cmdArgs := append([]string{action}, args...)
	cmd := exec.CommandContext(ctx, h.ScriptPath, cmdArgs...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("DNS_ACTION=%s", action))
	for k, v := range extraEnv {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("dns hook execution failed: %s: %w", stderr.String(), err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

func (h *HookProvider) ListRecords(ctx context.Context, recordName string) ([]string, error) {
	out, err := h.runHook(ctx, "list", map[string]string{
		"DNS_RECORD_NAME": recordName,
	}, recordName)
	if err != nil {
		return nil, err
	}

	if out == "" {
		return nil, nil
	}

	lines := strings.Split(out, "\n")
	var ips []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			ips = append(ips, trimmed)
		}
	}
	return ips, nil
}

func (h *HookProvider) AddRecord(ctx context.Context, recordName, ip, recordType string) error {
	_, err := h.runHook(ctx, "add", map[string]string{
		"DNS_RECORD_NAME": recordName,
		"DNS_RECORD_IP":   ip,
		"DNS_RECORD_TYPE": recordType,
	}, recordName, ip, recordType)
	return err
}

func (h *HookProvider) DeleteRecord(ctx context.Context, recordName, ip, recordType string) error {
	_, err := h.runHook(ctx, "delete", map[string]string{
		"DNS_RECORD_NAME": recordName,
		"DNS_RECORD_IP":   ip,
		"DNS_RECORD_TYPE": recordType,
	}, recordName, ip, recordType)
	return err
}

func (h *HookProvider) UpdateHTTPSRecord(ctx context.Context, params HTTPSRecordParams) error {
	rfcString := params.ToRFC9460String()
	_, err := h.runHook(ctx, "update_https", map[string]string{
		"DNS_DOMAIN":      params.Domain,
		"DNS_ECH_BASE64":  params.Base64ECH,
		"DNS_PUBLIC_NAME": params.PublicName,
		"DNS_TTL":         fmt.Sprintf("%d", params.GetTTL()),
		"DNS_VALUE":       rfcString,
	}, params.Domain, rfcString)
	return err
}
