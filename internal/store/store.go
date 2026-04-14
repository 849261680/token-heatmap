package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/849261680/token-heatmap/internal/model"
)

type Store struct {
	db *sql.DB
}

func DefaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".tokenheat", "tokenheat.db"), nil
}

func legacyDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".gitoken", "gitoken.db"), nil
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.migrateLegacyData(context.Background(), path); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS usage_events (
			id TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			source_file TEXT NOT NULL,
			event_time TEXT NOT NULL,
			day TEXT NOT NULL,
			model TEXT NOT NULL,
			input_tokens INTEGER NOT NULL,
			cache_read_tokens INTEGER NOT NULL,
			cache_write_tokens INTEGER NOT NULL,
			output_tokens INTEGER NOT NULL,
			total_tokens INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_day_provider
			ON usage_events(day, provider)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_source_file
			ON usage_events(provider, source_file)`,
		`CREATE TABLE IF NOT EXISTS file_states (
			provider TEXT NOT NULL,
			path TEXT NOT NULL,
			size_bytes INTEGER NOT NULL,
			mod_unix_ms INTEGER NOT NULL,
			PRIMARY KEY (provider, path)
		)`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate sqlite: %w", err)
		}
	}
	return nil
}

func (s *Store) migrateLegacyData(ctx context.Context, currentPath string) error {
	legacyPath, err := legacyDBPath()
	if err != nil {
		return err
	}
	if currentPath == legacyPath {
		return nil
	}
	if _, err := os.Stat(legacyPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat legacy db: %w", err)
	}

	currentCount, err := s.tableCount(ctx, "usage_events")
	if err != nil {
		return err
	}
	if currentCount > 0 {
		return nil
	}

	var legacyCount int
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_database_list WHERE name = 'legacy'`)
	if err := row.Scan(&legacyCount); err != nil {
		return fmt.Errorf("check attached legacy db: %w", err)
	}
	if legacyCount > 0 {
		if _, err := s.db.ExecContext(ctx, `DETACH DATABASE legacy`); err != nil {
			return fmt.Errorf("detach legacy db: %w", err)
		}
	}

	if _, err := s.db.ExecContext(ctx, `ATTACH DATABASE ? AS legacy`, legacyPath); err != nil {
		return fmt.Errorf("attach legacy db: %w", err)
	}
	defer s.db.ExecContext(ctx, `DETACH DATABASE legacy`)

	row = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM legacy.usage_events`)
	if err := row.Scan(&legacyCount); err != nil {
		return fmt.Errorf("count legacy usage events: %w", err)
	}
	if legacyCount == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin legacy migration tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO usage_events
		SELECT * FROM legacy.usage_events
	`); err != nil {
		return fmt.Errorf("copy legacy usage events: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO file_states
		SELECT * FROM legacy.file_states
	`); err != nil {
		return fmt.Errorf("copy legacy file states: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit legacy migration: %w", err)
	}
	return nil
}

func (s *Store) tableCount(ctx context.Context, table string) (int, error) {
	row := s.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table))
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("count table %s: %w", table, err)
	}
	return count, nil
}

type FileState struct {
	Provider  model.Provider
	Path      string
	SizeBytes int64
	ModUnixMS int64
}

func (s *Store) FileState(ctx context.Context, provider model.Provider, path string) (FileState, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT provider, path, size_bytes, mod_unix_ms
		FROM file_states
		WHERE provider = ? AND path = ?`,
		string(provider), path)

	var state FileState
	var providerName string
	if err := row.Scan(&providerName, &state.Path, &state.SizeBytes, &state.ModUnixMS); err != nil {
		if err == sql.ErrNoRows {
			return FileState{}, false, nil
		}
		return FileState{}, false, fmt.Errorf("query file state: %w", err)
	}
	state.Provider = model.Provider(providerName)
	return state, true, nil
}

