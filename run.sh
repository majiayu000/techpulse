#!/bin/bash
# Autonomous Runner (Go 版本) 启动脚本
# 用法与 Python 版本一致:
#   ./run.sh [max_iterations] [max_cost] [max_duration_hours] [project_dir]
#
# 示例:
#   ./run.sh                    # 使用 config.yaml / 二进制默认限制
#   ./run.sh 50 20 4            # 50次迭代, $20成本, 4小时
#   ./run.sh 0 0 0              # 无限制

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# 仅记录用户显式传入的参数；未传入的限制留给 config.yaml。
PROJECT_DIR=$(pwd)
ARGS=(--dir "$PROJECT_DIR")

if [ "$#" -ge 1 ]; then
    MAX_ITERATIONS=$1
    ARGS+=(--max-iterations "$MAX_ITERATIONS")
fi
if [ "$#" -ge 2 ]; then
    MAX_COST=$2
    ARGS+=(--max-cost "$MAX_COST")
fi
if [ "$#" -ge 3 ]; then
    MAX_DURATION=$3
    ARGS+=(--max-duration "$MAX_DURATION")
fi
if [ "$#" -ge 4 ]; then
    PROJECT_DIR=$4
    ARGS=(--dir "$PROJECT_DIR")
    # 重新附上已显式给出的限制（--dir 必须排在最前）。
    if [ "$#" -ge 1 ]; then
        ARGS+=(--max-iterations "$1")
    fi
    if [ "$#" -ge 2 ]; then
        ARGS+=(--max-cost "$2")
    fi
    if [ "$#" -ge 3 ]; then
        ARGS+=(--max-duration "$3")
    fi
fi

# 编译（如果需要）
if [ ! -f "./orchestrator" ] || [ "./cmd/orchestrator/main.go" -nt "./orchestrator" ]; then
    echo "编译中..."
    go build -o orchestrator ./cmd/orchestrator/ || exit 1
fi

echo "============================================================"
echo "  Autonomous Runner v2.0.0 (Go)"
echo "============================================================"
echo "项目目录:     $PROJECT_DIR"
if [ "$#" -ge 1 ]; then
    echo "最大迭代:     $1 (0=无限)"
else
    echo "最大迭代:     (config.yaml / defaults)"
fi
if [ "$#" -ge 2 ]; then
    echo "最大成本:     \$$2 (0=无限)"
else
    echo "最大成本:     (config.yaml / defaults)"
fi
if [ "$#" -ge 3 ]; then
    echo "最大时间:     ${3}h (0=无限)"
else
    echo "最大时间:     (config.yaml / defaults)"
fi
echo "============================================================"
echo ""

# 运行：只传递用户显式提供的限制参数
./orchestrator "${ARGS[@]}"
