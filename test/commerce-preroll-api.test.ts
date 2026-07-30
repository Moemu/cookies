import assert from "node:assert/strict";
import test from "node:test";
import { api } from "../src/data/api.ts";

test("commerce pre-roll restores the latest workspace and treats 404 as empty", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (input) => {
    assert.equal(
      String(input),
      "/api/creative/v1/projects/project_demo/creative-workspaces/commerce-preroll",
    );
    return jsonResponse({ error: { message: "not found" } }, 404);
  };
  try {
    assert.equal(await api.getLatestCommercePrerollWorkspace("project_demo"), null);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("commerce pre-roll draft update is revision-bound and idempotent", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ url: string; init: RequestInit }> = [];
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init });
    return jsonResponse({});
  };
  const input = {
    expected_revision: 3,
    template_ref: {
      template_id: "commerce.window-reveal" as const,
      template_version: 1 as const,
    },
    motion: "一只戴浅色手套的手只完成一次横向擦拭。",
  };
  try {
    await api.updateCommercePrerollDraft("project_demo", "task_1", input);
  } finally {
    globalThis.fetch = originalFetch;
  }

  assert.equal(
    calls[0].url,
    "/api/creative/v1/projects/project_demo/creative-tasks/task_1/commerce-preroll-draft",
  );
  assert.equal(calls[0].init.method, "PATCH");
  const operationKey = new Headers(calls[0].init.headers).get("Idempotency-Key");
  assert.match(operationKey ?? "", /^commerce-draft-task_1-3-/);
  assert.deepEqual(JSON.parse(String(calls[0].init.body)), input);
});

test("commerce pre-roll video creation treats an omitted empty attempt list as zero attempts", async () => {
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
    await api.createCommercePrerollWorkspaceVideoJob(
      "project_demo",
      {
        task: { id: "task_1" },
        video_draft: {
          commerce_preroll: {
            revision: 3,
            plan: {
              generation_spec: {
                generation_spec_hash: "sha256:1234567890abcdef",
              },
            },
          },
        },
      } as never,
    );
  } finally {
    globalThis.fetch = originalFetch;
  }

  assert.equal(
    calls[0].url,
    "/api/creative/v1/projects/project_demo/creative-tasks/task_1:video-job",
  );
  assert.equal(
    new Headers(calls[0].init.headers).get("Idempotency-Key"),
    "commerce-video-task_1-3-1-567890abcdef",
  );
});

function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
