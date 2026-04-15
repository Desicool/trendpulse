package badger

import (
	"context"
	"testing"
	"time"

	"trendpulse/internal/domain"
)

func makeSignal(id, trendID string, ts time.Time) *domain.Signal {
	return &domain.Signal{
		ID:                id,
		TrendID:           trendID,
		Timestamp:         ts,
		UsageCount:        100,
		UniqueCreators:    10,
		AvgViews:          500.0,
		AvgEngagement:     25.0,
		ViewConcentration: 0.1,
		CreatedAt:         time.Now().UTC(),
	}
}

// --- Insert ---

func TestSignalRepo_Insert_Success(t *testing.T) {
	store := newTestStore(t)
	repo := NewSignalRepository(store)
	sig := makeSignal("s1", "trend-1", time.Now().UTC())

	if err := repo.Insert(context.Background(), sig); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ListByTrendID ---

func TestSignalRepo_ListByTrendID_ReturnsInTimeRange(t *testing.T) {
	store := newTestStore(t)
	repo := NewSignalRepository(store)
	base := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)

	for i := 0; i < 5; i++ {
		ts := base.Add(time.Duration(i) * time.Hour)
		_ = repo.Insert(context.Background(), makeSignal("s"+string(rune('0'+i)), "trend-1", ts))
	}
	// Signal for different trend — should not appear
	_ = repo.Insert(context.Background(), makeSignal("other", "trend-2", base.Add(time.Hour)))

	from := base
	to := base.Add(3 * time.Hour)
	results, err := repo.ListByTrendID(context.Background(), "trend-1", from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// timestamps at base+0h, base+1h, base+2h, base+3h all fall in [from, to]
	if len(results) != 4 {
		t.Errorf("expected 4 signals, got %d", len(results))
	}
}

func TestSignalRepo_ListByTrendID_AscendingOrder(t *testing.T) {
	store := newTestStore(t)
	repo := NewSignalRepository(store)
	base := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)

	for i := 4; i >= 0; i-- {
		ts := base.Add(time.Duration(i) * time.Hour)
		_ = repo.Insert(context.Background(), makeSignal("s"+string(rune('0'+i)), "t1", ts))
	}

	from := base
	to := base.Add(10 * time.Hour)
	results, err := repo.ListByTrendID(context.Background(), "t1", from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 1; i < len(results); i++ {
		if results[i].Timestamp.Before(results[i-1].Timestamp) {
			t.Errorf("signals not in ascending order at index %d", i)
		}
	}
}

// --- GetLatestByTrendID ---

func TestSignalRepo_GetLatestByTrendID_Success(t *testing.T) {
	store := newTestStore(t)
	repo := NewSignalRepository(store)
	base := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		ts := base.Add(time.Duration(i) * time.Hour)
		_ = repo.Insert(context.Background(), makeSignal("s"+string(rune('0'+i)), "t1", ts))
	}

	latest, err := repo.GetLatestByTrendID(context.Background(), "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := base.Add(2 * time.Hour)
	if !latest.Timestamp.Equal(expected) {
		t.Errorf("expected timestamp %v, got %v", expected, latest.Timestamp)
	}
}

func TestSignalRepo_GetLatestByTrendID_NotFound(t *testing.T) {
	store := newTestStore(t)
	repo := NewSignalRepository(store)

	_, err := repo.GetLatestByTrendID(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected not-found error, got nil")
	}
}

// --- ListByTimeRange ---

func TestSignalRepo_ListByTimeRange_CrossTrend(t *testing.T) {
	store := newTestStore(t)
	repo := NewSignalRepository(store)
	base := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)

	_ = repo.Insert(context.Background(), makeSignal("a", "trend-1", base))
	_ = repo.Insert(context.Background(), makeSignal("b", "trend-2", base.Add(time.Hour)))
	_ = repo.Insert(context.Background(), makeSignal("c", "trend-3", base.Add(5*time.Hour))) // outside range

	results, err := repo.ListByTimeRange(context.Background(), base, base.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}
