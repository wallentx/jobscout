package fetcher

import "testing"

func TestNormalizeRoleFamilies(t *testing.T) {
	got := normalizeRoleFamilies([]RoleFamilyID{
		RoleDevOpsSRESystems,
		RoleDevOpsSRESystems,
		RoleData,
		RoleFamilyID(""),
		RoleFamilyID("unknown"),
	})

	if len(got) != 2 {
		t.Fatalf("normalizeRoleFamilies() len = %d; want 2", len(got))
	}
	if got[0] != RoleDevOpsSRESystems || got[1] != RoleData {
		t.Fatalf("normalizeRoleFamilies() = %#v; want [devops_sre_systems data]", got)
	}
}

func TestParseRoleFamilyCSV(t *testing.T) {
	got, err := parseRoleFamilyCSV("devops_sre_systems, Data, product management")
	if err != nil {
		t.Fatalf("parseRoleFamilyCSV() error = %v", err)
	}

	want := []RoleFamilyID{RoleDevOpsSRESystems, RoleData, RoleProductManagement}
	if len(got) != len(want) {
		t.Fatalf("parseRoleFamilyCSV() len = %d; want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseRoleFamilyCSV()[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func TestParseRoleFamilyCSVRejectsUnknownValue(t *testing.T) {
	if _, err := parseRoleFamilyCSV("devops_sre_systems, space_law"); err == nil {
		t.Fatal("parseRoleFamilyCSV() error = nil; want unknown role family error")
	}
}

func TestResolveCatalogSourcesProvidesRSSCoverageForCoreRoles(t *testing.T) {
	coreRoles := []RoleFamilyID{
		RoleFrontendEngineering,
		RoleBackendEngineering,
		RoleFullStackEngineering,
		RoleDevOpsSRESystems,
		RoleAIMLEngineering,
		RoleData,
		RoleDesign,
		RoleProductManagement,
		RoleOtherSpecialized,
	}

	requiredGroups := map[SourceGroup]bool{
		SourceGroupRSSRemotive:             true,
		SourceGroupRSSWeWorkRemotely:       true,
		SourceGroupRSSRealWorkFromAnywhere: true,
	}

	for _, role := range coreRoles {
		t.Run(string(role), func(t *testing.T) {
			resolved := resolveCatalogSources([]RoleFamilyID{role})
			seenGroups := make(map[SourceGroup]bool)
			for _, source := range resolved {
				if source.Kind == SourceKindRSS {
					seenGroups[source.Group] = true
				}
			}

			for group := range requiredGroups {
				if !seenGroups[group] {
					t.Fatalf("role %s missing RSS source group %s in %#v", role, group, resolved)
				}
			}
		})
	}
}

func TestRemotiveAIMLFeedUsesCurrentSlug(t *testing.T) {
	resolved := resolveCatalogSources([]RoleFamilyID{RoleAIMLEngineering})
	for _, source := range resolved {
		if source.ID == "rss-remotive-ai-ml" {
			if source.Target != "https://remotive.com/remote-jobs/feed/artificial-intelligence" {
				t.Fatalf("Remotive AI/ML target = %q; want current artificial-intelligence feed slug", source.Target)
			}
			return
		}
	}
	t.Fatalf("resolveCatalogSources(%q) missing rss-remotive-ai-ml source", RoleAIMLEngineering)
}

func TestResolveCatalogSourcesUsesSearchableSiteTargets(t *testing.T) {
	resolved := resolveCatalogSources([]RoleFamilyID{RoleBackendEngineering})
	blocked := map[string]bool{
		"boards.greenhouse.io": true,
		"jobs.lever.co":        true,
		"myworkdayjobs.com":    true,
		"icims.com":            true,
		"smartrecruiters.com":  true,
	}
	seenAggregator := false
	for _, source := range resolved {
		if source.Kind != SourceKindSite {
			continue
		}
		if blocked[source.Target] {
			t.Fatalf("resolveCatalogSources() returned bare ATS site target %q", source.Target)
		}
		if source.Group == SourceGroupSiteAggregator {
			seenAggregator = true
		}
	}
	if !seenAggregator {
		t.Fatalf("resolveCatalogSources() = %#v; want at least one aggregator site target", resolved)
	}
}

func TestSiteSearchTargetPreservesFullURLs(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   string
	}{
		{name: "host", target: "jobs.lever.co", want: "jobs.lever.co"},
		{name: "url", target: "https://builtin.com/jobs/remote", want: "https://builtin.com/jobs/remote"},
		{name: "regional url", target: "https://www.builtinaustin.com", want: "https://www.builtinaustin.com"},
		{name: "empty", target: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := siteSearchTarget(tt.target); got != tt.want {
				t.Fatalf("siteSearchTarget(%q) = %q; want %q", tt.target, got, tt.want)
			}
		})
	}
}

