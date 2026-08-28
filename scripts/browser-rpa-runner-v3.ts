import { chromium, type Locator, type Page } from "@playwright/test";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";

import type { OceanEngineFormPlan } from "./oceanengine-form-plan-compiler.ts";
import { resolveSessionPlaywrightEndpoint } from "./browser-rpa-edge-session.ts";
import {
  AuthorityError,
  consumeSubmitAuthority,
  validateSubmitAuthority,
} from "./rpa-authority.ts";

type PlanStep = OceanEngineFormPlan["steps"][number];

export type ReferenceSelectionSpec = {
  selection_kind: "text_option" | "async_row" | "media_card" | "image_card";
  label?: string;
  object_id?: string;
  index?: number;
  minimum_visible?: number;
  expected_total?: number;
  confirm_button?: string;
};

export type RunnerStepResult = {
  id: string;
  status: "succeeded" | "blocked_boundary" | "submitted" | "result_unknown" | "failed";
  readback?: unknown;
  error_code?: string;
  error_message?: string;
  page_reference?: string;
};

export type RunnerV3Result = {
  schema_version: "oceanengine-playwright-rpa-result/v2";
  outcome: "success" | "success_with_drift" | "blocked" | "failed" | "result_unknown";
  error_code: string;
  error_message?: string;
  final_click_performed: boolean;
  created_object_id?: string;
  reconciliation?: "not_started" | "matched" | "not_found" | "not_applicable";
  field_reconciliation?: FieldReconciliation;
  steps: RunnerStepResult[];
};

export type FieldReconciliation = {
  status: "matched" | "drifted" | "not_checked";
  fields: Array<{
    field_key: string;
    expected?: string | string[];
    observed?: string | string[];
    status: "matched" | "drifted" | "not_checked";
  }>;
};

export type SubmitObservation = {
  outcome: "success" | "validation_error" | "result_unknown";
  error_message?: string;
  created_object_id?: string;
};

export type ReconciliationResult = {
  status: "matched" | "not_found" | "not_applicable";
  created_object_id?: string;
  field_reconciliation?: FieldReconciliation;
};

export interface PageOperations {
  identifyPage(plan: OceanEngineFormPlan): Promise<void>;
  applyField(step: PlanStep): Promise<void>;
  readField(step: PlanStep): Promise<unknown>;
  assertFinalReady(step: PlanStep): Promise<void>;
  clickFinal(step: PlanStep): Promise<void>;
  observeSubmit(plan: OceanEngineFormPlan): Promise<SubmitObservation>;
  reconcileSubmit(plan: OceanEngineFormPlan, observation: SubmitObservation): Promise<ReconciliationResult>;
}

class RunnerV3Error extends Error {
  constructor(readonly code: string, message: string) {
    super(message);
  }
}

function validatePreparePlan(plan: OceanEngineFormPlan) {
  if (plan.schema_version !== "oceanengine-playwright-rpa-plan/v3") {
    throw new RunnerV3Error("invalid_plan", "unsupported plan schema");
  }
  if (plan.browser !== "msedge" || plan.mode !== "prepare") {
    throw new RunnerV3Error("invalid_plan", "runner v3 accepts Edge prepare plans only");
  }
  if (plan.allow_remote_write || plan.maximum_final_clicks !== 0) {
    throw new RunnerV3Error("write_blocked", "prepare plan cannot authorize a final click");
  }
  const boundary = plan.steps.at(-1);
  if (
    boundary?.kind !== "final_click_boundary" ||
    !boundary.remote_write ||
    !boundary.blocked ||
    boundary.block_reason !== "PREPARE_PLAN_REMOTE_WRITE_PROHIBITED"
  ) {
    throw new RunnerV3Error("invalid_plan", "prepare plan has no blocked final-click boundary");
  }
}

function validateSubmitPlan(plan: OceanEngineFormPlan, confirmToken: string | undefined, now: Date) {
  if (plan.schema_version !== "oceanengine-playwright-rpa-plan/v3" || plan.browser !== "msedge") {
    throw new RunnerV3Error("invalid_plan", "runner v3 accepts Edge v3 plans only");
  }
  if (!confirmToken) throw new RunnerV3Error("authority_missing", "submit mode needs --confirm-token");
  try {
    return validateSubmitAuthority(plan, confirmToken, now);
  } catch (error) {
    if (error instanceof AuthorityError) throw new RunnerV3Error(error.code, error.message);
    throw error;
  }
}

function stableReferenceID(value: unknown) {
  if (typeof value === "string" && value.trim()) return value.trim();
  if (!value || typeof value !== "object") return undefined;
  const record = value as Record<string, unknown>;
  for (const key of ["object_id", "objectId", "id"]) {
    if (typeof record[key] === "string" && /^\d+$/.test(record[key])) return record[key] as string;
  }
  return undefined;
}

export type ExecutePlanOptions = {
  confirmToken?: string;
  authorityStateDirectory?: string;
  now?: Date;
};

