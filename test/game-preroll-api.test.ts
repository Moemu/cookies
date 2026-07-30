import assert from "node:assert/strict";
import test from "node:test";
import { api } from "../src/data/api.ts";
import { buildDefendSunflowerInput } from "../src/data/gamePrerollFixture.ts";

test("game-preroll fixed fixture freezes licensed source and verified evidence", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ url: string; init: RequestInit }> = [];
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init });
    if (calls.length === 1) return jsonResponse({ id: "intake_game_1" }, 201);
    if (calls.length === 2) return jsonResponse({ id: "task_game_1" }, 201);
    return jsonResponse(sampleGameWorkspace());
  };
  try {
    await api.createManualGamePrerollWorkspace(
      "project_demo",
      buildDefendSunflowerInput({ asset_id: "asset_gameplay", version: 1 }),
    );
  } finally {
    globalThis.fetch = originalFetch;
  }

  assert.equal(calls.length, 3);
  const intake = JSON.parse(String(calls[0].init.body));
  assert.equal(intake.performance_mode, "game_preroll");
  assert.equal(intake.call_to_action, "立即下载");
  assert.deepEqual(intake.creative_routes[0].source_asset_refs, [{ asset_id: "asset_gameplay", version: 1 }]);
  assert.deepEqual(intake.creative_routes[0].evidence_refs, ["skill_choice_1", "skill_choice_2", "wave_2"]);
  assert.equal(intake.manual_game_preroll.source_video_rights, "confirmed");
  assert.deepEqual(
    intake.manual_game_preroll.allowed_mechanisms,
    ["choice_challenge", "tactical_tradeoff", "wave_escalation"],
  );
  assert.deepEqual(
    intake.manual_game_preroll.prohibited_mechanisms,
    ["failure_reversal", "merge_upgrade", "reward_reveal"],
  );
  assert.equal(
    calls[2].url,
    "/api/creative/v1/projects/project_demo/creative-tasks/task_game_1",
  );
});

test("game-preroll selected candidate uses the stable Seedance route alias", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ url: string; init: RequestInit }> = [];
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init });
    return jsonResponse({
      id: "job_game_1",
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
    await api.createGamePrerollVideoJob("project_demo", "task_game_1", 3, "candidate_2");
  } finally {
    globalThis.fetch = originalFetch;
  }

  assert.deepEqual(JSON.parse(String(calls[0].init.body)), { model_alias: "cookies.video.standard" });
  assert.equal(
    new Headers(calls[0].init.headers).get("Idempotency-Key"),
    "game-preroll-video-task_game_1-3-candidate_2",
  );
});

test("game-preroll candidate regeneration preserves the versioned generation config", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ url: string; init: RequestInit }> = [];
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init });
    return jsonResponse(sampleGameWorkspace());
  };
  try {
    await api.regenerateGamePrerollCandidates("project_demo", "task_game_1", 3);
  } finally {
    globalThis.fetch = originalFetch;
  }

  assert.equal(
    calls[0].url,
    "/api/creative/v1/projects/project_demo/creative-tasks/task_game_1/game-preroll:regenerate-candidates",
  );
  assert.deepEqual(JSON.parse(String(calls[0].init.body)), {
    expected_revision: 3,
    generation_config: {
      subtitle_style: "high_contrast_dynamic",
      hook_strength: 4,
      pace_profile: "punchy",
    },
  });
});

function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function sampleGameWorkspace() {
  return {
    task: { id: "task_game_1", performance_mode: "game_preroll", status: "draft" },
    video_draft: {
      revision: 1,
      game_preroll: {
        revision: 1,
        input_snapshot: {
          brief_id: "fixture_defend_sunflower_v1",
          brief_version: 1,
          brief_name: "《保卫向日葵》技能选择挑战",
          game_name: "保卫向日葵",
          gameplay_summary: "竖屏塔防技能选择",
          source_video: { asset_id: "asset_gameplay", version: 1 },
          source_video_rights: "confirmed",
          call_to_action: "立即下载",
          evidence_moments: [],
          allowed_mechanisms: [],
          prohibited_mechanisms: [],
        },
        readiness: {
          planning_ready: true,
          generation_ready: false,
          production_ready: false,
          blockers: ["selected_candidate"],
        },
        candidates: [],
      },
    },
  };
}
