package testutil

import (
	"context"
	"fmt"
	"sync"
)

// MockCategoryIndex is a hand-written in-memory mock of repository.CategoryIndex.
// It is safe for concurrent use.
type MockCategoryIndex struct {
	mu      sync.RWMutex
	// category -> []trendID
	index map[string][]string
}

// NewMockCategoryIndex returns a ready-to-use MockCategoryIndex.
func NewMockCategoryIndex() *MockCategoryIndex {
	return &MockCategoryIndex{
		index: make(map[string][]string),
	}
}

func (m *MockCategoryIndex) SetCategories(ctx context.Context, trendID string, categories []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, cat := range categories {
		// Avoid duplicate trendID entries for the same category.
		already := false
		for _, existing := range m.index[cat] {
			if existing == trendID {
				already = true
				break
			}
		}
		if !already {
			m.index[cat] = append(m.index[cat], trendID)
		}
	}
	return nil
}

func (m *MockCategoryIndex) GetTrendIDsByCategory(ctx context.Context, category string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids, ok := m.index[category]
	if !ok {
		return nil, fmt.Errorf("category %q not found", category)
	}
	// Return a copy to avoid external mutation.
	result := make([]string, len(ids))
	copy(result, ids)
	return result, nil
}
