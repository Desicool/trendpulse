package api

import (
	"encoding/json"
	"net/http"
	"time"

	"trendpulse/internal/api/handler"
	"trendpulse/internal/api/middleware"
)

// Triggerable is the interface the router uses to trigger manual scheduler runs.
// It is satisfied by *scheduler.Scheduler.
type Triggerable interface {
	TriggerNow(asOf time.Time)
}

// NewRouter builds and returns the HTTP handler for the TrendPulse API.
//
// Route registration order is critical:
//
//	GET /trends/rising  must be registered before GET /trends/{id}
//	so that "rising" is never mistaken for a path parameter.
func NewRouter(
	trendHandler *handler.TrendHandler,
	ingestHandler *handler.IngestHandler,
	triggerable Triggerable,
) http.Handler {
	mux := http.NewServeMux()

	// Trend endpoints — order matters: /rising before /{id}
	mux.HandleFunc("GET /trends/rising", trendHandler.Rising)
	mux.HandleFunc("GET /trends/{id}", trendHandler.Get)
	mux.HandleFunc("GET /trends", trendHandler.List)

	// Ingest endpoints
	mux.HandleFunc("POST /ingest/trends", ingestHandler.IngestTrend)
	mux.HandleFunc("POST /ingest/signals", ingestHandler.IngestSignal)

	// Internal scheduler trigger
	mux.HandleFunc("POST /internal/scheduler/trigger", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			AsOf string `json:"as_of"`
		}
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck

		asOf := time.Time{}
		if body.AsOf != "" {
			if t, err := time.Parse(time.RFC3339, body.AsOf); err == nil {
				asOf = t
			}
		}

		triggerable.TriggerNow(asOf)
		w.WriteHeader(http.StatusOK)
	})

	return middleware.Logging(mux)
}
