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
