#!/usr/bin/env bash
# 给「投后分析」的五个分析视图铺一段够长的演示数据。
#
# 为什么单独有这个脚本：数据接入那一轮只导了 3 天。3 天足够让「指标总览」出数，
# 但趋势、疲劳、异常、驱动因素这四个视图全部需要一段时间序列——3 天的数据里
# 它们只能诚实地回答「看不出来」，于是页面看上去像是坏了，其实是数据不够。
#
# 全程走真实的导入接口（POST /import-batches），不直接写库：
# 落库路径、口径校验、未匹配对象入队这些都要跟着一起被验证到。
# 日指标是 upsert（uq_insight_metric_daily_fact），重复跑不会翻倍。
#
# 用法：先起后端，再 bash scripts/seed-insight-performance-demo.sh
set -euo pipefail

# Windows 上 python 默认按 gbk 写 stdout，中文素材名会直接炸在编码上。
export PYTHONUTF8=1 PYTHONIOENCODING=utf-8

BASE="${COOKIES_API:-http://127.0.0.1:8080}"
PROJECT="${COOKIES_DEMO_PROJECT:-k_project_nova_home_launch}"
COOKIE_JAR="$(mktemp)"
trap 'rm -f "$COOKIE_JAR"' EXIT

curl -sS -c "$COOKIE_JAR" -X POST "$BASE/platform/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"Admin","password":"123456"}' -o /dev/null

SOURCE_ID="$(curl -sS -b "$COOKIE_JAR" "$BASE/api/insights/v1/projects/$PROJECT/data-sources" \
  | python -c 'import json,sys; print(json.load(sys.stdin)["items"][0]["id"])')"
echo "数据源：$SOURCE_ID"

# 四条曲线各自演示一件事，形状是刻意设计的，不是随机噪声：
#   AD-1001（数字人口播 v2）曝光一路放大、点击率一路下滑 —— 疲劳最典型的形态
#   AD-1002（数字人口播 v1）稳定 —— 对比里的另一边，也是趋势里的「持平」
#   AD-1003（公众号长文）中间有一天冲高、有一天完全缺数 —— 异常的两种形态
#   AD-1004（数字人口播 v1 补投）和 v2 同一个钩子类型 —— 驱动因素才凑得齐一组
BODY_FILE="$(mktemp)"
trap 'rm -f "$COOKIE_JAR" "$BODY_FILE"' EXIT

python - "$SOURCE_ID" "$BODY_FILE" <<'PY'
import datetime
import json
import sys

start = datetime.date(2026, 7, 8)
rows = []


def add(object_id, name, day, impressions, ctr, cvr, cpm_cents, rev_per_conv):
    clicks = round(impressions * ctr)
    conversions = round(clicks * cvr)
    rows.append({
        "platform_object_kind": "ad",
        "platform_object_id": object_id,
        "platform_object_name": name,
        "stat_date": (start + datetime.timedelta(days=day)).isoformat(),
        "counts": {
            "impressions": impressions,
            "clicks": clicks,
            "conversions": conversions,
            "video_views": round(impressions * 0.62),
            "video_completions": round(impressions * 0.18),
            "spend_cents": round(impressions / 1000 * cpm_cents),
            "revenue_cents": conversions * rev_per_conv,
        },
    })


for day in range(21):
    # v2：曝光 8000 → 18000，点击率 3.3% → 1.8%。钱越花越多，效果越来越差。
    add("AD-1001", "夏季清洁-数字人口播-v2", day,
        8000 + day * 500, 0.033 - day * 0.00075, 0.052, 7000, 24000)
    # v1：一直稳定，点击率在 4.2% 上下小幅波动。
    add("AD-1002", "夏季清洁-数字人口播-v1", day,
        9200 + (day % 3) * 400, 0.042 + (day % 4 - 1.5) * 0.0009, 0.066, 7800, 24000)
    # 公众号长文：平时不温不火，第 12 天推文被大号转了一次，第 16 天完全没有回流。
    if day == 16:
        continue
    spike = 4.5 if day == 12 else 1.0
    # 基线要带正常抖动。写成每天恰好 4000 反而测不出东西：异常检测遇到
    # 零波动的序列会直接跳过（那种序列多半是补录或四舍五入填出来的），
    # 于是这一天的 4.5 倍冲高在页面上会凭空消失。
    add("AD-1003", "夏季清洁-公众号长文", day,
        round((4000 + (day % 5) * 210) * spike), 0.014, 0.02, 7300, 24000)
    # v1 补投：第 8 天才开始跑，和 v2 用同一个钩子类型。
    if day >= 8:
        add("AD-1004", "夏季清洁-数字人口播-v1-补投", day,
            6400 + (day % 2) * 300, 0.031, 0.058, 7100, 24000)

with open(sys.argv[2], "w", encoding="utf-8") as handle:
    json.dump({
        "data_source_id": sys.argv[1],
        "kind": "backfill",
        "source_label": "演示数据 · 投后分析五视图",
        "rows": rows,
        "register_objects": True,
    }, handle, ensure_ascii=False)
print(f"生成 {len(rows)} 行日指标")
PY

curl -sS -b "$COOKIE_JAR" -X POST "$BASE/api/insights/v1/projects/$PROJECT/import-batches" \
  -H 'Content-Type: application/json' --data-binary "@$BODY_FILE" \
  | python -c 'import json,sys; d=json.load(sys.stdin); b=d.get("batch",d); print("导入：接受", b.get("accepted_rows"), "拒绝", b.get("rejected_rows"))'

echo "完成。AD-1004 会出现在「分析素材库 · 待匹配」，认领给数字人口播 v1 之后，驱动因素才凑得齐一组。"
