import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import Ajv2020 from "ajv/dist/2020.js";

import {
  compileEcommerceParentCondition,
  type EcommerceParentConditionManifest,
  type EcommerceParentContext,
} from "../scripts/oceanengine-ecommerce-field-compiler.ts";

const root = resolve(import.meta.dirname, "..");
const manifest = JSON.parse(
  readFileSync(resolve(root, "docs/delivery/fixtures/oceanengine-ecommerce-parent-condition-manifest-v1.json"), "utf8"),
) as EcommerceParentConditionManifest;

const schema = JSON.parse(
  readFileSync(resolve(root, "docs/delivery/schemas/oceanengine-ecommerce-parent-condition-manifest-v1.json"), "utf8"),
);

const base: EcommerceParentContext = {
  carrier: "orange_landing_page",
  optimization_target: "in_app_order",
  deep_optimization: "disabled",
  delivery_mode: "manual",
  placement_mode: "automatic",
};

const fieldKeys = (context: EcommerceParentContext) =>
  compileEcommerceParentCondition(manifest, context).promotion.fields.map((field) => field.key);

test("parent-condition manifest keeps remote writes disabled", () => {
  const validate = new Ajv2020({ allErrors: true, strict: false }).compile(schema);
  assert.equal(validate(manifest), true, JSON.stringify(validate.errors));
  assert.equal(manifest.remote_write_authorized, false);
  assert.equal(compileEcommerceParentCondition(manifest, base).remote_write_authorized, false);
});

test("all field deltas resolve to declared controls", () => {
  const referenced = new Set(manifest.base_promotion_fields);
  for (const rules of [
    manifest.delivery_rules,
    manifest.optimization_rules,
    manifest.deep_optimization_rules,
    manifest.placement_rules,
  ]) {
    for (const rule of Object.values(rules)) {
      for (const key of [...(rule.add_fields ?? []), ...(rule.remove_fields ?? [])]) referenced.add(key);
    }
  }
  for (const key of referenced) assert.ok(manifest.field_definitions[key], key);

  const projectReferenced = new Set(manifest.base_project_fields);
  for (const rules of [manifest.delivery_rules, manifest.optimization_rules, manifest.deep_optimization_rules, manifest.placement_rules]) {
    for (const rule of Object.values(rules)) {
      for (const key of [...(rule.project_add_fields ?? []), ...(rule.project_remove_fields ?? [])]) projectReferenced.add(key);
    }
  }
  for (const key of projectReferenced) assert.ok(manifest.project_field_definitions[key], key);
});

test("manual and UBMax compile different promotion limits and bid fields", () => {
  const manual = compileEcommerceParentCondition(manifest, base);
  const ubmax = compileEcommerceParentCondition(manifest, { ...base, delivery_mode: "ubmax" });
  assert.deepEqual(manual.promotion.material_limits, { video: 10, image: 10, image_text: 10, copy: 10 });
  assert.deepEqual(ubmax.promotion.material_limits, { video: 30, image: 50, image_text: 10, copy: 10 });
  assert.ok(fieldKeys(base).includes("promotion.daily_budget"));
  assert.ok(fieldKeys(base).includes("promotion.bid"));
  assert.ok(!fieldKeys({ ...base, delivery_mode: "ubmax" }).includes("promotion.bid"));
});

test("conversion ROI replaces the manual promotion bid", () => {
  const keys = fieldKeys({ ...base, deep_optimization: "conversion_roi" });
  assert.ok(keys.includes("promotion.daily_budget"));
  assert.ok(keys.includes("promotion.roi_coefficient"));
  assert.ok(!keys.includes("promotion.bid"));
});