export async function executePlan(
  plan: OceanEngineFormPlan,
  page: PageOperations,
  options: ExecutePlanOptions = {},
): Promise<RunnerV3Result> {
  const results: RunnerStepResult[] = [];
  let finalClickPerformed = false;
  try {
    const now = options.now ?? new Date();
    const authority = plan.mode === "prepare"
      ? (validatePreparePlan(plan), undefined)
      : validateSubmitPlan(plan, options.confirmToken, now);
    if (plan.status === "blocked") {
      return {
        schema_version: "oceanengine-playwright-rpa-result/v2",
        outcome: "blocked",
        error_code: "plan_blocked",
        error_message: plan.blocked_reasons.join(","),
        final_click_performed: false,
        reconciliation: "not_started",
        steps: results,
      };
    }

    const fieldSteps: PlanStep[] = [];
    for (const step of plan.steps) {
      if (step.kind === "identify_page") {
        await page.identifyPage(plan);
        results.push({ id: step.id, status: "succeeded" });
      } else if (step.kind === "field_action") {
        if (step.value_state === "missing") {
          results.push({ id: step.id, status: "succeeded", readback: null });
          continue;
        }
        await page.applyField(step);
        fieldSteps.push(step);
        results.push({ id: step.id, status: "succeeded", readback: await page.readField(step) });
      } else if (step.kind === "readback") {
        const readback: Record<string, unknown> = {};
        for (const field of fieldSteps) {
          if (field.field_key) readback[field.field_key] = await page.readField(field);
        }
        results.push({ id: step.id, status: "succeeded", readback });
      } else if (step.kind === "final_click_boundary") {
        if (plan.mode === "prepare") {
          results.push({ id: step.id, status: "blocked_boundary", error_code: step.block_reason });
          continue;
        }
        if (!authority) throw new RunnerV3Error("authority_missing", "submit authority is unavailable");
        const stateDirectory = options.authorityStateDirectory;
        if (!stateDirectory) throw new RunnerV3Error("authority_state_missing", "submit mode needs an authority state directory");
        await page.assertFinalReady(step);
        try {
          await consumeSubmitAuthority(authority, stateDirectory);
        } catch (error) {
          if (error instanceof AuthorityError) throw new RunnerV3Error(error.code, error.message);
          throw error;
        }
        await page.clickFinal(step);
        finalClickPerformed = true;
        const observation = await page.observeSubmit(plan);
        if (observation.outcome === "validation_error") {
          results.push({ id: step.id, status: "failed", error_code: "submit_validation_error", error_message: observation.error_message });
          return {
            schema_version: "oceanengine-playwright-rpa-result/v2",
            outcome: "failed",
            error_code: "submit_validation_error",
            error_message: observation.error_message,
            final_click_performed: true,
            reconciliation: "not_started",
            steps: results,
          };
        }
        if (observation.outcome === "result_unknown") {
          results.push({ id: step.id, status: "result_unknown", error_code: "result_unknown", error_message: observation.error_message });
          return {
            schema_version: "oceanengine-playwright-rpa-result/v2",
            outcome: "result_unknown",
            error_code: "result_unknown",
            error_message: observation.error_message,
            final_click_performed: true,
            reconciliation: "not_started",
            steps: results,
          };
        }
        const reconciliation = await page.reconcileSubmit(plan, observation);
        if (reconciliation.status === "not_found") {
          results.push({ id: step.id, status: "result_unknown", error_code: "reconciliation_not_found", readback: reconciliation });
          return {
            schema_version: "oceanengine-playwright-rpa-result/v2",
            outcome: "result_unknown",
            error_code: "reconciliation_not_found",
            error_message: "the submit succeeded but exact-name ID reconciliation did not find one row",
            final_click_performed: true,
            reconciliation: "not_found",
            steps: results,
          };
        }
        results.push({ id: step.id, status: "submitted", readback: reconciliation });
        const fieldStatus = reconciliation.field_reconciliation?.status;
        if (fieldStatus === "not_checked") {
          return {
            schema_version: "oceanengine-playwright-rpa-result/v2",
            outcome: "result_unknown",
            error_code: "field_reconciliation_not_checked",
            error_message: "the object ID was found but required persisted fields could not be checked",
            final_click_performed: true,
            ...(reconciliation.created_object_id ? { created_object_id: reconciliation.created_object_id } : {}),
            reconciliation: reconciliation.status,
            field_reconciliation: reconciliation.field_reconciliation,
            steps: results,
          };
        }
        const fieldDrifted = fieldStatus === "drifted";
        return {
          schema_version: "oceanengine-playwright-rpa-result/v2",
          outcome: fieldDrifted ? "success_with_drift" : "success",
          error_code: fieldDrifted ? "field_reconciliation_drift" : "ok",
          ...(fieldDrifted ? { error_message: "the object was created but one or more persisted fields differ from the submitted values" } : {}),
          final_click_performed: true,
          ...(reconciliation.created_object_id ? { created_object_id: reconciliation.created_object_id } : {}),
          reconciliation: reconciliation.status,
          ...(reconciliation.field_reconciliation ? { field_reconciliation: reconciliation.field_reconciliation } : {}),
          steps: results,
        };
      } else {
        throw new RunnerV3Error("invalid_plan", `unsupported step kind: ${step.kind}`);
      }
    }

    return {
      schema_version: "oceanengine-playwright-rpa-result/v2",
      outcome: "success",
      error_code: "ok",
      final_click_performed: false,
      reconciliation: "not_started",
      steps: results,
    };
  } catch (error) {
    const code = error instanceof RunnerV3Error ? error.code : finalClickPerformed ? "result_unknown" : "execution_failed";
    const message = error instanceof Error ? error.message : String(error);
    return {
      schema_version: "oceanengine-playwright-rpa-result/v2",
      outcome: finalClickPerformed ? "result_unknown" : "failed",
      error_code: code,
      error_message: message,
      final_click_performed: finalClickPerformed,
      reconciliation: "not_started",
      steps: results,
    };
  }
}

export async function executePreparePlan(
  plan: OceanEngineFormPlan,
  page: PageOperations,
) {
  return executePlan(plan, page);
}

export class PlaywrightPageOperations implements PageOperations {
  private preSubmitUrl = "";
  private readonly referenceReadbacks = new Map<string, unknown>();

  constructor(private readonly page: Page) {}

  async identifyPage(plan: OceanEngineFormPlan) {
    const url = new URL(this.page.url());
    if (url.protocol !== "https:" || url.hostname !== "ad.oceanengine.com") {
      throw new RunnerV3Error("page_drift", "the active page is not an OceanEngine HTTPS page");
    }
    const expectedPath = plan.plan_kind.startsWith("project_") ? "/superior/create-project" : "/superior/ads";
    if (!url.pathname.startsWith(expectedPath)) {
      throw new RunnerV3Error("page_drift", `expected ${expectedPath}, got ${url.pathname}`);
    }
    const observedAccount = url.searchParams.get("aadvid");
    if (
      observedAccount &&
      !plan.account_reference.startsWith("redacted:") &&
      observedAccount !== plan.account_reference
    ) {
      throw new RunnerV3Error("account_mismatch", "the active account does not match the plan");
    }
    const observedObject = url.searchParams.get(plan.plan_kind.startsWith("project_") ? "project_id" : "promotion_id");
    if (plan.object_reference && !plan.object_reference.startsWith("redacted:") && observedObject && observedObject !== plan.object_reference) {
      throw new RunnerV3Error("object_mismatch", "the active object does not match the plan");
    }
  }

  private scopeLocator(step: PlanStep) {
    if (!step.scope) throw new RunnerV3Error("invalid_plan", `${step.id}: scope is required`);
    return this.page.getByText(step.scope, { exact: true });
  }

