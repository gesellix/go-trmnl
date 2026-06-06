package httpx_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gesellix/go-trmnl/internal/httpx"
)

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.WriteJSON(rec, 201, map[string]any{"hello": "world", "n": 3})

	if rec.Code != 201 {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["hello"] != "world" {
		t.Errorf("body = %v", body)
	}
}

func TestWriteJSONNilBody(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.WriteJSON(rec, 204, nil)
	if rec.Code != 204 {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("expected empty body, got %q", rec.Body.String())
	}
}

func TestWriteProblem(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.WriteProblem(rec, 404, "/problem#device_id", "not_found", "Invalid device ID.", "/api/display",
		map[string]any{"errors": map[string]any{"ID": []string{"is missing"}}})

	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("content-type = %q", ct)
	}
	var p httpx.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.Status != "not_found" || p.Detail != "Invalid device ID." || p.Instance != "/api/display" {
		t.Errorf("problem fields wrong: %+v", p)
	}
	if p.Extensions == nil {
		t.Errorf("extensions missing")
	}
}
