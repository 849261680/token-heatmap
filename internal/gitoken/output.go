package gitoken

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/849261680/token-heatmap/internal/model"
)

func printCollectResults(results []collectResultView) {
	for _, result := range results {
		fmt.Printf(
			"%s: scanned=%d skipped=%d events=%d\n",
			result.Provider,
			result.FilesScanned,
			result.FilesSkipped,
			result.EventsWritten,
		)
	}
}

func printDailyRows(rows []model.DailyUsageRow) {
	if len(rows) == 0 {
		fmt.Println("no usage rows found")
		return
	}

	sortDailyRows(rows)

	fmt.Printf("%-12s %-8s %12s %12s %12s %12s %12s\n", "DAY", "PROVIDER", "INPUT", "CACHE_RD", "CACHE_WR", "OUTPUT", "TOTAL")
	for _, row := range rows {
		fmt.Printf(
			"%-12s %-8s %12d %12d %12d %12d %12d\n",
			row.Day,
			strings.ToUpper(string(row.Provider)),
			row.InputTokens,
			row.CacheReadTokens,
			row.CacheWriteTokens,
			row.OutputTokens,
			row.TotalTokens,
		)
	}
}

func printDailyRowsJSON(rows []model.DailyUsageRow, days int) error {
	sortDailyRows(rows)
	payload := struct {
		GeneratedAt string                `json:"generated_at"`
		Days        int                   `json:"days"`
		Rows        []model.DailyUsageRow `json:"rows"`
	}{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Days:        days,
		Rows:        rows,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func sortDailyRows(rows []model.DailyUsageRow) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Day != rows[j].Day {
			return rows[i].Day < rows[j].Day
		}
		return rows[i].Provider < rows[j].Provider
	})
}
