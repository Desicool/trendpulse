package sigmoid

import (
	"context"
	"fmt"
	"math"
	"time"

	"trendpulse/internal/calculator"
	"trendpulse/internal/domain"
)

// WeightsConfig holds the score component weights.
type WeightsConfig struct {
	ViewAcceleration  float64
	PostGrowthRate    float64
	CreatorGrowthRate float64
	EngagementSurge   float64
	ViewConcentration float64
}

// FeatureNormConfig holds the per-feature logistic normalization parameters.
// The normalization formula is: sigmoid((x - Center) * Scale)
type FeatureNormConfig struct {
	Center float64
	Scale  float64
}

// FeatureNormsConfig holds normalization parameters for all five score features.
type FeatureNormsConfig struct {
	ViewAcceleration  FeatureNormConfig
	PostGrowthRate    FeatureNormConfig
	CreatorGrowthRate FeatureNormConfig
	EngagementSurge   FeatureNormConfig
	ViewConcentration FeatureNormConfig
}

// Config is the configuration for the sigmoid_v1 strategy.
type Config struct {
	Weights         WeightsConfig
	FeatureNorms    FeatureNormsConfig
	Bias            float64
	LookbackShort   time.Duration
	LookbackAccel   time.Duration
	PhaseThresholds calculator.PhaseConfig
	// SignalInterval is used for Confidence calculation.
	// Defaults to 1 hour if zero.
	SignalInterval time.Duration
	// SignalLookback is the full lookback window for fetching signals.
	// Defaults to 48h if zero.
	SignalLookback time.Duration
}

// SigmoidV1 implements the sigmoid_v1 prediction strategy.
// It is safe for concurrent use.
type SigmoidV1 struct {
	cfg Config
}

// NewSigmoidV1 creates a SigmoidV1 strategy with the given config.
func NewSigmoidV1(cfg Config) *SigmoidV1 {
	if cfg.SignalInterval == 0 {
		cfg.SignalInterval = time.Hour
	}
	if cfg.SignalLookback == 0 {
		cfg.SignalLookback = 48 * time.Hour
	}
	return &SigmoidV1{cfg: cfg}
}

func (s *SigmoidV1) ID() string   { return "sigmoid_v1" }
func (s *SigmoidV1) Name() string { return "Sigmoid V1" }

