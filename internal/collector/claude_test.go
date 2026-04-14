package collector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseClaudeFileDedupesStreamingChunks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "sample.jsonl")
	content := `{"type":"assistant","message":{"model":"claude-sonnet-4-6","id":"msg_1","usage":{"input_tokens":3,"cache_creation_input_tokens":10,"cache_read_input_tokens":20,"output_tokens":30}},"requestId":"req_1","timestamp":"2026-04-12T04:48:20.228Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4-6","id":"msg_1","usage":{"input_tokens":3,"cache_creation_input_tokens":10,"cache_read_input_tokens":20,"output_tokens":30}},"requestId":"req_1","timestamp":"2026-04-12T04:48:21.309Z"}
{"type":"assistant","message":{"model":"claude-sonnet-4-6","id":"msg_2","usage":{"input_tokens":2,"cache_creation_input_tokens":1,"cache_read_input_tokens":4,"output_tokens":5}},"requestId":"req_2","timestamp":"2026-04-12T04:50:21.309Z"}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	events, err := parseClaudeFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}
