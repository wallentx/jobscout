package fetcher

import (
	"net/url"
	"strings"

	"github.com/wallentx/jobscout/internal/domain"
)

type RoleFamilyID = domain.RoleFamilyID

const (
	RoleFrontendEngineering  = domain.RoleFrontendEngineering
	RoleBackendEngineering   = domain.RoleBackendEngineering
	RoleFullStackEngineering = domain.RoleFullStackEngineering
	RoleDevOpsSRESystems     = domain.RoleDevOpsSRESystems
	RoleAIMLEngineering      = domain.RoleAIMLEngineering
	RoleData                 = domain.RoleData
	RoleDesign               = domain.RoleDesign
	RoleProductManagement    = domain.RoleProductManagement
	RoleOtherSpecialized     = domain.RoleOtherSpecialized
)

type SourceKind string

const (
	SourceKindRSS  SourceKind = "rss"
	SourceKindAPI  SourceKind = "api"
	SourceKindSite SourceKind = "site"
)

type SourceGroup string

const (
	SourceGroupRSSRemotive                SourceGroup = "rss_remotive"
	SourceGroupRSSWeWorkRemotely          SourceGroup = "rss_we_work_remotely"
	SourceGroupRSSRealWorkFromAnywhere    SourceGroup = "rss_real_work_from_anywhere"
	SourceGroupAPIRemotive                SourceGroup = "api_remotive"
	SourceGroupSiteAggregator             SourceGroup = "site_aggregator"
	SourceGroupSiteBuiltInRemote          SourceGroup = "site_builtin_remote"
	SourceGroupSiteBuiltInGeneral         SourceGroup = "site_builtin_general"
	SourceGroupSiteBuiltInRegionalListing SourceGroup = "site_builtin_regional_listing"
)

type RoleFamilySpec = domain.RoleFamilySpec

type SourceSpec struct {
	ID           string
	Label        string
	Target       string
	Kind         SourceKind
	Group        SourceGroup
	RoleFamilies []RoleFamilyID
}

func allRoleFamilyIDs() []RoleFamilyID {
	return domain.AllRoleFamilyIDs()
}

func parseRoleFamilyCSV(value string) ([]RoleFamilyID, error) {
	return domain.ParseRoleFamilyCSV(value)
}

func normalizeRoleFamilies(values []RoleFamilyID) []RoleFamilyID {
	return domain.NormalizeRoleFamilies(values)
}

