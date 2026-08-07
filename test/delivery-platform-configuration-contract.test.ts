import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { join, resolve } from "node:path";
import test from "node:test";
import Ajv2020 from "ajv/dist/2020.js";

const root = resolve(import.meta.dirname, "..");
const schemaPath = join(root, "docs", "delivery", "schemas", "delivery-platform-configuration-v1.json");
const fixturePath = join(root, "docs", "delivery", "fixtures", "delivery-platform-configuration-v1-valid.json");
const invalidDirectory = join(root, "docs", "delivery", "fixtures");
const canonicalHashTool = join(root, "test", "delivery-platform-configuration-hash.go");
const jcsVectorsPath = join(root, "docs", "delivery", "fixtures", "delivery-platform-configuration-v1-jcs-vectors.json");

function readJSON(path: string): Record<string, unknown> {
  return JSON.parse(readFileSync(path, "utf8")) as Record<string, unknown>;
}

function setPath(target: Record<string, unknown>, path: string, value: unknown): void {
  const segments = path.split(".");
  let current: unknown = target;
  for (const segment of segments.slice(0, -1)) {
    current = (current as Record<string, unknown>)[segment];
  }
  const leaf = segments.at(-1)!;
  if (Array.isArray(current)) {
    current[Number(leaf)] = value;
  } else {
    (current as Record<string, unknown>)[leaf] = value;
  }
}

function fixtureWithMutations(fileName: string): Record<string, unknown> {
  const descriptor = readJSON(join(invalidDirectory, fileName));
  const candidate = structuredClone(readJSON(fixturePath));
  for (const mutation of descriptor.mutations as Array<{ path: string; value: unknown }>) {
    setPath(candidate, mutation.path, mutation.value);
  }
  return candidate;
}

function canonicalHash(payload: unknown): string {
  const output = execFileSync("go", ["run", canonicalHashTool], { input: JSON.stringify(payload), encoding: "utf8" });
  return output.trim();
}

const ajv = new Ajv2020({ allErrors: true, strict: true });
const validate = ajv.compile(readJSON(schemaPath));

test("delivery configuration schema resolves internal refs and validates the v1 fixture", () => {
  const fixture = readJSON(fixturePath);
  assert.equal(validate(fixture), true, JSON.stringify(validate.errors));
  const payload = fixture.payload as Record<string, unknown>;
  assert.equal(fixture.canonical_hash, canonicalHash(payload));
  assert.equal("canonical_hash" in payload, false);
  assert.equal("compilation_metadata" in payload, false);
});

test("compilation metadata is outside the hash projection", () => {
  const fixture = readJSON(fixturePath);
  const changed = structuredClone(fixture);
  (changed.compilation_metadata as Record<string, unknown>).evidence_states = ["platform_pending"];
  (changed.compilation_metadata as Record<string, unknown>).evidence_refs = ["fixture://changed-evidence"];
  assert.deepEqual(changed.payload, fixture.payload);
  assert.equal(canonicalHash(changed.payload), canonicalHash(fixture.payload));
});

test("invalid fixtures fail schema validation for the intended reason", () => {
  for (const fileName of [
    "delivery-platform-configuration-v1-invalid-hash.json",
    "delivery-platform-configuration-v1-invalid-metadata.json",
    "delivery-platform-configuration-v1-invalid-action-boundary.json",
    "delivery-platform-configuration-v1-invalid-redundant-parent.json",
    "delivery-platform-configuration-v1-invalid-resolved-reference.json",
    "delivery-platform-configuration-v1-invalid-condition-result.json"
  ]) {
    const descriptor = readJSON(join(invalidDirectory, fileName));
    const expectedError = descriptor.expected_error as string;
    const candidate = fixtureWithMutations(fileName);
    assert.equal(validate(candidate), false, fileName);
    assert.ok(
      validate.errors?.some((error) => error.message?.includes(expectedError)),
      `${fileName} expected error "${expectedError}" but got ${JSON.stringify(validate.errors)}`
    );
  }
});

test("blocked condition results are representable (result:false + blocked_reason)", () => {
  const candidate = structuredClone(readJSON(fixturePath));
  (candidate.compilation_metadata as Record<string, unknown>).conditions = [
    {
      condition: { field: "payload.platform_project.carrier", equals: "orange_landing_page" },
      result: false,
      evidence_states: ["observed"],
      blocked_reason: "owner must confirm carrier matches page context"
    }
  ];
  assert.equal(validate(candidate), true, JSON.stringify(validate.errors));
});

test("canonical hash is sensitive to payload field changes", () => {
  const fixture = readJSON(fixturePath);
  const changed = structuredClone(fixture);
  setPath(changed.payload as Record<string, unknown>, "platform_project.project_name", "mutated-project-name");
  assert.notEqual(canonicalHash(changed.payload), canonicalHash(fixture.payload as Record<string, unknown>));
});

test("canonical hash fixtures match production RFC8785 output", () => {
  const vectorFixture = readJSON(jcsVectorsPath);
  const vectors = vectorFixture.vectors as Array<{ name: string; payload: unknown; expected_canonical_hash: string }>;
  for (const vector of vectors) {
    assert.equal(canonicalHash(vector.payload), vector.expected_canonical_hash, vector.name);
  }
});
