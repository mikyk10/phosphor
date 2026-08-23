package store

import (
	"context"
	"log/slog"
	"time"
)

// DefaultPruneInterval is how often the background pruner runs.
const DefaultPruneInterval = 5 * time.Minute

// StartPruner launches an in-process goroutine that periodically trims the
// execution history down to the `keep` most recent pipeline executions.
// It prunes once immediately and then every `interval`, stopping when ctx is
// cancelled. keep <= 0 disables pruning and StartPruner returns immediately.
func StartPruner(ctx context.Context, repo Repository, keep int, interval time.Duration) {
	if keep <= 0 {
		slog.Info("store: history pruning disabled", "max_history", keep)
		return
	}
	if interval <= 0 {
		interval = DefaultPruneInterval
	}

	slog.Info("store: history pruner started", "max_history", keep, "interval", interval)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		prune(repo, keep)
		for {
			select {
			case <-ctx.Done():
				slog.Info("store: history pruner stopped")
				return
			case <-ticker.C:
				prune(repo, keep)
			}
		}
	}()
}

func prune(repo Repository, keep int) {
	n, err := repo.PruneExecutions(keep)
	if err != nil {
		slog.Warn("store: prune executions failed", "err", err)
		return
	}
	if n > 0 {
		slog.Info("store: pruned execution history", "deleted", n, "kept", keep)
	}
}