func sourceCatalog() []SourceSpec {
	allRoles := allRoleFamilyIDs()

	return []SourceSpec{
		{
			ID:           "rss-remotive-software-development",
			Label:        "Remotive Software Development RSS",
			Target:       "https://remotive.com/remote-jobs/feed/software-development",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSRemotive,
			RoleFamilies: []RoleFamilyID{RoleFrontendEngineering, RoleBackendEngineering, RoleFullStackEngineering},
		},
		{
			ID:           "rss-remotive-devops",
			Label:        "Remotive DevOps RSS",
			Target:       "https://remotive.com/remote-jobs/feed/devops",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSRemotive,
			RoleFamilies: []RoleFamilyID{RoleDevOpsSRESystems},
		},
		{
			ID:           "rss-remotive-ai-ml",
			Label:        "Remotive AI/ML RSS",
			Target:       "https://remotive.com/remote-jobs/feed/artificial-intelligence",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSRemotive,
			RoleFamilies: []RoleFamilyID{RoleAIMLEngineering},
		},
		{
			ID:           "rss-remotive-data",
			Label:        "Remotive Data RSS",
			Target:       "https://remotive.com/remote-jobs/feed/data",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSRemotive,
			RoleFamilies: []RoleFamilyID{RoleData},
		},
		{
			ID:           "rss-remotive-design",
			Label:        "Remotive Design RSS",
			Target:       "https://remotive.com/remote-jobs/feed/design",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSRemotive,
			RoleFamilies: []RoleFamilyID{RoleDesign},
		},
		{
			ID:           "rss-remotive-product",
			Label:        "Remotive Product RSS",
			Target:       "https://remotive.com/remote-jobs/feed/product",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSRemotive,
			RoleFamilies: []RoleFamilyID{RoleProductManagement},
		},
		{
			ID:           "rss-remotive-other-specialized",
			Label:        "Remotive Other Specialized RSS",
			Target:       "https://remotive.com/remote-jobs/feed/all-others",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSRemotive,
			RoleFamilies: []RoleFamilyID{RoleOtherSpecialized},
		},
		{
			ID:           "rss-wwr-frontend",
			Label:        "We Work Remotely Frontend RSS",
			Target:       "https://weworkremotely.com/categories/remote-front-end-programming-jobs.rss",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSWeWorkRemotely,
			RoleFamilies: []RoleFamilyID{RoleFrontendEngineering},
		},
		{
			ID:           "rss-wwr-backend",
			Label:        "We Work Remotely Backend RSS",
			Target:       "https://weworkremotely.com/categories/remote-back-end-programming-jobs.rss",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSWeWorkRemotely,
			RoleFamilies: []RoleFamilyID{RoleBackendEngineering},
		},
		{
			ID:           "rss-wwr-fullstack",
			Label:        "We Work Remotely Full-Stack RSS",
			Target:       "https://weworkremotely.com/categories/remote-full-stack-programming-jobs.rss",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSWeWorkRemotely,
			RoleFamilies: []RoleFamilyID{RoleFullStackEngineering},
		},
		{
			ID:           "rss-wwr-devops",
			Label:        "We Work Remotely DevOps RSS",
			Target:       "https://weworkremotely.com/categories/remote-devops-sysadmin-jobs.rss",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSWeWorkRemotely,
			RoleFamilies: []RoleFamilyID{RoleDevOpsSRESystems},
		},
		{
			ID:           "rss-wwr-ai-ml",
			Label:        "We Work Remotely Programming RSS",
			Target:       "https://weworkremotely.com/categories/remote-programming-jobs.rss",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSWeWorkRemotely,
			RoleFamilies: []RoleFamilyID{RoleAIMLEngineering, RoleData},
		},
		{
			ID:           "rss-wwr-design",
			Label:        "We Work Remotely Design RSS",
			Target:       "https://weworkremotely.com/categories/remote-design-jobs.rss",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSWeWorkRemotely,
			RoleFamilies: []RoleFamilyID{RoleDesign},
		},
		{
			ID:           "rss-wwr-product",
			Label:        "We Work Remotely Product RSS",
			Target:       "https://weworkremotely.com/categories/remote-product-jobs.rss",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSWeWorkRemotely,
			RoleFamilies: []RoleFamilyID{RoleProductManagement},
		},
		{
			ID:           "rss-wwr-other-specialized",
			Label:        "We Work Remotely Other Specialized RSS",
			Target:       "https://weworkremotely.com/categories/all-other-remote-jobs.rss",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSWeWorkRemotely,
			RoleFamilies: []RoleFamilyID{RoleOtherSpecialized},
		},
		{
			ID:           "rss-rwfa-frontend",
			Label:        "Real Work From Anywhere Frontend RSS",
			Target:       "https://realworkfromanywhere.com/remote-frontend-jobs/rss.xml",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSRealWorkFromAnywhere,
			RoleFamilies: []RoleFamilyID{RoleFrontendEngineering},
		},
		{
			ID:           "rss-rwfa-backend",
			Label:        "Real Work From Anywhere Backend RSS",
			Target:       "https://realworkfromanywhere.com/remote-backend-jobs/rss.xml",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSRealWorkFromAnywhere,
			RoleFamilies: []RoleFamilyID{RoleBackendEngineering},
		},
		{
			ID:           "rss-rwfa-fullstack",
			Label:        "Real Work From Anywhere Full-Stack RSS",
			Target:       "https://realworkfromanywhere.com/remote-fullstack-jobs/rss.xml",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSRealWorkFromAnywhere,
			RoleFamilies: []RoleFamilyID{RoleFullStackEngineering},
		},
		{
			ID:           "rss-rwfa-devops",
			Label:        "Real Work From Anywhere DevOps RSS",
			Target:       "https://realworkfromanywhere.com/remote-devops-jobs/rss.xml",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSRealWorkFromAnywhere,
			RoleFamilies: []RoleFamilyID{RoleDevOpsSRESystems},
		},
		{
			ID:           "rss-rwfa-ai",
			Label:        "Real Work From Anywhere AI RSS",
			Target:       "https://realworkfromanywhere.com/remote-ai-jobs/rss.xml",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSRealWorkFromAnywhere,
			RoleFamilies: []RoleFamilyID{RoleAIMLEngineering},
		},
		{
			ID:           "rss-rwfa-data",
			Label:        "Real Work From Anywhere Data RSS",
			Target:       "https://realworkfromanywhere.com/remote-data-jobs/rss.xml",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSRealWorkFromAnywhere,
			RoleFamilies: []RoleFamilyID{RoleData},
		},
		{
			ID:           "rss-rwfa-design",
			Label:        "Real Work From Anywhere Design RSS",
			Target:       "https://realworkfromanywhere.com/remote-design-jobs/rss.xml",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSRealWorkFromAnywhere,
			RoleFamilies: []RoleFamilyID{RoleDesign},
		},
		{
			ID:           "rss-rwfa-product",
			Label:        "Real Work From Anywhere Product RSS",
			Target:       "https://realworkfromanywhere.com/remote-product-manager-jobs/rss.xml",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSRealWorkFromAnywhere,
			RoleFamilies: []RoleFamilyID{RoleProductManagement},
		},
		{
			ID:           "rss-rwfa-other-specialized",
			Label:        "Real Work From Anywhere Other Specialized RSS",
			Target:       "https://realworkfromanywhere.com/remote-web3-jobs/rss.xml",
			Kind:         SourceKindRSS,
			Group:        SourceGroupRSSRealWorkFromAnywhere,
			RoleFamilies: []RoleFamilyID{RoleOtherSpecialized},
		},
		{
			ID:           "site-indeed",
			Label:        "Indeed Jobs",
			Target:       "https://www.indeed.com/jobs",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteAggregator,
			RoleFamilies: allRoles,
		},
		{
			ID:           "site-linkedin",
			Label:        "LinkedIn Jobs",
			Target:       "https://www.linkedin.com/jobs/search",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteAggregator,
			RoleFamilies: allRoles,
		},
		{
			ID:           "site-ycombinator",
			Label:        "Y Combinator Jobs",
			Target:       "https://www.ycombinator.com/jobs",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteAggregator,
			RoleFamilies: allRoles,
		},
		{
			ID:           "site-himalayas",
			Label:        "Himalayas Remote Jobs",
			Target:       "https://himalayas.app/jobs",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteAggregator,
			RoleFamilies: allRoles,
		},
		{
			ID:           "site-kube-careers",
			Label:        "Kube Careers",
			Target:       "kube.careers",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteAggregator,
			RoleFamilies: []RoleFamilyID{RoleDevOpsSRESystems},
		},
		{
			ID:           "site-builtin-remote",
			Label:        "Built In Remote Tech Jobs",
			Target:       "https://builtin.com/jobs/remote",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteBuiltInRemote,
			RoleFamilies: allRoles,
		},
		{
			ID:           "site-builtin-general",
			Label:        "Built In Tech Jobs",
			Target:       "https://builtin.com/jobs",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteBuiltInGeneral,
			RoleFamilies: allRoles,
		},
		{
			ID:           "site-builtin-austin",
			Label:        "Built In Austin",
			Target:       "https://www.builtinaustin.com",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteBuiltInRegionalListing,
			RoleFamilies: allRoles,
		},
		{
			ID:           "site-builtin-boston",
			Label:        "Built In Boston",
			Target:       "https://www.builtinboston.com",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteBuiltInRegionalListing,
			RoleFamilies: allRoles,
		},
		{
			ID:           "site-builtin-charlotte",
			Label:        "Built In Charlotte",
			Target:       "https://builtincharlotte.com/",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteBuiltInRegionalListing,
			RoleFamilies: allRoles,
		},
		{
			ID:           "site-builtin-chicago",
			Label:        "Built In Chicago",
			Target:       "https://www.builtinchicago.org",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteBuiltInRegionalListing,
			RoleFamilies: allRoles,
		},
		{
			ID:           "site-builtin-colorado",
			Label:        "Built In Colorado",
			Target:       "https://www.builtincolorado.com",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteBuiltInRegionalListing,
			RoleFamilies: allRoles,
		},
		{
			ID:           "site-builtin-los-angeles",
			Label:        "Built In Los Angeles",
			Target:       "https://www.builtinla.com",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteBuiltInRegionalListing,
			RoleFamilies: allRoles,
		},
		{
			ID:           "site-builtin-nyc",
			Label:        "Built In NYC",
			Target:       "https://www.builtinnyc.com",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteBuiltInRegionalListing,
			RoleFamilies: allRoles,
		},
		{
			ID:           "site-builtin-seattle",
			Label:        "Built In Seattle",
			Target:       "https://www.builtinseattle.com",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteBuiltInRegionalListing,
			RoleFamilies: allRoles,
		},
		{
			ID:           "site-builtin-san-francisco",
			Label:        "Built In San Francisco",
			Target:       "https://www.builtinsf.com",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteBuiltInRegionalListing,
			RoleFamilies: allRoles,
		},
		{
			ID:           "site-builtin-singapore",
			Label:        "Built In Singapore",
			Target:       "https://builtinsingapore.com",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteBuiltInRegionalListing,
			RoleFamilies: allRoles,
		},
		{
			ID:           "site-builtin-melbourne",
			Label:        "Built In Melbourne",
			Target:       "https://builtinmelbourne.com",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteBuiltInRegionalListing,
			RoleFamilies: allRoles,
		},
		{
			ID:           "site-builtin-sydney",
			Label:        "Built In Sydney",
			Target:       "https://builtinsydney.au",
			Kind:         SourceKindSite,
			Group:        SourceGroupSiteBuiltInRegionalListing,
			RoleFamilies: allRoles,
		},
	}
}

