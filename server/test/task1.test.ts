import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { once } from "node:events";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import {
  assertChangeSetTransition,
  assertGenerationJobTransition,
} from "../domain.js";
import { isDomainError } from "../errors.js";
import { createApp } from "../index.js";
import { FileRepository } from "../repository.js";

async function createTemporaryRepository(): Promise<{
  filePath: string;
  dispose: () => Promise<void>;
}> {
  const directory = await mkdtemp(join(tmpdir(), "cookies-mvp-"));
  return {
    filePath: join(directory, "store.json"),
    dispose: () => rm(directory, { recursive: true, force: true }),
  };
}

async function startApi() {
  const temporary = await createTemporaryRepository();
  const repository = await FileRepository.open(temporary.filePath);
  const server = createApp({ repository });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(address && typeof address !== "string");

  return {
    request: async (method: string, path: string, body?: unknown) => {
      const response = await fetch(`http://127.0.0.1:${address.port}${path}`, {
        method,
        headers: body === undefined ? undefined : { "Content-Type": "application/json" },
        body: body === undefined ? undefined : JSON.stringify(body),
      });
      return { status: response.status, body: await response.json() as any };
    },
    dispose: async () => {
      await new Promise<void>((resolve, reject) => {
        server.close((error) => (error ? reject(error) : resolve()));
      });
      await temporary.dispose();
    },
  };
}

test("状态机只接受定义的生成任务和 ChangeSet 转换", () => {
  assert.doesNotThrow(() => assertGenerationJobTransition("queued", "running"));
  assert.doesNotThrow(() => assertChangeSetTransition("approved", "executing"));
  assert.throws(
    () => assertGenerationJobTransition("succeeded", "running"),
    (error: unknown) => isDomainError(error, "INVALID_STATE_TRANSITION"),
  );
  assert.throws(
    () => assertChangeSetTransition("draft", "executed"),
    (error: unknown) => isDomainError(error, "INVALID_STATE_TRANSITION"),
  );
});

test("文件仓储在重新打开后恢复数据和审计轨迹", async () => {
  const temporary = await createTemporaryRepository();
  try {
    const firstRepository = await FileRepository.open(temporary.filePath);
    const project = await firstRepository.createProject({
      name: "夏季新品",
      brand: "Cookies",
      objective: "提高新品转化",
      actor: "investor-demo",
    });
    const artifact = await firstRepository.createArtifact({
      projectId: project.id,
      kind: "brief",
      content: "强调轻盈材质与限时权益",
      actor: "investor-demo",
    });
    const job = await firstRepository.createGenerationJob({
      projectId: project.id,
      artifactKind: "image",
      model: "future-task-2-model",
    });
    await firstRepository.transitionGenerationJob(job.id, "running", {});
    const changeSet = await firstRepository.createChangeSet({
      projectId: project.id,
      name: "首轮投放",
      artifactIds: [artifact.id],
      budgetLimit: 3000,
    });

    const recoveredRepository = await FileRepository.open(temporary.filePath);
    assert.equal((await recoveredRepository.getProject(project.id))?.name, "夏季新品");
    assert.equal((await recoveredRepository.getArtifact(artifact.id))?.content, "强调轻盈材质与限时权益");
    assert.equal((await recoveredRepository.getGenerationJob(job.id))?.status, "running");
    assert.equal((await recoveredRepository.getChangeSet(changeSet.id))?.budgetLimit, 3000);
    assert.equal((await recoveredRepository.listAuditEvents(project.id)).length, 5);
  } finally {
    await temporary.dispose();
  }
});

test("资源 API 持久化五类资源、审计状态变更，并返回统一错误", async () => {
  const app = await startApi();
  try {
    const health = await app.request("GET", "/health");
    assert.deepEqual(health, { status: 200, body: { status: "ok" } });

    const project = await app.request("POST", "/api/projects", {
      name: "路演项目",
      brand: "Cookies",
      objective: "验证广告创意",
      actor: "demo-user",
    });
    assert.equal(project.status, 201);

    const artifact = await app.request("POST", "/api/artifacts", {
      projectId: project.body.id,
      kind: "brief",
      content: "可执行策略 Brief",
    });
    assert.equal(artifact.status, 201);

    const job = await app.request("POST", "/api/generation-jobs", {
      projectId: project.body.id,
      artifactKind: "image",
      model: "doubao-seedream-5-0-pro-260628",
    });
    const runningJob = await app.request("PATCH", `/api/generation-jobs/${job.body.id}`, {
      status: "running",
    });
    assert.equal(runningJob.status, 200);
    assert.equal(runningJob.body.status, "running");

    const changeSet = await app.request("POST", "/api/change-sets", {
      projectId: project.body.id,
      name: "夏季投放模拟",
      artifactIds: [artifact.body.id],
      budgetLimit: 2000,
    });
    const preflight = await app.request("PATCH", `/api/change-sets/${changeSet.body.id}`, {
      status: "preflight_passed",
    });
    assert.equal(preflight.body.status, "preflight_passed");

    const events = await app.request("GET", `/api/audit-events?projectId=${project.body.id}`);
    assert.equal(events.status, 200);
    assert.equal(events.body.length, 6);
    assert.equal(events.body.at(-1).action, "change_set.status_changed");

    const invalidTransition = await app.request("PATCH", `/api/generation-jobs/${job.body.id}`, {
      status: "queued",
    });
    assert.deepEqual(invalidTransition, {
      status: 409,
      body: {
        error: {
          code: "INVALID_STATE_TRANSITION",
          message: "Cannot transition generation job from running to queued",
        },
      },
    });

    const invalidProject = await app.request("POST", "/api/projects", { name: "缺少字段" });
    assert.equal(invalidProject.status, 400);
    assert.equal(invalidProject.body.error.code, "VALIDATION_ERROR");
    assert.deepEqual(invalidProject.body.error.details, [{
      field: "brand",
      message: "Required non-empty string",
    }]);

    const missing = await app.request("GET", "/api/projects/not-found");
    assert.deepEqual(missing, {
      status: 404,
      body: { error: { code: "NOT_FOUND", message: "Resource was not found" } },
    });
  } finally {
    await app.dispose();
  }
});
