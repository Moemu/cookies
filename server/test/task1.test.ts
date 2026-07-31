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
import { DEMO_PROJECT_NAME, seedDemoProject } from "../demo.js";
import { isDomainError } from "../errors.js";
import { createApp, openSeededRepository } from "../index.js";
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
    filePath: temporary.filePath,
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

test("预置路演项目可重复初始化且保留完整的投放模拟输入", async () => {
  const temporary = await createTemporaryRepository();
  try {
    const repository = await FileRepository.open(temporary.filePath);
    const first = await seedDemoProject(repository);
    const second = await seedDemoProject(repository);
    const artifacts = await repository.listArtifacts(first.id);
    const changeSets = await repository.listChangeSets(first.id);
    const auditEvents = await repository.listAuditEvents(first.id);
    const tasks = await repository.listBusinessTasks(first.id);

    assert.equal(first.id, second.id);
    assert.equal(first.name, DEMO_PROJECT_NAME);
    assert.equal((await repository.listProjects()).length, 1);
    assert.equal(artifacts.some((artifact) => artifact.kind === "brief" && artifact.status === "ready"), true);
    assert.equal(artifacts.some((artifact) => artifact.kind === "image" && artifact.status === "ready"), true);
    assert.equal(artifacts.some((artifact) => artifact.kind === "video" && artifact.status === "ready"), true);
    assert.equal(artifacts.some((artifact) => artifact.kind === "document" && artifact.content.startsWith("[strategy]")), true);
    assert.equal(tasks.length, 5);
    assert.equal(changeSets[0]?.status, "preflight_passed");
    assert.equal(changeSets[0]?.preflight?.passed, true);
    assert.equal(auditEvents.some((event) => event.action === "business_task.created"), true);
    assert.equal((await repository.listAuditEvents(first.id)).length, auditEvents.length);
  } finally {
    await temporary.dispose();
  }
});