func resolveCatalogSources(selected []RoleFamilyID) []SourceSpec {
	selected = normalizeRoleFamilies(selected)
	if len(selected) == 0 {
		return nil
	}

	selectedSet := make(map[RoleFamilyID]bool, len(selected))
	for _, role := range selected {
		selectedSet[role] = true
	}

	var resolved []SourceSpec
	for _, source := range sourceCatalog() {
		for _, role := range source.RoleFamilies {
			if selectedSet[role] {
				resolved = append(resolved, source)
				break
			}
		}
	}
	return resolved
}

func resolvedSourceIDs(selected []RoleFamilyID) []string {
	resolved := resolveCatalogSources(selected)
	ids := make([]string, 0, len(resolved))
	for _, source := range resolved {
		ids = append(ids, source.ID)
	}
	return ids
}

func ResolvedSourceIDs(selected []RoleFamilyID) []string {
	return resolvedSourceIDs(selected)
}

type ResolvedSourceSet struct {
	RSSFeeds      []RSSSource
	APISources    []APISource
	SiteTargets   []string
	LLMWebTargets []string
}

func resolveEffectiveSources(appCfg *AppConfig, criteria *CriteriaConfig) ResolvedSourceSet {
	if appCfg == nil {
		return ResolvedSourceSet{}
	}

	var out ResolvedSourceSet
	seenRSS := make(map[string]bool)
	seenAPI := make(map[string]bool)
	seenSite := make(map[string]bool)
	seenLLMWeb := make(map[string]bool)

	if appCfg.Sources.RSS.Enabled {
		for _, source := range appCfg.Sources.RSS.Feeds {
			target := strings.TrimSpace(source.URL)
			if target == "" || seenRSS[target] {
				continue
			}
			seenRSS[target] = true
			out.RSSFeeds = append(out.RSSFeeds, source)
		}
	}
	for _, source := range appCfg.Sources.APIs {
		if !source.Enabled {
			continue
		}
		target := strings.TrimSpace(source.URL)
		if target == "" || seenAPI[target] {
			continue
		}
		seenAPI[target] = true
		out.APISources = append(out.APISources, source)
	}
	roleFamilies := effectiveRoleFamilies(appCfg, criteria)

	if appCfg.Sources.SiteSearch.Enabled {
		for _, source := range appCfg.Sources.SiteSearch.Sites {
			target := siteSearchTarget(source)
			if target == "" || seenSite[target] {
				continue
			}
			if shouldSkipSiteTargetForRoles(target, roleFamilies) {
				logDebug("source resolution: skipped site target %s because DevOps / SRE / Systems is not selected", target)
				continue
			}
			seenSite[target] = true
			out.SiteTargets = append(out.SiteTargets, target)
		}
	}
	if appCfg.Sources.LLMWeb.Enabled {
		for _, source := range appCfg.Sources.LLMWeb.Targets {
			target := llmWebSearchTarget(source)
			if target == "" || seenLLMWeb[target] {
				continue
			}
			seenLLMWeb[target] = true
			out.LLMWebTargets = append(out.LLMWebTargets, target)
		}
	}

	if len(roleFamilies) == 0 {
		return out
	}

	resolved := resolveCatalogSources(roleFamilies)

	for _, source := range resolved {
		switch source.Kind {
		case SourceKindRSS:
			if !appCfg.Sources.RSS.Enabled {
				continue
			}
			if seenRSS[source.Target] {
				continue
			}
			seenRSS[source.Target] = true
			out.RSSFeeds = append(out.RSSFeeds, RSSSource{
				Name: source.Label,
				URL:  source.Target,
			})
		case SourceKindAPI:
			continue
		case SourceKindSite:
			if isBuiltInCatalogGroup(source.Group) {
				if !appCfg.Sources.BuiltinsEnabled {
					continue
				}

				if criteria != nil {
					ws := criteria.Filters.WorkSettings
					onlyRemote := ws.Remote && !ws.Hybrid && !ws.Onsite
					hasHybridOrOnsite := ws.Hybrid || ws.Onsite

					if onlyRemote {
						if source.Group != SourceGroupSiteBuiltInRemote {
							continue
						}
					} else if hasHybridOrOnsite {
						closestTarget := getClosestBuiltInTarget(criteria.Candidate.City, criteria.Candidate.State, criteria.Candidate.CountryCode)

						allowed := false
						if ws.Remote && source.Group == SourceGroupSiteBuiltInRemote {
							allowed = true
						} else if closestTarget != "" {
							if source.Target == closestTarget {
								allowed = true
							}
						} else {
							if source.Group == SourceGroupSiteBuiltInGeneral {
								allowed = true
							}
						}

						if !allowed {
							continue
						}
					}
				}
			} else if !appCfg.Sources.SiteSearch.Enabled {
				continue
			}
			target := siteSearchTarget(source.Target)
			if target == "" || seenSite[target] {
				continue
			}
			if shouldSkipSiteTargetForRoles(target, roleFamilies) {
				logDebug("source resolution: skipped catalog site target %s because DevOps / SRE / Systems is not selected", target)
				continue
			}
			seenSite[target] = true
			out.SiteTargets = append(out.SiteTargets, target)
		}
	}

	return out
}

