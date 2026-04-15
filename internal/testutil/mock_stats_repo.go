package testutil

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"trendpulse/internal/domain"
)

// MockStatsRepo is a hand-written in-memory mock of repository.StatsRepository.
// It is safe for concurrent use.
type MockStatsRepo struct {
	mu                   sync.RWMutex
	stats                map[string]*domain.TrendStats
	batchUpsertCallCount int
	upsertCallCount      int
}

// NewMockStatsRepo returns a ready-to-use MockStatsRepo.
func NewMockStatsRepo() *MockStatsRepo {
	return &MockStatsRepo{
		stats: make(map[string]*domain.TrendStats),
	}
}

func (m *MockStatsRepo) Upsert(ctx context.Context, stat *domain.TrendStats) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats[stat.ID] = stat
	m.upsertCallCount++
	return nil
}

func (m *MockStatsRepo) BatchUpsert(ctx context.Context, stats []*domain.TrendStats) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range stats {
		m.stats[s.ID] = s
	}
	m.batchUpsertCallCount++
	return nil
}

func (m *MockStatsRepo) GetByTrendID(ctx context.Context, trendID string, strategyID string) (*domain.TrendStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id := fmt.Sprintf("%s:%s", strategyID, trendID)
	s, ok := m.stats[id]
	if !ok {
		return nil, fmt.Errorf("stats not found for trend %q strategy %q", trendID, strategyID)
	}
	return s, nil
}

func (m *MockStatsRepo) ListRising(ctx context.Context, strategyID string, limit int) ([]*domain.TrendStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.TrendStats
	for _, s := range m.stats {
		if s.StrategyID == strategyID {
			result = append(result, s)
		}
	}
	// Sort by Score descending.
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *MockStatsRepo) ListByStrategyID(ctx context.Context, strategyID string) ([]*domain.TrendStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.TrendStats
	for _, s := range m.stats {
		if s.StrategyID == strategyID {
			result = append(result, s)
		}
	}
	return result, nil
}

// --- Call count accessors ---

func (m *MockStatsRepo) BatchUpsertCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.batchUpsertCallCount
}

func (m *MockStatsRepo) UpsertCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.upsertCallCount
}
