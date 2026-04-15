package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"trendpulse/internal/api/handler"
	"trendpulse/internal/api/response"
	"trendpulse/internal/domain"
	"trendpulse/internal/testutil"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newIngestHandler(trendRepo *testutil.MockTrendRepo, signalRepo *testutil.MockSignalRepo, catIdx *testutil.MockCategoryIndex) *handler.IngestHandler {
	return handler.NewIngestHandler(trendRepo, signalRepo, catIdx)
}

func postJSON(t *testing.T, h http.HandlerFunc, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		t.Fatalf("encode request body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h(w, req)
	return w
}

// seedTrend is a thin helper that adds a single trend to a MockTrendRepo.
func seedTrend(repo *testutil.MockTrendRepo, id string) {
	repo.SetTrends([]*domain.Trend{testutil.NewTrend(id)})
}

// ── IngestHandler.IngestTrend ─────────────────────────────────────────────────

func TestIngestHandler_IngestTrend_ValidRequest_Returns201(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	h := newIngestHandler(trendRepo, testutil.NewMockSignalRepo(), testutil.NewMockCategoryIndex())

	w := postJSON(t, h.IngestTrend, "/ingest/trends", map[string]interface{}{
		"id":          "trend-001",
		"name":        "AI Painting Challenge",
		"description": "Users generate AI art",
		"categories":  []string{"art", "technology"},
		"source":      "tiktok",
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeOK {
		t.Fatalf("expected code 0, got %d: %s", resp.Code, resp.Message)
	}
}

func TestIngestHandler_IngestTrend_MissingID_Returns422(t *testing.T) {
	h := newIngestHandler(testutil.NewMockTrendRepo(), testutil.NewMockSignalRepo(), testutil.NewMockCategoryIndex())
	w := postJSON(t, h.IngestTrend, "/ingest/trends", map[string]interface{}{
		"name":   "No ID Trend",
		"source": "tiktok",
	})

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeMissingRequiredField {
		t.Fatalf("expected code %d, got %d", response.CodeMissingRequiredField, resp.Code)
	}
}

func TestIngestHandler_IngestTrend_MissingName_Returns422(t *testing.T) {
	h := newIngestHandler(testutil.NewMockTrendRepo(), testutil.NewMockSignalRepo(), testutil.NewMockCategoryIndex())
	w := postJSON(t, h.IngestTrend, "/ingest/trends", map[string]interface{}{
		"id":     "trend-001",
		"source": "tiktok",
	})

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeMissingRequiredField {
		t.Fatalf("expected code %d, got %d", response.CodeMissingRequiredField, resp.Code)
	}
}

func TestIngestHandler_IngestTrend_MissingSource_Returns422(t *testing.T) {
	h := newIngestHandler(testutil.NewMockTrendRepo(), testutil.NewMockSignalRepo(), testutil.NewMockCategoryIndex())
	w := postJSON(t, h.IngestTrend, "/ingest/trends", map[string]interface{}{
		"id":   "trend-001",
		"name": "Valid Name",
	})

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeMissingRequiredField {
		t.Fatalf("expected code %d, got %d", response.CodeMissingRequiredField, resp.Code)
	}
}

func TestIngestHandler_IngestTrend_BadJSON_Returns400(t *testing.T) {
	h := newIngestHandler(testutil.NewMockTrendRepo(), testutil.NewMockSignalRepo(), testutil.NewMockCategoryIndex())
	req := httptest.NewRequest(http.MethodPost, "/ingest/trends", bytes.NewBufferString("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.IngestTrend(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeInvalidRequest {
		t.Fatalf("expected code %d, got %d", response.CodeInvalidRequest, resp.Code)
	}
}

func TestIngestHandler_IngestTrend_DuplicateID_Returns409(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	h := newIngestHandler(trendRepo, testutil.NewMockSignalRepo(), testutil.NewMockCategoryIndex())

	body := map[string]interface{}{
		"id":     "trend-dup",
		"name":   "Dup Trend",
		"source": "tiktok",
	}

	// First insert succeeds.
	w1 := postJSON(t, h.IngestTrend, "/ingest/trends", body)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first insert: expected 201, got %d: %s", w1.Code, w1.Body.String())
	}

	// Second insert conflicts.
	w2 := postJSON(t, h.IngestTrend, "/ingest/trends", body)
	if w2.Code != http.StatusConflict {
		t.Fatalf("second insert: expected 409, got %d: %s", w2.Code, w2.Body.String())
	}
	resp := decodeResponse(t, w2.Body.Bytes())
	if resp.Code != response.CodeTrendAlreadyExists {
		t.Fatalf("expected code %d, got %d", response.CodeTrendAlreadyExists, resp.Code)
	}
}

func TestIngestHandler_IngestTrend_ResponseContainsTimestamps(t *testing.T) {
	h := newIngestHandler(testutil.NewMockTrendRepo(), testutil.NewMockSignalRepo(), testutil.NewMockCategoryIndex())
	w := postJSON(t, h.IngestTrend, "/ingest/trends", map[string]interface{}{
		"id":     "trend-001",
		"name":   "Some Trend",
		"source": "youtube",
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var full map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&full); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := full["data"].(map[string]interface{})

	if _, ok := data["created_at"]; !ok {
		t.Fatal("expected 'created_at' in response data")
	}
	if _, ok := data["updated_at"]; !ok {
		t.Fatal("expected 'updated_at' in response data")
	}
}

// ── IngestHandler.IngestSignal ────────────────────────────────────────────────

func TestIngestHandler_IngestSignal_ValidRequest_Returns201(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	seedTrend(trendRepo, "trend-001")

	h := newIngestHandler(trendRepo, testutil.NewMockSignalRepo(), testutil.NewMockCategoryIndex())
	w := postJSON(t, h.IngestSignal, "/ingest/signals", map[string]interface{}{
		"trend_id":           "trend-001",
		"timestamp":          "2026-04-15T10:00:00Z",
		"usage_count":        12345,
		"unique_creators":    890,
		"avg_views":          5600.0,
		"avg_engagement":     340.5,
		"view_concentration": 0.72,
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeOK {
		t.Fatalf("expected code 0, got %d: %s", resp.Code, resp.Message)
	}
}

func TestIngestHandler_IngestSignal_MissingTrendID_Returns422(t *testing.T) {
	h := newIngestHandler(testutil.NewMockTrendRepo(), testutil.NewMockSignalRepo(), testutil.NewMockCategoryIndex())
	w := postJSON(t, h.IngestSignal, "/ingest/signals", map[string]interface{}{
		"timestamp":       "2026-04-15T10:00:00Z",
		"usage_count":     100,
		"unique_creators": 10,
		"avg_views":       500.0,
	})

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeMissingRequiredField {
		t.Fatalf("expected code %d, got %d", response.CodeMissingRequiredField, resp.Code)
	}
}

func TestIngestHandler_IngestSignal_MissingTimestamp_Returns422(t *testing.T) {
	h := newIngestHandler(testutil.NewMockTrendRepo(), testutil.NewMockSignalRepo(), testutil.NewMockCategoryIndex())
	w := postJSON(t, h.IngestSignal, "/ingest/signals", map[string]interface{}{
		"trend_id":        "trend-001",
		"usage_count":     100,
		"unique_creators": 10,
		"avg_views":       500.0,
	})

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIngestHandler_IngestSignal_InvalidTimestampFormat_Returns422(t *testing.T) {
	h := newIngestHandler(testutil.NewMockTrendRepo(), testutil.NewMockSignalRepo(), testutil.NewMockCategoryIndex())
	w := postJSON(t, h.IngestSignal, "/ingest/signals", map[string]interface{}{
		"trend_id":        "trend-001",
		"timestamp":       "not-a-date",
		"usage_count":     100,
		"unique_creators": 10,
		"avg_views":       500.0,
	})

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestIngestHandler_IngestSignal_TrendNotFound_Returns404(t *testing.T) {
	h := newIngestHandler(testutil.NewMockTrendRepo(), testutil.NewMockSignalRepo(), testutil.NewMockCategoryIndex())
	w := postJSON(t, h.IngestSignal, "/ingest/signals", map[string]interface{}{
		"trend_id":        "nonexistent",
		"timestamp":       "2026-04-15T10:00:00Z",
		"usage_count":     100,
		"unique_creators": 10,
		"avg_views":       500.0,
	})

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeTrendNotFound {
		t.Fatalf("expected code %d, got %d", response.CodeTrendNotFound, resp.Code)
	}
}

func TestIngestHandler_IngestSignal_NegativeUsageCount_Returns400(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	seedTrend(trendRepo, "trend-001")

	h := newIngestHandler(trendRepo, testutil.NewMockSignalRepo(), testutil.NewMockCategoryIndex())
	w := postJSON(t, h.IngestSignal, "/ingest/signals", map[string]interface{}{
		"trend_id":        "trend-001",
		"timestamp":       "2026-04-15T10:00:00Z",
		"usage_count":     -1,
		"unique_creators": 10,
		"avg_views":       500.0,
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeInvalidRequest {
		t.Fatalf("expected code %d, got %d", response.CodeInvalidRequest, resp.Code)
	}
}

func TestIngestHandler_IngestSignal_NegativeAvgEngagement_Returns400(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	seedTrend(trendRepo, "trend-001")

	h := newIngestHandler(trendRepo, testutil.NewMockSignalRepo(), testutil.NewMockCategoryIndex())
	w := postJSON(t, h.IngestSignal, "/ingest/signals", map[string]interface{}{
		"trend_id":        "trend-001",
		"timestamp":       "2026-04-15T10:00:00Z",
		"usage_count":     100,
		"unique_creators": 10,
		"avg_views":       500.0,
		"avg_engagement":  -5.0,
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeInvalidRequest {
		t.Fatalf("expected code %d, got %d", response.CodeInvalidRequest, resp.Code)
	}
}

func TestIngestHandler_IngestSignal_ViewConcentrationOutOfRange_Returns400(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	seedTrend(trendRepo, "trend-001")

	h := newIngestHandler(trendRepo, testutil.NewMockSignalRepo(), testutil.NewMockCategoryIndex())
	w := postJSON(t, h.IngestSignal, "/ingest/signals", map[string]interface{}{
		"trend_id":           "trend-001",
		"timestamp":          "2026-04-15T10:00:00Z",
		"usage_count":        100,
		"unique_creators":    10,
		"avg_views":          500.0,
		"view_concentration": 1.5,
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w.Body.Bytes())
	if resp.Code != response.CodeInvalidRequest {
		t.Fatalf("expected code %d, got %d", response.CodeInvalidRequest, resp.Code)
	}
}

func TestIngestHandler_IngestSignal_BadJSON_Returns400(t *testing.T) {
	h := newIngestHandler(testutil.NewMockTrendRepo(), testutil.NewMockSignalRepo(), testutil.NewMockCategoryIndex())
	req := httptest.NewRequest(http.MethodPost, "/ingest/signals", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.IngestSignal(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestIngestHandler_IngestSignal_ResponseContainsID(t *testing.T) {
	trendRepo := testutil.NewMockTrendRepo()
	seedTrend(trendRepo, "trend-001")

	h := newIngestHandler(trendRepo, testutil.NewMockSignalRepo(), testutil.NewMockCategoryIndex())
	w := postJSON(t, h.IngestSignal, "/ingest/signals", map[string]interface{}{
		"trend_id":        "trend-001",
		"timestamp":       "2026-04-15T10:00:00Z",
		"usage_count":     100,
		"unique_creators": 10,
		"avg_views":       500.0,
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var full map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&full); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := full["data"].(map[string]interface{})
	idVal, ok := data["id"]
	if !ok {
		t.Fatal("expected 'id' in response data")
	}
	if idStr, _ := idVal.(string); idStr == "" {
		t.Fatal("expected non-empty id in response data")
	}
}
