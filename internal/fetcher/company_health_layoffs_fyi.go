package fetcher

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	layoffsFYIURL                = "https://layoffs.fyi/"
	layoffsFYIAirtableHost       = "https://airtable.com"
	layoffsFYISharedViewDataPath = "/v0.3/view/"
)

var layoffsFYIData = newLayoffsFYIDataCache(fetchLayoffsFYISharedViewData)

type layoffsFYISharedViewResponse struct {
	Data struct {
		Table struct {
			Columns []layoffsFYIColumn `json:"columns"`
			Rows    []layoffsFYIRow    `json:"rows"`
		} `json:"table"`
	} `json:"data"`
}

type layoffsFYIColumn struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type layoffsFYIRow struct {
	CellValuesByColumnID map[string]any `json:"cellValuesByColumnId"`
}

type layoffsFYIDataCache struct {
	mu    sync.Mutex
	entry *layoffsFYIDataEntry
	fetch func() ([]byte, error)
}

type layoffsFYIDataEntry struct {
	done chan struct{}
	data []byte
	err  error
}

func newLayoffsFYIDataCache(fetch func() ([]byte, error)) *layoffsFYIDataCache {
	return &layoffsFYIDataCache{fetch: fetch}
}

func (c *layoffsFYIDataCache) data() ([]byte, error) {
	if c == nil || c.fetch == nil {
		return fetchLayoffsFYISharedViewData()
	}
	c.mu.Lock()
	if c.entry != nil {
		entry := c.entry
		c.mu.Unlock()
		<-entry.done
		logDebug("company health layoffs.fyi: shared data cache hit bytes=%d err=%t", len(entry.data), entry.err != nil)
		return entry.data, entry.err
	}
	entry := &layoffsFYIDataEntry{done: make(chan struct{})}
	c.entry = entry
	c.mu.Unlock()

	data, err := c.fetch()

	c.mu.Lock()
	entry.data = data
	entry.err = err
	if err != nil {
		c.entry = nil
	}
	close(entry.done)
	c.mu.Unlock()
	logDebug("company health layoffs.fyi: shared data cache stored bytes=%d err=%t", len(data), err != nil)
	return data, err
}

