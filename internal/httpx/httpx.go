// Package httpx holds small HTTP helpers shared across handlers: JSON writing
// and RFC 9457 (problem details) error responses.
package httpx

import (
	"encoding/json"
	"log"
	"net/http"
)

// WriteJSON writes v as a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("httpx: encode json: %v", err)
	}
}

// Problem is an RFC 9457 problem-details object. The TRMNL reference server
// returns errors in this shape.
type Problem struct {
	Type       string         `json:"type"`
	Status     string         `json:"status"`
	Detail     string         `json:"detail"`
	Instance   string         `json:"instance"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

// WriteProblem writes an RFC 9457 problem-details response. statusCode is the
// HTTP status; statusText mirrors it in the body (e.g. "not_found").
func WriteProblem(w http.ResponseWriter, statusCode int, problemType, statusText, detail, instance string, ext map[string]any) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(statusCode)
	//nolint:errchkjson // Problem.Extensions is a free-form map; encode errors here are unactionable.
	_ = json.NewEncoder(w).Encode(Problem{
		Type:       problemType,
		Status:     statusText,
		Detail:     detail,
		Instance:   instance,
		Extensions: ext,
	})
}
