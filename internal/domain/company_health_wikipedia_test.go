package domain

import "testing"

func TestParseWikidataCompanyFacts(t *testing.T) {
	data := []byte(`{
		"entities": {
			"Q21708200": {
				"claims": {
					"P571": [
						{
							"rank": "normal",
							"mainsnak": {
								"datavalue": {
									"value": {"time": "+2015-12-11T00:00:00Z"}
								}
							}
						}
					],
					"P1128": [
						{
							"rank": "normal",
							"mainsnak": {
								"datavalue": {
									"value": {"amount": "+1700"}
								}
							},
							"qualifiers": {
								"P585": [
									{
										"datavalue": {
											"value": {"time": "+2023-01-01T00:00:00Z"}
										}
									}
								]
							}
						},
						{
							"rank": "normal",
							"mainsnak": {
								"datavalue": {
									"value": {"amount": "+4500"}
								}
							},
							"qualifiers": {
								"P585": [
									{
										"datavalue": {
											"value": {"time": "+2025-01-01T00:00:00Z"}
										}
									}
								]
							}
						}
					],
					"P249": [
						{
							"rank": "normal",
							"mainsnak": {
								"datavalue": {
									"value": "ACME"
								}
							}
						}
					]
				}
			}
		}
	}`)

	facts, err := parseWikidataCompanyFacts("Q21708200", data)
	if err != nil {
		t.Fatalf("parseWikidataCompanyFacts() error = %v", err)
	}
	if facts.EntityURL != "https://www.wikidata.org/wiki/Q21708200" {
		t.Fatalf("EntityURL = %q, want Wikidata entity URL", facts.EntityURL)
	}
	if facts.FoundedYear == nil || *facts.FoundedYear != 2015 {
		t.Fatalf("FoundedYear = %#v, want 2015", facts.FoundedYear)
	}
	if facts.EmployeeCount == nil || *facts.EmployeeCount != 4500 {
		t.Fatalf("EmployeeCount = %#v, want latest employee count 4500", facts.EmployeeCount)
	}
	if facts.TickerSymbol == nil || *facts.TickerSymbol != "ACME" {
		t.Fatalf("TickerSymbol = %#v, want ACME", facts.TickerSymbol)
	}
	if facts.FoundedYearClaimCount != 1 {
		t.Fatalf("FoundedYearClaimCount = %d, want 1", facts.FoundedYearClaimCount)
	}
	if facts.EmployeeCountClaimCount != 2 {
		t.Fatalf("EmployeeCountClaimCount = %d, want 2", facts.EmployeeCountClaimCount)
	}
	if facts.TickerClaimCount != 1 {
		t.Fatalf("TickerClaimCount = %d, want 1", facts.TickerClaimCount)
	}
}

func TestParseWikidataCompanyFactsTracksMissingClaims(t *testing.T) {
	data := []byte(`{
		"entities": {
			"Q1": {
				"claims": {
					"P571": [],
					"P1128": [
						{
							"rank": "deprecated",
							"mainsnak": {
								"datavalue": {
									"value": {"amount": "+100"}
								}
							}
						}
					],
					"P249": [
						{
							"rank": "deprecated",
							"mainsnak": {
								"datavalue": {
									"value": "OLD"
								}
							}
						}
					]
				}
			}
		}
	}`)

	facts, err := parseWikidataCompanyFacts("Q1", data)
	if err != nil {
		t.Fatalf("parseWikidataCompanyFacts() error = %v", err)
	}
	if facts.FoundedYear != nil {
		t.Fatalf("FoundedYear = %#v, want nil", facts.FoundedYear)
	}
	if facts.EmployeeCount != nil {
		t.Fatalf("EmployeeCount = %#v, want nil for deprecated-only claim", facts.EmployeeCount)
	}
	if facts.TickerSymbol != nil {
		t.Fatalf("TickerSymbol = %#v, want nil for deprecated-only claim", facts.TickerSymbol)
	}
	if facts.FoundedYearClaimCount != 0 {
		t.Fatalf("FoundedYearClaimCount = %d, want 0", facts.FoundedYearClaimCount)
	}
	if facts.EmployeeCountClaimCount != 1 {
		t.Fatalf("EmployeeCountClaimCount = %d, want 1", facts.EmployeeCountClaimCount)
	}
	if facts.TickerClaimCount != 1 {
		t.Fatalf("TickerClaimCount = %d, want 1", facts.TickerClaimCount)
	}
}

func TestParseWikidataCompanyFactsGetsTickerFromStockExchangeQualifier(t *testing.T) {
	data := []byte(`{
		"entities": {
			"Q182477": {
				"claims": {
					"P414": [
						{
							"rank": "normal",
							"mainsnak": {
								"datavalue": {
									"value": {
										"entity-type": "item",
										"numeric-id": 82059,
										"id": "Q82059"
									}
								}
							},
							"qualifiers": {
								"P249": [
									{
										"datavalue": {
											"value": "NVDA"
										}
									}
								]
							}
						}
					]
				}
			}
		}
	}`)

	facts, err := parseWikidataCompanyFacts("Q182477", data)
	if err != nil {
		t.Fatalf("parseWikidataCompanyFacts() error = %v", err)
	}
	if facts.TickerSymbol == nil || *facts.TickerSymbol != "NVDA" {
		t.Fatalf("TickerSymbol = %#v, want NVDA", facts.TickerSymbol)
	}
	if facts.TickerClaimCount != 1 {
		t.Fatalf("TickerClaimCount = %d, want 1", facts.TickerClaimCount)
	}
}
