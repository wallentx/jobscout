package domain

var (
	negNewsKeywords = []string{
		"layoff", "layoffs", "furlough", "bankrupt", "bankruptcy", "insolv",
		"lawsuit", "settlement", "investigation", "probe", "breach", "hack",
		"outage", "downtime", "restructur", "shutdown", "foreclosure",
		"drop", "drops", "plunge", "plunges", "tumble", "tumbles", "sink", "sinks",
		"weakness", "misses", "missed", "loss", "losses", "cut", "cuts",
		"suspend", "suspends", "risk", "warning", "debt", "crash", "crashes",
		"scandal", "fraud", "resign", "resigns", "fired", "slumps", "slump",
		"rif", "reduction in force", "downsizing",
	}
	hnNegKeywords = []string{
		"layoff", "fired", "toxic", "leaving", "quit", "bad culture", "shuts down",
		"closing", "bankrupt", "lawsuit", "scandal", "security breach",
	}
	posNewsKeywords = []string{
		"acquires", "acquisition", "funding", "raises", "ipo", "profit",
		"partnership", "launches", "expands", "growth", "wins contract",
		"soar", "soars", "surge", "surges", "jump", "jumps", "rally", "rallies",
		"beat", "beats", "record", "strong", "gains", "gain", "climbs",
		"dividend", "buyback", "award", "awarded", "hire", "hiring",
		"breakthrough", "approval", "approved", "upgrades", "upgrade",
	}
	riskySECTerms = []string{
		"going concern", "material weakness", "restatement", "bankruptcy",
		"liquidity", "substantial doubt", "impairment",
	}
)
