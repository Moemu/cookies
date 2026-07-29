import assert from "node:assert/strict";
import test from "node:test";
import {
  createPlatformClient,
  toApiBusinessTask,
  toApiGenerationJob,
  toApiProject,
  toDeliveryChangeSet,
  type PlatformBusinessTask,
  type PlatformChangeSet,
  type PlatformProjectDetail,
  type PlatformProviderJob,
} from "../src/data/platformClient.ts";

const now = "2026-07-28T08:00:00.000Z";

test("platform adapters preserve legacy Project, Task, ChangeSet and Job models", () => {
  const detail = sampleProjectDetail();

  assert.deepEqual(toApiProject(detail), {
    id: "project_demo",
    name: "Go Seed Demo",
    brand: "Seed Brand",
    objective: "Use Go platform data",
    runtime: {
      code: "GO-DEMO",
      product: "Seed Product",
      stage: "投放预检",
      progress: 76,
      status: "active",
      owner: "Lin Wei",
      budget: 320000,
      currency: "CNY",
      timezone: "Asia/Shanghai",
    },
    version: 3,
    createdAt: now,
    updatedAt: now,
  });

  assert.deepEqual(toApiBusinessTask(detail.tasks[0]), {
    id: "task_1",
    projectId: "project_demo",
    type: "creative",
    name: "生成创意",
    objective: "产出可投放素材",
    status: "ready",
    sourceTaskIds: ["task_0"],
    sourceArtifactIds: ["brief"],
    outputArtifactIds: ["creative"],
    version: 2,
    createdAt: now,
    updatedAt: now,
  });

  assert.deepEqual(toDeliveryChangeSet(detail.change_sets[0]), {
    id: "changeset_1",
    projectId: "project_demo",
    name: "素材组合与探索预算优化",
    status: "executed",
    artifactIds: ["asset_1"],
    budgetLimit: 88000,
    preflight: {
      passed: true,
      checks: [{ code: "ready_creative", passed: true, message: "ok", repair: "" }],
      checkedAt: now,
    },
    execution: {
      simulated: true,
      evidence: [{ step: "sync", status: "ok", message: "synced", recordedAt: now }],
      executedAt: now,
    },
    rollback: undefined,
    version: 5,
    createdAt: now,
    updatedAt: now,
  });

  assert.deepEqual(toApiGenerationJob(sampleProviderJob()), {
    id: "job_1",
    projectId: "project_demo",
    artifactKind: "image",
    status: "succeeded",
    model: "cookies.image.standard",
    diagnostic: undefined,
    artifactId: "asset_1",
    version: 4,
    createdAt: now,
    updatedAt: now,
  });
});

