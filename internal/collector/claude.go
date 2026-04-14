package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"gitoken/internal/model"
	"gitoken/internal/store"
)

type ClaudeScanner struct{}

func (s *ClaudeScanner) Provider() model.Provider { return model.ProviderClaude }

func (s *ClaudeScanner) Collect(ctx context.Context, st *store.Store) (CollectResult, error) {
	roots, err := claudeRoots()
	if err != nil {
		return CollectResult{}, err
	}
	files, err := discoverJSONLFiles(roots)
	if err != nil {
		return CollectResult{}, err
	}

	currentPaths := make(map[string]struct{}, len(files))
	result := CollectResult{Provider: model.ProviderClaude}

	for _, file := range files {
		currentPaths[file] = struct{}{}
		info, err := os.Stat(file)
		if err != nil {
			return result, fmt.Errorf("stat claude file %q: %w", file, err)
		}

		state, ok, err := st.FileState(ctx, model.ProviderClaude, file)
		if err != nil {
			return result, err
		}
		if ok && !fileChanged(state, info) {
			result.FilesSkipped++
			continue
		}

		events, err := parseClaudeFile(file)
		if err != nil {
			return result, err
		}
		if err := st.ReplaceFileEvents(ctx, model.ProviderClaude, file, info.Size(), info.ModTime(), events); err != nil {
			return result, err
		}

		result.FilesScanned++
		result.EventsWritten += len(events)
	}

	if err := st.DeleteMissingFiles(ctx, model.ProviderClaude, currentPaths); err != nil {
		return result, err
	}
	return result, nil
}

func parseClaudeFile(path string) ([]model.UsageEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open claude file %q: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	keyed := map[string]model.UsageEvent{}
	var unkeyed []model.UsageEvent
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		var row map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			continue
		}
		if asString(row["type"]) != "assistant" {
			continue
		}

		message, _ := row["message"].(map[string]any)
		if message == nil {
			continue
		}
		usage, _ := message["usage"].(map[string]any)
		if usage == nil {
			continue
		}

		ts, ok := parseTimestamp(asString(row["timestamp"]))
		if !ok {
			continue
		}
		modelName := asString(message["model"])
		if modelName == "" {
			modelName = "unknown"
		}

		inputTokens := maxInt(0, asInt(usage["input_tokens"]))
		cacheWriteTokens := maxInt(0, asInt(usage["cache_creation_input_tokens"]))
		cacheReadTokens := maxInt(0, asInt(usage["cache_read_input_tokens"]))
		outputTokens := maxInt(0, asInt(usage["output_tokens"]))
		if inputTokens == 0 && cacheWriteTokens == 0 && cacheReadTokens == 0 && outputTokens == 0 {
			continue
		}

		messageID := asString(message["id"])
		requestID := asString(row["requestId"])
		event := model.UsageEvent{
			ID:               hashID("claude", path, fmt.Sprintf("%d", lineNumber)),
			Provider:         model.ProviderClaude,
			SourceFile:       path,
			EventTime:        ts,
			Day:              ts.In(time.Local).Format("2006-01-02"),
			Model:            modelName,
			InputTokens:      inputTokens,
			CacheReadTokens:  cacheReadTokens,
			CacheWriteTokens: cacheWriteTokens,
			OutputTokens:     outputTokens,
			TotalTokens:      inputTokens + cacheReadTokens + cacheWriteTokens + outputTokens,
		}

		if messageID != "" && requestID != "" {
			keyed[messageID+":"+requestID] = event
		} else {
			unkeyed = append(unkeyed, event)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan claude file %q: %w", path, err)
	}

	events := make([]model.UsageEvent, 0, len(keyed)+len(unkeyed))
	for _, event := range keyed {
		event.ID = hashID(
			"claude",
			path,
			event.Model,
			event.EventTime.UTC().Format(time.RFC3339Nano),
			fmt.Sprintf("%d", event.TotalTokens),
		)
		events = append(events, event)
	}
	events = append(events, unkeyed...)
	return events, nil
}
