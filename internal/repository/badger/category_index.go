package badger

import (
	"context"
	"fmt"

	badgerhold "github.com/timshannon/badgerhold/v4"
	"trendpulse/internal/domain"
)

// CategoryIndex is the BadgerHold implementation of repository.CategoryIndex.
type CategoryIndex struct {
	store *badgerhold.Store
}

// NewCategoryIndex creates a new CategoryIndex backed by store.
func NewCategoryIndex(store *badgerhold.Store) *CategoryIndex {
	return &CategoryIndex{store: store}
}

// SetCategories creates or updates CategoryMapping records for each
// (category, trendID) pair. Uses Upsert semantics — safe to call repeatedly.
func (c *CategoryIndex) SetCategories(_ context.Context, trendID string, categories []string) error {
	for _, category := range categories {
		id := fmt.Sprintf("%s:%s", category, trendID)
		mapping := &domain.CategoryMapping{
			ID:       id,
			Category: category,
			TrendID:  trendID,
		}
		if err := c.store.Upsert(id, mapping); err != nil {
			return err
		}
	}
	return nil
}

// GetTrendIDsByCategory returns all Trend IDs associated with category.
func (c *CategoryIndex) GetTrendIDsByCategory(_ context.Context, category string) ([]string, error) {
	var mappings []domain.CategoryMapping
	query := badgerhold.Where("Category").Eq(category)
	if err := c.store.Find(&mappings, query); err != nil {
		return nil, err
	}
	ids := make([]string, len(mappings))
	for i, m := range mappings {
		ids[i] = m.TrendID
	}
	return ids, nil
}
