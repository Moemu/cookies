import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";
import test from "node:test";
import Ajv2020 from "ajv/dist/2020.js";
import addFormats from "ajv-formats";

const root = resolve(import.meta.dirname, "..");
const contracts = join(root, "api", "contracts");
const fixtures = join(root, "api", "fixtures");
const ajv = new Ajv2020({ allErrors: true, strict: true });
addFormats(ajv);

function readJSON(path: string): Record<string, unknown> {
  return JSON.parse(readFileSync(path, "utf8")) as Record<string, unknown>;
}

for (const schemaName of ["platform-computer-use-run-v1.schema.json", "delivery-controlled-change-set-v1.schema.json", "delivery-platform-skill-definition-v1.schema.json"]) {
  ajv.addSchema(readJSON(join(contracts, schemaName)));
}

test("controlled Computer Use fixtures satisfy the frozen contracts", () => {
  for (const [schemaName, fixtureName] of [
    ["platform-computer-use-run-v1.schema.json", "platform-computer-use-run-v1-awaiting-confirmation.json"],
    ["delivery-controlled-change-set-v1.schema.json", "delivery-controlled-change-set-v1-ready.json"],
  ] as const) {
    const schema = readJSON(join(contracts, schemaName));
    const validate = ajv.getSchema(String(schema.$id));
    assert.ok(validate, `missing validator for ${schemaName}`);
    assert.equal(validate(readJSON(join(fixtures, fixtureName))), true, ajv.errorsText(validate.errors));
  }
});

test("stage B Skill calibration is bound and partial Skill identity is rejected", () => {
  for (const [schemaName, fixtureName] of [
    ["platform-computer-use-run-v1.schema.json", "platform-computer-use-run-v1-awaiting-confirmation.json"],
    ["delivery-controlled-change-set-v1.schema.json", "delivery-controlled-change-set-v1-ready.json"],
  ] as const) {
    const schema = readJSON(join(contracts, schemaName));
    const validate = ajv.getSchema(String(schema.$id));
    assert.ok(validate);
    const fixture = readJSON(join(fixtures, fixtureName));
    const binding = (schemaName.startsWith("platform") ? fixture.authority : fixture.binding) as Record<string, unknown>;
    assert.equal(binding.skill_id, "oceanengine-ecommerce-manual");
    assert.equal(binding.skill_version, "v0.1-calibration");
    delete binding.skill_version;
    assert.equal(validate(fixture), false);
  }
});

test("OceanEngine SkillDefinition records passed gate-one takeover calibration without enabling execution", () => {
  const definitionPath = join(root, "internal", "systems", "delivery", "platformskills", "definitions", "oceanengine-ecommerce-manual-v0.1.json");
  const definition = readJSON(definitionPath);
  const schema = readJSON(join(contracts, "delivery-platform-skill-definition-v1.schema.json"));
  const validate = ajv.getSchema(String(schema.$id));
  assert.ok(validate);
  assert.equal(validate(definition), true, ajv.errorsText(validate.errors));
  assert.equal(definition.executable, false);
  assert.equal(definition.real_browser_driver, false);
  assert.equal(definition.submit_allowed, false);
  assert.equal(definition.status, "gate_one_passed_takeover_calibration");
  assert.equal((definition.gate_one as Record<string, unknown>).ready, true);
  assert.equal((definition.gate_one as Record<string, unknown>).result, "passed");
  assert.equal((definition.gate_one as Record<string, unknown>).project_form_status, "passed");
  assert.equal((definition.gate_one as Record<string, unknown>).promotion_form_status, "passed");
  assert.equal((definition.ui_baseline as Record<string, unknown>).observed_at, "2026-08-06");
  assert.equal((definition.ui_baseline as Record<string, unknown>).revalidated_at, "2026-08-13");
  assert.equal(
    (definition.ui_baseline as Record<string, unknown>).drift_check,
    "promotion_configuration_branches_revalidated",
  );
  for (const reference of [definition.schema_ref, ...(definition.evidence_refs as string[])]) {
    assert.equal(existsSync(join(root, String(reference))), true, `missing stage B evidence ${reference}`);
  }
});

