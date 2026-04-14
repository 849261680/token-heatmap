package gitoken

import (
	"fmt"
	"sort"
	"strings"

	"gitoken/internal/model"
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

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Day != rows[j].Day {
			return rows[i].Day < rows[j].Day
		}
		return rows[i].Provider < rows[j].Provider
	})

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
