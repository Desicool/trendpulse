package testutil

import (
	"context"
	"fmt"
	"sync"
	"time"

	"trendpulse/internal/calculator"
	"trendpulse/internal/domain"
)

// MockStrategy is a hand-written mock of calculator.Strategy.
// It is safe for concurrent use.
type MockStrategy struct {
	id             string
	mu             sync.Mutex
	calculateCalls int
	// ReturnError, if set, is returned by every call to Calculate.
	ReturnError error
}

// NewMockStrategy creates a MockStrategy with the given strategy ID.
func NewMockStrategy(id string) *MockStrategy {
	return &MockStrategy{id: id}
}

func (m *MockStrategy) ID() string   { return m.id }
func (m *MockStrategy) Name() string { return "Mock Strategy " + m.id }

// Calculate increments the call counter and returns a fixed TrendStats with Score=50.0,
// or ReturnError if it is set.
func (m *MockStrategy) Calculate(ctx context.Context, trend *domain.Trend, reader calculator.SignalReader) (*domain.TrendStats, error) {
	m.mu.Lock()
	m.calculateCalls++
	m.mu.Unlock()

	if m.ReturnError != nil {
		return nil, m.ReturnError
	}
	return &domain.TrendStats{
		ID:           fmt.Sprintf("%s:%s", m.id, trend.ID),
		TrendID:      trend.ID,
		StrategyID:   m.id,
		CalculatedAt: time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
		Score:        50.0,
		Phase:        "growing",
		Confidence:   0.7,
	}, nil
}

// CalculateCallCount returns the number of times Calculate has been called.
func (m *MockStrategy) CalculateCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calculateCalls
}

// SliceSignalReader is a simple SignalReader backed by a []*domain.Signal slice.
// Use it in tests to pass a signal list to a Strategy.
type SliceSignalReader struct {
	signals []*domain.Signal
}

// NewSliceSignalReader creates a SignalReader backed by the given slice.
func NewSliceSignalReader(signals []*domain.Signal) *SliceSignalReader {
	return &SliceSignalReader{signals: signals}
}

func (r *SliceSignalReader) Latest(ctx context.Context, n int) ([]*domain.Signal, error) {
	if n <= 0 || len(r.signals) == 0 {
		return nil, nil
	}
	start := len(r.signals) - n
	if start < 0 {
		start = 0
	}
	return r.signals[start:], nil
}

func (r *SliceSignalReader) Range(ctx context.Context, from, to time.Time) ([]*domain.Signal, error) {
	var result []*domain.Signal
	for _, s := range r.signals {
		if (s.Timestamp.Equal(from) || s.Timestamp.After(from)) &&
			(s.Timestamp.Equal(to) || s.Timestamp.Before(to)) {
			result = append(result, s)
		}
	}
	return result, nil
}

func (r *SliceSignalReader) Aggregate(ctx context.Context, window time.Duration) ([]*calculator.AggregatedSignal, error) {
	if len(r.signals) == 0 || window <= 0 {
		return nil, nil
	}
	first := r.signals[0].Timestamp
	last := r.signals[len(r.signals)-1].Timestamp

	var windows []*calculator.AggregatedSignal
	for start := first; !start.After(last); start = start.Add(window) {
		end := start.Add(window)
		var inWindow []*domain.Signal
		for _, s := range r.signals {
			if (s.Timestamp.Equal(start) || s.Timestamp.After(start)) && s.Timestamp.Before(end) {
				inWindow = append(inWindow, s)
			}
		}
		if len(inWindow) == 0 {
			continue
		}
		agg := &calculator.AggregatedSignal{
			WindowStart: start,
			WindowEnd:   end,
			SampleCount: len(inWindow),
		}
		for _, s := range inWindow {
			agg.AvgUsageCount += float64(s.UsageCount)
			agg.AvgUniqueCreators += float64(s.UniqueCreators)
			agg.AvgViews += s.AvgViews
			agg.AvgEngagement += s.AvgEngagement
			agg.AvgViewConcentration += s.ViewConcentration
		}
		n := float64(len(inWindow))
		agg.AvgUsageCount /= n
		agg.AvgUniqueCreators /= n
		agg.AvgViews /= n
		agg.AvgEngagement /= n
		agg.AvgViewConcentration /= n
		windows = append(windows, agg)
	}
	return windows, nil
}
