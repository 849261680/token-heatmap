package model

import "time"

type Provider string

const (
	ProviderCodex    Provider = "codex"
	ProviderClaude   Provider = "claude"
	ProviderOpenCode Provider = "opencode"
)

func (p Provider) IsValid() bool {
	return p == ProviderCodex || p == ProviderClaude || p == ProviderOpenCode
}

type UsageEvent struct {
	ID               string    `json:"id"`
	Provider         Provider  `json:"provider"`
	SourceFile       string    `json:"source_file"`
	EventTime        time.Time `json:"event_time"`
	Day              string    `json:"day"`
	Model            string    `json:"model"`
	InputTokens      int       `json:"input_tokens"`
	CacheReadTokens  int       `json:"cache_read_tokens"`
	CacheWriteTokens int       `json:"cache_write_tokens"`
	OutputTokens     int       `json:"output_tokens"`
	TotalTokens      int       `json:"total_tokens"`
}

type DailyUsageRow struct {
	Day              string   `json:"day"`
	Provider         Provider `json:"provider"`
	InputTokens      int      `json:"input_tokens"`
	CacheReadTokens  int      `json:"cache_read_tokens"`
	CacheWriteTokens int      `json:"cache_write_tokens"`
	OutputTokens     int      `json:"output_tokens"`
	TotalTokens      int      `json:"total_tokens"`
}
