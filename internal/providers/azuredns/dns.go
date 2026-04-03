package azuredns

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"

	dnsiface "tackle/internal/providers/dns"
)

// recordsClient returns an Azure DNS RecordSetsClient for the configured
// subscription and credentials.
func (c *Client) recordsClient() (*armdns.RecordSetsClient, error) {
	cred, err := azidentity.NewClientSecretCredential(
		c.creds.TenantID,
		c.creds.ClientID,
		c.creds.ClientSecret,
		nil,
	)
	if err != nil {
		return nil, translateAzureError(err)
	}
	client, err := armdns.NewRecordSetsClient(c.creds.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("azure dns: create record sets client: %w", err)
	}
	return client, nil
}

// ListRecords returns all DNS records in the zone identified by domain.
func (c *Client) ListRecords(ctx context.Context, zone string) ([]dnsiface.Record, error) {
	c.rateLimiter.Wait()

	client, err := c.recordsClient()
	if err != nil {
		return nil, fmt.Errorf("azure dns list: %w", err)
	}

	pager := client.NewListByDNSZonePager(c.creds.ResourceGroup, zone, nil)
	var records []dnsiface.Record

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("azure dns list: %w", translateAzureError(err))
		}
		for _, rs := range page.Value {
			records = append(records, azureRSToRecords(rs, zone)...)
		}
	}

	return records, nil
}

// CreateRecord creates a DNS record via RecordSets.CreateOrUpdate.
func (c *Client) CreateRecord(ctx context.Context, zone string, record dnsiface.Record) (dnsiface.Record, error) {
	c.rateLimiter.Wait()

	client, err := c.recordsClient()
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("azure dns create: %w", err)
	}

	rs := recordToAzureRS(record)
	relName := azureRelativeName(record.Name, zone)

	_, err = client.CreateOrUpdate(ctx, c.creds.ResourceGroup, zone, relName,
		armdns.RecordType(string(record.Type)), rs, nil)
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("azure dns create: %w", translateAzureError(err))
	}

	record.ID = azureRecordID(record, c.creds.ResourceGroup, zone)
	return record, nil
}

// UpdateRecord replaces a DNS record via RecordSets.CreateOrUpdate.
// recordID is expected in "resourceGroup/zone/TYPE/name" format.
func (c *Client) UpdateRecord(ctx context.Context, zone string, _ string, record dnsiface.Record) (dnsiface.Record, error) {
	c.rateLimiter.Wait()

	client, err := c.recordsClient()
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("azure dns update: %w", err)
	}

	rs := recordToAzureRS(record)
	relName := azureRelativeName(record.Name, zone)

	_, err = client.CreateOrUpdate(ctx, c.creds.ResourceGroup, zone, relName,
		armdns.RecordType(string(record.Type)), rs, nil)
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("azure dns update: %w", translateAzureError(err))
	}

	record.ID = azureRecordID(record, c.creds.ResourceGroup, zone)
	return record, nil
}

// DeleteRecord removes a DNS record by record set name and type.
// recordID is expected in "resourceGroup/zone/TYPE/name" format.
func (c *Client) DeleteRecord(ctx context.Context, zone string, recordID string) error {
	c.rateLimiter.Wait()

	client, err := c.recordsClient()
	if err != nil {
		return fmt.Errorf("azure dns delete: %w", err)
	}

	_, rType, rName, err := parseAzureRecordID(recordID)
	if err != nil {
		return fmt.Errorf("azure dns delete: %w", err)
	}

	relName := azureRelativeName(rName, zone)
	_, err = client.Delete(ctx, c.creds.ResourceGroup, zone, relName,
		armdns.RecordType(rType), nil)
	if err != nil {
		azErr := translateAzureError(err)
		if strings.Contains(err.Error(), "ResourceNotFound") || strings.Contains(err.Error(), "NotFound") {
			return dnsiface.ErrRecordNotFound
		}
		return fmt.Errorf("azure dns delete: %w", azErr)
	}
	return nil
}

// GetSOA returns the SOA record for the zone.
func (c *Client) GetSOA(ctx context.Context, zone string) (dnsiface.Record, error) {
	c.rateLimiter.Wait()

	client, err := c.recordsClient()
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("azure dns get soa: %w", err)
	}

	rs, err := client.Get(ctx, c.creds.ResourceGroup, zone, "@", armdns.RecordTypeSOA, nil)
	if err != nil {
		return dnsiface.Record{}, fmt.Errorf("azure dns get soa: %w", translateAzureError(err))
	}

	recs := azureRSToRecords(&rs.RecordSet, zone)
	if len(recs) == 0 {
		return dnsiface.Record{Type: dnsiface.RecordTypeSOA, Name: "@", Value: zone}, nil
	}
	return recs[0], nil
}

// --- internal helpers ---

// azureRecordID constructs a canonical "rg/zone/TYPE/name" identifier.
func azureRecordID(r dnsiface.Record, rg, zone string) string {
	return rg + "/" + zone + "/" + string(r.Type) + "/" + r.Name
}

