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

test("OceanEngine SkillDefinition binds stage B evidence without claiming Browser Driver readiness", () => {
  const definitionPath = join(root, "internal", "systems", "delivery", "platformskills", "definitions", "oceanengine-ecommerce-manual-v0.1.json");
  const definition = readJSON(definitionPath);
  const schema = readJSON(join(contracts, "delivery-platform-skill-definition-v1.schema.json"));
  const validate = ajv.getSchema(String(schema.$id));
  assert.ok(validate);
  assert.equal(validate(definition), true, ajv.errorsText(validate.errors));
  assert.equal(definition.executable, false);
  assert.equal(definition.real_browser_driver, false);
  assert.equal(definition.submit_allowed, false);
  assert.equal(definition.status, "gate_one_partial_live_calibration");
  assert.equal((definition.gate_one as Record<string, unknown>).ready, false);
  assert.equal((definition.gate_one as Record<string, unknown>).result, "partial");
  assert.equal((definition.gate_one as Record<string, unknown>).project_form_status, "passed");
  assert.equal((definition.gate_one as Record<string, unknown>).promotion_form_status, "pending");
  assert.equal((definition.ui_baseline as Record<string, unknown>).observed_at, "2026-08-06");
  assert.equal((definition.ui_baseline as Record<string, unknown>).revalidated_at, "2026-08-12");
  for (const reference of [definition.schema_ref, ...(definition.evidence_refs as string[])]) {
    assert.equal(existsSync(join(root, String(reference))), true, `missing stage B evidence ${reference}`);
  }
});

test("PR 50 preserves partial live calibration without claiming a complete real Browser Driver", () => {
  assert.equal(existsSync(join(root, "internal", "systems", "delivery", "oceanengineskill", "gate_one.go")), false);
  const definition = readFileSync(join(root, "internal", "systems", "delivery", "platformskills", "definitions", "oceanengine-ecommerce-manual-v0.1.json"), "utf8");
  assert.match(definition, /"locator_contract": "project_form_live_dom_partial"/);
  assert.match(definition, /"promotion_form_live_calibrated": false/);
  assert.match(definition, /"control_plane_evidence_recorded": false/);
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
