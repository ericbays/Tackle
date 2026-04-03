package report

import (
	"encoding/json"
	"strings"
)

func jsonToMap(data []byte) map[string]any {
	m := make(map[string]any)
	if len(data) > 0 {
		_ = json.Unmarshal(data, &m)
	}
	return m
}

func mapToJSON(m map[string]any) []byte {
	if m == nil {
		return []byte("{}")
	}
	b, _ := json.Marshal(m)
	return b
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// uuidArray converts a slice of UUID strings to a PostgreSQL UUID array literal.
func uuidArray(ids []string) string {
	if len(ids) == 0 {
		return "{}"
	}
	return "{" + strings.Join(ids, ",") + "}"
}

// parseUUIDArray parses a PostgreSQL UUID array text representation.
func parseUUIDArray(s string) []string {
	s = strings.Trim(s, "{}")
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
