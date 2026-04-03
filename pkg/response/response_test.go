package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"tackle/pkg/response"
)

func TestSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	response.Success(w, map[string]string{"key": "value"})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body struct {
		Data map[string]string `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data["key"] != "value" {
		t.Errorf("unexpected data: %v", body.Data)
	}
}

func TestCreated(t *testing.T) {
	w := httptest.NewRecorder()
	response.Created(w, map[string]string{"id": "123"})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var body struct {
		Data map[string]string `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data["id"] != "123" {
		t.Errorf("unexpected data: %v", body.Data)
	}
}

func TestList(t *testing.T) {
	w := httptest.NewRecorder()
	response.List(w, []string{"a", "b"}, response.Pagination{Page: 1, PerPage: 10, Total: 2, TotalPages: 1})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body struct {
		Data       []string           `json:"data"`
		Pagination response.Pagination `json:"pagination"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data) != 2 {
		t.Errorf("expected 2 items, got %d", len(body.Data))
	}
	if body.Pagination.Total != 2 {
		t.Errorf("expected total=2, got %d", body.Pagination.Total)
	}
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()
	response.Error(w, "NOT_FOUND", "resource not found", http.StatusNotFound, "corr-123")

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	var body struct {
		Error struct {
			Code          string `json:"code"`
			Message       string `json:"message"`
			CorrelationID string `json:"correlation_id"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "NOT_FOUND" {
		t.Errorf("unexpected code: %q", body.Error.Code)
	}
	if body.Error.CorrelationID != "corr-123" {
		t.Errorf("unexpected correlation_id: %q", body.Error.CorrelationID)
	}
}
