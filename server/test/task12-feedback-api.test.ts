import assert from "node:assert/strict";
import test from "node:test";
import { api } from "../../src/data/api.js";

test("反馈飞轮前端 API 追加评分事件并创建 Planner 权重快照", async () => {
  const calls: Array<{ url: string; method: string; body?: any }> = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
    const url = String(input);
    const body = init?.body ? JSON.parse(String(init.body)) : undefined;
    calls.push({ url, method: init?.method ?? "GET", body });
    const payload = url.endsWith("/remix-feedback-events")
      ? feedbackEventResponse(body, calls.length)
      : url.endsWith("/remix-asset-performance")
        ? assetPerformanceResponse()
        : url.endsWith("/remix-planner-weight-snapshots")
          ? weightSnapshotResponse()
          : { error: { message: "unexpected path" } };
    const status = "error" in payload ? 404 : url.endsWith("/remix-asset-performance") ? 200 : 201;
    return new Response(JSON.stringify(payload), {
      status,
      headers: { "Content-Type": "application/json" },
    });
  }) as typeof fetch;
  try {
    const assetVersion = { asset_id: "asset_output_1", version: 1 };
    const planEvent = await api.createRemixFeedbackEvent("project_1", {
      event_type: "rating",
      target_type: "remix_plan",
      target_id: "remixplan_1",
      rating: 5,
      comment: "结构清晰，商品卖点完整。",
    });
    await api.createRemixFeedbackEvent("project_1", {
      event_type: "render_succeeded",
      target_type: "render_job",
      target_id: "remixrender_1",
      asset_version: assetVersion,
    });
    const performance = await api.getRemixAssetPerformance("project_1");
    const snapshot = await api.createPlannerWeightSnapshot("project_1");

    assert.equal(planEvent.id, "feedback_1");
    assert.equal(calls.length, 4);
    assert.match(calls[0]?.url ?? "", /\/platform\/v1\/projects\/project_1\/remix-feedback-events$/);
    assert.equal(calls[0]?.method, "POST");
    assert.equal(calls[0]?.body.target_type, "remix_plan");
    assert.equal(calls[1]?.body.event_type, "render_succeeded");
    assert.deepEqual(calls[1]?.body.asset_version, assetVersion);
    assert.equal(performance.items[0]?.render_succeeded_count, 1);
    assert.equal(snapshot.asset_weights[0]?.asset_version.asset_id, "asset_output_1");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

function feedbackEventResponse(body: any, index: number) {
  return {
    id: `feedback_${index}`,
    organization_id: "org_1",
    project_id: "project_1",
    created_at: "2026-07-26T00:00:00Z",
    ...body,
  };
}

function assetPerformanceResponse() {
  return {
    items: [{
      asset_version: { asset_id: "asset_output_1", version: 1 },
      selected_count: 2,
      render_succeeded_count: 1,
      feedback_count: 1,
      average_rating: 5,
      updated_at: "2026-07-26T00:00:00Z",
    }],
  };
}

function weightSnapshotResponse() {
  return {
    id: "weights_1",
    organization_id: "org_1",
    project_id: "project_1",
    asset_weights: [{
      asset_version: { asset_id: "asset_output_1", version: 1 },
      weight: 1.2,
      reasons: ["selected:2", "render_succeeded:1", "average_rating:5.00"],
    }],
    created_at: "2026-07-26T00:00:00Z",
  };
}
