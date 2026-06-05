package tokenheat

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/849261680/token-heatmap/internal/model"
	"github.com/849261680/token-heatmap/internal/store"
)

const launchAgentLabel = "com.tokenheat.daily-sync"

func runRun(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing run subcommand: use 'daily'")
	}
	switch args[0] {
	case "daily":
		return runDaily(args[1:])
	default:
		return fmt.Errorf("unknown run subcommand %q", args[0])
	}
}

func runDaily(args []string) error {
	opts, providers, err := parseDailyOptions(args)
	if err != nil {
		return err
	}

	results, err := executeCollect(collectOptions{
		DBPath:    opts.Generate.DBPath,
		Providers: providers,
	})
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
	return syncGitHub(opts)
}

func parseDailyOptions(args []string) (syncOptions, []model.Provider, error) {
	opts, err := parseSyncGitHubOptions(nil)
	if err != nil {
		return syncOptions{}, nil, err
	}

	fs := newFlagSet("run daily")
	repoDir := fs.String("repo-dir", opts.RepoDir, "git repository directory")
	dbPath := fs.String("db", opts.Generate.DBPath, "sqlite database path")
	days := fs.Int("days", opts.Generate.Days, "number of local days to export")
	outputDir := fs.String("output-dir", "docs", "output directory relative to repo-dir")
	remote := fs.String("remote", opts.Remote, "git remote name")
	branch := fs.String("branch", "", "branch to push (defaults to current branch)")
	profileRepoDir := fs.String("profile-repo-dir", "", "optional GitHub profile repository directory")
	profileRemote := fs.String("profile-remote", opts.ProfileRemote, "git remote for profile repo")
	profileBranch := fs.String("profile-branch", "", "branch to push for profile repo (defaults to current branch)")
	profileAsset := fs.String("profile-asset", opts.ProfileAsset, "heatmap asset path relative to profile repo")
	providerArg := fs.String("provider", "all", "all|codex|claude|opencode")
	if err := fs.Parse(args); err != nil {
		return syncOptions{}, nil, err
	}
	if *days <= 0 {
		return syncOptions{}, nil, fmt.Errorf("--days must be positive")
	}

	providers, err := parseProviders(*providerArg)
	if err != nil {
		return syncOptions{}, nil, err
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
	opts.Generate.OutputDir = resolveOutputDir(opts.RepoDir, *outputDir)
	opts.Generate.Now = time.Now()

	if opts.ProfileRepoDir == "" {
		if cfg, err := LoadConfig(); err == nil && cfg.ProfileRepoDir != "" {
			opts.ProfileRepoDir = cfg.ProfileRepoDir
		}
	}
	return opts, providers, nil
}

func runSchedule(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing schedule subcommand: use 'install', 'status', or 'remove'")
	}
	switch args[0] {
	case "install":
		return runScheduleInstall(args[1:])
	case "status":
		return runScheduleStatus(args[1:])
	case "remove":
		return runScheduleRemove(args[1:])
	default:
		return fmt.Errorf("unknown schedule subcommand %q", args[0])
	}
}

type scheduleInstallOptions struct {
	Time           string
	Interval       int
	BinaryPath     string
	RepoDir        string
	ProfileRepoDir string
	DBPath         string
	Days           int
	OutputDir      string
	Remote         string
	Branch         string
	ProfileRemote  string
	ProfileBranch  string
	ProfileAsset   string
	Provider       string
}

func runScheduleInstall(args []string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("schedule install currently supports macOS only")
	}

	opts, err := parseScheduleInstallOptions(args)
	if err != nil {
		return err
	}

	hour, minute := 0, 0
	if opts.Interval == 0 {
		var err error
		hour, minute, err = parseScheduleTime(opts.Time)
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(launchAgentPath()), 0o755); err != nil {
		return fmt.Errorf("create launch agents directory: %w", err)
	}
	if err := os.MkdirAll(scheduleLogDir(), 0o755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}

	plist := buildLaunchAgentPlist(scheduleTrigger{Hour: hour, Minute: minute, Interval: opts.Interval}, launchAgentProgramArgs(opts), opts.RepoDir)
	if err := os.WriteFile(launchAgentPath(), []byte(plist), 0o644); err != nil {
		return fmt.Errorf("write launch agent plist: %w", err)
	}

	_ = exec.Command("launchctl", "bootout", launchAgentDomain(), launchAgentPath()).Run()
	if err := exec.Command("launchctl", "enable", launchAgentService()).Run(); err != nil {
		return fmt.Errorf("enable launch agent: %w", err)
	}
	if err := exec.Command("launchctl", "bootstrap", launchAgentDomain(), launchAgentPath()).Run(); err != nil {
		return fmt.Errorf("load launch agent: %w", err)
	}

	if opts.Interval > 0 {
		fmt.Printf("installed sync schedule: %s every %d seconds\n", launchAgentPath(), opts.Interval)
	} else {
		fmt.Printf("installed daily schedule: %s at %02d:%02d\n", launchAgentPath(), hour, minute)
	}
	fmt.Printf("launch label: %s\n", launchAgentLabel)
	return nil
}

