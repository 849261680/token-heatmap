package gitoken

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gitoken/internal/exporter"
	"gitoken/internal/store"
)

type generateOptions struct {
	DBPath    string
	Days      int
	OutputDir string
	Now       time.Time
}

func runGenerate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing generate subcommand: use 'heatmap'")
	}
	switch args[0] {
	case "heatmap":
		return runGenerateHeatmap(args[1:])
	default:
		return fmt.Errorf("unknown generate subcommand %q", args[0])
	}
}

func runGenerateHeatmap(args []string) error {
	opts, err := parseGenerateOptions(args)
	if err != nil {
		return err
	}
	return generateArtifacts(opts)
}

func generateArtifacts(opts generateOptions) error {
	st, err := store.Open(opts.DBPath)
	if err != nil {
		return err
	}
	defer st.Close()

	since := opts.Now.In(time.Local).AddDate(0, 0, -(opts.Days - 1)).Format("2006-01-02")
	rows, err := st.DailyUsageSince(context.Background(), since)
	if err != nil {
		return err
	}

	summaries := exporter.BuildDailySummaries(rows, opts.Days, opts.Now)
	export := exporter.UsageExport{
		GeneratedAt: opts.Now.Format(time.RFC3339),
		Timezone:    time.Now().In(time.Local).Location().String(),
		Days:        opts.Days,
		Rows:        summaries,
	}

	jsonPath := filepath.Join(opts.OutputDir, "usage.json")
	svgPath := filepath.Join(opts.OutputDir, "heatmap.svg")
	if err := exporter.WriteUsageJSON(jsonPath, export); err != nil {
		return err
	}
	title := fmt.Sprintf("gitoken · %d-day Token Heatmap", opts.Days)
	if err := exporter.WriteHeatmapSVG(svgPath, summaries, title); err != nil {
		return err
	}

	fmt.Printf("generated: %s\n", jsonPath)
	fmt.Printf("generated: %s\n", svgPath)
	return nil
}

type syncOptions struct {
	Generate generateOptions
	RepoDir  string
	Remote   string
	Branch   string
}

func runSync(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing sync subcommand: use 'github'")
	}
	switch args[0] {
	case "github":
		return runSyncGitHub(args[1:])
	default:
		return fmt.Errorf("unknown sync subcommand %q", args[0])
	}
}

func runSyncGitHub(args []string) error {
	dbPathDefault, err := store.DefaultDBPath()
	if err != nil {
		return err
	}

	opts := syncOptions{
		Generate: generateOptions{
			DBPath:    dbPathDefault,
			Days:      180,
			OutputDir: "docs",
			Now:       time.Now(),
		},
		RepoDir: ".",
		Remote:  "origin",
	}

	fs := newFlagSet("sync github")
	repoDir := fs.String("repo-dir", opts.RepoDir, "git repository directory")
	dbPath := fs.String("db", opts.Generate.DBPath, "sqlite database path")
	days := fs.Int("days", opts.Generate.Days, "number of local days to export")
	outputDir := fs.String("output-dir", opts.Generate.OutputDir, "output directory relative to repo-dir")
	remote := fs.String("remote", opts.Remote, "git remote name")
	branch := fs.String("branch", "", "branch to push (defaults to current branch)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *days <= 0 {
		return fmt.Errorf("--days must be positive")
	}

	opts.RepoDir = *repoDir
	opts.Remote = *remote
	opts.Branch = *branch
	opts.Generate.DBPath = *dbPath
	opts.Generate.Days = *days
	opts.Generate.OutputDir = filepath.Join(opts.RepoDir, *outputDir)
	opts.Generate.Now = time.Now()

	return syncGitHub(opts)
}

func syncGitHub(opts syncOptions) error {
	if err := generateArtifacts(opts.Generate); err != nil {
		return err
	}

	branch := opts.Branch
	if branch == "" {
		current, err := gitOutput(opts.RepoDir, "branch", "--show-current")
		if err != nil {
			return err
		}
		branch = strings.TrimSpace(current)
		if branch == "" {
			return fmt.Errorf("could not determine current git branch")
		}
	}

	outputRel, err := filepath.Rel(opts.RepoDir, opts.Generate.OutputDir)
	if err != nil {
		return fmt.Errorf("resolve output dir relative path: %w", err)
	}
	if err := gitRun(opts.RepoDir, "add", filepath.Join(outputRel, "usage.json"), filepath.Join(outputRel, "heatmap.svg")); err != nil {
		return err
	}

	changed, err := gitHasStagedChanges(opts.RepoDir)
	if err != nil {
		return err
	}
	if !changed {
		fmt.Println("no export changes to sync")
		return nil
	}

	commitMessage := fmt.Sprintf("Update token usage for %s", opts.Generate.Now.In(time.Local).Format("2006-01-02"))
	if err := gitRun(opts.RepoDir, "commit", "-m", commitMessage); err != nil {
		return err
	}
	if err := gitRun(opts.RepoDir, "push", opts.Remote, branch); err != nil {
		return err
	}

	fmt.Printf("synced %s to %s/%s\n", outputRel, opts.Remote, branch)
	return nil
}

func gitRun(repoDir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func gitOutput(repoDir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func gitHasStagedChanges(repoDir string) (bool, error) {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = repoDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 1 {
			return true, nil
		}
	}
	return false, fmt.Errorf("git diff --cached --quiet: %w\n%s", err, strings.TrimSpace(stderr.String()))
}
