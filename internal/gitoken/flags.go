package gitoken

import (
	"flag"
	"fmt"
	"os"
	"time"

	"gitoken/internal/store"
)

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func parseGenerateOptions(args []string) (generateOptions, error) {
	dbPathDefault, err := store.DefaultDBPath()
	if err != nil {
		return generateOptions{}, err
	}

	opts := generateOptions{
		DBPath:    dbPathDefault,
		Days:      180,
		OutputDir: "docs",
		Now:       timeNow(),
	}

	fs := newFlagSet("generate heatmap")
	dbPath := fs.String("db", opts.DBPath, "sqlite database path")
	days := fs.Int("days", opts.Days, "number of local days to export")
	outputDir := fs.String("output-dir", opts.OutputDir, "directory to write usage artifacts")
	if err := fs.Parse(args); err != nil {
		return generateOptions{}, err
	}
	if *days <= 0 {
		return generateOptions{}, fmt.Errorf("--days must be positive")
	}
	opts.DBPath = *dbPath
	opts.Days = *days
	opts.OutputDir = *outputDir
	return opts, nil
}

func timeNow() time.Time {
	return time.Now()
}
