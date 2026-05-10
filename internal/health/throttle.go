package health

import (
	"context"
	"time"
)

const (
	companyHealthBrowserConcurrency = 1
	companyHealthLLMConcurrency     = 2
)

var (
	companyHealthBrowserSem = make(chan struct{}, companyHealthBrowserConcurrency)
	companyHealthLLMSem     = make(chan struct{}, companyHealthLLMConcurrency)
)

func runThrottledHealthStep(ctx context.Context, sem chan struct{}, label string, company string, fn func() error) error {
	start := time.Now()
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
	case <-ctx.Done():
		return ctx.Err()
	}
	wait := time.Since(start)
	if wait > 100*time.Millisecond {
		logDebug("%s throttle waited company=%q duration=%s", label, company, wait.Round(time.Millisecond))
	}
	return fn()
}
