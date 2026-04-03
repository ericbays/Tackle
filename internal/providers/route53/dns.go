package route53

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	dnsiface "tackle/internal/providers/dns"
)

// r53DNSAPI extends the internal interface with DNS record operations.
// This is separate from r53API to avoid changing the test surface of the
// existing TestConnection code.
type r53DNSAPI interface {
	r53API
	ListResourceRecordSets(ctx context.Context, params *route53.ListResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error)
	ChangeResourceRecordSets(ctx context.Context, params *route53.ChangeResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ChangeResourceRecordSetsOutput, error)
}

// dnsClient returns the underlying r53 as an r53DNSAPI, or an error if the
// runtime client does not implement the required methods.
func (c *Client) dnsClient() (r53DNSAPI, error) {
	api, ok := c.r53.(r53DNSAPI)
	if !ok {
		return nil, fmt.Errorf("route53: DNS operations require a full Route 53 client (got test stub)")
	}
	return api, nil
}

// GetHostedZoneForDomain resolves the Route 53 hosted zone ID for a given domain.
// It iterates hosted zones and finds the best (longest suffix) match.
func (c *Client) GetHostedZoneForDomain(ctx context.Context, domain string) (string, error) {
	api, err := c.dnsClient()
	if err != nil {
		return "", err
	}

	c.rateLimiter.Wait()

	// Normalise to FQDN with trailing dot.
	fqdn := toFQDN(domain)

	var marker *string
	for {
		out, err := api.ListHostedZones(ctx, &route53.ListHostedZonesInput{
			Marker: marker,
		})
		if err != nil {
			return "", translateAWSError(err)
		}

		for _, z := range out.HostedZones {
			if z.Name != nil && *z.Name == fqdn {
				return zoneID(*z.Id), nil
			}
		}

		if !out.IsTruncated {
			break
		}
		marker = out.NextMarker
	}

	return "", fmt.Errorf("route53: %w: no hosted zone found for %q", dnsiface.ErrZoneNotFound, domain)
}

// ListRecords returns all DNS records in the zone identified by domain.
func (c *Client) ListRecords(ctx context.Context, zone string) ([]dnsiface.Record, error) {
	api, err := c.dnsClient()
	if err != nil {
		return nil, err
	}

	zoneID, err := c.GetHostedZoneForDomain(ctx, zone)
	if err != nil {
		return nil, fmt.Errorf("route53 dns list: %w", err)
	}

	c.rateLimiter.Wait()

	var records []dnsiface.Record
	var startName, startType *string

	for {
		out, err := api.ListResourceRecordSets(ctx, &route53.ListResourceRecordSetsInput{
			HostedZoneId:    aws.String(zoneID),
			StartRecordName: startName,
			StartRecordType: func() r53types.RRType {
				if startType != nil {
					return r53types.RRType(*startType)
				}
				return ""
			}(),
		})
		if err != nil {
			return nil, fmt.Errorf("route53 dns list: %w", translateAWSError(err))
		}

		for _, rrs := range out.ResourceRecordSets {
			records = append(records, r53RRSToRecords(rrs, zoneID)...)
		}

		if !out.IsTruncated {
			break
		}
		startName = out.NextRecordName
		t := string(out.NextRecordType)
		startType = &t
	}

	return records, nil
}

// CreateRecord creates a DNS record via ChangeResourceRecordSets with CREATE action.
func (c *Client) CreateRecord(ctx context.Context, zone string, record dnsiface.Record) (dnsiface.Record, error) {
	api, err := c.dnsClient()
	if err != nil {
		return dnsiface.Record{}, err
	}

	zoneID, err := c.GetHostedZoneForDomain(ctx, zone)
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("route53 dns create: %w", err)
	}

	c.rateLimiter.Wait()

	rrs := recordToR53RRS(record, zone)
	_, err = api.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch: &r53types.ChangeBatch{
			Changes: []r53types.Change{
				{
					Action:            r53types.ChangeActionCreate,
					ResourceRecordSet: &rrs,
				},
			},
		},
	})
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("route53 dns create: %w", translateAWSError(err))
	}

	record.ID = r53RecordID(record, zoneID)
	return record, nil
}

// UpdateRecord replaces a DNS record via ChangeResourceRecordSets with UPSERT action.
func (c *Client) UpdateRecord(ctx context.Context, zone string, _ string, record dnsiface.Record) (dnsiface.Record, error) {
	api, err := c.dnsClient()
	if err != nil {
		return dnsiface.Record{}, err
	}

	zoneID, err := c.GetHostedZoneForDomain(ctx, zone)
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("route53 dns update: %w", err)
	}

	c.rateLimiter.Wait()

	rrs := recordToR53RRS(record, zone)
	_, err = api.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch: &r53types.ChangeBatch{
			Changes: []r53types.Change{
				{
					Action:            r53types.ChangeActionUpsert,
					ResourceRecordSet: &rrs,
				},
			},
		},
	})
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("route53 dns update: %w", translateAWSError(err))
	}

	record.ID = r53RecordID(record, zoneID)
	return record, nil
}

