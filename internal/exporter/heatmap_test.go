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

func TestHeatColorUsesUsageBucketsNotSingleMax(t *testing.T) {
	t.Parallel()

	thresholds := heatThresholds([]DailySummary{
		{Day: "2026-04-01", TotalTokens: 100},
		{Day: "2026-04-02", TotalTokens: 200},
		{Day: "2026-04-03", TotalTokens: 300},
		{Day: "2026-04-04", TotalTokens: 400},
		{Day: "2026-04-05", TotalTokens: 100000},
	})
	if got := heatColor(400, thresholds); got != "#3182bd" {
		t.Fatalf("expected high non-outlier usage to use upper bucket, got %s", got)
	}
	if got := heatColor(100000, thresholds); got != "#08519c" {
		t.Fatalf("expected outlier usage to use darkest bucket, got %s", got)
	}
}
