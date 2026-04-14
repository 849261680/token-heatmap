package model

import "time"

type Provider string

const (
	ProviderCodex  Provider = "codex"
	ProviderClaude Provider = "claude"
)

func (p Provider) IsValid() bool {
	return p == ProviderCodex || p == ProviderClaude
}

type UsageEvent struct {
	ID               string
	Provider         Provider
	SourceFile       string
	EventTime        time.Time
	Day              string
	Model            string
	InputTokens      int
	CacheReadTokens  int
	CacheWriteTokens int
	OutputTokens     int
	TotalTokens      int
}

type DailyUsageRow struct {
	Day              string
	Provider         Provider
	InputTokens      int
	CacheReadTokens  int
	CacheWriteTokens int
	OutputTokens     int
	TotalTokens      int
}
