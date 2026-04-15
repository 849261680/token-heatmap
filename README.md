# Token Heatmap

把本地 AI 编程使用量转成 GitHub 风格的 Token 热力图。

## 数据源

- `Codex`：`~/.codex/sessions`、`~/.codex/archived_sessions`
- `Claude Code`：`~/.config/claude/projects`、`~/.claude/projects`
- `OpenCode`：`~/.local/share/opencode/opencode.db`

## 能力

- 扫描本地使用记录，写入 `~/.tokenheat/tokenheat.db`
- 生成最近 `365` 天热力图
- 导出：
  - `docs/usage.json`
  - `docs/heatmap.svg`
- 同步到项目仓库
- 可同步到 GitHub 个人主页仓库
- 支持 macOS `launchd` 每日自动执行
- 提供 macOS 菜单栏应用外壳

## 常用命令

```bash
./tokenheat collect
./tokenheat report today
./tokenheat report today --json
./tokenheat generate heatmap
./tokenheat sync github --profile-repo-dir ../849261680
./tokenheat run daily --profile-repo-dir ../849261680
./tokenheat schedule install --profile-repo-dir ../849261680
./tokenheat schedule status
./tokenheat schedule remove
```

## 菜单栏应用

源码：`apps/macos/TokenHeatMenu`

构建：

```bash
./scripts/build-tokenheat-menu.sh
```

要求：

- macOS
- 完整 Xcode

产物：

```bash
dist/Token Heatmap.app
```
