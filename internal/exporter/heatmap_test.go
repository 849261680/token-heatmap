package exporter

import (
	"strings"
	"testing"
	"time"

	"github.com/849261680/token-heatmap/internal/model"
)

func TestBuildDailySummariesFillsMissingDays(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.Local)
	rows := []model.DailyUsageRow{
		{Day: "2026-04-14", Provider: model.ProviderCodex, TotalTokens: 10},
		{Day: "2026-04-15", Provider: model.ProviderClaude, TotalTokens: 20},
	}
	summaries := BuildDailySummaries(rows, 3, now)
	if len(summaries) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(summaries))
	}
	if summaries[0].Day != "2026-04-13" || summaries[0].TotalTokens != 0 {
		t.Fatalf("unexpected first summary: %+v", summaries[0])
	}
}

func TestBuildHeatmapSVGContainsRectangles(t *testing.T) {
	t.Parallel()

	svg := buildHeatmapSVG([]DailySummary{
		{Day: "2026-04-14", TotalTokens: 10, Providers: map[string]int{"codex": 10}},
		{Day: "2026-04-15", TotalTokens: 20, Providers: map[string]int{"claude": 20}},
	}, "Example")
	if !strings.Contains(svg, "<svg") || !strings.Contains(svg, "<rect") {
		t.Fatalf("expected svg rectangles, got %q", svg)
	}
}
