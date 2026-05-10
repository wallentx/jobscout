package domain

import (
	"strings"
	"testing"
)

func TestDefaultCompanyHealthUserAgentIncludesSECContact(t *testing.T) {
	t.Setenv(userAgentEnv, "")

	got := companyHealthUserAgent()
	if !strings.Contains(strings.ToLower(got), "jobscout") {
		t.Fatalf("companyHealthUserAgent() = %q; want app name", got)
	}
	if !strings.Contains(got, "wallentx@linux.com") {
		t.Fatalf("companyHealthUserAgent() = %q; want SEC contact email", got)
	}
}

func TestCompanyHealthUserAgentUsesConfiguredValue(t *testing.T) {
	t.Setenv(userAgentEnv, "JobScout Tests tests@example.invalid")

	if got := companyHealthUserAgent(); got != "JobScout Tests tests@example.invalid" {
		t.Fatalf("companyHealthUserAgent() = %q; want configured value", got)
	}
}
