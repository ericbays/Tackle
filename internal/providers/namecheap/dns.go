package namecheap

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	dnsiface "tackle/internal/providers/dns"
)

// ListRecords fetches all DNS host records for the given domain zone via
// namecheap.domains.dns.getHosts and maps them to the provider-agnostic format.
func (c *Client) ListRecords(_ context.Context, zone string) ([]dnsiface.Record, error) {
	c.rateLimiter.Wait()

	sld, tld, err := splitDomain(zone)
	if err != nil {
		return nil, err
	}

	params := url.Values{
		"ApiUser":  {c.creds.APIUser},
		"ApiKey":   {c.creds.APIKey},
		"UserName": {c.creds.Username},
		"ClientIp": {c.creds.ClientIP},
		"Command":  {"namecheap.domains.dns.getHosts"},
		"SLD":      {sld},
		"TLD":      {tld},
	}

	body, err := c.doGet(params)
	if err != nil {
		return nil, fmt.Errorf("namecheap dns list: %w", err)
	}

	var resp ncGetHostsResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("namecheap dns list: parse response: %w", err)
	}
	if resp.Status != "OK" {
		return nil, c.translateDNSError(resp.Errors, zone)
	}

	return ncHostsToRecords(resp.CommandResponse.DomainDNSGetHostsResult.Hosts), nil
}

// CreateRecord adds a new DNS record for zone. Namecheap's API requires
// the full record set to be submitted atomically (read-modify-write).
func (c *Client) CreateRecord(_ context.Context, zone string, record dnsiface.Record) (dnsiface.Record, error) {
	c.rateLimiter.Wait()

	current, err := c.listRaw(zone)
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("namecheap dns create: %w", err)
	}

	// Assign a synthetic ID based on position so callers can reference the record.
	record.ID = fmt.Sprintf("%s/%s/%d", record.Type, record.Name, len(current))
	current = append(current, record)

	if err := c.setHosts(zone, current); err != nil {
		return dnsiface.Record{}, fmt.Errorf("namecheap dns create: %w", err)
	}
	return record, nil
}

// UpdateRecord replaces the record identified by recordID in the zone.
// recordID uses the format "TYPE/Name/index" assigned by CreateRecord.
func (c *Client) UpdateRecord(_ context.Context, zone string, recordID string, record dnsiface.Record) (dnsiface.Record, error) {
	c.rateLimiter.Wait()

	current, err := c.listRaw(zone)
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("namecheap dns update: %w", err)
	}

	idx, err := findRecordIndex(current, recordID)
	if err != nil {
		return dnsiface.Record{}, err
	}

	record.ID = recordID
	current[idx] = record

	if err := c.setHosts(zone, current); err != nil {
		return dnsiface.Record{}, fmt.Errorf("namecheap dns update: %w", err)
	}
	return record, nil
}

// DeleteRecord removes the record identified by recordID from the zone.
func (c *Client) DeleteRecord(_ context.Context, zone string, recordID string) error {
	c.rateLimiter.Wait()

	current, err := c.listRaw(zone)
	if err != nil {
		return fmt.Errorf("namecheap dns delete: %w", err)
	}

	idx, err := findRecordIndex(current, recordID)
	if err != nil {
		return err
	}

	updated := append(current[:idx], current[idx+1:]...)
	if err := c.setHosts(zone, updated); err != nil {
		return fmt.Errorf("namecheap dns delete: %w", err)
	}
	return nil
}

// GetSOA returns a synthesised SOA record using the zone's NS information.
// Namecheap's API does not expose the SOA directly; we return a placeholder.
func (c *Client) GetSOA(_ context.Context, zone string) (dnsiface.Record, error) {
	return dnsiface.Record{
		Type:  dnsiface.RecordTypeSOA,
		Name:  "@",
		Value: fmt.Sprintf("dns1.registrar-servers.com. hostmaster.%s. 1 28800 7200 604800 300", zone),
		TTL:   300,
	}, nil
}

// --- internal helpers ---

// listRaw fetches all records from Namecheap and returns them in the
// provider-agnostic format, preserving original index information in IDs.
func (c *Client) listRaw(zone string) ([]dnsiface.Record, error) {
	sld, tld, err := splitDomain(zone)
	if err != nil {
		return nil, err
	}

	params := url.Values{
		"ApiUser":  {c.creds.APIUser},
		"ApiKey":   {c.creds.APIKey},
		"UserName": {c.creds.Username},
		"ClientIp": {c.creds.ClientIP},
		"Command":  {"namecheap.domains.dns.getHosts"},
		"SLD":      {sld},
		"TLD":      {tld},
	}

	body, err := c.doGet(params)
	if err != nil {
		return nil, err
	}

	var resp ncGetHostsResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if resp.Status != "OK" {
		return nil, c.translateDNSError(resp.Errors, zone)
	}

	return ncHostsToRecords(resp.CommandResponse.DomainDNSGetHostsResult.Hosts), nil
}