func runScheduleStatus(args []string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("schedule status currently supports macOS only")
	}
	if len(args) > 0 {
		return fmt.Errorf("schedule status does not accept arguments")
	}

	plistPath := launchAgentPath()
	if _, err := os.Stat(plistPath); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("schedule not installed")
			return nil
		}
		return err
	}

	cmd := exec.Command("launchctl", "print", launchAgentService())
	output, err := cmd.CombinedOutput()
	loaded := err == nil
	fmt.Printf("plist: %s\n", plistPath)
	fmt.Printf("loaded: %t\n", loaded)
	if len(output) > 0 {
		fmt.Print(string(output))
	}
	return nil
}

func runScheduleRemove(args []string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("schedule remove currently supports macOS only")
	}
	if len(args) > 0 {
		return fmt.Errorf("schedule remove does not accept arguments")
	}

	plistPath := launchAgentPath()
	if _, err := os.Stat(plistPath); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("schedule not installed")
			return nil
		}
		return err
	}

	_ = exec.Command("launchctl", "bootout", launchAgentDomain(), plistPath).Run()
	if err := os.Remove(plistPath); err != nil {
		return fmt.Errorf("remove launch agent plist: %w", err)
	}

	fmt.Printf("removed daily schedule: %s\n", plistPath)
	return nil
}

func parseScheduleInstallOptions(args []string) (scheduleInstallOptions, error) {
	dbPathDefault, err := store.DefaultDBPath()
	if err != nil {
		return scheduleInstallOptions{}, err
	}

	binaryPath, err := os.Executable()
	if err != nil {
		return scheduleInstallOptions{}, fmt.Errorf("resolve current binary: %w", err)
	}
	binaryPath, err = filepath.Abs(binaryPath)
	if err != nil {
		return scheduleInstallOptions{}, err
	}

	repoDir, err := filepath.Abs(".")
	if err != nil {
		return scheduleInstallOptions{}, err
	}

	opts := scheduleInstallOptions{
		Time:          "00:05",
		BinaryPath:    binaryPath,
		RepoDir:       repoDir,
		DBPath:        dbPathDefault,
		Days:          365,
		OutputDir:     "docs",
		Remote:        "origin",
		ProfileRemote: "origin",
		ProfileAsset:  "heatmap.svg",
		Provider:      "all",
	}

	fs := newFlagSet("schedule install")
	timeArg := fs.String("time", opts.Time, "daily local time in HH:MM")
	intervalArg := fs.Int("interval", 0, "sync interval in seconds; overrides --time when set")
	binaryPathArg := fs.String("binary", opts.BinaryPath, "tokenheat binary path")
	repoDirArg := fs.String("repo-dir", opts.RepoDir, "git repository directory")
	profileRepoDirArg := fs.String("profile-repo-dir", "", "optional GitHub profile repository directory")
	dbPathArg := fs.String("db", opts.DBPath, "sqlite database path")
	daysArg := fs.Int("days", opts.Days, "number of local days to export")
	outputDirArg := fs.String("output-dir", opts.OutputDir, "output directory relative to repo-dir")
	remoteArg := fs.String("remote", opts.Remote, "git remote name")
	branchArg := fs.String("branch", "", "branch to push (defaults to current branch)")
	profileRemoteArg := fs.String("profile-remote", opts.ProfileRemote, "git remote for profile repo")
	profileBranchArg := fs.String("profile-branch", "", "branch to push for profile repo (defaults to current branch)")
	profileAssetArg := fs.String("profile-asset", opts.ProfileAsset, "heatmap asset path relative to profile repo")
	providerArg := fs.String("provider", opts.Provider, "all|codex|claude|opencode")
	if err := fs.Parse(args); err != nil {
		return scheduleInstallOptions{}, err
	}
	if *daysArg <= 0 {
		return scheduleInstallOptions{}, fmt.Errorf("--days must be positive")
	}
	if *intervalArg < 0 {
		return scheduleInstallOptions{}, fmt.Errorf("--interval must be non-negative")
	}
	if _, err := parseProviders(*providerArg); err != nil {
		return scheduleInstallOptions{}, err
	}

	opts.Time = *timeArg
	opts.Interval = *intervalArg
	opts.BinaryPath, err = filepath.Abs(*binaryPathArg)
	if err != nil {
		return scheduleInstallOptions{}, err
	}
	opts.RepoDir, err = filepath.Abs(*repoDirArg)
	if err != nil {
		return scheduleInstallOptions{}, err
	}
	opts.ProfileRepoDir = *profileRepoDirArg
	if opts.ProfileRepoDir != "" {
		opts.ProfileRepoDir, err = filepath.Abs(opts.ProfileRepoDir)
		if err != nil {
			return scheduleInstallOptions{}, err
		}
	}
	opts.DBPath = *dbPathArg
	opts.Days = *daysArg
	opts.OutputDir = *outputDirArg
	opts.Remote = *remoteArg
	opts.Branch = *branchArg
	opts.ProfileRemote = *profileRemoteArg
	opts.ProfileBranch = *profileBranchArg
	opts.ProfileAsset = *profileAssetArg
	opts.Provider = *providerArg

	if opts.ProfileRepoDir == "" {
		if cfg, err := LoadConfig(); err == nil && cfg.ProfileRepoDir != "" {
			opts.ProfileRepoDir = cfg.ProfileRepoDir
		}
	}
	return opts, nil
}

