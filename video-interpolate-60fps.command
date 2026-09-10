#!/bin/bash
# 强制 UTF-8 locale，避免 Ghostty 等终端双击 .command 时中文乱码
export LANG="${LANG:-en_US.UTF-8}"
export LC_ALL="${LC_ALL:-en_US.UTF-8}"

# ==============================================
#  视频补帧到 60FPS 工具 (优化版)
#  引擎：rife-ncnn-vulkan (AI 插帧) + ffmpeg
#  用法：双击运行，把视频拖进窗口或粘贴路径
# ==============================================
#  v2 优化项：
#    - macOS VideoToolbox 硬件解码（自动检测）
#    - 实时进度、帧率、ETA 剩余时间
#    - 实时 CPU / 内存占用监控
#    - 批量任务总 ETA
#    - 单视频用时统计

# ---------- 配置 ----------
OUTPUT_DIR="$HOME/Downloads/补帧60fps"          # 输出目录
INSTALL_DIR="$HOME/rife-ncnn-vulkan"            # rife 安装目录
RIFE_URL="https://github.com/nihui/rife-ncnn-vulkan/releases/download/20221029/rife-ncnn-vulkan-20221029-macos.zip"
TARGET_FPS=60                                   # 目标帧率
GPU_INDEX=0                                     # 0=GPU, -1=CPU
THREADS="4:4:4"                                 # 线程数（load:proc:save）
FRAME_FORMAT="jpg"                              # 中间帧格式 jpg/png
JPG_QUALITY=2                                   # jpg 质量 1-31（越小越好）
ENCODER="libx264"                               # 视频编码器
CRF=20                                          # 画质（越小越好）
USE_HW_DECODER=auto                             # auto=自动用 VideoToolbox，off=纯软件
PROGRESS_INTERVAL=2                             # 进度刷新间隔（秒）

# ---------- 工具函数 ----------
format_time() {
    local s=${1:-0}
    printf "%02d:%02d:%02d" $((s/3600)) $((s%3600/60)) $((s%60))
}

# 取 PID 的 CPU% 和 RSS(MB)，输出 "12.3 1024"
get_process_stats() {
    local pid=$1
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
        ps -p "$pid" -o %cpu= -o rss= 2>/dev/null | awk 'NR==1{printf "%.1f %.0f", $1, $2/1024}'
    else
        echo "0.0 0"
    fi
}

# 监控 RIFE 输出目录帧数 + CPU/内存
monitor_rife_progress() {
    local outdir="$1"
    local total="$2"
    local start=$(date +%s)

    while true; do
        local processed=$(ls "$outdir" 2>/dev/null | wc -l | tr -d ' ')
        local now=$(date +%s)
        local elapsed=$((now - start))
        [[ $elapsed -lt 1 ]] && elapsed=1
        local pct=0
        [[ $total -gt 0 ]] && pct=$((processed * 100 / total))
        [[ $pct -gt 100 ]] && pct=100
        local rate=$((processed / elapsed))
        local eta_str="--:--"
        if [[ $rate -gt 0 && $processed -lt $total ]]; then
            local remain=$(( (total - processed) / rate ))
            eta_str=$(format_time "$remain")
        elif [[ $processed -ge $total ]]; then
            eta_str="即将完成"
        fi

        local stats="0.0 0"
        local rife_pid=$(pgrep -f "rife-ncnn-vulkan" 2>/dev/null | head -1)
        if [[ -n "$rife_pid" ]]; then
            stats=$(get_process_stats "$rife_pid")
        fi

        printf "\r  [RIFE ] %3d%%  %d/%d帧  %2d帧/秒  剩余 %s  CPU %s%%  MEM %sMB   " \
            "$pct" "$processed" "$total" "$rate" "$eta_str" \
            "${stats% *}" "${stats#* }"

        sleep "$PROGRESS_INTERVAL"
    done
}

