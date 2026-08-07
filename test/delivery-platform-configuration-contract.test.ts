import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { join, resolve } from "node:path";
import test from "node:test";
import Ajv2020 from "ajv/dist/2020.js";

const root = resolve(import.meta.dirname, "..");
const schemaPath = join(root, "docs", "delivery", "schemas", "delivery-platform-configuration-v1.json");
const fixturePath = join(root, "docs", "delivery", "fixtures", "delivery-platform-configuration-v1-valid.json");
const invalidDirectory = join(root, "docs", "delivery", "fixtures");

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

function canonicalJSON(value: unknown): string {
  if (value === null || typeof value !== "object") return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(canonicalJSON).join(",")}]`;
  const object = value as Record<string, unknown>;
  return `{${Object.keys(object).sort().map((key) => `${JSON.stringify(key)}:${canonicalJSON(object[key])}`).join(",")}}`;
}

const ajv = new Ajv2020({ allErrors: true, strict: true });
const validate = ajv.compile(readJSON(schemaPath));

test("delivery configuration schema resolves internal refs and validates the v1 fixture", () => {
  const fixture = readJSON(fixturePath);
  assert.equal(validate(fixture), true, JSON.stringify(validate.errors));
  const payload = fixture.payload as Record<string, unknown>;
  const expectedHash = createHash("sha256").update(canonicalJSON(payload)).digest("hex");
  assert.equal(fixture.canonical_hash, expectedHash);
  assert.equal("canonical_hash" in payload, false);
  assert.equal("compilation_metadata" in payload, false);
});

test("compilation metadata is outside the hash projection", () => {
  const fixture = readJSON(fixturePath);
  const changed = structuredClone(fixture);
  (changed.compilation_metadata as Record<string, unknown>).evidence_states = ["platform_pending"];
  (changed.compilation_metadata as Record<string, unknown>).evidence_refs = ["fixture://changed-evidence"];
  assert.deepEqual(changed.payload, fixture.payload);
  assert.equal(canonicalJSON(changed.payload), canonicalJSON(fixture.payload));
});

test("invalid fixtures fail schema validation and parent refs are checked within the payload", () => {
  for (const fileName of [
    "delivery-platform-configuration-v1-invalid-hash.json",
    "delivery-platform-configuration-v1-invalid-metadata.json",
  ]) {
    const candidate = fixtureWithMutations(fileName);
    assert.equal(validate(candidate), false, fileName);
  }

  const orphanCandidate = fixtureWithMutations("delivery-platform-configuration-v1-invalid-reference.json");
  assert.equal(validate(orphanCandidate), true, JSON.stringify(validate.errors));
  const payload = orphanCandidate.payload as Record<string, unknown>;
  const project = payload.platform_project as Record<string, unknown>;
  const promotions = payload.platform_promotions as Array<Record<string, unknown>>;
  assert.equal(promotions.some((promotion) => promotion.parent_project_draft_id === project.project_draft_id), false);
});
