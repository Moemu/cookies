import assert from "node:assert/strict";
import { once } from "node:events";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { createApp } from "../index.js";
import { FileRepository } from "../repository.js";

async function createTemporaryRepository(): Promise<{
  filePath: string;
  dispose: () => Promise<void>;
}> {
  const directory = await mkdtemp(join(tmpdir(), "cookies-task2-"));
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

const featurePayload = {
  schemaVersion: "asset_feature_v1" as const,
  featureVersion: "vlm-2026-07-26",
  hookStrength: 0.86,
  productVisibility: 0.74,
  sceneTags: ["factory", "macro"],
  productTags: ["cnc"],
  personTags: ["engineer"],
  actionTags: ["cutting"],
  emotionTags: ["trust"],
  sellingPoints: ["±0.01mm precision", "98% on-time delivery"],
  ctaPresence: true,
  similarityGroup: "precision-demo-a",
  similarityRisk: "medium" as const,
  evidence: ["00:00-00:03 shows macro hook", "00:04 product logo visible"],
};

test("AssetFeature upsert 按组织、项目、素材版本和 feature version 持久化隔离", async () => {
  const temporary = await createTemporaryRepository();
  try {
    const repository = await FileRepository.open(temporary.filePath);
    const project = await repository.createProject({
      name: "Task2 项目",
      brand: "Cookies",
      objective: "验证素材特征",
    });
    const otherProject = await repository.createProject({
      name: "隔离项目",
      brand: "Other",
      objective: "验证权限隔离",
    });
    const asset = await repository.createArtifact({
      projectId: project.id,
      kind: "video",
      content: "https://assets.test/video.mp4",
      status: "ready",
    });

    const created = await repository.upsertAssetFeature({
      organizationId: "org-a",
      projectId: project.id,
      assetId: asset.id,
      assetVersion: asset.version,
      ...featurePayload,
    });
    assert.equal(created.version, 1);
    assert.equal(created.hookStrength, 0.86);
    assert.deepEqual(created.sellingPoints, ["±0.01mm precision", "98% on-time delivery"]);

    const updated = await repository.upsertAssetFeature({
      organizationId: "org-a",
      projectId: project.id,
      assetId: asset.id,
      assetVersion: asset.version,
      ...featurePayload,
      hookStrength: 0.91,
      evidence: ["updated evidence"],
    });
    assert.equal(updated.id, created.id);
    assert.equal(updated.version, 2);
    assert.equal(updated.hookStrength, 0.91);

    const recovered = await FileRepository.open(temporary.filePath);
    const projectFeatures = await recovered.listAssetFeatures({
      organizationId: "org-a",
      projectId: project.id,
    });
    assert.equal(projectFeatures.length, 1);
    assert.equal(projectFeatures[0]?.assetId, asset.id);
    assert.equal(projectFeatures[0]?.featureVersion, featurePayload.featureVersion);
    await assert.rejects(
      () => recovered.getAssetFeature({
        organizationId: "org-a",
        projectId: otherProject.id,
        assetId: asset.id,
        assetVersion: asset.version,
        featureVersion: featurePayload.featureVersion,
      }),
      { code: "NOT_FOUND" },
    );
  } finally {
    await temporary.dispose();
  }
});

test("AssetFeature HTTP API 读取当前 Project 特征并对缺失特征安全降级", async () => {
  const app = await startApi();
  try {
    const project = await app.request("POST", "/api/projects", {
      name: "HTTP 特征项目",
      brand: "Cookies",
      objective: "验证 HTTP API",
    });
    const otherProject = await app.request("POST", "/api/projects", {
      name: "越权项目",
      brand: "Other",
      objective: "验证隔离",
    });
    const asset = await app.request("POST", "/api/artifacts", {
      projectId: project.body.id,
      kind: "video",
      content: "https://assets.test/video.mp4",
      status: "ready",
    });

    const missing = await app.request(
      "GET",
      `/api/asset-features?organizationId=org-a&projectId=${project.body.id}&assetId=${asset.body.id}&assetVersion=2&featureVersion=${featurePayload.featureVersion}`,
    );
    assert.deepEqual(missing, { status: 200, body: { feature: null } });

    const created = await app.request("PUT", "/api/asset-features", {
      organizationId: "org-a",
      projectId: project.body.id,
      assetId: asset.body.id,
      assetVersion: asset.body.version,
      ...featurePayload,
    });
    assert.equal(created.status, 200);
    assert.equal(created.body.schemaVersion, "asset_feature_v1");
    assert.equal(created.body.productVisibility, 0.74);

    const list = await app.request("GET", `/api/asset-features?organizationId=org-a&projectId=${project.body.id}`);
    assert.equal(list.status, 200);
    assert.equal(list.body.items.length, 1);
    assert.equal(list.body.items[0].assetId, asset.body.id);

    const exact = await app.request(
      "GET",
      `/api/asset-features?organizationId=org-a&projectId=${project.body.id}&assetId=${asset.body.id}&assetVersion=1&featureVersion=${featurePayload.featureVersion}`,
    );
    assert.equal(exact.status, 200);
    assert.equal(exact.body.feature.similarityRisk, "medium");

    const forbidden = await app.request("PUT", "/api/asset-features", {
      organizationId: "org-a",
      projectId: otherProject.body.id,
      assetId: asset.body.id,
      assetVersion: asset.body.version,
      ...featurePayload,
    });
    assert.equal(forbidden.status, 404);
    assert.equal(forbidden.body.error.code, "NOT_FOUND");

    const invalid = await app.request("PUT", "/api/asset-features", {
      organizationId: "org-a",
      projectId: project.body.id,
      assetId: asset.body.id,
      assetVersion: asset.body.version,
      ...featurePayload,
      hookStrength: 1.2,
    });
    assert.equal(invalid.status, 400);
    assert.equal(invalid.body.error.code, "VALIDATION_ERROR");
    assert.equal(invalid.body.error.details[0].field, "hookStrength");
  } finally {
    await app.dispose();
  }
});
