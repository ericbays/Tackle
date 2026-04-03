package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"tackle/internal/handlers"
)

func TestHealth(t *testing.T) {
	t.Run("returns 200 with correct JSON structure", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		w := httptest.NewRecorder()

		handlers.Health("test-version").ServeHTTP(w, req)

		resp := w.Result()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var body struct {
			Data struct {
				Status  string `json:"status"`
				Version string `json:"version"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}

		if body.Data.Status != "ok" {
			t.Errorf("expected status 'ok', got %q", body.Data.Status)
		}
		if body.Data.Version != "test-version" {
			t.Errorf("expected version 'test-version', got %q", body.Data.Version)
		}
	})
}
