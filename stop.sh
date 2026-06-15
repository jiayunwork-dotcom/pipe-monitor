#!/usr/bin/env bash
set -e
PROJECT_ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd "$PROJECT_ROOT"

echo "停止 Pipeline Monitor 服务..."

if command -v docker-compose &> /dev/null; then
  docker-compose down
else
  docker compose down
fi

echo ""
echo "所有服务已停止。"
echo "如果需要清理数据卷，请执行："
echo "  docker compose down -v"
