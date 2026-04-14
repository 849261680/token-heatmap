package collector

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseCodexFileComputesDeltasFromTotals(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "sample.jsonl")
	content := `{"timestamp":"2025-09-16T14:47:59.240Z","type":"turn_context","payload":{"model":"gpt-5-codex"}}
{"timestamp":"2025-09-16T14:48:02.551Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":80,"output_tokens":10}}}}
{"timestamp":"2025-09-16T14:49:02.551Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":140,"cached_input_tokens":100,"output_tokens":15}}}}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	events, err := parseCodexFile(path, codexMetadata{}, func(string, time.Time) codexTotals { return codexTotals{} })
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].TotalTokens != 190 {
		t.Fatalf("unexpected first total tokens: %d", events[0].TotalTokens)
	}
	if events[1].InputTokens != 40 || events[1].CacheReadTokens != 20 || events[1].OutputTokens != 5 {
		t.Fatalf("unexpected second delta: %+v", events[1])
	}
}