test("启动补齐既有路演项目，并通过 API 保留用户项目和完整路演路径", async () => {
  const temporary = await createTemporaryRepository();
  let server: ReturnType<typeof createApp> | undefined;
  try {
    const initial = await FileRepository.open(temporary.filePath);
    const userProject = await initial.createProject({
      name: "用户保存的项目",
      brand: "用户品牌",
      objective: "不得被预置初始化覆盖",
      actor: "user",
    });
    const legacyDemo = await initial.createProject({
      name: DEMO_PROJECT_NAME,
      brand: "白域精工",
      objective: "向采购与研发负责人展示精度证据，获取高质量销售线索。",
      actor: "legacy-demo",
    });

    const repository = await openSeededRepository(temporary.filePath);
    assert.equal((await repository.getProject(userProject.id))?.brand, "用户品牌");
    assert.equal((await repository.getProject(legacyDemo.id))?.id, legacyDemo.id);
    assert.equal((await repository.listArtifacts(legacyDemo.id)).some(
      (artifact) => artifact.kind === "brief" && artifact.status === "ready",
    ), true);
    assert.equal((await repository.listArtifacts(legacyDemo.id)).some(
      (artifact) => artifact.kind === "image" && artifact.status === "ready",
    ), true);

    server = createApp({ repository });
    server.listen(0, "127.0.0.1");
    await once(server, "listening");
    const address = server.address();
    assert.ok(address && typeof address !== "string");
    const request = async (method: string, path: string, body?: unknown) => {
      const response = await fetch(`http://127.0.0.1:${address.port}${path}`, {
        method,
        headers: body === undefined ? undefined : { "Content-Type": "application/json" },
        body: body === undefined ? undefined : JSON.stringify(body),
      });
      return { status: response.status, body: await response.json() as any };
    };
    const projects = await request("GET", "/api/projects");
    const demo = projects.body.find((project: { id: string }) => project.id === legacyDemo.id);
    assert.equal(demo.id, legacyDemo.id);

    const artifacts = await request("GET", `/api/artifacts?projectId=${legacyDemo.id}`);
    const changeSets = await request("GET", `/api/change-sets?projectId=${legacyDemo.id}`);
    const changeSet = changeSets.body.find((item: { status: string }) => item.status === "preflight_passed");
    assert.equal(artifacts.body.some((artifact: { kind: string; status: string }) => (
      artifact.kind === "brief" && artifact.status === "ready"
    )), true);
    assert.equal(artifacts.body.some((artifact: { kind: string; status: string }) => (
      artifact.kind === "image" && artifact.status === "ready"
    )), true);
    assert.equal(changeSet.preflight.passed, true);
    assert.equal((await request("POST", `/api/change-sets/${changeSet.id}/approve`, {
      actor: "demo-smoke",
      role: "demo-approver",
    })).body.status, "approved");
    assert.equal((await request("POST", `/api/change-sets/${changeSet.id}/execute`, {
      actor: "demo-smoke",
    })).body.status, "executed");
    const audit = await request("GET", `/api/audit-events?projectId=${legacyDemo.id}`);
    assert.equal(audit.body.some((event: { action: string }) => event.action === "change_set.simulation_completed"), true);

    const reopened = await openSeededRepository(temporary.filePath);
    assert.equal((await reopened.getProject(userProject.id))?.objective, "不得被预置初始化覆盖");
    assert.equal((await reopened.listChangeSets(legacyDemo.id)).some(
      (item) => item.status === "preflight_passed" && item.preflight?.passed,
    ), true);
  } finally {
    if (server) {
      await new Promise<void>((resolve, reject) => {
        server!.close((error) => (error ? reject(error) : resolve()));
      });
    }
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
    const preflight = await app.request("POST", `/api/change-sets/${changeSet.body.id}/preflight`);
    assert.equal(preflight.body.status, "preflight_failed");

    const events = await app.request("GET", `/api/audit-events?projectId=${project.body.id}`);
    assert.equal(events.status, 200);
    assert.equal(events.body.length, 6);
    assert.equal(events.body.at(-1).action, "change_set.preflight_completed");

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

test("投放模拟必须通过预检并经演示审批后才能执行，且可审计回滚", async () => {
  const app = await startApi();
  try {
    const project = await app.request("POST", "/api/projects", {
      name: "投放模拟项目",
      brand: "Cookies",
      objective: "验证受控投放",
    });
    const incomplete = await app.request("POST", "/api/change-sets", {
      projectId: project.body.id,
      name: "缺失输入的投放",
      budgetLimit: 0,
    });
    const failedPreflight = await app.request("POST", `/api/change-sets/${incomplete.body.id}/preflight`);
    assert.equal(failedPreflight.status, 200);
    assert.equal(failedPreflight.body.status, "preflight_failed");
    assert.deepEqual(
      failedPreflight.body.preflight.checks.map((check: { code: string }) => check.code),
      ["confirmed_brief", "ready_creative", "budget_boundary"],
    );

    const brief = await app.request("POST", "/api/artifacts", {
      projectId: project.body.id,
      kind: "brief",
      content: "已确认的策略 Brief",
      status: "ready",
    });
    const creative = await app.request("POST", "/api/artifacts", {
      projectId: project.body.id,
      kind: "image",
      content: "已确认的创意资产",
      status: "ready",
    });
    const changeSet = await app.request("POST", "/api/change-sets", {
      projectId: project.body.id,
      name: "合规的投放模拟",
      artifactIds: [brief.body.id, creative.body.id],
      budgetLimit: 8600,
    });

    const blockedApproval = await app.request("POST", `/api/change-sets/${changeSet.body.id}/approve`, {
      actor: "Amelia",
      role: "demo-approver",
    });
    assert.equal(blockedApproval.status, 409);
    assert.equal(blockedApproval.body.error.code, "INVALID_STATE_TRANSITION");

    const passedPreflight = await app.request("POST", `/api/change-sets/${changeSet.body.id}/preflight`);
    assert.equal(passedPreflight.body.status, "preflight_passed");
    assert.equal(passedPreflight.body.preflight.passed, true);

    const forbiddenApproval = await app.request("POST", `/api/change-sets/${changeSet.body.id}/approve`, {
      actor: "Amelia",
      role: "viewer",
    });
    assert.equal(forbiddenApproval.status, 403);

    const approved = await app.request("POST", `/api/change-sets/${changeSet.body.id}/approve`, {
      actor: "Amelia",
      role: "demo-approver",
    });
    assert.equal(approved.body.status, "approved");

    const executed = await app.request("POST", `/api/change-sets/${changeSet.body.id}/execute`, {
      actor: "Amelia",
    });
    assert.equal(executed.body.status, "executed");
    assert.equal(executed.body.execution.simulated, true);
    assert.equal(executed.body.execution.evidence.length, 3);

    const rolledBack = await app.request("POST", `/api/change-sets/${changeSet.body.id}/rollback`, {
      actor: "Amelia",
      reason: "验证模拟回滚",
    });
    assert.equal(rolledBack.body.status, "rolled_back");
    assert.equal(rolledBack.body.rollback.reason, "验证模拟回滚");

    const audit = await app.request("GET", `/api/audit-events?projectId=${project.body.id}`);
    assert.deepEqual(
      audit.body.slice(-6).map((event: { action: string }) => event.action),
      [
        "change_set.preflight_completed",
        "change_set.approved",
        "change_set.simulation_started",
        "change_set.simulation_completed",
        "change_set.rollback_started",
        "change_set.rolled_back",
      ],
    );
  } finally {
    await app.dispose();
  }
});

test("ChangeSet 只能由命令端点推进，并在批准和执行时重验服务端预检", async () => {
  const app = await startApi();
  try {
    const project = await app.request("POST", "/api/projects", {
      name: "状态机防绕过项目",
      brand: "Cookies",
      objective: "验证 ChangeSet 服务端控制",
    });
    const incomplete = await app.request("POST", "/api/change-sets", {
      projectId: project.body.id,
      name: "缺失投放输入",
      budgetLimit: 0,
    });
    for (const status of ["preflight_passed", "executing"]) {
      assert.deepEqual(
        await app.request("PATCH", `/api/change-sets/${incomplete.body.id}`, { status }),
        {
          status: 405,
          body: {
            error: {
              code: "METHOD_NOT_ALLOWED",
              message: "Method is not allowed for this route",
            },
          },
        },
      );
    }
    const failedPreflight = await app.request("POST", `/api/change-sets/${incomplete.body.id}/preflight`);
    assert.equal(failedPreflight.body.status, "preflight_failed");
    assert.deepEqual(
      failedPreflight.body.preflight.checks.map((check: { passed: boolean }) => check.passed),
      [false, false, false],
    );

    const brief = await app.request("POST", "/api/artifacts", {
      projectId: project.body.id,
      kind: "brief",
      content: "已确认 Brief",
      status: "ready",
    });
    const creative = await app.request("POST", "/api/artifacts", {
      projectId: project.body.id,
      kind: "video",
      content: "已就绪创意",
      status: "ready",
    });
    const changeSet = await app.request("POST", "/api/change-sets", {
      projectId: project.body.id,
      name: "受控投放模拟",
      artifactIds: [brief.body.id, creative.body.id],
      budgetLimit: 1200,
    });
    assert.equal(
      (await app.request("POST", `/api/change-sets/${changeSet.body.id}/preflight`)).body.status,
      "preflight_passed",
    );
    assert.equal(
      (await app.request("POST", `/api/change-sets/${changeSet.body.id}/approve`, {
        actor: "Amelia",
        role: "viewer",
      })).status,
      403,
    );

    await app.request("PATCH", `/api/artifacts/${creative.body.id}`, { status: "archived" });
    const staleApproval = await app.request("POST", `/api/change-sets/${changeSet.body.id}/approve`, {
      actor: "Amelia",
      role: "demo-approver",
    });
    assert.deepEqual(staleApproval, {
      status: 409,
      body: {
        error: {
          code: "INVALID_STATE_TRANSITION",
          message: "Cannot approve change set because server preflight checks no longer pass",
        },
      },
    });

    await app.request("PATCH", `/api/artifacts/${creative.body.id}`, { status: "ready" });
    assert.equal(
      (await app.request("POST", `/api/change-sets/${changeSet.body.id}/approve`, {
        actor: "Amelia",
        role: "demo-approver",
      })).body.status,
      "approved",
    );
    await app.request("PATCH", `/api/artifacts/${creative.body.id}`, { status: "archived" });
    const staleExecution = await app.request("POST", `/api/change-sets/${changeSet.body.id}/execute`, {
      actor: "Amelia",
    });
    assert.equal(staleExecution.status, 409);
    assert.equal(staleExecution.body.error.code, "INVALID_STATE_TRANSITION");

    await app.request("PATCH", `/api/artifacts/${creative.body.id}`, { status: "ready" });
    assert.equal(
      (await app.request("POST", `/api/change-sets/${changeSet.body.id}/execute`, {
        actor: "Amelia",
      })).body.status,
      "executed",
    );

    const reopenedRepository = await FileRepository.open(app.filePath);
    const persistedAudit = await reopenedRepository.listAuditEvents(project.body.id);
    const approvalAudit = persistedAudit.find((event) => event.action === "change_set.approved");
    const executionAudit = persistedAudit.find((event) => event.action === "change_set.simulation_completed");
    assert.equal(typeof approvalAudit?.metadata.preflightCheckedAt, "string");
    assert.equal(executionAudit?.metadata.evidenceCount, 3);
  } finally {
    await app.dispose();
  }
});

test("前端主链路资源和 ChangeSet 在刷新后恢复服务端最终状态", async () => {
  const app = await startApi();
  try {
    const project = await app.request("POST", "/api/projects", {
      name: "前端恢复验证项目",
      brand: "Cookies",
      objective: "验证刷新后的持久化状态",
    });
    const brief = await app.request("POST", "/api/artifacts", {
      projectId: project.body.id,
      kind: "brief",
      content: "已确认的前端 Brief",
      status: "ready",
    });
    const creative = await app.request("POST", "/api/artifacts", {
      projectId: project.body.id,
      kind: "image",
      content: "已保存的前端创意",
      status: "ready",
    });
    const changeSet = await app.request("POST", "/api/change-sets", {
      projectId: project.body.id,
      name: "前端创建的投放模拟",
      artifactIds: [brief.body.id, creative.body.id],
      budgetLimit: 8600,
    });

    assert.equal((await app.request("POST", `/api/change-sets/${changeSet.body.id}/preflight`)).body.status, "preflight_passed");
    assert.equal((await app.request("POST", `/api/change-sets/${changeSet.body.id}/approve`, {
      actor: "Amelia Meng",
      role: "demo-approver",
    })).body.status, "approved");
    assert.equal((await app.request("POST", `/api/change-sets/${changeSet.body.id}/execute`, {
      actor: "Amelia Meng",
    })).body.status, "executed");
    assert.equal((await app.request("POST", `/api/change-sets/${changeSet.body.id}/rollback`, {
      actor: "Amelia Meng",
      reason: "验证刷新恢复",
    })).body.status, "rolled_back");

    const reopened = await FileRepository.open(app.filePath);
    const restoredArtifacts = await reopened.listArtifacts(project.body.id);
    const restoredChangeSets = await reopened.listChangeSets(project.body.id);
    const restoredAudit = await reopened.listAuditEvents(project.body.id);
    assert.deepEqual(restoredArtifacts.map((artifact) => [artifact.kind, artifact.status]), [
      ["brief", "ready"],
      ["image", "ready"],
    ]);
    assert.equal(restoredChangeSets[0]?.status, "rolled_back");
    assert.equal(restoredChangeSets[0]?.rollback?.reason, "验证刷新恢复");
    assert.deepEqual(
      restoredAudit.slice(-6).map((event) => event.action),
      [
        "change_set.preflight_completed",
        "change_set.approved",
        "change_set.simulation_started",
        "change_set.simulation_completed",
        "change_set.rollback_started",
        "change_set.rolled_back",
      ],
    );
  } finally {
    await app.dispose();
  }
});
