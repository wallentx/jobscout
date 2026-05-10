package storage

import (
	"strings"
	"time"
)

func formatUnixDate(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).Format("2006-01-02")
}

func unixFromDateString(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if len(value) > 10 {
		value = value[:10]
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return 0
	}
	return t.Unix()
}
