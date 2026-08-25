import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import Ajv2020 from "ajv/dist/2020.js";

const root = resolve(import.meta.dirname, "..");
const readJSON = (relative) =>
  JSON.parse(readFileSync(resolve(root, relative), "utf8"));
const readText = (relative) => readFileSync(resolve(root, relative), "utf8");
const schema = readJSON(
  "docs/delivery/schemas/oceanengine-calibration-manifest-v1.json",
);
const manifest = readJSON(
  "docs/delivery/fixtures/oceanengine-calibration-manifest-v1.json",
);
const evidence = readJSON(manifest.session_evidence_ref);
const writeCalibrationEvidence = readJSON(
  "docs/delivery/evidence/oceanengine-d3-write-calibration-2026-08-24.json",
);
const skillInstructions = readText(
  "internal/systems/delivery/platformskills/skills/oceanengine-ecommerce-manual/SKILL.md",
);
const contractSource = [
  "internal/systems/delivery/platform_configuration_contracts.go",
  "internal/systems/delivery/decision_workflow.go",
  "internal/systems/delivery/platformskills/registry.go",
  "api/openapi/delivery-v1.yaml",
  "internal/systems/delivery/platformskills/definitions/oceanengine-ecommerce-manual-v0.1.json",
]
  .map(readText)
  .join("\n");
const validate = new Ajv2020({ allErrors: true, strict: false }).compile(
  schema,
);

if (!validate(manifest))
  throw new Error(
    `invalid calibration manifest: ${JSON.stringify(validate.errors)}`,
  );
if (manifest.observation_boundary.remote_write_authorized)
  throw new Error("calibration manifest must never authorize remote writes");
for (const requiredSkillTerm of [
  "playwright_rpa",
  "expected_target_count",
  "blocked_by_platform_capability",
  "page_drift",
]) {
  if (!skillInstructions.includes(requiredSkillTerm))
    throw new Error(
      `OceanEngine Skill does not consume Manifest form controls: ${requiredSkillTerm}`,
    );
}
if (
  !manifest.page_families.some(
    (page) =>
      page.page_kind === "project_create" && page.evidence_state === "observed",
  )
)
  throw new Error("manifest must contain observed project-create evidence");
if (
  !manifest.coverage_cases.some(
    (item) => item.status === "blocked_by_event_asset",
  )
)
  throw new Error("manifest must retain the observed event-asset block");
const unique = (values, description) => {
  if (new Set(values).size !== values.length)
    throw new Error(`manifest has duplicate ${description}`);
};
unique(
  manifest.page_families.map((page) => page.id),
  "page-family IDs",
);
unique(
  manifest.fields.map((field) => field.key),
  "field keys",
);
unique(
  manifest.coverage_cases.map((item) => item.id),
  "coverage-case IDs",
);
unique(
  manifest.condition_vocabulary.map((item) => item.key),
  "condition-vocabulary keys",
);
const forbiddenConditionNames = new Set([
  "lead_mode",
  "scenario",
  "optimization_target",
  "product_or_application",
]);
const conditionVocabulary = new Map(
  manifest.condition_vocabulary.map((item) => [item.key, item]),
);
for (const item of manifest.condition_vocabulary) {
  if (forbiddenConditionNames.has(item.key))
    throw new Error(
      `manifest contains a non-standard condition name: ${item.key}`,
    );
  if (item.unknown_value_treatment !== "platform_pending")
    throw new Error(
      `unknown condition values must be platform_pending: ${item.key}`,
    );
  if (/display.?name.?snapshot/i.test(item.source))
    throw new Error(
      `condition source cannot use a display-name snapshot: ${item.key}`,
    );
  if (item.known_values)
    unique(item.known_values, `known values for ${item.key}`);
}
for (const dimension of manifest.path_dimensions) {
  if (forbiddenConditionNames.has(dimension.key))
    throw new Error(
      `manifest contains a non-standard path name: ${dimension.key}`,
    );
}
const manifestFieldKeys = new Set(manifest.fields.map((field) => field.key));
const caseObservedFieldKeys = new Set();
for (const item of manifest.coverage_cases) {
  unique(item.field_keys, `field keys for ${item.id}`);
  for (const fieldKey of item.field_keys) {
    if (!manifestFieldKeys.has(fieldKey))
      throw new Error(
        `coverage case references an unknown field: ${item.id}:${fieldKey}`,
      );
    if (item.status !== "not_in_scope") caseObservedFieldKeys.add(fieldKey);
  }
  if (
    item.status === "page_drift" &&
    !/contradict|conflict/i.test(item.reason || "")
  )
    throw new Error(
      `page drift must record a conflict with the frozen Manifest: ${item.id}`,
    );
}
for (const field of manifest.fields) {
  if (!caseObservedFieldKeys.has(field.key))
    throw new Error(
      `manifest field has no covered or blocked case: ${field.key}`,
    );
}
for (const page of manifest.page_families) {
  for (const locator of [page.entry_locator, ...page.page_fingerprint]) {
    if (locator.kind === "css" || /nth-child|\[\d+\]/.test(locator.value))
      throw new Error("manifest contains an unstable locator");
  }
}
if (
  manifest.fields.some(
    (field) =>
      !manifest.page_families.some((page) => page.id === field.page_family),
  )
)
  throw new Error("manifest field references an unknown page family");
