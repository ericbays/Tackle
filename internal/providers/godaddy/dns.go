package godaddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	dnsiface "tackle/internal/providers/dns"
)

// gdRecord is the GoDaddy REST API record format.
type gdRecord struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Data     string `json:"data"`
	TTL      int    `json:"ttl"`
	Priority int    `json:"priority,omitempty"`
}

// ListRecords returns all DNS records for the zone via GET /v1/domains/{domain}/records.
func (c *Client) ListRecords(ctx context.Context, zone string) ([]dnsiface.Record, error) {
	c.rateLimiter.Wait()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+fmt.Sprintf("/v1/domains/%s/records", zone), nil)
	if err != nil {
		return nil, fmt.Errorf("godaddy dns list: build request: %w", err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Accept", "application/json")

	body, status, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("godaddy dns list: %w", err)
	}

	if status == http.StatusNotFound {
		return nil, dnsiface.ErrZoneNotFound
	}
	if status == http.StatusTooManyRequests {
		return nil, dnsiface.ErrAPIRateLimit
	}
	if status != http.StatusOK {
		return nil, c.translateError(status, body)
	}

	var gdrecs []gdRecord
	if err := json.Unmarshal(body, &gdrecs); err != nil {
		return nil, fmt.Errorf("godaddy dns list: parse response: %w", err)
	}

	return gdRecordsToRecords(gdrecs), nil
}

// CreateRecord adds a new DNS record via PATCH /v1/domains/{domain}/records/{type}/{name}.
func (c *Client) CreateRecord(ctx context.Context, zone string, record dnsiface.Record) (dnsiface.Record, error) {
	c.rateLimiter.Wait()

	gdr := recordToGD(record)
	payload, err := json.Marshal([]gdRecord{gdr})
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("godaddy dns create: marshal: %w", err)
	}

	path := fmt.Sprintf("/v1/domains/%s/records/%s/%s", zone, string(record.Type), record.Name)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("godaddy dns create: build request: %w", err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	body, status, err := c.doRequest(req)
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("godaddy dns create: %w", err)
	}

	if status == http.StatusNotFound {
		return dnsiface.Record{}, dnsiface.ErrZoneNotFound
	}
	if status == http.StatusConflict {
		return dnsiface.Record{}, dnsiface.ErrRecordConflict
	}
	if status == http.StatusTooManyRequests {
		return dnsiface.Record{}, dnsiface.ErrAPIRateLimit
	}
	if status != http.StatusOK && status != http.StatusNoContent {
		return dnsiface.Record{}, c.translateError(status, body)
	}

	record.ID = gdRecordID(record)
	return record, nil
}

// UpdateRecord replaces all records of the given type+name via
// PUT /v1/domains/{domain}/records/{type}/{name}.
// recordID is expected in "TYPE/name" format.
func (c *Client) UpdateRecord(ctx context.Context, zone string, recordID string, record dnsiface.Record) (dnsiface.Record, error) {
	c.rateLimiter.Wait()

	rType, rName, err := parseGDRecordID(recordID)
	if err != nil {
		return dnsiface.Record{}, err
	}

	gdr := recordToGD(record)
	payload, err := json.Marshal([]gdRecord{gdr})
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("godaddy dns update: marshal: %w", err)
	}

	path := fmt.Sprintf("/v1/domains/%s/records/%s/%s", zone, rType, rName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("godaddy dns update: build request: %w", err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	body, status, err := c.doRequest(req)
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("godaddy dns update: %w", err)
	}

	if status == http.StatusNotFound {
		return dnsiface.Record{}, dnsiface.ErrRecordNotFound
	}
	if status == http.StatusTooManyRequests {
		return dnsiface.Record{}, dnsiface.ErrAPIRateLimit
	}
	if status != http.StatusOK && status != http.StatusNoContent {
		return dnsiface.Record{}, c.translateError(status, body)
	}

	record.ID = gdRecordID(record)
	return record, nil
}

// DeleteRecord removes a record by type+name via
// DELETE /v1/domains/{domain}/records/{type}/{name}.
// recordID is expected in "TYPE/name" format.
func (c *Client) DeleteRecord(ctx context.Context, zone string, recordID string) error {
	c.rateLimiter.Wait()

	rType, rName, err := parseGDRecordID(recordID)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/v1/domains/%s/records/%s/%s", zone, rType, rName)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("godaddy dns delete: build request: %w", err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Accept", "application/json")

	body, status, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("godaddy dns delete: %w", err)
	}

	if status == http.StatusNotFound {
		return dnsiface.ErrRecordNotFound
	}
	if status == http.StatusTooManyRequests {
		return dnsiface.ErrAPIRateLimit
	}
	if status != http.StatusNoContent && status != http.StatusOK {
		return c.translateError(status, body)
	}
	return nil
}

// GetSOA fetches the SOA record for the zone by querying the SOA type.
func (c *Client) GetSOA(ctx context.Context, zone string) (dnsiface.Record, error) {
	c.rateLimiter.Wait()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+fmt.Sprintf("/v1/domains/%s/records/SOA/@", zone), nil)
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("godaddy dns get soa: build request: %w", err)
	}
	c.setAuthHeader(req)
	req.Header.Set("Accept", "application/json")

	body, status, err := c.doRequest(req)
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("godaddy dns get soa: %w", err)
	}

	if status == http.StatusNotFound {
		return dnsiface.Record{}, dnsiface.ErrZoneNotFound
	}
	if status != http.StatusOK {
		return dnsiface.Record{}, c.translateError(status, body)
	}

	var gdrecs []gdRecord
	if err := json.Unmarshal(body, &gdrecs); err != nil || len(gdrecs) == 0 {
		return dnsiface.Record{Type: dnsiface.RecordTypeSOA, Name: "@", Value: zone}, nil
	}

	r := gdrecs[0]
	return dnsiface.Record{
		ID:    gdRecordID(dnsiface.Record{Type: dnsiface.RecordTypeSOA, Name: r.Name}),
		Type:  dnsiface.RecordTypeSOA,
		Name:  r.Name,
		Value: r.Data,
		TTL:   r.TTL,
	}, nil
}

// --- internal helpers ---

// gdRecordID constructs the canonical "TYPE/name" record ID used for GoDaddy.
func gdRecordID(r dnsiface.Record) string {
	return string(r.Type) + "/" + r.Name
}

// parseGDRecordID splits a "TYPE/name" record ID into its components.
func parseGDRecordID(id string) (rType, rName string, err error) {
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("godaddy dns: invalid record ID %q: expected TYPE/name", id)
	}
	return parts[0], parts[1], nil
}

// recordToGD converts a provider-agnostic record to GoDaddy API format.
func recordToGD(r dnsiface.Record) gdRecord {
	ttl := r.TTL
	if ttl < 600 {
		ttl = 600 // GoDaddy minimum TTL
	}
	return gdRecord{
		Type:     string(r.Type),
		Name:     r.Name,
		Data:     r.Value,
		TTL:      ttl,
		Priority: r.Priority,
	}
}

// gdRecordsToRecords converts GoDaddy API records to the provider-agnostic format.
func gdRecordsToRecords(gdrecs []gdRecord) []dnsiface.Record {
	out := make([]dnsiface.Record, 0, len(gdrecs))
	for _, r := range gdrecs {
		rec := dnsiface.Record{
			Type:     dnsiface.RecordType(strings.ToUpper(r.Type)),
			Name:     r.Name,
			Value:    r.Data,
			TTL:      r.TTL,
			Priority: r.Priority,
		}
		rec.ID = gdRecordID(rec)
		out = append(out, rec)
	}
	return out
}
