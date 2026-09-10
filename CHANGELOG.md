# Changelog

All notable changes to vif are documented here. Format follows [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

## [0.2.0] - 2026-09-10

### Changed
- TUI 全量重写：从 Mole 双栏卡片样式换成 queen（你 `github.com/kesonglab/queen`）的 banner + 菜单 + taskRow 布局。色板换成 5 色（green/blue/red/yellow/gray），banner 用 `━` 分隔 + `👑` 标题，菜单用 `[1]` `[2]` 数字键 + `▶` 游标，section title 用 `── Title ──`，进度行定宽对齐 + sparkline。删了 Reely mascot 和双栏 Source/Output/Encode/System dashboard，改用线性 taskRow + 批量进度条。
- 删掉了 Mole 风格的双列 sparkline 和 MiniBar，queen 用单 spinner + 单进度条。

### Fixed
- `vif doctor` 找不到 rife-ncnn-vulkan —— GitHub release zip 解压到 `~/rife-ncnn-vulkan/rife-ncnn-vulkan-<date>-macos/`，原硬编码路径漏掉了。改成递归搜，匹配原 bash 脚本的 `find` 行为。

## [0.1.0] - 2026-09-10

### Added
- Initial release
- TUI mode (default) with 7 screens: Welcome, FilePicker, Multiplier, Encoder, Confirm, Processing, Summary
- CLI mode (`vif run ...`) with text progress
- Frame rate multipliers: x2 / x4 / x8
- VideoToolbox hardware decoding + HEVC/H.264 hardware encoding (default)
- Software encoding fallback: libx264 / libx265
- Quality presets: quality / balanced / speed
- Chunked processing for long videos (default 3000 frames per chunk)
- Watch mode (`--watch <dir>`)
- JSON output mode (`--json`) with NDJSON events
- Doctor command for environment diagnostics
- Self-update command (`vif update`)