if (
  manifest.fields.some(
    (field) =>
      field.locator.kind === "css" ||
      /nth-child|\[\d+\]/.test(field.locator.value),
  )
)
  throw new Error("manifest contains an unstable locator");
const mappedPairs = new Set(
  manifest.consumer_mappings.map(
    (mapping) => `${mapping.field_key}:${mapping.destination}`,
  ),
);
for (const field of manifest.fields) {
  for (const destination of field.consumers) {
    if (!mappedPairs.has(`${field.key}:${destination}`))
      throw new Error(
        `unconsumed manifest field destination: ${field.key}:${destination}`,
      );
  }
  if (!field.playwright_rpa)
    throw new Error(`manifest field has no Computer Use control: ${field.key}`);
  const control = field.playwright_rpa;
  if (field.condition) {
    if (!field.condition_dimensions?.length)
      throw new Error(
        `conditional field has no condition dimensions: ${field.key}`,
      );
    unique(field.condition_dimensions, `condition dimensions for ${field.key}`);
    for (const dimension of field.condition_dimensions) {
      if (forbiddenConditionNames.has(dimension))
        throw new Error(
          `field uses a non-standard condition name: ${field.key}:${dimension}`,
        );
      if (!conditionVocabulary.has(dimension))
        throw new Error(
          `field uses an undeclared condition dimension: ${field.key}:${dimension}`,
        );
    }
    if (field.condition_state === "dependency_only") {
      if (field.condition_rule)
        throw new Error(
          `dependency-only field has a machine rule: ${field.key}`,
        );
      if (
        field.evidence_state !== "platform_pending" ||
        control.operation !== "no_action"
      )
        throw new Error(
          `dependency-only field can produce a fill action: ${field.key}`,
        );
    } else {
      if (!field.condition_rule?.all?.length)
        throw new Error(`evaluable field has no machine rule: ${field.key}`);
      for (const predicate of field.condition_rule.all) {
        if (!field.condition_dimensions.includes(predicate.dimension))
          throw new Error(
            `machine rule uses an undeclared field dimension: ${field.key}:${predicate.dimension}`,
          );
        const vocabularyItem = conditionVocabulary.get(predicate.dimension);
        if (!vocabularyItem)
          throw new Error(
            `machine rule uses an unknown vocabulary key: ${field.key}:${predicate.dimension}`,
          );
        if (
          vocabularyItem.value_kind === "reference" &&
          !["present", "absent"].includes(predicate.operator)
        )
          throw new Error(
            `dynamic references cannot use semantic-value comparison: ${field.key}:${predicate.dimension}`,
          );
      }
    }
  }
  if (control.operation !== "no_action") {
    for (const locator of [control.scope, control.target, control.readback]) {
      if (locator.kind === "css" || /nth-child|\[\d+\]/.test(locator.value))
        throw new Error(
          `Computer Use control has an unstable locator: ${field.key}`,
        );
    }
  }
  if (
    control.operation === "choose_exact_visible_option" &&
    (!control.observed_options || control.observed_options.length === 0)
  )
    throw new Error(`choice control has no observed options: ${field.key}`);
  if (field.value_type === "money_minor") {
    const constraints = control.input_constraints || {};
    if (
      control.operation !== "fill_money" ||
      constraints.input_unit !== "CNY_yuan" ||
      constraints.model_unit !== "CNY_fen" ||
      constraints.minor_per_input_unit !== 100
    )
      throw new Error(
        `money control lacks exact CNY unit conversion: ${field.key}`,
      );
  }
}
for (const mapping of manifest.consumer_mappings) {
  const field = manifest.fields.find(
    (candidate) => candidate.key === mapping.field_key,
  );
  if (!field)
    throw new Error(
      `consumer mapping references an unknown field: ${mapping.field_key}`,
    );
  if (!field.consumers.includes(mapping.destination))
    throw new Error(
      `consumer mapping is not declared by field: ${mapping.field_key}:${mapping.destination}`,
    );
  const leaf = mapping.contract_path.split(".").at(-1);
  if (mapping.treatment === "evidence_only") {
    if (
      !/(?:CalibrationManifest|FieldEvidence|calibration_manifest)$/.test(
        mapping.contract_path,
      )
    )
      throw new Error(
        `evidence-only mapping has an executable-looking path: ${mapping.field_key}:${mapping.destination}`,
      );
  } else if (!contractSource.includes(leaf)) {
    throw new Error(
      `manifest contract path is not represented in the checked-in contracts: ${mapping.contract_path}`,
    );
  }
}
const observedPath = manifest.coverage_cases
  .flatMap((item) => item.path)
  .join("|");