// setHosts submits the full host record set to Namecheap via
// namecheap.domains.dns.setHosts.
func (c *Client) setHosts(zone string, records []dnsiface.Record) error {
	c.rateLimiter.Wait()

	sld, tld, err := splitDomain(zone)
	if err != nil {
		return err
	}

	params := url.Values{
		"ApiUser":  {c.creds.APIUser},
		"ApiKey":   {c.creds.APIKey},
		"UserName": {c.creds.Username},
		"ClientIp": {c.creds.ClientIP},
		"Command":  {"namecheap.domains.dns.setHosts"},
		"SLD":      {sld},
		"TLD":      {tld},
	}

	for i, r := range records {
		n := strconv.Itoa(i + 1)
		params.Set("HostName"+n, r.Name)
		params.Set("RecordType"+n, string(r.Type))
		params.Set("Address"+n, r.Value)
		params.Set("TTL"+n, strconv.Itoa(r.TTL))
		if r.Type == dnsiface.RecordTypeMX {
			params.Set("MXPref"+n, strconv.Itoa(r.Priority))
		}
	}

	body, err := c.doGet(params)
	if err != nil {
		return err
	}

	var resp ncSetHostsResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if resp.Status != "OK" {
		return c.translateDNSError(resp.Errors, zone)
	}
	return nil
}

// translateDNSError maps Namecheap DNS-specific error codes.
func (c *Client) translateDNSError(errs []Error, zone string) error {
	if len(errs) == 0 {
		return fmt.Errorf("namecheap dns: unknown error for zone %q", zone)
	}
	switch errs[0].Number {
	case "2019166":
		return dnsiface.ErrZoneNotFound
	case "2016166", "3050900":
		return dnsiface.ErrAPIRateLimit
	default:
		return c.translateError(errs)
	}
}

// splitDomain splits "example.com" into ("example", "com").
// For a domain like "sub.example.com" — the zone is "example.com", so
// callers should always pass the registrable domain, not a subdomain.
func splitDomain(zone string) (sld, tld string, err error) {
	parts := strings.SplitN(zone, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("namecheap: invalid zone %q: expected SLD.TLD format", zone)
	}
	return parts[0], parts[1], nil
}

// findRecordIndex locates a record in the list by its synthetic ID.
// Returns ErrRecordNotFound if the record is not present.
func findRecordIndex(records []dnsiface.Record, recordID string) (int, error) {
	for i, r := range records {
		if r.ID == recordID {
			return i, nil
		}
		// Fallback: match by TYPE/Name prefix for resilience.
		prefix := strings.Join(strings.SplitN(recordID, "/", 3)[:2], "/")
		if strings.HasPrefix(r.ID, prefix+"/") || r.ID == prefix {
			return i, nil
		}
	}
	return -1, fmt.Errorf("namecheap dns: %w: %s", dnsiface.ErrRecordNotFound, recordID)
}

// --- XML response types ---

type ncGetHostsResponse struct {
	XMLName         xml.Name `xml:"ApiResponse"`
	Status          string   `xml:"Status,attr"`
	Errors          []Error  `xml:"Errors>Error"`
	CommandResponse struct {
		DomainDNSGetHostsResult struct {
			Hosts []ncHost `xml:"host"`
		} `xml:"DomainDNSGetHostsResult"`
	} `xml:"CommandResponse"`
}

type ncSetHostsResponse struct {
	XMLName xml.Name `xml:"ApiResponse"`
	Status  string   `xml:"Status,attr"`
	Errors  []Error  `xml:"Errors>Error"`
}

type ncHost struct {
	HostID  string `xml:"HostId,attr"`
	Name    string `xml:"Name,attr"`
	Type    string `xml:"Type,attr"`
	Address string `xml:"Address,attr"`
	TTL     string `xml:"TTL,attr"`
	MXPref  string `xml:"MXPref,attr"`
}

// ncHostsToRecords converts Namecheap XML host entries to provider-agnostic records.
func ncHostsToRecords(hosts []ncHost) []dnsiface.Record {
	out := make([]dnsiface.Record, 0, len(hosts))
	for i, h := range hosts {
		rt := dnsiface.RecordType(strings.ToUpper(h.Type))
		ttl, _ := strconv.Atoi(h.TTL)
		prio, _ := strconv.Atoi(h.MXPref)

		id := h.HostID
		if id == "" {
			id = fmt.Sprintf("%s/%s/%d", rt, h.Name, i)
		}

		out = append(out, dnsiface.Record{
			ID:       id,
			Type:     rt,
			Name:     h.Name,
			Value:    h.Address,
			TTL:      ttl,
			Priority: prio,
		})
	}
	return out
}