test("PR 50 preserves passed takeover calibration without claiming an installed automated Browser Driver", () => {
  assert.equal(existsSync(join(root, "internal", "systems", "delivery", "oceanengineskill", "gate_one.go")), false);
  const definition = readFileSync(join(root, "internal", "systems", "delivery", "platformskills", "definitions", "oceanengine-ecommerce-manual-v0.1.json"), "utf8");
  assert.match(definition, /"locator_contract": "project_and_promotion_forms_live_dom"/);
  assert.match(definition, /"promotion_form_live_calibrated": true/);
  assert.match(definition, /"control_plane_evidence_recorded": true/);
  assert.match(definition, /"agent_final_submit_documentation": "deferred_until_end_to_end_flow_validated"/);
});

test("live gate-one evidence proves project-form readback and no write without persisting raw account identity", () => {
  const evidencePath = join(root, "docs", "delivery", "evidence", "oceanengine-gate-one-project-form-2026-08-12.json");
  const rawEvidence = readFileSync(evidencePath, "utf8");
  const evidence = JSON.parse(rawEvidence) as Record<string, unknown>;
  assert.doesNotMatch(rawEvidence, /\b\d{16}\b/);
  assert.equal(evidence.gate_one_result, "partial");
  assert.equal(evidence.real_browser_driver_calibrated, false);
  assert.equal(evidence.submit_allowed, false);
  const projectForm = evidence.project_form as Record<string, unknown>;
  assert.equal(projectForm.status, "passed");
  assert.equal(projectForm.starting_project_count, 163);
  assert.equal(projectForm.ending_project_count, 163);
  assert.deepEqual(projectForm.write_boundaries_clicked, []);
  assert.equal(projectForm.remote_side_effect_detected, false);
  for (const field of projectForm.fields as Array<Record<string, unknown>>) {
    assert.equal(field.matched, true, `unmatched live readback for ${String(field.field)}`);
  }
  const promotionForm = evidence.promotion_form as Record<string, unknown>;
  assert.equal(promotionForm.status, "pending");
  assert.equal(promotionForm.write_boundary_crossed, false);
  assert.equal((evidence.control_plane_recording as Record<string, unknown>).status, "pending");
});

test("live locator baseline has semantic write boundaries and no coordinate fallback", () => {
  const locatorPath = join(root, "docs", "delivery", "fixtures", "oceanengine-ecommerce-manual-live-locators-v0.1.json");
  const locators = readJSON(locatorPath);
  assert.equal(locators.status, "project_form_calibrated_promotion_form_pending");
  assert.equal(locators.coordinate_fallback_allowed, false);
  const locatorMap = locators.locators as Record<string, Record<string, unknown>>;
  assert.equal(locatorMap.save_and_new_promotion_boundary.action_allowed, false);
  assert.equal(locatorMap.save_and_close_boundary.action_allowed, false);
  assert.equal(locatorMap.discard_project_draft.name, "取消");
  assert.ok((locators.known_gaps as string[]).includes("promotion_create_live_locators_not_revalidated"));
  assert.equal(existsSync(join(root, "internal", "systems", "delivery", "platformskills", "SKILL.md")), false);
});

test("gate-one replay preparation is non-write and uses the exact takeover evidence sequence", () => {
  const planPath = join(root, "docs", "delivery", "fixtures", "oceanengine-gate-one-replay-plan-v0.1.json");
  const rawPlan = readFileSync(planPath, "utf8");
  const plan = JSON.parse(rawPlan) as Record<string, unknown>;
  assert.equal(plan.status, "completed_no_write");
  const liveForm = plan.live_form_calibration as Record<string, unknown>;
  assert.equal(liveForm.project_form, "passed");
  assert.equal(liveForm.promotion_form, "passed");
  assert.equal(liveForm.control_plane_evidence, "passed");
  assert.equal(plan.remote_write_allowed, false);
  assert.equal(plan.final_confirmation_allowed, false);
  assert.equal(plan.automated_prepare_allowed, false);
  assert.equal(plan.automated_submit_allowed, false);
  assert.equal((plan.server_resolved_authority as Record<string, unknown>).client_authority_json_allowed, false);
  assert.deepEqual(
    (plan.takeover_sequence as Array<Record<string, unknown>>).map((step) => step.action),
    ["observe_page", "begin_form_fill", "field_readback", "discard_draft", "verify_no_write"],
  );
  assert.ok((plan.stop_conditions as string[]).includes("write_boundary_required"));
  assert.ok((plan.forbidden_actions as string[]).includes("submit"));
  assert.doesNotMatch(rawPlan, /\b\d{16}\b/);
});