  private async selectedCallToActions(page: Page = this.page) {
    let scope = page.getByText("行动号召", { exact: true });
    for (let attempt = 0; attempt < 40 && (await scope.count()) === 0; attempt += 1) {
      await page.waitForTimeout(250);
      scope = page.getByText("行动号召", { exact: true });
    }
    if ((await scope.count()) < 1) return [];
    const row = scope.first().locator(
      "xpath=ancestor::div[contains(concat(' ',normalize-space(@class),' '),' oc-row ')][1]",
    );
    const tags = row.locator(".oc-tag-text");
    const values: string[] = [];
    for (let index = 0; index < await tags.count(); index += 1) {
      const tag = tags.nth(index);
      if (!await tag.isVisible()) continue;
      const value = (await tag.innerText()).trim();
      if (value && !values.includes(value)) values.push(value);
    }
    return values;
  }

  private xpathLiteral(value: string) {
    if (!value.includes("'")) return `'${value}'`;
    if (!value.includes('"')) return `"${value}"`;
    return `concat(${value.split("'").map((part) => `'${part}'`).join(", \"'\", ")})`;
  }

  private async targetLocator(step: PlanStep): Promise<Locator> {
    if (!step.target) throw new RunnerV3Error("invalid_plan", `${step.id}: target is required`);
    const scope = this.scopeLocator(step);
    for (let attempt = 0; attempt < 40 && (await scope.count()) < 1; attempt += 1) {
      await this.page.waitForTimeout(250);
    }
    if ((await scope.count()) < 1) throw new RunnerV3Error("page_drift", `${step.id}: scope did not load`);
    if (step.field_key === "project.marketing_product_reference") {
      const requested = this.page.getByText(step.target, { exact: true });
      if ((await requested.count()) === 0) {
        const replacement = this.page.getByRole("button", { name: "更换", exact: true });
        if ((await replacement.count()) === 1 && await replacement.isVisible()) return replacement;
      }
    }
    if (step.field_key === "promotion.base_materials") {
      const addVideo = this.page.getByRole("button", { name: "添加视频", exact: true });
      if ((await addVideo.count()) === 1 && await addVideo.isVisible()) return addVideo;
    }
    if (step.field_key === "promotion.product_image_references") {
      const productImageScope = this.page.getByText("产品主图", { exact: true });
      const productImageRow = productImageScope.first().locator(
        "xpath=ancestor::div[contains(concat(' ',normalize-space(@class),' '),' oc-row ')][1]",
      );
      const addProductImage = productImageRow.locator(".oc-create-product-img-add-button");
      if ((await addProductImage.count()) === 1 && await addProductImage.isVisible()) return addProductImage;
    }
    if (step.field_key === "promotion.promotion_name") {
      const candidates = this.page.getByPlaceholder("请输入", { exact: true });
      const editable: Locator[] = [];
      for (let index = 0; index < await candidates.count(); index += 1) {
        const candidate = candidates.nth(index);
        if (await candidate.isVisible() && await candidate.isEnabled()) editable.push(candidate);
      }
      if (editable.length > 0) return editable.at(-1)!;
    }
    if (step.field_key === "promotion.landing_page_reference") {
      const landingInput = this.page.getByPlaceholder(step.target, { exact: true });
      if ((await landingInput.count()) === 1) {
        if (step.operation === "fill_text" && await landingInput.isVisible()) return landingInput;
        const pickerControl = landingInput.locator("xpath=following-sibling::*[contains(@class,'input__suffix')][1]");
        if ((await pickerControl.count()) === 1 && await pickerControl.isVisible()) return pickerControl;
      }
    }
    if (step.field_key === "promotion.comments_enabled") {
      const commentScope = this.page.getByText("单元评论", { exact: true }).last();
      const option = commentScope.locator("xpath=ancestor::*[contains(@class,'oc-row')][1]").getByText(step.target, { exact: true });
      if ((await option.count()) === 1 && await option.isVisible()) return option;
    }
    if (step.field_key === "promotion.smart_generation_enabled") {
      const checkbox = this.page.getByRole("checkbox", { name: "开启智能生成", exact: true });
      if ((await checkbox.count()) === 1 && await checkbox.isVisible()) return checkbox;
    }
    if (step.target === "spinbutton") {
      if (step.field_key === "project.daily_budget" && (await this.page.getByRole("spinbutton").count()) === 0) {
        const reveal = await this.uniqueVisibleText("设置预算", step.id);
        await reveal.click();
        for (let attempt = 0; attempt < 20 && (await this.page.getByRole("spinbutton").count()) === 0; attempt += 1) {
          await this.page.waitForTimeout(200);
        }
      }
      if (step.field_key === "promotion.daily_budget" || step.field_key === "promotion.bid") {
        const promotionMoney = this.page.getByRole("spinbutton");
        for (let attempt = 0; attempt < 40 && (await promotionMoney.count()) < 2; attempt += 1) {
          await this.page.waitForTimeout(250);
        }
        if ((await promotionMoney.count()) === 2) {
          return promotionMoney.nth(step.field_key === "promotion.daily_budget" ? 0 : 1);
        }
      }
      const scoped = scope.first().locator("xpath=ancestor::*[.//*[@role='spinbutton'] or .//input][1]").getByRole("spinbutton");
      if ((await scoped.count()) === 1) return scoped;
      const all = this.page.getByRole("spinbutton");
      if ((await all.count()) === 1) return all;
      throw new RunnerV3Error("locator_not_unique", `${step.id}: spinbutton is not unique`);
    }
    const scopedInput = scope.first()
      .locator("xpath=ancestor::*[.//input][1]")
      .getByPlaceholder(step.target, { exact: true });
    if ((await scopedInput.count()) === 1 && await scopedInput.isVisible()) return scopedInput;
    const placeholder = this.page.getByPlaceholder(step.target, { exact: true });
    if ((await placeholder.count()) === 1 && await placeholder.isVisible()) return placeholder;
    const scopedText = scope.first()
      .locator(`xpath=ancestor::*[.//*[normalize-space(.)=${this.xpathLiteral(step.target)}]][1]`)
      .getByText(step.target, { exact: true });
    if ((await scopedText.count()) === 1 && await scopedText.isVisible()) return scopedText;
    const text = this.page.getByText(step.target, { exact: true });
    if ((await text.count()) === 1) return text;
    throw new RunnerV3Error("locator_not_unique", `${step.id}: target is not unique`);
  }

