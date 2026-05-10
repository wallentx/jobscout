package config

func ApplyFetchSourceSelection(cfg *AppConfig, sources []string) {
	if cfg == nil || len(sources) == 0 {
		return
	}

	selected := make(map[string]bool, len(sources))
	for _, source := range sources {
		selected[source] = true
	}

	wantRSS := selected["rss"]
	wantAPI := selected["api"]
	wantSite := selected["site"]
	wantLLM := selected["llm"]
	wantLLMWeb := selected["llm_web"]
	wantConfiguredSources := wantRSS || wantAPI || wantSite

	if wantLLM || wantLLMWeb {
		cfg.LLM.Enabled = true
	}
	cfg.LLM.JobSearch = wantLLM
	cfg.Sources.Enabled = wantConfiguredSources
	cfg.Sources.RSS.Enabled = wantRSS
	cfg.Sources.SiteSearch.Enabled = wantSite
	cfg.Sources.LLMWeb.Enabled = wantLLMWeb
	cfg.Sources.BuiltinsEnabled = wantSite
	if !wantSite {
		cfg.Sources.SiteSearch.Sites = nil
	}

	cfg.Sources.APIs = append([]APISource(nil), cfg.Sources.APIs...)
	for i := range cfg.Sources.APIs {
		cfg.Sources.APIs[i].Enabled = wantAPI
	}
}
