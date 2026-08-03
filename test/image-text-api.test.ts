import assert from "node:assert/strict";
import test from "node:test";
import { api } from "../src/data/api.ts";

test("manual image-text intake freezes a v3 direct-input request", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ url: string; init: RequestInit }> = [];
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init });
    return jsonResponse({ id: "intake_manual_1", source: "manual", status: "ready" }, 201);
  };
  try {
    const created = await api.createManualImageTextIntake("project_demo", {
      objective: " 建立新品认知 ",
      audience: "年轻通勤用户",
      coreMessage: " 0 糖青柠气泡水适合通勤场景 ",
      callToAction: " 搜索品牌 ",
      tone: ["清爽", "克制"],
      visualKeywords: ["青柠绿"],
      mandatoryElements: ["品牌名"],
      prohibitedClaims: ["不得虚构功效"],
    });
    assert.equal(created.id, "intake_manual_1");
  } finally {
    globalThis.fetch = originalFetch;
  }

  assert.equal(
    calls[0].url,
    "/api/creative/v1/projects/project_demo/creative-intakes",
  );
  assert.equal(calls[0].init.method, "POST");
  assert.deepEqual(JSON.parse(String(calls[0].init.body)), {
    contract_version: "creative-intake-create/v3",
    source: "manual",
    channel: "xiaohongshu",
    objective: "建立新品认知",
    audience: "年轻通勤用户",
    core_message: "0 糖青柠气泡水适合通勤场景",
    call_to_action: "搜索品牌",
    concept: "",
    tone: ["清爽", "克制"],
    visual_keywords: ["青柠绿"],
    mandatory_elements: ["品牌名"],
    prohibited_claims: ["不得虚构功效"],
  });
  assert.match(
    new Headers(calls[0].init.headers).get("Idempotency-Key") ?? "",
    /^manual-image-text-\d+-[a-z0-9]+$/,
  );
});

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

test("creative task library reads persisted tasks for the selected project", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ url: string; init: RequestInit }> = [];
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init });
    return jsonResponse({ items: [{ id: "task_saved_1", format: "image_text" }] });
  };
  try {
    const result = await api.listCreativeTasks("project with space", 30);
    assert.equal(result.items[0].id, "task_saved_1");
  } finally {
    globalThis.fetch = originalFetch;
  }

  assert.equal(
    calls[0].url,
    "/api/creative/v1/projects/project%20with%20space/creative-tasks?limit=30",
  );
  assert.equal(calls[0].init.method, "GET");
});

test("image-text delivery recovery reads persisted creative packages", async () => {
  const originalFetch = globalThis.fetch;
  const calls: Array<{ url: string; init: RequestInit }> = [];
  globalThis.fetch = async (input, init = {}) => {
    calls.push({ url: String(input), init });
    return jsonResponse({
      items: [{
        id: "creativepackage_1",
        creative_version_id: "creativeversion_1",
        format: "image_text",
      }],
    });
  };
  try {
    const result = await api.listCreativePackages("project demo", 25);
    assert.equal(result.items[0].creative_version_id, "creativeversion_1");
  } finally {
    globalThis.fetch = originalFetch;
  }

  assert.equal(
    calls[0].url,
    "/api/creative/v1/projects/project%20demo/creative-packages?limit=25",
  );
  assert.equal(calls[0].init.method, "GET");
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
