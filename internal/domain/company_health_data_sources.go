package domain

type CompanySiteProfile struct {
	SearchQuery string
	SearchURL   string
	WebsiteURL  string
	AboutURL    string
	WebsiteText string
	AboutText   string
}

type CompanyHealthDataSources struct {
	FetchCompanySiteProfile func(company string) (*CompanySiteProfile, error)
	FetchLayoffsFYI         func(company string) ([]LayoffSignal, error)
}
