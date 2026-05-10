package fetcher

func mergeRejectedBySearch(dst map[string]map[string][]string, searchType string, src map[string][]string) map[string]map[string][]string {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]map[string][]string)
	}
	grouped := dst[searchType]
	if grouped == nil {
		grouped = make(map[string][]string, len(src))
	}
	for reason, entries := range src {
		grouped[reason] = append(grouped[reason], entries...)
	}
	dst[searchType] = grouped
	return dst
}

func mergeFiltered(dst map[string][]Job, src map[string][]Job) map[string][]Job {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string][]Job, len(src))
	}
	for reason, entries := range src {
		dst[reason] = append(dst[reason], entries...)
	}
	return dst
}
