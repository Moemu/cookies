import assert from "node:assert/strict";
import test from "node:test";
import { api, buildRemixPrerollInput, type ApiRemixPlan, type ApiRemixPreroll } from "../../src/data/api.js";

test("AI 前贴 Hook 前端流程按创建、读取、插入顺序调用平台 API", async () => {
  const calls: Array<{ url: string; method: string; body?: any }> = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input);
    const body = init?.body ? JSON.parse(String(init.body)) : undefined;
    calls.push({ url, method: init?.method ?? "GET", body });
    const payload = url.endsWith("/remix-prerolls")
      ? prerollResponse(body)
      : url.endsWith("/remix-prerolls/preroll_1")
        ? prerollResponse()
        : url.endsWith("/remix-prerolls/preroll_1/apply")
          ? appliedPlanResponse()
          : { error: { message: "unexpected path" } };
    const status = "error" in payload ? 404 : (url.endsWith("/apply") ? 200 : url.endsWith("/remix-prerolls") ? 201 : 200);
    return new Response(JSON.stringify(payload), {
      status,
      headers: { "Content-Type": "application/json" },
    });
  }) as typeof fetch;
  try {
    const input = buildRemixPrerollInput(
      "remixplan_1",
      "conflict",
      { asset_id: "asset_opening", version: 1 },
      "generate_video",
      4,
      ["9:16 竖版", "静音可理解"],
    );
    const preroll = await api.createRemixPreroll("project_1", input);
    const latest = await api.getRemixPreroll("project_1", preroll.id);
    const plan = await api.applyRemixPreroll("project_1", latest.id);

    assert.equal(calls.length, 3);
    assert.equal(calls[0]?.method, "POST");
    assert.match(calls[0]?.url ?? "", /\/platform\/v1\/projects\/project_1\/remix-prerolls$/);
    assert.equal(calls[0]?.body.hook_type, "conflict");
    assert.equal(calls[0]?.body.reference_asset.asset_id, "asset_opening");
    assert.equal(calls[0]?.body.mode, "generate_video");
    assert.equal(preroll.status, "ready");
    assert.match(calls[1]?.url ?? "", /\/remix-prerolls\/preroll_1$/);
    assert.match(calls[2]?.url ?? "", /\/remix-prerolls\/preroll_1\/apply$/);
    assert.equal(plan.segments[0]?.shots[0]?.asset_version.asset_id, "preroll_asset");
    assert.equal(plan.segments[0]?.shots[1]?.timeline?.start_seconds, 4);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

function prerollResponse(body?: any): ApiRemixPreroll {
  return {
    id: "preroll_1",
    organization_id: "org_1",
    project_id: "project_1",
    plan_id: body?.plan_id ?? "remixplan_1",
    hook_type: body?.hook_type ?? "conflict",
    reference_asset: body?.reference_asset ?? { asset_id: "asset_opening", version: 1 },
    style_constraints: body?.style_constraints ?? ["9:16 竖版", "静音可理解"],
    duration_seconds: body?.duration_seconds ?? 4,
    mode: body?.mode ?? "generate_video",
    prompt_draft: "为 RemixPlan remixplan_1 生成 4 秒 conflict 前贴 Hook",
    output_asset: { project_id: "project_1", asset_version: { asset_id: "preroll_asset", version: 1 } },
    quality_verdict: "pass",
    status: "ready",
    created_at: "2026-07-26T00:00:00Z",
    updated_at: "2026-07-26T00:00:00Z",
  };
}

function appliedPlanResponse(): ApiRemixPlan {
  return {
    id: "remixplan_1",
    organization_id: "org_1",
    project_id: "project_1",
    schema_version: "remix_plan_v2",
    client_plan_id: "client_plan_1",
    target_seconds: 30,
    actual_seconds: 18.4,
    pace: "balanced",
    segments: [
      {
        segment: "opening",
        label: "前段",
        target_seconds: 10,
        actual_seconds: 8.8,
        shots: [
          {
            id: "preroll_1_shot",
            source: "existing_asset",
            asset_version: { asset_id: "preroll_asset", version: 1 },
            timeline: { start_seconds: 0, duration_seconds: 4, in_point_seconds: 0, out_point_seconds: 4 },
            creative: { scene: "AI 前贴 Hook", shot_type: "conflict", dialogue_or_narration: "强钩子", subtitle: "3 秒内制造停留理由", cta_element: "" },
          },
          {
            id: "opening_shot_1",
            source: "existing_asset",
            asset_version: { asset_id: "asset_opening", version: 1 },
            timeline: { start_seconds: 4, duration_seconds: 4.8, in_point_seconds: 0, out_point_seconds: 4.8 },
            creative: { scene: "原 opening", shot_type: "asset_clip", dialogue_or_narration: "", subtitle: "", cta_element: "" },
          },
        ],
      },
      { segment: "middle", label: "中段", target_seconds: 15, actual_seconds: 4.8, shots: [] },
      { segment: "ending", label: "后段", target_seconds: 7, actual_seconds: 4.8, shots: [] },
    ],
    warnings: ["ai_preroll_applied"],
    summary: { selected_assets: 4, used_assets: 4, coverage_percent: 61, strategy: "balanced" },
  };
}
