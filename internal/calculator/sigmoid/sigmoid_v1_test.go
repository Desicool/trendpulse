package sigmoid_test

import (
	"context"
	"math"
	"testing"
	"time"

	"trendpulse/internal/calculator"
	"trendpulse/internal/calculator/sigmoid"
	"trendpulse/internal/domain"
)

// --- mock SignalReader ---

type mockSignalReader struct {
	latest     []*domain.Signal
	rangeOut   []*domain.Signal
	aggregated []*calculator.AggregatedSignal
	rangeErr   error
	latestErr  error
	aggrErr    error
}

func (m *mockSignalReader) Latest(_ context.Context, n int) ([]*domain.Signal, error) {
	if m.latestErr != nil {
		return nil, m.latestErr
	}
	if len(m.latest) == 0 {
		return nil, nil
	}
	start := len(m.latest) - n
	if start < 0 {
		start = 0
	}
	return m.latest[start:], nil
}

func (m *mockSignalReader) Range(_ context.Context, _, _ time.Time) ([]*domain.Signal, error) {
	if m.rangeErr != nil {
		return nil, m.rangeErr
	}
	return m.rangeOut, nil
}

func (m *mockSignalReader) Aggregate(_ context.Context, _ time.Duration) ([]*calculator.AggregatedSignal, error) {
	if m.aggrErr != nil {
		return nil, m.aggrErr
	}
	return m.aggregated, nil
}

// --- helpers ---

func defaultConfig() sigmoid.Config {
	return sigmoid.Config{
		Weights: sigmoid.WeightsConfig{
			ViewAcceleration:  2.5,
			PostGrowthRate:    1.5,
			CreatorGrowthRate: 1.5,
			EngagementSurge:   2.0,
			ViewConcentration: 1.0,
		},
		Bias:          3.0,
		LookbackShort: 6 * time.Hour,
		LookbackAccel: 3 * time.Hour,
		PhaseThresholds: calculator.PhaseConfig{
			ExplodingAccelThreshold:      0.5,
			ExplodingEngagementThreshold: 0.3,
			PeakingGrowthRateMax:         0.05,
			PeakingGrowthRateMin:         -0.02,
		},
	}
}

func newTrend(id string) *domain.Trend {
	return &domain.Trend{
		ID:        id,
		Name:      "Test Trend " + id,
		CreatedAt: time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC),
	}
}

func now() time.Time {
	return time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
}

// makeSignalSequence builds n signals spaced 1 hour apart with exponential growth.
func makeSignalSequence(trendID string, count int, base float64, growthRate float64) []*domain.Signal {
	signals := make([]*domain.Signal, count)
	t0 := now().Add(-time.Duration(count) * time.Hour)
	for i := 0; i < count; i++ {
		mult := math.Pow(growthRate, float64(i))
		signals[i] = &domain.Signal{
			ID:             trendID + "-" + string(rune('0'+i)),
			TrendID:        trendID,
			Timestamp:      t0.Add(time.Duration(i) * time.Hour),
			UsageCount:     int64(base * mult),
			UniqueCreators: int64((base / 10) * mult),
			AvgViews:       base * mult * 3.5,
		}
	}
	return signals
}

// makeAggregatedSequence builds n aggregated windows with exponential view growth.
func makeAggregatedSequence(count int, baseViews float64, growthRate float64) []*calculator.AggregatedSignal {
	wins := make([]*calculator.AggregatedSignal, count)
	t0 := now().Add(-time.Duration(count) * time.Hour)
	for i := 0; i < count; i++ {
		mult := math.Pow(growthRate, float64(i))
		wins[i] = &calculator.AggregatedSignal{
			WindowStart:   t0.Add(time.Duration(i) * time.Hour),
			WindowEnd:     t0.Add(time.Duration(i+1) * time.Hour),
			AvgViews:      baseViews * mult,
			AvgUsageCount: 100 * mult,
			SampleCount:   1,
		}
	}
	return wins
}

// --- tests ---

func TestSigmoidV1_EmptySignals_ReturnsZeroScoreEmergingPhase(t *testing.T) {
	s := sigmoid.NewSigmoidV1(defaultConfig())
	reader := &mockSignalReader{
		latest:     nil,
		rangeOut:   nil,
		aggregated: nil,
	}
	stats, err := s.Calculate(context.Background(), newTrend("t1"), reader)
	if err != nil {
		t.Fatalf("Calculate() error = %v, want nil", err)
	}
	if stats.Score != 0 {
		t.Errorf("Score = %v, want 0", stats.Score)
	}
	if stats.Confidence != 0 {
		t.Errorf("Confidence = %v, want 0", stats.Confidence)
	}
	if stats.Phase != "emerging" {
		t.Errorf("Phase = %q, want emerging", stats.Phase)
	}
	if stats.TrendID != "t1" {
		t.Errorf("TrendID = %q, want t1", stats.TrendID)
	}
	if stats.StrategyID != "sigmoid_v1" {
		t.Errorf("StrategyID = %q, want sigmoid_v1", stats.StrategyID)
	}
	if stats.ID != "sigmoid_v1:t1" {
		t.Errorf("ID = %q, want sigmoid_v1:t1", stats.ID)
	}
}

