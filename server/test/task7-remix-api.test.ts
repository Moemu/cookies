import assert from "node:assert/strict";
import test from "node:test";
import { api, buildHitAnalysisInput, buildProductMappingInput, type ApiHitAnalysis } from "../../src/data/api.js";

test("爆款复刻前端流程按分析、映射、生成草案顺序调用平台 API", async () => {
  const calls: Array<{ url: string; method: string; body?: any }> = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input);
    const body = init?.body ? JSON.parse(String(init.body)) : undefined;
    calls.push({ url, method: init?.method ?? "GET", body });
    const payload = url.endsWith("/remix-hit-analyses")
      ? hitAnalysisResponse(body)
      : url.endsWith("/remix-product-mappings")
        ? productMappingResponse(body)
        : url.endsWith("/remix-product-mappings/productmapping_1/plans")
          ? remixPlanResponse()
          : { error: { message: "unexpected path" } };
    const status = "error" in payload ? 404 : 201;
    return new Response(JSON.stringify(payload), {
      status,
      headers: { "Content-Type": "application/json" },
    });
  }) as typeof fetch;
  try {
    const hitInput = buildHitAnalysisInput({ asset_id: "source_video", version: 1 }, "30 秒爆款结构样本", 30);
    const analysis = await api.createHitAnalysis("project_1", hitInput);
    const mappingInput = buildProductMappingInput(
      analysis,
      { name: "白域精工新品", selling_points: ["±0.01mm 精度", "98% 准时交付"], cta: "预约获取打样方案" },
      {
        hook: { asset_id: "target_hook", version: 1 },
        proof: { asset_id: "target_proof", version: 1 },
        cta: { asset_id: "target_cta", version: 1 },
      },
    );
    const mapping = await api.createProductMapping("project_1", mappingInput);
    const plan = await api.generatePlanFromProductMapping("project_1", mapping.id);

    assert.equal(calls.length, 3);
    assert.equal(calls[0]?.method, "POST");
    assert.match(calls[0]?.url ?? "", /\/platform\/v1\/projects\/project_1\/remix-hit-analyses$/);
    assert.equal(calls[1]?.body.hit_analysis_id, "hitanalysis_1");
    assert.deepEqual(calls[1]?.body.required_assets.map((asset: { asset_id: string }) => asset.asset_id), [
      "target_hook",
      "target_proof",
      "target_cta",
    ]);
    assert.ok(calls[1]?.body.replacement_rules.every((rule: { target_asset: { asset_id: string } }) => rule.target_asset.asset_id !== "source_video"));
    assert.match(calls[2]?.url ?? "", /\/remix-product-mappings\/productmapping_1\/plans$/);
    assert.equal(plan.schema_version, "remix_plan_v2");
    assert.equal(plan.summary.used_assets, 3);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

function hitAnalysisResponse(body: any): ApiHitAnalysis {
  return {
    id: "hitanalysis_1",
    organization_id: "org_1",
    project_id: "project_1",
    source_asset: body.source_asset,
    title: body.title,
    video_meta: { duration_seconds: body.duration_seconds, language: "zh-CN" },
    segments: [
      { id: "seg_1", start_seconds: 0, end_seconds: 6, role: "hook", summary: "开场反差", script: "先抛结果", visual_element: "强对比", conversion_cue: "停留", replication_hint: "替换卖点冲突" },
      { id: "seg_2", start_seconds: 6, end_seconds: 22, role: "proof", summary: "证据证明", script: "展示证据", visual_element: "产品细节", conversion_cue: "信任", replication_hint: "使用授权素材" },
      { id: "seg_3", start_seconds: 22, end_seconds: 30, role: "cta", summary: "行动引导", script: "预约打样", visual_element: "品牌收口", conversion_cue: "转化", replication_hint: "替换 CTA" },
    ],
    scripts: [],
    visual_elements: [],
    conversion_nodes: [],
    replication_insights: ["结构可复刻，但不得复用源视频二进制。"],
    created_at: "2026-07-26T00:00:00Z",
    updated_at: "2026-07-26T00:00:00Z",
  };
}

function productMappingResponse(body: any) {
  return {
    id: "productmapping_1",
    organization_id: "org_1",
    project_id: "project_1",
    created_at: "2026-07-26T00:00:00Z",
    updated_at: "2026-07-26T00:00:00Z",
    ...body,
  };
}

function remixPlanResponse() {
  return {
    id: "remixplan_1",
    organization_id: "org_1",
    project_id: "project_1",
    schema_version: "remix_plan_v2",
    client_plan_id: "mapping_productmapping_1",
    target_seconds: 30,
    actual_seconds: 30,
    pace: "balanced",
    segments: [],
    warnings: ["计划由爆款结构映射生成，未默认复用原视频二进制。"],
    summary: { selected_assets: 3, used_assets: 3, coverage_percent: 100, strategy: "hit-analysis product mapping" },
  };
}
