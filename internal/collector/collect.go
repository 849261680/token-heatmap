package collector

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/849261680/token-heatmap/internal/model"
	"github.com/849261680/token-heatmap/internal/store"
)

type CollectResult struct {
	Provider      model.Provider
	FilesScanned  int
	FilesSkipped  int
	EventsWritten int
}

type Scanner interface {
	Provider() model.Provider
	Collect(ctx context.Context, st *store.Store) (CollectResult, error)
}

func ScanAll(ctx context.Context, st *store.Store, providers []model.Provider) ([]CollectResult, error) {
	var scanners []Scanner
	for _, provider := range providers {
		switch provider {
		case model.ProviderCodex:
			scanners = append(scanners, &CodexScanner{})
		case model.ProviderClaude:
			scanners = append(scanners, &ClaudeScanner{})
		case model.ProviderOpenCode:
			scanners = append(scanners, &OpenCodeScanner{})
		default:
			return nil, fmt.Errorf("unsupported provider %q", provider)
		}
	}

	var results []CollectResult
	for _, scanner := range scanners {
		result, err := scanner.Collect(ctx, st)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func fileChanged(stored store.FileState, info os.FileInfo) bool {
	return stored.SizeBytes != info.Size() || stored.ModUnixMS != info.ModTime().UnixMilli()
}

func localDayKey(t time.Time) string {
	return t.In(time.Local).Format("2006-01-02")
}
