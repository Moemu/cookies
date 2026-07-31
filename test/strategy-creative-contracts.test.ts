import assert from "node:assert/strict";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import test from "node:test";
import Ajv2020, { type ValidateFunction } from "ajv/dist/2020.js";
import addFormats from "ajv-formats";
import {
  CREATIVE_CONTRACT_VERSIONS,
  CREATIVE_ROUTE_PROFILES,
  CREATIVE_SHARED_WORKFLOW_VERSION,
  CREATIVE_STATE_MACHINE,
  canTransitionCreativeState,
} from "../src/contracts/creative.js";

const repositoryRoot = resolve(import.meta.dirname, "..");
const contractsDirectory = join(repositoryRoot, "api", "contracts");
const fixturesDirectory = join(repositoryRoot, "api", "fixtures");
const openAPIDirectory = join(repositoryRoot, "api", "openapi");

const fixtureContracts = [
  ["strategy-creative-handoff-v1-ready.json", "strategy-creative-handoff-v1.schema.json"],
  ["strategy-creative-handoff-v1-blocked.json", "strategy-creative-handoff-v1.schema.json"],
  ["creative-intake-create-v2.json", "creative-intake-create-v2.schema.json"],
  ["creative-intake-v2-ready.json", "creative-intake-v2.schema.json"],
  ["strategy-creative-task-plan-v2-ready.json", "strategy-creative-task-plan-v2.schema.json"],
  ["strategy-creative-task-strategy-v2-ready.json", "strategy-creative-task-strategy-v2.schema.json"],
  ["strategy-creative-task-overlay-v1-ready.json", "strategy-creative-task-overlay-v1.schema.json"],
  ["creative-intake-create-v3-base.json", "creative-intake-create-v3.schema.json"],
  ["creative-intake-create-v3-overlay.json", "creative-intake-create-v3.schema.json"],
  ["creative-intake-create-v3-overlay-mismatch.json", "creative-intake-create-v3.schema.json"],
  ["creative-intake-v3-ready.json", "creative-intake-v3.schema.json"],
  ["creative-planning-context-v1-enhanced.json", "creative-planning-context-v1.schema.json"],
  ["creative-direction-v1-candidate.json", "creative-direction-v1.schema.json"],
  ["creative-direction-candidate-batch-v1-ready.json", "creative-direction-candidate-batch-v1.schema.json"],
  ["creative-direction-candidate-batch-v1-failed.json", "creative-direction-candidate-batch-v1.schema.json"],
  ["creative-shared-workflow-v1-frozen.json", "creative-shared-workflow-v1.schema.json"],
] as const;

const ajv = new Ajv2020({
  allErrors: true,
  allowUnionTypes: true,
  strict: true,
  strictRequired: false,
});
addFormats(ajv);

for (const filename of readdirSync(contractsDirectory).filter((name) => name.endsWith(".schema.json"))) {
  ajv.addSchema(readJSON(join(contractsDirectory, filename)));
}

function readJSON(path: string): Record<string, unknown> {
  return JSON.parse(readFileSync(path, "utf8")) as Record<string, unknown>;
}

function validatorFor(schemaFilename: string): ValidateFunction {
  const schema = readJSON(join(contractsDirectory, schemaFilename));
  const validator = ajv.getSchema(String(schema.$id));
  assert.ok(validator, `schema ${schemaFilename} was not registered`);
  return validator;
}

function assertValidFixture(fixtureFilename: string, schemaFilename: string): void {
  const fixture = readJSON(join(fixturesDirectory, fixtureFilename));
  const validate = validatorFor(schemaFilename);
  assert.equal(
    validate(fixture),
    true,
    `${fixtureFilename} does not satisfy ${schemaFilename}: ${ajv.errorsText(validate.errors, { separator: "\n" })}`,
  );
}

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

test("frozen Strategy-to-Creative fixtures satisfy their declared JSON Schemas", () => {
  for (const [fixture, schema] of fixtureContracts) {
    assertValidFixture(fixture, schema);
  }
});

