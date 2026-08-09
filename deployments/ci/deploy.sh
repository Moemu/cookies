#!/usr/bin/env bash
# 自动部署脚本（在服务器上执行，由 GitHub Actions 通过 SSH 触发）。
#
# 用法: deploy.sh <commit-sha>
#
# 前置: 调用方已把源码包放到 $ROOT/incoming/<sha>.tar.gz
#
# 为什么不在服务器上 git clone：
#   该服务器访问 GitHub 的下载速率约 26 KB/s，clone 必定超时。
#   因此改由 GitHub Actions 侧打包上传，服务器只负责构建与启动。
#
# 独立于 /opt/cookies-test 运行，项目名与端口均已错开，互不影响。

set -euo pipefail

COMMIT="${1:?用法: deploy.sh <commit-sha>}"
SHORT="${COMMIT:0:7}"

ROOT=/opt/cookies-ci
ENV_FILE="$ROOT/ci.env"
TARBALL="$ROOT/incoming/$COMMIT.tar.gz"
RELEASE="$ROOT/releases/release-$(date +%Y%m%d-%H%M)-$SHORT"
PROJECT=cookies-ci
COMPOSE_FILE=deployments/test/docker-compose.yml
HEALTH_URL=http://127.0.0.1:8091/healthz
KEEP_RELEASES=5

log() { echo "[$(date '+%F %T')] $*"; }

mkdir -p "$ROOT/incoming" "$ROOT/releases"

# 同一时间只允许一次部署，避免两次 push 撞车。
exec 9>"$ROOT/deploy.lock"
if ! flock -n 9; then
  log "已有部署在进行中，本次退出"
  exit 1
fi

[ -f "$ENV_FILE" ] || { log "缺少 $ENV_FILE"; exit 1; }
[ -f "$TARBALL" ]  || { log "缺少源码包 $TARBALL"; exit 1; }

# --- 展开源码 -------------------------------------------------------------
log "展开 $SHORT 到 $RELEASE"
mkdir -p "$RELEASE"
tar -xzf "$TARBALL" -C "$RELEASE"
echo "$COMMIT" > "$RELEASE/RELEASE_COMMIT"

cd "$RELEASE"

# --- 起服务 ---------------------------------------------------------------
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a
export COOKIES_IMAGE_TAG="ci-$SHORT"

log "构建并启动（项目 $PROJECT，标签 ci-$SHORT）"
docker compose -p "$PROJECT" -f "$COMPOSE_FILE" up -d --build --remove-orphans

# --- 健康检查 -------------------------------------------------------------
log "等待服务就绪"
for _ in $(seq 1 36); do
  if curl -fsS --max-time 5 "$HEALTH_URL" >/dev/null 2>&1; then
    log "部署成功: $SHORT 已上线 http://14.103.24.58:8091"
    ln -sfn "$RELEASE" "$ROOT/current"
    rm -f "$TARBALL"
    # 只保留最近几次发布，避免磁盘堆积
    ls -1dt "$ROOT/releases"/*/ 2>/dev/null | tail -n +$((KEEP_RELEASES + 1)) | xargs -r rm -rf
    docker image prune -f --filter 'until=168h' >/dev/null 2>&1 || true
    exit 0
  fi
  sleep 5
done

log "健康检查超时，导出容器状态供排查"
docker compose -p "$PROJECT" -f "$COMPOSE_FILE" ps
docker compose -p "$PROJECT" -f "$COMPOSE_FILE" logs --tail 50
exit 1
