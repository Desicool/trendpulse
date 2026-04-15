package badger

import (
	"context"
	"fmt"
	"sort"

	badgerhold "github.com/timshannon/badgerhold/v4"
	"trendpulse/internal/domain"
)

// StatsRepository is the BadgerHold implementation of repository.StatsRepository.
type StatsRepository struct {
	store *badgerhold.Store
}

// NewStatsRepository creates a new StatsRepository backed by store.
func NewStatsRepository(store *badgerhold.Store) *StatsRepository {
	return &StatsRepository{store: store}
}

// Upsert inserts or overwrites a TrendStats record identified by stat.ID.
func (r *StatsRepository) Upsert(_ context.Context, stat *domain.TrendStats) error {
	return r.store.Upsert(stat.ID, stat)
}

// BatchUpsert calls Upsert for each stat.
func (r *StatsRepository) BatchUpsert(ctx context.Context, stats []*domain.TrendStats) error {
	for _, s := range stats {
		if err := r.Upsert(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

// GetByTrendID retrieves a TrendStats by the composite key "{strategyID}:{trendID}".
func (r *StatsRepository) GetByTrendID(_ context.Context, trendID, strategyID string) (*domain.TrendStats, error) {
	id := strategyID + ":" + trendID
	var s domain.TrendStats
	if err := r.store.Get(id, &s); err != nil {
		if err == badgerhold.ErrNotFound {
			return nil, fmt.Errorf("stats %q: %w", id, ErrNotFound)
		}
		return nil, err
	}
	return &s, nil
}

// ListRising returns the top-limit TrendStats for strategyID ordered by Score
// descending.
func (r *StatsRepository) ListRising(_ context.Context, strategyID string, limit int) ([]*domain.TrendStats, error) {
	var results []domain.TrendStats
	query := badgerhold.Where("StrategyID").Eq(strategyID)
	if err := r.store.Find(&results, query); err != nil {
		return nil, err
	}
	// Sort descending by Score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}
	out := make([]*domain.TrendStats, len(results))
	for i := range results {
		out[i] = &results[i]
	}
	return out, nil
}

// ListByStrategyID returns all TrendStats computed by the given strategy.
func (r *StatsRepository) ListByStrategyID(_ context.Context, strategyID string) ([]*domain.TrendStats, error) {
	var results []domain.TrendStats
	query := badgerhold.Where("StrategyID").Eq(strategyID)
	if err := r.store.Find(&results, query); err != nil {
		return nil, err
	}
	out := make([]*domain.TrendStats, len(results))
	for i := range results {
		out[i] = &results[i]
	}
	return out, nil
}
