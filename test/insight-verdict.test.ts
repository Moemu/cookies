import assert from "node:assert/strict";
import test from "node:test";
import {
  upgradeLabel,
  verdictIcon,
  verdictOfConfidence,
  weakestVerdict,
  type Verdict,
} from "../src/data/verdict.ts";

// 前端的收敛表必须和后端 internal/systems/insights/verdict.go 一模一样。
// 两边各写一套是这个模块以前最典型的毛病：同一份数据两页显示不同档位。
test("四档收敛成三档，规则与后端一致", () => {
  assert.equal(verdictOfConfidence("sufficient"), "explained");
  assert.equal(verdictOfConfidence("directional"), "observed");
  assert.equal(verdictOfConfidence("confounded"), "observed");
  assert.equal(verdictOfConfidence("low_sample"), "unclear");
});

test("三档的图标是固定的", () => {
  assert.equal(verdictIcon.explained, "✅");
  assert.equal(verdictIcon.observed, "👁");
  assert.equal(verdictIcon.unclear, "❓");
});

test("屏级档位取最弱的一条", () => {
  const items = (verdicts: Verdict[]) => verdicts.map((verdict) => ({ verdict }));
  assert.equal(weakestVerdict(items(["explained", "explained"])), "explained");
  assert.equal(weakestVerdict(items(["explained", "observed"])), "observed");
  assert.equal(weakestVerdict(items(["explained", "unclear", "observed"])), "unclear");
  // 一条都没有是「算不出来」，不是「能归因」——空屏不该比有数据的屏更可信。
  assert.equal(weakestVerdict([]), "unclear");
});

test("升级通道的文案", () => {
  assert.equal(upgradeLabel("experiment"), "做个实验");
  assert.equal(upgradeLabel("similar_assets"), "找相似素材");
  assert.equal(upgradeLabel(undefined), "");
});
