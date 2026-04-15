package calculator

// PhaseSignals holds the computed feature values used to determine Phase.
type PhaseSignals struct {
	PostGrowthRate     float64 // recent post-count growth rate (short-window first-order diff)
	ViewAcceleration   float64 // view acceleration (second-order diff)
	EngagementGrowth   float64 // engagement surge ratio (latest / moving avg)
	ViewConcentration  float64 // view concentration from latest signal [0, 1]
	AllMetricsNegative bool    // true when all growth metrics are negative
}

// PhaseConfig holds the classification thresholds for DeterminePhase.
type PhaseConfig struct {
	ExplodingAccelThreshold      float64
	ExplodingEngagementThreshold float64
	PeakingGrowthRateMax         float64
	PeakingGrowthRateMin         float64
}

// DefaultPhaseConfig returns the default thresholds from the spec.
func DefaultPhaseConfig() PhaseConfig {
	return PhaseConfig{
		ExplodingAccelThreshold:      0.5,
		ExplodingEngagementThreshold: 0.3,
		PeakingGrowthRateMax:         0.05,
		PeakingGrowthRateMin:         -0.02,
	}
}

// DeterminePhase classifies the current trend lifecycle stage.
//
// Priority order:
//  1. declining  — all metrics are negative
//  2. exploding  — view acceleration AND engagement both above thresholds
//  3. peaking    — post growth rate is near zero (inside [min, max])
//  4. emerging   — everything else
func DeterminePhase(s PhaseSignals, cfg PhaseConfig) string {
	if s.AllMetricsNegative {
		return "declining"
	}
	if s.ViewAcceleration > cfg.ExplodingAccelThreshold &&
		s.EngagementGrowth > cfg.ExplodingEngagementThreshold {
		return "exploding"
	}
	if s.PostGrowthRate < cfg.PeakingGrowthRateMax &&
		s.PostGrowthRate > cfg.PeakingGrowthRateMin {
		return "peaking"
	}
	return "emerging"
}