// parseAzureRecordID splits a "rg/zone/TYPE/name" record ID.
func parseAzureRecordID(id string) (rg, rType, rName string, err error) {
	parts := strings.SplitN(id, "/", 4)
	if len(parts) != 4 {
		return "", "", "", fmt.Errorf("azure dns: invalid record ID %q: expected rg/zone/TYPE/name", id)
	}
	return parts[0], parts[2], parts[3], nil
}

// azureRelativeName converts a record name to the relative name Azure expects.
// Azure uses "@" for the zone apex and plain subdomain labels for others.
func azureRelativeName(name, zone string) string {
	if name == "@" || name == zone || name == zone+"." {
		return "@"
	}
	return strings.TrimSuffix(name, "."+zone)
}

// recordToAzureRS converts a provider-agnostic record to an Azure RecordSet.
func recordToAzureRS(r dnsiface.Record) armdns.RecordSet {
	ttl := int64(r.TTL)
	if ttl <= 0 {
		ttl = 300
	}

	rs := armdns.RecordSet{
		Properties: &armdns.RecordSetProperties{
			TTL: &ttl,
		},
	}

	switch r.Type {
	case dnsiface.RecordTypeA:
		rs.Properties.ARecords = []*armdns.ARecord{
			{IPv4Address: &r.Value},
		}
	case dnsiface.RecordTypeAAAA:
		rs.Properties.AaaaRecords = []*armdns.AaaaRecord{
			{IPv6Address: &r.Value},
		}
	case dnsiface.RecordTypeCNAME:
		rs.Properties.CnameRecord = &armdns.CnameRecord{Cname: &r.Value}
	case dnsiface.RecordTypeMX:
		prio := int32(r.Priority)
		rs.Properties.MxRecords = []*armdns.MxRecord{
			{Exchange: &r.Value, Preference: &prio},
		}
	case dnsiface.RecordTypeTXT:
		rs.Properties.TxtRecords = []*armdns.TxtRecord{
			{Value: []*string{&r.Value}},
		}
	case dnsiface.RecordTypeNS:
		rs.Properties.NsRecords = []*armdns.NsRecord{
			{Nsdname: &r.Value},
		}
	}

	return rs
}

// azureRSToRecords converts an Azure RecordSet to provider-agnostic records.
// Azure record sets can hold multiple values; we expand them.
func azureRSToRecords(rs *armdns.RecordSet, zone string) []dnsiface.Record {
	if rs == nil || rs.Properties == nil || rs.Type == nil || rs.Name == nil {
		return nil
	}

	// Azure type format: "Microsoft.Network/dnszones/A" — extract last segment.
	azType := *rs.Type
	if idx := strings.LastIndex(azType, "/"); idx >= 0 {
		azType = azType[idx+1:]
	}

	rType := dnsiface.RecordType(strings.ToUpper(azType))
	name := *rs.Name
	ttl := int(0)
	if rs.Properties.TTL != nil {
		ttl = int(*rs.Properties.TTL)
	}

	props := rs.Properties
	var out []dnsiface.Record

	makeRecord := func(value string, priority int) dnsiface.Record {
		rec := dnsiface.Record{
			Type:     rType,
			Name:     name,
			Value:    value,
			TTL:      ttl,
			Priority: priority,
		}
		rg := "" // resource group not available from record set alone
		rec.ID = rg + "/" + zone + "/" + string(rType) + "/" + name
		return rec
	}

	switch rType {
	case dnsiface.RecordTypeA:
		for _, r := range props.ARecords {
			if r.IPv4Address != nil {
				out = append(out, makeRecord(*r.IPv4Address, 0))
			}
		}
	case dnsiface.RecordTypeAAAA:
		for _, r := range props.AaaaRecords {
			if r.IPv6Address != nil {
				out = append(out, makeRecord(*r.IPv6Address, 0))
			}
		}
	case dnsiface.RecordTypeCNAME:
		if props.CnameRecord != nil && props.CnameRecord.Cname != nil {
			out = append(out, makeRecord(*props.CnameRecord.Cname, 0))
		}
	case dnsiface.RecordTypeMX:
		for _, r := range props.MxRecords {
			if r.Exchange != nil {
				prio := 0
				if r.Preference != nil {
					prio = int(*r.Preference)
				}
				out = append(out, makeRecord(*r.Exchange, prio))
			}
		}
	case dnsiface.RecordTypeTXT:
		for _, r := range props.TxtRecords {
			var parts []string
			for _, v := range r.Value {
				if v != nil {
					parts = append(parts, *v)
				}
			}
			out = append(out, makeRecord(strings.Join(parts, ""), 0))
		}
	case dnsiface.RecordTypeNS:
		for _, r := range props.NsRecords {
			if r.Nsdname != nil {
				out = append(out, makeRecord(*r.Nsdname, 0))
			}
		}
	case dnsiface.RecordTypeSOA:
		if props.SoaRecord != nil {
			val := ""
			if props.SoaRecord.Host != nil {
				val = *props.SoaRecord.Host
			}
			out = append(out, makeRecord(val, 0))
		}
	}

	return out
}
