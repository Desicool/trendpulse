package badger

import (
	"context"
	"testing"
	"time"

	"trendpulse/internal/domain"
)

func makeStat(strategyID, trendID string, score float64, phase string) *domain.TrendStats {
	return &domain.TrendStats{
		ID:           strategyID + ":" + trendID,
		TrendID:      trendID,
		StrategyID:   strategyID,
		CalculatedAt: time.Now().UTC(),
		Score:        score,
		Phase:        phase,
		Confidence:   0.8,
	}
}

// --- Upsert ---

func TestStatsRepo_Upsert_InsertAndOverwrite(t *testing.T) {
	store := newTestStore(t)
	repo := NewStatsRepository(store)

	stat := makeStat("momentum_v1", "trend-1", 75.0, "emerging")
	if err := repo.Upsert(context.Background(), stat); err != nil {
		t.Fatalf("unexpected error on first upsert: %v", err)
	}

	// Overwrite with new score
	stat.Score = 90.0
	if err := repo.Upsert(context.Background(), stat); err != nil {
		t.Fatalf("unexpected error on second upsert: %v", err)
	}

	got, err := repo.GetByTrendID(context.Background(), "trend-1", "momentum_v1")
	if err != nil {
		t.Fatalf("unexpected error on get: %v", err)
	}
	if got.Score != 90.0 {
		t.Errorf("expected score=90, got %f", got.Score)
	}
}

// --- BatchUpsert ---

func TestStatsRepo_BatchUpsert_Success(t *testing.T) {
	store := newTestStore(t)
	repo := NewStatsRepository(store)

	stats := []*domain.TrendStats{
		makeStat("strategy_a", "t1", 60.0, "emerging"),
		makeStat("strategy_a", "t2", 80.0, "rising"),
		makeStat("strategy_a", "t3", 40.0, "declining"),
	}

	if err := repo.BatchUpsert(context.Background(), stats); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	all, err := repo.ListByStrategyID(context.Background(), "strategy_a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3, got %d", len(all))
	}
}

// --- GetByTrendID ---

func TestStatsRepo_GetByTrendID_NotFound(t *testing.T) {
	store := newTestStore(t)
	repo := NewStatsRepository(store)

	_, err := repo.GetByTrendID(context.Background(), "nonexistent", "any_strategy")
	if err == nil {
		t.Fatal("expected not-found error, got nil")
	}
}

// --- ListRising ---

func TestStatsRepo_ListRising_OrderedByScoreDesc(t *testing.T) {
	store := newTestStore(t)
	repo := NewStatsRepository(store)

	scores := []float64{20.0, 90.0, 50.0, 75.0}
	for i, score := range scores {
		id := string(rune('a' + i))
		_ = repo.Upsert(context.Background(), makeStat("strat1", "t"+id, score, "emerging"))
	}

	results, err := repo.ListRising(context.Background(), "strat1", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	// Should be ordered descending
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("not descending at index %d: %f > %f", i, results[i].Score, results[i-1].Score)
		}
	}
}

// --- ListByStrategyID ---

func TestStatsRepo_ListByStrategyID_FiltersByStrategy(t *testing.T) {
	store := newTestStore(t)
	repo := NewStatsRepository(store)

	_ = repo.Upsert(context.Background(), makeStat("strat_x", "t1", 60.0, "emerging"))
	_ = repo.Upsert(context.Background(), makeStat("strat_x", "t2", 80.0, "rising"))
	_ = repo.Upsert(context.Background(), makeStat("strat_y", "t1", 40.0, "declining"))

	results, err := repo.ListByStrategyID(context.Background(), "strat_x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}
