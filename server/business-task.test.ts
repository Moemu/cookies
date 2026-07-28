import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { FileRepository } from "./repository.js";

test("业务任务持久化类型、项目来源和状态版本", async () => {
  const directory = await mkdtemp(join(tmpdir(), "cookies-business-task-"));
  const filePath = join(directory, "mvp-store.json");
  try {
    const repository = await FileRepository.open(filePath);
    const project = await repository.createProject({
      name: "真实任务项目",
      brand: "白域精工",
      objective: "验证策略到创意链路",
    });
    const brief = await repository.createArtifact({
      projectId: project.id,
      kind: "brief",
      content: "已确认 Brief",
      status: "ready",
    });
    const strategy = await repository.createBusinessTask({
      projectId: project.id,
      type: "strategy",
      name: "新品增长策略",
      objective: "形成可评审策略",
      sourceArtifactIds: [brief.id],
    });
    const creative = await repository.createBusinessTask({
      projectId: project.id,
      type: "short_drama_preroll",
      name: "短剧前贴",
      objective: "生成六秒冲突钩子",
      sourceTaskIds: [strategy.id],
      sourceArtifactIds: [brief.id],
    });
    const updated = await repository.updateBusinessTask(creative.id, {
      status: "in_progress",
      name: undefined,
      objective: undefined,
    });

    assert.equal(strategy.type, "strategy");
    assert.deepEqual(strategy.sourceArtifactIds, [brief.id]);
    assert.equal(updated.status, "in_progress");
    assert.equal(updated.name, "短剧前贴");
    assert.equal(updated.objective, "生成六秒冲突钩子");
    assert.deepEqual(updated.sourceTaskIds, [strategy.id]);
    assert.equal(updated.version, 2);

    const reopened = await FileRepository.open(filePath);
    assert.deepEqual(
      (await reopened.listBusinessTasks(project.id)).map((task) => [task.type, task.status]),
      [["strategy", "draft"], ["short_drama_preroll", "in_progress"]],
    );
    assert.equal(
      (await reopened.listAuditEvents(project.id)).filter((event) => event.entityType === "business_task").length,
      3,
    );
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});

test("业务任务拒绝关联其他 Project 的产物", async () => {
  const directory = await mkdtemp(join(tmpdir(), "cookies-business-task-boundary-"));
  try {
    const repository = await FileRepository.open(join(directory, "mvp-store.json"));
    const first = await repository.createProject({ name: "A", brand: "A", objective: "A" });
    const second = await repository.createProject({ name: "B", brand: "B", objective: "B" });
    const foreignBrief = await repository.createArtifact({
      projectId: second.id,
      kind: "brief",
      content: "B Brief",
      status: "ready",
    });
    await assert.rejects(
      repository.createBusinessTask({
        projectId: first.id,
        type: "creative",
        name: "错误关联",
        objective: "不应创建",
        sourceArtifactIds: [foreignBrief.id],
      }),
      /same project/,
    );
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});
