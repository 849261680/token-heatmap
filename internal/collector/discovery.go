package collector

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func userHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return home, nil
}

func discoverJSONLFiles(roots []string) ([]string, error) {
	var files []string
	seen := map[string]struct{}{}

	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}

		info, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat root %q: %w", root, err)
		}
		if !info.IsDir() {
			continue
		}

		err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if strings.EqualFold(filepath.Ext(path), ".jsonl") {
				if _, ok := seen[path]; !ok {
					seen[path] = struct{}{}
					files = append(files, path)
				}
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk root %q: %w", root, err)
		}
	}

	sort.Strings(files)
	return files, nil
}

func codexRoots() ([]string, error) {
	home, err := userHomeDir()
	if err != nil {
		return nil, err
	}

	base := strings.TrimSpace(os.Getenv("CODEX_HOME"))
	if base == "" {
		base = filepath.Join(home, ".codex")
	}
	return []string{
		filepath.Join(base, "sessions"),
		filepath.Join(base, "archived_sessions"),
	}, nil
}

func claudeRoots() ([]string, error) {
	home, err := userHomeDir()
	if err != nil {
		return nil, err
	}

	env := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR"))
	if env != "" {
		var roots []string
		for _, part := range strings.Split(env, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if filepath.Base(part) == "projects" {
				roots = append(roots, part)
			} else {
				roots = append(roots, filepath.Join(part, "projects"))
			}
		}
		return roots, nil
	}

	return []string{
		filepath.Join(home, ".config", "claude", "projects"),
		filepath.Join(home, ".claude", "projects"),
	}, nil
}

func opencodeDBPath() (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}

	if value := strings.TrimSpace(os.Getenv("OPENCODE_DATA_DIR")); value != "" {
		return filepath.Join(value, "opencode.db"), nil
	}
	if value := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); value != "" {
		return filepath.Join(value, "opencode", "opencode.db"), nil
	}
	return filepath.Join(home, ".local", "share", "opencode", "opencode.db"), nil
}
