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
    assert.equal(project.runtime.stage, "投放审批");
    assert.equal(project.runtime.progress, 82);
    assert.equal(project.runtime.budget, 1_000_000);
    assert.ok(firstRecords.length >= 20);
    assert.equal(firstWorkItem?.fields.owner, "Amelia Meng");
    assert.equal(metric?.fields.summary, "证据前置版本，正在形成稳定增量。");
    assert.equal(metric?.fields.recommendation, "下一轮将证据前置版本扩大到 30% 素材覆盖，并保留纯产品特写作为对照组。");
    assert.equal(metric?.fields.points, "32,39,36,49,54,51,63,67,72,78,75,86");

    await seedDemoProject(repository);
    const secondRecords = await repository.listOperationalRecords(project.id);
    const secondWorkItem = secondRecords.find((record) => record.id === "WORK-2607-01");
    assert.equal(secondRecords.length, firstRecords.length);
    assert.equal(secondWorkItem?.createdAt, firstWorkItem?.createdAt);
    assert.equal(secondWorkItem?.updatedAt, firstWorkItem?.updatedAt);

    const reopened = await FileRepository.open(temporary.filePath);
    assert.equal((await reopened.getProject(project.id))?.runtime.owner, "Noah Xu");
    assert.equal((await reopened.listOperationalRecords(project.id)).length, firstRecords.length);
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
    assert.equal(demoRecords.body[0].id, "ACTION-P0");
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
