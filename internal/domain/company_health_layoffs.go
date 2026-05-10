package domain

import (
	"strings"
	"time"
)

func mergeLayoffSignals(primary []LayoffSignal, extra []LayoffSignal) []LayoffSignal {
	if len(primary) == 0 {
		return extra
	}
	merged := append([]LayoffSignal{}, primary...)
	seen := make(map[string]bool, len(primary))
	for _, signal := range primary {
		seen[layoffSignalKey(signal)] = true
	}
	for _, signal := range extra {
		key := layoffSignalKey(signal)
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, signal)
	}
	return merged
}

func MergeLayoffSignals(primary []LayoffSignal, extra []LayoffSignal) []LayoffSignal {
	return mergeLayoffSignals(primary, extra)
}

func layoffSignalKey(signal LayoffSignal) string {
	if signal.URL != "" {
		return strings.ToLower(signal.URL)
	}
	date := ""
	if signal.Date != nil {
		date = signal.Date.Format(time.DateOnly)
	}
	return strings.ToLower(signal.Title) + "|" + date
}
