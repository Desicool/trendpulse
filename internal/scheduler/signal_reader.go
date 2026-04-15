package scheduler

import (
	"context"
	"sort"
	"time"

	"trendpulse/internal/calculator"
	"trendpulse/internal/domain"
	"trendpulse/internal/repository"
)

// repositorySignalReader implements calculator.SignalReader bound to a specific
// trendID and lookback window.
type repositorySignalReader struct {
	trendID    string
	signalRepo repository.SignalRepository
	lookback   time.Duration
	now        time.Time // anchor for Latest() and Aggregate() — set by scheduler
}

// NewSignalReader creates a SignalReader backed by the given SignalRepository.
// All reads are scoped to trendID within the lookback window.
// now is the anchor time used by Latest() and Aggregate() as the upper bound.
func NewSignalReader(trendID string, repo repository.SignalRepository, lookback time.Duration, now time.Time) calculator.SignalReader {
	return &repositorySignalReader{
		trendID:    trendID,
		signalRepo: repo,
		lookback:   lookback,
		now:        now,
	}
}

// Latest returns the most recent n signals for the trend, ascending by Timestamp.
func (r *repositorySignalReader) Latest(ctx context.Context, n int) ([]*domain.Signal, error) {
	now := r.now
	from := now.Add(-r.lookback)
	signals, err := r.signalRepo.ListByTrendID(ctx, r.trendID, from, now)
	if err != nil {
		return nil, err
	}
	// ListByTrendID should return ascending, but sort to be safe.
	sortAscending(signals)
	if len(signals) <= n {
		return signals, nil
	}
	return signals[len(signals)-n:], nil
}

// Range returns all signals in [from, to] for the trend, ascending by Timestamp.
func (r *repositorySignalReader) Range(ctx context.Context, from, to time.Time) ([]*domain.Signal, error) {
	signals, err := r.signalRepo.ListByTrendID(ctx, r.trendID, from, to)
	if err != nil {
		return nil, err
	}
	sortAscending(signals)
	return signals, nil
}

// Aggregate fetches all signals in the lookback window and groups them into
// time buckets of `window` duration, computing per-bucket averages.
func (r *repositorySignalReader) Aggregate(ctx context.Context, window time.Duration) ([]*calculator.AggregatedSignal, error) {
	now := r.now
	from := now.Add(-r.lookback)
	signals, err := r.signalRepo.ListByTrendID(ctx, r.trendID, from, now)
	if err != nil {
		return nil, err
	}
	if len(signals) == 0 {
		return nil, nil
	}
	sortAscending(signals)
	return aggregateByWindow(signals, from, now, window), nil
}

// aggregateByWindow groups signals into fixed-size time buckets and computes averages.
func aggregateByWindow(signals []*domain.Signal, from, to time.Time, window time.Duration) []*calculator.AggregatedSignal {
	if window <= 0 {
		window = time.Hour
	}
	// Build buckets
	type bucket struct {
		start   time.Time
		signals []*domain.Signal
	}

	// Determine bucket boundaries starting at `from`.
	bucketOf := func(ts time.Time) time.Time {
		offset := ts.Sub(from)
		bucketIndex := int64(offset / window)
		return from.Add(time.Duration(bucketIndex) * window)
	}

	bucketMap := make(map[time.Time]*bucket)
	var bucketOrder []time.Time

	for _, sig := range signals {
		start := bucketOf(sig.Timestamp)
		if _, exists := bucketMap[start]; !exists {
			bucketMap[start] = &bucket{start: start}
			bucketOrder = append(bucketOrder, start)
		}
		bucketMap[start].signals = append(bucketMap[start].signals, sig)
	}

	sort.Slice(bucketOrder, func(i, j int) bool {
		return bucketOrder[i].Before(bucketOrder[j])
	})

	result := make([]*calculator.AggregatedSignal, 0, len(bucketOrder))
	for _, start := range bucketOrder {
		b := bucketMap[start]
		agg := computeBucketAvg(b.signals, start, start.Add(window))
		result = append(result, agg)
	}
	return result
}

func computeBucketAvg(signals []*domain.Signal, windowStart, windowEnd time.Time) *calculator.AggregatedSignal {
	n := float64(len(signals))
	agg := &calculator.AggregatedSignal{
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		SampleCount: len(signals),
	}
	for _, sig := range signals {
		agg.AvgUsageCount += float64(sig.UsageCount)
		agg.AvgUniqueCreators += float64(sig.UniqueCreators)
		agg.AvgViews += sig.AvgViews
		agg.AvgEngagement += sig.AvgEngagement
		agg.AvgViewConcentration += sig.ViewConcentration
	}
	agg.AvgUsageCount /= n
	agg.AvgUniqueCreators /= n
	agg.AvgViews /= n
	agg.AvgEngagement /= n
	agg.AvgViewConcentration /= n
	return agg
}

func sortAscending(signals []*domain.Signal) {
	sort.Slice(signals, func(i, j int) bool {
		return signals[i].Timestamp.Before(signals[j].Timestamp)
	})
}
