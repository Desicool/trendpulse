package domain

type CategoryMapping struct {
	ID       string `json:"id"       badgerhold:"key"`
	Category string `json:"category" badgerhold:"index"`
	TrendID  string `json:"trend_id" badgerhold:"index"`
}