func TestSigmoidV1_SingleSignal_ReturnsZeroScoreEmergingPhase(t *testing.T) {
	s := sigmoid.NewSigmoidV1(defaultConfig())
	signals := makeSignalSequence("t2", 1, 1000, 1.0)
	reader := &mockSignalReader{
		latest:     signals,
		rangeOut:   signals,
		aggregated: makeAggregatedSequence(1, 1000, 1.0),
	}
	stats, err := s.Calculate(context.Background(), newTrend("t2"), reader)
	if err != nil {
		t.Fatalf("Calculate() error = %v, want nil", err)
	}
	// fewer than 2 valid signals → low-confidence early return
	if stats.Score != 0 {
		t.Errorf("Score = %v, want 0 for single signal", stats.Score)
	}
	if stats.Confidence != 0 {
		t.Errorf("Confidence = %v, want 0 for single signal", stats.Confidence)
	}
	if stats.Phase != "emerging" {
		t.Errorf("Phase = %q, want emerging for single signal", stats.Phase)
	}
}

func TestSigmoidV1_ViralSignal_ExplodingWithHighScore(t *testing.T) {
	s := sigmoid.NewSigmoidV1(defaultConfig())
	// Build signals with strong acceleration and engagement
	signals := makeSignalSequence("t3", 10, 1000, 1.3)
	// Add engagement values to signals
	for i, sig := range signals {
		sig.AvgEngagement = 10.0 * math.Pow(1.5, float64(i))
	}
	aggWindows := []*calculator.AggregatedSignal{
		{AvgViews: 1000, AvgEngagement: 10, AvgUsageCount: 100, AvgUniqueCreators: 10, SampleCount: 2},
		{AvgViews: 2000, AvgEngagement: 20, AvgUsageCount: 130, AvgUniqueCreators: 13, SampleCount: 2},
		{AvgViews: 5000, AvgEngagement: 80, AvgUsageCount: 169, AvgUniqueCreators: 17, SampleCount: 2},
	}

	reader := &mockSignalReader{
		latest:     signals,
		rangeOut:   signals,
		aggregated: aggWindows,
	}
	stats, err := s.Calculate(context.Background(), newTrend("t3"), reader)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	if stats.Score <= 70.0 {
		t.Errorf("Score = %v, want > 70 for viral signal", stats.Score)
	}
	if stats.Phase != "exploding" {
		t.Errorf("Phase = %q, want exploding for viral signal", stats.Phase)
	}
	if stats.Score < 0 || stats.Score > 100 {
		t.Errorf("Score = %v out of [0,100]", stats.Score)
	}
	if stats.Confidence <= 0 {
		t.Errorf("Confidence = %v, want > 0", stats.Confidence)
	}
}

func TestSigmoidV1_SteadyDecline_DecliningPhaseAndLowScore(t *testing.T) {
	s := sigmoid.NewSigmoidV1(defaultConfig())
	// Declining signals: 30% drop per step
	signals := makeSignalSequence("t4", 10, 5000, 0.7)
	aggWindows := []*calculator.AggregatedSignal{
		{AvgViews: 5000, AvgUsageCount: 500, AvgUniqueCreators: 50, SampleCount: 2},
		{AvgViews: 3500, AvgUsageCount: 350, AvgUniqueCreators: 35, SampleCount: 2},
		{AvgViews: 2450, AvgUsageCount: 245, AvgUniqueCreators: 25, SampleCount: 2},
	}
	reader := &mockSignalReader{
		latest:     signals,
		rangeOut:   signals,
		aggregated: aggWindows,
	}
	stats, err := s.Calculate(context.Background(), newTrend("t4"), reader)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	if stats.Score >= 50.0 {
		t.Errorf("Score = %v, want < 50 for declining signal", stats.Score)
	}
	if stats.Phase != "declining" {
		t.Errorf("Phase = %q, want declining", stats.Phase)
	}
}

