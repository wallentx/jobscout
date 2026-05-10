package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type RemotiveResponse struct {
	Jobs []struct {
		ID                int      `json:"id"`
		URL               string   `json:"url"`
		Title             string   `json:"title"`
		CompanyName       string   `json:"company_name"`
		Category          string   `json:"category"`
		Tags              []string `json:"tags"`
		JobType           string   `json:"job_type"`
		PublicationDate   string   `json:"publication_date"`
		CandidateLocation string   `json:"candidate_required_location"`
		Salary            string   `json:"salary"`
		Description       string   `json:"description"`
	} `json:"jobs"`
}

func fetchRemotive(ctx context.Context, source APISource, criteria *CriteriaConfig) ([]Job, map[string][]Job, error) {
	url := source.URL
	logDebug("api %s: fetching type=%s url=%s", source.Name, source.Type, url)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", jobscoutUserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("remotive API returned status %d", resp.StatusCode)
	}

	var res RemotiveResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, nil, err
	}
	logDebug("api %s: decoded %d remotive jobs", source.Name, len(res.Jobs))

	var jobs []Job
	filtered := make(map[string][]Job)
	for _, j := range res.Jobs {
		comp := j.Salary
		if comp == "" {
			comp = "Not listed"
		}

		pubDate := j.PublicationDate
		if len(pubDate) > 10 {
			pubDate = pubDate[:10]
		}

		job := Job{
			Company:      j.CompanyName,
			Title:        j.Title,
			ApplyURL:     j.URL,
			Source:       formatSearchSource(fetchSearchAPI, source.Name),
			Status:       "Unopened",
			Remote:       j.CandidateLocation,
			Compensation: comp,
			Description:  j.Description,
		}
		job.SetDateFromString(pubDate)
		enrichJobFromDescription(&job)

		if reason := filterJobReason(&job, criteria); reason != "" {
			logDebug("api %s: filtered %s - %s: %s", source.Name, job.Company, job.Title, reason)
			filtered[reason] = append(filtered[reason], job)
			continue
		}
		jobs = append(jobs, job)
	}

	logDebug("api %s: accepted %d; filtered %d", source.Name, len(jobs), countFilteredJobs(filtered))
	return jobs, filtered, nil
}

func fetchJobsFromAPISource(ctx context.Context, source APISource, criteria *CriteriaConfig) ([]Job, map[string][]Job, error) {
	switch strings.ToLower(strings.TrimSpace(source.Type)) {
	case "remotive":
		return fetchRemotive(ctx, source, criteria)
	default:
		return nil, nil, fmt.Errorf("unsupported configured source type %q", source.Type)
	}
}
