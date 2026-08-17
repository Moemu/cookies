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
  condition_vocabulary: Array<{
    key: string;
    source_kind: string;
    source: string;
    value_kind: string;
    known_values?: string[];
    unknown_value_treatment: string;
  }>;
  fields: Array<{
    key: string;
    consumers: string[];
    condition?: string;
    condition_dimensions?: string[];
    condition_state?: "evaluable" | "dependency_only";
    condition_rule?: {
      all: Array<{ dimension: string; operator: string; values?: string[] }>;
    };
    evidence_state: string;
    computer_use?: {
      operation: string;
      expected_target_count?: number;
      blocked_state?: string;
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
  const vocabularyKeys = new Set(
    manifest.condition_vocabulary.map((item) => item.key),
  );
  assert.equal(vocabularyKeys.size, manifest.condition_vocabulary.length);
  for (const item of manifest.condition_vocabulary) {
    assert.equal(item.unknown_value_treatment, "platform_pending");
    assert.doesNotMatch(item.source, /display.?name.?snapshot/i);
  }
  for (const oldName of [
    "lead_mode",
    "scenario",
    "optimization_target",
    "product_or_application",
  ]) {
    assert.ok(
      !vocabularyKeys.has(oldName),
      `non-standard condition name: ${oldName}`,
    );
  }
  const expectedConditionDimensions = new Map<string, string[]>([
    ["project.marketing_scenario", ["marketing_purpose"]],
    ["project.marketing_product_reference", ["marketing_purpose"]],
    ["project.application_reference", ["marketing_purpose"]],
    [
      "project.optimization_target_reference",
      [
        "marketing_purpose",
        "carrier",
        "marketing_product_reference",
        "application_reference",
      ],
    ],
    ["project.targeting", ["delivery_mode"]],
    ["project.monitoring_references", ["optimization_target_reference"]],
    ["project.product_catalog_reference", ["marketing_purpose"]],
    ["project.placement_strategy", ["marketing_purpose"]],
    ["project.product_targeting", ["marketing_purpose"]],
    [
      "project.application_launch_mode",
      ["marketing_purpose", "application_scenario"],
    ],
  ]);
  for (const [fieldKey, dimensions] of expectedConditionDimensions) {
    const field = manifest.fields.find((item) => item.key === fieldKey);
    assert.deepEqual(field?.condition_dimensions, dimensions, fieldKey);
  }
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
    if (field.condition) {
      assert.ok(field.condition_dimensions?.length);
      for (const dimension of field.condition_dimensions ?? [])
        assert.ok(vocabularyKeys.has(dimension));
      if (field.condition_state === "dependency_only") {
        assert.equal(field.evidence_state, "platform_pending");
        assert.equal(field.computer_use?.operation, "no_action");
        assert.equal(field.computer_use?.blocked_state, "platform_pending");
        assert.equal(field.condition_rule, undefined);
      } else {
        assert.equal(field.condition_state, "evaluable");
        assert.ok(field.condition_rule?.all.length);
        for (const predicate of field.condition_rule?.all ?? []) {
          assert.ok(field.condition_dimensions?.includes(predicate.dimension));
          const vocabularyItem = manifest.condition_vocabulary.find(
            (item) => item.key === predicate.dimension,
          );
          if (vocabularyItem?.value_kind === "reference")
            assert.ok(["present", "absent"].includes(predicate.operator));
        }
      }
    }
    if (field.computer_use?.operation !== "no_action")
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
  const writeCalibrationEvidencePath = join(
    root,
    "docs/delivery/evidence/oceanengine-d3-write-calibration-2026-08-16.json",
  );
  assert.ok(existsSync(writeCalibrationEvidencePath));
  const writeCalibrationEvidence = JSON.parse(
    readFileSync(writeCalibrationEvidencePath, "utf8"),
  ) as { operations: Array<{ case_id: string; outcome: string }> };
  for (const operation of writeCalibrationEvidence.operations)
    evidenceByCase.set(operation.case_id, operation.outcome);
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
