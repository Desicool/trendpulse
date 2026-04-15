package testutil

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"trendpulse/internal/domain"
)

// MockTrendRepo is a hand-written in-memory mock of repository.TrendRepository.
// It is safe for concurrent use.
type MockTrendRepo struct {
	mu          sync.RWMutex
	trends      map[string]*domain.Trend
	insertCalls int
	getCalls    int
	listCalls   int
	updateCalls int
}

// NewMockTrendRepo returns a ready-to-use MockTrendRepo.
func NewMockTrendRepo() *MockTrendRepo {
	return &MockTrendRepo{
		trends: make(map[string]*domain.Trend),
	}
}

// SetTrends pre-populates the mock with a set of trends.
func (m *MockTrendRepo) SetTrends(trends []*domain.Trend) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trends = make(map[string]*domain.Trend, len(trends))
	for _, t := range trends {
		m.trends[t.ID] = t
	}
}

func (m *MockTrendRepo) Insert(ctx context.Context, trend *domain.Trend) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.trends[trend.ID]; exists {
		return fmt.Errorf("trend %q already exists", trend.ID)
	}
	m.trends[trend.ID] = trend
	m.insertCalls++
	return nil
}

func (m *MockTrendRepo) GetByID(ctx context.Context, id string) (*domain.Trend, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.getCalls++
	t, ok := m.trends[id]
	if !ok {
		return nil, fmt.Errorf("trend %q not found", id)
	}
	return t, nil
}

func (m *MockTrendRepo) List(ctx context.Context, offset, limit int) ([]*domain.Trend, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.listCalls++

	all := make([]*domain.Trend, 0, len(m.trends))
	for _, t := range m.trends {
		all = append(all, t)
	}
	// Sort by ID for determinism.
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })

	total := len(all)
	if offset >= total {
		return []*domain.Trend{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}

func (m *MockTrendRepo) Update(ctx context.Context, trend *domain.Trend) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.trends[trend.ID]; !exists {
		return fmt.Errorf("trend %q not found", trend.ID)
	}
	m.trends[trend.ID] = trend
	m.updateCalls++
	return nil
}

func (m *MockTrendRepo) ListByIDs(ctx context.Context, ids []string) ([]*domain.Trend, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*domain.Trend, 0, len(ids))
	for _, id := range ids {
		if t, ok := m.trends[id]; ok {
			result = append(result, t)
		}
	}
	return result, nil
}

// --- Call count accessors ---

func (m *MockTrendRepo) InsertCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.insertCalls
}

func (m *MockTrendRepo) GetCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getCalls
}

func (m *MockTrendRepo) ListCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.listCalls
}

func (m *MockTrendRepo) UpdateCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.updateCalls
}