  async applyField(step: PlanStep) {
    if (!step.operation) throw new RunnerV3Error("invalid_plan", `${step.id}: operation is required`);
    if (step.field_key === "project.marketing_product_reference" && this.isReferenceSelectionSpec(step.value)) {
      const label = step.value.label;
      if (label) {
        const existingProduct = this.page.getByText(label, { exact: true });
        for (let index = 0; index < await existingProduct.count(); index += 1) {
          if (await existingProduct.nth(index).isVisible()) {
            this.referenceReadbacks.set(step.id, {
              selection_kind: step.value.selection_kind,
              selected_count: 1,
              label,
              object_id: step.value.object_id,
              reused_existing_selection: true,
            });
            return;
          }
        }
      }
    }
    if (step.field_key === "promotion.delivery_identity") {
      const spec = this.isReferenceSelectionSpec(step.value) ? step.value : undefined;
      const label = spec?.label ?? (typeof step.value === "string" ? step.value : undefined);
      if (label) {
        const selected = this.page.getByText(label, { exact: true });
        for (let attempt = 0; attempt < 40 && (await selected.count()) === 0; attempt += 1) {
          await this.page.waitForTimeout(250);
        }
        for (let index = 0; index < await selected.count(); index += 1) {
          if (await selected.nth(index).isVisible()) {
            this.referenceReadbacks.set(step.id, { selection_kind: spec?.selection_kind ?? "text_option", selected_count: 1, label });
            return;
          }
        }
      }
    }
    if (step.field_key === "promotion.base_materials") {
      const existingVideo = this.page.getByText(/视频\([1-9]\d*\/\d+\)/);
      if ((await existingVideo.count()) === 1 && await existingVideo.isVisible()) {
        this.referenceReadbacks.set(step.id, { selection_kind: "media_card", selected_count: 1, reused_existing_selection: true });
        return;
      }
    }
    if (step.field_key === "promotion.product_selling_points") {
      const values = Array.isArray(step.value) ? step.value.map(String) : [String(step.value)];
      const sellingPointScope = this.page.getByText("产品卖点", { exact: true }).first();
      const sellingPointRow = sellingPointScope.locator(
        "xpath=ancestor::div[contains(concat(' ',normalize-space(@class),' '),' oc-row ')][1]",
      );
      let allSelected = values.length > 0;
      for (const value of values) {
        const selected = sellingPointRow.getByText(value, { exact: true });
        if ((await selected.count()) < 1) allSelected = false;
      }
      if (allSelected) {
        this.referenceReadbacks.set(step.id, values);
        return;
      }
    }
    if (step.field_key === "promotion.product_image_references") {
      const productImageScope = this.page.getByText("产品主图", { exact: true }).first();
      const productImageRow = productImageScope.locator(
        "xpath=ancestor::div[contains(concat(' ',normalize-space(@class),' '),' oc-row ')][1]",
      );
      const selectedCount = productImageRow.getByText(/已添加：\s*[1-9]\d*\/\d+/);
      if ((await selectedCount.count()) === 1 && await selectedCount.isVisible()) {
        this.referenceReadbacks.set(step.id, { selection_kind: "image_card", selected_count: 1, reused_existing_selection: true });
        return;
      }
    }
    if (step.field_key === "promotion.call_to_action") {
      const values = Array.isArray(step.value) ? step.value.map(String) : [];
      if (values.length < 1 || values.length > 10 || new Set(values).size !== values.length || values.some((value) => !value.trim())) {
        throw new RunnerV3Error("invalid_value", `${step.id}: call to action needs 1 to 10 unique values`);
      }
      const selected = await this.selectedCallToActions();
      const matches = selected.length === values.length && values.every((value) => selected.includes(value));
      if (!matches) {
        throw new RunnerV3Error("operator_required", `${step.id}: call-to-action multi-select mutation is not calibrated`);
      }
      this.referenceReadbacks.set(step.id, selected);
      return;
    }
    if (step.field_key === "promotion.category") {
      const target = await this.targetLocator(step);
      await target.click();
      for (const segment of String(step.value).split("/").filter(Boolean)) {
        const option = await this.uniqueVisibleText(segment, step.id);
        await option.click();
      }
      return;
    }
    if (step.field_key === "promotion.brand_reference" && this.isReferenceSelectionSpec(step.value)) {
      const label = step.value.label;
      if (!label) throw new RunnerV3Error("invalid_value", `${step.id}: brand label is required`);
      const target = await this.targetLocator(step);
      await target.click();
      let option = this.page.getByText(label, { exact: true });
      let visibleOption: Locator | undefined;
      for (let index = 0; index < await option.count(); index += 1) {
        if (await option.nth(index).isVisible()) visibleOption = option.nth(index);
      }
      if (!visibleOption) {
        const custom = this.page.getByRole("button", { name: "自定义品牌名称", exact: true });
        if ((await custom.count()) !== 1 || !(await custom.isVisible())) {
          throw new RunnerV3Error("async_load_timeout", `${step.id}: brand option did not load`);
        }
        await custom.click();
        const confirm = this.page.getByRole("button", { name: "确定", exact: true });
        const confirmScope = confirm.locator("xpath=ancestor::*[.//input][1]");
        const customInput = confirmScope.getByPlaceholder("请输入", { exact: true });
        if ((await customInput.count()) !== 1) {
          throw new RunnerV3Error("locator_not_unique", `${step.id}: custom brand input is not unique`);
        }
        await customInput.fill(label);
        await confirm.click();
        option = this.page.getByText(label, { exact: true });
        for (let attempt = 0; attempt < 20 && !visibleOption; attempt += 1) {
          for (let index = 0; index < await option.count(); index += 1) {
            if (await option.nth(index).isVisible()) visibleOption = option.nth(index);
          }
          if (!visibleOption) await this.page.waitForTimeout(100);
        }
      }
      if (!visibleOption) throw new RunnerV3Error("async_load_timeout", `${step.id}: brand option did not become visible`);
      await visibleOption.click();
      this.referenceReadbacks.set(step.id, {
        selection_kind: step.value.selection_kind,
        selected_count: 1,
        label,
      });
      return;
    }
    if (step.operation === "choose_exact_visible_option") {
      const value = String(step.value);
      if (step.target !== value) {
        const target = await this.targetLocator(step);
        if (!(await target.isEnabled())) {
          const observed = await target.inputValue().catch(async () => (await target.textContent()) ?? "");
          if (observed.trim() === value) return;
          throw new RunnerV3Error("immutable_field_mismatch", `${step.id}: locked field does not match the requested value`);
        }
        await target.click();
        const option = await this.uniqueVisibleText(value, step.id);
        await option.click();
      } else {
        const option = await this.targetLocator(step);
        await option.click();
      }
      return;
    }

    const target = await this.targetLocator(step);
    if (step.operation === "fill_text" || step.operation === "fill_money" || step.operation === "fill_decimal") {
      await target.fill(String(step.value));
    } else if (step.operation === "toggle") {
      if (typeof step.value !== "boolean") throw new RunnerV3Error("invalid_value", `${step.id}: toggle needs a boolean`);
      await this.setCheckbox(target, step.value, step.id);
    } else if (step.operation === "open_reference_picker") {
      await target.click();
      if (this.isReferenceSelectionSpec(step.value)) {
        this.referenceReadbacks.set(step.id, await this.selectReference(step, step.value));
      } else {
        const values = Array.isArray(step.value) ? step.value : [step.value];
        for (const value of values) {
          const option = await this.uniqueVisibleText(String(value), step.id);
          await option.click();
        }
        this.referenceReadbacks.set(step.id, { selection_kind: "text_option", selected_count: values.length, values });
      }
    } else if (step.operation === "configure_object") {
      await this.configureObject(step, target);
    } else {
      throw new RunnerV3Error("invalid_plan", `${step.id}: unsupported operation ${step.operation}`);
    }
  }