test("platform client uses project-scoped /platform/v1 endpoints", async () => {
  const calls: Array<{ url: string; init: RequestInit }> = [];
  const client = createPlatformClient({
    baseUrl: "https://cookies.example/platform/v1",
    idempotencyKey: () => "test-key",
    fetcher: async (url, init = {}) => {
      calls.push({ url: String(url), init });
      if (String(url).endsWith("/projects")) {
        return jsonResponse({ items: [sampleProjectDetail().project] });
      }
      if (String(url).endsWith("/projects/project_demo")) {
        return jsonResponse(sampleProjectDetail());
      }
      if (String(url).endsWith("/projects/project_demo/tasks")) {
        return jsonResponse(sampleProjectDetail().tasks[0], 201);
      }
      if (String(url).endsWith("/projects/project_demo/change-sets")) {
        return jsonResponse(sampleProjectDetail().change_sets[0], 201);
      }
      if (String(url).endsWith("/projects/project_demo/model/jobs")) {
        return jsonResponse(sampleProviderJob(), 202);
      }
      throw new Error(`unexpected URL ${url}`);
    },
  });

  await client.listProjects();
  await client.getProjectDetail("project_demo");
  await client.createTask("project_demo", { type: "creative", name: "生成创意", objective: "产出可投放素材" });
  await client.createChangeSet("project_demo", { name: "素材组合与探索预算优化", artifactIds: ["asset_1"], budgetLimit: 88000 });
  await client.createMedia("project_demo", "image", "launch poster", "brief_1");

  assert.deepEqual(calls.map(call => call.url), [
    "https://cookies.example/platform/v1/projects",
    "https://cookies.example/platform/v1/projects/project_demo",
    "https://cookies.example/platform/v1/projects/project_demo/tasks",
    "https://cookies.example/platform/v1/projects/project_demo/change-sets",
    "https://cookies.example/platform/v1/projects/project_demo/model/jobs",
  ]);
  assert.equal(calls[2].init.method, "POST");
  assert.equal(new Headers(calls[2].init.headers).get("Idempotency-Key"), "test-key");
  assert.equal(JSON.parse(calls[2].init.body as string).source_task_ids.length, 0);
  assert.deepEqual(JSON.parse(calls[3].init.body as string).artifact_refs, [
    { project_id: "project_demo", asset_version: { asset_id: "asset_1", version: 1 } },
  ]);
  assert.equal(JSON.parse(calls[4].init.body as string).capability, "image.generate");
});

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function sampleProjectDetail(): PlatformProjectDetail {
  return {
    project: {
      id: "project_demo",
      organization_id: "org_demo",
      name: "Go Seed Demo",
      status: "active",
      primary_brand_id: "brand_demo",
      project_context_version: 3,
      created_at: now,
      updated_at: now,
    },
    runtime: {
      code: "GO-DEMO",
      brand: "Seed Brand",
      product: "Seed Product",
      goal: "Use Go platform data",
      stage: "投放预检",
      progress: 76,
      status: "active",
      owner: "Lin Wei",
      budget: 320000,
      currency: "CNY",
      timezone: "Asia/Shanghai",
      knowledge_count: 2,
      updated_at: now,
    },
    artifacts: [{
      id: "artifact_brief",
      key: "brief",
      label: "策略 Brief",
      version: "v1.0",
      status: "已确认",
      owner: "服务端存档",
      updated_at: now,
      summary: "Brief from Go seed",
    }],
    assets: [],
    tasks: [sampleTask()],
    operations: [],
    change_sets: [sampleChangeSet()],
  };
}

function sampleTask(): PlatformBusinessTask {
  return {
    id: "task_1",
    project_id: "project_demo",
    type: "creative",
    name: "生成创意",
    objective: "产出可投放素材",
    status: "ready",
    source_task_ids: ["task_0"],
    source_artifact_ids: ["brief"],
    output_artifact_ids: ["creative"],
    version: 2,
    created_at: now,
    updated_at: now,
  };
}

function sampleChangeSet(): PlatformChangeSet {
  return {
    id: "changeset_1",
    project_id: "project_demo",
    name: "素材组合与探索预算优化",
    status: "executed",
    artifact_refs: [{ project_id: "project_demo", asset_version: { asset_id: "asset_1", version: 1 } }],
    budget_limit: 88000,
    preflight: {
      passed: true,
      checks: [{ code: "ready_creative", passed: true, message: "ok", repair: "" }],
      checked_at: now,
    },
    execution: {
      simulated: true,
      evidence: [{ step: "sync", status: "ok", message: "synced", recorded_at: now }],
      executed_at: now,
    },
    rollback: null,
    audit_events: [],
    version: 5,
    created_at: now,
    updated_at: now,
  };
}

function sampleProviderJob(): PlatformProviderJob {
  return {
    id: "job_1",
    kind: "provider.image.generate",
    organization_id: "org_demo",
    project_id: "project_demo",
    execution_status: "succeeded",
    provider_status: "succeeded",
    progress: 100,
    project_asset_refs: [{ project_id: "project_demo", asset_version: { asset_id: "asset_1", version: 1 } }],
    error: null,
    attempt_count: 1,
    max_attempts: 3,
    version: 4,
    created_at: now,
    updated_at: now,
  };
}
