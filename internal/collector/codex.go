package collector

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/849261680/token-heatmap/internal/model"
	"github.com/849261680/token-heatmap/internal/store"
)

type CodexScanner struct{}

func (s *CodexScanner) Provider() model.Provider { return model.ProviderCodex }

type codexMetadata struct {
	SessionID     string
	ForkedFromID  string
	ForkTimestamp string
}

type codexTotals struct {
	Input  int
	Cached int
	Output int
}

type codexSnapshot struct {
	Timestamp time.Time
	Totals    codexTotals
}

func (s *CodexScanner) Collect(ctx context.Context, st *store.Store) (CollectResult, error) {
	roots, err := codexRoots()
	if err != nil {
		return CollectResult{}, err
	}
	files, err := discoverJSONLFiles(roots)
	if err != nil {
		return CollectResult{}, err
	}

	sessionFiles := map[string]string{}
	metadataByFile := map[string]codexMetadata{}
	for _, file := range files {
		meta, _ := readCodexMetadata(file)
		metadataByFile[file] = meta
		if meta.SessionID != "" {
			sessionFiles[meta.SessionID] = file
		}
	}

	snapshotCache := map[string][]codexSnapshot{}
	resolveInherited := func(sessionID string, cutoff time.Time) codexTotals {
		parentFile := sessionFiles[sessionID]
		if parentFile == "" {
			return codexTotals{}
		}

		snapshots, ok := snapshotCache[parentFile]
		if !ok {
			snaps, _ := readCodexSnapshots(parentFile)
			snapshots = snaps
			snapshotCache[parentFile] = snapshots
		}

		var inherited codexTotals
		for _, snapshot := range snapshots {
			if snapshot.Timestamp.After(cutoff) {
				break
			}
			inherited = snapshot.Totals
		}
		return inherited
	}

	currentPaths := make(map[string]struct{}, len(files))
	result := CollectResult{Provider: model.ProviderCodex}

	for _, file := range files {
		currentPaths[file] = struct{}{}
		info, err := os.Stat(file)
		if err != nil {
			return result, fmt.Errorf("stat codex file %q: %w", file, err)
		}

		state, ok, err := st.FileState(ctx, model.ProviderCodex, file)
		if err != nil {
			return result, err
		}
		if ok && !fileChanged(state, info) {
			result.FilesSkipped++
			continue
		}

		events, err := parseCodexFile(file, metadataByFile[file], resolveInherited)
		if err != nil {
			return result, err
		}
		if err := st.ReplaceFileEvents(ctx, model.ProviderCodex, file, info.Size(), info.ModTime(), events); err != nil {
			return result, err
		}

		result.FilesScanned++
		result.EventsWritten += len(events)
	}

	if err := st.DeleteMissingFiles(ctx, model.ProviderCodex, currentPaths); err != nil {
		return result, err
	}

	return result, nil
}

func readCodexMetadata(path string) (codexMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return codexMetadata{}, fmt.Errorf("open codex metadata file %q: %w", path, err)
	}
	defer file.Close()

	meta := codexMetadata{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	for scanner.Scan() {
		var row map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			continue
		}
		if asString(row["type"]) != "session_meta" {
			continue
		}

		payload, _ := row["payload"].(map[string]any)
		meta.SessionID = firstNonEmpty(
			asString(payload["session_id"]),
			asString(payload["sessionId"]),
			asString(payload["id"]),
			asString(row["session_id"]),
			asString(row["sessionId"]),
			asString(row["id"]),
		)
		meta.ForkedFromID = firstNonEmpty(
			asString(payload["forked_from_id"]),
			asString(payload["forkedFromId"]),
			asString(payload["parent_session_id"]),
			asString(payload["parentSessionId"]),
		)
		meta.ForkTimestamp = firstNonEmpty(
			asString(payload["timestamp"]),
			asString(row["timestamp"]),
		)
		break
	}

	if err := scanner.Err(); err != nil {
		return codexMetadata{}, fmt.Errorf("scan codex metadata file %q: %w", path, err)
	}
	if meta.SessionID == "" {
		meta.SessionID = parseCodexSessionIDFromPath(path)
	}
	return meta, nil
}

func readCodexSnapshots(path string) ([]codexSnapshot, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open codex snapshot file %q: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	var snapshots []codexSnapshot
	var previous codexTotals
	for scanner.Scan() {
		var row map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			continue
		}
		if asString(row["type"]) != "event_msg" {
			continue
		}
		payload, _ := row["payload"].(map[string]any)
		if asString(payload["type"]) != "token_count" {
			continue
		}
		info, _ := payload["info"].(map[string]any)
		ts, ok := parseTimestamp(asString(row["timestamp"]))
		if !ok {
			continue
		}

		if totalUsage, ok := info["total_token_usage"].(map[string]any); ok {
			previous = codexTotals{
				Input:  asInt(totalUsage["input_tokens"]),
				Cached: asInt(firstNonNil(totalUsage["cached_input_tokens"], totalUsage["cache_read_input_tokens"])),
				Output: asInt(totalUsage["output_tokens"]),
			}
			snapshots = append(snapshots, codexSnapshot{Timestamp: ts, Totals: previous})
			continue
		}

		if lastUsage, ok := info["last_token_usage"].(map[string]any); ok {
			previous = codexTotals{
				Input: previous.Input + asInt(lastUsage["input_tokens"]),
				Cached: previous.Cached +
					asInt(firstNonNil(lastUsage["cached_input_tokens"], lastUsage["cache_read_input_tokens"])),
				Output: previous.Output + asInt(lastUsage["output_tokens"]),
			}
			snapshots = append(snapshots, codexSnapshot{Timestamp: ts, Totals: previous})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan codex snapshot file %q: %w", path, err)
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp.Before(snapshots[j].Timestamp)
	})
	return snapshots, nil
}

