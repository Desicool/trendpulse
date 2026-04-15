package badger

import (
	"context"
	"fmt"
	"time"

	badgerhold "github.com/timshannon/badgerhold/v4"
	"trendpulse/internal/domain"
)

// SignalRepository is the BadgerHold implementation of repository.SignalRepository.
type SignalRepository struct {
	store *badgerhold.Store
}

// NewSignalRepository creates a new SignalRepository backed by store.
func NewSignalRepository(store *badgerhold.Store) *SignalRepository {
	return &SignalRepository{store: store}
}

// Insert persists a new Signal.
func (r *SignalRepository) Insert(_ context.Context, signal *domain.Signal) error {
	return r.store.Insert(signal.ID, signal)
}

// ListByTrendID returns all Signals for a trend within [from, to], sorted
// ascending by Timestamp.
func (r *SignalRepository) ListByTrendID(_ context.Context, trendID string, from, to time.Time) ([]*domain.Signal, error) {
	var results []domain.Signal
	query := badgerhold.Where("TrendID").Eq(trendID).
		And("Timestamp").Ge(from).
		And("Timestamp").Le(to).
		SortBy("Timestamp")
	if err := r.store.Find(&results, query); err != nil {
		return nil, err
	}
	out := make([]*domain.Signal, len(results))
	for i := range results {
		out[i] = &results[i]
	}
	return out, nil
}

// GetLatestByTrendID returns the most recent Signal for a trend. Returns
// ErrNotFound when no signals exist for that trend.
func (r *SignalRepository) GetLatestByTrendID(_ context.Context, trendID string) (*domain.Signal, error) {
	var results []domain.Signal
	query := badgerhold.Where("TrendID").Eq(trendID).
		SortBy("Timestamp").Reverse().
		Limit(1)
	if err := r.store.Find(&results, query); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("signal for trend %q: %w", trendID, ErrNotFound)
	}
	return &results[0], nil
}

// ListByTimeRange returns all Signals whose Timestamp falls within [from, to].
func (r *SignalRepository) ListByTimeRange(_ context.Context, from, to time.Time) ([]*domain.Signal, error) {
	var results []domain.Signal
	query := badgerhold.Where("Timestamp").Ge(from).And("Timestamp").Le(to)
	if err := r.store.Find(&results, query); err != nil {
		return nil, err
	}
	out := make([]*domain.Signal, len(results))
	for i := range results {
		out[i] = &results[i]
	}
	return out, nil
}
