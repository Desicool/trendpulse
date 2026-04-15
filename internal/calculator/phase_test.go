package calculator_test

import (
	"testing"

	"trendpulse/internal/calculator"
)

func TestDeterminePhase(t *testing.T) {
	cfg := calculator.DefaultPhaseConfig()

	tests := []struct {
		name      string
		signals   calculator.PhaseSignals
		wantPhase string
	}{
		{
			name: "all_metrics_negative_returns_declining",
			signals: calculator.PhaseSignals{
				AllMetricsNegative: true,
				ViewAcceleration:   -1.0,
				EngagementGrowth:   -0.5,
				PostGrowthRate:     -0.1,
			},
			wantPhase: "declining",
		},
		{
			name: "high_accel_and_engagement_returns_exploding",
			signals: calculator.PhaseSignals{
				AllMetricsNegative: false,
				ViewAcceleration:   0.8, // > 0.5 threshold
				EngagementGrowth:   0.6, // > 0.3 threshold
				PostGrowthRate:     0.2,
			},
			wantPhase: "rising",
		},
		{
			name: "accel_just_above_threshold_engagement_just_above_returns_exploding",
			signals: calculator.PhaseSignals{
				AllMetricsNegative: false,
				ViewAcceleration:   0.51,
				EngagementGrowth:   0.31,
				PostGrowthRate:     0.5,
			},
			wantPhase: "rising",
		},
		{
			name: "high_accel_but_low_engagement_not_exploding",
			signals: calculator.PhaseSignals{
				AllMetricsNegative: false,
				ViewAcceleration:   0.8,
				EngagementGrowth:   0.1, // below 0.3 threshold
				PostGrowthRate:     0.5,
			},
			wantPhase: "emerging",
		},
		{
			name: "post_growth_at_zero_returns_peaking",
			signals: calculator.PhaseSignals{
				AllMetricsNegative: false,
				ViewAcceleration:   0.2,
				EngagementGrowth:   0.1,
				PostGrowthRate:     0.0, // in (-0.02, 0.05)
			},
			wantPhase: "peaking",
		},
		{
			name: "post_growth_at_max_boundary_not_peaking",
			signals: calculator.PhaseSignals{
				AllMetricsNegative: false,
				ViewAcceleration:   0.1,
				EngagementGrowth:   0.1,
				PostGrowthRate:     0.05, // equal to max — not strictly less
			},
			wantPhase: "emerging",
		},
		{
			name: "post_growth_at_min_boundary_not_peaking",
			signals: calculator.PhaseSignals{
				AllMetricsNegative: false,
				ViewAcceleration:   0.1,
				EngagementGrowth:   0.1,
				PostGrowthRate:     -0.02, // equal to min — not strictly greater
			},
			wantPhase: "emerging",
		},
		{
			name: "positive_growth_no_special_condition_returns_emerging",
			signals: calculator.PhaseSignals{
				AllMetricsNegative: false,
				ViewAcceleration:   0.2,
				EngagementGrowth:   0.1,
				PostGrowthRate:     0.3, // above peaking range
			},
			wantPhase: "emerging",
		},
		{
			name: "declining_overrides_exploding_condition",
			signals: calculator.PhaseSignals{
				AllMetricsNegative: true,
				ViewAcceleration:   1.0, // would trigger exploding if not declining
				EngagementGrowth:   1.0,
				PostGrowthRate:     -0.5,
			},
			wantPhase: "declining",
		},
		{
			name: "small_positive_post_growth_in_peaking_range",
			signals: calculator.PhaseSignals{
				AllMetricsNegative: false,
				ViewAcceleration:   0.1,
				EngagementGrowth:   0.1,
				PostGrowthRate:     0.01, // 1%, inside (-2%, 5%)
			},
			wantPhase: "peaking",
		},
		{
			name: "slightly_negative_post_growth_in_peaking_range",
			signals: calculator.PhaseSignals{
				AllMetricsNegative: false,
				ViewAcceleration:   0.1,
				EngagementGrowth:   0.05,
				PostGrowthRate:     -0.01, // -1%, inside (-2%, 5%)
			},
			wantPhase: "peaking",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculator.DeterminePhase(tt.signals, cfg)
			if got != tt.wantPhase {
				t.Errorf("DeterminePhase() = %q, want %q", got, tt.wantPhase)
			}
		})
	}
}