test("promotion locator capture remains an empty template rather than fabricated evidence", () => {
  const templatePath = join(root, "docs", "delivery", "fixtures", "oceanengine-promotion-live-locator-capture-v0.1-template.json");
  const template = readJSON(templatePath);
  assert.equal(template.status, "template_not_observed");
  assert.equal(template.evidence, false);
  assert.equal(template.observed_at, null);
  assert.equal(template.coordinate_fallback_allowed, false);
  assert.deepEqual(template.captured_locators, []);
  assert.deepEqual(template.dynamic_behaviors, []);
  assert.equal((template.page_identity as Record<string, unknown>).observed, false);
  assert.equal((template.write_boundary as Record<string, unknown>).action_allowed, false);
  assert.equal((template.safe_exit as Record<string, unknown>).observed, false);
  assert.ok((template.fields_to_capture as string[]).includes("promotion_name"));
  assert.ok((template.selector_surfaces_to_capture as string[]).includes("landing_page_hybrid"));
});

test("conservative promotion drift stop is retained as a corrected configuration-branch audit record", () => {
  const evidencePath = join(
    root,
    "docs",
    "delivery",
    "evidence",
    "oceanengine-gate-one-promotion-drift-2026-08-13.json",
  );
  const rawEvidence = readFileSync(evidencePath, "utf8");
  const evidence = JSON.parse(rawEvidence) as Record<string, unknown>;
  assert.doesNotMatch(rawEvidence, /\b\d{16}\b/);
  assert.equal(evidence.status, "conservative_stop_reclassified_as_configuration_branch");
  assert.equal(evidence.original_stop_reason, "PAGE_DRIFT");
  assert.equal(evidence.gate_one_result, "partial");
  assert.equal(evidence.real_browser_driver_calibrated, false);
  assert.equal(evidence.submit_allowed, false);
  const actions = evidence.actions as Record<string, unknown>;
  assert.equal(actions.form_fill_started, false);
  assert.equal(actions.field_values_changed, false);
  assert.deepEqual(actions.write_boundaries_clicked, []);
  assert.equal(actions.remote_side_effect_detected, false);
  const reclassification = evidence.reclassification as Record<string, unknown>;
  assert.equal(reclassification.landing_page_label, "parent_project_delivery_carrier_branch");
  assert.equal(reclassification.material_capacity, "conditional_platform_limit_not_frozen_constant");
});

test("promotion form pass proves semantic readback, fenced evidence, safe exit, and exact no-write search", () => {
  const evidencePath = join(
    root,
    "docs",
    "delivery",
    "evidence",
    "oceanengine-gate-one-promotion-form-2026-08-13.json",
  );
  const rawEvidence = readFileSync(evidencePath, "utf8");
  const evidence = JSON.parse(rawEvidence) as Record<string, unknown>;
  assert.doesNotMatch(rawEvidence, /\b\d{16}\b/);
  assert.equal(evidence.gate_one_result, "passed");
  assert.equal(evidence.real_browser_driver_calibrated, true);
  assert.equal(evidence.submit_allowed, false);
  const promotionForm = evidence.promotion_form as Record<string, unknown>;
  assert.equal(promotionForm.status, "passed");
  assert.equal(promotionForm.safe_exit_exercised, true);
  assert.deepEqual(promotionForm.write_boundaries_clicked, []);
  for (const field of promotionForm.approved_field_readback as Array<Record<string, unknown>>) {
    assert.equal(field.matched, true, `unmatched promotion readback for ${String(field.field)}`);
  }
  assert.ok((promotionForm.approved_field_readback as Array<Record<string, unknown>>).some((field) => field.field === "base_material_ref"));
  assert.ok((promotionForm.approved_field_readback as Array<Record<string, unknown>>).some((field) => field.field === "landing_page_ref"));
  const noWrite = evidence.no_write_verification as Record<string, unknown>;
  assert.equal(noWrite.exact_temporary_name_search_executed, true);
  assert.equal(noWrite.matching_platform_objects, 0);
  assert.equal(noWrite.remote_side_effect_detected, false);
  const recording = evidence.control_plane_recording as Record<string, unknown>;
  assert.equal(recording.status, "passed");
  assert.equal(recording.lease_released, true);
  assert.equal(recording.run_final_state, "cancelled");
  assert.equal(recording.final_confirmation_issued, false);
  assert.deepEqual(recording.recorded_actions, ["observe_page", "begin_form_fill", "field_readback", "discard_draft", "verify_no_write"]);
});

