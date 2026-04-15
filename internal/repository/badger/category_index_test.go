package badger

import (
	"context"
	"testing"
)

// --- SetCategories / GetTrendIDsByCategory ---

func TestCategoryIndex_SetAndGet_Success(t *testing.T) {
	store := newTestStore(t)
	idx := NewCategoryIndex(store)

	if err := idx.SetCategories(context.Background(), "trend-1", []string{"music", "dance"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids, err := idx.GetTrendIDsByCategory(context.Background(), "music")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "trend-1" {
		t.Errorf("expected [trend-1], got %v", ids)
	}
}

func TestCategoryIndex_MultiTrendSameCategory(t *testing.T) {
	store := newTestStore(t)
	idx := NewCategoryIndex(store)

	_ = idx.SetCategories(context.Background(), "trend-1", []string{"music"})
	_ = idx.SetCategories(context.Background(), "trend-2", []string{"music", "art"})

	ids, err := idx.GetTrendIDsByCategory(context.Background(), "music")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 trend IDs, got %d: %v", len(ids), ids)
	}
}

func TestCategoryIndex_SetCategories_Idempotent(t *testing.T) {
	store := newTestStore(t)
	idx := NewCategoryIndex(store)

	// Call SetCategories twice for the same trend; should not duplicate
	_ = idx.SetCategories(context.Background(), "trend-1", []string{"fitness"})
	_ = idx.SetCategories(context.Background(), "trend-1", []string{"fitness"})

	ids, err := idx.GetTrendIDsByCategory(context.Background(), "fitness")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 {
		t.Errorf("expected 1 ID (idempotent), got %d: %v", len(ids), ids)
	}
}

func TestCategoryIndex_GetTrendIDsByCategory_NotFound(t *testing.T) {
	store := newTestStore(t)
	idx := NewCategoryIndex(store)

	ids, err := idx.GetTrendIDsByCategory(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty slice, got %v", ids)
	}
}

func TestCategoryIndex_EmptyCategories_NoOp(t *testing.T) {
	store := newTestStore(t)
	idx := NewCategoryIndex(store)

	if err := idx.SetCategories(context.Background(), "trend-1", []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