func TestSigmoidV1_StableSignals_PeakingPhaseAndMidScore(t *testing.T) {
	s := sigmoid.NewSigmoidV1(defaultConfig())
	// Stable signals: near-zero growth
	signals := makeSignalSequence("t5", 10, 3000, 1.0)
	// Set engagement to non-zero for dimension coverage
	for i := range signals {
		signals[i].AvgEngagement = 15.0
	}
	aggWindows := []*calculator.AggregatedSignal{
		{AvgViews: 3000, AvgUsageCount: 300, AvgUniqueCreators: 30, AvgEngagement: 15, SampleCount: 3},
		{AvgViews: 3010, AvgUsageCount: 301, AvgUniqueCreators: 30, AvgEngagement: 15, SampleCount: 3},
		{AvgViews: 3005, AvgUsageCount: 300, AvgUniqueCreators: 30, AvgEngagement: 15, SampleCount: 3},
	}
	reader := &mockSignalReader{
		latest:     signals,
		rangeOut:   signals,
		aggregated: aggWindows,
	}
	stats, err := s.Calculate(context.Background(), newTrend("t5"), reader)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	if stats.Phase != "peaking" {
		t.Errorf("Phase = %q, want peaking for stable signals", stats.Phase)
	}
}

func TestSigmoidV1_OutputFieldsPopulated(t *testing.T) {
	s := sigmoid.NewSigmoidV1(defaultConfig())
	signals := makeSignalSequence("t6", 8, 1000, 1.1)
	for i := range signals {
		signals[i].AvgEngagement = 5.0
		signals[i].ViewConcentration = 0.3
	}
	aggWindows := makeAggregatedSequence(3, 1000, 1.1)
	for _, w := range aggWindows {
		w.AvgEngagement = 5.0
		w.AvgViewConcentration = 0.3
	}
	reader := &mockSignalReader{
		latest:     signals,
		rangeOut:   signals,
		aggregated: aggWindows,
	}
	trend := newTrend("t6")
	stats, err := s.Calculate(context.Background(), trend, reader)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}

	latest := signals[len(signals)-1]
	if stats.LatestUsageCount != latest.UsageCount {
		t.Errorf("LatestUsageCount = %d, want %d", stats.LatestUsageCount, latest.UsageCount)
	}
	if stats.LatestUniqueCreators != latest.UniqueCreators {
		t.Errorf("LatestUniqueCreators = %d, want %d", stats.LatestUniqueCreators, latest.UniqueCreators)
	}
	if stats.LatestAvgViews != latest.AvgViews {
		t.Errorf("LatestAvgViews = %v, want %v", stats.LatestAvgViews, latest.AvgViews)
	}
	if stats.LatestAvgEngagement != latest.AvgEngagement {
		t.Errorf("LatestAvgEngagement = %v, want %v", stats.LatestAvgEngagement, latest.AvgEngagement)
	}
	if stats.LatestViewConcentration != latest.ViewConcentration {
		t.Errorf("LatestViewConcentration = %v, want %v", stats.LatestViewConcentration, latest.ViewConcentration)
	}
	if stats.Metadata == nil {
		t.Error("Metadata is nil, want non-nil map")
	}
	if stats.Score < 0 || stats.Score > 100 {
		t.Errorf("Score = %v out of [0,100]", stats.Score)
	}
	if stats.Confidence < 0 || stats.Confidence > 1 {
		t.Errorf("Confidence = %v out of [0,1]", stats.Confidence)
	}
	if stats.CalculatedAt.IsZero() {
		t.Error("CalculatedAt is zero")
	}
}

func TestSigmoidV1_SparseEngagementAndViewConc_SkippedDimensions(t *testing.T) {
	s := sigmoid.NewSigmoidV1(defaultConfig())
	// Signals with zero engagement and view_concentration — sparse fields
	signals := makeSignalSequence("t7", 6, 1000, 1.1)
	// engagement and view_concentration are zero (default)
	aggWindows := makeAggregatedSequence(3, 1000, 1.1)
	reader := &mockSignalReader{
		latest:     signals,
		rangeOut:   signals,
		aggregated: aggWindows,
	}
	stats, err := s.Calculate(context.Background(), newTrend("t7"), reader)
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	// Should still produce a valid result, just with fewer dimensions
	if stats.Score < 0 || stats.Score > 100 {
		t.Errorf("Score = %v out of [0,100] with sparse fields", stats.Score)
	}
	if stats.Confidence < 0 || stats.Confidence > 1 {
		t.Errorf("Confidence = %v out of [0,1] with sparse fields", stats.Confidence)
	}
}

func TestSigmoidV1_ID_And_Name(t *testing.T) {
	s := sigmoid.NewSigmoidV1(defaultConfig())
	if s.ID() != "sigmoid_v1" {
		t.Errorf("ID() = %q, want sigmoid_v1", s.ID())
	}
	if s.Name() == "" {
		t.Error("Name() returned empty string")
	}
}