func fetchLayoffsFYISharedViewData() ([]byte, error) {
	homeHTML, err := httpGet(layoffsFYIURL)
	if err != nil {
		return nil, err
	}

	embedURL, err := extractLayoffsFYIAirtableEmbedURL(string(homeHTML))
	if err != nil {
		return nil, err
	}

	embedHTML, err := httpGet(embedURL)
	if err != nil {
		return nil, err
	}

	sharedViewURL, err := extractLayoffsFYISharedViewDataURL(string(embedHTML))
	if err != nil {
		return nil, err
	}

	appID := airtableApplicationID(embedURL)
	headers := map[string]string{
		"x-user-locale":                   "en",
		"x-time-zone":                     "UTC",
		"X-Requested-With":                "XMLHttpRequest",
		"x-airtable-inter-service-client": "webClient",
		"Accept":                          "application/json",
	}
	if appID != "" {
		headers["x-airtable-application-id"] = appID
	}

	data, err := doHTTPGet(sharedViewURL, headers)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func fetchLayoffsFYISignals(company string) ([]LayoffSignal, error) {
	data, err := layoffsFYIData.data()
	if err != nil {
		return nil, err
	}
	return parseLayoffsFYISignals(data, company)
}

func FetchLayoffsFYISignals(company string) ([]LayoffSignal, error) {
	return fetchLayoffsFYISignals(company)
}

func extractLayoffsFYIAirtableEmbedURL(html string) (string, error) {
	re := regexp.MustCompile(`https://airtable\.com/embed/[^"'<> ]+`)
	match := re.FindString(html)
	if match == "" {
		return "", fmt.Errorf("layoffs.fyi Airtable embed URL not found")
	}
	return strings.ReplaceAll(match, "&amp;", "&"), nil
}

func extractLayoffsFYISharedViewDataURL(html string) (string, error) {
	re := regexp.MustCompile(`urlWithParams:\s*"((?:\\.|[^"\\])*)"`)
	matches := re.FindStringSubmatch(html)
	if len(matches) != 2 {
		return "", fmt.Errorf("airtable shared-view data URL not found")
	}

	path, err := strconv.Unquote(`"` + matches[1] + `"`)
	if err != nil {
		return "", fmt.Errorf("decode Airtable shared-view data URL: %w", err)
	}
	if !strings.HasPrefix(path, layoffsFYISharedViewDataPath) {
		return "", fmt.Errorf("unexpected Airtable shared-view data path %q", path)
	}

	return layoffsFYIAirtableHost + path, nil
}

func airtableApplicationID(embedURL string) string {
	parsed, err := url.Parse(embedURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for _, part := range parts {
		if strings.HasPrefix(part, "app") {
			return part
		}
	}
	return ""
}

func parseLayoffsFYISignals(data []byte, company string) ([]LayoffSignal, error) {
	var response layoffsFYISharedViewResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}

	columnIDs := layoffsFYIColumnIDs(response.Data.Table.Columns)
	companyID := columnIDs["Company"]
	if companyID == "" {
		return nil, fmt.Errorf("layoffs.fyi company column not found")
	}

	var signals []LayoffSignal
	targetCompany := cleanCompanyName(company)
	for _, row := range response.Data.Table.Rows {
		rowCompany := stringCellValue(row.CellValuesByColumnID[columnIDs["Company"]])
		if cleanCompanyName(rowCompany) != targetCompany {
			continue
		}
		if signal, ok := layoffsFYIRowSignal(row.CellValuesByColumnID, columnIDs, rowCompany); ok {
			signals = append(signals, signal)
		}
	}

	sort.SliceStable(signals, func(i, j int) bool {
		if signals[i].Date == nil {
			return false
		}
		if signals[j].Date == nil {
			return true
		}
		return signals[i].Date.After(*signals[j].Date)
	})

	return signals, nil
}

func layoffsFYIColumnIDs(columns []layoffsFYIColumn) map[string]string {
	ids := make(map[string]string, len(columns))
	for _, column := range columns {
		ids[column.Name] = column.ID
	}
	return ids
}

func layoffsFYIRowSignal(cells map[string]any, columnIDs map[string]string, company string) (LayoffSignal, bool) {
	count := intCellValue(cells[columnIDs["# Laid Off"]])
	percentage := percentageCellValue(cells[columnIDs["%"]])
	date := timeCellValue(cells[columnIDs["Date"]])
	source := stringCellValue(cells[columnIDs["Source"]])

	title := fmt.Sprintf("Layoffs.fyi: %s layoff record", company)
	if count != nil {
		title = fmt.Sprintf("Layoffs.fyi: %s laid off %d employees", company, *count)
	}
	if percentage != "" {
		title = fmt.Sprintf("%s (%s)", title, percentage)
	}

	return LayoffSignal{
		Title:         title,
		Date:          date,
		URL:           source,
		EmployeeCount: count,
		PercentageStr: percentage,
	}, true
}

func stringCellValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}

func intCellValue(value any) *int {
	switch v := value.(type) {
	case float64:
		if v <= 0 {
			return nil
		}
		out := int(v)
		return &out
	case string:
		v = strings.TrimSpace(strings.ReplaceAll(v, ",", ""))
		if v == "" {
			return nil
		}
		out, err := strconv.Atoi(v)
		if err != nil || out <= 0 {
			return nil
		}
		return &out
	default:
		return nil
	}
}

func timeCellValue(value any) *time.Time {
	raw := stringCellValue(value)
	if raw == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	return &parsed
}

func percentageCellValue(value any) string {
	switch v := value.(type) {
	case float64:
		if v <= 0 {
			return ""
		}
		if v <= 1 {
			v *= 100
		}
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", v), "0"), ".") + "%"
	case string:
		return strings.TrimSpace(v)
	default:
		return ""
	}
}
