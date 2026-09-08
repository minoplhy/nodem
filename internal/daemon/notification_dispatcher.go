package daemon

import (
	"context"
	"log/slog"

	"node_monitor_go/internal/db"
	"node_monitor_go/internal/notifications"
)

func dispatchBatchedNotifications(
	ctx context.Context,
	repo db.Repository,
	group *db.TargetGroup,
	transitionsUp []notifications.TransitionUp,
	transitionsDown []string,
) {
	ipsList, err := repo.ListIPs(ctx, group.ID)
	if err != nil {
		ipsList = nil
	}

	var ipsStatus []notifications.IPStatus
	for _, target := range ipsList {
		ipsStatus = append(ipsStatus, notifications.IPStatus{
			IP:     target.IP,
			Status: target.Status,
		})
	}

	groupNotifs, err := repo.ListGroupNotifications(ctx, group.ID)
	if err != nil {
		return
	}

	for _, gn := range groupNotifs {
		var activeUps []notifications.TransitionUp
		if gn.GroupNotification.NotifyOnUp {
			activeUps = transitionsUp
		}

		var activeDowns []string
		if gn.GroupNotification.NotifyOnDown {
			activeDowns = transitionsDown
		}

		if len(activeUps) == 0 && len(activeDowns) == 0 {
			continue
		}

		notifier, err := notifications.CreateNotifier(&gn.Channel)
		if err != nil {
			slog.Error("Failed to create notifier", "channel", gn.Channel.Name, "id", gn.Channel.ID, "error", err)
			continue
		}

		go func(n notifications.Notifier, grp *db.TargetGroup, ups []notifications.TransitionUp, downs []string, status []notifications.IPStatus, chanName string, chanID int64) {
			bgCtx := context.Background()
			if err := n.SendDnsUpdateNotification(bgCtx, grp, ups, downs, status); err != nil {
				slog.Error("Failed to send DNS update notification", "channel", chanName, "id", chanID, "error", err)
			}
		}(notifier, group, activeUps, activeDowns, ipsStatus, gn.Channel.Name, gn.Channel.ID)
	}
}

func dispatchBatchedUnmanagedNotifications(
	ctx context.Context,
	repo db.Repository,
	group *db.TargetGroup,
	newlyDetected []string,
	newlyCleared []string,
) {
	groupNotifs, err := repo.ListGroupNotifications(ctx, group.ID)
	if err != nil {
		return
	}

	for _, gn := range groupNotifs {
		notifier, err := notifications.CreateNotifier(&gn.Channel)
		if err != nil {
			slog.Error("Failed to create notifier", "channel", gn.Channel.Name, "id", gn.Channel.ID, "error", err)
			continue
		}

		go func(n notifications.Notifier, grp *db.TargetGroup, detected, cleared []string, chanName string, chanID int64) {
			bgCtx := context.Background()
			if err := n.SendUnmanagedIpsNotification(bgCtx, grp, detected, cleared); err != nil {
				slog.Error("Failed to send unmanaged alert", "channel", chanName, "id", chanID, "error", err)
			}
		}(notifier, group, newlyDetected, newlyCleared, gn.Channel.Name, gn.Channel.ID)
	}
}