for (const dimension of manifest.path_dimensions) {
  unique(dimension.observed_values, `values for ${dimension.key}`);
  for (const value of dimension.observed_values) {
    if (!observedPath.includes(value))
      throw new Error(
        `path dimension value has no coverage case: ${dimension.key}:${value}`,
      );
  }
}
const serialized = JSON.stringify(manifest);
if (/(?:aadvid=|https?:\/\/|cookie|token|余额|[0-9]{10,})/i.test(serialized))
  throw new Error("manifest redaction scan failed");
if (
  /(?:aadvid=|https?:\/\/|cookie|token|余额|[0-9]{10,})/i.test(
    JSON.stringify(writeCalibrationEvidence),
  )
)
  throw new Error("Phase D write-calibration redaction scan failed");
if (
  evidence.manifest_ref !==
    "docs/delivery/fixtures/oceanengine-calibration-manifest-v1.json" ||
  evidence.safe_exit?.remote_write_detected ||
  evidence.safe_exit?.final_confirmation_count !== 0 ||
  evidence.safe_exit?.controlled_action_attempt_count !== 0 ||
  evidence.safe_exit?.object_budget_and_status_changed
)
  throw new Error("calibration evidence does not prove the no-write boundary");
const evidenceCases = new Map(
  evidence.cases.map((item) => [item.case_id, item.outcome]),
);
if (
  writeCalibrationEvidence.scope?.production_authority_objects_created !==
    false ||
  writeCalibrationEvidence.final_state?.project_switch !== false ||
  writeCalibrationEvidence.final_state?.promotion_switch !== true ||
  writeCalibrationEvidence.final_state?.promotion_budget_minor !== 30000 ||
  writeCalibrationEvidence.final_state?.original_material_restored !== true
)
  throw new Error(
    "Phase D write calibration does not prove restored original state",
  );
for (const operation of writeCalibrationEvidence.operations)
  evidenceCases.set(operation.case_id, operation.outcome);
for (const item of manifest.coverage_cases.filter(
  (item) => item.status !== "not_in_scope",
)) {
  if (evidenceCases.get(item.id) !== item.status)
    throw new Error(`coverage case has no matching evidence: ${item.id}`);
}
const destinations = new Set(
  manifest.consumer_mappings.map((mapping) => mapping.destination),
);
for (const destination of [
  "DeliveryIntent",
  "OceanEngineConfiguration",
  "DeliveryDecisionCandidate",
  "CompiledDeliveryWorkflow",
  "PlatformSkill",
]) {
  if (!destinations.has(destination))
    throw new Error(`manifest has no consumer mapping for ${destination}`);
}
console.log(
  `validated ${manifest.manifest_id}: ${manifest.fields.length} fields, ${manifest.coverage_cases.length} cases`,
);
