# video-interpolate (vif)

用 RIFE 做视频补帧的命令行工具，把 30fps 视频补成 60/120/240fps。macOS 原生，解码编码都走 VideoToolbox，跑起来是个 Mole 风格的 TUI。

## Features

- x2 / x4 / x8 补帧倍率
- VideoToolbox 硬编（hevc / h264），不支持时自动回退软编
- Mole 风格 TUI：任务队列、进度条、实时帧率
- 跳过已存在的输出，断点续跑
- 多任务并行 + 长视频分块处理
- Watch 模式自动处理新文件
- JSON 输出（NDJSON 事件流）
- 自更新（`vif update`）

## Installation

### Homebrew (macOS / Linux)

```bash
brew install kesonglab/tap/vif
```

### Install Script (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/kesonglab/video-interpolate/main/install.sh | bash
```

装到 `/usr/local/bin`（可写时）或 `~/.local/bin`。可用环境变量覆盖：

```bash
VIF_INSTALL_DIR=/opt/bin curl -fsSL ... | bash   # 自定义目录
VIF_VERSION=v0.1.0 curl -fsSL ... | bash          # 指定版本
```

### From Source

```bash
go install github.com/kesonglab/video-interpolate/cmd/vif@latest
```

### Updating

```bash
vif update        # 更新到最新版
vif update --check  # 只检查不更新
```

## Requirements

- macOS 12+ (Apple Silicon recommended for VideoToolbox)
- Linux (x86_64 / arm64) — works without hardware acceleration
- ffmpeg + ffprobe (`brew install ffmpeg` or `apt install ffmpeg`)
- rife-ncnn-vulkan for actual interpolation (run `vif doctor` to verify; will prompt to install)

## Quick Start

TUI（默认）：

```bash
vif
```

选文件 → 选倍率 → 选编码器 → 确认，然后看实时仪表盘。

CLI：

```bash
vif run clip.mp4 -m 4
```

## Usage

### TUI (default)

```bash
vif          # launch the interactive TUI
```

`vif` bare on a terminal opens the TUI: pick files, choose a multiplier and
encoder, then confirm. Confirm starts the run and lands on a live dashboard:
two columns of cards (Source/Output on the left, RIFE/Encode/System on the
right) with progress bars, ETA, a frame-rate sparkline and system stats. The
batch ends on a Summary screen.

Processing keys: `c` cancel current job, `n` skip (TODO), `p` pause (TODO),
`m` toggle mascot, `q` quit (asks first). Summary keys: `enter` open the output
folder, `c` copy the output list, `r` restart, `q` quit.

```
Interpolate  ● Batch 2/5                    MacBook Pro · M5 · VideoToolbox h264
════════════════════════════════════════════════════════════════════════════════
◉ Source  ╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌   ▶ RIFE    ╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
File   clip1.mp4            Stage  generating frames
Res    1920 × 1080          Prog   ██████████░░░░░░   62.5%
FPS    24.00                ETA    00:01:24
Dur    00:02:15             Speed  8.4 fps
Frames 3240                 Trend  ▁▂▃▄▅▆▇█▁▁▃▅▆▂▁

▦ Output  ╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌   ◈ Encode  ╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
Mult   x4                   Stage  writing video
Target 96 fps               Prog   █████░░░░░░░░░░░   31.2%
Enc    VideoToolbox h264    ETA    00:02:10
Preset balanced             Out    142 MB · 18.2 Mb/s
Out    ~/interpolated/clip1_96fps.mp4
                            ⚙ System  ╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
                            CPU    ████████░░░░░░░░   42.0%
                            Mem    ████████████░░░░   58.0%
                            GPU    ▮▮▮▮▯  76%
════════════════════════════════════════════════════════════════════════════════
c cancel current  n skip  p pause  m toggle mascot  q quit
```

Summary:

```
Interpolate  Summary  ● done                MacBook Pro · M5 · VideoToolbox h264
════════════════════════════════════════════════════════════════════════════════
✓ Success        4
✗ Failed         1
⚠ Skipped        0
Total time     00:14:32
╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
Output files:
✓ ~/Videos/clip1_96fps.mp4  312.0 MB · 03:12
✗ ~/Videos/clip3_96fps.mp4  failed: encoder timeout
════════════════════════════════════════════════════════════════════════════════
enter open output folder  c copy list  r restart  q quit
```

### CLI

```bash
vif run file.mp4 --multiplier 2x
vif run -m 4 --encoder auto *.mp4    # 4x, encoder auto
vif run --help                       # all flags
vif doctor                           # check ffmpeg / RIFE environment
vif version
vif config edit
vif update --check                   # check for updates
```

### Watch Mode

```bash
vif run --watch ~/inbox
vif run --watch ~/inbox --json
```

监听目录，新视频落地后自动入队处理。Ctrl+C 退出。

### JSON Output

```bash
vif run --json files...
vif run --watch ~/inbox --json
```

输出机器可读 JSON 报告到 stdout。最终报告是完整 summary；运行中输出 NDJSON（每行一个事件）。

## Chunking

长视频默认按 3000 帧分片（约 100s @ 30fps）。每片独立 decode → RIFE → encode，
处理完立即释放临时帧目录，最后用 ffmpeg concat 拼回，音频单独提取一次后 mux 进
最终文件。分片失败或取消时临时目录自动清理，不会残留中间产物。

控制分片大小：

```bash
vif run movie.mp4 --chunk-size 1000    # 更小分片，更省磁盘，更慢
vif run movie.mp4 --no-chunk            # 禁用分片
```

每片峰值占用 = 一片输入帧 + 输出帧的磁盘空间。想压内存/磁盘就把 `--chunk-size`
调小；机器空闲、不在乎磁盘就 `--no-chunk` 一把梭。

## Screenshot

ASCII previews of the Processing dashboard and Summary are in the TUI section
above. A real terminal capture lands with the v1 release.

## Homebrew Formula

实际公式由 GoReleaser 在打 tag 时自动维护到 `kesonglab/homebrew-tap` 仓库。
仓库根目录 `homebrew-tap/Formula/vif.rb` 只是手动维护的初始版本 + 参考。

## License

MIT，见 [LICENSE](LICENSE)。