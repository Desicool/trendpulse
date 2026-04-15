package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"trendpulse/internal/api/response"
	"trendpulse/internal/domain"
	"trendpulse/internal/repository"
)

// validPhases is the set of accepted phase filter values.
var validPhases = map[string]bool{
	"emerging":  true,
	"rising":    true,
	"peaking":   true,
	"declining": true,
}

// TrendHandler handles GET /trends, GET /trends/{id}, and GET /trends/rising.
type TrendHandler struct {
	trendRepo      repository.TrendRepository
	statsRepo      repository.StatsRepository
	activeStrategy string
	defaultLimit   int
	maxLimit       int
	risingTopK     int
	risingMinScore float64
}

// NewTrendHandler constructs a TrendHandler with the supplied dependencies and
// configuration values.
func NewTrendHandler(
	trendRepo repository.TrendRepository,
	statsRepo repository.StatsRepository,
	activeStrategy string,
	defaultLimit int,
	maxLimit int,
	risingTopK int,
	risingMinScore float64,
) *TrendHandler {
	return &TrendHandler{
		trendRepo:      trendRepo,
		statsRepo:      statsRepo,
		activeStrategy: activeStrategy,
		defaultLimit:   defaultLimit,
		maxLimit:       maxLimit,
		risingTopK:     risingTopK,
		risingMinScore: risingMinScore,
	}
}

// List handles GET /trends — paginated list with optional ?phase= filter.
func (h *TrendHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	q := r.URL.Query()

	// --- parse & validate pagination ---
	offset, err := parseIntParam(q.Get("offset"), 0)
	if err != nil || offset < 0 {
		response.Error(w, http.StatusBadRequest, response.CodeInvalidPagination, "offset must be a non-negative integer")
		return
	}

	limit, err := parseIntParam(q.Get("limit"), h.defaultLimit)
	if err != nil || limit <= 0 || limit > h.maxLimit {
		response.Error(w, http.StatusBadRequest, response.CodeInvalidPagination,
			"limit must be a positive integer not exceeding "+strconv.Itoa(h.maxLimit))
		return
	}

	// --- parse & validate optional phase filter ---
	phase := strings.TrimSpace(q.Get("phase"))
	if phase != "" && !validPhases[phase] {
		response.Error(w, http.StatusBadRequest, response.CodeInvalidRequest,
			"invalid phase value; valid values are: emerging, rising, peaking, declining")
		return
	}

	if phase == "" {
		// No phase filter — query directly from TrendRepository.
		trends, total, err := h.trendRepo.List(ctx, offset, limit)
		if err != nil {
			response.Error(w, http.StatusInternalServerError, response.CodeInternal, "failed to list trends")
			return
		}
		response.OK(w, map[string]interface{}{
			"items": trends,
			"pagination": map[string]interface{}{
				"total":  total,
				"offset": offset,
				"limit":  limit,
			},
		})
		return
	}

	// Phase filter path: list all stats for active strategy, filter by phase,
	// collect trend IDs, batch-fetch trends, apply manual offset/limit.
	allStats, err := h.statsRepo.ListByStrategyID(ctx, h.activeStrategy)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternal, "failed to retrieve stats")
		return
	}

	var trendIDs []string
	for _, s := range allStats {
		if s.Phase == phase {
			trendIDs = append(trendIDs, s.TrendID)
		}
	}

	total := len(trendIDs)

	// Apply offset/limit to the ID slice before fetching.
	if offset >= total {
		response.OK(w, map[string]interface{}{
			"items": []*struct{}{},
			"pagination": map[string]interface{}{
				"total":  total,
				"offset": offset,
				"limit":  limit,
			},
		})
		return
	}
	end := offset + limit
	if end > total {
		end = total
	}
	pageIDs := trendIDs[offset:end]

	trends, err := h.trendRepo.ListByIDs(ctx, pageIDs)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternal, "failed to fetch trends by IDs")
		return
	}

	response.OK(w, map[string]interface{}{
		"items": trends,
		"pagination": map[string]interface{}{
			"total":  total,
			"offset": offset,
			"limit":  limit,
		},
	})
}

// Get handles GET /trends/{id}.
func (h *TrendHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	id := r.PathValue("id")

	trend, err := h.trendRepo.GetByID(ctx, id)
	if err != nil {
		response.Error(w, http.StatusNotFound, response.CodeTrendNotFound, "trend ID not found")
		return
	}

	// Attempt to retrieve precomputed stats; a miss is not an error.
	stats, _ := h.statsRepo.GetByTrendID(ctx, id, h.activeStrategy)
	// stats may be nil when no stats have been computed yet.

	response.OK(w, map[string]interface{}{
		"trend": trend,
		"stats": stats,
	})
}

// Rising handles GET /trends/rising — returns top-K rising trends by score.
func (h *TrendHandler) Rising(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	q := r.URL.Query()

	limit, err := parseIntParam(q.Get("limit"), h.risingTopK)
	if err != nil || limit <= 0 || limit > h.maxLimit {
		response.Error(w, http.StatusBadRequest, response.CodeInvalidPagination,
			"limit must be a positive integer not exceeding "+strconv.Itoa(h.maxLimit))
		return
	}

	statsList, err := h.statsRepo.ListRising(ctx, h.activeStrategy, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternal, "failed to list rising stats")
		return
	}

	// Filter by minimum score threshold
	if h.risingMinScore > 0 {
		filtered := make([]*domain.TrendStats, 0, len(statsList))
		for _, s := range statsList {
			if s.Score >= h.risingMinScore {
				filtered = append(filtered, s)
			}
		}
		statsList = filtered
	}

	type risingItem struct {
		Trend interface{} `json:"trend"`
		Stats interface{} `json:"stats"`
	}

	items := make([]risingItem, 0, len(statsList))
	for _, stat := range statsList {
		trend, err := h.trendRepo.GetByID(ctx, stat.TrendID)
		if err != nil {
			// Orphaned stat (trend was deleted); skip silently.
			continue
		}
		items = append(items, risingItem{Trend: trend, Stats: stat})
	}

	response.OK(w, map[string]interface{}{
		"items": items,
		"pagination": map[string]interface{}{
			"total":       len(items),
			"limit":       limit,
			"strategy_id": h.activeStrategy,
		},
	})
}

// parseIntParam parses s as an integer, returning defaultVal when s is empty.
func parseIntParam(s string, defaultVal int) (int, error) {
	if s == "" {
		return defaultVal, nil
	}
	return strconv.Atoi(s)
}