test("promotion live locators use stable semantics and retain the write boundary", () => {
  const locatorPath = join(
    root,
    "docs",
    "delivery",
    "fixtures",
    "oceanengine-promotion-live-locators-v0.1.json",
  );
  const rawLocators = readFileSync(locatorPath, "utf8");
  const locators = JSON.parse(rawLocators) as Record<string, unknown>;
  assert.equal(locators.status, "live_form_calibrated");
  assert.equal(locators.coordinate_fallback_allowed, false);
  assert.doesNotMatch(rawLocators, /\b\d{16}\b/);
  const captured = locators.captured_locators as Array<Record<string, unknown>>;
  const accountInfo = captured.find((locator) => locator.field === "delivery_identity.account_info");
  assert.equal(accountInfo?.selector, "[data-e2e='createad_nativetype_0']");
  assert.equal(accountInfo?.selected_state, "class_contains_ovui-radio-item--checked");
  assert.ok(captured.some((locator) => locator.field === "safe_exit" && locator.action_allowed === true));
  assert.ok(captured.some((locator) => locator.field === "final_write_boundary" && locator.action_allowed === false));
  assert.ok((locators.dynamic_behaviors as string[]).includes("copy_library_auto_preselects_one_recommended_title_on_open"));
  const crossBranch = locators.cross_branch_observations as Array<Record<string, unknown>>;
  assert.ok(crossBranch.some((observation) => observation.delivery_carrier === "orange_landing_page" && observation.remote_side_effect_detected === false));
  assert.equal((locators.reference_configuration as Record<string, unknown>).status, "complete_for_observed_unsubmitted_form");
  assert.deepEqual(locators.reference_gaps, []);
});

test("gate-two preflight is frozen without granting or mounting final submit", () => {
  const schemaPath = join(root, "docs", "delivery", "schemas", "oceanengine-gate-two-preflight-v0.1.json");
  const fixturePath = join(root, "docs", "delivery", "fixtures", "oceanengine-gate-two-preflight-v0.1.json");
  const rawFixture = readFileSync(fixturePath, "utf8");
  const schema = readJSON(schemaPath);
  const fixture = JSON.parse(rawFixture) as Record<string, unknown>;
  const validate = ajv.compile(schema);
  assert.equal(validate(fixture), true, ajv.errorsText(validate.errors));
  assert.equal(fixture.execution_authorized, false);
  assert.equal(fixture.final_confirmation_issued, false);
  assert.equal(fixture.production_submit_port_mounted, false);
  assert.equal(fixture.skill_submit_allowed, false);
  assert.equal(fixture.maximum_final_clicks, 1);
  assert.equal(fixture.result_unknown_policy, "query_or_takeover_never_resubmit");
  assert.equal(fixture.mapping_confirmation_policy, "result_page_and_list_page_ids_and_statuses_must_match");
  assert.ok((fixture.implementation_gaps as string[]).includes("takeover_remote_write_authorization_port"));
  assert.doesNotMatch(rawFixture, /\b\d{16}\b/);
});

test("run-time blocks cannot reuse the Phase C compile-time prohibition", () => {
  const schema = readJSON(join(contracts, "platform-computer-use-run-v1.schema.json"));
  const validate = ajv.getSchema(String(schema.$id));
  assert.ok(validate);
  const fixture = readJSON(join(fixtures, "platform-computer-use-run-v1-awaiting-confirmation.json"));
  fixture.blocking_reason = "PHASE_C_REMOTE_WRITE_PROHIBITED";
  assert.equal(validate(fixture), false);
});

test("historical workflow and observatory contracts remain write-disabled", () => {
  const workflow = readJSON(join(root, "docs", "delivery", "schemas", "compiled-delivery-workflow-v1.json"));
  const observatory = readJSON(join(root, "docs", "delivery", "schemas", "delivery-observatory-run-v1.json"));
  const workflowProperties = workflow.properties as Record<string, Record<string, unknown>>;
  const observatoryProperties = observatory.properties as Record<string, Record<string, unknown>>;
  assert.equal(workflowProperties.remote_write_enabled.const, false);
  assert.equal(observatoryProperties.remote_write_enabled.const, false);
});
