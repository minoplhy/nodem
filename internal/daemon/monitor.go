package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"node_monitor_go/internal/checkers"
	"node_monitor_go/internal/db"
	"node_monitor_go/internal/notifications"
	"node_monitor_go/internal/providers"
	"node_monitor_go/internal/rules"
)

type checkExecutionResult struct {
	IPID    int64
	CheckID int64
	Result  checkers.CheckResult
}

func runGroupMonitor(ctx context.Context, repo db.Repository, initialGroup db.TargetGroup) {
	groupID := initialGroup.ID
	checker := checkers.NewNodeChecker()

	notifiedUnmanagedIPs := make(map[string]bool)
	if existingAnomalies, err := repo.ListGroupAnomalies(ctx, groupID); err == nil {
		for _, ip := range existingAnomalies {
			notifiedUnmanagedIPs[ip] = true
		}
	} else {
		slog.Error("Error initializing unmanaged IPs from DB", "group_id", groupID, "error", err)
	}

	for {
		if ctx.Err() != nil {
			return
		}

		// 1. Fetch latest group details from DB
		group, err := repo.GetGroupDirect(ctx, groupID)
		if err != nil || group == nil {
			slog.Info("Group no longer exists in DB. Exiting monitor task.", "group_id", groupID)
			return
		}

		slog.Debug("Group monitor cycle tick", "group_id", groupID, "name", group.Name)

		ips, err := repo.ListIPs(ctx, groupID)
		if err != nil {
			slog.Error("Error listing IPs for group", "group_id", groupID, "error", err)
			select {
			case <-time.After(10 * time.Second):
				continue
			case <-ctx.Done():
				return
			}
		}

		checks, err := repo.ListChecks(ctx, groupID)
		if err != nil {
			slog.Error("Error listing checks for group", "group_id", groupID, "error", err)
			select {
			case <-time.After(10 * time.Second):
				continue
			case <-ctx.Done():
				return
			}
		}

		groupRules, err := repo.ListRules(ctx, groupID)
		if err != nil {
			slog.Error("Error listing rules for group", "group_id", groupID, "error", err)
			select {
			case <-time.After(10 * time.Second):
				continue
			case <-ctx.Done():
				return
			}
		}

		slog.Debug("Group loaded configuration", "group_id", groupID, "name", group.Name, "ips", len(ips), "checks", len(checks), "rules", len(groupRules))

		if len(ips) > 0 && len(checks) > 0 {
			// Run all checks across all enabled IPs in parallel
			var wg sync.WaitGroup
			var mu sync.Mutex
			var results []checkExecutionResult

			for _, ip := range ips {
				if !ip.Enabled {
					continue
				}
				for _, chk := range checks {
					wg.Add(1)
					go func(targetIP db.TargetIp, check db.CheckConfig) {
						defer wg.Done()
						res := checker.RunCheck(ctx, targetIP.IP, &check)
						var msg string
						if res.Message != nil {
							msg = *res.Message
						}
						slog.Debug("Group check finished", "group_id", groupID, "check_id", check.ID, "ip", targetIP.IP, "success", res.Success, "message", msg)

						mu.Lock()
						results = append(results, checkExecutionResult{
							IPID:    targetIP.ID,
							CheckID: check.ID,
							Result:  res,
						})
						mu.Unlock()
					}(ip, chk)
				}
			}
			wg.Wait()

			// Check results by check_id to evaluate global check failures
			checkResultsByCheck := make(map[int64][]bool)
			for _, r := range results {
				checkResultsByCheck[r.CheckID] = append(checkResultsByCheck[r.CheckID], r.Result.Success)
			}

			// Update status and counter values per IP per check
			for _, ip := range ips {
				if !ip.Enabled {
					continue
				}
				for _, chk := range checks {
					var rawSuccess bool
					var resultMsg *string
					for _, r := range results {
						if r.IPID == ip.ID && r.CheckID == chk.ID {
							rawSuccess = r.Result.Success
							resultMsg = r.Result.Message
							break
						}
					}

					// Apply global bypass logic
					isSuccess := rawSuccess
					bypassApplied := false
					if !rawSuccess && chk.BypassOnGlobalFailure {
						allFailed := true
						list := checkResultsByCheck[chk.ID]
						for _, succ := range list {
							if succ {
								allFailed = false
								break
							}
						}
						if allFailed && len(list) > 0 {
							isSuccess = true
							bypassApplied = true
						}
					}

					prevState, _ := repo.GetCheckState(ctx, ip.ID, chk.ID)
					var consecutiveUp, consecutiveDown int64
					status := "UNKNOWN"
					if prevState != nil {
						consecutiveUp = prevState.ConsecutiveUp
						consecutiveDown = prevState.ConsecutiveDown
						status = prevState.Status
					}
					prevStatus := status

					if isSuccess {
						consecutiveUp++
						consecutiveDown = 0
						if consecutiveUp >= chk.UpThreshold {
							status = "UP"
						}
					} else {
						consecutiveDown++
						consecutiveUp = 0
						if consecutiveDown >= chk.DownThreshold {
							status = "DOWN"
						}
					}

					slog.Debug("Updating check state", "group_id", groupID, "ip", ip.IP, "check_id", chk.ID, "raw_success", rawSuccess, "bypass_applied", bypassApplied, "is_success", isSuccess, "status", status, "consecutive_up", consecutiveUp, "consecutive_down", consecutiveDown)

					_ = repo.UpdateCheckState(ctx, ip.ID, chk.ID, consecutiveUp, consecutiveDown, status, resultMsg)

					// Log high-signal events
					shouldLog := !isSuccess || (isSuccess && prevStatus != "UP" && status == "UP")
					if shouldLog {
						var logMsg *string
						if isSuccess {
							msg := "Check recovered / healthy"
							logMsg = &msg
						} else {
							logMsg = resultMsg
						}
						_, _ = repo.AddCheckLog(ctx, ip.ID, chk.ID, isSuccess, logMsg)
					}
				}
			}

			// Evaluate rules
			groupStates, _ := repo.ListCheckStatesForGroup(ctx, groupID)
			statesMap := make(map[rules.CheckStateKey]db.CheckState)
			for _, st := range groupStates {
				statesMap[rules.CheckStateKey{IPID: st.IPID, CheckID: st.CheckID}] = st
			}

			var ipIDs []int64
			for _, ip := range ips {
				ipIDs = append(ipIDs, ip.ID)
			}

			providerConfig, _ := repo.GetProviderByIDDirect(ctx, group.DnsProviderID)
			if providerConfig != nil {
				dnsClient, err := providers.CreateProviderClient(providerConfig)
				if err == nil {
					activeDnsRecords, listErr := dnsClient.ListRecords(ctx, group.DnsRecord)
					var dnsRecordsState *[]string
					if listErr == nil {
						slog.Debug("Listed DNS records successfully", "group_id", groupID, "records", activeDnsRecords)
						dnsRecordsState = &activeDnsRecords
					} else {
						slog.Error("Failed to list DNS records from provider", "group_id", groupID, "error", listErr)
					}

					var transitionsUp []notifications.TransitionUp
					var transitionsDown []string
					var newlyDetected []string
					var newlyCleared []string

					if dnsRecordsState != nil {
						currentUnmanaged := make(map[string]bool)
						for _, activeIP := range *dnsRecordsState {
							found := false
							for _, target := range ips {
								if target.IP == activeIP {
									found = true
									break
								}
							}
							if !found {
								currentUnmanaged[activeIP] = true
							}
						}

						// Newly detected anomalies
						for ip := range currentUnmanaged {
							if !notifiedUnmanagedIPs[ip] {
								notifiedUnmanagedIPs[ip] = true
								slog.Info("Unmanaged IP detected in DNS record", "group_id", groupID, "ip", ip, "record", group.DnsRecord)
								_ = repo.AddGroupAnomaly(ctx, groupID, ip)
								newlyDetected = append(newlyDetected, ip)
							}
						}

						// Resolved anomalies
						for ip := range notifiedUnmanagedIPs {
							if !currentUnmanaged[ip] {
								delete(notifiedUnmanagedIPs, ip)
								slog.Info("Unmanaged IP cleared from DNS record", "group_id", groupID, "ip", ip, "record", group.DnsRecord)
								_ = repo.DeleteGroupAnomaly(ctx, groupID, ip)
								newlyCleared = append(newlyCleared, ip)
							}
						}
					}

					for _, ip := range ips {
						evalCtx := rules.EvaluationContext{
							IPID:        ip.ID,
							GroupID:     groupID,
							CheckStates: statesMap,
							IPIDs:       ipIDs,
						}

						shouldBeAdded := ip.DnsAdded
						hasStates := false
						for key := range statesMap {
							if key.IPID == ip.ID {
								hasStates = true
								break
							}
						}

						var ipStates []db.CheckState
						for key, st := range statesMap {
							if key.IPID == ip.ID {
								ipStates = append(ipStates, st)
							}
						}

						hasDown := false
						for _, s := range ipStates {
							if s.Status == "DOWN" {
								hasDown = true
								break
							}
						}
						allUp := len(ipStates) > 0
						for _, s := range ipStates {
							if s.Status != "UP" {
								allUp = false
								break
							}
						}

						if !ip.Enabled {
							shouldBeAdded = false
							slog.Debug("IP disabled, forcing should_be_added = false", "group_id", groupID, "ip", ip.IP)
						} else if len(groupRules) == 0 {
							// Safe default policy: UP if all checks are UP, DOWN if any check is DOWN
							if hasDown {
								shouldBeAdded = false
							} else if allUp {
								shouldBeAdded = true
							}
							slog.Debug("Default policy evaluation", "group_id", groupID, "ip", ip.IP, "has_down", hasDown, "all_up", allUp, "target_should_be_added", shouldBeAdded)
						} else {
							// Custom rules policy evaluation
							hasRemoveRules := false
							hasAddRules := false
							removeMatched := false
							addMatched := false

							for _, r := range groupRules {
								if r.Action == "RemoveFromDns" {
									hasRemoveRules = true
									if !removeMatched {
										var expr rules.RuleExpr
										if err := json.Unmarshal([]byte(r.ExpressionJSON), &expr); err == nil {
											if expr.Evaluate(&evalCtx) {
												removeMatched = true
											}
										}
									}
								} else if r.Action == "AddToDns" {
									hasAddRules = true
									if !addMatched {
										var expr rules.RuleExpr
										if err := json.Unmarshal([]byte(r.ExpressionJSON), &expr); err == nil {
											if expr.Evaluate(&evalCtx) {
												addMatched = true
											}
										}
									}
								}
							}

							if removeMatched {
								shouldBeAdded = false
							} else if addMatched {
								shouldBeAdded = true
							} else if hasRemoveRules && !hasAddRules {
								// If only Remove rules were configured, healthy nodes not matching removal
								// should be added/retained in DNS.
								if allUp {
									shouldBeAdded = true
								}
							} else if hasAddRules && !hasRemoveRules {
								// If only Add rules were configured, unhealthy nodes not matching add
								// should be removed from DNS.
								if hasDown {
									shouldBeAdded = false
								}
							}
							slog.Debug("Custom rules evaluation", "group_id", groupID, "ip", ip.IP, "remove_matched", removeMatched, "add_matched", addMatched, "target_should_be_added", shouldBeAdded)
						}

						// Sync state to DNS provider
						if dnsRecordsState != nil {
							ipIsPresent := false
							for _, existingIP := range *dnsRecordsState {
								if existingIP == ip.IP {
									ipIsPresent = true
									break
								}
							}

							rtype := "A"
							if strings.Contains(ip.IP, ":") {
								rtype = "AAAA"
							}

							now := time.Now().UTC()

							if shouldBeAdded && !ipIsPresent {
								slog.Info("IP transitions to UP. Adding to DNS record", "ip", ip.IP, "record", group.DnsRecord, "zone", providerConfig.Zone)
								if err := dnsClient.AddRecord(ctx, group.DnsRecord, ip.IP, rtype); err == nil {
									prevList, _ := repo.ListIPs(ctx, groupID)
									var transitionTime *time.Time
									for _, item := range prevList {
										if item.ID == ip.ID {
											transitionTime = item.LastChecked
											break
										}
									}
									_ = repo.UpdateIPStatus(ctx, ip.ID, "UP", true, &now)
									transitionsUp = append(transitionsUp, notifications.TransitionUp{
										IP:        ip.IP,
										DownSince: transitionTime,
									})
								} else {
									slog.Error("Failed to add DNS record", "error", err)
								}
							} else if !shouldBeAdded && ipIsPresent {
								slog.Info("IP transitions to DOWN. Removing from DNS record", "ip", ip.IP, "record", group.DnsRecord, "zone", providerConfig.Zone)
								if err := dnsClient.DeleteRecord(ctx, group.DnsRecord, ip.IP, rtype); err == nil {
									status := "DOWN"
									if !ip.Enabled || !hasStates {
										status = "UNKNOWN"
									}
									_ = repo.UpdateIPStatus(ctx, ip.ID, status, false, &now)
									if ip.Enabled {
										transitionsDown = append(transitionsDown, ip.IP)
									}
								} else {
									slog.Error("Failed to delete DNS record", "error", err)
								}
							} else {
								// Already in sync with provider
								currentStatus := "DOWN"
								if !ip.Enabled || !hasStates {
									currentStatus = "UNKNOWN"
								} else if ipIsPresent {
									currentStatus = "UP"
								}
								_ = repo.UpdateIPStatus(ctx, ip.ID, currentStatus, ipIsPresent, &now)
							}
						} else {
							// Provider list failure
							now := time.Now().UTC()
							currentStatus := "DOWN"
							if !ip.Enabled || !hasStates {
								currentStatus = "UNKNOWN"
							} else if ip.DnsAdded {
								currentStatus = "UP"
							}
							_ = repo.UpdateIPStatus(ctx, ip.ID, currentStatus, ip.DnsAdded, &now)
						}
					}

					if len(transitionsUp) > 0 || len(transitionsDown) > 0 {
						dispatchBatchedNotifications(ctx, repo, group, transitionsUp, transitionsDown)
					}

					if len(newlyDetected) > 0 || len(newlyCleared) > 0 {
						dispatchBatchedUnmanagedNotifications(ctx, repo, group, newlyDetected, newlyCleared)
					}
				}
			}
		}

		interval := time.Duration(group.CheckIntervalSecs) * time.Second
		slog.Debug("Group monitor cycle completed, sleeping", "group_id", groupID, "interval", interval)
		select {
		case <-time.After(interval):
		case <-ctx.Done():
			slog.Info("Monitor task for group stopping", "group_id", groupID)
			return
		}
	}
}
