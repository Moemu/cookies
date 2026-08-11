import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { api } from "../src/data/api.ts";

const page = readFileSync(
  new URL("../src/components/MiyunMaterialsPage.tsx", import.meta.url),
  "utf8",
);

test("裂变任务只允许 confirmed 且 imported/deduplicated 的素材和 confirmed profile", () => {
  assert.match(page, /material\.selection_status === "confirmed"/);
  assert.match(page, /material\.import_status === "imported"/);
  assert.match(page, /material\.import_status === "deduplicated"/);
  assert.match(page, /profile\.status === "confirmed"/);
  assert.match(page, /source_material_ids: selectedHandoffMaterials\.map\(\(material\) => material\.id\)/);
  assert.match(page, /type="checkbox"/);
  assert.match(page, /selectedMaterialIds\.includes\(material\.id\)/);
  assert.match(page, /product_profile_id: selectedHandoffProfile!\.id/);
  assert.match(page, /product_asset_refs\.length/);
  assert.match(page, /knowledge_document_ids\.length/);
  assert.match(page, /product_asset_refs: selectedProfile\.product_asset_refs \?\? \[\]/);
  assert.match(page, /knowledge_document_ids: selectedProfile\.knowledge_document_ids \?\? \[\]/);
});

test("裂变任务使用原生下载并且只允许 exported 的人工交付确认", () => {
  assert.match(page, /getMiyunHandoffExportUrl/);
  assert.match(page, /handoff\.status === "exporting" \|\| handoff\.status === "exported" \|\| handoff\.status === "delivered"/);
  assert.match(page, /导出并下载交接 zip/);
  assert.match(page, /handoff\.status === "exported"/);
  assert.match(page, /markMiyunHandoffDelivered\(currentProject\.id, handoff\.id, handoff\.version\)/);
  assert.match(page, /已导出.*已交付/);
  assert.match(page, /成功后才可人工确认交付/);
});

test("handoff API 为同源交接端点，创建携带幂等键、交付携带 expected_version", async () => {
  const original = globalThis.fetch;
  const calls: Array<{ input: RequestInfo | URL; init?: RequestInit }> = [];
  globalThis.fetch = async (input, init) => {
    calls.push({ input, init });
    return new Response("{}", { status: 200, headers: { "content-type": "application/json" } });
  };
  try {
    await api.createMiyunHandoff("p", { source_material_ids: ["m", "m2"], product_profile_id: "profile" }, "key");
    await api.markMiyunHandoffDelivered("p", "handoff", 7);
  } finally {
    globalThis.fetch = original;
  }
  assert.equal(calls.length, 2);
  assert.equal(new Headers(calls[0].init?.headers).get("Idempotency-Key"), "key");
  assert.match(api.getMiyunHandoffExportUrl("p", "handoff"), /^\/api\//);
});
