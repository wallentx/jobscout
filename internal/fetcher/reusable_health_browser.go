package fetcher

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-rod/rod"
	"github.com/wallentx/jobscout/internal/domain"
)

type ReusableHealthBrowser struct {
	mu      sync.Mutex
	browser *rod.Browser
	cleanup func()
	closed  bool
}

func NewReusableHealthBrowser() *ReusableHealthBrowser {
	return &ReusableHealthBrowser{}
}

func (s *ReusableHealthBrowser) FetchCompanySiteProfile(ctx context.Context, identity domain.CompanyHealthContext) (*domain.CompanySiteProfile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	identity.Company = strings.TrimSpace(identity.Company)
	identity.Website = strings.TrimSpace(identity.Website)
	if identity.Company == "" && identity.Website == "" {
		return nil, nil
	}
	browser, err := s.getBrowser()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, companyProfileBrowserTimeout)
	defer cancel()
	return discoverCompanySiteProfileForIdentity(ctx, browser, identity), nil
}

func (s *ReusableHealthBrowser) FetchEmployerReviewSignals(ctx context.Context, company string) ([]domain.EmployerReviewSignal, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	company = strings.TrimSpace(company)
	if company == "" {
		return nil, nil
	}
	browser, err := s.getBrowser()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, companyProfileBrowserTimeout)
	defer cancel()
	return discoverEmployerReviewSignals(ctx, browser, company), nil
}

func (s *ReusableHealthBrowser) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeLocked()
}

func (s *ReusableHealthBrowser) getBrowser() (*rod.Browser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, fmt.Errorf("reusable health browser is closed")
	}
	if s.browser != nil {
		return s.browser, nil
	}
	if FindSiteSearchBrowserBinary() == "" {
		return nil, ErrBrowserNotInstalled
	}

	browser, cleanup, err := NewSiteSearchBrowser()
	if err != nil {
		return nil, err
	}
	logDebug("health browser reuse started")
	s.browser = browser
	s.cleanup = cleanup
	return s.browser, nil
}

func (s *ReusableHealthBrowser) closeLocked() {
	if s.cleanup != nil {
		s.cleanup()
		logDebug("health browser reuse cleanup complete")
	}
	s.browser = nil
	s.cleanup = nil
	s.closed = true
}