# 监控 ffmpeg 编码进度 + CPU/内存
monitor_ffmpeg_progress() {
    local progress_file="$1"
    local ffmpeg_pid="$2"
    local total="$3"
    local start=$(date +%s)

    while kill -0 "$ffmpeg_pid" 2>/dev/null; do
        local frame
        frame=$(grep "^frame=" "$progress_file" 2>/dev/null | tail -1 | cut -d= -f2 | tr -d ' ')
        [[ -z "$frame" || ! "$frame" =~ ^[0-9]+$ ]] && frame=0
        local now=$(date +%s)
        local elapsed=$((now - start))
        [[ $elapsed -lt 1 ]] && elapsed=1
        local pct=0
        [[ $total -gt 0 ]] && pct=$((frame * 100 / total))
        [[ $pct -gt 100 ]] && pct=100
        local rate=$((frame / elapsed))
        local eta_str="--:--"
        if [[ $rate -gt 0 && $frame -lt $total ]]; then
            local remain=$(( (total - frame) / rate ))
            eta_str=$(format_time "$remain")
        fi

        local stats=$(get_process_stats "$ffmpeg_pid")
        printf "\r  [编码 ] %3d%%  %d/%d帧  %2d帧/秒  剩余 %s  CPU %s%%  MEM %sMB   " \
            "$pct" "$frame" "$total" "$rate" "$eta_str" \
            "${stats% *}" "${stats#* }"

        sleep "$PROGRESS_INTERVAL"
    done
}

# 检测硬件解码器
detect_hw_decoder() {
    if [[ "$USE_HW_DECODER" == "off" ]]; then
        echo ""
    elif ffmpeg -hide_banner -hwaccels 2>/dev/null | grep -q videotoolbox; then
        echo "videotoolbox"
    else
        echo ""
    fi
}

# ---------- 检查环境 ----------
missing=""
command -v ffmpeg &>/dev/null || missing="$missing ffmpeg"
command -v ffprobe &>/dev/null || missing="$missing ffprobe"
if [[ -n "$missing" ]]; then
    echo "缺少依赖：$missing"
    if command -v brew &>/dev/null; then
        read -p "是否用 brew 安装？(y/n) " ans
        if [[ "$ans" == "y" || "$ans" == "Y" ]]; then
            brew install ffmpeg
        else
            echo "未安装依赖，退出。"
            read -p "按回车退出..."
            exit 1
        fi
    else
        echo "错误：未找到 brew，请先安装 Homebrew：https://brew.sh"
        read -p "按回车退出..."
        exit 1
    fi
fi

# 检测硬件解码器
HW_DECODER=$(detect_hw_decoder)

# ---------- 检查/安装 rife-ncnn-vulkan ----------
RIFE_BIN=""
if command -v rife-ncnn-vulkan &>/dev/null; then
    RIFE_BIN=$(command -v rife-ncnn-vulkan)
else
    RIFE_BIN=$(find "$INSTALL_DIR" -name "rife-ncnn-vulkan" -type f 2>/dev/null | head -1)
fi

if [[ -z "$RIFE_BIN" ]]; then
    echo "未找到 rife-ncnn-vulkan，需要下载（约 436MB）"
    read -p "是否现在下载安装？(y/n) " ans
    if [[ "$ans" != "y" && "$ans" != "Y" ]]; then
        echo "未安装，退出。"
        read -p "按回车退出..."
        exit 1
    fi
    echo "正在下载..."
    mkdir -p "$INSTALL_DIR"
    RIFE_ZIP="$HOME/Downloads/rife-ncnn-vulkan.zip"
    curl -L --progress-bar --proxy http://127.0.0.1:7890 -o "$RIFE_ZIP" "$RIFE_URL"
    echo "正在解压..."
    unzip -o -q "$RIFE_ZIP" -d "$INSTALL_DIR"
    rm -f "$RIFE_ZIP"
    RIFE_BIN=$(find "$INSTALL_DIR" -name "rife-ncnn-vulkan" -type f 2>/dev/null | head -1)
    if [[ -z "$RIFE_BIN" ]]; then
        echo "错误：安装失败，未找到 rife-ncnn-vulkan 二进制。"
        read -p "按回车退出..."
        exit 1
    fi
    chmod +x "$RIFE_BIN"
