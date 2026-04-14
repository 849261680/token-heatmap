package gitoken

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/849261680/token-heatmap/internal/store"
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
		Days:      365,
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

func parseSyncGitHubOptions(args []string) (syncOptions, error) {
	dbPathDefault, err := store.DefaultDBPath()
	if err != nil {
		return syncOptions{}, err
	}

	opts := syncOptions{
		Generate: generateOptions{
			DBPath:    dbPathDefault,
			Days:      365,
			OutputDir: "docs",
			Now:       time.Now(),
		},
		RepoDir:       ".",
		Remote:        "origin",
		ProfileRemote: "origin",
		ProfileAsset:  "heatmap.svg",
	}

	fs := newFlagSet("sync github")
	repoDir := fs.String("repo-dir", opts.RepoDir, "git repository directory")
	dbPath := fs.String("db", opts.Generate.DBPath, "sqlite database path")
	days := fs.Int("days", opts.Generate.Days, "number of local days to export")
	outputDir := fs.String("output-dir", opts.Generate.OutputDir, "output directory relative to repo-dir")
	remote := fs.String("remote", opts.Remote, "git remote name")
	branch := fs.String("branch", "", "branch to push (defaults to current branch)")
	profileRepoDir := fs.String("profile-repo-dir", "", "optional GitHub profile repository directory")
	profileRemote := fs.String("profile-remote", opts.ProfileRemote, "git remote for profile repo")
	profileBranch := fs.String("profile-branch", "", "branch to push for profile repo (defaults to current branch)")
	profileAsset := fs.String("profile-asset", opts.ProfileAsset, "heatmap asset path relative to profile repo")
	if err := fs.Parse(args); err != nil {
		return syncOptions{}, err
	}
	if *days <= 0 {
		return syncOptions{}, fmt.Errorf("--days must be positive")
	}

	opts.RepoDir = *repoDir
	opts.Remote = *remote
	opts.Branch = *branch
	opts.ProfileRepoDir = *profileRepoDir
	opts.ProfileRemote = *profileRemote
	opts.ProfileBranch = *profileBranch
	opts.ProfileAsset = *profileAsset
	opts.Generate.DBPath = *dbPath
	opts.Generate.Days = *days
	opts.Generate.OutputDir = filepath.Join(opts.RepoDir, *outputDir)
	opts.Generate.Now = time.Now()
	return opts, nil
}

func timeNow() time.Time {
	return time.Now()
}
