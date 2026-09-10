# Changelog

All notable changes to vif are documented here. Format follows [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

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