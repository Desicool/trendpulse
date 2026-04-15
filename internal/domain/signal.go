package domain

import "time"

type Signal struct {
	ID                string    `json:"id"              badgerhold:"key"`
	TrendID           string    `json:"trend_id"        badgerhold:"index"`
	Timestamp         time.Time `json:"timestamp"       badgerhold:"index"`
	UsageCount        int64     `json:"usage_count"`
	UniqueCreators    int64     `json:"unique_creators"`
	AvgViews          float64   `json:"avg_views"`
	AvgEngagement     float64   `json:"avg_engagement"`
	ViewConcentration float64   `json:"view_concentration"`
	CreatedAt         time.Time `json:"created_at"`
}
