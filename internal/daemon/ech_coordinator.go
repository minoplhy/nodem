package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/minoplhy/nodem/internal/db"
	"github.com/minoplhy/nodem/internal/ech/engine"
	"github.com/minoplhy/nodem/internal/ech/transport"
	"github.com/minoplhy/nodem/internal/providers"
)

// RunECHCoordinator handles background ECH rotation scheduling, embedded SSH server, and two-phase DNS sync.
func RunECHCoordinator(ctx context.Context, repo db.Repository, sshPort uint16) {
	slog.Info("ECH coordinator starting...", "ssh_port", sshPort)

	pullService := transport.NewPullService(repo)

	// 1. Start embedded SSH server if port configured
	if sshPort > 0 {
		sshServer, err := transport.NewSSHServer(pullService, sshPort)
		if err != nil {
			slog.Error("Failed to initialize ECH SSH server", "error", err)
		} else {
			go func() {
				if err := sshServer.Start(ctx); err != nil {
					slog.Error("ECH SSH server exited with error", "error", err)
				}
			}()
		}
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Initial cycle
	reconcileECHClusters(ctx, repo)

	for {
		select {
		case <-ticker.C:
			reconcileECHClusters(ctx, repo)
		case <-ctx.Done():
			slog.Info("ECH coordinator shutting down.")
			return
		}
	}
}

func reconcileECHClusters(ctx context.Context, repo db.Repository) {
	clusters, err := repo.ListAllECHClusters(ctx)
	if err != nil {
		slog.Error("ECH reconcile: failed to list clusters", "error", err)
		return
	}

	now := time.Now().UTC()
	eng := engine.NewEngine("", "auto")

	for _, c := range clusters {
		if ctx.Err() != nil {
			return
		}

		// 1. Check if cluster needs key rotation
		needsRotation := false
		if c.AutoRotate {
			if c.CurrentVersion == 0 {
				needsRotation = true
			} else if c.NextRotationAt != nil && now.After(*c.NextRotationAt) {
				needsRotation = true
			}
		}

		if needsRotation {
			slog.Info("ECH reconcile: rotating keys for cluster", "cluster_id", c.ID, "name", c.Name)
			key, err := eng.GenerateECHKeyPair(ctx, c.PublicName, c.CipherSuite, c.MaxNameLen)
			if err != nil {
				slog.Error("ECH reconcile: key generation failed", "cluster_id", c.ID, "error", err)
				_, _ = repo.AddECHLog(ctx, c.ID, nil, nil, "ERROR", fmt.Sprintf("Key generation failed: %v", err))
				continue
			}

			nextRot := now.Add(time.Duration(c.RotationIntervalHours) * time.Hour)
			newVersion, err := repo.IncrementClusterVersion(ctx, c.ID, now, nextRot)
			if err != nil {
				slog.Error("ECH reconcile: failed to increment cluster version", "cluster_id", c.ID, "error", err)
				continue
			}

			_, err = repo.SaveNewECHKey(ctx, c.ID, newVersion, key.Base64ECH, string(key.PrivateKeyPEM), string(key.ECHConfigPEM), string(key.PEMBytes))
			if err != nil {
				slog.Error("ECH reconcile: failed to save new ECH key", "cluster_id", c.ID, "error", err)
				continue
			}

			// Mark all domains in cluster as PENDING for DNS sync
			domains, _ := repo.ListECHDomains(ctx, c.ID)
			for _, d := range domains {
				_ = repo.UpdateECHDomainSyncStatus(ctx, d.ID, "PENDING", nil)
			}

			c.CurrentVersion = newVersion
			_, _ = repo.AddECHLog(ctx, c.ID, nil, nil, "GENERATE", fmt.Sprintf("Auto-generated ECH key version %d", newVersion))
		}

		// 2. Check node sync readiness and update DNS records (Two-Phase Rollout)
		if c.CurrentVersion > 0 {
			allInSync, err := repo.AreAllNodesInSync(ctx, c.ID, c.CurrentVersion)
			if err != nil {
				slog.Error("ECH reconcile: failed checking node in-sync status", "cluster_id", c.ID, "error", err)
				continue
			}

			if allInSync {
				syncClusterDomainsDNS(ctx, repo, c)
			}
		}
	}
}

func syncClusterDomainsDNS(ctx context.Context, repo db.Repository, cluster db.ECHCluster) {
	domains, err := repo.ListECHDomains(ctx, cluster.ID)
	if err != nil {
		slog.Error("ECH reconcile: failed listing domains", "cluster_id", cluster.ID, "error", err)
		return
	}

	activeKey, err := repo.GetActiveECHKey(ctx, cluster.ID)
	if err != nil || activeKey == nil {
		return
	}

	now := time.Now().UTC()

	for _, d := range domains {
		if d.DNSStatus == "SYNCED" {
			continue
		}

		// Lookup DNS provider config
		provCfg, err := repo.GetProviderByIDDirect(ctx, d.DNSProviderID)
		if err != nil || provCfg == nil {
			slog.Error("ECH reconcile: provider not found for domain", "domain", d.Domain, "provider_id", d.DNSProviderID)
			continue
		}

		client, err := providers.CreateProviderClient(provCfg)
		if err != nil {
			slog.Error("ECH reconcile: failed creating provider client", "domain", d.Domain, "error", err)
			continue
		}

		var alpnList []string
		for _, part := range strings.Split(d.ALPN, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				alpnList = append(alpnList, trimmed)
			}
		}

		var ipv4List []string
		if d.IPv4Hint != nil && *d.IPv4Hint != "" {
			for _, p := range strings.Split(*d.IPv4Hint, ",") {
				if t := strings.TrimSpace(p); t != "" {
					ipv4List = append(ipv4List, t)
				}
			}
		}

		var ipv6List []string
		if d.IPv6Hint != nil && *d.IPv6Hint != "" {
			for _, p := range strings.Split(*d.IPv6Hint, ",") {
				if t := strings.TrimSpace(p); t != "" {
					ipv6List = append(ipv6List, t)
				}
			}
		}

		params := providers.HTTPSRecordParams{
			Domain:     d.Domain,
			Base64ECH:  activeKey.Base64ECH,
			PublicName: cluster.PublicName,
			TTL:        d.TTL,
			ALPN:       alpnList,
			IPv4Hint:   ipv4List,
			IPv6Hint:   ipv6List,
			Priority:   1,
			Target:     ".",
		}

		slog.Info("ECH reconcile: publishing DNS HTTPS record", "domain", d.Domain, "cluster_id", cluster.ID)
		if err := client.UpdateHTTPSRecord(ctx, params); err != nil {
			slog.Error("ECH reconcile: DNS update failed", "domain", d.Domain, "error", err)
			_ = repo.UpdateECHDomainSyncStatus(ctx, d.ID, "FAILED", &now)
			_, _ = repo.AddECHLog(ctx, cluster.ID, nil, &d.ID, "ERROR", fmt.Sprintf("DNS update failed for '%s': %v", d.Domain, err))
			continue
		}

		_ = repo.UpdateECHDomainSyncStatus(ctx, d.ID, "SYNCED", &now)
		_, _ = repo.AddECHLog(ctx, cluster.ID, nil, &d.ID, "DNS_UPDATE", fmt.Sprintf("DNS HTTPS record published for '%s'", d.Domain))
		slog.Info("ECH reconcile: DNS HTTPS record published successfully", "domain", d.Domain)
	}
}
