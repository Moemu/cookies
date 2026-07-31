import assert from "node:assert/strict";
import test from "node:test";
import { api } from "../src/data/api.ts";

test("short-drama API restores the latest workspace and treats 404 as an empty workspace", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (input) => {
    assert.equal(
      String(input),
      "/api/creative/v1/projects/project_demo/creative-workspaces/short-drama-preroll",
    );
    return jsonResponse({ error: { message: "not found" } }, 404);
  };
  try {
    assert.equal(await api.getLatestShortDramaPrerollWorkspace("project_demo"), null);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("short-drama candidate regeneration sends controlled config and a stable operation key", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ url: string; init: RequestInit }> = [];
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init });
    return jsonResponse(sampleWorkspace());
  };
  try {
    await api.regenerateShortDramaPrerollCandidates(
      "project_demo",
      "task_1",
      3,
      {
        subtitle_style: "brand_minimal",
        hook_strength: 5,
        pace_profile: "punchy",
      },
      "more_visual",
    );
  } finally {
    globalThis.fetch = originalFetch;
  }

  assert.equal(
    calls[0].url,
    "/api/creative/v1/projects/project_demo/creative-tasks/task_1/short-drama-preroll:regenerate-candidates",
  );
  assert.equal(calls[0].init.method, "POST");
  assert.equal(
    new Headers(calls[0].init.headers).get("Idempotency-Key"),
    "short-drama-regenerate-task_1-3-brand_minimal-5-punchy-more_visual",
  );
  assert.deepEqual(JSON.parse(String(calls[0].init.body)), {
    expected_revision: 3,
    generation_config: {
      subtitle_style: "brand_minimal",
      hook_strength: 5,
      pace_profile: "punchy",
    },
    variation_intent: "more_visual",
  });
});

test("short-drama video generation key is bound to draft and candidate lineage", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ url: string; init: RequestInit }> = [];
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init });
    return jsonResponse({
      id: "job_1",
      project_id: "project_demo",
      job_type: "video.generate",
      execution_status: "queued",
      provider_status: "submitted",
      model_alias: "cookies.video.standard",
      project_asset_refs: [],
      version: 1,
      created_at: "2026-07-30T08:00:00Z",
      updated_at: "2026-07-30T08:00:00Z",
    });
  };
  try {
    await api.createShortDramaPrerollVideoJob("project_demo", "task_1", 4, "candidate_2");
  } finally {
    globalThis.fetch = originalFetch;
  }

  assert.equal(
    new Headers(calls[0].init.headers).get("Idempotency-Key"),
    "short-drama-video-task_1-4-candidate_2",
  );
});

function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function sampleWorkspace() {
  return {
    task: { id: "task_1", performance_mode: "short_drama_preroll", status: "in_progress" },
    video_draft: {
      revision: 4,
      short_drama_preroll: {
        revision: 4,
        input_snapshot: {
          brief_id: "brief_local_urban_reversal_v1",
          brief_version: 1,
          brief_name: "都市逆袭",
          story_title: "她的反击",
          synopsis: "一段满足服务端长度约束并且经过审核的短剧故事梗概，用于验证候选重新生成请求。",
          reviewed_selling_points: ["身份反转"],
          hook_strategy: "conflict_reversal",
          subtitle_style: "brand_minimal",
          transition: "hard_cut",
          hook_strength: 5,
          call_to_action: "点击看她如何翻盘",
        },
        readiness: {
          planning_ready: true,
          generation_ready: false,
          production_ready: false,
          blockers: ["candidate_selection_required"],
        },
        candidates: [],
      },
    },
  };
}
