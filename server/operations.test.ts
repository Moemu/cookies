import assert from "node:assert/strict";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { once } from "node:events";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { seedDemoProject } from "./demo.js";
import { createApp } from "./index.js";
import { FileRepository } from "./repository.js";

async function temporaryRepository() {
  const directory = await mkdtemp(join(tmpdir(), "cookies-operations-"));
  return {
    filePath: join(directory, "mvp-store.json"),
    dispose: () => rm(directory, { recursive: true, force: true }),
  };
}

test("历史项目在读取时补齐可识别的服务端 runtime 默认值", async () => {
  const temporary = await temporaryRepository();
  try {
    await writeFile(temporary.filePath, JSON.stringify({
      projects: [{
        id: "legacy-project",
        name: "历史项目",
        brand: "Cookies",
        objective: "验证 runtime 迁移",
        version: 1,
        createdAt: "2026-07-01T00:00:00.000Z",
        updatedAt: "2026-07-01T00:00:00.000Z",
      }],
    }), "utf8");
    const repository = await FileRepository.open(temporary.filePath);

    assert.deepEqual((await repository.getProject("legacy-project"))?.runtime, {
      code: "PRJ",
      product: "",
      stage: "需求梳理",
      progress: 0,
      status: "active",
      owner: "demo-user",
      budget: 0,
      currency: "CNY",
      timezone: "Asia/Shanghai",
    });
  } finally {
    await temporary.dispose();
  }
});

test("路演运营示例以稳定 ID 幂等写入，并在重启后保留项目运行信息", async () => {
  const temporary = await temporaryRepository();
  try {
    const repository = await FileRepository.open(temporary.filePath);
    const project = await seedDemoProject(repository);
    const firstRecords = await repository.listOperationalRecords(project.id);
    const firstWorkItem = firstRecords.find((record) => record.id === "WORK-2607-01");
    const metric = firstRecords.find((record) => record.id === "METRIC-2607-01");
    const crmMetric = firstRecords.find((record) => record.id === "METRIC-2607-02");
    const deploymentCheck = firstRecords.find((record) => record.id === "WORK-2607-06");
    const artifacts = await repository.listArtifacts(project.id);
    const tasks = await repository.listBusinessTasks(project.id);
    assert.equal(project.runtime.stage, "投放审批");
    assert.equal(project.runtime.progress, 82);
    assert.equal(project.runtime.budget, 1_000_000);
    assert.ok(firstRecords.length >= 50);
    assert.equal(artifacts.some((artifact) => artifact.kind === "document" && artifact.content.startsWith("[strategy]")), true);
    assert.equal(artifacts.some((artifact) => artifact.kind === "document" && artifact.content.startsWith("[insight]")), true);
    assert.equal(artifacts.some((artifact) => artifact.kind === "document" && artifact.content.startsWith("[delivery]")), true);
    assert.equal(artifacts.some((artifact) => artifact.kind === "video" && artifact.content.includes("15 秒精密制造品牌片")), true);
    assert.equal(tasks.length, 5);
    assert.equal(tasks.some((task) => task.type === "strategy" && task.status === "ready"), true);
    assert.equal(tasks.some((task) => task.type === "short_drama_preroll" && task.status === "ready"), true);
    assert.equal(tasks.some((task) => task.type === "video_edit" && task.outputArtifactIds.length > 0), true);
    assert.equal(firstWorkItem?.fields.owner, "Amelia Meng");
    assert.equal(deploymentCheck?.fields.owner, "系统");
    assert.equal(metric?.fields.summary, "证据前置版本，正在形成稳定增量。");
    assert.equal(metric?.fields.recommendation, "下一轮将证据前置版本扩大到 30% 素材覆盖，并保留纯产品特写作为对照组。");
    assert.equal(metric?.fields.points, "32,39,36,49,54,51,63,67,72,78,75,86");
    assert.equal(crmMetric?.fields.summary, "高意向表单占比持续提升，研发负责人样本贡献最高。");

    await seedDemoProject(repository);
    const secondRecords = await repository.listOperationalRecords(project.id);
    const secondWorkItem = secondRecords.find((record) => record.id === "WORK-2607-01");
    assert.equal(secondRecords.length, firstRecords.length);
    assert.equal(secondWorkItem?.createdAt, firstWorkItem?.createdAt);
    assert.equal(secondWorkItem?.updatedAt, firstWorkItem?.updatedAt);
    assert.equal((await repository.listBusinessTasks(project.id)).length, tasks.length);

    const reopened = await FileRepository.open(temporary.filePath);
    assert.equal((await reopened.getProject(project.id))?.runtime.owner, "Noah Xu");
    assert.equal((await reopened.listOperationalRecords(project.id)).length, firstRecords.length);
  } finally {
    await temporary.dispose();
  }
});

