package gitoken

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"gitoken/internal/collector"
	"gitoken/internal/model"
	"gitoken/internal/store"
)

type collectResultView struct {
	Provider      string
	FilesScanned  int
	FilesSkipped  int
	EventsWritten int
}

func Run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}

	switch args[0] {
	case "collect":
		return runCollect(args[1:])
	case "report":
		return runReport(args[1:])
	case "help", "-h", "--help":
		return usageError()
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usageText())
	}
}

func runCollect(args []string) error {
	fs := flag.NewFlagSet("collect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	dbPathDefault, err := store.DefaultDBPath()
	if err != nil {
		return err
	}
	dbPath := fs.String("db", dbPathDefault, "sqlite database path")
	providerArg := fs.String("provider", "all", "codex|claude|all")

	if err := fs.Parse(args); err != nil {
		return err
	}

	providers, err := parseProviders(*providerArg)
	if err != nil {
		return err
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	results, err := collector.ScanAll(context.Background(), st, providers)
	if err != nil {
		return err
	}

	var views []collectResultView
	for _, result := range results {
		views = append(views, collectResultView{
			Provider:      string(result.Provider),
			FilesScanned:  result.FilesScanned,
			FilesSkipped:  result.FilesSkipped,
			EventsWritten: result.EventsWritten,
		})
	}
	printCollectResults(views)
	return nil
}

func runReport(args []string) error {
	if len(args) == 0 {
		return errors.New("missing report subcommand: use 'today' or 'daily'")
	}

	switch args[0] {
	case "today":
		return runReportToday(args[1:])
	case "daily":
		return runReportDaily(args[1:])
	default:
		return fmt.Errorf("unknown report subcommand %q", args[0])
	}
}

func runReportToday(args []string) error {
	fs := flag.NewFlagSet("report today", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	dbPathDefault, err := store.DefaultDBPath()
	if err != nil {
		return err
	}
	dbPath := fs.String("db", dbPathDefault, "sqlite database path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	day := time.Now().In(time.Local).Format("2006-01-02")
	rows, err := st.DailyUsageForDay(context.Background(), day)
	if err != nil {
		return err
	}
	printDailyRows(rows)
	return nil
}

func runReportDaily(args []string) error {
	fs := flag.NewFlagSet("report daily", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	dbPathDefault, err := store.DefaultDBPath()
	if err != nil {
		return err
	}
	dbPath := fs.String("db", dbPathDefault, "sqlite database path")
	days := fs.Int("days", 30, "number of local days to show")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *days <= 0 {
		return fmt.Errorf("--days must be positive")
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	since := time.Now().In(time.Local).AddDate(0, 0, -(*days - 1)).Format("2006-01-02")
	rows, err := st.DailyUsageSince(context.Background(), since)
	if err != nil {
		return err
	}
	printDailyRows(rows)
	return nil
}

func parseProviders(value string) ([]model.Provider, error) {
	switch value {
	case "all":
		return []model.Provider{model.ProviderCodex, model.ProviderClaude}, nil
	case "codex":
		return []model.Provider{model.ProviderCodex}, nil
	case "claude":
		return []model.Provider{model.ProviderClaude}, nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", value)
	}
}

func usageError() error {
	return errors.New(usageText())
}

func usageText() string {
	return `gitoken

Usage:
  gitoken collect [--provider all|codex|claude] [--db PATH]
  gitoken report today [--db PATH]
  gitoken report daily [--days N] [--db PATH]`
}
