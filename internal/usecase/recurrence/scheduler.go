package recurrence

import (
	"context"
	"log/slog"
	"time"
)

const tickInterval = 24 * time.Hour

// Scheduler runs a daily goroutine that keeps task instances pre-generated
// for all active recurrence rules within a rolling horizon window.
type Scheduler struct {
	usecase     Usecase
	horizonDays int
	logger      *slog.Logger
}

func NewScheduler(usecase Usecase, horizonDays int, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		usecase:     usecase,
		horizonDays: horizonDays,
		logger:      logger,
	}
}

// Run blocks until ctx is cancelled. It extends the horizon immediately on
// startup, then repeats every 24 hours.
func (s *Scheduler) Run(ctx context.Context) {
	s.extend(ctx)

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.extend(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (s *Scheduler) extend(ctx context.Context) {
	until := time.Now().UTC().AddDate(0, 0, s.horizonDays)
	if err := s.usecase.ExtendHorizon(ctx, until); err != nil {
		s.logger.Error("extend recurrence horizon", "error", err)
	}
}