test("project fields follow delivery, search, and net ROI conditions", () => {
  const manualFields = compileEcommerceParentCondition(manifest, base).project.fields;
  const manual = manualFields.map((field) => field.key);
  assert.ok(manual.includes("project.placement_strategy"));
  assert.ok(manual.includes("project.search_bid_coefficient"));
  assert.ok(!manual.includes("project.bid"));
  assert.equal(manualFields.find((field) => field.key === "project.delivery_mode")?.target, "手动投放");
  assert.equal(manualFields.find((field) => field.key === "project.placement_strategy")?.target, "通投智选");

  const click = compileEcommerceParentCondition(manifest, { ...base, optimization_target: "click" }).project.fields.map((field) => field.key);
  assert.ok(!click.includes("project.deep_optimization_mode"));
  assert.ok(!click.includes("project.search_bid_coefficient"));

  const netRoi = compileEcommerceParentCondition(manifest, { ...base, delivery_mode: "ubmax", deep_optimization: "net_roi" }).project.fields.map((field) => field.key);
  assert.ok(netRoi.includes("project.roi_coefficient"));
  assert.ok(!netRoi.includes("project.bid"));
});

test("project option targets follow the selected parent context", () => {
  const owned = compileEcommerceParentCondition(manifest, { ...base, carrier: "owned_landing_page" });
  assert.equal(owned.project.fields.find((field) => field.key === "project.carrier")?.target, "自研落地页");
  const roi = compileEcommerceParentCondition(manifest, { ...base, deep_optimization: "conversion_roi" });
  assert.equal(roi.project.fields.find((field) => field.key === "project.deep_optimization_mode")?.target, "成交ROI");
  const preferred = compileEcommerceParentCondition(manifest, { ...base, placement_mode: "preferred_media" });
  assert.equal(preferred.project.fields.find((field) => field.key === "project.placement_strategy")?.target, "首选媒体");
});

test("click and impression remove the native anchor and disable search expansion", () => {
  for (const optimization_target of ["click", "impression"]) {
    const context = { ...base, optimization_target };
    assert.ok(!fieldKeys(context).includes("promotion.native_anchor_reference"));
    const blocked = compileEcommerceParentCondition(manifest, { ...context, search_targeting_expansion: true });
    assert.equal(blocked.status, "blocked");
    assert.ok(blocked.blocked_reasons.includes("search_targeting_expansion_unavailable"));
  }
});

test("preferred media adds the Pangle coefficient", () => {
  const compiled = compileEcommerceParentCondition(manifest, { ...base, placement_mode: "preferred_media" });
  assert.ok(compiled.promotion.fields.some((field) => field.key === "promotion.pangle_bid_coefficient"));
  assert.deepEqual(compiled.project.available_media, ["今日头条", "西瓜视频", "抖音", "番茄系媒体", "穿山甲"]);
});

test("owned landing page changes the promotion target", () => {
  const compiled = compileEcommerceParentCondition(manifest, { ...base, carrier: "owned_landing_page" });
  assert.equal(
    compiled.promotion.fields.find((field) => field.key === "promotion.landing_page_reference")?.target,
    "请选择或填写自研落地页链接",
  );
});

test("miniapp carriers require an existing parent reference", () => {
  const blocked = compileEcommerceParentCondition(manifest, { ...base, carrier: "byte_miniapp" });
  assert.equal(blocked.status, "blocked");
  assert.ok(blocked.blocked_reasons.includes("missing_parent_reference:byte_miniapp_reference"));
  const ready = compileEcommerceParentCondition(manifest, {
    ...base,
    carrier: "byte_miniapp",
    parent_references: { byte_miniapp_reference: "managed-object-id" },
  });
  assert.equal(ready.status, "ready");
});

test("net order and net ROI reject manual delivery", () => {
  for (const deep_optimization of ["net_order", "net_roi"]) {
    const compiled = compileEcommerceParentCondition(manifest, { ...base, deep_optimization });
    assert.equal(compiled.status, "blocked");
    assert.ok(compiled.blocked_reasons.includes("deep_optimization_unsupported_delivery_mode:manual"));
  }
});

test("newly observed fields always compile from the base form", () => {
  const keys = fieldKeys(base);
  assert.ok(keys.includes("promotion.copy_materials"));
  assert.ok(keys.includes("promotion.direct_link_backup_reference"));
});
