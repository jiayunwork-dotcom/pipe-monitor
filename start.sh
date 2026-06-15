#!/usr/bin/env bash
set -e

echo "============================================"
echo "  Pipeline Monitor 完整部署启动脚本"
echo "============================================"

PROJECT_ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
cd "$PROJECT_ROOT"

if [ -f .env ]; then
  echo "[1/7] 已加载 .env 文件"
else
  if [ -f .env.example ]; then
    echo "[1/7] 未找到 .env，从 .env.example 复制一份..."
    cp .env.example .env
    echo "  → 请根据实际需要修改 .env 中的密钥和告警通道配置"
  else
    echo "[1/7] 警告：未找到 .env 文件，使用默认配置"
  fi
fi

echo "[2/7] 检查 Docker 环境..."
if ! command -v docker &> /dev/null; then
    echo "  ✗ 错误：未安装 Docker，请先安装 Docker Desktop"
    exit 1
fi
if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
    echo "  ✗ 错误：未安装 docker-compose / docker compose 插件"
    exit 1
fi
echo "  ✓ Docker 环境已就绪"

echo "[3/7] 创建 Docker Volume..."
docker volume ls | grep -q pipe-monitor_postgres_data || docker volume create --name=pipe-monitor_postgres_data >/dev/null 2>&1 || true
docker volume ls | grep -q pipe-monitor_redis_data || docker volume create --name=pipe-monitor_redis_data >/dev/null 2>&1 || true

echo "[4/7] 下载基础镜像..."
docker compose pull --quiet 2>/dev/null || true

echo "[5/7] 构建并启动所有服务（首次可能需要较长时间）..."
if command -v docker-compose &> /dev/null; then
  docker-compose up -d --build
else
  docker compose up -d --build
fi

echo "[6/7] 等待服务健康检查通过..."
echo "  - PostgreSQL ..."
until docker exec pipe-monitor-postgres pg_isready -U pipe_admin -d pipe_monitor >/dev/null 2>&1; do
  sleep 2; echo -n "."
done
echo " ✓"
echo "  - Redis ..."
until docker exec pipe-monitor-redis redis-cli ping >/dev/null 2>&1; do
  sleep 2; echo -n "."
done
echo " ✓"

echo ""
echo "============================================"
echo "  🎉 部署完成！"
echo "============================================"
echo ""
echo "  前端管理界面:  http://localhost"
echo "  后端API文档:   http://localhost:8080/api/health"
echo "  数据库:       localhost:5432"
echo "  Redis:        localhost:6379"
echo ""
echo "  默认账户："
echo "    superadmin / Super@2024!  (超级管理员)"
echo "    bi_admin   / Admin@2024!  (业务线管理员)"
echo "    bi_member  / User@2024!   (普通成员)"
echo ""
echo "  运行状态上报 Webhook："
echo "    POST http://localhost/webhook/run/:pipelineCode"
echo "    Header: X-Webhook-Token=<管道的WebhookToken>"
echo ""
echo "  查看日志："
echo "    后端： docker logs -f pipe-monitor-backend"
echo "    Nginx：docker logs -f pipe-monitor-nginx"
echo "  停止："
echo "    ./stop.sh  或  docker compose down"
echo "============================================"
