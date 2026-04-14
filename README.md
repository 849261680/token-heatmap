# gitoken

Local CLI for collecting `Codex` and `Claude Code` token usage from native JSONL logs.

## Current scope

- `Codex`
  - Reads `~/.codex/sessions/**/*.jsonl`
  - Reads `~/.codex/archived_sessions/**/*.jsonl`
  - Supports `CODEX_HOME`
- `Claude Code`
  - Reads `~/.config/claude/projects/**/*.jsonl`
  - Reads `~/.claude/projects/**/*.jsonl`
  - Supports `CLAUDE_CONFIG_DIR`

## Commands

```bash
./gitoken collect
./gitoken collect --provider codex
./gitoken report today
./gitoken report daily --days 30
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
- Claude streaming chunks are deduped by `message.id + requestId`.
- Changed files are re-parsed and replace their prior rows in SQLite.
- Deleted log files are removed from the local ledger.
