package calculator

import (
	"context"
	"time"

	"trendpulse/internal/domain"
)

// AggregatedSignal is a time-window aggregation of raw signals.
type AggregatedSignal struct {
	WindowStart          time.Time
	WindowEnd            time.Time
	AvgUsageCount        float64
	AvgUniqueCreators    float64
	AvgViews             float64
	AvgEngagement        float64
	AvgViewConcentration float64
	SampleCount          int // number of raw signals in this window
}

// SignalReader provides lazy signal access bound to a specific trend.
type SignalReader interface {
	// Latest returns the most recent n signals, ascending by Timestamp (newest last).
	Latest(ctx context.Context, n int) ([]*domain.Signal, error)
	// Range returns all signals in [from, to], ascending by Timestamp.
	Range(ctx context.Context, from, to time.Time) ([]*domain.Signal, error)
	// Aggregate groups signals into time windows of `window` duration and
	// returns per-window averages over the full lookback period.
	Aggregate(ctx context.Context, window time.Duration) ([]*AggregatedSignal, error)
}

// Strategy defines the unified interface for all prediction strategies.
// Every implementation must be concurrency-safe (goroutine-safe).
type Strategy interface {
	// ID returns the strategy's unique identifier (e.g. "sigmoid_v1").
	ID() string
	// Name returns a human-readable display name.
	Name() string
	// Calculate computes Score (burst probability) and Phase (lifecycle stage)
	// from the signals provided via reader.
	//
	// When no signals are available, Calculate must return a low-confidence
	// TrendStats (Score=0, Confidence=0, Phase="emerging") and a nil error.
	Calculate(ctx context.Context, trend *domain.Trend, reader SignalReader) (*domain.TrendStats, error)
}
