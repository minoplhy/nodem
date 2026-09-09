package engine

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed Dockerfile.openssl
var DefaultDockerfile []byte

// GeneratedKey represents the cryptographic components of an ECH key pair.
type GeneratedKey struct {
	PEMBytes      []byte
	PrivateKeyPEM []byte
	ECHConfigPEM  []byte
	Base64ECH     string
	PublicName    string
	CipherSuite   string
	MaxNameLen    int
}

// Engine wraps container execution and cryptographic parsing for OpenSSL with native ECH support.
type Engine struct {
	DockerImage      string
	ContainerRuntime string // "auto", "docker", "podman"
}

// NewEngine creates a new ECH generation engine.
func NewEngine(dockerImage, containerRuntime string) *Engine {
	if dockerImage == "" {
		dockerImage = "echm-openssl4"
	}
	if containerRuntime == "" {
		containerRuntime = "auto"
	}
	return &Engine{
		DockerImage:      dockerImage,
		ContainerRuntime: containerRuntime,
	}
}

// DetectRuntime identifies the available container or host runtime.
func (e *Engine) DetectRuntime() (string, error) {
	if e.ContainerRuntime != "" && e.ContainerRuntime != "auto" {
		if _, err := exec.LookPath(e.ContainerRuntime); err != nil {
			return "", fmt.Errorf("configured container runtime '%s' not found on PATH: %w", e.ContainerRuntime, err)
		}
		return e.ContainerRuntime, nil
	}

	// 1. Check host native openssl with ech support
	if opensslPath, err := exec.LookPath("openssl"); err == nil {
		cmd := exec.Command(opensslPath, "ech", "-help")
		if err := cmd.Run(); err == nil {
			return "host-openssl", nil
		}
	}

	// 2. Check container engines
	for _, rt := range []string{"docker", "podman"} {
		if _, err := exec.LookPath(rt); err == nil {
			return rt, nil
		}
	}

	return "", fmt.Errorf("no suitable ECH engine found: host OpenSSL lacks 'ech' support, and neither Docker nor Podman is installed")
}

// CheckImageExists verifies if the container image is available locally.
func (e *Engine) CheckImageExists(ctx context.Context, runtimeBin string) bool {
	cmd := exec.CommandContext(ctx, runtimeBin, "image", "inspect", e.DockerImage)
	return cmd.Run() == nil
}

// BuildImage builds the OpenSSL ECH container using the embedded Dockerfile.
func (e *Engine) BuildImage(ctx context.Context, tag string, stdout, stderr io.Writer) error {
	if tag == "" {
		tag = e.DockerImage
	}

	rt, err := e.DetectRuntime()
	if err != nil && rt == "" {
		for _, bin := range []string{"docker", "podman"} {
			if _, err := exec.LookPath(bin); err == nil {
				rt = bin
				break
			}
		}
	}
	if rt != "docker" && rt != "podman" {
		for _, bin := range []string{"docker", "podman"} {
			if _, err := exec.LookPath(bin); err == nil {
				rt = bin
				break
			}
		}
	}

	if rt != "docker" && rt != "podman" {
		return fmt.Errorf("neither 'docker' nor 'podman' was found on PATH")
	}

	tmpDir, err := os.MkdirTemp("", "ech-build-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, DefaultDockerfile, 0644); err != nil {
		return fmt.Errorf("failed to write embedded Dockerfile: %w", err)
	}

	cmd := exec.CommandContext(ctx, rt, "build", "-t", tag, "-f", dockerfilePath, tmpDir)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("container build failed: %w", err)
	}

	return nil
}

