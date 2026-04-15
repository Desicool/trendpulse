package scheduler_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"trendpulse/internal/calculator"
	"trendpulse/internal/domain"
	"trendpulse/internal/scheduler"
)

// --- mock TrendRepository ---

type mockTrendRepo struct {
	mu     sync.RWMutex
	trends []*domain.Trend
}

func newMockTrendRepo(trends []*domain.Trend) *mockTrendRepo {
	return &mockTrendRepo{trends: trends}
}

func (m *mockTrendRepo) Insert(_ context.Context, t *domain.Trend) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trends = append(m.trends, t)
	return nil
}

func (m *mockTrendRepo) GetByID(_ context.Context, id string) (*domain.Trend, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, t := range m.trends {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("trend %q not found", id)
}

func (m *mockTrendRepo) List(_ context.Context, offset, limit int) ([]*domain.Trend, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	total := len(m.trends)
	if offset >= total {
		return nil, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return m.trends[offset:end], total, nil
}

func (m *mockTrendRepo) Update(_ context.Context, t *domain.Trend) error { return nil }

func (m *mockTrendRepo) ListByIDs(_ context.Context, ids []string) ([]*domain.Trend, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*domain.Trend
	for _, t := range m.trends {
		for _, id := range ids {
			if t.ID == id {
				out = append(out, t)
			}
		}
	}
	return out, nil
}

// --- mock StatsRepository ---

type mockStatsRepo struct {
	mu               sync.Mutex
	batchUpsertCalls int
	upsertCalls      int
	stored           []*domain.TrendStats
}

func (m *mockStatsRepo) Upsert(_ context.Context, stat *domain.TrendStats) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertCalls++
	m.stored = append(m.stored, stat)
	return nil
}

func (m *mockStatsRepo) BatchUpsert(_ context.Context, stats []*domain.TrendStats) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batchUpsertCalls++
	m.stored = append(m.stored, stats...)
	return nil
}

func (m *mockStatsRepo) GetByTrendID(_ context.Context, trendID, strategyID string) (*domain.TrendStats, error) {
	return nil, nil
}

func (m *mockStatsRepo) ListRising(_ context.Context, strategyID string, limit int) ([]*domain.TrendStats, error) {
	return nil, nil
}

func (m *mockStatsRepo) ListByStrategyID(_ context.Context, strategyID string) ([]*domain.TrendStats, error) {
	return nil, nil
}

func (m *mockStatsRepo) BatchUpsertCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.batchUpsertCalls
}

// --- mock Strategy ---

type mockStrategy struct {
	id             string
	mu             sync.Mutex
	calculateCalls int
	ReturnError    error
}

func newMockStrategy(id string) *mockStrategy {
	return &mockStrategy{id: id}
}

func (s *mockStrategy) ID() string   { return s.id }
func (s *mockStrategy) Name() string { return "Mock Strategy " + s.id }

func (s *mockStrategy) Calculate(_ context.Context, trend *domain.Trend, _ calculator.SignalReader) (*domain.TrendStats, error) {
	s.mu.Lock()
	s.calculateCalls++
	s.mu.Unlock()
	if s.ReturnError != nil {
		return nil, s.ReturnError
	}
	return &domain.TrendStats{
		ID:           fmt.Sprintf("%s:%s", s.id, trend.ID),
		TrendID:      trend.ID,
		StrategyID:   s.id,
		CalculatedAt: time.Now().UTC(),
		Score:        50.0,
		Phase:        "emerging",
		Confidence:   0.7,
	}, nil
}

func (s *mockStrategy) CalculateCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calculateCalls
}

// --- helpers ---

