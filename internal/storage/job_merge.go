package storage

import "github.com/wallentx/jobscout/internal/domain"

func MergeJobs(existing []Job, newJobs []Job) (int, []Job) {
	return domain.MergeJobs(existing, newJobs)
}
