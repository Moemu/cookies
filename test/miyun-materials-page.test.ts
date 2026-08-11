import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import {
  latestMiyunCard,
  miyunCrawlErrorCopy,
  miyunStateCopy,
  profileQuery,
  sortMiyunMaterials,
} from "../src/components/MiyunMaterialsPage.tsx";
import { api } from "../src/data/api.ts";

test("密云状态明确区分授权、冷却、部分完成和禁止访问", () => {
  assert.match(miyunStateCopy("auth_required").action, /更新会话/);
  assert.match(miyunStateCopy("cooling_down").action, /冷却/);
  assert.match(miyunStateCopy("partial").action, /重试/);
  assert.match(miyunStateCopy("forbidden").title, /没有访问/);
});
test("密云 API 使用位置 expected_version 且候选预览为本域代理", async () => {
  const original = globalThis.fetch;
  const calls: Request[] = [];
  globalThis.fetch = async (input) => {
    calls.push(input as Request);
    return new Response("{}", {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  };
  try {
    await api.verifyMiyunConnection("p", 3);
    await api.confirmMiyunProductProfile("p", "profile", 4, {
      product_name: "x",
      keywords: ["x"],
      material_content_types: ["video"],
      window_start: "2026-01-01",
      window_end: "2026-01-02",
    });
    await api.cancelMiyunCrawlJob("p", "job", 5);
    await api.confirmMiyunMaterial("p", "material", 6, "ok");
    await api.retryMiyunMaterialImport("p", "material", 7);
  } finally {
    globalThis.fetch = original;
  }
  assert.equal(calls.length, 5);
  assert.match(api.getMiyunMaterialPreviewUrl("p", "m"), /^\/api\//);
});
test("详情快照提供真实数据卡，未知达人排序置末", () => {
  const base = {
    organization_id: "o",
    project_id: "p",
    import_method: "crawler" as const,
    source_ref_status: "verified" as const,
    selection_status: "discovered" as const,
    import_status: "pending" as const,
    version: 1,
    created_by: "u",
    created_at: "",
    updated_at: "",
  };
  const rows = [
    {
      ...base,
      id: "unknown",
      miyun_material_id: "1",
      snapshots: [{ related_creators_known: false }],
    },
    {
      ...base,
      id: "known",
      miyun_material_id: "2",
      snapshots: [{ related_creators_known: true, related_creators: 2 }],
    },
  ] as any;
  assert.deepEqual(
    sortMiyunMaterials(rows, "related_creators").map((row) => row.id),
    ["known", "unknown"],
  );
  assert.equal(latestMiyunCard(rows[0])?.related_creators_known, false);
});

test("页面为加载、409 刷新与空列表提供可见恢复动作", () => {
  const page = readFileSync(
    new URL("../src/components/MiyunMaterialsPage.tsx", import.meta.url),
    "utf8",
  );
  assert.match(miyunStateCopy("loading").title, /读取/);
  assert.match(page, /notice === ["']数据已更新，请刷新。["']/);
  assert.match(page, /onClick=\{\(\) => void load\(\)\}>刷新/);
  assert.match(page, /miyunStateCopy\("empty", "materials"\)/);
});

test("米云产品身份可选且资料上传按知识文档和项目素材分流", () => {
  const page = readFileSync(
    new URL("../src/components/MiyunMaterialsPage.tsx", import.meta.url),
    "utf8",
  );
  assert.match(page, /getMiyunProductSource/);
  assert.match(page, /产品名称（可选）/);
  assert.match(page, /名称作为待确认身份/);
  assert.match(page, /pdf\|docx\|md/);
  assert.match(page, /uploadKnowledgeDocument/);
  assert.match(page, /uploadProjectAsset/);
  assert.match(page, /onDragOver/);
  assert.match(page, /从本地上传/);
  assert.match(page, /从已有项目素材中导入/);
  assert.match(page, /miyun-upload-preview-list/);
  assert.match(page, /miyun-upload-preview-remove/);
  assert.match(page, /assetPickerMode/);
  assert.match(page, /miyun-asset-picker/);
  assert.match(page, /asset\.contentUrl/);
  assert.doesNotMatch(page, />选择文件</);
  assert.match(page, /miyun-connection-banner/);
  assert.match(page, /href="\/settings"/);
  assert.doesNotMatch(page, /setSession/);
});

test("米云确认查询把后端 RFC3339 窗口转换为日期条件", () => {
  const query = profileQuery({
    product_name: "Cup", category_id: "cid_1", category_name: "Drinkware",
    keywords: ["cup"], material_content_types: ["product"],
    window_start: "2026-08-01T00:00:00Z", window_end: "2026-08-10T23:59:59+08:00",
  } as any);
  assert.equal(query.window_start, "2026-08-01");
  assert.equal(query.window_end, "2026-08-10");
});

test("米云采集失败仅展示安全分类和代码", () => {
  assert.match(
    miyunCrawlErrorCopy({ last_error_kind: "graphql_error", last_error_code: "00:" }) ?? "",
    /00:/,
  );
  assert.match(
    miyunCrawlErrorCopy({ last_error_kind: "auth_required", last_error_code: "00:403005" }) ?? "",
    /更新会话/,
  );
});

test("已确认但尚未入库的素材可恢复请求导入", () => {
  const page = readFileSync(
    new URL("../src/components/MiyunMaterialsPage.tsx", import.meta.url),
    "utf8",
  );
  assert.match(page, /\["pending", "failed"\]\.includes\(material\.import_status\)/);
});