func newTestTrend(id string) *domain.Trend {
	return &domain.Trend{
		ID:        id,
		Name:      "Trend " + id,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
}

func newTestConfig() scheduler.Config {
	return scheduler.Config{
		Interval:       time.Minute,
		SignalLookback: 48 * time.Hour,
	}
}

// --- scheduler tests ---

func TestScheduler_RunOnce_CallsAllStrategies(t *testing.T) {
	trends := []*domain.Trend{
		newTestTrend("trend-001"),
		newTestTrend("trend-002"),
	}
	trendRepo := newMockTrendRepo(trends)
	signalRepo := &mockSignalRepo{}
	statsRepo := &mockStatsRepo{}

	stratA := newMockStrategy("strategy_a")
	stratB := newMockStrategy("strategy_b")

	reg := calculator.NewRegistry()
	_ = reg.Register(stratA)
	_ = reg.Register(stratB)

	sched := scheduler.NewScheduler(trendRepo, signalRepo, statsRepo, reg, newTestConfig())

	if err := sched.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	if stratA.CalculateCallCount() != 2 {
		t.Errorf("strategy_a.Calculate called %d times, want 2", stratA.CalculateCallCount())
	}
	if stratB.CalculateCallCount() != 2 {
		t.Errorf("strategy_b.Calculate called %d times, want 2", stratB.CalculateCallCount())
	}
	// BatchUpsert called once per strategy
	if statsRepo.BatchUpsertCallCount() != 2 {
		t.Errorf("BatchUpsert called %d times, want 2", statsRepo.BatchUpsertCallCount())
	}
}

func TestScheduler_RunOnce_StrategyError_ContinuesOtherStrategies(t *testing.T) {
	trends := []*domain.Trend{newTestTrend("trend-001")}
	trendRepo := newMockTrendRepo(trends)
	signalRepo := &mockSignalRepo{}
	statsRepo := &mockStatsRepo{}

	stratA := newMockStrategy("strategy_a")
	stratA.ReturnError = fmt.Errorf("compute error")
	stratB := newMockStrategy("strategy_b")

	reg := calculator.NewRegistry()
	_ = reg.Register(stratA)
	_ = reg.Register(stratB)

	sched := scheduler.NewScheduler(trendRepo, signalRepo, statsRepo, reg, newTestConfig())

	if err := sched.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() should not error when strategy fails, got: %v", err)
	}
	// strategy_b should still be called
	if stratB.CalculateCallCount() != 1 {
		t.Errorf("strategy_b.Calculate called %d times, want 1", stratB.CalculateCallCount())
	}
}

func TestScheduler_RunOnce_NoTrends_NoCalls(t *testing.T) {
	trendRepo := newMockTrendRepo(nil)
	signalRepo := &mockSignalRepo{}
	statsRepo := &mockStatsRepo{}

	stratA := newMockStrategy("strategy_a")
	reg := calculator.NewRegistry()
	_ = reg.Register(stratA)

	sched := scheduler.NewScheduler(trendRepo, signalRepo, statsRepo, reg, newTestConfig())

	if err := sched.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if stratA.CalculateCallCount() != 0 {
		t.Errorf("strategy_a.Calculate called %d times, want 0 with no trends", stratA.CalculateCallCount())
	}
}

func TestScheduler_RunOnce_NoStrategies_NoError(t *testing.T) {
	trends := []*domain.Trend{newTestTrend("trend-001")}
	trendRepo := newMockTrendRepo(trends)
	signalRepo := &mockSignalRepo{}
	statsRepo := &mockStatsRepo{}

	reg := calculator.NewRegistry()
	sched := scheduler.NewScheduler(trendRepo, signalRepo, statsRepo, reg, newTestConfig())

	if err := sched.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
}

func TestScheduler_RunOnce_ContextCancelled_StopsEarly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	trends := []*domain.Trend{newTestTrend("t1")}
	trendRepo := newMockTrendRepo(trends)
	signalRepo := &mockSignalRepo{}
	statsRepo := &mockStatsRepo{}
	reg := calculator.NewRegistry()

	sched := scheduler.NewScheduler(trendRepo, signalRepo, statsRepo, reg, newTestConfig())
	// Should not panic or hang on cancelled context.
	_ = sched.RunOnce(ctx)
}

// --- captureTimeStrategy records the "now" time passed via SignalReader ---

type captureTimeStrategy struct {
	id         string
	mu         sync.Mutex
	capturedAt time.Time
}

func newCaptureTimeStrategy(id string) *captureTimeStrategy {
	return &captureTimeStrategy{id: id}
}

