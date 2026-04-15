package badger

import (
	"context"
	"errors"
	"fmt"

	badgerhold "github.com/timshannon/badgerhold/v4"
	"trendpulse/internal/domain"
)

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = errors.New("not found")

// TrendRepository is the BadgerHold implementation of repository.TrendRepository.
type TrendRepository struct {
	store *badgerhold.Store
}

// NewTrendRepository creates a new TrendRepository backed by store.
func NewTrendRepository(store *badgerhold.Store) *TrendRepository {
	return &TrendRepository{store: store}
}

// Insert persists a new Trend. Returns an error if an entry with the same ID
// already exists.
func (r *TrendRepository) Insert(_ context.Context, trend *domain.Trend) error {
	return r.store.Insert(trend.ID, trend)
}

// GetByID retrieves a Trend by its primary key. Returns ErrNotFound when the
// key does not exist.
func (r *TrendRepository) GetByID(_ context.Context, id string) (*domain.Trend, error) {
	var t domain.Trend
	err := r.store.Get(id, &t)
	if errors.Is(err, badgerhold.ErrNotFound) {
		return nil, fmt.Errorf("trend %q: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// List returns a page of Trends together with the total count.
func (r *TrendRepository) List(_ context.Context, offset, limit int) ([]*domain.Trend, int, error) {
	var results []domain.Trend
	// A nil query matches all records in badgerhold v4.
	if err := r.store.Find(&results, nil); err != nil {
		return nil, 0, err
	}
	total := len(results)

	// Apply manual offset / limit
	if offset >= total {
		return []*domain.Trend{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	slice := results[offset:end]
	out := make([]*domain.Trend, len(slice))
	for i := range slice {
		out[i] = &slice[i]
	}
	return out, total, nil
}

// Update overwrites an existing Trend record.
func (r *TrendRepository) Update(_ context.Context, trend *domain.Trend) error {
	return r.store.Update(trend.ID, trend)
}

// ListByIDs returns all Trends whose IDs are in the given slice.
// Missing IDs are silently skipped.
func (r *TrendRepository) ListByIDs(ctx context.Context, ids []string) ([]*domain.Trend, error) {
	if len(ids) == 0 {
		return []*domain.Trend{}, nil
	}
	out := make([]*domain.Trend, 0, len(ids))
	for _, id := range ids {
		t, err := r.GetByID(ctx, id)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}
