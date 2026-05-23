package health

import (
	"context"

	"github.com/wallentx/jobscout/internal/domain"
)

type BrowserSession interface {
	FetchCompanySiteProfile(context.Context, domain.CompanyHealthContext) (*domain.CompanySiteProfile, error)
	FetchEmployerReviewSignals(context.Context, string) ([]domain.EmployerReviewSignal, error)
	FetchArticleText(context.Context, string) (string, error)
}

type browserSessionContextKey struct{}

func ContextWithBrowserSession(ctx context.Context, session BrowserSession) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if session == nil {
		return ctx
	}
	return context.WithValue(ctx, browserSessionContextKey{}, session)
}

func browserSessionFromContext(ctx context.Context) BrowserSession {
	if ctx == nil {
		return nil
	}
	session, _ := ctx.Value(browserSessionContextKey{}).(BrowserSession)
	return session
}