fi

# 定位模型目录（-m 需要绝对路径）
MODEL=""
for m in rife-v4.6 rife-v4 rife-v2.3 rife-anime; do
    MODEL=$(find "$(dirname "$RIFE_BIN")" -maxdepth 2 -type d -name "$m" 2>/dev/null | head -1)
    [[ -n "$MODEL" ]] && break
done
if [[ -z "$MODEL" ]]; then
    echo "警告：未找到 RIFE 模型目录，将使用默认模型（需在 rife 目录下运行）"
fi

# ---------- 添加路径（支持拖拽/粘贴/目录） ----------
links=()
add_paths() {
    local line="$1"
    # 整行是文件（含拖入时的转义空格）
    local unescaped="${line//\\ / }"
    if [[ -f "$unescaped" ]]; then
        links+=("$unescaped")
        return
    fi
    # 整行是目录：批量处理里面所有视频
    if [[ -d "$unescaped" ]]; then
        local before=${#links[@]}
        while IFS= read -r f; do
            links+=("$f")
        done < <(find "$unescaped" -maxdepth 1 -type f \( -iname "*.mp4" -o -iname "*.mkv" -o -iname "*.mov" -o -iname "*.avi" -o -iname "*.webm" -o -iname "*.flv" -o -iname "*.ts" -o -iname "*.m4v" -o -iname "*.wmv" \) 2>/dev/null)
        local after=${#links[@]}
        echo "  目录下找到 $((after-before)) 个视频"
        return
    fi
    # 按空格拆分（一次拖入多个文件）
    local parts
    parts=($line)
    local i=0
    while [[ $i -lt ${#parts[@]} ]]; do
        local cand="${parts[$i]}"
        local j=$((i+1))
        local found=0
        while :; do
            local cand_unesc="${cand//\\ / }"
            if [[ -f "$cand_unesc" ]]; then
                links+=("$cand_unesc")
                i=$j
                found=1
                break
            fi
            [[ $j -ge ${#parts[@]} ]] && break
            cand="$cand ${parts[$j]}"
            j=$((j+1))
        done
        if [[ $found -eq 0 ]]; then
            echo "  ⚠ 找不到文件：${cand//\\ / }"
            i=$((i+1))
        fi
    done
}

# ---------- 处理单个视频 ----------
process_video() {
    local file="$1"
    local idx="$2"
    local total="$3"
    local base
    base=$(basename "$file")
    base="${base%.*}"

    local file_start=$(date +%s)

    # 读取源帧率
    local fps_str fps
    fps_str=$(ffprobe -v error -select_streams v:0 -show_entries stream=r_frame_rate -of default=noprint_wrappers=1:nokey=1 "$file" 2>/dev/null)
    if [[ -z "$fps_str" ]]; then
        echo "  ⚠ 无法读取帧率，跳过"
        return 1
    fi
    fps=$(awk -F/ '{ if ($2+0 > 0) printf "%.3f", $1/$2; else print $1 }' <<< "$fps_str")
    echo "  源帧率：${fps}fps"

    # 已达标则跳过
    if awk -v f="$fps" -v t="$TARGET_FPS" 'BEGIN { exit !(f >= t) }'; then
        echo "  已是 ${TARGET_FPS}fps 或更高，跳过"
        return 0
    fi

    local vdir="$WORKDIR/$idx"
    mkdir -p "$vdir/in" "$vdir/out"

    # 提取音频（无音频流则跳过）
    local has_audio=0
    if ffmpeg -y -v error -i "$file" -vn -acodec copy "$vdir/audio.m4a" 2>/dev/null; then
        has_audio=1
    fi

    # 拆帧（带硬件解码加速）
    echo "  [1/4] 拆帧中..."
    local qarg=""
    [[ "$FRAME_FORMAT" == "jpg" ]] && qarg="-qscale:v $JPG_QUALITY"
    local hw_args=()
    if [[ -n "$HW_DECODER" ]]; then
        hw_args=(-hwaccel "$HW_DECODER")
    fi
    if ! ffmpeg -y -v error "${hw_args[@]}" -i "$file" $qarg "$vdir/in/frame_%08d.$FRAME_FORMAT"; then
        # 硬件解码失败时回退到软件解码
        if [[ ${#hw_args[@]} -gt 0 ]]; then
            echo "  ⚠ 硬件解码失败，改用软件解码"
            if ! ffmpeg -y -v error -i "$file" $qarg "$vdir/in/frame_%08d.$FRAME_FORMAT"; then
                echo "  ⚠ 拆帧失败，跳过"
                return 1
            fi
        else
            echo "  ⚠ 拆帧失败，跳过"
            return 1
        fi
    fi

    local input_frames=$(ls "$vdir/in" 2>/dev/null | wc -l | tr -d ' ')
    if [[ $input_frames -lt 2 ]]; then
        echo "  ⚠ 输入帧数过少，跳过"
        return 1
    fi
    local expected_output=$((input_frames * 2 - 1))
    echo "  输入 $input_frames 帧 → 预期输出 $expected_output 帧"

    # AI 插帧（默认 2x：30fps→60fps，24fps→48fps）
    echo "  [2/4] AI 插帧中（2x）..."
    local model_arg=""
    [[ -n "$MODEL" ]] && model_arg="-m $MODEL"

    monitor_rife_progress "$vdir/out" "$expected_output" &
    local monitor_pid=$!

    local rife_ok=0
    if "$RIFE_BIN" -i "$vdir/in" -o "$vdir/out" -g "$GPU_INDEX" -j "$THREADS" -f "%08d.$FRAME_FORMAT" $model_arg; then
        rife_ok=1
    else
        kill $monitor_pid 2>/dev/null
        wait $monitor_pid 2>/dev/null
        echo ""
        echo "  GPU 处理失败，尝试 CPU..."
        monitor_rife_progress "$vdir/out" "$expected_output" &
        monitor_pid=$!
        if "$RIFE_BIN" -i "$vdir/in" -o "$vdir/out" -g -1 -j "$THREADS" -f "%08d.$FRAME_FORMAT" $model_arg; then
            rife_ok=1
        fi
    fi

    kill $monitor_pid 2>/dev/null
    wait $monitor_pid 2>/dev/null
    echo ""

    if [[ $rife_ok -eq 0 ]]; then
        echo "  ⚠ 插帧失败，跳过"
        return 1
    fi

    # 合成目标帧率视频
    echo "  [3/4] 合成 ${TARGET_FPS}fps 视频..."
    local newfps
    newfps=$(awk -v f="$fps" 'BEGIN { printf "%.3f", f*2 }')
    local outfile="$OUTPUT_DIR/${base}_${TARGET_FPS}fps.mp4"
    local n=1
    while [[ -f "$outfile" ]]; do
        outfile="$OUTPUT_DIR/${base}_${TARGET_FPS}fps_$n.mp4"
        n=$((n+1))
    done

    local PROGRESS_FILE=$(mktemp -t ffmpeg_progress)
    local encode_start=$(date +%s)
    local encode_total=$expected_output

    if [[ $has_audio -eq 1 ]]; then
        ffmpeg -y -v error -progress "$PROGRESS_FILE" -framerate "$newfps" -i "$vdir/out/%08d.$FRAME_FORMAT" -i "$vdir/audio.m4a" -c:a copy -r "$TARGET_FPS" -c:v "$ENCODER" -crf "$CRF" -pix_fmt yuv420p -movflags +faststart "$outfile" &
    else
        ffmpeg -y -v error -progress "$PROGRESS_FILE" -framerate "$newfps" -i "$vdir/out/%08d.$FRAME_FORMAT" -r "$TARGET_FPS" -c:v "$ENCODER" -crf "$CRF" -pix_fmt yuv420p -movflags +faststart "$outfile" &
    fi
    local ffmpeg_pid=$!

    monitor_ffmpeg_progress "$PROGRESS_FILE" "$ffmpeg_pid" "$encode_total"
    wait "$ffmpeg_pid"
    local encode_ok=$?
    rm -f "$PROGRESS_FILE"

    echo ""

    if [[ $encode_ok -ne 0 ]]; then
        echo "  ⚠ 合成失败，跳过"
        return 1
    fi

    local file_elapsed=$(($(date +%s) - file_start))
    echo "  [4/4] ✅ 完成：$outfile  ⏱ $(format_time $file_elapsed)"
    echo "$file_elapsed" >> "$WORKDIR/times.log"
    return 0
}

# ---------- 界面 ----------
clear
echo "=============================================="
echo "          视频补帧到 ${TARGET_FPS}FPS 工具"
echo "=============================================="
echo "  引擎：rife-ncnn-vulkan (AI 插帧)"
echo "  输出目录：$OUTPUT_DIR"
if [[ -n "$HW_DECODER" ]]; then
    echo "  硬件加速：$HW_DECODER ✅"
else
    echo "  硬件加速：未启用（软件模式）"
fi
echo "=============================================="
echo ""
echo "操作方式："
echo "  把视频文件拖进窗口（可一次拖多个）"
echo "  或粘贴视频路径（每行一个）"
echo "  也可以拖入文件夹（批量处理里面所有视频）"
echo "  p = 从剪贴板读取路径"
echo "  q = 退出"
echo "  空行 = 开始处理"
echo ""

# ---------- 收集文件 ----------
while true; do
    read -r -p "视频> " input
    input="${input#"${input%%[![:space:]]*}"}"
    input="${input%"${input##*[![:space:]]}"}"
    if [[ -z "$input" ]]; then
        break
    fi
    case "$input" in
        q|Q)
            echo "已退出。"
            exit 0
            ;;
        p|P)
            echo "正在从剪贴板读取..."
            while IFS= read -r line; do
                line="${line#"${line%%[![:space:]]*}"}"
                line="${line%"${line##*[![:space:]]}"}"
                [[ -n "$line" ]] && add_paths "$line"
            done < <(pbpaste)
            echo "已读取 ${#links[@]} 个文件"
            ;;
        *)
            add_paths "$input"
            ;;
    esac
done

if [[ ${#links[@]} -eq 0 ]]; then
    echo "没有输入文件，退出。"
    read -p "按回车退出..."
    exit 0
fi

# 去重
unique=()
for f in "${links[@]}"; do
    if ! printf '%s\n' "${unique[@]}" | grep -qxF "$f"; then
        unique+=("$f")
    fi
done
links=("${unique[@]}")

mkdir -p "$OUTPUT_DIR"
echo ""
echo "共 ${#links[@]} 个视频，开始处理..."
echo "----------------------------------------------"

WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT
: > "$WORKDIR/times.log"

success=0
failed=0
total=${#links[@]}
batch_start=$(date +%s)
for i in "${!links[@]}"; do
    file="${links[$i]}"
    echo "[$((i+1))/$total] $file"

    # 显示批量 ETA（基于已完成视频的平均用时）
    if [[ $i -gt 0 && -s "$WORKDIR/times.log" ]]; then
        local avg_time=$(awk '{ s += $1; n += 1 } END { if (n > 0) printf "%.0f", s/n; else print 0 }' "$WORKDIR/times.log")
        local remain_count=$((total - i))
        local batch_eta=$((avg_time * remain_count))
        local elapsed_batch=$(($(date +%s) - batch_start))
        echo "  批量进度：$i/$total | 已用 $(format_time $elapsed_batch) | 预计剩余 $(format_time $batch_eta)"
    fi

    if process_video "$file" "$i" "$total"; then
        success=$((success+1))
    else
        failed=$((failed+1))
    fi
done

batch_elapsed=$(($(date +%s) - batch_start))
echo "----------------------------------------------"
echo "完成：成功 $success 个，失败 $failed 个"
echo "总用时：$(format_time $batch_elapsed)"
echo "输出目录：$OUTPUT_DIR"
echo ""
read -p "按回车退出..."