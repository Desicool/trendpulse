package scheduler_test

import (
	"context"
	"testing"
	"time"

	"trendpulse/internal/domain"
	"trendpulse/internal/scheduler"
)

// --- mock SignalRepository for signal_reader tests ---

type mockSignalRepo struct {
	signals []*domain.Signal
}

func (m *mockSignalRepo) Insert(_ context.Context, _ *domain.Signal) error {
	return nil
}

func (m *mockSignalRepo) ListByTrendID(_ context.Context, trendID string, from, to time.Time) ([]*domain.Signal, error) {
	var out []*domain.Signal
	for _, sig := range m.signals {
		if sig.TrendID == trendID && !sig.Timestamp.Before(from) && !sig.Timestamp.After(to) {
			out = append(out, sig)
		}
	}
	return out, nil
}

func (m *mockSignalRepo) GetLatestByTrendID(_ context.Context, trendID string) (*domain.Signal, error) {
	var latest *domain.Signal
	for _, sig := range m.signals {
		if sig.TrendID == trendID {
			if latest == nil || sig.Timestamp.After(latest.Timestamp) {
				latest = sig
			}
		}
	}
	return latest, nil
}

func (m *mockSignalRepo) ListByTimeRange(_ context.Context, from, to time.Time) ([]*domain.Signal, error) {
	var out []*domain.Signal
	for _, sig := range m.signals {
		if !sig.Timestamp.Before(from) && !sig.Timestamp.After(to) {
			out = append(out, sig)
		}
	}
	return out, nil
}

// makeSignals creates n signals for a trend, spaced 1h apart starting `lookback` ago.
func makeSignals(trendID string, count int, lookback time.Duration) []*domain.Signal {
	signals := make([]*domain.Signal, count)
	now := time.Now().UTC()
	interval := lookback / time.Duration(count)
	for i := 0; i < count; i++ {
		signals[i] = &domain.Signal{
			ID:             trendID + "-" + string(rune('a'+i)),
			TrendID:        trendID,
			Timestamp:      now.Add(-lookback + time.Duration(i)*interval),
			UsageCount:     int64(100 * (i + 1)),
			UniqueCreators: int64(10 * (i + 1)),
			AvgViews:       float64(500 * (i + 1)),
		}
	}
	return signals
}

func TestSignalReader_Latest_ReturnsNewestN(t *testing.T) {
	signals := makeSignals("trend-001", 10, 48*time.Hour)
	repo := &mockSignalRepo{signals: signals}
	reader := scheduler.NewSignalReader("trend-001", repo, 48*time.Hour)

	got, err := reader.Latest(context.Background(), 3)
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("Latest(3) returned %d signals, want 3", len(got))
	}
	// Should be ascending by timestamp (newest last)
	for i := 1; i < len(got); i++ {
		if got[i].Timestamp.Before(got[i-1].Timestamp) {
			t.Errorf("signals not ascending at index %d", i)
		}
	}
	// Last one should be the most recent
	if got[2].ID != signals[9].ID {
		t.Errorf("Latest(3) last signal ID = %q, want %q", got[2].ID, signals[9].ID)
	}
}

func TestSignalReader_Latest_FewerThanN_ReturnsAll(t *testing.T) {
	// Use a shorter window than lookback so all signals are within range.
	signals := makeSignals("trend-001", 2, 24*time.Hour)
	repo := &mockSignalRepo{signals: signals}
	reader := scheduler.NewSignalReader("trend-001", repo, 48*time.Hour)

	got, err := reader.Latest(context.Background(), 10)
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if len(got) != 2 {
		t.Errorf("Latest(10) with 2 signals returned %d, want 2", len(got))
	}
}

func TestSignalReader_Range_FiltersCorrectly(t *testing.T) {
	signals := makeSignals("trend-001", 10, 48*time.Hour)
	repo := &mockSignalRepo{signals: signals}
	reader := scheduler.NewSignalReader("trend-001", repo, 48*time.Hour)

	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now

	got, err := reader.Range(context.Background(), from, to)
	if err != nil {
		t.Fatalf("Range() error = %v", err)
	}
	// All returned signals should be within [from, to]
	for _, sig := range got {
		if sig.Timestamp.Before(from) || sig.Timestamp.After(to) {
			t.Errorf("signal %s timestamp %v out of range [%v, %v]", sig.ID, sig.Timestamp, from, to)
		}
	}
}

func TestSignalReader_Aggregate_GroupsIntoWindows(t *testing.T) {
	signals := makeSignals("trend-001", 12, 12*time.Hour)
	repo := &mockSignalRepo{signals: signals}
	reader := scheduler.NewSignalReader("trend-001", repo, 12*time.Hour)

	got, err := reader.Aggregate(context.Background(), 3*time.Hour)
	if err != nil {
		t.Fatalf("Aggregate() error = %v", err)
	}
	// Should produce windows
	if len(got) == 0 {
		t.Fatal("Aggregate() returned 0 windows, want > 0")
	}
	// Each window should have SampleCount > 0
	for i, w := range got {
		if w.SampleCount == 0 {
			t.Errorf("window[%d] SampleCount = 0", i)
		}
		if w.WindowEnd.Before(w.WindowStart) {
			t.Errorf("window[%d] end before start", i)
		}
	}
}

func TestSignalReader_Aggregate_ComputesAverages(t *testing.T) {
	// Two signals with known values
	now := time.Now().UTC()
	signals := []*domain.Signal{
		{
			ID:          "s1",
			TrendID:     "trend-001",
			Timestamp:   now.Add(-2 * time.Hour),
			UsageCount:  100,
			AvgViews:    200.0,
			AvgEngagement: 10.0,
		},
		{
			ID:          "s2",
			TrendID:     "trend-001",
			Timestamp:   now.Add(-1 * time.Hour),
			UsageCount:  200,
			AvgViews:    400.0,
			AvgEngagement: 30.0,
		},
	}
	repo := &mockSignalRepo{signals: signals}
	reader := scheduler.NewSignalReader("trend-001", repo, 6*time.Hour)

	got, err := reader.Aggregate(context.Background(), 6*time.Hour)
	if err != nil {
		t.Fatalf("Aggregate() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Aggregate() returned %d windows, want 1", len(got))
	}
	w := got[0]
	wantAvgUsage := 150.0
	if w.AvgUsageCount != wantAvgUsage {
		t.Errorf("AvgUsageCount = %v, want %v", w.AvgUsageCount, wantAvgUsage)
	}
	wantAvgViews := 300.0
	if w.AvgViews != wantAvgViews {
		t.Errorf("AvgViews = %v, want %v", w.AvgViews, wantAvgViews)
	}
	wantAvgEng := 20.0
	if w.AvgEngagement != wantAvgEng {
		t.Errorf("AvgEngagement = %v, want %v", w.AvgEngagement, wantAvgEng)
	}
	if w.SampleCount != 2 {
		t.Errorf("SampleCount = %d, want 2", w.SampleCount)
	}
}

func TestSignalReader_Aggregate_EmptySignals_ReturnsEmpty(t *testing.T) {
	repo := &mockSignalRepo{signals: nil}
	reader := scheduler.NewSignalReader("trend-001", repo, 48*time.Hour)

	got, err := reader.Aggregate(context.Background(), time.Hour)
	if err != nil {
		t.Fatalf("Aggregate() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Aggregate() on empty signals returned %d windows, want 0", len(got))
	}
}