test("既有路演 Project 在部署启动后会迁移 runtime 并补齐完整演示包", async () => {
  const temporary = await temporaryRepository();
  try {
    await writeFile(temporary.filePath, JSON.stringify({
      projects: [{
        id: "existing-demo",
        name: "投资人路演：精度证据增长",
        brand: "白域精工",
        objective: "向采购与研发负责人展示精度证据，获取高质量销售线索。",
        runtime: {
          code: "PRJ",
          product: "",
          stage: "需求梳理",
          progress: 0,
          status: "active",
          owner: "legacy-user",
          budget: 0,
          currency: "CNY",
          timezone: "Asia/Shanghai",
        },
        version: 1,
        createdAt: "2026-07-01T00:00:00.000Z",
        updatedAt: "2026-07-01T00:00:00.000Z",
      }],
    }), "utf8");
    const repository = await FileRepository.open(temporary.filePath);

    const project = await seedDemoProject(repository);

    assert.equal(project.id, "existing-demo");
    assert.deepEqual(project.runtime, {
      code: "SP",
      product: "高精度 CNC 加工零部件",
      stage: "投放审批",
      progress: 82,
      status: "active",
      owner: "Noah Xu",
      budget: 1_000_000,
      currency: "CNY",
      timezone: "Asia/Shanghai",
    });
    assert.equal((await repository.listArtifacts(project.id)).filter((artifact) => artifact.status === "ready").length >= 6, true);
    assert.equal((await repository.listBusinessTasks(project.id)).length, 5);
    assert.equal((await repository.listOperationalRecords(project.id)).some((record) => record.id === "REC-AUDIT-2607-04"), true);
  } finally {
    await temporary.dispose();
  }
});

test("项目运营数据 API 按项目隔离，并拒绝不存在的项目", async () => {
  const temporary = await temporaryRepository();
  let server: ReturnType<typeof createApp> | undefined;
  try {
    const repository = await FileRepository.open(temporary.filePath);
    const demo = await seedDemoProject(repository);
    const otherProject = await repository.createProject({
      name: "隔离验证项目",
      brand: "Cookies",
      objective: "验证运营数据隔离",
    });
    server = createApp({ repository });
    server.listen(0, "127.0.0.1");
    await once(server, "listening");
    const address = server.address();
    assert.ok(address && typeof address !== "string");
    const request = async (path: string) => {
      const response = await fetch(`http://127.0.0.1:${address.port}${path}`);
      return { status: response.status, body: await response.json() as any };
    };

    const projects = await request("/api/projects");
    const demoProject = projects.body.find((project: { id: string }) => project.id === demo.id);
    assert.equal(demoProject.runtime.owner, "Noah Xu");
    assert.equal(demoProject.runtime.budget, 1_000_000);
    const demoRecords = await request(`/api/projects/${demo.id}/operations`);
    assert.equal(demoRecords.status, 200);
    assert.equal(demoRecords.body.some((record: { id: string }) => record.id === "DIAG-2607-01"), true);
    assert.equal(demoRecords.body.some((record: { kind: string }) => record.kind === "performance_ad"), true);
    const trend = demoRecords.body.find((record: { id: string }) => record.id === "METRIC-2607-01");
    assert.equal(trend.fields.points, "32,39,36,49,54,51,63,67,72,78,75,86");
    assert.equal(demoRecords.body.some((record: { id: string }) => record.id === "WORK-2607-06"), true);
    assert.equal(demoRecords.body.some((record: { id: string }) => record.id === "METRIC-2607-02"), true);
    assert.equal(demoRecords.body.every((record: { projectId: string }) => record.projectId === demo.id), true);
    const otherRecords = await request(`/api/projects/${otherProject.id}/operations`);
    assert.deepEqual(otherRecords, { status: 200, body: [] });
    const missing = await request("/api/projects/missing/operations");
    assert.deepEqual(missing, {
      status: 404,
      body: { error: { code: "NOT_FOUND", message: "Resource was not found" } },
    });
  } finally {
    if (server) {
      await new Promise<void>((resolve, reject) => {
        server!.close((error) => (error ? reject(error) : resolve()));
      });
    }
    await temporary.dispose();
  }
});

