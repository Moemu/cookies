import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";
import test from "node:test";

const root = resolve(import.meta.dirname, "..");
const manifest = JSON.parse(
  readFileSync(
    join(
      root,
      "docs/delivery/fixtures/oceanengine-calibration-manifest-v1.json",
    ),
    "utf8",
  ),
) as {
  session_evidence_ref: string;
  path_dimensions: Array<{ key: string; observed_values: string[] }>;
  fields: Array<{
    key: string;
    consumers: string[];
    computer_use?: {
      operation: string;
      expected_target_count: number;
      observed_options?: string[];
    };
  }>;
  coverage_cases: Array<{
    id: string;
    path: string[];
    field_keys: string[];
    status: string;
  }>;
  consumer_mappings: Array<{
    field_key: string;
    destination: string;
    treatment: string;
    contract_path: string;
  }>;
};

test("OceanEngine calibration manifest drives consumer and coverage checks", () => {
  assert.ok(existsSync(join(root, manifest.session_evidence_ref)));
  const fieldKeys = new Set(manifest.fields.map((field) => field.key));
  const consumers = new Set<string>();
  const mappedPairs = new Set<string>();
  for (const mapping of manifest.consumer_mappings) {
    assert.ok(
      fieldKeys.has(mapping.field_key),
      `unknown manifest field: ${mapping.field_key}`,
    );
    assert.ok(
      mapping.contract_path.length > 0,
      `missing contract path: ${mapping.field_key}:${mapping.destination}`,
    );
    if (mapping.treatment === "evidence_only") {
      assert.match(
        mapping.contract_path,
        /(?:CalibrationManifest|FieldEvidence|calibration_manifest)$/,
        `evidence-only mapping must not masquerade as an executable field: ${mapping.field_key}:${mapping.destination}`,
      );
    }
    consumers.add(mapping.destination);
    mappedPairs.add(`${mapping.field_key}:${mapping.destination}`);
  }
  for (const field of manifest.fields) {
    for (const consumer of field.consumers) {
      assert.ok(
        mappedPairs.has(`${field.key}:${consumer}`),
        `unconsumed manifest field: ${field.key}:${consumer}`,
      );
    }
  }
  const computerUseFields = manifest.fields.filter(
    (field) => field.computer_use,
  );
  assert.equal(
    computerUseFields.length,
    manifest.fields.length,
    "every Manifest field must be Computer Use-ready",
  );
  for (const field of computerUseFields) {
    assert.equal(
      field.computer_use?.expected_target_count,
      1,
      `non-unique Computer Use target: ${field.key}`,
    );
    assert.ok(
      field.computer_use?.operation.length,
      `missing Computer Use operation: ${field.key}`,
    );
    if (field.computer_use?.observed_options) {
      assert.equal(
        new Set(field.computer_use.observed_options).size,
        field.computer_use.observed_options.length,
        `duplicate Computer Use options: ${field.key}`,
      );
    }
  }
  for (const destination of [
    "DeliveryIntent",
    "OceanEngineConfiguration",
    "DeliveryDecisionCandidate",
    "CompiledDeliveryWorkflow",
    "PlatformSkill",
  ]) {
    assert.ok(
      consumers.has(destination),
      `missing manifest consumer: ${destination}`,
    );
  }
  assert.ok(
    manifest.coverage_cases.some(
      (item) => item.status === "blocked_by_event_asset",
    ),
  );
  assert.ok(
    manifest.coverage_cases.some(
      (item) => item.status === "blocked_by_platform_capability",
    ),
  );
  assert.ok(manifest.coverage_cases.every((item) => item.id.startsWith("OE-")));
  const paths = manifest.coverage_cases.flatMap((item) => item.path).join("|");
  for (const dimension of manifest.path_dimensions) {
    for (const value of dimension.observed_values)
      assert.ok(
        paths.includes(value),
        `uncovered path dimension: ${dimension.key}:${value}`,
      );
  }
  const caseFieldKeys = new Set(
    manifest.coverage_cases
      .filter((item) => item.status !== "not_in_scope")
      .flatMap((item) => item.field_keys),
  );
  for (const key of fieldKeys) {
    assert.ok(
      caseFieldKeys.has(key),
      `field has no covered or blocked case: ${key}`,
    );
  }
  const evidence = JSON.parse(
    readFileSync(join(root, manifest.session_evidence_ref), "utf8"),
  ) as { cases: Array<{ case_id: string; outcome: string }> };
  const evidenceByCase = new Map(
    evidence.cases.map((item) => [item.case_id, item.outcome]),
  );
  for (const item of manifest.coverage_cases.filter(
    (item) => item.status !== "not_in_scope",
  )) {
    assert.equal(
      evidenceByCase.get(item.id),
      item.status,
      `evidence outcome mismatch: ${item.id}`,
    );
  }
});
