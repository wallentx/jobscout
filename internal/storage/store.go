package storage

import (
	"context"
	"time"

	"github.com/wallentx/jobscout/internal/domain"
)

type JobStore interface {
	LoadJobs() ([]Job, error)
	SaveJobs(jobs []Job) error
}

type HealthStore interface {
	LoadHealthCache() (HealthCache, error)
	SaveHealthCache(cache HealthCache) error
	GetHealth(company string) (*CompanyHealthResult, time.Time, error)
	SetHealth(company string, result *CompanyHealthResult, fetchedAt time.Time) error
	DeleteHealth(company string) error
	ClearHealthCache() error
}

type CompanyIdentityStore interface {
	UpsertCompanyIdentity(ctx context.Context, identity domain.CompanyIdentity) error
	GetCompanyIdentity(ctx context.Context, companyName string, websiteOrDomain string) (*domain.CompanyIdentity, error)
}

type NoopCompanyIdentityStore struct{}

func (NoopCompanyIdentityStore) UpsertCompanyIdentity(ctx context.Context, identity domain.CompanyIdentity) error {
	return nil
}

func (NoopCompanyIdentityStore) GetCompanyIdentity(ctx context.Context, companyName string, websiteOrDomain string) (*domain.CompanyIdentity, error) {
	return nil, nil
}
