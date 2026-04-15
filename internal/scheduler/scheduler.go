package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"trendpulse/internal/calculator"
	"trendpulse/internal/domain"
	"trendpulse/internal/repository"
)

// Config holds the scheduler's runtime parameters.
type Config struct {
	// Interval between each runOnce cycle.
	Interval time.Duration
	// SignalLookback is the time window to fetch signals for each trend.
	SignalLookback time.Duration
}

// Scheduler periodically triggers all registered strategies to compute
// TrendStats for all trends and persists the results.
type Scheduler struct {
	trendRepo    repository.TrendRepository
	signalRepo   repository.SignalRepository
	statsRepo    repository.StatsRepository
	registry     *calculator.Registry
	cfg          Config
	logger       *slog.Logger
	trigger      chan struct{} // buffered size 1
	mu           sync.Mutex
	simulatedNow *time.Time // non-nil when simulator has set an explicit time
}

// NewScheduler creates a Scheduler with the provided dependencies.
func NewScheduler(
	trendRepo repository.TrendRepository,
	signalRepo repository.SignalRepository,
	statsRepo repository.StatsRepository,
	registry *calculator.Registry,
	cfg Config,
) *Scheduler {
	return &Scheduler{
		trendRepo:  trendRepo,
		signalRepo: signalRepo,
		statsRepo:  statsRepo,
		registry:   registry,
		cfg:        cfg,
		logger:     slog.Default(),
		trigger:    make(chan struct{}, 1),
	}
}

// getCurrentTime returns the simulated time if set, otherwise time.Now().UTC().
func (s *Scheduler) getCurrentTime() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.simulatedNow != nil {
		return *s.simulatedNow
	}
	return time.Now().UTC()
}

// TriggerNow signals the scheduler to run a calculation round immediately.
// If asOf is non-zero, it becomes the "current time" for the triggered run
// (used by simulator to handle historical seed data).
// Non-blocking: if a trigger is already pending, this is a no-op.
func (s *Scheduler) TriggerNow(asOf time.Time) {
	if !asOf.IsZero() {
		s.mu.Lock()
		t := asOf.UTC()
		s.simulatedNow = &t
		s.mu.Unlock()
	}
	select {
	case s.trigger <- struct{}{}:
	default:
	}
}

// Run starts the scheduling loop. It runs RunOnce immediately, then on each
// tick of cfg.Interval. It returns ctx.Err() when ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	if err := s.RunOnce(ctx); err != nil {
		s.logger.Error("scheduler run failed", "error", err)
	}

	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Reset to real time for scheduled runs.
			s.mu.Lock()
			s.simulatedNow = nil
			s.mu.Unlock()
			if err := s.RunOnce(ctx); err != nil {
				s.logger.Error("scheduler run failed", "error", err)
			}
		case <-s.trigger:
			if err := s.RunOnce(ctx); err != nil {
				s.logger.Error("scheduler triggered run failed", "error", err)
			}
		}
	}
}

// RunOnce executes one full round of computation across all strategies and trends.
// It is exported so tests can invoke it directly.
func (s *Scheduler) RunOnce(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// 1. Page through all trends.
	const batchSize = 100
	var allTrends []*domain.Trend
	offset := 0
	for {
		trends, _, err := s.trendRepo.List(ctx, offset, batchSize)
		if err != nil {
			return err
		}
		allTrends = append(allTrends, trends...)
		if len(trends) < batchSize {
			break
		}
		offset += batchSize
	}

	strategies := s.registry.All()
	if len(strategies) == 0 || len(allTrends) == 0 {
		return nil
	}

	// 2. Run each strategy in parallel.
	var wg sync.WaitGroup
	for _, strat := range strategies {
		wg.Add(1)
		go func(st calculator.Strategy) {
			defer wg.Done()
			s.runStrategy(ctx, st, allTrends)
		}(strat)
	}
	wg.Wait()
	return nil
}

func (s *Scheduler) runStrategy(ctx context.Context, strat calculator.Strategy, trends []*domain.Trend) {
	now := s.getCurrentTime()
	var statsToWrite []*domain.TrendStats
	for _, trend := range trends {
		if ctx.Err() != nil {
			return
		}
		reader := NewSignalReader(trend.ID, s.signalRepo, s.cfg.SignalLookback, now)
		stats, err := strat.Calculate(ctx, trend, reader)
		if err != nil {
			s.logger.Error("strategy calculate failed",
				"strategy", strat.ID(),
				"trend_id", trend.ID,
				"error", err)
			continue
		}
		statsToWrite = append(statsToWrite, stats)
	}

	if len(statsToWrite) == 0 {
		return
	}
	if err := s.statsRepo.BatchUpsert(ctx, statsToWrite); err != nil {
		s.logger.Error("batch upsert failed",
			"strategy", strat.ID(),
			"error", err)
	}
}