func parseScheduleTime(value string) (int, int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid --time %q: expected HH:MM", value)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("invalid --time %q: hour must be 00-23", value)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid --time %q: minute must be 00-59", value)
	}
	return hour, minute, nil
}

func launchAgentProgramArgs(opts scheduleInstallOptions) []string {
	args := []string{
		opts.BinaryPath,
		"run",
		"daily",
		"--repo-dir", opts.RepoDir,
		"--db", opts.DBPath,
		"--days", strconv.Itoa(opts.Days),
		"--output-dir", opts.OutputDir,
		"--remote", opts.Remote,
		"--provider", opts.Provider,
	}
	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}
	if opts.ProfileRepoDir != "" {
		args = append(args,
			"--profile-repo-dir", opts.ProfileRepoDir,
			"--profile-remote", opts.ProfileRemote,
			"--profile-asset", opts.ProfileAsset,
		)
		if opts.ProfileBranch != "" {
			args = append(args, "--profile-branch", opts.ProfileBranch)
		}
	}
	return args
}

type scheduleTrigger struct {
	Hour     int
	Minute   int
	Interval int
}

func buildLaunchAgentPlist(trigger scheduleTrigger, programArgs []string, workingDir string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString(`<plist version="1.0">` + "\n")
	b.WriteString(`<dict>` + "\n")
	b.WriteString("  <key>Label</key>\n")
	b.WriteString("  <string>" + xmlEscape(launchAgentLabel) + "</string>\n")
	b.WriteString("  <key>ProgramArguments</key>\n")
	b.WriteString("  <array>\n")
	for _, arg := range programArgs {
		b.WriteString("    <string>" + xmlEscape(arg) + "</string>\n")
	}
	b.WriteString("  </array>\n")
	b.WriteString("  <key>WorkingDirectory</key>\n")
	b.WriteString("  <string>" + xmlEscape(workingDir) + "</string>\n")
	if trigger.Interval > 0 {
		b.WriteString("  <key>StartInterval</key>\n")
		b.WriteString("  <integer>" + strconv.Itoa(trigger.Interval) + "</integer>\n")
	} else {
		b.WriteString("  <key>StartCalendarInterval</key>\n")
		b.WriteString("  <dict>\n")
		b.WriteString("    <key>Hour</key>\n")
		b.WriteString("    <integer>" + strconv.Itoa(trigger.Hour) + "</integer>\n")
		b.WriteString("    <key>Minute</key>\n")
		b.WriteString("    <integer>" + strconv.Itoa(trigger.Minute) + "</integer>\n")
		b.WriteString("  </dict>\n")
	}
	b.WriteString("  <key>StandardOutPath</key>\n")
	b.WriteString("  <string>" + xmlEscape(filepath.Join(scheduleLogDir(), "scheduler.out.log")) + "</string>\n")
	b.WriteString("  <key>StandardErrorPath</key>\n")
	b.WriteString("  <string>" + xmlEscape(filepath.Join(scheduleLogDir(), "scheduler.err.log")) + "</string>\n")
	b.WriteString("</dict>\n")
	b.WriteString("</plist>\n")
	return b.String()
}

func launchAgentPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
}

// launchAgentDomain returns the current user's GUI launchd domain.
func launchAgentDomain() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
}

// launchAgentService returns the current user's launchd service identifier.
func launchAgentService() string {
	return launchAgentDomain() + "/" + launchAgentLabel
}

func scheduleLogDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".tokenheat", "logs")
}

func xmlEscape(value string) string {
	var buf strings.Builder
	if err := xml.EscapeText(&buf, []byte(value)); err != nil {
		return value
	}
	return buf.String()
}
