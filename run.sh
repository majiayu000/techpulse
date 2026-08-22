#!/bin/bash
# TechPulse 启动脚本
# 用法:
#   ./run.sh [techpulse 参数...]
#
# 示例:
#   ./run.sh                          # 单次采集,生成日报
#   ./run.sh --daemon --interval 1h   # 守护进程模式,每小时采集
#   ./run.sh --list-sources           # 列出可用数据源

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# 编译(源码比二进制新时)
if [ ! -f ./techpulse ] || [ -n "$(find cmd internal -name '*.go' -newer ./techpulse -print -quit 2>/dev/null)" ]; then
    echo "编译中..."
    go build -o techpulse ./cmd/techpulse/ || exit 1
fi

echo "============================================================"
echo "  TechPulse - AI/Tech News Aggregator"
echo "============================================================"

exec ./techpulse "$@"
