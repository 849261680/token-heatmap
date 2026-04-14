package collector

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestParseOpenCodeDBReadsStepFinishTokens(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	stmts := []string{
		`CREATE TABLE message (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			time_created INTEGER NOT NULL,
			time_updated INTEGER NOT NULL,
			data TEXT NOT NULL
		)`,
		`CREATE TABLE part (
			id TEXT PRIMARY KEY,
			message_id TEXT NOT NULL,
			session_id TEXT NOT NULL,
			time_created INTEGER NOT NULL,
			time_updated INTEGER NOT NULL,
			data TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	messageJSON := `{"role":"assistant","modelID":"deepseek/deepseek-v3.2","providerID":"deepseek"}`
	stepStartJSON := `{"type":"step-start"}`
	stepFinishJSON := `{"type":"step-finish","tokens":{"total":30865,"input":30533,"output":332,"reasoning":0,"cache":{"write":0,"read":0}},"cost":0}`
	textJSON := `{"type":"text","text":"hello"}`

	if _, err := db.Exec(
		`INSERT INTO message(id, session_id, time_created, time_updated, data) VALUES (?, ?, ?, ?, ?)`,
		"msg_1", "ses_1", int64(1776069954512), int64(1776069975877), messageJSON,
	); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		id   string
		data string
	}{
		{"prt_start", stepStartJSON},
		{"prt_text", textJSON},
		{"prt_finish", stepFinishJSON},
	} {
		if _, err := db.Exec(
			`INSERT INTO part(id, message_id, session_id, time_created, time_updated, data) VALUES (?, ?, ?, ?, ?, ?)`,
			row.id, "msg_1", "ses_1", int64(1776069975877), int64(1776069975877), row.data,
		); err != nil {
			t.Fatal(err)
		}
	}

	events, err := parseOpenCodeDB(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	event := events[0]
	if event.Provider != "opencode" {
		t.Fatalf("unexpected provider: %s", event.Provider)
	}
	if event.Model != "deepseek/deepseek-v3.2" {
		t.Fatalf("unexpected model: %s", event.Model)
	}
	if event.InputTokens != 30533 || event.OutputTokens != 332 || event.TotalTokens != 30865 {
		t.Fatalf("unexpected token counts: %+v", event)
	}
}

func TestOpenCodeDBPathUsesXDGDataHome(t *testing.T) {
	t.Setenv("OPENCODE_DATA_DIR", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	path, err := opencodeDBPath()
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(os.Getenv("XDG_DATA_HOME"), "opencode", "opencode.db")
	if path != expected {
		t.Fatalf("expected %q, got %q", expected, path)
	}
}
