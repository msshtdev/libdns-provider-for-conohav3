package conohav3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const dnsServiceBaseURL = "https://dns-service.%s.conoha.io"

// dnsClient is a ConoHa API client for DNS service.
type dnsClient struct {
	token string

	baseURL    *url.URL
	HTTPClient *http.Client
}

// newDnsClient returns a client for DNS service instance logged into the ConoHa service.
func newDnsClient(region, token string) (*dnsClient, error) {
	if region == "" {
		region = "c3j1"
	}

	baseURL, err := url.Parse(fmt.Sprintf(dnsServiceBaseURL, region))
	if err != nil {
		return nil, err
	}

	return &dnsClient{
		token:      token,
		baseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}, nil
}

// getDomainID returns an ID of specified domain.
func (c *dnsClient) getDomainID(ctx context.Context, domainName string) (string, error) {
	domainList, err := c.getDomains(ctx)
	if err != nil {
		return "", err
	}

	for _, domain := range domainList.Domains {
		if domain.Name == domainName {
			return domain.UUID, nil
		}
	}

	return "", fmt.Errorf("no such domain: %s", domainName)
}

// getDomains returns a list of domains registered in DNS.
// https://doc.conoha.jp/reference/api-vps3/api-dns-vps3/dnsaas-get_domains_list-v3/?btn_id=reference-api-vps3--sidebar_reference-dnsaas-get_domains_list-v3
func (c *dnsClient) getDomains(ctx context.Context) (*domainListResponse, error) {
	endpoint := c.baseURL.JoinPath("v1", "domains")

	req, err := newJSONRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	domainList := &domainListResponse{}

	err = c.do(req, domainList)
	if err != nil {
		return nil, err
	}

	return domainList, nil
}

// getRecordID returns an ID of specified record.
func (c *dnsClient) getRecordID(ctx context.Context, domainID, recordName, recordType string) (string, error) {
	recordList, err := c.getRecords(ctx, domainID)
	if err != nil {
		return "", err
	}

	for _, record := range recordList.Records {
		if record.Name == recordName && record.Type == recordType {
			return record.UUID, nil
		}
	}

	return "", errRecordNotFound
}

// getRecords returns a list of records registered for the domain identified by the domainID.
// https://doc.conoha.jp/reference/api-vps3/api-dns-vps3/dnsaas-get_records_list-v3/?btn_id=reference-dnsaas-get_domains_list-v3--sidebar_reference-dnsaas-get_records_list-v3
func (c *dnsClient) getRecords(ctx context.Context, domainID string) (*recordListResponse, error) {
	endpoint := c.baseURL.JoinPath("v1", "domains", domainID, "records")

	req, err := newJSONRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	recordList := &recordListResponse{}

	err = c.do(req, recordList)
	if err != nil {
		return nil, err
	}

	return recordList, nil
}

// createRecord adds new record.
// https://doc.conoha.jp/reference/api-vps3/api-dns-vps3/dnsaas-create_record-v3/?btn_id=reference-dnsaas-get_records_list-v3--sidebar_reference-dnsaas-create_record-v3
func (c *dnsClient) createRecord(ctx context.Context, domainID string, record conohaDNSRecord) (*conohaDNSRecord, error) {
	endpoint := c.baseURL.JoinPath("v1", "domains", domainID, "records")

	req, err := newJSONRequest(ctx, http.MethodPost, endpoint, record)
	if err != nil {
		return nil, err
	}

	newRecord := &conohaDNSRecord{}

	err = c.do(req, newRecord)
	if err != nil {
		return nil, err
	}

	return newRecord, nil
}

// updateRecord update specified record.
// https://doc.conoha.jp/reference/api-vps3/api-dns-vps3/dnsaas-update_record-v3/?btn_id=reference-dnsaas-update_record-v3--sidebar_reference-dnsaas-update_record-v3
func (c *dnsClient) updateRecord(ctx context.Context, domainID string, recordID string, record conohaDNSRecord) (*conohaDNSRecord, error) {
	endpoint := c.baseURL.JoinPath("v1", "domains", domainID, "records", recordID)
	// NOTE: ConoHa's DNS API inconsistently may handle the `ttl` field:
	//       - `ttl` is accepted in record creation (POST), even though it's undocumented.
	//       - `ttl` causes a 400 error during update (PUT), even if set to the same value.
	//       As a workaround, we zero-out TTL on updates to suppress the field via `omitempty`.
	//       This should be revisited if the API adds consistent TTL support for updates.

	record.TTL = 0

	req, err := newJSONRequest(ctx, http.MethodPut, endpoint, record)
	if err != nil {
		return nil, err
	}

	newRecord := &conohaDNSRecord{}
	err = c.do(req, newRecord)
	if err != nil {
		return nil, err
	}

	return newRecord, nil
}

// DeleteRecord removes specified record.
// https://doc.conoha.jp/reference/api-vps3/api-dns-vps3/dnsaas-delete_record-v3/?btn_id=reference-dnsaas-create_record-v3--sidebar_reference-dnsaas-delete_record-v3
func (c *dnsClient) deleteRecord(ctx context.Context, domainID, recordID string) error {
	endpoint := c.baseURL.JoinPath("v1", "domains", domainID, "records", recordID)

	req, err := newJSONRequest(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}

	return c.do(req, nil)
}

// do sends an HTTP request and optionally decodes the JSON response into the provided result.
func (c *dnsClient) do(req *http.Request, result any) error {
	if c.token != "" {
		req.Header.Set("X-Auth-Token", c.token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("got error status: HTTP %d\nResponse body: %s", resp.StatusCode, string(bodyBytes))
	}

	if result == nil {
		return nil
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(raw, result)
	if err != nil {
		return err
	}

	return nil
}

// newJSONRequest creates a new HTTP request with a JSON-encoded payload.
func newJSONRequest(ctx context.Context, method string, endpoint *url.URL, payload any) (*http.Request, error) {
	buf := new(bytes.Buffer)

	if payload != nil {
		err := json.NewEncoder(buf).Encode(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to create request JSON body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), buf)
	if err != nil {
		return nil, fmt.Errorf("unable to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}
