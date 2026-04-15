package response

import (
	"encoding/json"
	"net/http"
)

// Business-level code constants returned in the "code" field of every response.
const (
	CodeOK                  = 0
	CodeInvalidRequest      = 40001
	CodeInvalidPagination   = 40002
	CodeTrendNotFound       = 40401
	CodeStatsNotAvailable   = 40402
	CodeTrendAlreadyExists  = 40901
	CodeMissingRequiredField = 42201
	CodeInternal            = 50001
)

// Response is the unified envelope for every HTTP response.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// Pagination carries offset-based pagination metadata.
type Pagination struct {
	Total  int `json:"total"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

// JSON writes r as JSON with the given HTTP status code.
func JSON(w http.ResponseWriter, httpStatus int, code int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	resp := Response{
		Code:    code,
		Message: message,
		Data:    data,
	}
	// Best-effort encode; if the connection is broken there is nothing to do.
	_ = json.NewEncoder(w).Encode(resp)
}

// OK writes a 200 success response.
func OK(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusOK, CodeOK, "ok", data)
}

// Created writes a 201 success response.
func Created(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusCreated, CodeOK, "ok", data)
}

// Error writes an error response with the supplied HTTP status and business code.
func Error(w http.ResponseWriter, httpStatus int, code int, message string) {
	JSON(w, httpStatus, code, message, nil)
}
