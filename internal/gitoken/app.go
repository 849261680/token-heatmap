package gitoken

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/849261680/token-heatmap/internal/collector"
	"github.com/849261680/token-heatmap/internal/model"
	"github.com/849261680/token-heatmap/internal/store"
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
	case "generate":
		return runGenerate(args[1:])
	case "run":
		return runRun(args[1:])
	case "sync":
		return runSync(args[1:])
	case "schedule":
		return runSchedule(args[1:])
	case "help", "-h", "--help":
		return usageError()
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usageText())
	}
}

func runCollect(args []string) error {
	opts, err := parseCollectOptions(args)
	if err != nil {
		return err
	}
	results, err := executeCollect(opts)
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

type collectOptions struct {
	DBPath    string
	Providers []model.Provider
}

func parseCollectOptions(args []string) (collectOptions, error) {
	dbPathDefault, err := store.DefaultDBPath()
	if err != nil {
		return collectOptions{}, err
	}

	opts := collectOptions{
		DBPath: dbPathDefault,
	}

	fs := newFlagSet("collect")
	dbPath := fs.String("db", opts.DBPath, "sqlite database path")
	providerArg := fs.String("provider", "all", "all|codex|claude|opencode")
	if err := fs.Parse(args); err != nil {
		return collectOptions{}, err
	}

	providers, err := parseProviders(*providerArg)
	if err != nil {
		return collectOptions{}, err
	}
	opts.DBPath = *dbPath
	opts.Providers = providers
	return opts, nil
}

func executeCollect(opts collectOptions) ([]collector.CollectResult, error) {
	st, err := store.Open(opts.DBPath)
	if err != nil {
		return nil, err
	}
	defer st.Close()

	return collector.ScanAll(context.Background(), st, opts.Providers)
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
	jsonOut := fs.Bool("json", false, "emit JSON")
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
	if *jsonOut {
		return printDailyRowsJSON(rows, 1)
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
	jsonOut := fs.Bool("json", false, "emit JSON")
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
	if *jsonOut {
		return printDailyRowsJSON(rows, *days)
	}
	printDailyRows(rows)
	return nil
}

func parseProviders(value string) ([]model.Provider, error) {
	switch value {
	case "all":
		return []model.Provider{model.ProviderCodex, model.ProviderClaude, model.ProviderOpenCode}, nil
	case "codex":
		return []model.Provider{model.ProviderCodex}, nil
	case "claude":
		return []model.Provider{model.ProviderClaude}, nil
	case "opencode":
		return []model.Provider{model.ProviderOpenCode}, nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", value)
	}
}

func usageError() error {
	return errors.New(usageText())
}

func usageText() string {
	return `tokenheat

Usage:
  tokenheat collect [--provider all|codex|claude|opencode] [--db PATH]
  tokenheat report today [--db PATH]
  tokenheat report daily [--days N] [--db PATH]
  tokenheat generate heatmap [--days N] [--output-dir DIR] [--db PATH]
  tokenheat run daily [--repo-dir DIR] [--profile-repo-dir DIR] [--days N] [--db PATH]
  tokenheat sync github [--days N] [--output-dir DIR] [--repo-dir DIR] [--db PATH]
  tokenheat sync github [--profile-repo-dir DIR]
  tokenheat schedule install [--time HH:MM] [--repo-dir DIR] [--profile-repo-dir DIR]
  tokenheat schedule status
  tokenheat schedule remove`
}
