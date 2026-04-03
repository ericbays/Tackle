// Package validator provides input validation helpers for the Tackle API.
package validator

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// MaxBodyBytes is the maximum allowed request body size (1 MB).
const MaxBodyBytes = 1 << 20

// DecodeJSON reads and decodes a JSON request body into dst.
// It enforces a maximum body size and rejects unknown fields.
func DecodeJSON(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, MaxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}

	// Ensure there is no trailing garbage after the first JSON value.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("request body must contain only a single JSON value")
	}

	return nil
}
