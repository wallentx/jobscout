package fetcher

import (
	"regexp"
)

var (
	whitespaceRe  = regexp.MustCompile(`\s+`)
	salaryRangeRe = regexp.MustCompile(`(?i)(?:\$|USD\s*)\s*\d{2,3}(?:,\d{3})?\s*[kK]?(?:\s*(?:-|–|to)\s*(?:\$|USD\s*)?\d{2,3}(?:,\d{3})?\s*[kK]?)?\s*(?:USD|/year|per year|annually|annual)?`)
)
