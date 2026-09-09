package version

import (
	"fmt"
	"strings"
)

var (
	// Version is injected via -ldflags: -X github.com/minoplhy/nodem/internal/version.Version=...
	Version = "dev"
	// Commit is injected via -ldflags: -X github.com/minoplhy/nodem/internal/version.Commit=...
	Commit = "none"
	// BuildDate is injected via -ldflags: -X github.com/minoplhy/nodem/internal/version.BuildDate=...
	BuildDate = "unknown"
)

// Short returns the concise version or dev tag (e.g., "v1.2.0" or "dev-abc1234")
func Short() string {
	cleanVer := strings.TrimSpace(Version)
	if cleanVer == "" || cleanVer == "dev" {
		if Commit != "" && Commit != "none" {
			shortCommit := Commit
			if len(shortCommit) > 7 {
				shortCommit = shortCommit[:7]
			}
			return fmt.Sprintf("dev-%s", shortCommit)
		}
		return "dev"
	}
	return cleanVer
}

// Full returns the full formatted version with commit and build date
func Full() string {
	shortCommit := Commit
	if len(shortCommit) > 7 {
		shortCommit = shortCommit[:7]
	}

	cleanVer := strings.TrimSpace(Version)
	if cleanVer == "" || cleanVer == "dev" {
		if shortCommit != "" && shortCommit != "none" {
			return fmt.Sprintf("dev-%s", shortCommit)
		}
		return "dev"
	}

	if shortCommit != "" && shortCommit != "none" {
		if BuildDate != "" && BuildDate != "unknown" {
			return fmt.Sprintf("%s (commit: %s, built: %s)", cleanVer, shortCommit, BuildDate)
		}
		return fmt.Sprintf("%s (commit: %s)", cleanVer, shortCommit)
	}

	return cleanVer
}
