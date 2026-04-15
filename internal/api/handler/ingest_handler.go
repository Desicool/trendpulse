package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"trendpulse/internal/api/response"
	"trendpulse/internal/domain"
	"trendpulse/internal/repository"
)

// IngestHandler handles POST /ingest/trends and POST /ingest/signals.
type IngestHandler struct {
	trendRepo     repository.TrendRepository
	signalRepo    repository.SignalRepository
	categoryIndex repository.CategoryIndex
}

// NewIngestHandler constructs an IngestHandler with the supplied dependencies.
func NewIngestHandler(
	trendRepo repository.TrendRepository,
	signalRepo repository.SignalRepository,
	categoryIndex repository.CategoryIndex,
) *IngestHandler {
	return &IngestHandler{
		trendRepo:     trendRepo,
		signalRepo:    signalRepo,
		categoryIndex: categoryIndex,
	}
}

// ingestTrendRequest is the JSON body for POST /ingest/trends.
type ingestTrendRequest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Categories  []string `json:"categories"`
	Source      string   `json:"source"`
}

// IngestTrend handles POST /ingest/trends.
func (h *IngestHandler) IngestTrend(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	var req ingestTrendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeInvalidRequest, "invalid JSON body")
		return
	}

	// Required field validation.
	if strings.TrimSpace(req.ID) == "" {
		response.Error(w, http.StatusUnprocessableEntity, response.CodeMissingRequiredField, "field 'id' is required")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		response.Error(w, http.StatusUnprocessableEntity, response.CodeMissingRequiredField, "field 'name' is required")
		return
	}
	if strings.TrimSpace(req.Source) == "" {
		response.Error(w, http.StatusUnprocessableEntity, response.CodeMissingRequiredField, "field 'source' is required")
		return
	}

	now := time.Now().UTC()
	categories := req.Categories
	if categories == nil {
		categories = []string{}
	}

	trend := &domain.Trend{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		Categories:  categories,
		Source:      req.Source,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.trendRepo.Insert(ctx, trend); err != nil {
		// Detect duplicate-key errors from any backend.
		errMsg := err.Error()
		if strings.Contains(errMsg, "already exists") {
			response.Error(w, http.StatusConflict, response.CodeTrendAlreadyExists, "trend ID already exists")
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternal, "failed to insert trend")
		return
	}

	// Best-effort category index update; errors are not fatal for the caller.
	if len(categories) > 0 {
		_ = h.categoryIndex.SetCategories(ctx, req.ID, categories)
	}

	response.Created(w, trend)
}

// ingestSignalRequest is the JSON body for POST /ingest/signals.
type ingestSignalRequest struct {
	TrendID           string  `json:"trend_id"`
	Timestamp         string  `json:"timestamp"`
	UsageCount        int64   `json:"usage_count"`
	UniqueCreators    int64   `json:"unique_creators"`
	AvgViews          float64 `json:"avg_views"`
	AvgEngagement     float64 `json:"avg_engagement"`
	ViewConcentration float64 `json:"view_concentration"`
}

// IngestSignal handles POST /ingest/signals.
func (h *IngestHandler) IngestSignal(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	var req ingestSignalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeInvalidRequest, "invalid JSON body")
		return
	}

	// Required field validation.
	if strings.TrimSpace(req.TrendID) == "" {
		response.Error(w, http.StatusUnprocessableEntity, response.CodeMissingRequiredField, "field 'trend_id' is required")
		return
	}
	if strings.TrimSpace(req.Timestamp) == "" {
		response.Error(w, http.StatusUnprocessableEntity, response.CodeMissingRequiredField, "field 'timestamp' is required")
		return
	}

	ts, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		response.Error(w, http.StatusUnprocessableEntity, response.CodeMissingRequiredField,
			"field 'timestamp' must be a valid RFC3339 datetime")
		return
	}

	// Domain value validations.
	if req.UsageCount < 0 {
		response.Error(w, http.StatusBadRequest, response.CodeInvalidRequest, "usage_count must be >= 0")
		return
	}
	if req.AvgEngagement < 0 {
		response.Error(w, http.StatusBadRequest, response.CodeInvalidRequest, "avg_engagement must be >= 0")
		return
	}
	if req.ViewConcentration != 0 && (req.ViewConcentration < 0 || req.ViewConcentration > 1) {
		response.Error(w, http.StatusBadRequest, response.CodeInvalidRequest,
			"view_concentration must be in [0.0, 1.0]")
		return
	}

	// Verify that the referenced trend exists.
	if _, err := h.trendRepo.GetByID(ctx, req.TrendID); err != nil {
		response.Error(w, http.StatusNotFound, response.CodeTrendNotFound, "trend ID not found")
		return
	}

	now := time.Now().UTC()
	signal := &domain.Signal{
		ID:                fmt.Sprintf("sig-%d", now.UnixNano()),
		TrendID:           req.TrendID,
		Timestamp:         ts,
		UsageCount:        req.UsageCount,
		UniqueCreators:    req.UniqueCreators,
		AvgViews:          req.AvgViews,
		AvgEngagement:     req.AvgEngagement,
		ViewConcentration: req.ViewConcentration,
		CreatedAt:         now,
	}

	if err := h.signalRepo.Insert(ctx, signal); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternal, "failed to insert signal")
		return
	}

	response.Created(w, signal)
}
