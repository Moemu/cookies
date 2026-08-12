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

for (const schemaName of ["platform-computer-use-run-v1.schema.json", "delivery-controlled-change-set-v1.schema.json"]) {
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

test("unregistered Platform Skill is absent and partial Skill identity is rejected", () => {
  for (const [schemaName, fixtureName] of [
    ["platform-computer-use-run-v1.schema.json", "platform-computer-use-run-v1-awaiting-confirmation.json"],
    ["delivery-controlled-change-set-v1.schema.json", "delivery-controlled-change-set-v1-ready.json"],
  ] as const) {
    const schema = readJSON(join(contracts, schemaName));
    const validate = ajv.getSchema(String(schema.$id));
    assert.ok(validate);
    const fixture = readJSON(join(fixtures, fixtureName));
    const binding = (schemaName.startsWith("platform") ? fixture.authority : fixture.binding) as Record<string, unknown>;
    assert.equal(binding.skill_id, undefined);
    assert.equal(binding.skill_version, undefined);
    binding.skill_id = "unregistered-placeholder";
    assert.equal(validate(fixture), false);
  }
});

test("PR 50 does not self-register an OceanEngine Platform Skill", () => {
  const prd = readFileSync(join(root, "docs", "04-intelligent-delivery-prd.md"), "utf8");
  for (const skill of ["douyin-delivery", "kuaishou-delivery", "delivery-preflight"]) {
    assert.match(prd, new RegExp(`\\b${skill}\\b`));
  }
  assert.equal(existsSync(join(root, "internal", "systems", "delivery", "oceanengineskill", "gate_one.go")), false);
  const compiler = readFileSync(join(root, "internal", "systems", "delivery", "controlled_authority_service.go"), "utf8");
  assert.doesNotMatch(compiler, /oceanengine-ecommerce-manual/);
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
