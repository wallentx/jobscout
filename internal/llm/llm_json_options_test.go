package llm

import (
	"context"
	"testing"
)

func TestJSONLLMCallsRequestJSONMode(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name    string
		content string
		run     func(*optionCaptureLLM) error
	}{
		{
			name: "job search",
			content: `[{
				"company": "Example Apps",
				"title": "Backend Developer",
				"remote": "Remote",
				"compensation": "$132,000",
				"apply_url": "https://careers.example.test/jobs/backend-developer-128",
				"description": "Build Go APIs."
			}]`,
			run: func(llm *optionCaptureLLM) error {
				_, _, err := executeLLMSearchWithUsage(ctx, llm, "Find a remote backend job.")
				return err
			},
		},
		{
			name: "job identity",
			content: `{
				"company_website": "https://example.test",
				"company_summary": "Example Apps builds workflow software.",
				"company_industry": "Software",
				"website_confidence": "high",
				"summary_confidence": "high",
				"industry_confidence": "medium",
				"industry_provisional": false,
				"company_website_reason": "The page includes the company domain.",
				"company_summary_reason": "The page describes the product.",
				"company_industry_reason": "Software is stated in the text."
			}`,
			run: func(llm *optionCaptureLLM) error {
				_, _, err := enrichJobIdentityWithLLMUsage(ctx, llm, Job{
					Company: "Example Apps",
					Title:   "Backend Developer",
				}, JobIdentityPage{
					URL:  "https://careers.example.test/jobs/backend-developer-128",
					Text: "Example Apps builds workflow software. Website: https://example.test",
				})
				return err
			},
		},
		{
			name: "company health",
			content: `{
				"summary": "Low risk.",
				"recommendation": "Proceed.",
				"risk_level": "low",
				"positive_signals": ["Stable hiring"],
				"concerns": [],
				"follow_up_questions": []
			}`,
			run: func(llm *optionCaptureLLM) error {
				_, err := evaluateCompanyHealthWithLLM(ctx, llm, &CompanyHealthResult{Company: "Example Apps"})
				return err
			},
		},
		{
			name: "job filter",
			content: `{
				"matches": true,
				"compensation_extracted": "$132,000",
				"remote_eligibility": "Remote",
				"why_it_matches": ["Remote backend role"],
				"why_rejected": []
			}`,
			run: func(llm *optionCaptureLLM) error {
				_, err := evaluateJobWithLLM(ctx, llm, Job{
					Company:      "Example Apps",
					Title:        "Backend Developer",
					Remote:       "Remote",
					Compensation: "$132,000",
					Description:  "Build Go APIs.",
				}, nil)
				return err
			},
		},
		{
			name: "job filter batch",
			content: `{
				"results": {
					"job-1": {
						"matches": true,
						"compensation_extracted": "$132,000",
						"remote_eligibility": "Remote",
						"why_it_matches": ["Remote backend role"],
						"why_rejected": []
					}
				}
			}`,
			run: func(llm *optionCaptureLLM) error {
				_, err := evaluateJobFilterBatchWithLLM(ctx, llm, benchmarkJobFilterBatchInput{
					Jobs: []benchmarkJobFilterBatchEntry{{
						ID: "job-1",
						Job: Job{
							Company:      "Example Apps",
							Title:        "Backend Developer",
							Remote:       "Remote",
							Compensation: "$132,000",
							Description:  "Build Go APIs.",
						},
					}},
				})
				return err
			},
		},
		{
			name: "resume criteria",
			content: `{
				"candidate": {
					"city": "Austin",
					"state": "TX",
					"country_code": "US",
					"years_of_experience": 7
				},
				"role_families": ["devops_sre_systems"],
				"title_requires": [],
				"title_includes": ["Site Reliability Engineer"],
				"title_excludes": [],
				"work_settings": ["remote", "hybrid"],
				"min_base_usd": 130000,
				"priority_signals": ["Kubernetes", "Terraform"]
			}`,
			run: func(llm *optionCaptureLLM) error {
				_, _, err := evaluateResumeCriteriaWithLLMUsage(ctx, llm, "Austin SRE resume")
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			llm := &optionCaptureLLM{content: tc.content}
			if err := tc.run(llm); err != nil {
				t.Fatalf("%s call error = %v", tc.name, err)
			}
			if !llm.options.JSONMode {
				t.Fatalf("%s JSONMode = false, want true", tc.name)
			}
		})
	}
}