// DeleteRecord removes a DNS record via ChangeResourceRecordSets with DELETE action.
// recordID is expected in "zoneID/TYPE/name" format.
func (c *Client) DeleteRecord(ctx context.Context, zone string, recordID string) error {
	api, err := c.dnsClient()
	if err != nil {
		return err
	}

	zoneID, rType, rName, err := parseR53RecordID(recordID)
	if err != nil {
		// Fallback: resolve zone from domain name.
		var zerr error
		zoneID, zerr = c.GetHostedZoneForDomain(ctx, zone)
		if zerr != nil {
			return fmt.Errorf("route53 dns delete: %w", zerr)
		}
		// Try to split by last known pattern.
		return fmt.Errorf("route53 dns delete: invalid record ID %q: %w", recordID, err)
	}

	// We need the full record to construct a valid DELETE change.
	// Look it up first so we have the correct values.
	records, err := c.ListRecords(ctx, zone)
	if err != nil {
		return fmt.Errorf("route53 dns delete: list: %w", err)
	}

	var target *dnsiface.Record
	for i := range records {
		if string(records[i].Type) == rType && stripTrailingDot(records[i].Name) == stripTrailingDot(rName) {
			target = &records[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("route53 dns delete: %w: %s", dnsiface.ErrRecordNotFound, recordID)
	}

	c.rateLimiter.Wait()

	rrs := recordToR53RRS(*target, zone)
	_, err = api.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch: &r53types.ChangeBatch{
			Changes: []r53types.Change{
				{
					Action:            r53types.ChangeActionDelete,
					ResourceRecordSet: &rrs,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("route53 dns delete: %w", translateAWSError(err))
	}
	return nil
}

// GetSOA returns the SOA record for the zone.
func (c *Client) GetSOA(ctx context.Context, zone string) (dnsiface.Record, error) {
	records, err := c.ListRecords(ctx, zone)
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("route53 get soa: %w", err)
	}
	for _, r := range records {
		if r.Type == dnsiface.RecordTypeSOA {
			return r, nil
		}
	}
	return dnsiface.Record{}, fmt.Errorf("route53 get soa: %w", dnsiface.ErrRecordNotFound)
}

// --- internal helpers ---

// toFQDN appends a trailing dot if not already present.
func toFQDN(domain string) string {
	if strings.HasSuffix(domain, ".") {
		return domain
	}
	return domain + "."
}

// stripTrailingDot removes a trailing dot from a DNS name.
func stripTrailingDot(s string) string {
	return strings.TrimSuffix(s, ".")
}

// zoneID strips the "/hostedzone/" prefix from a Route 53 zone ID ARN.
func zoneID(id string) string {
	return strings.TrimPrefix(id, "/hostedzone/")
}

// r53RecordID constructs a canonical "zoneID/TYPE/name" identifier.
func r53RecordID(r dnsiface.Record, zID string) string {
	return zID + "/" + string(r.Type) + "/" + r.Name
}

// parseR53RecordID splits a "zoneID/TYPE/name" record ID.
func parseR53RecordID(id string) (zoneID, rType, rName string, err error) {
	parts := strings.SplitN(id, "/", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("invalid Route 53 record ID %q: expected zoneID/TYPE/name", id)
	}
	return parts[0], parts[1], parts[2], nil
}

// recordToR53RRS converts a provider-agnostic record to a Route 53 ResourceRecordSet.
func recordToR53RRS(r dnsiface.Record, zone string) r53types.ResourceRecordSet {
	name := r.Name
	if name == "@" {
		name = zone
	}
	if !strings.HasSuffix(name, ".") {
		name = name + "." + zone + "."
	}

	ttl := int64(r.TTL)
	if ttl <= 0 {
		ttl = 300
	}

	rrs := r53types.ResourceRecordSet{
		Name: aws.String(name),
		Type: r53types.RRType(string(r.Type)),
		TTL:  aws.Int64(ttl),
		ResourceRecords: []r53types.ResourceRecord{
			{Value: aws.String(r53FormatValue(r))},
		},
	}

	if r.Type == dnsiface.RecordTypeMX {
		rrs.ResourceRecords[0].Value = aws.String(fmt.Sprintf("%d %s", r.Priority, r.Value))
	}

	return rrs
}

// r53FormatValue returns the record value in Route 53 wire format.
func r53FormatValue(r dnsiface.Record) string {
	switch r.Type {
	case dnsiface.RecordTypeTXT:
		// Route 53 requires TXT values quoted.
		if !strings.HasPrefix(r.Value, "\"") {
			return fmt.Sprintf("%q", r.Value)
		}
		return r.Value
	case dnsiface.RecordTypeCNAME, dnsiface.RecordTypeNS:
		return toFQDN(r.Value)
	default:
		return r.Value
	}
}

// r53RRSToRecords converts a Route 53 ResourceRecordSet to provider-agnostic records.
// A single RRS can have multiple ResourceRecords; we expand them to one record each.
func r53RRSToRecords(rrs r53types.ResourceRecordSet, zoneID string) []dnsiface.Record {
	rType := dnsiface.RecordType(string(rrs.Type))
	name := ""
	if rrs.Name != nil {
		name = stripTrailingDot(*rrs.Name)
	}
	ttl := 0
	if rrs.TTL != nil {
		ttl = int(*rrs.TTL)
	}

	var out []dnsiface.Record
	for _, rr := range rrs.ResourceRecords {
		val := ""
		if rr.Value != nil {
			val = *rr.Value
		}
		prio := 0

		if rType == dnsiface.RecordTypeMX {
			var p int
			var v string
			if _, err := fmt.Sscanf(val, "%d %s", &p, &v); err == nil {
				prio = p
				val = v
			}
		}

		// Strip quotes from TXT records.
		if rType == dnsiface.RecordTypeTXT {
			val = strings.Trim(val, "\"")
		}

		rec := dnsiface.Record{
			Type:     rType,
			Name:     name,
			Value:    val,
			TTL:      ttl,
			Priority: prio,
		}
		rec.ID = r53RecordID(rec, zoneID)
		out = append(out, rec)
	}
	return out
}
