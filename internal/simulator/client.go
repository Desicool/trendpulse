package simulator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client is an HTTP client that talks to the TrendPulse API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a Client pointing at baseURL (e.g. "http://localhost:8080").
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ingestTrendRequest mirrors the POST /ingest/trends request body.
type ingestTrendRequest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Categories  []string `json:"categories"`
	Source      string   `json:"source"`
}

// ingestSignalRequest mirrors the POST /ingest/signals request body.
type ingestSignalRequest struct {
	TrendID           string    `json:"trend_id"`
	Timestamp         time.Time `json:"timestamp"`
	UsageCount        int64     `json:"usage_count"`
	UniqueCreators    int64     `json:"unique_creators"`
	AvgViews          float64   `json:"avg_views"`
	AvgEngagement     float64   `json:"avg_engagement"`
	ViewConcentration float64   `json:"view_concentration"`
}

// triggerRequest mirrors the POST /internal/scheduler/trigger request body.
type triggerRequest struct {
	AsOf time.Time `json:"as_of"`
}

// IngestTrend posts a trend to POST /ingest/trends.
func (c *Client) IngestTrend(ctx context.Context, spec TrendSpec) error {
	body := ingestTrendRequest{
		ID:          spec.ID,
		Name:        spec.Name,
		Description: spec.Description,
		Categories:  spec.Categories,
		Source:      spec.Source,
	}
	return c.post(ctx, "/ingest/trends", body)
}

// IngestSignalBatch posts all signals in a batch to POST /ingest/signals,
// one request per signal.
func (c *Client) IngestSignalBatch(ctx context.Context, batch SignalBatch) error {
	for _, sig := range batch.Signals {
		req := ingestSignalRequest{
			TrendID:           sig.TrendID,
			Timestamp:         sig.Timestamp,
			UsageCount:        sig.UsageCount,
			UniqueCreators:    sig.UniqueCreators,
			AvgViews:          sig.AvgViews,
			AvgEngagement:     sig.AvgEngagement,
			ViewConcentration: sig.ViewConcentration,
		}
		if err := c.post(ctx, "/ingest/signals", req); err != nil {
			return err
		}
	}
	return nil
}

// TriggerCalculation calls POST /internal/scheduler/trigger with the given asOf time.
func (c *Client) TriggerCalculation(ctx context.Context, asOf time.Time) error {
	return c.post(ctx, "/internal/scheduler/trigger", triggerRequest{AsOf: asOf})
}

// post marshals body to JSON and sends a POST request to path.
// It returns an error if the HTTP status is >= 400.
func (c *Client) post(ctx context.Context, path string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("client: marshal %s: %w", path, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("client: new request %s: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("client: post %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("client: post %s: unexpected status %d", path, resp.StatusCode)
	}
	return nil
}
