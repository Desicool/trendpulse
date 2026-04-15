package testutil

import (
	"fmt"
	"math"
	"time"

	"trendpulse/internal/domain"
)

// fixedDate is a deterministic base date used in all fixtures.
var fixedDate = time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC)

// NewTrend constructs a Trend object suitable for testing.
func NewTrend(id string) *domain.Trend {
	return &domain.Trend{
		ID:          id,
		Name:        "Test Trend " + id,
		Description: "A test trend",
		Categories:  []string{"test"},
		Source:      "test-source",
		CreatedAt:   fixedDate,
		UpdatedAt:   fixedDate,
	}
}

// NewSignal constructs a Signal object suitable for testing.
func NewSignal(trendID string, timestamp time.Time, usageCount int64) *domain.Signal {
	return &domain.Signal{
		ID:             "signal-" + trendID + "-" + timestamp.Format("150405"),
		TrendID:        trendID,
		Timestamp:      timestamp,
		UsageCount:     usageCount,
		UniqueCreators: usageCount / 10,
		AvgViews:       float64(usageCount) * 3.5,
		CreatedAt:      time.Now().UTC(),
	}
}

// NewTrendStats constructs a TrendStats object suitable for testing.
func NewTrendStats(strategyID, trendID string, score float64) *domain.TrendStats {
	return &domain.TrendStats{
		ID:           fmt.Sprintf("%s:%s", strategyID, trendID),
		TrendID:      trendID,
		StrategyID:   strategyID,
		CalculatedAt: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
		Score:        score,
		Confidence:   0.8,
		Phase:        "growing",
	}
}

// NewSignalSequence constructs a slice of Signals with a deterministic growth pattern.
// base is the initial UsageCount; growthRate is the per-step multiplier
// (1.0 = flat, 1.1 = 10% growth per step).
// Timestamps start at 2026-04-15 10:00 UTC and increment by one hour per step.
func NewSignalSequence(trendID string, count int, base int64, growthRate float64) []*domain.Signal {
	signals := make([]*domain.Signal, count)
	now := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
	for i := 0; i < count; i++ {
		multiplier := math.Pow(growthRate, float64(i))
		signals[i] = &domain.Signal{
			ID:             fmt.Sprintf("signal-%s-%d", trendID, i),
			TrendID:        trendID,
			Timestamp:      now.Add(time.Duration(i) * time.Hour),
			UsageCount:     int64(float64(base) * multiplier),
			UniqueCreators: int64(float64(base/10) * multiplier),
			AvgViews:       float64(base) * multiplier * 3.5,
		}
	}
	return signals
}