func ResolveEffectiveSources(appCfg *AppConfig, criteria *CriteriaConfig) ResolvedSourceSet {
	return resolveEffectiveSources(appCfg, criteria)
}

func effectiveRoleFamilies(appCfg *AppConfig, criteria *CriteriaConfig) []RoleFamilyID {
	if criteria != nil && len(criteria.RoleFamilies) > 0 {
		return normalizeRoleFamilies(criteria.RoleFamilies)
	}
	if appCfg != nil && len(appCfg.Criteria.RoleFamilies) > 0 {
		return normalizeRoleFamilies(appCfg.Criteria.RoleFamilies)
	}
	return nil
}

func shouldSkipSiteTargetForRoles(target string, roleFamilies []RoleFamilyID) bool {
	return isKubeCareersTarget(target) && !roleFamilySelected(roleFamilies, RoleDevOpsSRESystems)
}

func roleFamilySelected(roleFamilies []RoleFamilyID, want RoleFamilyID) bool {
	for _, role := range roleFamilies {
		if role == want {
			return true
		}
	}
	return false
}

func isKubeCareersTarget(target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	if strings.Contains(target, "://") {
		u, err := url.Parse(target)
		if err != nil {
			return false
		}
		return strings.EqualFold(strings.TrimPrefix(u.Hostname(), "www."), "kube.careers")
	}
	if host, _, ok := strings.Cut(target, "/"); ok {
		target = host
	}
	return strings.EqualFold(strings.TrimPrefix(target, "www."), "kube.careers")
}