// GenerateECHKeyPair generates a new ECH key pair for a given public cover name.
func (e *Engine) GenerateECHKeyPair(ctx context.Context, publicName, cipherSuite string, maxNameLen int) (*GeneratedKey, error) {
	if publicName == "" {
		return nil, fmt.Errorf("publicName must not be empty")
	}

	cipherSuite = NormalizeCipherSuite(cipherSuite)

	rt, err := e.DetectRuntime()
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "ech-gen-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory for keygen: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	outFile := "echconfig.pem"
	hostOutPath := filepath.Join(tmpDir, outFile)

	var cmd *exec.Cmd

	if rt == "host-openssl" {
		args := []string{"ech", "-public_name", publicName, "-out", hostOutPath}
		if cipherSuite != "" {
			args = append(args, "-suite", cipherSuite)
		}
		if maxNameLen > 0 {
			args = append(args, "-max_name_len", fmt.Sprintf("%d", maxNameLen))
		}
		cmd = exec.CommandContext(ctx, "openssl", args...)
	} else {
		if !e.CheckImageExists(ctx, rt) {
			return nil, fmt.Errorf("container image '%s' not found locally in %s (run BuildImage or build container first)", e.DockerImage, rt)
		}

		volMount := fmt.Sprintf("%s:/data", tmpDir)
		if rt == "podman" || runtime.GOOS == "linux" {
			volMount = fmt.Sprintf("%s:/data:Z", tmpDir)
		}

		args := []string{"run", "--rm"}
		if runtime.GOOS != "windows" {
			args = append(args, "--user", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()))
		}

		args = append(args,
			"-v", volMount,
			e.DockerImage,
			"ech", "-public_name", publicName, "-out", fmt.Sprintf("/data/%s", outFile),
		)
		if cipherSuite != "" {
			args = append(args, "-suite", cipherSuite)
		}
		if maxNameLen > 0 {
			args = append(args, "-max_name_len", fmt.Sprintf("%d", maxNameLen))
		}
		cmd = exec.CommandContext(ctx, rt, args...)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("openssl ech execution failed (%s): %s: %w", rt, stderr.String(), err)
	}

	pemData, err := os.ReadFile(hostOutPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read generated ECH PEM file: %w", err)
	}

	key, err := ParseECHPEM(pemData)
	if err != nil {
		return nil, err
	}

	key.PublicName = publicName
	key.CipherSuite = cipherSuite
	key.MaxNameLen = maxNameLen

	return key, nil
}

// ParseECHPEM parses an ECH PEM file containing PRIVATE KEY and ECHCONFIG blocks.
func ParseECHPEM(pemData []byte) (*GeneratedKey, error) {
	var (
		privKeyPEM []byte
		echPEM     []byte
		base64ECH  string
	)

	rest := pemData
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}

		switch block.Type {
		case "PRIVATE KEY", "EC PRIVATE KEY", "RSA PRIVATE KEY":
			privKeyPEM = pem.EncodeToMemory(block)
		case "ECHCONFIG":
			echPEM = pem.EncodeToMemory(block)
			base64ECH = base64.StdEncoding.EncodeToString(block.Bytes)
		}
	}

	if len(echPEM) == 0 {
		return nil, fmt.Errorf("invalid ECH PEM: missing '-----BEGIN ECHCONFIG-----' block")
	}

	base64ECH = strings.TrimSpace(base64ECH)

	return &GeneratedKey{
		PEMBytes:      pemData,
		PrivateKeyPEM: privKeyPEM,
		ECHConfigPEM:  echPEM,
		Base64ECH:     base64ECH,
	}, nil
}

// NormalizeCipherSuite standardizes HPKE cipher suite strings for OpenSSL's OSSL_HPKE_str2suite.
func NormalizeCipherSuite(suite string) string {
	suite = strings.TrimSpace(strings.ToLower(suite))
	if suite == "" {
		return "x25519,hkdf-sha256,aes-128-gcm"
	}

	// Support slash format (e.g. "hkdf_sha256/aes_128_gcm")
	suite = strings.ReplaceAll(suite, "/", ",")

	rawParts := strings.Split(suite, ",")
	var parts []string
	for _, p := range rawParts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Convert underscores to hyphens for standard cipher names (e.g. hkdf_sha256 -> hkdf-sha256)
		p = strings.ReplaceAll(p, "_", "-")

		switch p {
		case "aes128gcm", "aes-128-gcm":
			parts = append(parts, "aes-128-gcm")
		case "aes256gcm", "aes-256-gcm":
			parts = append(parts, "aes-256-gcm")
		case "chacha20poly1305", "chachapoly1305", "chacha20-poly1305":
			parts = append(parts, "chacha20-poly1305")
		case "p256", "p-256":
			parts = append(parts, "p-256")
		case "p384", "p-384":
			parts = append(parts, "p-384")
		case "p521", "p-521":
			parts = append(parts, "p-521")
		case "x25519":
			parts = append(parts, "x25519")
		case "hkdfsha256", "hkdf-sha256":
			parts = append(parts, "hkdf-sha256")
		case "hkdfsha384", "hkdf-sha384":
			parts = append(parts, "hkdf-sha384")
		case "hkdfsha512", "hkdf-sha512":
			parts = append(parts, "hkdf-sha512")
		default:
			parts = append(parts, p)
		}
	}

	// If KEM was omitted (only 2 parts: KDF, AEAD like "hkdf-sha256,aes-128-gcm"),
	// prepend the standard default KEM: "x25519"
	if len(parts) == 2 {
		parts = append([]string{"x25519"}, parts...)
	}

	if len(parts) == 0 {
		return "x25519,hkdf-sha256,aes-128-gcm"
	}

	return strings.Join(parts, ",")
}
