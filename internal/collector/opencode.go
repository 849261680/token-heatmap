package collector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/849261680/token-heatmap/internal/model"
	"github.com/849261680/token-heatmap/internal/store"
)

type OpenCodeScanner struct{}

func (s *OpenCodeScanner) Provider() model.Provider { return model.ProviderOpenCode }

func (s *OpenCodeScanner) Collect(ctx context.Context, st *store.Store) (CollectResult, error) {
	dbPath, err := opencodeDBPath()
	if err != nil {
		return CollectResult{}, err
	}

	currentPaths := map[string]struct{}{}
	result := CollectResult{Provider: model.ProviderOpenCode}

	info, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := st.DeleteMissingFiles(ctx, model.ProviderOpenCode, currentPaths); err != nil {
				return result, err
			}
			return result, nil
		}
		return result, fmt.Errorf("stat opencode db %q: %w", dbPath, err)
	}

	currentPaths[dbPath] = struct{}{}
	state, ok, err := st.FileState(ctx, model.ProviderOpenCode, dbPath)
	if err != nil {
		return result, err
	}
	if ok && !fileChanged(state, info) {
		result.FilesSkipped++
		if err := st.DeleteMissingFiles(ctx, model.ProviderOpenCode, currentPaths); err != nil {
			return result, err
		}
		return result, nil
	}

	events, err := parseOpenCodeDB(dbPath)
	if err != nil {
		return result, err
	}
	if err := st.ReplaceFileEvents(ctx, model.ProviderOpenCode, dbPath, info.Size(), info.ModTime(), events); err != nil {
		return result, err
	}

	result.FilesScanned++
	result.EventsWritten += len(events)

	if err := st.DeleteMissingFiles(ctx, model.ProviderOpenCode, currentPaths); err != nil {
		return result, err
	}
	return result, nil
}

type openCodeMessageRow struct {
	Role       string `json:"role"`
	ModelID    string `json:"modelID"`
	ProviderID string `json:"providerID"`
}

type openCodePartRow struct {
	Type   string `json:"type"`
	Tokens struct {
		Total     int `json:"total"`
		Input     int `json:"input"`
		Output    int `json:"output"`
		Reasoning int `json:"reasoning"`
		Cache     struct {
			Write int `json:"write"`
			Read  int `json:"read"`
		} `json:"cache"`
	} `json:"tokens"`
}

func parseOpenCodeDB(path string) ([]model.UsageEvent, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open opencode db %q: %w", path, err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	rows, err := db.Query(`
		SELECT
			part.id,
			part.session_id,
			part.time_created,
			message.data,
			part.data
		FROM part
		JOIN message ON message.id = part.message_id
		ORDER BY part.time_created ASC, part.id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query opencode parts: %w", err)
	}
	defer rows.Close()

	var events []model.UsageEvent
	for rows.Next() {
		var (
			partID      string
			sessionID   string
			timeCreated int64
			messageData string
			partData    string
		)
		if err := rows.Scan(&partID, &sessionID, &timeCreated, &messageData, &partData); err != nil {
			return nil, fmt.Errorf("scan opencode row: %w", err)
		}

		var message openCodeMessageRow
		if err := json.Unmarshal([]byte(messageData), &message); err != nil {
			continue
		}
		if message.Role != "" && message.Role != "assistant" {
			continue
		}

		var part openCodePartRow
		if err := json.Unmarshal([]byte(partData), &part); err != nil {
			continue
		}
		if part.Type != "step-finish" {
			continue
		}

		inputTokens := maxInt(0, part.Tokens.Input)
		cacheReadTokens := maxInt(0, part.Tokens.Cache.Read)
		cacheWriteTokens := maxInt(0, part.Tokens.Cache.Write)
		outputTokens := maxInt(0, part.Tokens.Output)
		totalTokens := maxInt(0, part.Tokens.Total)
		if totalTokens == 0 {
			totalTokens = inputTokens + cacheReadTokens + cacheWriteTokens + outputTokens
		}
		if totalTokens == 0 {
			continue
		}

		eventTime := time.UnixMilli(timeCreated).In(time.Local)
		modelName := firstNonEmpty(message.ModelID, message.ProviderID, "unknown")
		events = append(events, model.UsageEvent{
			ID:               hashID("opencode", path, sessionID, partID),
			Provider:         model.ProviderOpenCode,
			SourceFile:       path,
			EventTime:        eventTime,
			Day:              localDayKey(eventTime),
			Model:            modelName,
			InputTokens:      inputTokens,
			CacheReadTokens:  cacheReadTokens,
			CacheWriteTokens: cacheWriteTokens,
			OutputTokens:     outputTokens,
			TotalTokens:      totalTokens,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate opencode rows: %w", err)
	}
	return events, nil
}
