package domain

import (
	"encoding/json"
	"fmt"
)

// YahooFinanceChart represents Yahoo Finance chart data
type YahooFinanceChart struct {
	Chart struct {
		Result []struct {
			Indicators struct {
				Quote []struct {
					Close []float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
	} `json:"chart"`
}

// fetchStockHistory gets 1-year stock price history from Yahoo Finance
func fetchStockHistory(ticker string) ([]float64, error) {
	data, err := httpGet(fmt.Sprintf(yahooFinanceChartURL, ticker))
	if err != nil {
		return nil, err
	}

	var chart YahooFinanceChart
	if err := json.Unmarshal(data, &chart); err != nil {
		return nil, err
	}

	if len(chart.Chart.Result) == 0 {
		return nil, fmt.Errorf("no chart data")
	}

	quotes := chart.Chart.Result[0].Indicators.Quote
	if len(quotes) == 0 {
		return nil, fmt.Errorf("no quote data")
	}

	closes := quotes[0].Close

	// Filter out nil values (represented as 0 in JSON unmarshaling)
	filtered := []float64{}
	for _, c := range closes {
		if c != 0 {
			filtered = append(filtered, c)
		}
	}

	return filtered, nil
}
