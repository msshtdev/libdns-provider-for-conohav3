package conohav3

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/libdns/libdns"
)

var (
	apiTenantID = ""
	apiUserID   = ""
	apiPassword = ""
	zone        = ""
	testRecords = []libdns.Record{}
)

func TestMain(m *testing.M) {
	fmt.Println("Loading environment variables to set up provider")
	apiTenantID = os.Getenv("API_TENANT_ID")
	apiUserID = os.Getenv("API_USER_ID")
	apiPassword = os.Getenv("API_PASSWORD")
	zone = os.Getenv("ZONE")
	testRecords = []libdns.Record{
		libdns.TXT{
			Name: "test." + zone,
			Text: "testval1",
			TTL:  time.Duration(1200) * time.Second,
		},
	}

	os.Exit(m.Run())
}

func setupTestRecords(t *testing.T, p *Provider) []libdns.Record {
	fmt.Println("Appending test records")
	records, err := p.AppendRecords(context.TODO(), zone, testRecords)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
		return nil
	}
	return records
}

func cleanupRecords(t *testing.T, p *Provider, records []libdns.Record) {
	fmt.Println("Cleaning up test records")
	if _, err := p.DeleteRecords(context.TODO(), zone, records); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
}

func isSameRecord(a libdns.Record, b libdns.Record) bool {
	rawa, _ := convertToConohaDNSRecord(a)
	rawb, _ := convertToConohaDNSRecord(b)

	// NOTE: We intentionally do not compare TTL values here.
	// ConoHa's API does not consistently preserve or allow updates to TTL,
	// especially during record updates, where TTL may be omitted or reset.
	// Comparing TTL would cause false mismatches in those cases.

	return rawa.Name == rawb.Name && rawa.Type == rawb.Type && rawa.Data == rawb.Data
}

func TestProvider_GetRecords(t *testing.T) {
	fmt.Println("Test GetRecords")

	p := &Provider{
		APITenantID: apiTenantID,
		APIUserID:   apiUserID,
		APIPassword: apiPassword,
	}

	setupRecords := setupTestRecords(t, p)
	defer cleanupRecords(t, p, setupRecords)

	records, err := p.GetRecords(context.TODO(), zone)
	if err != nil {
		t.Fatal(err)
	}

	for _, testRec := range testRecords {
		found := false
		for _, rec := range records {
			if isSameRecord(testRec, rec) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("testRecord not found: %+v", testRec)
		}
	}
}

func TestProvider_SetProvider(t *testing.T) {
	fmt.Println("Test SetRecords")

	p := &Provider{
		APITenantID: apiTenantID,
		APIUserID:   apiUserID,
		APIPassword: apiPassword,
	}

	setupRecords := setupTestRecords(t, p)
	defer cleanupRecords(t, p, setupRecords)

	var newTestRecords []libdns.Record
	newData := "updated"
	newTTL := 1200

	for _, testRec := range testRecords {
		rawRec, err := convertToConohaDNSRecord(testRec)
		if err != nil {
			t.Fatal(err)
		}

		rawRec.Data = newData
		rawRec.TTL = newTTL

		newRec, err := convertToLibdnsRecord(rawRec)
		if err != nil {
			t.Fatal(err)
		}

		newTestRecords = append(newTestRecords, newRec)
	}

	_, err := p.SetRecords(context.TODO(), zone, newTestRecords)
	if err != nil {
		t.Fatal(err)
	}

	records, err := p.GetRecords(context.TODO(), zone)
	if err != nil {
		t.Fatal(err)
	}

	for _, testRec := range newTestRecords {
		found := false
		for _, rec := range records {
			if isSameRecord(testRec, rec) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("testRecord not found: %+v", testRec)
		}
	}
}