func (s *Store) ReplaceFileEvents(
	ctx context.Context,
	provider model.Provider,
	path string,
	sizeBytes int64,
	modTime time.Time,
	events []model.UsageEvent,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sqlite tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM usage_events WHERE provider = ? AND source_file = ?`,
		string(provider),
		path,
	); err != nil {
		return fmt.Errorf("delete prior file events: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO usage_events (
			id, provider, source_file, event_time, day, model,
			input_tokens, cache_read_tokens, cache_write_tokens, output_tokens, total_tokens
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare usage event insert: %w", err)
	}
	defer stmt.Close()

	for _, event := range events {
		if _, err := stmt.ExecContext(
			ctx,
			event.ID,
			string(event.Provider),
			event.SourceFile,
			event.EventTime.UTC().Format(time.RFC3339Nano),
			event.Day,
			event.Model,
			event.InputTokens,
			event.CacheReadTokens,
			event.CacheWriteTokens,
			event.OutputTokens,
			event.TotalTokens,
		); err != nil {
			return fmt.Errorf("insert usage event: %w", err)
		}
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO file_states(provider, path, size_bytes, mod_unix_ms)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(provider, path) DO UPDATE SET
		   size_bytes = excluded.size_bytes,
		   mod_unix_ms = excluded.mod_unix_ms`,
		string(provider),
		path,
		sizeBytes,
		modTime.UnixMilli(),
	); err != nil {
		return fmt.Errorf("upsert file state: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite tx: %w", err)
	}
	return nil
}

func (s *Store) DeleteMissingFiles(
	ctx context.Context,
	provider model.Provider,
	currentPaths map[string]struct{},
) error {
	rows, err := s.db.QueryContext(ctx, `SELECT path FROM file_states WHERE provider = ?`, string(provider))
	if err != nil {
		return fmt.Errorf("query provider file states: %w", err)
	}
	defer rows.Close()

	var stale []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return fmt.Errorf("scan provider file state: %w", err)
		}
		if _, ok := currentPaths[path]; !ok {
			stale = append(stale, path)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate provider file states: %w", err)
	}

	if len(stale) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sqlite tx for stale files: %w", err)
	}
	defer tx.Rollback()

	for _, path := range stale {
		if _, err := tx.ExecContext(
			ctx,
			`DELETE FROM usage_events WHERE provider = ? AND source_file = ?`,
			string(provider),
			path,
		); err != nil {
			return fmt.Errorf("delete stale usage events: %w", err)
		}
		if _, err := tx.ExecContext(
			ctx,
			`DELETE FROM file_states WHERE provider = ? AND path = ?`,
			string(provider),
			path,
		); err != nil {
			return fmt.Errorf("delete stale file state: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit stale file cleanup: %w", err)
	}
	return nil
}

func (s *Store) DailyUsageSince(ctx context.Context, sinceDay string) ([]model.DailyUsageRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT day, provider,
		       COALESCE(SUM(input_tokens), 0),
		       COALESCE(SUM(cache_read_tokens), 0),
		       COALESCE(SUM(cache_write_tokens), 0),
		       COALESCE(SUM(output_tokens), 0),
		       COALESCE(SUM(total_tokens), 0)
		FROM usage_events
		WHERE day >= ?
		GROUP BY day, provider
		ORDER BY day ASC, provider ASC`,
		sinceDay,
	)
	if err != nil {
		return nil, fmt.Errorf("query daily usage: %w", err)
	}
	defer rows.Close()

	var out []model.DailyUsageRow
	for rows.Next() {
		var row model.DailyUsageRow
		var providerName string
		if err := rows.Scan(
			&row.Day,
			&providerName,
			&row.InputTokens,
			&row.CacheReadTokens,
			&row.CacheWriteTokens,
			&row.OutputTokens,
			&row.TotalTokens,
		); err != nil {
			return nil, fmt.Errorf("scan daily usage row: %w", err)
		}
		row.Provider = model.Provider(providerName)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily usage rows: %w", err)
	}
	return out, nil
}

func (s *Store) DailyUsageForDay(ctx context.Context, day string) ([]model.DailyUsageRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT day, provider,
		       COALESCE(SUM(input_tokens), 0),
		       COALESCE(SUM(cache_read_tokens), 0),
		       COALESCE(SUM(cache_write_tokens), 0),
		       COALESCE(SUM(output_tokens), 0),
		       COALESCE(SUM(total_tokens), 0)
		FROM usage_events
		WHERE day = ?
		GROUP BY day, provider
		ORDER BY provider ASC`,
		day,
	)
	if err != nil {
		return nil, fmt.Errorf("query day usage: %w", err)
	}
	defer rows.Close()

	var out []model.DailyUsageRow
	for rows.Next() {
		var row model.DailyUsageRow
		var providerName string
		if err := rows.Scan(
			&row.Day,
			&providerName,
			&row.InputTokens,
			&row.CacheReadTokens,
			&row.CacheWriteTokens,
			&row.OutputTokens,
			&row.TotalTokens,
		); err != nil {
			return nil, fmt.Errorf("scan day usage row: %w", err)
		}
		row.Provider = model.Provider(providerName)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate day usage rows: %w", err)
	}
	return out, nil
}