func parseCodexFile(
	path string,
	meta codexMetadata,
	resolveInherited func(sessionID string, cutoff time.Time) codexTotals,
) ([]model.UsageEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open codex file %q: %w", path, err)
	}
	defer file.Close()

	var inherited codexTotals
	var inheritedRemaining codexTotals
	if meta.ForkedFromID != "" {
		if cutoff, ok := parseTimestamp(meta.ForkTimestamp); ok {
			inherited = resolveInherited(meta.ForkedFromID, cutoff)
			inheritedRemaining = inherited
		}
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	var (
		currentModel  = "unknown"
		previous      codexTotals
		events        []model.UsageEvent
		lineNumber    int
		localTimezone = time.Local
	)

	for scanner.Scan() {
		lineNumber++

		var row map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			continue
		}

		rowType := asString(row["type"])
		switch rowType {
		case "turn_context":
			payload, _ := row["payload"].(map[string]any)
			if modelName := firstNonEmpty(
				asString(payload["model"]),
				asString(mapStringAny(payload["info"])["model"]),
			); modelName != "" {
				currentModel = modelName
			}
		case "event_msg":
			payload, _ := row["payload"].(map[string]any)
			if asString(payload["type"]) != "token_count" {
				continue
			}

			info, _ := payload["info"].(map[string]any)
			ts, ok := parseTimestamp(asString(row["timestamp"]))
			if !ok {
				continue
			}

			modelName := firstNonEmpty(
				asString(info["model"]),
				asString(info["model_name"]),
				asString(payload["model"]),
				asString(row["model"]),
				currentModel,
			)
			if modelName == "" {
				modelName = "unknown"
			}

			var delta codexTotals
			if totalUsage, ok := info["total_token_usage"].(map[string]any); ok {
				rawTotals := codexTotals{
					Input:  asInt(totalUsage["input_tokens"]),
					Cached: asInt(firstNonNil(totalUsage["cached_input_tokens"], totalUsage["cache_read_input_tokens"])),
					Output: asInt(totalUsage["output_tokens"]),
				}
				currentTotals := codexTotals{
					Input:  maxInt(0, rawTotals.Input-inherited.Input),
					Cached: maxInt(0, rawTotals.Cached-inherited.Cached),
					Output: maxInt(0, rawTotals.Output-inherited.Output),
				}
				delta = codexTotals{
					Input:  maxInt(0, currentTotals.Input-previous.Input),
					Cached: maxInt(0, currentTotals.Cached-previous.Cached),
					Output: maxInt(0, currentTotals.Output-previous.Output),
				}
				previous = currentTotals
				inheritedRemaining = codexTotals{}
			} else if lastUsage, ok := info["last_token_usage"].(map[string]any); ok {
				rawDelta := codexTotals{
					Input:  maxInt(0, asInt(lastUsage["input_tokens"])),
					Cached: maxInt(0, asInt(firstNonNil(lastUsage["cached_input_tokens"], lastUsage["cache_read_input_tokens"]))),
					Output: maxInt(0, asInt(lastUsage["output_tokens"])),
				}
				delta = codexTotals{
					Input:  maxInt(0, rawDelta.Input-inheritedRemaining.Input),
					Cached: maxInt(0, rawDelta.Cached-inheritedRemaining.Cached),
					Output: maxInt(0, rawDelta.Output-inheritedRemaining.Output),
				}
				inheritedRemaining = codexTotals{
					Input:  maxInt(0, inheritedRemaining.Input-rawDelta.Input),
					Cached: maxInt(0, inheritedRemaining.Cached-rawDelta.Cached),
					Output: maxInt(0, inheritedRemaining.Output-rawDelta.Output),
				}
				previous = codexTotals{
					Input:  previous.Input + delta.Input,
					Cached: previous.Cached + delta.Cached,
					Output: previous.Output + delta.Output,
				}
			}

			if delta.Input == 0 && delta.Cached == 0 && delta.Output == 0 {
				continue
			}

			cached := minInt(delta.Cached, delta.Input)
			event := model.UsageEvent{
				ID:               hashID("codex", path, fmt.Sprintf("%d", lineNumber)),
				Provider:         model.ProviderCodex,
				SourceFile:       path,
				EventTime:        ts,
				Day:              ts.In(localTimezone).Format("2006-01-02"),
				Model:            modelName,
				InputTokens:      delta.Input,
				CacheReadTokens:  cached,
				CacheWriteTokens: 0,
				OutputTokens:     delta.Output,
				TotalTokens:      delta.Input + cached + delta.Output,
			}
			events = append(events, event)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan codex file %q: %w", path, err)
	}
	return events, nil
}

func parseCodexSessionIDFromPath(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	parts := strings.Split(base, "-")
	if len(parts) >= 6 {
		return strings.Join(parts[len(parts)-5:], "-")
	}
	return ""
}

func hashID(parts ...string) string {
	sum := sha1.Sum([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func mapStringAny(value any) map[string]any {
	mapped, _ := value.(map[string]any)
	return mapped
}

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func asInt(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	case json.Number:
		i, _ := typed.Int64()
		return int(i)
	default:
		return 0
	}
}

func parseTimestamp(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