  private isReferenceSelectionSpec(value: unknown): value is ReferenceSelectionSpec {
    if (!value || Array.isArray(value) || typeof value !== "object") return false;
    return ["text_option", "async_row", "media_card", "image_card"].includes(
      String((value as Record<string, unknown>).selection_kind),
    );
  }

  private async uniqueVisibleText(value: string | RegExp, stepId: string, root: Page | Locator = this.page) {
    for (let attempt = 0; attempt < 40; attempt += 1) {
      const options = root.getByText(value, typeof value === "string" ? { exact: true } : undefined);
      const visible: Locator[] = [];
      for (let index = 0; index < await options.count(); index += 1) {
        const option = options.nth(index);
        if (await option.isVisible()) visible.push(option);
      }
      if (visible.length === 1) return visible[0];
      if (visible.length > 1) throw new RunnerV3Error("locator_not_unique", `${stepId}: visible reference option is not unique`);
      await this.page.waitForTimeout(250);
    }
    throw new RunnerV3Error("async_load_timeout", `${stepId}: reference option did not load`);
  }

  private async stableVisibleCount(locator: Locator, minimum: number, stepId: string) {
    let prior = -1;
    let stable = 0;
    for (let attempt = 0; attempt < 40; attempt += 1) {
      let count = 0;
      for (let index = 0; index < await locator.count(); index += 1) {
        if (await locator.nth(index).isVisible()) count += 1;
      }
      stable = count === prior ? stable + 1 : 0;
      if (count >= minimum && stable >= 2) return count;
      prior = count;
      await this.page.waitForTimeout(250);
    }
    throw new RunnerV3Error("async_load_timeout", `${stepId}: visible reference count did not become stable`);
  }

  private async pickerRoot() {
    const dialogs = this.page.locator("[role='dialog']:visible,.ovui-modal:visible,[class*='modal']:visible,.ovui-drawer:visible");
    for (let attempt = 0; attempt < 40 && (await dialogs.count()) === 0; attempt += 1) {
      await this.page.waitForTimeout(250);
    }
    return (await dialogs.count()) > 0 ? dialogs.last() : this.page.locator("body");
  }

  private async confirmPicker(root: Locator, spec: ReferenceSelectionSpec, stepId: string) {
    const buttonName = spec.confirm_button ?? "确定";
    const local = root.getByRole("button", { name: buttonName, exact: true });
    if ((await local.count()) === 1 && await local.isVisible()) {
      await local.click();
      return;
    }
    const global = this.page.getByRole("button", { name: buttonName, exact: true });
    const visible: Locator[] = [];
    for (let index = 0; index < await global.count(); index += 1) {
      if (await global.nth(index).isVisible()) visible.push(global.nth(index));
    }
    if (visible.length !== 1) throw new RunnerV3Error("locator_not_unique", `${stepId}: picker confirmation is not unique`);
    await visible[0].click();
  }

  private async setCheckbox(checkbox: Locator, checked: boolean, stepId: string) {
    if (await checkbox.isChecked() === checked) return;
    const visualControl = checkbox.locator("xpath=following-sibling::*[contains(@class,'checkbox__inner')][1]");
    if ((await visualControl.count()) === 1 && await visualControl.isVisible()) {
      await visualControl.click();
    } else {
      await checkbox.setChecked(checked, { force: true });
    }
    if (await checkbox.isChecked() !== checked) {
      throw new RunnerV3Error("reference_not_selected", `${stepId}: checkbox state did not change`);
    }
  }

