package http

import (
	"encoding/json"
	"net/http"
)

// A single, predictable JSON error shape for the whole API:
//
//	{ "error": { "code": "not_found", "message": "task not found" } }
//
// Course mapping: Chapter 12 — Error handling (one shape, everywhere).
type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	// Fields carries per-field detail for validation failures (422). It is
	// omitted for every other error so the common shape stays unchanged.
	Fields []fieldError `json:"fields,omitempty" nullable:"false"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{Error: errorBody{Code: code, Message: message}})
}
