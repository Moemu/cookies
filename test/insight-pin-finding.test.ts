import assert from "node:assert/strict";
import test from "node:test";
import { api } from "../src/data/api.ts";
import { hasInadmissibleFeature } from "../src/components/insight/analysis/ComparisonView.tsx";
import { pinKey } from "../src/components/insight/analysis/usePinFinding.ts";

// 归因不可用的变量不能进复盘。页面上这条表现为按钮禁用，但真正的约束是这个
// 谓词——数据里只要混进一个模型推断出来的变量，整条对比就不该被记下来。
test("有一个变量归因不可用，整条对比就不能记一笔", () => {
  assert.equal(hasInadmissibleFeature([{ admissible: true }, { admissible: true }]), false);
  assert.equal(hasInadmissibleFeature([{ admissible: true }, { admissible: false }]), true);
  // 没有变量不等于有不可用的变量：那种情况归不了因是另一个原因（缺内容特征），
  // 由后端的 no_features 档负责说清楚，不该借这个谓词把按钮关掉。
  assert.equal(hasInadmissibleFeature([]), false);
  assert.equal(hasInadmissibleFeature(undefined), false);
});

// 前端的点亮键比后端的去重键多带 source_ref：后端问「这条结论说的哪个变量」，
// 同一个变量在复盘里只留一条；页面问「我按过哪一行」，同一个变量出现在两个素材
// 上就是两行，只该点亮人刚按的那一行。
test("同一个变量在不同素材上是两行，点亮键要分得开", () => {
  const a = pinKey({ dimension: "comparisons", variable: "hook_type", source_ref: "asset_1" });
  const b = pinKey({ dimension: "comparisons", variable: "hook_type", source_ref: "asset_2" });
  assert.notEqual(a, b);
  assert.equal(a, pinKey({ dimension: "comparisons", variable: "hook_type", source_ref: "asset_1" }));
  // 总览没有主语，只有维度。分隔符用 NUL，变量名里出现空格也不会把两段粘错。
  assert.equal(pinKey({ dimension: "overview" }), "overview\x00\x00");
});

// 判定是后端算的，前端只报「哪一条」。带上 verdict 会被后端的
// DisallowUnknownFields 直接打回 400，所以这个请求体里必须干净。
test("记一笔只报坐标，不报判定", async () => {
  const originalFetch = globalThis.fetch;
  let captured: unknown;
  let capturedUrl = "";
  globalThis.fetch = async (input, init) => {
    capturedUrl = String(input);
    captured = JSON.parse(String(init?.body));
    return new Response(JSON.stringify({ id: "insightreport_1", digest: [] }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  };
  try {
    await api.pinFinding("project_1", {
      window: { start: "2026-07-12", end: "2026-08-11" },
      dimension: "fatigue",
      source_ref: "insightasset_1",
    });
  } finally {
    globalThis.fetch = originalFetch;
  }
  assert.match(capturedUrl, /\/api\/insights\/v1\/projects\/project_1\/findings$/);
  assert.deepEqual(captured, {
    window: { start: "2026-07-12", end: "2026-08-11" },
    dimension: "fatigue",
    source_ref: "insightasset_1",
  });
  assert.equal(Object.keys(captured as object).includes("verdict"), false);
});
