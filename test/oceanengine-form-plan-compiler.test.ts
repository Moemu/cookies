import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import Ajv2020 from "ajv/dist/2020.js";

import { type EcommerceParentConditionManifest } from "../scripts/oceanengine-ecommerce-field-compiler.ts";
import {
  compileOceanEngineFormPlan,
  type OceanEnginePlanCompilerInput,
  type OceanEnginePlanKind,
} from "../scripts/oceanengine-form-plan-compiler.ts";

const root = resolve(import.meta.dirname, "..");
const readJSON = (relative: string) => JSON.parse(readFileSync(resolve(root, relative), "utf8"));
const manifest = readJSON("docs/delivery/fixtures/oceanengine-ecommerce-parent-condition-manifest-v1.json") as EcommerceParentConditionManifest;
const schema = readJSON("docs/delivery/schemas/oceanengine-playwright-rpa-plan-v3.json");
const authoritySchema = readJSON("docs/delivery/schemas/oceanengine-execution-authority-v1.json");
const ajv = new Ajv2020({ allErrors: true, strict: false });
ajv.addSchema(authoritySchema);
const validate = ajv.compile(schema);

const fixtures: Array<[OceanEnginePlanKind, string]> = [
  ["project_create", "docs/delivery/fixtures/oceanengine-project-create-plan-input-v1.json"],
  ["project_edit", "docs/delivery/fixtures/oceanengine-project-edit-plan-input-v1.json"],
  ["promotion_create", "docs/delivery/fixtures/oceanengine-promotion-create-plan-input-v1.json"],
  ["promotion_edit", "docs/delivery/fixtures/oceanengine-promotion-edit-plan-input-v1.json"],
];

test("all four form plan compilers produce schema-valid prepare plans", () => {
  for (const [kind, fixture] of fixtures) {
    const input = readJSON(fixture) as OceanEnginePlanCompilerInput;
    const plan = compileOceanEngineFormPlan(kind, manifest, input);
    assert.equal(validate(plan), true, `${kind}: ${JSON.stringify(validate.errors)}`);
    assert.equal(plan.status, "ready", `${kind}: ${plan.blocked_reasons.join(",")}`);
    assert.equal(plan.allow_remote_write, false);
    assert.equal(plan.maximum_final_clicks, 0);
    const boundary = plan.steps.at(-1);
    assert.equal(boundary?.kind, "final_click_boundary");
    assert.equal(boundary?.remote_write, true);
    assert.equal(boundary?.blocked, true);
    assert.equal(boundary?.block_reason, "PREPARE_PLAN_REMOTE_WRITE_PROHIBITED");
  }
});

test("project product picker target follows create and edit page labels", () => {
  const createInput = readJSON("docs/delivery/fixtures/oceanengine-project-create-plan-input-v1.json") as OceanEnginePlanCompilerInput;
  const editInput = readJSON("docs/delivery/fixtures/oceanengine-project-edit-plan-input-v1.json") as OceanEnginePlanCompilerInput;
  assert.equal(
    compileOceanEngineFormPlan("project_create", manifest, createInput).steps.find((step) => step.field_key === "project.marketing_product_reference")?.target,
    "点击选择商品",
  );
  assert.equal(
    compileOceanEngineFormPlan("project_edit", manifest, editInput).steps.find((step) => step.field_key === "project.marketing_product_reference")?.target,
    "更换",
  );
});

test("promotion plans consume parent-dependent promotion fields", () => {
  const input = readJSON("docs/delivery/fixtures/oceanengine-promotion-edit-plan-input-v1.json") as OceanEnginePlanCompilerInput;
  const plan = compileOceanEngineFormPlan("promotion_edit", manifest, input);
  const fields = plan.steps.flatMap((step) => step.field_key ? [step.field_key] : []);
  assert.ok(fields.includes("promotion.copy_materials"));
  assert.ok(fields.includes("promotion.direct_link_backup_reference"));
  assert.ok(fields.includes("promotion.roi_coefficient"));
  assert.ok(!fields.includes("promotion.bid"));
  assert.equal(
    plan.steps.find((step) => step.field_key === "promotion.landing_page_reference")?.target,
    "请选择或填写自研落地页链接",
  );
});

test("edit and promotion plans require stable object bindings", () => {
  const base = readJSON("docs/delivery/fixtures/oceanengine-project-create-plan-input-v1.json") as OceanEnginePlanCompilerInput;
  assert.ok(compileOceanEngineFormPlan("project_edit", manifest, base).blocked_reasons.includes("missing_object_reference"));
  assert.ok(compileOceanEngineFormPlan("promotion_create", manifest, base).blocked_reasons.includes("missing_parent_project_reference"));
  assert.ok(compileOceanEngineFormPlan("promotion_edit", manifest, base).blocked_reasons.includes("missing_object_reference"));
});

test("missing required values block a plan before browser execution", () => {
  const input = readJSON("docs/delivery/fixtures/oceanengine-promotion-create-plan-input-v1.json") as OceanEnginePlanCompilerInput;
  delete input.values["promotion.delivery_identity"];
  const plan = compileOceanEngineFormPlan("promotion_create", manifest, input);
  assert.equal(plan.status, "blocked");
  assert.ok(plan.blocked_reasons.includes("missing_required_value:promotion.delivery_identity"));
});