  private async selectReference(step: PlanStep, spec: ReferenceSelectionSpec) {
    const root = await this.pickerRoot();
    if (spec.expected_total !== undefined) {
      const total = this.page.getByText(new RegExp(`共\\s*${spec.expected_total}\\s*条`));
      await this.stableVisibleCount(total, 1, step.id);
    }
    if (spec.selection_kind === "text_option" || spec.selection_kind === "async_row") {
      if (!spec.label && !spec.object_id) {
        throw new RunnerV3Error("invalid_value", `${step.id}: reference label or object_id is required`);
      }
      if (spec.object_id) {
        const idText = await this.uniqueVisibleText(new RegExp(`ID[:：]\\s*${spec.object_id}`), step.id, root);
        const cardRow = idText.locator("xpath=ancestor::*[contains(@class,'create-material-list-card-item')][1]");
        if ((await cardRow.count()) === 1) {
          if (spec.label && !(await cardRow.innerText()).includes(spec.label)) {
            throw new RunnerV3Error("object_mismatch", `${step.id}: reference ID does not match the expected label`);
          }
          await cardRow.click();
          const selectedCount = root.getByText(/已选择\s*1\s*\/\s*1/);
          await this.stableVisibleCount(selectedCount, 1, step.id);
          await this.confirmPicker(root, spec, step.id);
          return {
            selection_kind: spec.selection_kind,
            selected_count: 1,
            expected_total: spec.expected_total,
            label: spec.label,
            object_id: spec.object_id,
            element_verified: "card_and_selected_count",
          };
        }
        const row = idText.locator("xpath=ancestor::*[.//*[@role='checkbox'] or .//input[@type='checkbox']][1]");
        if ((await row.count()) !== 1) throw new RunnerV3Error("page_drift", `${step.id}: reference row has no unique checkbox scope`);
        if (spec.label && !(await row.innerText()).includes(spec.label)) {
          throw new RunnerV3Error("object_mismatch", `${step.id}: reference ID does not match the expected label`);
        }
        const checkbox = row.locator("input[type='checkbox'],[role='checkbox']");
        if ((await checkbox.count()) !== 1) throw new RunnerV3Error("locator_not_unique", `${step.id}: reference row checkbox is not unique`);
        await this.setCheckbox(checkbox, true, step.id);
      } else {
        const option = await this.uniqueVisibleText(spec.label!, step.id, root);
        await option.click();
      }
      await this.confirmPicker(root, spec, step.id);
      return {
        selection_kind: spec.selection_kind,
        selected_count: 1,
        expected_total: spec.expected_total,
        label: spec.label,
        object_id: spec.object_id,
      };
    }

    if (step.field_key === "promotion.product_image_references" && spec.selection_kind === "image_card") {
      const images = root.locator("img:visible");
      const visibleCount = await this.stableVisibleCount(images, spec.minimum_visible ?? 1, step.id);
      const index = spec.index ?? 0;
      if (index < 0 || index >= visibleCount) {
        throw new RunnerV3Error("invalid_value", `${step.id}: product image index is out of range`);
      }
      await images.nth(index).click();
      await this.stableVisibleCount(root.getByText(/已选择\s*1\s*\/\s*10/), 1, step.id);
      await this.confirmPicker(root, spec, step.id);
      return {
        selection_kind: spec.selection_kind,
        selected_count: 1,
        visible_count: visibleCount,
        index,
        element_verified: "img_and_selected_count",
      };
    }

    let cards = root.locator(".oc-create-material-card-content:visible");
    if (spec.selection_kind === "image_card") cards = cards.filter({ has: root.locator("img") });
    const visibleCount = await this.stableVisibleCount(cards, spec.minimum_visible ?? 1, step.id);
    let card: Locator;
    if (spec.label) {
      const labelled = cards.filter({ hasText: spec.label });
      if ((await labelled.count()) !== 1) throw new RunnerV3Error("locator_not_unique", `${step.id}: material card is not unique`);
      card = labelled;
    } else {
      const index = spec.index ?? 0;
      if (index < 0 || index >= visibleCount) throw new RunnerV3Error("invalid_value", `${step.id}: material card index is out of range`);
      card = cards.nth(index);
    }
    if (spec.selection_kind === "image_card" && (await card.locator("img").count()) < 1) {
      throw new RunnerV3Error("page_drift", `${step.id}: selected image card has no img element`);
    }
    const checkbox = card.locator("input[type='checkbox'],[role='checkbox']");
    if ((await checkbox.count()) > 0) await this.setCheckbox(checkbox.first(), true, step.id);
    else await card.click();
    const checked = card.locator("input[type='checkbox']:checked,[role='checkbox'][aria-checked='true']");
    if ((await checked.count()) < 1 && !(await card.getAttribute("class"))?.includes("select")) {
      throw new RunnerV3Error("reference_not_selected", `${step.id}: material card did not report a selected state`);
    }
    await this.confirmPicker(root, spec, step.id);
    return {
      selection_kind: spec.selection_kind,
      selected_count: 1,
      visible_count: visibleCount,
      label: spec.label,
      index: spec.index ?? 0,
      element_verified: spec.selection_kind === "image_card" ? "img_and_checkbox" : "card_and_checkbox",
    };
  }

  private async configureObject(step: PlanStep, target: Locator) {
    if (step.field_key === "project.schedule") {
      const value = step.value as { start?: unknown; end?: unknown };
      if (!value || typeof value !== "object" || !value.start || !value.end) {
        throw new RunnerV3Error("invalid_value", `${step.id}: schedule needs start and end dates`);
      }
      await target.click();
      const start = this.page.getByPlaceholder("请选择开始日期", { exact: true });
      const end = this.page.getByPlaceholder("请选择结束日期", { exact: true });
      for (let attempt = 0; attempt < 40 && ((await start.count()) !== 1 || (await end.count()) !== 1); attempt += 1) {
        await this.page.waitForTimeout(250);
      }
      if ((await start.count()) !== 1 || (await end.count()) !== 1) {
        throw new RunnerV3Error("locator_not_unique", `${step.id}: schedule inputs are not unique`);
      }
      if (!(await start.isEnabled()) || !(await end.isEnabled())) {
        const observedStart = await start.inputValue();
        const observedEnd = await end.inputValue();
        if (observedStart === String(value.start) && observedEnd === String(value.end)) return;
        throw new RunnerV3Error("immutable_field_mismatch", `${step.id}: locked schedule does not match the requested value`);
      }
      await start.fill(String(value.start));
      await end.fill(String(value.end));
      return;
    }
    if (step.field_key === "project.placement_media") {
      if (!Array.isArray(step.value)) throw new RunnerV3Error("invalid_value", `${step.id}: placement media needs an array`);
      const selected = new Set(step.value.map(String));
      for (const name of ["今日头条", "西瓜视频", "抖音", "番茄系媒体", "穿山甲"]) {
        const label = this.page.getByText(name, { exact: true });
        if ((await label.count()) !== 1) throw new RunnerV3Error("locator_not_unique", `${step.id}: media label ${name} is not unique`);
        const checkbox = label.locator("xpath=preceding-sibling::*[@role='checkbox'][1] | preceding-sibling::input[@type='checkbox'][1]");
        if ((await checkbox.count()) !== 1) throw new RunnerV3Error("page_drift", `${step.id}: media checkbox ${name} is missing`);
        await this.setCheckbox(checkbox, selected.has(name), step.id);
      }
      return;
    }
    if (step.field_key === "promotion.copy_materials" || step.field_key === "promotion.product_selling_points") {
      const values = Array.isArray(step.value) ? step.value.map(String) : [String(step.value)];
      if (step.field_key === "promotion.copy_materials") {
        await target.fill(values.join("\n"));
      } else {
        for (const value of values) {
          await target.fill(value);
          await target.press("Enter");
        }
        this.referenceReadbacks.set(step.id, values);
      }
      return;
    }
    throw new RunnerV3Error("operator_required", `${step.id}: complex object configuration needs a field-specific adapter`);
  }

