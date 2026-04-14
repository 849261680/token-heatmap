# gitoken

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
./gitoken collect
./gitoken collect --provider codex
./gitoken collect --provider opencode
./gitoken report today
./gitoken report daily --days 30
./gitoken generate heatmap
./gitoken sync github
./gitoken sync github --profile-repo-dir ../849261680
```

## Storage

SQLite database path:

```text
~/.gitoken/gitoken.db
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

- `./gitoken generate heatmap` writes:
  - `docs/usage.json`
  - `docs/heatmap.svg`
- Default heatmap window is `365` days.
- `./gitoken sync github` regenerates those files, commits them, and pushes to the current Git remote.
- `./gitoken sync github --profile-repo-dir ../849261680` also copies `docs/heatmap.svg` into the profile repo and pushes that update.
