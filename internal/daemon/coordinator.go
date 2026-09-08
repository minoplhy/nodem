package daemon

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"node_monitor_go/internal/db"
)

type activeTask struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// Daemon coordinates background group health checks and session maintenance.
type Daemon struct {
	repo        db.Repository
	activeTasks map[int64]*activeTask
	mu          sync.Mutex
}

// New creates a new Daemon instance.
func New(repo db.Repository) *Daemon {
	return &Daemon{
		repo:        repo,
		activeTasks: make(map[int64]*activeTask),
	}
}

// Start launches the coordinator loop and session cleanup goroutine.
func (d *Daemon) Start(ctx context.Context) {
	slog.Info("Daemon coordinator starting...")

	// Spawn frequent periodic session cleanup (every 2 minutes)
	go func() {
		// Run initial cleanup immediately
		if err := d.repo.CleanupExpiredSessions(ctx); err != nil {
			slog.Error("Failed initial cleanup of expired sessions", "error", err)
		}
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				slog.Debug("Running periodic session cleanup task...")
				if err := d.repo.CleanupExpiredSessions(ctx); err != nil {
					slog.Error("Failed to clean up expired sessions", "error", err)
				}
			case <-ctx.Done():
				slog.Debug("Session cleanup task shutting down.")
				return
			}
		}
	}()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Initial reconcile
	if err := d.reconcileTasks(ctx); err != nil {
		slog.Error("Error reconciling target groups", "error", err)
	}

	for {
		select {
		case <-ticker.C:
			slog.Debug("Daemon coordinator tick: reconciling tasks")
			if err := d.reconcileTasks(ctx); err != nil {
				slog.Error("Error reconciling target groups", "error", err)
			}
		case <-ctx.Done():
			slog.Info("Daemon coordinator shutting down: stopping active monitoring tasks...")
			d.mu.Lock()
			var waitChans []chan struct{}
			for id, task := range d.activeTasks {
				task.cancel()
				waitChans = append(waitChans, task.done)
				delete(d.activeTasks, id)
			}
			d.mu.Unlock()

			for _, done := range waitChans {
				<-done
			}
			slog.Info("All active monitoring tasks stopped.")
			return
		}
	}
}

func (d *Daemon) reconcileTasks(ctx context.Context) error {
	groups, err := d.repo.ListAllGroups(ctx)
	if err != nil {
		return err
	}

	currentGroupIDs := make(map[int64]db.TargetGroup)
	for _, g := range groups {
		if g.Enabled {
			currentGroupIDs[g.ID] = g
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Stop tasks for groups that no longer exist or are disabled
	for id, task := range d.activeTasks {
		if _, exists := currentGroupIDs[id]; !exists {
			task.cancel()
			<-task.done
			delete(d.activeTasks, id)
			slog.Info("Stopped monitoring task for group", "group_id", id)
		}
	}

	// Start tasks for new groups
	for id, group := range currentGroupIDs {
		if _, exists := d.activeTasks[id]; !exists {
			taskCtx, cancel := context.WithCancel(ctx)
			done := make(chan struct{})

			d.activeTasks[id] = &activeTask{
				cancel: cancel,
				done:   done,
			}

			slog.Info("Started monitoring task for group", "name", group.Name, "group_id", id)
			go func(grp db.TargetGroup) {
				defer close(done)
				runGroupMonitor(taskCtx, d.repo, grp)
			}(group)
		}
	}

	return nil
}
