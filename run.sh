#!/bin/bash
# Autonomous Runner (Go 版本) 启动脚本
# 用法与 Python 版本一致:
#   ./run.sh [max_iterations] [max_cost] [max_duration_hours]
#
# 示例:
#   ./run.sh                    # 默认: 100次, $50, 8小时
#   ./run.sh 50 20 4            # 50次迭代, $20成本, 4小时
#   ./run.sh 0 0 0              # 无限制

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# 参数解析（与 Python 版本一致）
MAX_ITERATIONS=${1:-100}
MAX_COST=${2:-50}
MAX_DURATION=${3:-8}
PROJECT_DIR=${4:-$(pwd)}

# 编译（如果需要）
if [ ! -f "./orchestrator" ] || [ "./cmd/orchestrator/main.go" -nt "./orchestrator" ]; then
    echo "编译中..."
    go build -o orchestrator ./cmd/orchestrator/ || exit 1
fi

echo "============================================================"
echo "  Autonomous Runner v2.0.0 (Go)"
echo "============================================================"
echo "项目目录:     $PROJECT_DIR"
echo "最大迭代:     $MAX_ITERATIONS (0=无限)"
echo "最大成本:     \$$MAX_COST (0=无限)"
echo "最大时间:     ${MAX_DURATION}h (0=无限)"
echo "============================================================"
echo ""

# 运行
./orchestrator \
    --dir "$PROJECT_DIR" \
    --max-iterations "$MAX_ITERATIONS" \
    --max-cost "$MAX_COST" \
    --max-duration "$MAX_DURATION"
