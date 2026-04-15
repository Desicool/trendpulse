package testutil

import (
	"trendpulse/internal/calculator"
	"trendpulse/internal/repository"
)

// Compile-time interface compliance checks.
var _ repository.TrendRepository = (*MockTrendRepo)(nil)
var _ repository.SignalRepository = (*MockSignalRepo)(nil)
var _ repository.StatsRepository = (*MockStatsRepo)(nil)
var _ repository.CategoryIndex = (*MockCategoryIndex)(nil)
var _ calculator.Strategy = (*MockStrategy)(nil)
var _ calculator.SignalReader = (*SliceSignalReader)(nil)