func TestResolveEffectiveSourcesUsesRoleFamiliesWhenPresent(t *testing.T) {
	appCfg := defaultAppConfig()
	criteria := defaultCriteriaConfig()
	criteria.RoleFamilies = []RoleFamilyID{RoleDevOpsSRESystems}

	resolved := resolveEffectiveSources(&appCfg, &criteria)

	if len(resolved.RSSFeeds) != 3 {
		t.Fatalf("resolveEffectiveSources().RSSFeeds len = %d; want 3 role-family RSS feeds", len(resolved.RSSFeeds))
	}
	if len(resolved.APISources) != 0 {
		t.Fatalf("resolveEffectiveSources().APISources len = %d; want 0 catalog API sources", len(resolved.APISources))
	}
	if len(resolved.SiteTargets) == 0 {
		t.Fatal("resolveEffectiveSources().SiteTargets empty; want catalog-backed site targets")
	}

	foundDevopsRSS := false
	for _, feed := range resolved.RSSFeeds {
		if feed.URL == "https://weworkremotely.com/categories/remote-devops-sysadmin-jobs.rss" {
			foundDevopsRSS = true
			break
		}
	}
	if !foundDevopsRSS {
		t.Fatalf("resolveEffectiveSources().RSSFeeds = %#v; want We Work Remotely DevOps feed", resolved.RSSFeeds)
	}
	if !siteTargetsContain(resolved.SiteTargets, "kube.careers") {
		t.Fatalf("resolveEffectiveSources().SiteTargets = %#v; want kube.careers for DevOps role", resolved.SiteTargets)
	}
}

func TestResolveEffectiveSourcesFallsBackToConfigWhenNoRoleFamilies(t *testing.T) {
	appCfg := defaultAppConfig()
	criteria := defaultCriteriaConfig()
	criteria.RoleFamilies = nil

	resolved := resolveEffectiveSources(&appCfg, &criteria)

	if len(resolved.RSSFeeds) != len(appCfg.Sources.RSS.Feeds) {
		t.Fatalf("resolveEffectiveSources().RSSFeeds len = %d; want config fallback len %d", len(resolved.RSSFeeds), len(appCfg.Sources.RSS.Feeds))
	}
	if siteTargetsContain(resolved.SiteTargets, "kube.careers") {
		t.Fatalf("resolveEffectiveSources().SiteTargets = %#v; did not want kube.careers without DevOps role", resolved.SiteTargets)
	}
	wantSiteTargets := len(appCfg.Sources.SiteSearch.Sites)
	if len(resolved.SiteTargets) != wantSiteTargets {
		t.Fatalf("resolveEffectiveSources().SiteTargets len = %d; want config fallback len %d", len(resolved.SiteTargets), wantSiteTargets)
	}
	if len(resolved.LLMWebTargets) != 0 {
		t.Fatalf("resolveEffectiveSources().LLMWebTargets = %#v; want none by default", resolved.LLMWebTargets)
	}
}

func TestResolveEffectiveSourcesIncludesLLMWebTargetsWhenEnabled(t *testing.T) {
	appCfg := defaultAppConfig()
	appCfg.Sources.LLMWeb.Enabled = true
	criteria := defaultCriteriaConfig()

	resolved := resolveEffectiveSources(&appCfg, &criteria)

	if len(resolved.LLMWebTargets) != len(appCfg.Sources.LLMWeb.Targets) {
		t.Fatalf("resolveEffectiveSources().LLMWebTargets len = %d; want config fallback len %d", len(resolved.LLMWebTargets), len(appCfg.Sources.LLMWeb.Targets))
	}
	if !siteTargetsContain(resolved.LLMWebTargets, "site:job-boards.greenhouse.io") {
		t.Fatalf("resolveEffectiveSources().LLMWebTargets = %#v; want web search target", resolved.LLMWebTargets)
	}
}

func TestResolveEffectiveSourcesSkipsKubeCareersWithoutDevOpsRole(t *testing.T) {
	appCfg := defaultAppConfig()
	criteria := defaultCriteriaConfig()
	criteria.RoleFamilies = []RoleFamilyID{RoleFrontendEngineering}

	resolved := resolveEffectiveSources(&appCfg, &criteria)

	if siteTargetsContain(resolved.SiteTargets, "kube.careers") {
		t.Fatalf("resolveEffectiveSources().SiteTargets = %#v; did not want kube.careers for frontend-only criteria", resolved.SiteTargets)
	}
}

func siteTargetsContain(targets []string, want string) bool {
	for _, target := range targets {
		if target == want {
			return true
		}
	}
	return false
}
