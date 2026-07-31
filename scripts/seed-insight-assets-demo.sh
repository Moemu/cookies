#!/usr/bin/env bash
# 给「投后分析」补齐平台对象背后的素材，让素材对比 / 驱动因素两个视图能出数。
#
# 为什么单独有这个脚本：seed-insight-performance-demo.sh 只导日指标，导进来的
# AD-1001~AD-1004 全部停在「待匹配」。指标总览、趋势、异常这三个视图不需要素材
# 也能算（它们看的是平台对象自己的时间序列），但素材对比和驱动因素要的是
# 「同一个素材的不同版本」，没有素材就一格都出不来。
#
# 素材的特征不是为了让图好看编的，是照着标题写的：
#   v1 问题+反差开场，v2 利益开场。seed-insight-drivers-demo.sh 补的 v3（利益）、
#   v4（问题+反差）跟这两条配成 2 对 2，钩子类型才够得上 minDriverAssets。
#   hook_type 的词条顺序必须和这里一致——分组按取值文本严格相等做，顺序一变就分成两组。
#
# 全程走真实接口，可重复执行：按标题查重，已存在就复用。
# 用法：先起后端并跑完 seed-insight-performance-demo.sh，再 bash scripts/seed-insight-assets-demo.sh
set -euo pipefail

export PYTHONUTF8=1 PYTHONIOENCODING=utf-8

BASE="${COOKIES_API:-http://127.0.0.1:8080}"
PROJECT="${COOKIES_DEMO_PROJECT:-k_project_nova_home_launch}"

python - "$BASE" "$PROJECT" <<'PY'
import http.cookiejar
import json
import sys
import urllib.error
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

ASSETS = [
    {
        "title": "Nova 夏季清洁 数字人口播 v1",
        "asset_type": "digital_human_ad",
        "objects": ["AD-1002", "AD-1004"],
        "note": "投放计划书里这两条创意用的都是 v1 原片，AD-1004 是同一版本的补投。",
        "features": [
            {"key": "hook_type", "value": {"kind": "enum_multi", "terms": ["问题", "反差"]}},
            {"key": "presenter", "value": {"kind": "enum", "terms": ["数字人"]}},
            {"key": "selling_points", "value": {"kind": "tags", "terms": ["强效去油", "免手洗"]}},
        ],
    },
    {
        "title": "Nova 夏季清洁 数字人口播 v2（利益开场）",
        "asset_type": "digital_human_ad",
        "objects": ["AD-1001"],
        "note": "投放计划书里这条创意就是 v2 原片。",
        "features": [
            {"key": "hook_type", "value": {"kind": "enum_multi", "terms": ["利益"]}},
            {"key": "presenter", "value": {"kind": "enum", "terms": ["数字人"]}},
            {"key": "selling_points", "value": {"kind": "tags", "terms": ["强效去油", "免手洗"]}},
        ],
    },
    {
        "title": "Nova 夏季清洁 公众号长文",
        "asset_type": "wechat_article",
        "objects": ["AD-1003"],
        "note": "公众号推文原文，和数字人素材不是同一类型，不参与钩子类型的分组。",
        "features": [],
    },
]

existing = {item["title"]: item for item in call("GET", f"{ROOT}/assets?limit=200")["items"]}

for spec in ASSETS:
    asset = existing.get(spec["title"])
    if asset is None:
        asset = call("POST", f"{ROOT}/assets", {
            "title": spec["title"],
            "source_kind": "upload",
            "asset_type": spec["asset_type"],
            "asset_type_source": "human",
        })
        if spec["features"]:
            call("PATCH", f"{ROOT}/assets/{asset['id']}/features", {
                "expected_version": asset["version"],
                "reason": "从交付件原片登记钩子、出镜形式和卖点。",
                "features": [dict(item, review_state="confirmed") for item in spec["features"]],
            })
        print(f"新建素材 {asset['id']} {spec['title']}")
    else:
        print(f"素材已存在，复用 {asset['id']} {spec['title']}")
    spec["asset_id"] = asset["id"]

targets = {}
for spec in ASSETS:
    for object_id in spec["objects"]:
        targets[object_id] = (spec["asset_id"], spec["note"])

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

print("完成。再跑 seed-insight-drivers-demo.sh 补 v3/v4，驱动因素才凑得齐 2 对 2。")
PY
