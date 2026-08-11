import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { api } from "../src/data/api.ts";

const page = readFileSync(
  new URL("../src/components/MiyunMaterialsPage.tsx", import.meta.url),
  "utf8",
);

test("裂变任务只允许 confirmed 且 imported/deduplicated 的素材和 confirmed profile", () => {
  assert.match(page, /handoffEligible: view === "fission"/);
  assert.match(page, /const handoffMaterials = materials/);
  assert.match(page, /selectedCrawlProfile\?\.status === "confirmed"/);
  assert.match(page, /source_material_ids: handoffMaterialIds/);
  assert.match(page, /type="checkbox"/);
  assert.match(page, /selectedMaterialIds\.includes\(material\.id\)/);
  assert.match(page, /全选本页/);
  assert.match(page, /onSelectPage\(pageMaterialIds\)/);
  assert.match(page, /清空已选/);
  assert.match(page, /product_profile_id: selectedHandoffProfile!\.id/);
  assert.match(page, /crawl_job_id: selectedCrawlJob!\.id/);
  assert.match(page, /product_asset_refs\.length/);
  assert.match(page, /knowledge_document_ids\.length/);
  assert.match(page, /product_asset_refs: profile\.product_asset_refs \?\? \[\]/);
  assert.match(page, /knowledge_document_ids: profile\.knowledge_document_ids \?\? \[\]/);
});

test("裂变任务使用原生下载并且只允许 exported 的人工交付确认", () => {
  assert.match(page, /getMiyunHandoffExportUrl/);
  assert.match(page, /handoff\.status === "exporting" \|\| handoff\.status === "exported" \|\| handoff\.status === "delivered"/);
  assert.match(page, /下载裂变源素材 ZIP/);
  assert.match(page, /下载项目资料 ZIP/);
  assert.match(page, /"sources"/);
  assert.match(page, /"project"/);
  assert.match(page, /handoff\.status === "exported"/);
  assert.match(page, /markMiyunHandoffDelivered\(currentProject\.id, handoff\.id, handoff\.version\)/);
  assert.match(page, /两个压缩包均需上传至 AI 系统/);
  assert.match(page, /两个文件都已下载/);
});

test("handoff API 为同源交接端点，创建携带幂等键、交付携带 expected_version", async () => {
  const original = globalThis.fetch;
  const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = [];
  globalThis.fetch = async (input, init) => {
    calls.push({ input, init });
    return new Response("{}", { status: 200, headers: { "content-type": "application/json" } });
  };
  try {
    await api.createMiyunHandoff("p", { source_material_ids: ["m", "m2"], product_profile_id: "profile", crawl_job_id: "job" }, "key");
    await api.markMiyunHandoffDelivered("p", "handoff", 7);
  } finally {
    globalThis.fetch = original;
  }
  assert.equal(calls.length, 2);
  assert.equal(new Headers(calls[0].init?.headers).get("Idempotency-Key"), "key");
  assert.match(api.getMiyunHandoffExportUrl("p", "handoff", "sources"), /\/export\?package=sources$/);
  assert.match(api.getMiyunHandoffExportUrl("p", "handoff", "project"), /\/export\?package=project$/);
});

test("人工回传允许持续补充 MP4/ZIP，并展示进度、安全重试和关联约定", () => {
  assert.match(page, /handoff\.status === "exported" \|\| handoff\.status === "delivered" \|\| handoff\.status === "returned"/);
  assert.match(page, /accept="video\/mp4,application\/zip,\.mp4,\.zip"/);
  assert.match(page, /miyunReturnFile/);
  assert.match(page, /request\.upload\.onprogress/);
  assert.match(page, /<progress max="100" value=\{progress\}/);
  assert.match(page, /重试回传/);
  assert.match(page, /handoff\.status === "returned"/);
  assert.match(page, /无元信息时默认关联当前采集任务/);
  assert.match(page, /血缘：本 Project/);
  assert.match(page, /expected_version/);
  assert.match(page, /\/returns:import/);
  assert.doesNotMatch(page, /handoffs\/\$\{encodeURIComponent\(handoffId\)\}:return/);
});

test("回传错误不会渲染服务端响应或存储路径", () => {
  assert.doesNotMatch(page, /request\.responseText/);
  assert.match(page, /回传未完成，请检查 MP4 \/ ZIP 格式、文件大小和素材映射后重试/);
});
