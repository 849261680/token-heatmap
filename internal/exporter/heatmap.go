package exporter

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gitoken/internal/model"
)

type DailySummary struct {
	Day         string         `json:"day"`
	TotalTokens int            `json:"total_tokens"`
	Providers   map[string]int `json:"providers"`
}

type UsageExport struct {
	GeneratedAt string         `json:"generated_at"`
	Timezone    string         `json:"timezone"`
	Days        int            `json:"days"`
	Rows        []DailySummary `json:"rows"`
}

func BuildDailySummaries(rows []model.DailyUsageRow, days int, now time.Time) []DailySummary {
	byDay := map[string]*DailySummary{}
	for _, row := range rows {
		summary := byDay[row.Day]
		if summary == nil {
			summary = &DailySummary{
				Day:       row.Day,
				Providers: map[string]int{},
			}
			byDay[row.Day] = summary
		}
		summary.TotalTokens += row.TotalTokens
		summary.Providers[string(row.Provider)] += row.TotalTokens
	}

	start := now.In(time.Local).AddDate(0, 0, -(days - 1))
	var out []DailySummary
	for i := 0; i < days; i++ {
		day := start.AddDate(0, 0, i).Format("2006-01-02")
		if existing, ok := byDay[day]; ok {
			out = append(out, *existing)
			continue
		}
		out = append(out, DailySummary{
			Day:       day,
			Providers: map[string]int{},
		})
	}
	return out
}

func WriteUsageJSON(path string, export UsageExport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create export directory: %w", err)
	}
	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal usage json: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write usage json: %w", err)
	}
	return nil
}

func WriteHeatmapSVG(path string, summaries []DailySummary, title string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create export directory: %w", err)
	}
	svg := buildHeatmapSVG(summaries, title)
	if err := os.WriteFile(path, []byte(svg), 0o644); err != nil {
		return fmt.Errorf("write heatmap svg: %w", err)
	}
	return nil
}

func buildHeatmapSVG(summaries []DailySummary, title string) string {
	const (
		cell   = 12
		gap    = 3
		left   = 46
		top    = 38
		footer = 42
	)

	if len(summaries) == 0 {
		return `<svg xmlns="http://www.w3.org/2000/svg" width="640" height="120"></svg>`
	}

	maxTokens := 0
	dayMap := make(map[string]DailySummary, len(summaries))
	for _, summary := range summaries {
		dayMap[summary.Day] = summary
		if summary.TotalTokens > maxTokens {
			maxTokens = summary.TotalTokens
		}
	}

	start, _ := time.ParseInLocation("2006-01-02", summaries[0].Day, time.Local)
	end, _ := time.ParseInLocation("2006-01-02", summaries[len(summaries)-1].Day, time.Local)
	start = start.AddDate(0, 0, -int(start.Weekday()))
	end = end.AddDate(0, 0, 6-int(end.Weekday()))
	weeks := int(end.Sub(start).Hours()/24)/7 + 1

	width := left + weeks*(cell+gap) + 24
	height := top + 7*(cell+gap) + footer

	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" fill="none">`, width, height, width, height))
	b.WriteString(`<style>
.label{font:12px ui-sans-serif,system-ui,-apple-system,sans-serif;fill:#57606a}
.title{font:600 15px ui-sans-serif,system-ui,-apple-system,sans-serif;fill:#24292f}
.sub{font:11px ui-sans-serif,system-ui,-apple-system,sans-serif;fill:#57606a}
</style>`)
	b.WriteString(`<rect width="100%" height="100%" rx="14" fill="#ffffff"/>`)
	b.WriteString(fmt.Sprintf(`<text class="title" x="%d" y="22">%s</text>`, left, escapeXML(title)))
	b.WriteString(fmt.Sprintf(`<text class="sub" x="%d" y="%d">Daily token usage</text>`, left, top-12))

	weekdayLabels := []struct {
		Label string
		Day   time.Weekday
	}{
		{"Mon", time.Monday},
		{"Wed", time.Wednesday},
		{"Fri", time.Friday},
	}
	for _, label := range weekdayLabels {
		y := top + int(label.Day)*(cell+gap) + cell - 2
		b.WriteString(fmt.Sprintf(`<text class="label" x="8" y="%d">%s</text>`, y, label.Label))
	}

	monthSeen := map[string]bool{}
	for week := 0; week < weeks; week++ {
		weekStart := start.AddDate(0, 0, week*7)
		monthKey := weekStart.Format("2006-01")
		if !monthSeen[monthKey] && weekStart.Day() <= 7 {
			monthSeen[monthKey] = true
			x := left + week*(cell+gap)
			b.WriteString(fmt.Sprintf(`<text class="label" x="%d" y="%d">%s</text>`, x, top-20, weekStart.Format("Jan")))
		}
		for weekday := 0; weekday < 7; weekday++ {
			current := weekStart.AddDate(0, 0, weekday)
			dayKey := current.Format("2006-01-02")
			summary, ok := dayMap[dayKey]
			tokens := 0
			if ok {
				tokens = summary.TotalTokens
			}
			x := left + week*(cell+gap)
			y := top + weekday*(cell+gap)
			fill := heatColor(tokens, maxTokens)
			b.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" rx="2" fill="%s">`, x, y, cell, cell, fill))
			tooltip := fmt.Sprintf("%s: %d tokens", dayKey, tokens)
			b.WriteString(fmt.Sprintf(`<title>%s</title></rect>`, escapeXML(tooltip)))
		}
	}

	legendX := width - 150
	legendY := height - 20
	b.WriteString(fmt.Sprintf(`<text class="sub" x="%d" y="%d">Less</text>`, legendX-28, legendY+10))
	legendLevels := []string{"#ebedf0", "#9be9a8", "#40c463", "#30a14e", "#216e39"}
	for i, fill := range legendLevels {
		x := legendX + i*(cell+4)
		b.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" rx="2" fill="%s"/>`, x, legendY, cell, cell, fill))
	}
	b.WriteString(fmt.Sprintf(`<text class="sub" x="%d" y="%d">More</text>`, legendX+len(legendLevels)*(cell+4)+4, legendY+10))

	total := 0
	for _, summary := range summaries {
		total += summary.TotalTokens
	}
	b.WriteString(fmt.Sprintf(`<text class="sub" x="%d" y="%d">%s</text>`, left, height-14, escapeXML(fmt.Sprintf("%d days · %s total tokens", len(summaries), formatCompact(total)))))
	b.WriteString(`</svg>`)
	return b.String()
}

func heatColor(value int, max int) string {
	if value <= 0 || max <= 0 {
		return "#ebedf0"
	}
	ratio := float64(value) / float64(max)
	switch {
	case ratio < 0.25:
		return "#9be9a8"
	case ratio < 0.5:
		return "#40c463"
	case ratio < 0.75:
		return "#30a14e"
	default:
		return "#216e39"
	}
}

func formatCompact(value int) string {
	abs := math.Abs(float64(value))
	switch {
	case abs >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(value)/1_000_000_000)
	case abs >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(value)/1_000_000)
	case abs >= 1_000:
		return fmt.Sprintf("%.1fK", float64(value)/1_000)
	default:
		return fmt.Sprintf("%d", value)
	}
}

func escapeXML(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

func ProviderTotals(summary DailySummary) []string {
	var keys []string
	for key := range summary.Providers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var out []string
	for _, key := range keys {
		out = append(out, fmt.Sprintf("%s=%d", key, summary.Providers[key]))
	}
	return out
}
