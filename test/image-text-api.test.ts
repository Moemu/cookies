import assert from "node:assert/strict";
import test from "node:test";
import { api } from "../src/data/api.ts";

test("image-text slot generation binds task and draft revisions and uses a fresh operation key", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ url: string; init: RequestInit }> = [];
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init });
    return jsonResponse({ attempt: { id: "attempt_1" } });
  };
  try {
    await api.generateImageTextSlot("project_demo", "task_1", 2, 7, 4);
  } finally {
    globalThis.fetch = originalFetch;
  }

  assert.equal(
    calls[0].url,
    "/api/creative/v1/projects/project_demo/creative-tasks/task_1/image-slots/2:generate",
  );
  assert.equal(calls[0].init.method, "POST");
  assert.deepEqual(JSON.parse(String(calls[0].init.body)), {
    expected_task_version: 7,
    draft_revision: 4,
  });
  assert.match(
    new Headers(calls[0].init.headers).get("Idempotency-Key") ?? "",
    /^image-slot-task_1-4-2-\d+$/,
  );
});

test("image-text adoption forwards the server selection version", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ url: string; init: RequestInit }> = [];
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init });
    return jsonResponse({});
  };
  try {
    await api.adoptImageTextAttempt("project_demo", "task_1", 3, "attempt_4", 11, 2);
  } finally {
    globalThis.fetch = originalFetch;
  }

  assert.equal(
    calls[0].url,
    "/api/creative/v1/projects/project_demo/creative-tasks/task_1/image-slots/3/attempts/attempt_4:adopt",
  );
  assert.deepEqual(JSON.parse(String(calls[0].init.body)), {
    expected_task_version: 11,
    expected_selection_version: 2,
  });
});

test("image-text authoring save advances from the exact task and draft revisions", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ url: string; init: RequestInit }> = [];
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init });
    return jsonResponse({});
  };
  const workspace = {
    task: { version: 6 },
    draft: {
      version: 3,
      selected_title: "旧标题",
      title_candidates: ["旧标题", "候选二", "候选三"],
      topics: ["#品牌"],
      image_plan: [
        { order: 1, overlay_copy: "旧封面" },
        { order: 2, overlay_copy: "旧证据" },
        { order: 3, overlay_copy: "旧行动" },
      ],
    },
  } as never;
  try {
    await api.updateImageTextDraft("project_demo", "task_1", workspace, {
      selectedTitle: "新标题",
      body: "新正文",
      overlayCopy: { 1: "新封面", 2: "新证据", 3: "新行动" },
    });
  } finally {
    globalThis.fetch = originalFetch;
  }

  const body = JSON.parse(String(calls[0].init.body));
  assert.equal(calls[0].init.method, "PATCH");
  assert.equal(body.expected_task_version, 6);
  assert.equal(body.expected_draft_revision, 3);
  assert.deepEqual(body.title_candidates, ["新标题", "候选二", "候选三"]);
  assert.equal(body.image_plan[2].overlay_copy, "新行动");
});

function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
