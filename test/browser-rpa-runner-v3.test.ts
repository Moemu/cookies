import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { resolve } from "node:path";
import test from "node:test";
import Ajv2020 from "ajv/dist/2020.js";

import {
  executePlan,
  executePreparePlan,
  type PageOperations,
  type ReconciliationResult,
  type SubmitObservation,
} from "../scripts/browser-rpa-runner-v3.ts";
import { authorizeSubmitPlan } from "../scripts/rpa-authority.ts";
import { type EcommerceParentConditionManifest } from "../scripts/oceanengine-ecommerce-field-compiler.ts";
import {
  compileOceanEngineFormPlan,
  type OceanEngineFormPlan,
  type OceanEnginePlanCompilerInput,
} from "../scripts/oceanengine-form-plan-compiler.ts";

const root = resolve(import.meta.dirname, "..");
const readJSON = (relative: string) => JSON.parse(readFileSync(resolve(root, relative), "utf8"));
const manifest = readJSON("docs/delivery/fixtures/oceanengine-ecommerce-parent-condition-manifest-v1.json") as EcommerceParentConditionManifest;
const resultSchema = readJSON("docs/delivery/schemas/oceanengine-playwright-rpa-result-v2.json");
const validateResult = new Ajv2020({ allErrors: true, strict: false }).compile(resultSchema);
const runnerSource = readFileSync(resolve(root, "scripts/browser-rpa-runner-v3.ts"), "utf8");

class FakePage implements PageOperations {
  readonly applied: string[] = [];
  identified = 0;
  finalClicks = 0;
  submitObservation: SubmitObservation = { outcome: "success" };
  reconciliation: ReconciliationResult = { status: "matched", created_object_id: "7677604041052405801" };

  async identifyPage() {
    this.identified += 1;
  }

  async applyField(step: OceanEngineFormPlan["steps"][number]) {
    if (step.field_key) this.applied.push(step.field_key);
  }

  async readField(step: OceanEngineFormPlan["steps"][number]) {
    return step.value;
  }

  async assertFinalReady() {}

  async clickFinal() {
    this.finalClicks += 1;
  }

  async observeSubmit() {
    return this.submitObservation;
  }

  async reconcileSubmit() {
    return this.reconciliation;
  }
}

function compilePromotionPlan() {
  const input = readJSON("docs/delivery/fixtures/oceanengine-promotion-create-plan-input-v1.json") as OceanEnginePlanCompilerInput;
  return compileOceanEngineFormPlan("promotion_create", manifest, input);
}

test("runner v3 executes prepare fields and stops at the final-click boundary", async () => {
  const plan = compilePromotionPlan();
  const page = new FakePage();
  const result = await executePreparePlan(plan, page);
  assert.equal(validateResult(result), true, JSON.stringify(validateResult.errors));
  assert.equal(result.outcome, "success");
  assert.equal(result.final_click_performed, false);
  assert.equal(page.identified, 1);
  assert.ok(page.applied.includes("promotion.copy_materials"));
  assert.equal(result.steps.at(-1)?.status, "blocked_boundary");
});

test("a blocked compiled plan does not access the page", async () => {
  const plan = compilePromotionPlan();
  plan.status = "blocked";
  plan.blocked_reasons = ["missing_parent_reference:test"];
  const page = new FakePage();
  const result = await executePreparePlan(plan, page);
  assert.equal(result.outcome, "blocked");
  assert.equal(page.identified, 0);
  assert.deepEqual(page.applied, []);
});

test("runner v3 rejects any plan that enables a final click", async () => {
  const plan = compilePromotionPlan();
  const unsafe = { ...plan, allow_remote_write: true } as unknown as OceanEngineFormPlan;
  const result = await executePreparePlan(unsafe, new FakePage());
  assert.equal(result.outcome, "failed");
  assert.equal(result.error_code, "write_blocked");
  assert.equal(result.final_click_performed, false);
});

test("runner v3 rejects a missing blocked boundary", async () => {
  const plan = compilePromotionPlan();
  plan.steps = plan.steps.slice(0, -1);
  const result = await executePreparePlan(plan, new FakePage());
  assert.equal(result.outcome, "failed");
  assert.equal(result.error_code, "invalid_plan");
});

test("runner v3 keeps complex field values in readback", async () => {
  const plan = compilePromotionPlan();
  const page = new FakePage();
  const result = await executePreparePlan(plan, page);
  const copyStep = result.steps.find((step) => step.id.includes("promotion.copy_materials"));
  assert.deepEqual(copyStep?.readback, ["校准文案"]);
});

test("runner v3 retains the live promotion adapters", () => {
  assert.match(runnerSource, /promotion\.product_image_references/);
  assert.match(runnerSource, /\.oc-create-product-img-add-button/);
  assert.match(runnerSource, /root\.locator\("img:visible"\)/);
  assert.match(runnerSource, /String\(step\.value\)\.split\("\/"\)/);
  assert.match(runnerSource, /自定义品牌名称/);
  assert.match(runnerSource, /await target\.press\("Enter"\)/);
  assert.match(runnerSource, /step\.operation === "toggle"/);
  assert.match(runnerSource, /\.isChecked\(\)/);
});

