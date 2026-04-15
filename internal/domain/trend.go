package domain

import "time"

type Trend struct {
	ID          string    `json:"id"           badgerhold:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Categories  []string  `json:"categories"`
	Source      string    `json:"source"       badgerhold:"index"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
