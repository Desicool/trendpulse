package testutil

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"trendpulse/internal/domain"
)

// MockSignalRepo is a hand-written in-memory mock of repository.SignalRepository.
// It is safe for concurrent use.
type MockSignalRepo struct {
	mu      sync.RWMutex
	signals []*domain.Signal
}

// NewMockSignalRepo returns a ready-to-use MockSignalRepo.
func NewMockSignalRepo() *MockSignalRepo {
	return &MockSignalRepo{}
}

// AddSignal adds a signal to the mock store.
func (m *MockSignalRepo) AddSignal(s *domain.Signal) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.signals = append(m.signals, s)
}

func (m *MockSignalRepo) Insert(ctx context.Context, signal *domain.Signal) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.signals {
		if s.ID == signal.ID {
			return fmt.Errorf("signal %q already exists", signal.ID)
		}
	}
	m.signals = append(m.signals, signal)
	return nil
}

func (m *MockSignalRepo) ListByTrendID(ctx context.Context, trendID string, from, to time.Time) ([]*domain.Signal, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.Signal
	for _, s := range m.signals {
		if s.TrendID == trendID &&
			(s.Timestamp.Equal(from) || s.Timestamp.After(from)) &&
			(s.Timestamp.Equal(to) || s.Timestamp.Before(to)) {
			result = append(result, s)
		}
	}
	sortSignalsByTimestamp(result)
	return result, nil
}

func (m *MockSignalRepo) GetLatestByTrendID(ctx context.Context, trendID string) (*domain.Signal, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var latest *domain.Signal
	for _, s := range m.signals {
		if s.TrendID != trendID {
			continue
		}
		if latest == nil || s.Timestamp.After(latest.Timestamp) {
			latest = s
		}
	}
	if latest == nil {
		return nil, fmt.Errorf("no signals found for trend %q", trendID)
	}
	return latest, nil
}

func (m *MockSignalRepo) ListByTimeRange(ctx context.Context, from, to time.Time) ([]*domain.Signal, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*domain.Signal
	for _, s := range m.signals {
		if (s.Timestamp.Equal(from) || s.Timestamp.After(from)) &&
			(s.Timestamp.Equal(to) || s.Timestamp.Before(to)) {
			result = append(result, s)
		}
	}
	sortSignalsByTimestamp(result)
	return result, nil
}

func sortSignalsByTimestamp(signals []*domain.Signal) {
	sort.Slice(signals, func(i, j int) bool {
		return signals[i].Timestamp.Before(signals[j].Timestamp)
	})
}
