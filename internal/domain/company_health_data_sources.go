package domain

type CompanySiteProfile struct {
	SearchQuery    string
	SearchURL      string
	WebsiteURL     string
	AboutURL       string
	WebsiteText    string
	AboutText      string
	Summary        string
	Industry       string
	PublicProfiles []CompanyPublicProfile
}

type CompanyPublicProfile struct {
	Source             string `json:"source"`
	URL                string `json:"url,omitempty"`
	Title              string `json:"title,omitempty"`
	Snippet            string `json:"snippet,omitempty"`
	Industry           string `json:"industry,omitempty"`
	EmployeeRange      string `json:"employee_range,omitempty"`
	EstimatedEmployees *int   `json:"estimated_employees,omitempty"`
	FoundedYear        *int   `json:"founded_year,omitempty"`
	Headquarters       string `json:"headquarters,omitempty"`
	Revenue            string `json:"revenue,omitempty"`
	Rating             string `json:"rating,omitempty"`
	ReviewCount        *int   `json:"review_count,omitempty"`
	RecommendPercent   *int   `json:"recommend_percent,omitempty"`
	CEOApprovalPercent *int   `json:"ceo_approval_percent,omitempty"`
}

type CompanyHealthDataSources struct {
	FetchCompanySiteProfile func(identity CompanyHealthContext) (*CompanySiteProfile, error)
	FetchLayoffsFYI         func(company string) ([]LayoffSignal, error)
}
