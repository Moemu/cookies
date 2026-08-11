import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import {
  latestMiyunCard,
  miyunCrawlErrorCopy,
  miyunJobMaxPages,
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

test("米云工作流按活动状态自动同步，并在用户返回页面时刷新", () => {
  const page = readFileSync(
    new URL("../src/components/MiyunMaterialsPage.tsx", import.meta.url),
    "utf8",
  );
  assert.match(page, /activeJobStatuses/);
  assert.match(page, /activeMaterialStatuses/);
  assert.match(page, /window\.setInterval\(\(\) => void refresh\(\), 5000\)/);
  assert.match(page, /document\.addEventListener\("visibilitychange", refreshOnReturn\)/);
  assert.match(page, /window\.addEventListener\("focus", refreshOnReturn\)/);
  assert.match(page, /已同步至/);
});

test("米云视图使用独立说明、结构化状态与逐素材备注", () => {
  const page = readFileSync(
    new URL("../src/components/MiyunMaterialsPage.tsx", import.meta.url),
    "utf8",
  );
  assert.match(page, /const viewCopy/);
  assert.match(page, /miyun-material-card-heading/);
  assert.match(page, /miyun-metric-grid/);
  assert.match(page, /notes\[material\.id\]/);
  assert.match(page, /miyun-profile-card/);
  assert.match(page, /miyun-job-row/);
});

test("素材候选分页并以受控并发自动加载当前页预览", () => {
  const page = readFileSync(
    new URL("../src/components/MiyunMaterialsPage.tsx", import.meta.url),
    "utf8",
  );
  assert.match(page, /const materialPageSize = 8/);
  assert.match(page, /const previewConcurrency = 2/);
  assert.match(page, /paginatedMaterials\.map/);
  assert.match(page, /aria-label="素材候选分页"/);
  assert.match(page, /offset: \(materialPage - 1\) \* materialPageSize/);
  assert.match(page, /Math\.ceil\(materialTotal \/ materialPageSize\)/);
  assert.match(page, /remainingPreviewIds\.slice\(0, previewConcurrency\)/);
  assert.match(page, /previewBatchKey = `\$\{view\}:/);
  assert.match(page, /activePreviewIds\.includes\(material\.id\)/);
  assert.match(page, /等待自动加载预览/);
  assert.match(page, /页面同时只请求 \{previewConcurrency\} 条/);
  assert.match(page, /onPreviewSettled\(\)/);
});

test("裂变交接分页只统计服务端判定为可交接的素材", () => {
  const page = readFileSync(
    new URL("../src/components/MiyunMaterialsPage.tsx", import.meta.url),
    "utf8",
  );
  assert.match(page, /handoffEligible: view === "fission"/);
  assert.match(page, /const handoffMaterials = materials/);
  assert.doesNotMatch(page, /const handoffMaterials = materials\.filter/);
});

test("素材、裂变和任务入口共享可恢复的采集任务上下文", () => {
  const page = readFileSync(
    new URL("../src/components/MiyunMaterialsPage.tsx", import.meta.url),
    "utf8",
  );
  assert.match(page, /crawl_job_id/);
  assert.match(page, /localStorage\.setItem\(crawlContextStorageKey/);
  assert.match(page, /snapshot\.crawl_job_id === crawlJobId/);
  assert.match(page, /查看该批次素材/);
  assert.match(page, /进入该批次裂变/);
});

test("确认画像后提供采集引导，并用关键词和素材类型区分画像与任务", () => {
  const page = readFileSync(
    new URL("../src/components/MiyunMaterialsPage.tsx", import.meta.url),
    "utf8",
  );
  assert.match(page, /下一步：创建采集任务/);
  assert.match(page, /profileKeywordPreview/);
  assert.match(page, /profileMaterialTypePreview/);
  assert.match(page, /miyun-selected-profile-preview/);
  assert.match(page, /miyun-job-profile-summary/);
});

test("创建采集任务限制为 1–50 页，取消动作覆盖冷却任务", () => {
  const page = readFileSync(
    new URL("../src/components/MiyunMaterialsPage.tsx", import.meta.url),
    "utf8",
  );
  assert.match(page, /name="miyun-max-pages"/);
  assert.match(page, /max="50"/);
  assert.match(page, /max_pages: parsedMaxPages/);
  assert.match(page, /onClick=\{\(\) => void cancelJob\(job\)\}/);
  assert.doesNotMatch(page, /disabled=\{busy \|\| job\.status === "cooling_down"\}/);
  assert.match(page, /取消后不会再请求下一页/);
  assert.equal(miyunJobMaxPages({ query_snapshot: { max_pages: 25 } } as any), 25);
  assert.equal(miyunJobMaxPages({ query_snapshot: {} } as any), null);
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
  assert.doesNotMatch(page, /api\.verifyMiyunConnection/);
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