// Calculate computes Score and Phase for the given trend using the signal reader.
func (s *SigmoidV1) Calculate(ctx context.Context, trend *domain.Trend, reader calculator.SignalReader) (*domain.TrendStats, error) {
	base := &domain.TrendStats{
		ID:           fmt.Sprintf("%s:%s", s.ID(), trend.ID),
		TrendID:      trend.ID,
		StrategyID:   s.ID(),
		CalculatedAt: time.Now().UTC(),
		Score:        0,
		Confidence:   0,
		Phase:        "emerging",
	}

	// Fetch latest signals to populate Latest* fields and check minimum data.
	latestSignals, err := reader.Latest(ctx, 100)
	if err != nil {
		return nil, fmt.Errorf("sigmoid_v1: Latest: %w", err)
	}

	if len(latestSignals) < 2 {
		return base, nil
	}

	// Populate latest metric fields from the most recent signal.
	latest := latestSignals[len(latestSignals)-1]
	base.LatestUsageCount = latest.UsageCount
	base.LatestUniqueCreators = latest.UniqueCreators
	base.LatestAvgViews = latest.AvgViews
	base.LatestAvgEngagement = latest.AvgEngagement
	base.LatestViewConcentration = latest.ViewConcentration

	// Derive "now" from the latest signal timestamp (not wall-clock time),
	// so simulated/historical data uses the correct time window.
	now := latest.Timestamp
	from := now.Add(-s.cfg.SignalLookback)
	rangeSignals, err := reader.Range(ctx, from, now)
	if err != nil {
		return nil, fmt.Errorf("sigmoid_v1: Range: %w", err)
	}

	// Fetch aggregated windows for view acceleration.
	aggWindows, err := reader.Aggregate(ctx, s.cfg.LookbackAccel)
	if err != nil {
		return nil, fmt.Errorf("sigmoid_v1: Aggregate: %w", err)
	}

	validCount := len(rangeSignals)
	if validCount < 2 {
		return base, nil
	}

	// --- Feature extraction ---

	// 1. view_accel: second-order diff of AvgViews using last 3 aggregated windows.
	viewAccel := 0.0
	if len(aggWindows) >= 3 {
		n := len(aggWindows)
		vt := aggWindows[n-1].AvgViews
		vt1 := aggWindows[n-2].AvgViews
		vt2 := aggWindows[n-3].AvgViews
		if vt2 != 0 {
			viewAccel = (vt - 2*vt1 + vt2) / vt2
		}
	}

	// 2. post_growth: (u[t] - u[t-k]) / u[t-k]
	//    Using first and last signals in the short lookback range.
	postGrowth := 0.0
	creatorGrowth := 0.0
	shortFrom := now.Add(-s.cfg.LookbackShort)
	shortSignals := filterSignalsAfter(rangeSignals, shortFrom)
	if len(shortSignals) >= 2 {
		first := shortSignals[0]
		last := shortSignals[len(shortSignals)-1]
		if first.UsageCount != 0 {
			postGrowth = float64(last.UsageCount-first.UsageCount) / float64(first.UsageCount)
		}
		if first.UniqueCreators != 0 {
			creatorGrowth = float64(last.UniqueCreators-first.UniqueCreators) / float64(first.UniqueCreators)
		}
	} else if len(rangeSignals) >= 2 {
		// Fallback: use all range signals.
		first := rangeSignals[0]
		last := rangeSignals[len(rangeSignals)-1]
		if first.UsageCount != 0 {
			postGrowth = float64(last.UsageCount-first.UsageCount) / float64(first.UsageCount)
		}
		if first.UniqueCreators != 0 {
			creatorGrowth = float64(last.UniqueCreators-first.UniqueCreators) / float64(first.UniqueCreators)
		}
	}

	// 3. engagement_surge: avg_engagement[t] / moving_avg_engagement (short window).
	engagementSurge := 0.0
	engagementDim := false
	{
		nonZeroEng := nonZeroEngagements(rangeSignals)
		if len(nonZeroEng) >= 2 {
			engagementDim = true
			movingAvg := mean(nonZeroEng)
			if movingAvg != 0 {
				// latest non-zero engagement
				latestEng := lastNonZeroEngagement(rangeSignals)
				engagementSurge = latestEng / movingAvg
			}
		}
	}

	// 4. view_conc: latest signal's ViewConcentration.
	viewConc := 0.0
	viewConcDim := false
	{
		nonZeroVC := nonZeroViewConcentrations(rangeSignals)
		if len(nonZeroVC) >= 2 {
			viewConcDim = true
			viewConc = latest.ViewConcentration
		}
	}

	// --- Dimension coverage ---
	// 5 dimensions: view_accel, post_growth, creator_growth, engagement_surge, view_conc
	covered := 3 // core three always covered (view_accel, post_growth, creator_growth)
	if engagementDim {
		covered++
	}
	if viewConcDim {
		covered++
	}
	dimensionCoverage := float64(covered) / 5.0

	// --- Per-feature normalization ---
	fn := s.cfg.FeatureNorms
	viewAccelNorm := featureNorm(viewAccel, fn.ViewAcceleration.Center, fn.ViewAcceleration.Scale)
	postGrowthNorm := featureNorm(postGrowth, fn.PostGrowthRate.Center, fn.PostGrowthRate.Scale)
	creatorGrowthNorm := featureNorm(creatorGrowth, fn.CreatorGrowthRate.Center, fn.CreatorGrowthRate.Scale)
	engagementSurgeNorm := featureNorm(engagementSurge, fn.EngagementSurge.Center, fn.EngagementSurge.Scale)
	viewConcNorm := featureNorm(viewConc, fn.ViewConcentration.Center, fn.ViewConcentration.Scale)

	// --- Score ---
	w := s.cfg.Weights
	raw := w.ViewAcceleration*viewAccelNorm +
		w.PostGrowthRate*postGrowthNorm +
		w.CreatorGrowthRate*creatorGrowthNorm
	if engagementDim {
		raw += w.EngagementSurge * engagementSurgeNorm
	}
	if viewConcDim {
		raw += w.ViewConcentration * viewConcNorm
	}
	raw -= s.cfg.Bias
	score := 100.0 * sigmoid(raw)

	// --- Confidence ---
	expectedCount := max1(int(s.cfg.SignalLookback / s.cfg.SignalInterval))
	confidence := (float64(validCount) / float64(expectedCount)) * dimensionCoverage
	if confidence > 1.0 {
		confidence = 1.0
	}

	// --- Phase ---
	// Compute first-order view growth from aggregated windows if available.
	viewGrowthNeg := false
	if len(aggWindows) >= 2 {
		n := len(aggWindows)
		first := aggWindows[0].AvgViews
		last := aggWindows[n-1].AvgViews
		viewGrowthNeg = last < first
	}
	allNeg := postGrowth < 0 && creatorGrowth < 0 && viewGrowthNeg
	if engagementDim {
		allNeg = allNeg && (engagementSurge < 1.0)
	}
	phaseSignals := calculator.PhaseSignals{
		PostGrowthRate:     postGrowth,
		ViewAcceleration:   viewAccel,
		EngagementGrowth:   engagementSurge,
		ViewConcentration:  viewConc,
		AllMetricsNegative: allNeg,
	}
	phase := calculator.DeterminePhase(phaseSignals, s.cfg.PhaseThresholds)

	base.Score = score
	base.Confidence = confidence
	base.Phase = phase
	base.Metadata = map[string]interface{}{
		"view_accel":            viewAccel,
		"view_accel_norm":       viewAccelNorm,
		"post_growth":           postGrowth,
		"post_growth_norm":      postGrowthNorm,
		"creator_growth":        creatorGrowth,
		"creator_growth_norm":   creatorGrowthNorm,
		"engagement_surge":      engagementSurge,
		"engagement_surge_norm": engagementSurgeNorm,
		"view_conc":             viewConc,
		"view_conc_norm":        viewConcNorm,
		"raw":                   raw,
		"valid_count":           validCount,
		"dim_coverage":          dimensionCoverage,
	}

	return base, nil
}

// --- helpers ---

func sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

// featureNorm applies per-feature logistic normalization:
// sigmoid((x - center) * scale)
func featureNorm(x, center, scale float64) float64 {
	return sigmoid((x - center) * scale)
}

func filterSignalsAfter(signals []*domain.Signal, from time.Time) []*domain.Signal {
	var out []*domain.Signal
	for _, sig := range signals {
		if !sig.Timestamp.Before(from) {
			out = append(out, sig)
		}
	}
	return out
}

func nonZeroEngagements(signals []*domain.Signal) []float64 {
	var out []float64
	for _, sig := range signals {
		if sig.AvgEngagement != 0 {
			out = append(out, sig.AvgEngagement)
		}
	}
	return out
}

func lastNonZeroEngagement(signals []*domain.Signal) float64 {
	for i := len(signals) - 1; i >= 0; i-- {
		if signals[i].AvgEngagement != 0 {
			return signals[i].AvgEngagement
		}
	}
	return 0
}

func nonZeroViewConcentrations(signals []*domain.Signal) []float64 {
	var out []float64
	for _, sig := range signals {
		if sig.ViewConcentration != 0 {
			out = append(out, sig.ViewConcentration)
		}
	}
	return out
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
