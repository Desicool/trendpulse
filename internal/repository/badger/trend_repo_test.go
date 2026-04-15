package badger

import (
	"context"
	"testing"
	"time"

	badgerhold "github.com/timshannon/badgerhold/v4"
	"trendpulse/internal/domain"
)

// newTestStore creates an in-memory BadgerHold store for testing.
// When InMemory is true, Dir and ValueDir must be empty strings.
func newTestStore(t *testing.T) *badgerhold.Store {
	t.Helper()
	opts := badgerhold.DefaultOptions
	opts.InMemory = true
	opts.Dir = ""
	opts.ValueDir = ""
	store, err := badgerhold.Open(opts)
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func makeTrend(id, name, source string) *domain.Trend {
	now := time.Now().UTC()
	return &domain.Trend{
		ID:          id,
		Name:        name,
		Description: "desc",
		Categories:  []string{"cat1"},
		Source:      source,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// --- Insert ---

func TestTrendRepo_Insert_Success(t *testing.T) {
	store := newTestStore(t)
	repo := NewTrendRepository(store)
	trend := makeTrend("t1", "Trend One", "tiktok")

	if err := repo.Insert(context.Background(), trend); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTrendRepo_Insert_DuplicateReturnsError(t *testing.T) {
	store := newTestStore(t)
	repo := NewTrendRepository(store)
	trend := makeTrend("t1", "Trend One", "tiktok")

	_ = repo.Insert(context.Background(), trend)
	err := repo.Insert(context.Background(), trend)
	if err == nil {
		t.Fatal("expected error for duplicate insert, got nil")
	}
}

// --- GetByID ---

func TestTrendRepo_GetByID_Success(t *testing.T) {
	store := newTestStore(t)
	repo := NewTrendRepository(store)
	trend := makeTrend("t1", "Trend One", "tiktok")
	_ = repo.Insert(context.Background(), trend)

	got, err := repo.GetByID(context.Background(), "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "t1" || got.Name != "Trend One" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestTrendRepo_GetByID_NotFound(t *testing.T) {
	store := newTestStore(t)
	repo := NewTrendRepository(store)

	_, err := repo.GetByID(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected not-found error, got nil")
	}
}

// --- List ---

func TestTrendRepo_List_Pagination(t *testing.T) {
	store := newTestStore(t)
	repo := NewTrendRepository(store)
	for i := 0; i < 5; i++ {
		id := string(rune('a' + i))
		_ = repo.Insert(context.Background(), makeTrend(id, "Trend "+id, "tiktok"))
	}

	results, total, err := repo.List(context.Background(), 0, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestTrendRepo_List_Empty(t *testing.T) {
	store := newTestStore(t)
	repo := NewTrendRepository(store)

	results, total, err := repo.List(context.Background(), 0, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 || len(results) != 0 {
		t.Errorf("expected empty list, got total=%d len=%d", total, len(results))
	}
}

// --- Update ---

func TestTrendRepo_Update_Success(t *testing.T) {
	store := newTestStore(t)
	repo := NewTrendRepository(store)
	trend := makeTrend("t1", "Trend One", "tiktok")
	_ = repo.Insert(context.Background(), trend)

	trend.Name = "Updated"
	if err := repo.Update(context.Background(), trend); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := repo.GetByID(context.Background(), "t1")
	if got.Name != "Updated" {
		t.Errorf("expected Updated, got %s", got.Name)
	}
}

// --- ListByIDs ---

func TestTrendRepo_ListByIDs_Success(t *testing.T) {
	store := newTestStore(t)
	repo := NewTrendRepository(store)
	for _, id := range []string{"x1", "x2", "x3"} {
		_ = repo.Insert(context.Background(), makeTrend(id, "T "+id, "youtube"))
	}

	results, err := repo.ListByIDs(context.Background(), []string{"x1", "x3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestTrendRepo_ListByIDs_Empty(t *testing.T) {
	store := newTestStore(t)
	repo := NewTrendRepository(store)

	results, err := repo.ListByIDs(context.Background(), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}
