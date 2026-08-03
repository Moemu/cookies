#!/usr/bin/env bash
# 给「投后分析 · 驱动因素」补足够的素材，并修一条认领错了的映射。
#
# 为什么需要这一步：驱动因素的门槛写在 performance.go 的 minDriverAssets——
# 同类型、带特征的素材不足 4 个时，任何分组都会退化成「一个对一个」，那不是
# 驱动因素，那是素材对比。演示项目里数字人素材只有 v1、v2 两个，所以那一格
# 无论怎么点都是空的。这里再补两个数字人素材（v3 走利益开场，v4 走问题+反差
# 开场），钩子类型才凑得出 2 对 2 的对照。
#
# 顺带修一条数据错误：上一轮把 AD-1004（v1 补投）认领给了一个已退役的 v1 副本。
# 退役素材不允许再写特征（PatchFeatures 会返回 409），而且把 2026-07 的投放挂在
# 已经下线的素材上本身就是错的，这里改挂到在用的那条 v1。
#
# 全程走真实接口，可重复执行：按标题查重，已存在就复用。
# 用法：先起后端，再 bash scripts/seed-insight-drivers-demo.sh
set -euo pipefail

export PYTHONUTF8=1 PYTHONIOENCODING=utf-8

BASE="${COOKIES_API:-http://127.0.0.1:8080}"
PROJECT="${COOKIES_DEMO_PROJECT:-k_project_nova_home_launch}"

python - "$BASE" "$PROJECT" <<'PY'
import datetime
import http.cookiejar
import json
import sys
import urllib.request

BASE, PROJECT = sys.argv[1], sys.argv[2]
ROOT = f"{BASE}/api/insights/v1/projects/{PROJECT}"

opener = urllib.request.build_opener(
    urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))


def call(method, url, body=None):
    data = json.dumps(body, ensure_ascii=False).encode("utf-8") if body is not None else None
    request = urllib.request.Request(url, data=data, method=method)
    if data is not None:
        request.add_header("Content-Type", "application/json")
    try:
        with opener.open(request) as response:
            raw = response.read()
    except urllib.error.HTTPError as err:
        raise SystemExit(f"{method} {url} -> {err.code} {err.read().decode('utf-8', 'replace')}")
    return json.loads(raw) if raw else {}


call("POST", f"{BASE}/platform/v1/auth/login", {"username": "Admin", "password": "123456"})

# 两个新素材的特征是照着标题写的，不是为了让图好看编的：
# v3 和 v2 同为利益开场，v4 和 v1 同为问题+反差开场——钩子类型上就有了 2 对 2。
# hook_type 的词条顺序必须和 v1 现有的一致（["问题","反差"]），
# 分组是按取值文本严格相等做的，顺序一变就分成两组了。
NEW_ASSETS = [
    {
        "title": "Nova 夏季清洁 数字人口播 v3（利益开场·紧字幕）",
        "object_id": "AD-1005",
        "object_name": "夏季清洁-数字人口播-v3",
        "features": [
            {"key": "hook_type", "value": {"kind": "enum_multi", "terms": ["利益"]}},
            {"key": "presenter", "value": {"kind": "enum", "terms": ["数字人"]}},
            {"key": "selling_points", "value": {"kind": "tags", "terms": ["强效去油", "免手洗"]}},
        ],
        # 利益开场这一组整体偏弱，和 v2 的走势对得上。
        "impressions": 7600, "ctr": 0.024, "cvr": 0.049,
    },
    {
        "title": "Nova 夏季清洁 数字人口播 v4（问题开场·反差结尾）",
        "object_id": "AD-1006",
        "object_name": "夏季清洁-数字人口播-v4",
        "features": [
            {"key": "hook_type", "value": {"kind": "enum_multi", "terms": ["问题", "反差"]}},
            {"key": "presenter", "value": {"kind": "enum", "terms": ["数字人"]}},
            {"key": "selling_points", "value": {"kind": "tags", "terms": ["强效去油", "免手洗"]}},
        ],
        "impressions": 8100, "ctr": 0.044, "cvr": 0.063,
    },
]

existing = {item["title"]: item for item in call("GET", f"{ROOT}/assets?limit=200")["items"]}

for spec in NEW_ASSETS:
    asset = existing.get(spec["title"])
    if asset is None:
        asset = call("POST", f"{ROOT}/assets", {
            "title": spec["title"],
            "source_kind": "upload",
            "asset_type": "digital_human_ad",
            "asset_type_source": "human",
        })
        asset = call("PATCH", f"{ROOT}/assets/{asset['id']}/features", {
            "expected_version": asset["version"],
            "reason": "从交付件原片登记钩子、出镜形式和卖点。",
            "features": [dict(item, review_state="confirmed") for item in spec["features"]],
        }) and call("GET", f"{ROOT}/assets/{asset['id']}")
        print(f"新建素材 {asset['id']} {spec['title']}")
    else:
        print(f"素材已存在，复用 {asset['id']} {spec['title']}")
    spec["asset_id"] = asset["id"]

source_id = call("GET", f"{ROOT}/data-sources")["items"][0]["id"]

start = datetime.date(2026, 7, 8)
rows = []
for spec in NEW_ASSETS:
    for day in range(21):
        impressions = spec["impressions"] + (day % 4) * 260
        clicks = round(impressions * spec["ctr"])
        conversions = round(clicks * spec["cvr"])
        rows.append({
            "platform_object_kind": "ad",
            "platform_object_id": spec["object_id"],
            "platform_object_name": spec["object_name"],
            "stat_date": (start + datetime.timedelta(days=day)).isoformat(),
            "counts": {
                "impressions": impressions,
                "clicks": clicks,
                "conversions": conversions,
                "video_views": round(impressions * 0.62),
                "video_completions": round(impressions * 0.18),
                "spend_cents": round(impressions / 1000 * 7200),
                "revenue_cents": conversions * 24000,
            },
        })

batch = call("POST", f"{ROOT}/import-batches", {
    "data_source_id": source_id,
    "kind": "backfill",
    "source_label": "演示数据 · 驱动因素对照组",
    "rows": rows,
    "register_objects": True,
})
batch = batch.get("batch", batch)
print(f"导入：接受 {batch.get('accepted_rows')} 拒绝 {batch.get('rejected_rows')}")

# 认领新对象，并把 AD-1004 从退役的 v1 副本改挂到在用的那条 v1。
live_v1 = next(
    item["id"] for item in call("GET", f"{ROOT}/assets?limit=200")["items"]
    if item["title"] == "Nova 夏季清洁 数字人口播 v1" and item["analysis_status"] != "retired")

targets = {spec["object_id"]: (spec["asset_id"], "投放计划书里这条创意就是该素材的原片。")
           for spec in NEW_ASSETS}
targets["AD-1004"] = (live_v1, "补投用的是在用的 v1 原片；之前挂到了已退役的同名副本上，属于认领错误。")

for mapping in call("GET", f"{ROOT}/asset-mappings?limit=200")["items"]:
    target = targets.get(mapping["platform_object_id"])
    if target is None or mapping.get("insight_asset_id") == target[0]:
        continue
    call("POST", f"{ROOT}/asset-mappings/{mapping['id']}:resolve", {
        "expected_version": mapping["version"],
        "status": "matched",
        "asset_id": target[0],
        "note": target[1],
    })
    print(f"认领 {mapping['platform_object_id']} → {target[0]}")

print("完成。数字人素材现在有 4 个带特征的，钩子类型能凑出 2 对 2 的对照组。")
PY