test("strict contracts reject additional properties, missing route lineage, and creative leakage", () => {
  const create = readJSON(join(fixturesDirectory, "creative-intake-create-v3-base.json"));
  const validateCreate = validatorFor("creative-intake-create-v3.schema.json");

  const additionalProperty = { ...create, route_index: 0 };
  assert.equal(validateCreate(additionalProperty), false, "v3 create accepted route_index");

  const missingRoute = clone(create);
  delete missingRoute.selected_route_id;
  assert.equal(validateCreate(missingRoute), false, "v3 create accepted missing selected_route_id");

  const overlay = readJSON(join(fixturesDirectory, "strategy-creative-task-overlay-v1-ready.json"));
  const validateOverlay = validatorFor("strategy-creative-task-overlay-v1.schema.json");
  assert.equal(
    validateOverlay({ ...overlay, concept: "must remain Creative-owned" }),
    false,
    "overlay accepted a Creative-owned concept",
  );

  const taskStrategy = readJSON(join(fixturesDirectory, "strategy-creative-task-strategy-v2-ready.json"));
  const validateTaskStrategy = validatorFor("strategy-creative-task-strategy-v2.schema.json");
  const leakingTaskStrategy = clone(taskStrategy);
  leakingTaskStrategy.business_strategy = {
    ...(leakingTaskStrategy.business_strategy as Record<string, unknown>),
    hook: "must remain Creative-owned",
  };
  assert.equal(
    validateTaskStrategy(leakingTaskStrategy),
    false,
    "task strategy accepted a Creative-owned hook",
  );
});

test("the mismatch fixture is structurally valid but carries a deliberate route-lineage conflict", () => {
  const mismatch = readJSON(join(fixturesDirectory, "creative-intake-create-v3-overlay-mismatch.json"));
  const overlay = readJSON(join(fixturesDirectory, "strategy-creative-task-overlay-v1-ready.json"));
  const validate = validatorFor("creative-intake-create-v3.schema.json");

  assert.equal(validate(mismatch), true, ajv.errorsText(validate.errors));
  assert.notEqual(mismatch.selected_route_id, overlay.selected_route_id);
});

test("the frontend consumes the frozen Creative contract versions and state graph", () => {
  const frozen = readJSON(join(fixturesDirectory, "creative-shared-workflow-v1-frozen.json"));

  assert.equal(frozen.contract_version, CREATIVE_SHARED_WORKFLOW_VERSION);
  assert.deepEqual(frozen.contracts, CREATIVE_CONTRACT_VERSIONS);
  assert.deepEqual(frozen.route_profiles, CREATIVE_ROUTE_PROFILES);
  assert.deepEqual(frozen.states, CREATIVE_STATE_MACHINE);

  assert.equal(canTransitionCreativeState("intake", "ready", "superseded"), true);
  assert.equal(canTransitionCreativeState("direction", "confirmed", "candidate"), false);
  assert.equal(canTransitionCreativeState("task", "archived", "draft"), false);
  assert.equal(canTransitionCreativeState("creative_version", "checked", "approved"), true);
});

test("every OpenAPI contract reference resolves to a checked-in schema", () => {
  const missing: string[] = [];
  const referencePattern = /\$ref:\s*['"]?([^'"\s}]+)/g;

  for (const filename of readdirSync(openAPIDirectory).filter((name) => name.endsWith(".yaml"))) {
    const openAPIPath = join(openAPIDirectory, filename);
    const document = readFileSync(openAPIPath, "utf8");
    for (const match of document.matchAll(referencePattern)) {
      const reference = match[1];
      if (!reference.startsWith("../contracts/")) continue;
      const schemaPath = resolve(dirname(openAPIPath), reference.split("#", 1)[0]);
      if (!existsSync(schemaPath)) missing.push(`${filename}: ${reference}`);
    }
  }

  assert.deepEqual(missing, []);
});
