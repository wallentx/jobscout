package fetcher

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseLayoffsFYISignalsCardlytics(t *testing.T) {
	input := []byte(`{
		"data": {
			"table": {
				"columns": [
					{"id": "fldCompany", "name": "Company"},
					{"id": "fldLaidOff", "name": "# Laid Off"},
					{"id": "fldDate", "name": "Date"},
					{"id": "fldPercent", "name": "%"},
					{"id": "fldSource", "name": "Source"}
				],
				"rows": [
					{
						"cellValuesByColumnId": {
							"fldCompany": "Cardlytics",
							"fldLaidOff": 51,
							"fldDate": "2022-11-14T00:00:00.000Z",
							"fldSource": "https://example.com/cardlytics"
						}
					},
					{
						"cellValuesByColumnId": {
							"fldCompany": "OtherCo",
							"fldLaidOff": 20,
							"fldDate": "2024-01-01T00:00:00.000Z"
						}
					}
				]
			}
		}
	}`)

	got, err := parseLayoffsFYISignals(input, "Cardlytics")
	if err != nil {
		t.Fatalf("parseLayoffsFYISignals(Cardlytics) error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("parseLayoffsFYISignals(Cardlytics) len = %d, want 1", len(got))
	}
	if got[0].Title != "Layoffs.fyi: Cardlytics laid off 51 employees" {
		t.Errorf("parseLayoffsFYISignals(Cardlytics)[0].Title = %q, want Cardlytics title", got[0].Title)
	}
	if got[0].EmployeeCount == nil || *got[0].EmployeeCount != 51 {
		t.Errorf("parseLayoffsFYISignals(Cardlytics)[0].EmployeeCount = %#v, want 51", got[0].EmployeeCount)
	}
	if got[0].Date == nil || got[0].Date.Format("2006-01-02") != "2022-11-14" {
		t.Errorf("parseLayoffsFYISignals(Cardlytics)[0].Date = %#v, want 2022-11-14", got[0].Date)
	}
	if got[0].URL != "https://example.com/cardlytics" {
		t.Errorf("parseLayoffsFYISignals(Cardlytics)[0].URL = %q, want fixture source", got[0].URL)
	}
}

func TestExtractLayoffsFYISharedViewDataURL(t *testing.T) {
	input := `window.__stashedPrefetch = { urlWithParams: "/v0.3/view/viw123/readSharedViewData?stringifiedObjectParams={\"shareId\":\"shr123\"}" }`

	got, err := extractLayoffsFYISharedViewDataURL(input)
	if err != nil {
		t.Fatalf("extractLayoffsFYISharedViewDataURL() error = %v", err)
	}

	want := `https://airtable.com/v0.3/view/viw123/readSharedViewData?stringifiedObjectParams={"shareId":"shr123"}`
	if got != want {
		t.Fatalf("extractLayoffsFYISharedViewDataURL() = %q, want %q", got, want)
	}
}

func TestFetchLayoffsFYISignalsCachesSharedViewDataAcrossCompanies(t *testing.T) {
	previousHTTPGet := httpGet
	previousDoHTTPGet := doHTTPGet
	previousCache := layoffsFYIData
	t.Cleanup(func() {
		httpGet = previousHTTPGet
		doHTTPGet = previousDoHTTPGet
		layoffsFYIData = previousCache
	})

	layoffsFYIData = newLayoffsFYIDataCache(fetchLayoffsFYISharedViewData)
	homeRequests := 0
	embedRequests := 0
	sharedRequests := 0
	httpGet = func(rawURL string) ([]byte, error) {
		switch {
		case rawURL == layoffsFYIURL:
			homeRequests++
			return []byte(`<iframe src="https://airtable.com/embed/appTest/shrTest/tblTest"></iframe>`), nil
		case strings.HasPrefix(rawURL, "https://airtable.com/embed/"):
			embedRequests++
			return []byte(`window.__stashedPrefetch = { urlWithParams: "/v0.3/view/viw123/readSharedViewData?stringifiedObjectParams={\"shareId\":\"shr123\"}" }`), nil
		default:
			return nil, fmt.Errorf("unexpected httpGet URL %q", rawURL)
		}
	}
	doHTTPGet = func(rawURL string, headers map[string]string) ([]byte, error) {
		if !strings.HasPrefix(rawURL, layoffsFYIAirtableHost+layoffsFYISharedViewDataPath) {
			return nil, fmt.Errorf("unexpected doHTTPGet URL %q", rawURL)
		}
		sharedRequests++
		return []byte(`{
			"data": {
				"table": {
					"columns": [
						{"id": "fldCompany", "name": "Company"},
						{"id": "fldLaidOff", "name": "# Laid Off"},
						{"id": "fldDate", "name": "Date"},
						{"id": "fldPercent", "name": "%"},
						{"id": "fldSource", "name": "Source"}
					],
					"rows": [
						{"cellValuesByColumnId": {"fldCompany": "Cardlytics", "fldLaidOff": 51, "fldDate": "2022-11-14T00:00:00.000Z"}},
						{"cellValuesByColumnId": {"fldCompany": "Acme", "fldLaidOff": 20, "fldDate": "2024-01-01T00:00:00.000Z"}}
					]
				}
			}
		}`), nil
	}

	cardlytics, err := FetchLayoffsFYISignals("Cardlytics")
	if err != nil {
		t.Fatalf("FetchLayoffsFYISignals(Cardlytics) error = %v", err)
	}
	acme, err := FetchLayoffsFYISignals("Acme")
	if err != nil {
		t.Fatalf("FetchLayoffsFYISignals(Acme) error = %v", err)
	}
	if len(cardlytics) != 1 || len(acme) != 1 {
		t.Fatalf("signals len = Cardlytics:%d Acme:%d; want both 1", len(cardlytics), len(acme))
	}
	if homeRequests != 1 || embedRequests != 1 || sharedRequests != 1 {
		t.Fatalf("requests = home:%d embed:%d shared:%d; want each 1", homeRequests, embedRequests, sharedRequests)
	}
}

func TestLayoffsFYIDataCacheSingleflightsConcurrentFetches(t *testing.T) {
	var fetches atomic.Int32
	cache := newLayoffsFYIDataCache(func() ([]byte, error) {
		fetches.Add(1)
		time.Sleep(10 * time.Millisecond)
		return []byte(`{"data":{"table":{"columns":[],"rows":[]}}}`), nil
	})

	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := cache.data(); err != nil {
				t.Errorf("cache.data() error = %v, want nil", err)
			}
		}()
	}
	wg.Wait()

	if got := fetches.Load(); got != 1 {
		t.Fatalf("fetches = %d; want 1", got)
	}
}