  async readField(step: PlanStep) {
    if (this.referenceReadbacks.has(step.id)) {
      return this.referenceReadbacks.get(step.id);
    }
    if (step.operation === "toggle") {
      return (await this.targetLocator(step)).isChecked();
    }
    if (step.operation === "choose_exact_visible_option" || step.operation === "open_reference_picker") {
      return step.value;
    }
    const target = await this.targetLocator(step);
    const tagName = String(await target.evaluate((element) => element.tagName)).toLowerCase();
    if (tagName === "input" || tagName === "textarea") return target.inputValue();
    return (await target.innerText()).trim();
  }

  async clickFinal(step: PlanStep) {
    await this.assertFinalReady(step);
    const button = this.page.getByRole("button", { name: step.target!, exact: true });
    this.preSubmitUrl = this.page.url();
    await button.click({ noWaitAfter: true });
  }

  async assertFinalReady(step: PlanStep) {
    if (!step.target) throw new RunnerV3Error("invalid_plan", `${step.id}: final target is required`);
    const button = this.page.getByRole("button", { name: step.target, exact: true });
    if ((await button.count()) !== 1) throw new RunnerV3Error("locator_not_unique", `${step.id}: final button is not unique`);
    if (!(await button.isEnabled())) throw new RunnerV3Error("submit_disabled", `${step.id}: final button is disabled`);
  }

  async observeSubmit(_plan: OceanEngineFormPlan): Promise<SubmitObservation> {
    for (let attempt = 0; attempt < 24; attempt += 1) {
      const success = this.page.getByText(/(?:保存|创建).{0,8}成功/);
      for (let index = 0; index < await success.count(); index += 1) {
        if (await success.nth(index).isVisible()) return { outcome: "success" };
      }
      const errors = this.page.locator(".ovui-form-item-error,[class*='form-item-error'],[class*='FormItemError']");
      const messages: string[] = [];
      for (let index = 0; index < await errors.count(); index += 1) {
        const error = errors.nth(index);
        if (await error.isVisible()) messages.push((await error.innerText()).trim());
      }
      if (messages.some(Boolean)) return { outcome: "validation_error", error_message: messages.filter(Boolean).join("; ") };
      const validationSummary = this.page.getByText("有些项目填写错误，请修改后再提交", { exact: true });
      if ((await validationSummary.count()) > 0 && await validationSummary.last().isVisible()) {
        return { outcome: "validation_error", error_message: "the platform reported form validation errors" };
      }
      if (this.page.url() !== this.preSubmitUrl) return { outcome: "success" };
      await this.page.waitForTimeout(250);
    }
    return { outcome: "result_unknown", error_message: "no success, validation error, or navigation was observed after one click" };
  }

  async reconcileSubmit(plan: OceanEngineFormPlan, observation: SubmitObservation): Promise<ReconciliationResult> {
    if (plan.plan_kind.endsWith("_edit")) {
      return plan.object_reference
        ? { status: "matched", created_object_id: plan.object_reference }
        : { status: "not_applicable" };
    }
    if (observation.created_object_id) return { status: "matched", created_object_id: observation.created_object_id };
    const queryKey = plan.plan_kind === "project_create" ? "project_id" : "promotion_id";
    let queryId: string | null = null;
    for (let attempt = 0; attempt < 40 && !queryId; attempt += 1) {
      queryId = new URL(this.page.url()).searchParams.get(queryKey);
      if (!queryId) await this.page.waitForTimeout(250);
    }
    if (queryId) return { status: "matched", created_object_id: queryId };

    const nameKey = plan.plan_kind === "project_create" ? "project.project_name" : "promotion.promotion_name";
    const expectedName = plan.steps.find((step) => step.field_key === nameKey)?.value;
    if (typeof expectedName !== "string" || !expectedName) return { status: "not_applicable" };
    const placeholder = plan.plan_kind === "project_create"
      ? "输入项目ID或名称后回车搜索"
      : "输入单元ID或名称后回车搜索";
    let search = this.page.getByPlaceholder(placeholder, { exact: true });
    for (let attempt = 0; attempt < 40 && (await search.count()) === 0; attempt += 1) {
      await this.page.waitForTimeout(250);
      search = this.page.getByPlaceholder(placeholder, { exact: true });
    }
    if (plan.plan_kind === "promotion_create" && (await search.count()) === 0) {
      const promotionTabs = this.page.getByText("单元", { exact: true });
      for (let index = 0; index < await promotionTabs.count(); index += 1) {
        const promotionTab = promotionTabs.nth(index);
        if (await promotionTab.isVisible()) {
          await promotionTab.click();
          break;
        }
      }
      for (let attempt = 0; attempt < 40 && (await search.count()) === 0; attempt += 1) {
        await this.page.waitForTimeout(250);
        search = this.page.getByPlaceholder(placeholder, { exact: true });
      }
    }
    if ((await search.count()) === 1) {
      await search.fill(expectedName);
      await search.press("Enter");
    }
    for (let attempt = 0; attempt < 40; attempt += 1) {
      const row = this.page.getByRole("row").filter({ hasText: expectedName });
      if ((await row.count()) === 1) {
        const match = (await row.innerText()).match(/ID[:：]\s*(\d+)/);
        if (match?.[1]) {
          const fieldReconciliation = plan.plan_kind === "promotion_create"
            ? await this.reconcilePromotionFields(plan, row)
            : undefined;
          return {
            status: "matched",
            created_object_id: match[1],
            ...(fieldReconciliation ? { field_reconciliation: fieldReconciliation } : {}),
          };
        }
      }
      await this.page.waitForTimeout(250);
    }
    return { status: "not_found" };
  }

