package repository

import (
	"context"
	"time"

	"trendpulse/internal/domain"
)

// TrendRepository manages persistence for Trend entities.
type TrendRepository interface {
	Insert(ctx context.Context, trend *domain.Trend) error
	GetByID(ctx context.Context, id string) (*domain.Trend, error)
	List(ctx context.Context, offset, limit int) ([]*domain.Trend, int, error)
	Update(ctx context.Context, trend *domain.Trend) error
	ListByIDs(ctx context.Context, ids []string) ([]*domain.Trend, error)
}

// SignalRepository manages persistence for Signal entities.
type SignalRepository interface {
	Insert(ctx context.Context, signal *domain.Signal) error
	ListByTrendID(ctx context.Context, trendID string, from, to time.Time) ([]*domain.Signal, error)
	GetLatestByTrendID(ctx context.Context, trendID string) (*domain.Signal, error)
	ListByTimeRange(ctx context.Context, from, to time.Time) ([]*domain.Signal, error)
}

// StatsRepository manages persistence for TrendStats entities.
type StatsRepository interface {
	Upsert(ctx context.Context, stat *domain.TrendStats) error
	BatchUpsert(ctx context.Context, stats []*domain.TrendStats) error
	GetByTrendID(ctx context.Context, trendID string, strategyID string) (*domain.TrendStats, error)
	ListRising(ctx context.Context, strategyID string, limit int) ([]*domain.TrendStats, error)
	ListByStrategyID(ctx context.Context, strategyID string) ([]*domain.TrendStats, error)
}

// CategoryIndex manages the category→trendID mapping index.
type CategoryIndex interface {
	SetCategories(ctx context.Context, trendID string, categories []string) error
	GetTrendIDsByCategory(ctx context.Context, category string) ([]string, error)
}
