package conohav3

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/libdns/libdns"
)

// Provider facilitates DNS record management using the ConoHa VPS API (v3.0).
// It implements the libdns interfaces for getting, appending, setting, and deleting DNS records.
type Provider struct {
	APITenantID string `json:"api_tenant_id,omitempty"` // ConoHa API tenant ID
	APIUserID   string `json:"api_user_id,omitempty"`   // ConoHa API user ID
	APIPassword string `json:"api_password,omitempty"`  // ConoHa API password
	Region      string `json:"region,omitempty"`        // ConoHa API region (e.g. "c3j1")

	mutex sync.Mutex
}

// initClient initializes a new DNS API client with an authentication token.
func (p *Provider) initClient(ctx context.Context) (*dnsClient, error) {
	identifier, err := newIdentifier(p.Region)
	if err != nil {
		return nil, err
	}

	token, err := identifier.getToken(ctx, p.APITenantID, p.APIUserID, p.APIPassword)
	if err != nil {
		return nil, err
	}

	return newDnsClient(p.Region, token)
}

// GetRecords lists all the DNS records in the specified zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	dnsClient, err := p.initClient(ctx)
	if err != nil {
		return nil, err
	}

	domainID, err := dnsClient.getDomainID(ctx, zone)
	if err != nil {
		return nil, err
	}

	rawRecordList, err := dnsClient.getRecords(ctx, domainID)
	if err != nil {
		return nil, err
	}

	var libRecords []libdns.Record
	for _, record := range rawRecordList.Records {
		libRecord, err := convertToLibdnsRecord(record)
		if err != nil {
			if err == errRecordNotSupported {
				continue
			}
			return nil, err
		}
		libRecords = append(libRecords, libRecord)
	}

	return libRecords, nil
}

// AppendRecords adds the specified records to the zone.
// It returns the successfully added records.
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	dnsClient, err := p.initClient(ctx)
	if err != nil {
		return nil, err
	}

	domainID, err := dnsClient.getDomainID(ctx, zone)
	if err != nil {
		return nil, err
	}

	for _, rec := range records {
		rawRecord, err := convertToConohaDNSRecord(rec)
		if err != nil {
			return nil, err
		}

		_, err = dnsClient.createRecord(ctx, domainID, rawRecord)
		if err != nil {
			return nil, err
		}
	}

	return records, nil
}

// SetRecords sets the records in the zone, updating existing ones or creating new ones.
// It returns the records that were updated or added.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	dnsClient, err := p.initClient(ctx)
	if err != nil {
		return nil, err
	}

	domainID, err := dnsClient.getDomainID(ctx, zone)
	if err != nil {
		return nil, err
	}

	for _, rec := range records {
		converted, err := convertToConohaDNSRecord(rec)
		if err != nil {
			return nil, err
		}

		recordID, err := dnsClient.getRecordID(ctx, domainID, converted.Name, converted.Type)
		if err != nil {
			if errors.Is(err, errRecordNotFound) {
				_, err = dnsClient.createRecord(ctx, domainID, converted)
				if err != nil {
					return nil, err
				}
				continue
			}
			return nil, err
		}

		_, err = dnsClient.updateRecord(ctx, domainID, recordID, converted)
		if err != nil {
			return nil, err
		}
	}

	return records, nil
}

// DeleteRecords deletes the specified records from the zone.
// It returns the records that were successfully deleted.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	dnsClient, err := p.initClient(ctx)
	if err != nil {
		return nil, err
	}

	domainID, err := dnsClient.getDomainID(ctx, zone)
	if err != nil {
		return nil, err
	}

	for _, rec := range records {
		converted, err := convertToConohaDNSRecord(rec)
		if err != nil {
			return nil, err
		}

		recordID, err := dnsClient.getRecordID(ctx, domainID, converted.Name, converted.Type)
		if err != nil {
			return nil, err
		}

		if err := dnsClient.deleteRecord(ctx, domainID, recordID); err != nil {
			return nil, err
		}
	}

	return records, nil
}

// convertToLibdnsRecord converts a raw API record to a libdns-compatible record.
func convertToLibdnsRecord(rec conohaDNSRecord) (libdns.Record, error) {
	ttl := time.Duration(rec.TTL) * time.Second

	switch rec.Type {
	case "A", "AAAA":
		ip, err := netip.ParseAddr(rec.Data)
		if err != nil {
			return nil, err
		}
		return libdns.Address{
			Name: rec.Name,
			TTL:  ttl,
			IP:   ip,
		}, nil
	case "CNAME":
		return libdns.CNAME{
			Name:   rec.Name,
			TTL:    ttl,
			Target: rec.Data,
		}, nil
	case "TXT":
		return libdns.TXT{
			Name: rec.Name,
			TTL:  ttl,
			Text: rec.Data,
		}, nil
	default:
		return nil, errRecordNotSupported
	}
}

// convertToConohaDNSRecord converts a libdns.Record into a ConoHa-compatible raw Record struct.
func convertToConohaDNSRecord(rec libdns.Record) (conohaDNSRecord, error) {
	rr := rec.RR()
	parsed, err := rr.Parse()
	if err != nil {
		return conohaDNSRecord{}, fmt.Errorf("failed to parse record: %w", err)
	}

	if parsed == nil {
		return conohaDNSRecord{}, fmt.Errorf("record is nil after parsing: %v", rec)
	}

	switch r := parsed.(type) {
	case libdns.Address:
		return conohaDNSRecord{
			Name: r.Name,
			Type: rr.Type,
			Data: r.IP.String(),
			TTL:  int(r.TTL.Seconds()),
		}, nil
	case libdns.CNAME:
		return conohaDNSRecord{
			Name: r.Name,
			Type: rr.Type,
			Data: r.Target,
			TTL:  int(r.TTL.Seconds()),
		}, nil
	case libdns.TXT:
		return conohaDNSRecord{
			Name: r.Name,
			Type: rr.Type,
			Data: r.Text,
			TTL:  int(r.TTL.Seconds()),
		}, nil
	default:
		return conohaDNSRecord{}, errRecordNotSupported
	}
}

// Interface guards
var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