func isBuiltInCatalogGroup(group SourceGroup) bool {
	switch group {
	case SourceGroupSiteBuiltInRemote, SourceGroupSiteBuiltInGeneral, SourceGroupSiteBuiltInRegionalListing:
		return true
	default:
		return false
	}
}

func siteSearchTarget(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(target), "site:") {
		return ""
	}
	if strings.Contains(target, "://") {
		u, err := url.Parse(target)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(u.String())
	}
	return strings.TrimSpace(target)
}

func getClosestBuiltInTarget(city, state, countryCode string) string {
	state = strings.ToUpper(strings.TrimSpace(state))
	city = strings.ToLower(strings.TrimSpace(city))
	countryCode = strings.ToUpper(strings.TrimSpace(countryCode))

	if countryCode == "SG" || countryCode == "SGP" || strings.Contains(strings.ToLower(countryCode), "singapore") {
		return "https://builtinsingapore.com"
	}
	if countryCode == "AU" || countryCode == "AUS" || strings.Contains(strings.ToLower(countryCode), "australia") {
		if state == "VIC" || state == "VICTORIA" || city == "melbourne" {
			return "https://builtinmelbourne.com"
		}
		return "https://builtinsydney.au"
	}

	switch state {
	case "TX", "OK", "NM", "AR", "LA", "TEXAS", "OKLAHOMA", "NEW MEXICO", "ARKANSAS", "LOUISIANA":
		return "https://www.builtinaustin.com"
	case "MA", "ME", "NH", "VT", "RI", "MASSACHUSETTS", "MAINE", "NEW HAMPSHIRE", "VERMONT", "RHODE ISLAND":
		return "https://www.builtinboston.com"
	case "NC", "SC", "VA", "TN", "GA", "FL", "AL", "MS", "WV", "KY", "NORTH CAROLINA", "SOUTH CAROLINA", "VIRGINIA", "TENNESSEE", "GEORGIA", "FLORIDA", "ALABAMA", "MISSISSIPPI", "WEST VIRGINIA", "KENTUCKY":
		return "https://builtincharlotte.com/"
	case "IL", "WI", "IN", "MI", "OH", "IA", "MO", "MN", "ND", "SD", "NE", "KS", "ILLINOIS", "WISCONSIN", "INDIANA", "MICHIGAN", "OHIO", "IOWA", "MISSOURI", "MINNESOTA", "NORTH DAKOTA", "SOUTH DAKOTA", "NEBRASKA", "KANSAS":
		return "https://www.builtinchicago.org"
	case "CO", "WY", "UT", "MT", "COLORADO", "WYOMING", "UTAH", "MONTANA":
		return "https://www.builtincolorado.com"
	case "NY", "NJ", "CT", "PA", "DE", "MD", "DC", "NEW YORK", "NEW JERSEY", "CONNECTICUT", "PENNSYLVANIA", "DELAWARE", "MARYLAND", "DISTRICT OF COLUMBIA":
		return "https://www.builtinnyc.com"
	case "WA", "OR", "ID", "AK", "WASHINGTON", "OREGON", "IDAHO", "ALASKA":
		return "https://www.builtinseattle.com"
	case "CA", "CALIFORNIA":
		if strings.Contains(city, "los angeles") || strings.Contains(city, "san diego") || strings.Contains(city, "irvine") {
			return "https://www.builtinla.com"
		}
		return "https://www.builtinsf.com"
	case "NV", "AZ", "HI", "NEVADA", "ARIZONA", "HAWAII":
		return "https://www.builtinla.com"
	default:
		if strings.Contains(city, "austin") {
			return "https://www.builtinaustin.com"
		}
		if strings.Contains(city, "boston") {
			return "https://www.builtinboston.com"
		}
		if strings.Contains(city, "charlotte") {
			return "https://builtincharlotte.com/"
		}
		if strings.Contains(city, "chicago") {
			return "https://www.builtinchicago.org"
		}
		if strings.Contains(city, "denver") || strings.Contains(city, "boulder") {
			return "https://www.builtincolorado.com"
		}
		if strings.Contains(city, "los angeles") || strings.Contains(city, "la ") {
			return "https://www.builtinla.com"
		}
		if strings.Contains(city, "new york") || strings.Contains(city, "nyc") {
			return "https://www.builtinnyc.com"
		}
		if strings.Contains(city, "seattle") {
			return "https://www.builtinseattle.com"
		}
		if strings.Contains(city, "san francisco") || strings.Contains(city, "sf ") {
			return "https://www.builtinsf.com"
		}
		if strings.Contains(city, "singapore") {
			return "https://builtinsingapore.com"
		}
		if strings.Contains(city, "melbourne") {
			return "https://builtinmelbourne.com"
		}
		if strings.Contains(city, "sydney") {
			return "https://builtinsydney.au"
		}

		return ""
	}
}