test("核心演示种子会清理误归属到非 demo Project 的旧数据", async () => {
  const temporary = await temporaryRepository();
  try {
    const userProjectId = "user-project";
    await writeFile(temporary.filePath, JSON.stringify({
      projects: [{
        id: userProjectId,
        name: "用户项目",
        brand: "Cookies",
        objective: "验证 demo 数据不会污染用户项目",
        version: 1,
        createdAt: "2026-07-01T00:00:00.000Z",
        updatedAt: "2026-07-01T00:00:00.000Z",
      }],
      operationalRecords: [{
        id: "WORK-2607-01",
        projectId: userProjectId,
        kind: "work_item",
        title: "误归属工作项",
        status: "待评审",
        occurredAt: "2026-07-22T08:30:00.000Z",
        fields: { owner: "Amelia Meng" },
        createdAt: "2026-07-22T08:30:00.000Z",
        updatedAt: "2026-07-22T08:30:00.000Z",
      }],
      artifacts: [{
        id: "leaked-demo-brief",
        projectId: userProjectId,
        kind: "brief",
        status: "ready",
        content: "已确认 Brief：以 ±0.01mm 精度、98%+ 准时交付和真实制造场景为核心证据，面向采购与研发负责人获取销售线索。",
        version: 1,
        createdAt: "2026-07-22T08:30:00.000Z",
        updatedAt: "2026-07-22T08:30:00.000Z",
      }],
      changeSets: [{
        id: "leaked-demo-change-set",
        projectId: userProjectId,
        name: "精度证据创意与探索预算模拟",
        status: "preflight_passed",
        artifactIds: ["leaked-demo-brief"],
        budgetLimit: 8600,
        preflight: {
          passed: true,
          checkedAt: "2026-07-22T08:30:00.000Z",
          checks: [],
        },
        version: 1,
        createdAt: "2026-07-22T08:30:00.000Z",
        updatedAt: "2026-07-22T08:30:00.000Z",
      }],
      auditEvents: [{
        id: "leaked-demo-audit",
        projectId: userProjectId,
        action: "demo.seed_verified",
        entityType: "project",
        entityId: userProjectId,
        actor: "demo-seeder",
        metadata: { source: "startup" },
        occurredAt: "2026-07-22T08:30:00.000Z",
      }],
    }), "utf8");
    const repository = await FileRepository.open(temporary.filePath);

    const demo = await seedDemoProject(repository);

    assert.notEqual(demo.id, userProjectId);
    assert.equal((await repository.listOperationalRecords(userProjectId)).some((record) => record.id === "WORK-2607-01"), false);
    assert.equal((await repository.listArtifacts(userProjectId)).some((artifact) => artifact.id === "leaked-demo-brief"), false);
    assert.equal((await repository.listChangeSets(userProjectId)).some((changeSet) => changeSet.name === "精度证据创意与探索预算模拟"), false);
    assert.equal((await repository.listAuditEvents(userProjectId)).some((event) => event.action === "demo.seed_verified"), false);
    assert.equal((await repository.listOperationalRecords(demo.id)).some((record) => record.id === "WORK-2607-01"), true);
    assert.equal((await repository.listArtifacts(demo.id)).some((artifact) => artifact.content.includes("±0.01mm 精度")), true);
  } finally {
    await temporary.dispose();
  }
});