test("promotion reconciliation reads the full call-to-action multi-select", () => {
  assert.match(runnerSource, /promotion\.landing_page_reference/);
  assert.match(runnerSource, /promotion\.call_to_action/);
  assert.match(runnerSource, /selectedCallToActions/);
  assert.match(runnerSource, /call-to-action multi-select mutation is not calibrated/);
  assert.doesNotMatch(runnerSource, /const previewTitle = editPage\.getByText\("单元素材预览"/);
});

test("runner v3 consumes one authority and performs one final click", async () => {
  const preparePlan = compilePromotionPlan();
  preparePlan.account_reference = "1855554434276391";
  preparePlan.parent_project_reference = "7677595885572784182";
  const now = new Date("2026-08-25T01:00:00.000Z");
  const bundle = authorizeSubmitPlan(preparePlan, {
    account_reference: preparePlan.account_reference,
    maximum_money_cny: 300,
    schedule_date: "2026-08-26",
  }, now);
  const stateDirectory = await mkdtemp(join(tmpdir(), "cookies-runner-authority-"));
  try {
    const page = new FakePage();
    const result = await executePlan(bundle.plan, page, {
      confirmToken: bundle.confirm_token,
      authorityStateDirectory: stateDirectory,
      now,
    });
    assert.equal(validateResult(result), true, JSON.stringify(validateResult.errors));
    assert.equal(result.outcome, "success");
    assert.equal(result.final_click_performed, true);
    assert.equal(result.created_object_id, "7677604041052405801");
    assert.equal(result.reconciliation, "matched");
    assert.equal(page.finalClicks, 1);

    const retryPage = new FakePage();
    const retry = await executePlan(bundle.plan, retryPage, {
      confirmToken: bundle.confirm_token,
      authorityStateDirectory: stateDirectory,
      now,
    });
    assert.equal(retry.outcome, "failed");
    assert.equal(retry.error_code, "authority_already_used");
    assert.equal(retryPage.finalClicks, 0);
  } finally {
    await rm(stateDirectory, { recursive: true, force: true });
  }
});

test("runner v3 reports a successful create with persisted field drift", async () => {
  const preparePlan = compilePromotionPlan();
  preparePlan.account_reference = "1855554434276391";
  preparePlan.parent_project_reference = "7677595885572784182";
  const now = new Date("2026-08-25T01:00:00.000Z");
  const bundle = authorizeSubmitPlan(preparePlan, {
    account_reference: preparePlan.account_reference,
    maximum_money_cny: 300,
    schedule_date: "2026-08-26",
  }, now);
  const stateDirectory = await mkdtemp(join(tmpdir(), "cookies-runner-drift-"));
  try {
    const page = new FakePage();
    page.reconciliation = {
      status: "matched",
      created_object_id: "7677604041052405801",
      field_reconciliation: {
        status: "drifted",
        fields: [{
          field_key: "promotion.landing_page_reference",
          expected: "7545332540006350875",
          observed: "7667531743012438066",
          status: "drifted",
        }],
      },
    };
    const result = await executePlan(bundle.plan, page, {
      confirmToken: bundle.confirm_token,
      authorityStateDirectory: stateDirectory,
      now,
    });
    assert.equal(validateResult(result), true, JSON.stringify(validateResult.errors));
    assert.equal(result.outcome, "success_with_drift");
    assert.equal(result.error_code, "field_reconciliation_drift");
    assert.equal(result.field_reconciliation?.status, "drifted");
    assert.equal(page.finalClicks, 1);
  } finally {
    await rm(stateDirectory, { recursive: true, force: true });
  }
});

test("runner v3 does not click when the confirmation token is wrong", async () => {
  const preparePlan = compilePromotionPlan();
  preparePlan.account_reference = "1855554434276391";
  preparePlan.parent_project_reference = "7677595885572784182";
  const now = new Date("2026-08-25T01:00:00.000Z");
  const bundle = authorizeSubmitPlan(preparePlan, {
    account_reference: preparePlan.account_reference,
    maximum_money_cny: 300,
    schedule_date: "2026-08-26",
  }, now);
  const page = new FakePage();
  const result = await executePlan(bundle.plan, page, {
    confirmToken: "incorrect-token",
    authorityStateDirectory: join(tmpdir(), "cookies-authority-unused"),
    now,
  });
  assert.equal(result.outcome, "failed");
  assert.equal(result.error_code, "authority_token_mismatch");
  assert.equal(page.finalClicks, 0);
});

test("runner v3 reports result_unknown without a second click", async () => {
  const preparePlan = compilePromotionPlan();
  preparePlan.account_reference = "1855554434276391";
  preparePlan.parent_project_reference = "7677595885572784182";
  const now = new Date("2026-08-25T01:00:00.000Z");
  const bundle = authorizeSubmitPlan(preparePlan, {
    account_reference: preparePlan.account_reference,
    maximum_money_cny: 300,
    schedule_date: "2026-08-26",
  }, now);
  const stateDirectory = await mkdtemp(join(tmpdir(), "cookies-runner-unknown-"));
  try {
    const page = new FakePage();
    page.submitObservation = { outcome: "result_unknown", error_message: "no stable result" };
    const result = await executePlan(bundle.plan, page, {
      confirmToken: bundle.confirm_token,
      authorityStateDirectory: stateDirectory,
      now,
    });
    assert.equal(result.outcome, "result_unknown");
    assert.equal(result.final_click_performed, true);
    assert.equal(page.finalClicks, 1);
  } finally {
    await rm(stateDirectory, { recursive: true, force: true });
  }
});