func (s *captureTimeStrategy) ID() string   { return s.id }
func (s *captureTimeStrategy) Name() string { return "Capture Time Strategy " + s.id }

func (s *captureTimeStrategy) Calculate(ctx context.Context, trend *domain.Trend, reader calculator.SignalReader) (*domain.TrendStats, error) {
	// Call Latest so that the underlying repo's ListByTrendID is invoked,
	// allowing recordingSignalRepo to capture the to/from arguments.
	_, _ = reader.Latest(ctx, 1)
	return &domain.TrendStats{
		ID:           fmt.Sprintf("%s:%s", s.id, trend.ID),
		TrendID:      trend.ID,
		StrategyID:   s.id,
		CalculatedAt: time.Now().UTC(),
		Score:        50.0,
		Phase:        "emerging",
		Confidence:   0.5,
	}, nil
}

// recordingSignalRepo records the time range passed to ListByTrendID.
type recordingSignalRepo struct {
	mockSignalRepo
	mu      sync.Mutex
	lastTo  time.Time
	lastFrom time.Time
}

func (r *recordingSignalRepo) ListByTrendID(ctx context.Context, trendID string, from, to time.Time) ([]*domain.Signal, error) {
	r.mu.Lock()
	r.lastFrom = from
	r.lastTo = to
	r.mu.Unlock()
	return r.mockSignalRepo.ListByTrendID(ctx, trendID, from, to)
}

func (r *recordingSignalRepo) getLastRange() (from, to time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastFrom, r.lastTo
}

func TestScheduler_TriggerNow_UsesAsOfTime(t *testing.T) {
	trends := []*domain.Trend{newTestTrend("trend-001")}
	trendRepo := newMockTrendRepo(trends)
	recRepo := &recordingSignalRepo{}
	statsRepo := &mockStatsRepo{}

	reg := calculator.NewRegistry()
	_ = reg.Register(newCaptureTimeStrategy("strategy_a"))

	sched := scheduler.NewScheduler(trendRepo, recRepo, statsRepo, reg, newTestConfig())

	// Trigger with a historical time two days ago.
	historical := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	sched.TriggerNow(historical)

	// RunOnce should use the historical time as the anchor.
	if err := sched.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}

	_, to := recRepo.getLastRange()
	// The "to" bound used in ListByTrendID must equal the historical time (within 1s).
	diff := to.Sub(historical)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Errorf("ListByTrendID to = %v, want ~%v (diff=%v)", to, historical, diff)
	}
}

func TestScheduler_TriggerNow_ZeroAsOf_UsesRealTime(t *testing.T) {
	trends := []*domain.Trend{newTestTrend("trend-001")}
	trendRepo := newMockTrendRepo(trends)
	recRepo := &recordingSignalRepo{}
	statsRepo := &mockStatsRepo{}

	reg := calculator.NewRegistry()
	_ = reg.Register(newCaptureTimeStrategy("strategy_a"))

	sched := scheduler.NewScheduler(trendRepo, recRepo, statsRepo, reg, newTestConfig())

	before := time.Now().UTC()
	sched.TriggerNow(time.Time{}) // zero => use real time
	if err := sched.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	after := time.Now().UTC()

	_, to := recRepo.getLastRange()
	if to.Before(before) || to.After(after.Add(time.Second)) {
		t.Errorf("ListByTrendID to = %v, expected in range [%v, %v]", to, before, after)
	}
}

func TestScheduler_TriggerNow_NonBlocking_WhenAlreadyPending(t *testing.T) {
	trendRepo := newMockTrendRepo(nil)
	signalRepo := &mockSignalRepo{}
	statsRepo := &mockStatsRepo{}
	reg := calculator.NewRegistry()

	sched := scheduler.NewScheduler(trendRepo, signalRepo, statsRepo, reg, newTestConfig())

	historical := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	// Call TriggerNow twice without draining the channel; second call must not block.
	done := make(chan struct{})
	go func() {
		sched.TriggerNow(historical)
		sched.TriggerNow(historical) // second call is a no-op, must not block
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(time.Second):
		t.Fatal("TriggerNow blocked when trigger channel already full")
	}
}
