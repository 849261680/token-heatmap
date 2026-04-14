# Token Heatmap

Local CLI for collecting `Codex`, `Claude Code`, and `OpenCode` token usage from local data stores.

## Current scope

- `Codex`
  - Reads `~/.codex/sessions/**/*.jsonl`
  - Reads `~/.codex/archived_sessions/**/*.jsonl`
  - Supports `CODEX_HOME`
- `Claude Code`
  - Reads `~/.config/claude/projects/**/*.jsonl`
  - Reads `~/.claude/projects/**/*.jsonl`
  - Supports `CLAUDE_CONFIG_DIR`
- `OpenCode`
  - Reads `~/.local/share/opencode/opencode.db`
  - Supports `OPENCODE_DATA_DIR`

## Commands

```bash
./tokenheat collect
./tokenheat collect --provider codex
./tokenheat collect --provider opencode
./tokenheat report today
./tokenheat report today --json
./tokenheat report daily --days 30
./tokenheat generate heatmap
./tokenheat run daily --profile-repo-dir ../849261680
./tokenheat sync github
./tokenheat sync github --profile-repo-dir ../849261680
./tokenheat schedule install --profile-repo-dir ../849261680
./tokenheat schedule status
./tokenheat schedule remove
```

## Storage

SQLite database path:

```text
~/.tokenheat/tokenheat.db
```

## Notes

- The collector follows the same core approach as `CodexBar` for `Codex` and `Claude`:
  - Codex: parse `turn_context` + `event_msg/token_count`
  - Claude: parse `assistant` rows with `message.usage`
- `OpenCode` is collected from its local SQLite store:
  - `message.data.modelID`
  - `part.data.type == "step-finish"`
  - `part.data.tokens`
- Claude streaming chunks are deduped by `message.id + requestId`.
- Changed files are re-parsed and replace their prior rows in SQLite.
- Deleted log files are removed from the local ledger.

## Generated artifacts

- `./tokenheat generate heatmap` writes:
  - `docs/usage.json`
  - `docs/heatmap.svg`
- Default heatmap window is `365` days.
- `./tokenheat run daily` runs `collect` first, then regenerates and syncs GitHub artifacts.
- `./tokenheat sync github` regenerates those files, commits them, and pushes to the current Git remote.
- `./tokenheat sync github --profile-repo-dir ../849261680` also copies `docs/heatmap.svg` into the profile repo and pushes that update.

## Scheduling

- `./tokenheat schedule install --profile-repo-dir ../849261680` installs a macOS `launchd` job.
- Default run time is local `00:05` every day.
- The scheduled command is `tokenheat run daily`, so it performs `collect + sync`.
- Logs are written to `~/.tokenheat/logs/`.

## Menu Bar App

- Swift source lives in `apps/macos/TokenHeatMenu`.
- `./scripts/build-tokenheat-menu.sh` builds `dist/Token Heatmap.app`.
- Building the menu bar app currently requires full Xcode on macOS.
- The menu bar app shells out to the bundled `tokenheat` CLI.
- It shows today's token totals, supports `Sync Now`, and can install/remove daily sync.
