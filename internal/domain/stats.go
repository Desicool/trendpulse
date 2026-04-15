package domain

import "time"

type TrendStats struct {
	ID           string    `json:"id"            badgerhold:"key"`
	TrendID      string    `json:"trend_id"      badgerhold:"index"`
	StrategyID   string    `json:"strategy_id"   badgerhold:"index"`
	CalculatedAt time.Time `json:"calculated_at"`

	Score      float64 `json:"score"`
	Phase      string  `json:"phase"      badgerhold:"index"`
	Confidence float64 `json:"confidence"`

	LatestUsageCount        int64   `json:"latest_usage_count"`
	LatestUniqueCreators    int64   `json:"latest_unique_creators"`
	LatestAvgViews          float64 `json:"latest_avg_views"`
	LatestAvgEngagement     float64 `json:"latest_avg_engagement"`
	LatestViewConcentration float64 `json:"latest_view_concentration"`

	Metadata map[string]interface{} `json:"metadata,omitempty"`
}