  private async reconcilePromotionFields(plan: OceanEngineFormPlan, row: Locator): Promise<FieldReconciliation> {
    const landingFieldKey = "promotion.landing_page_reference";
    const landingExpected = stableReferenceID(plan.steps.find((step) => step.field_key === landingFieldKey)?.value);
    const callToActionFieldKey = "promotion.call_to_action";
    const callToActionValue = plan.steps.find((step) => step.field_key === callToActionFieldKey)?.value;
    const callToActionExpected = Array.isArray(callToActionValue) ? callToActionValue.map(String) : undefined;
    const notChecked = (): FieldReconciliation => ({
      status: "not_checked",
      fields: [
        { field_key: landingFieldKey, ...(landingExpected ? { expected: landingExpected } : {}), status: "not_checked" },
        { field_key: callToActionFieldKey, ...(callToActionExpected ? { expected: callToActionExpected } : {}), status: "not_checked" },
      ],
    });

    const edit = row.getByText("编辑", { exact: true });
    if ((await edit.count()) !== 1) {
      return notChecked();
    }

    const pagePromise = this.page.context().waitForEvent("page", { timeout: 5_000 }).catch(() => undefined);
    await edit.click({ noWaitAfter: true });
    const editPage = await pagePromise;
    if (!editPage) {
      return notChecked();
    }

    try {
      await editPage.waitForLoadState("domcontentloaded", { timeout: 10_000 }).catch(() => undefined);
      const fields: FieldReconciliation["fields"] = [];
      let landingInput = editPage.getByPlaceholder(/落地页链接/);
      for (let attempt = 0; attempt < 40 && (await landingInput.count()) === 0; attempt += 1) {
        await editPage.waitForTimeout(250);
        landingInput = editPage.getByPlaceholder(/落地页链接/);
      }
      const landingObserved = (await landingInput.count()) > 0
        ? landingExpected?.startsWith("http")
          ? (await landingInput.first().inputValue()).trim()
          : await landingInput.first().evaluate((element) => {
            let current: Element | null = element;
            for (let depth = 0; current && depth < 10; depth += 1, current = current.parentElement) {
              const match = current.textContent?.match(/ID[:：]\s*(\d+)/);
              if (match?.[1]) return match[1];
            }
            return undefined;
          })
        : undefined;
      const landingStatus = !landingExpected || !landingObserved
        ? "not_checked"
        : landingObserved === landingExpected ? "matched" : "drifted";
      fields.push({
        field_key: landingFieldKey,
        ...(landingExpected ? { expected: landingExpected } : {}),
        ...(landingObserved ? { observed: landingObserved } : {}),
        status: landingStatus,
      });

      const callToActionObserved = await this.selectedCallToActions(editPage);
      const callToActionStatus = !callToActionExpected || callToActionObserved.length === 0
        ? "not_checked"
        : callToActionObserved.length === callToActionExpected.length && callToActionExpected.every((value) => callToActionObserved.includes(value))
          ? "matched"
          : "drifted";
      fields.push({
        field_key: callToActionFieldKey,
        ...(callToActionExpected ? { expected: callToActionExpected } : {}),
        ...(callToActionObserved.length > 0 ? { observed: callToActionObserved } : {}),
        status: callToActionStatus,
      });

      const status = fields.some((field) => field.status === "drifted")
        ? "drifted"
        : fields.some((field) => field.status === "not_checked") ? "not_checked" : "matched";
      return { status, fields };
    } finally {
      await editPage.close().catch(() => undefined);
    }
  }
}

async function readStdin() {
  return new Promise<string>((resolve, reject) => {
    let data = "";
    process.stdin.setEncoding("utf8");
    process.stdin.on("data", (chunk) => { data += chunk; });
    process.stdin.on("end", () => resolve(data));
    process.stdin.on("error", reject);
  });
}

async function main() {
  const args = process.argv.slice(2);
  const option = (name: string) => {
    const index = args.indexOf(name);
    return index >= 0 ? args[index + 1] : undefined;
  };
  // Some npm versions remove an unknown named option but keep its value.
  const sessionFile = option("--session-file") ?? (args[0]?.toLowerCase().endsWith(".json") ? args[0] : undefined);
  const cdpURL = sessionFile ? await resolveSessionPlaywrightEndpoint(sessionFile) : args[0];
  if (!cdpURL || cdpURL.startsWith("--")) {
    throw new Error("Usage: echo PLAN.json | tsx scripts/browser-rpa-runner-v3.ts CDP_URL [--session-file PATH] [--confirm-token TOKEN] [--authority-state-dir DIR]");
  }
  const plan = JSON.parse(await readStdin()) as OceanEngineFormPlan;
  const confirmToken = option("--confirm-token");
  const authorityStateDirectory = option("--authority-state-dir") ?? join(
    process.env.LOCALAPPDATA ?? tmpdir(),
    "cookies",
    "browser-rpa",
    "authority-consumed",
  );
  const browser = await chromium.connectOverCDP(cdpURL);
  const context = browser.contexts()[0];
  if (!context) throw new Error("the Edge session has no browser context");
  const expectedPath = plan.plan_kind.startsWith("project_") ? "/superior/create-project" : "/superior/ads";
  const objectParameter = plan.plan_kind.startsWith("project_") ? "project_id" : "promotion_id";
  const page = context.pages().find((candidate) => {
    try {
      const url = new URL(candidate.url());
      if (url.hostname !== "ad.oceanengine.com" || !url.pathname.startsWith(expectedPath)) return false;
      if (!plan.object_reference || plan.object_reference.startsWith("redacted:")) return true;
      return url.searchParams.get(objectParameter) === plan.object_reference;
    } catch {
      return false;
    }
  }) ?? context.pages().find((candidate) => /oceanengine\.com/i.test(candidate.url())) ?? context.pages()[0];
  if (!page) throw new Error("the Edge session has no page");
  const result = await executePlan(plan, new PlaywrightPageOperations(page), {
    ...(confirmToken ? { confirmToken } : {}),
    authorityStateDirectory,
  });
  const finalStep = result.steps.at(-1);
  if (finalStep) {
    const current = new URL(page.url());
    finalStep.page_reference = `${current.protocol}//${current.host}${current.pathname}`;
  }
  await new Promise<void>((resolvePromise, rejectPromise) => {
    process.stdout.write(JSON.stringify(result), (error) => error ? rejectPromise(error) : resolvePromise());
  });
  // Exit releases only this CDP connection. It does not close the user's Edge browser.
  process.exit(0);
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  main().catch((error) => {
    const result: RunnerV3Result = {
      schema_version: "oceanengine-playwright-rpa-result/v2",
      outcome: "failed",
      error_code: "runner_failed",
      error_message: error instanceof Error ? error.message : String(error),
      final_click_performed: false,
      steps: [],
    };
    process.stdout.write(JSON.stringify(result));
    process.exitCode = 1;
  });
}
