package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"trendpulse/internal/api/handler"
	"trendpulse/internal/api/response"
	"trendpulse/internal/domain"
	"trendpulse/internal/testutil"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func newTrendHandler(trendRepo *testutil.MockTrendRepo, statsRepo *testutil.MockStatsRepo) *handler.TrendHandler {
	return handler.NewTrendHandler(trendRepo, statsRepo, testutil.NewMockSignalRepo(), "momentum_v1", 20, 100, 20, 0.0)
}

func decodeResponse(t *testing.T, body []byte) response.Response {
	t.Helper()
	var resp response.Response
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to decode response body: %v\nbody: %s", err, body)
	}
	return resp
}

// ── TrendHandler.List ────────────────────────────────────────────────────────

func TestTrendHandler_List_ReturnsPaginatedTrends(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	statsRepo := testutil.NewMockStatsRepo()

	trends := []*domain.Trend{
		testutil.NewTrend("trend-001"),
		testutil.NewTrend("trend-002"),
		testutil.NewTrend("trend-003"),
	}
	trendRepo.SetTrends(trends)

	h := newTrendHandler(trendRepo, statsRepo)
	req := httptest.NewRequest(http.MethodGet, "/trends?offset=0&limit=2", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeOK {
		t.Fatalf("expected code 0, got %d: %s", resp.Code, resp.Message)
	}

	data := resp.Data.(map[string]interface{})
	items := data["items"].([]interface{})
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	pag := data["pagination"].(map[string]interface{})
	total := int(pag["total"].(float64))
	if total != 3 {
		t.Fatalf("expected total=3, got %d", total)
	}
}

func TestTrendHandler_List_DefaultPaginationApplied(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	statsRepo := testutil.NewMockStatsRepo()
	h := newTrendHandler(trendRepo, statsRepo)

	req := httptest.NewRequest(http.MethodGet, "/trends", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := decodeResponse(t, w.Body.Bytes())
	data := resp.Data.(map[string]interface{})
	pag := data["pagination"].(map[string]interface{})
	limit := int(pag["limit"].(float64))
	if limit != 20 {
		t.Fatalf("expected default limit=20, got %d", limit)
	}
}

func TestTrendHandler_List_NegativeOffset_Returns400(t *testing.T) {
	h := newTrendHandler(testutil.NewMockTrendRepo(), testutil.NewMockStatsRepo())
	req := httptest.NewRequest(http.MethodGet, "/trends?offset=-1", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeInvalidPagination {
		t.Fatalf("expected code %d, got %d", response.CodeInvalidPagination, resp.Code)
	}
}

func TestTrendHandler_List_ZeroLimit_Returns400(t *testing.T) {
	h := newTrendHandler(testutil.NewMockTrendRepo(), testutil.NewMockStatsRepo())
	req := httptest.NewRequest(http.MethodGet, "/trends?limit=0", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeInvalidPagination {
		t.Fatalf("expected code %d, got %d", response.CodeInvalidPagination, resp.Code)
	}
}

func TestTrendHandler_List_LimitExceedsMax_Returns400(t *testing.T) {
	h := newTrendHandler(testutil.NewMockTrendRepo(), testutil.NewMockStatsRepo())
	req := httptest.NewRequest(http.MethodGet, "/trends?limit=101", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeInvalidPagination {
		t.Fatalf("expected code %d, got %d", response.CodeInvalidPagination, resp.Code)
	}
}

func TestTrendHandler_List_InvalidPhase_Returns400(t *testing.T) {
	h := newTrendHandler(testutil.NewMockTrendRepo(), testutil.NewMockStatsRepo())
	req := httptest.NewRequest(http.MethodGet, "/trends?phase=invalid", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeInvalidRequest {
		t.Fatalf("expected code %d, got %d", response.CodeInvalidRequest, resp.Code)
	}
}

func TestTrendHandler_List_PhaseFilter_ReturnsMatchingTrends(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	statsRepo := testutil.NewMockStatsRepo()

	trends := []*domain.Trend{
		testutil.NewTrend("trend-001"),
		testutil.NewTrend("trend-002"),
		testutil.NewTrend("trend-003"),
	}
	trendRepo.SetTrends(trends)

	// Only trend-001 and trend-002 are in "rising" phase for this strategy.
	stat1 := testutil.NewTrendStats("momentum_v1", "trend-001", 80)
	stat1.Phase = "rising"
	stat2 := testutil.NewTrendStats("momentum_v1", "trend-002", 70)
	stat2.Phase = "rising"
	stat3 := testutil.NewTrendStats("momentum_v1", "trend-003", 60)
	stat3.Phase = "emerging"
	statsRepo.Upsert(nil, stat1) //nolint:errcheck
	statsRepo.Upsert(nil, stat2) //nolint:errcheck
	statsRepo.Upsert(nil, stat3) //nolint:errcheck

	h := newTrendHandler(trendRepo, statsRepo)
	req := httptest.NewRequest(http.MethodGet, "/trends?phase=rising", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w.Body.Bytes())
	data := resp.Data.(map[string]interface{})
	items := data["items"].([]interface{})
	if len(items) != 2 {
		t.Fatalf("expected 2 items with phase=rising, got %d", len(items))
	}
}

func TestTrendHandler_List_ValidPhaseValues_Return200(t *testing.T) {
	validPhases := []string{"emerging", "rising", "peaking", "declining"}
	for _, phase := range validPhases {
		t.Run(phase, func(t *testing.T) {
			trendRepo := testutil.NewMockTrendRepo()
			statsRepo := testutil.NewMockStatsRepo()
			h := newTrendHandler(trendRepo, statsRepo)
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/trends?phase=%s", phase), nil)
			w := httptest.NewRecorder()
			h.List(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("phase=%s: expected 200, got %d: %s", phase, w.Code, w.Body.String())
			}
		})
	}
}

// ── TrendHandler.Get ─────────────────────────────────────────────────────────

func TestTrendHandler_Get_ReturnsTrendWithStats(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	statsRepo := testutil.NewMockStatsRepo()

	trendRepo.SetTrends([]*domain.Trend{testutil.NewTrend("trend-001")})
	stat := testutil.NewTrendStats("momentum_v1", "trend-001", 85)
	statsRepo.Upsert(nil, stat) //nolint:errcheck

	h := newTrendHandler(trendRepo, statsRepo)

	// Simulate Go 1.22 ServeMux path value via httptest manually.
	req := httptest.NewRequest(http.MethodGet, "/trends/trend-001", nil)
	req.SetPathValue("id", "trend-001")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeOK {
		t.Fatalf("expected code 0, got %d: %s", resp.Code, resp.Message)
	}
	data := resp.Data.(map[string]interface{})
	if _, ok := data["trend"]; !ok {
		t.Fatal("expected 'trend' key in data")
	}
	if _, ok := data["stats"]; !ok {
		t.Fatal("expected 'stats' key in data")
	}
	if _, ok := data["signals"]; !ok {
		t.Fatal("expected 'signals' key in data")
	}
}

func TestTrendHandler_Get_TrendNotFound_Returns404(t *testing.T) {
	h := newTrendHandler(testutil.NewMockTrendRepo(), testutil.NewMockStatsRepo())
	req := httptest.NewRequest(http.MethodGet, "/trends/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeTrendNotFound {
		t.Fatalf("expected code %d, got %d", response.CodeTrendNotFound, resp.Code)
	}
}

func TestTrendHandler_Get_TrendExistsButNoStats_ReturnsNullStats(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	trendRepo.SetTrends([]*domain.Trend{testutil.NewTrend("trend-001")})

	h := newTrendHandler(trendRepo, testutil.NewMockStatsRepo())
	req := httptest.NewRequest(http.MethodGet, "/trends/trend-001", nil)
	req.SetPathValue("id", "trend-001")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w.Body.Bytes())
	data := resp.Data.(map[string]interface{})
	if data["stats"] != nil {
		t.Fatalf("expected stats=null, got %v", data["stats"])
	}
	if _, ok := data["signals"]; !ok {
		t.Fatal("expected 'signals' key in data")
	}
}

// ── TrendHandler.Rising ──────────────────────────────────────────────────────

func TestTrendHandler_Rising_ReturnsTopKTrends(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	statsRepo := testutil.NewMockStatsRepo()

	for i := 1; i <= 5; i++ {
		id := fmt.Sprintf("trend-%03d", i)
		trendRepo.SetTrends(append(func() []*domain.Trend {
			all := make([]*domain.Trend, 0)
			for j := 1; j <= 5; j++ {
				all = append(all, testutil.NewTrend(fmt.Sprintf("trend-%03d", j)))
			}
			return all
		}(), testutil.NewTrend(id)))
		stat := testutil.NewTrendStats("momentum_v1", id, float64(i*10))
		statsRepo.Upsert(nil, stat) //nolint:errcheck
	}

	// Re-seed properly
	trendRepo2 := testutil.NewMockTrendRepo()
	statsRepo2 := testutil.NewMockStatsRepo()
	trends := make([]*domain.Trend, 5)
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("trend-%03d", i+1)
		trends[i] = testutil.NewTrend(id)
		stat := testutil.NewTrendStats("momentum_v1", id, float64((i+1)*10))
		statsRepo2.Upsert(nil, stat) //nolint:errcheck
	}
	trendRepo2.SetTrends(trends)

	h := newTrendHandler(trendRepo2, statsRepo2)
	req := httptest.NewRequest(http.MethodGet, "/trends/rising?limit=3", nil)
	w := httptest.NewRecorder()
	h.Rising(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeOK {
		t.Fatalf("expected code 0, got %d: %s", resp.Code, resp.Message)
	}

	data := resp.Data.(map[string]interface{})
	items := data["items"].([]interface{})
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	pag := data["pagination"].(map[string]interface{})
	strategyID, ok := pag["strategy_id"]
	if !ok {
		t.Fatal("expected 'strategy_id' in pagination")
	}
	if strategyID != "momentum_v1" {
		t.Fatalf("expected strategy_id='momentum_v1', got %v", strategyID)
	}
}

func TestTrendHandler_Rising_InvalidLimit_Returns400(t *testing.T) {
	h := newTrendHandler(testutil.NewMockTrendRepo(), testutil.NewMockStatsRepo())
	req := httptest.NewRequest(http.MethodGet, "/trends/rising?limit=0", nil)
	w := httptest.NewRecorder()
	h.Rising(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeInvalidPagination {
		t.Fatalf("expected code %d, got %d", response.CodeInvalidPagination, resp.Code)
	}
}

func TestTrendHandler_Rising_LimitExceedsMax_Returns400(t *testing.T) {
	h := newTrendHandler(testutil.NewMockTrendRepo(), testutil.NewMockStatsRepo())
	req := httptest.NewRequest(http.MethodGet, "/trends/rising?limit=101", nil)
	w := httptest.NewRecorder()
	h.Rising(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeInvalidPagination {
		t.Fatalf("expected code %d, got %d", response.CodeInvalidPagination, resp.Code)
	}
}

func TestTrendHandler_Rising_DefaultLimitApplied(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	statsRepo := testutil.NewMockStatsRepo()
	h := newTrendHandler(trendRepo, statsRepo)

	req := httptest.NewRequest(http.MethodGet, "/trends/rising", nil)
	w := httptest.NewRecorder()
	h.Rising(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w.Body.Bytes())
	data := resp.Data.(map[string]interface{})
	pag := data["pagination"].(map[string]interface{})
	limit := int(pag["limit"].(float64))
	if limit != 20 {
		t.Fatalf("expected default limit=20, got %d", limit)
	}
}

func TestTrendHandler_Rising_SkipsMissingTrends(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	statsRepo := testutil.NewMockStatsRepo()

	// Add trend only for trend-001; trend-002 has stats but no trend doc
	trendRepo.SetTrends([]*domain.Trend{testutil.NewTrend("trend-001")})

	stat1 := testutil.NewTrendStats("momentum_v1", "trend-001", 90)
	stat2 := testutil.NewTrendStats("momentum_v1", "trend-002", 80) // no matching trend
	statsRepo.Upsert(nil, stat1)                                    //nolint:errcheck
	statsRepo.Upsert(nil, stat2)                                    //nolint:errcheck

	h := newTrendHandler(trendRepo, statsRepo)
	req := httptest.NewRequest(http.MethodGet, "/trends/rising?limit=10", nil)
	w := httptest.NewRecorder()
	h.Rising(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w.Body.Bytes())
	data := resp.Data.(map[string]interface{})
	items := data["items"].([]interface{})
	// Only trend-001 should appear; trend-002 silently skipped
	if len(items) != 1 {
		t.Fatalf("expected 1 item (orphan stat skipped), got %d", len(items))
	}
}

func TestTrendHandler_Rising_EachItemHasTrendAndStats(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	statsRepo := testutil.NewMockStatsRepo()

	trendRepo.SetTrends([]*domain.Trend{testutil.NewTrend("trend-001")})
	stat := testutil.NewTrendStats("momentum_v1", "trend-001", 77)
	statsRepo.Upsert(nil, stat) //nolint:errcheck

	h := newTrendHandler(trendRepo, statsRepo)
	req := httptest.NewRequest(http.MethodGet, "/trends/rising", nil)
	w := httptest.NewRecorder()
	h.Rising(w, req)

	resp := decodeResponse(t, w.Body.Bytes())
	data := resp.Data.(map[string]interface{})
	items := data["items"].([]interface{})
	if len(items) == 0 {
		t.Fatal("expected at least one item")
	}
	item := items[0].(map[string]interface{})
	if _, ok := item["trend"]; !ok {
		t.Fatal("expected 'trend' key in item")
	}
	if _, ok := item["stats"]; !ok {
		t.Fatal("expected 'stats' key in item")
	}
}

// Ensure the handler uses the configured activeStrategy, not a hard-coded one.
func TestTrendHandler_Rising_UsesActiveStrategy(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	statsRepo := testutil.NewMockStatsRepo()

	trendRepo.SetTrends([]*domain.Trend{testutil.NewTrend("trend-001")})
	// Stats stored under a different strategy — should NOT appear.
	stat := testutil.NewTrendStats("other_strategy", "trend-001", 99)
	statsRepo.Upsert(nil, stat) //nolint:errcheck

	h := newTrendHandler(trendRepo, statsRepo)
	req := httptest.NewRequest(http.MethodGet, "/trends/rising", nil)
	w := httptest.NewRecorder()
	h.Rising(w, req)

	resp := decodeResponse(t, w.Body.Bytes())
	data := resp.Data.(map[string]interface{})
	items := data["items"].([]interface{})
	if len(items) != 0 {
		t.Fatalf("expected 0 items (wrong strategy), got %d", len(items))
	}
}

func TestTrendHandler_Rising_FiltersLowScores(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	statsRepo := testutil.NewMockStatsRepo()

	trendRepo.SetTrends([]*domain.Trend{
		testutil.NewTrend("trend-001"),
		testutil.NewTrend("trend-002"),
		testutil.NewTrend("trend-003"),
	})

	// Create stats with varying scores
	stat1 := testutil.NewTrendStats("momentum_v1", "trend-001", 85.0)
	stat2 := testutil.NewTrendStats("momentum_v1", "trend-002", 40.0)
	stat3 := testutil.NewTrendStats("momentum_v1", "trend-003", 70.0)
	statsRepo.Upsert(nil, stat1) //nolint:errcheck
	statsRepo.Upsert(nil, stat2) //nolint:errcheck
	statsRepo.Upsert(nil, stat3) //nolint:errcheck

	// Use min score of 60
	h := handler.NewTrendHandler(trendRepo, statsRepo, testutil.NewMockSignalRepo(), "momentum_v1", 20, 100, 20, 60.0)
	req := httptest.NewRequest(http.MethodGet, "/trends/rising", nil)
	w := httptest.NewRecorder()
	h.Rising(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := decodeResponse(t, w.Body.Bytes())
	data := resp.Data.(map[string]interface{})
	items := data["items"].([]interface{})
	// Only trend-001 (85) and trend-003 (70) should appear
	if len(items) != 2 {
		t.Fatalf("expected 2 items above threshold, got %d", len(items))
	}
}

// ── ensure time is importable (avoids unused-import lint) ────────────────────
var _ = time.Now

// TestTrendHandler_Get_ReturnsSignals verifies that signals associated with a
// trend are returned in the "signals" field of the GET /trends/{id} response.
func TestTrendHandler_Get_ReturnsSignals(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	statsRepo := testutil.NewMockStatsRepo()
	signalRepo := testutil.NewMockSignalRepo()

	trendRepo.SetTrends([]*domain.Trend{testutil.NewTrend("trend-001")})

	// Add two signals for the trend.
	ts1 := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 4, 15, 11, 0, 0, 0, time.UTC)
	signalRepo.AddSignal(testutil.NewSignal("trend-001", ts1, 1000))
	signalRepo.AddSignal(testutil.NewSignal("trend-001", ts2, 1200))

	h := handler.NewTrendHandler(trendRepo, statsRepo, signalRepo, "momentum_v1", 20, 100, 20, 0.0)

	req := httptest.NewRequest(http.MethodGet, "/trends/trend-001", nil)
	req.SetPathValue("id", "trend-001")
	w := httptest.NewRecorder()
	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeOK {
		t.Fatalf("expected code 0, got %d: %s", resp.Code, resp.Message)
	}

	data := resp.Data.(map[string]interface{})
	rawSignals, ok := data["signals"]
	if !ok {
		t.Fatal("expected 'signals' key in data")
	}
	signals, ok := rawSignals.([]interface{})
	if !ok {
		t.Fatalf("expected signals to be an array, got %T", rawSignals)
	}
	if len(signals) != 2 {
		t.Fatalf("expected 2 signals, got %d", len(signals))
	}
}
