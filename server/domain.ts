import { DomainError } from "./errors.js";

export const GENERATION_JOB_STATUSES = [
  "queued",
  "running",
  "succeeded",
  "failed",
  "cancelled",
] as const;
export type GenerationJobStatus = (typeof GENERATION_JOB_STATUSES)[number];

export const CHANGE_SET_STATUSES = [
  "draft",
  "preflight_passed",
  "preflight_failed",
  "approved",
  "rejected",
  "executing",
  "executed",
  "rolled_back",
] as const;
export type ChangeSetStatus = (typeof CHANGE_SET_STATUSES)[number];

export type ArtifactKind = "brief" | "image" | "video" | "document";
export type ArtifactStatus = "draft" | "ready" | "archived";

export const VIDEO_PURPOSES = ["preroll"] as const;
export type VideoPurpose = (typeof VIDEO_PURPOSES)[number];

export const PREROLL_TYPES = ["short_drama", "game", "commerce"] as const;
export type PrerollType = (typeof PREROLL_TYPES)[number];

export const BUSINESS_TASK_TYPES = [
  "strategy",
  "creative",
  "video",
  "brand_video",
  "short_drama_preroll",
  "game_preroll",
  "commerce_preroll",
  "viral_remake",
  "video_edit",
] as const;
export type BusinessTaskType = (typeof BUSINESS_TASK_TYPES)[number];

export const BUSINESS_TASK_STATUSES = ["draft", "in_progress", "ready", "completed", "failed"] as const;
export type BusinessTaskStatus = (typeof BUSINESS_TASK_STATUSES)[number];

export interface Project {
  id: string;
  name: string;
  brand: string;
  objective: string;
  runtime: ProjectRuntime;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface ProjectRuntime {
  code: string;
  product: string;
  stage: string;
  progress: number;
  status: "active" | "completed";
  owner: string;
  budget: number;
  currency: "CNY";
  timezone: "Asia/Shanghai";
}

export const OPERATIONAL_RECORD_KINDS = [
  "work_item",
  "evidence",
  "activity",
  "metric",
  "performance_ad",
  "audience_mix",
  "method",
  "delivery_diagnostic",
  "delivery_action",
  "unified_record",
] as const;
export type OperationalRecordKind = (typeof OPERATIONAL_RECORD_KINDS)[number];

export interface OperationalRecord {
  id: string;
  projectId: string;
  kind: OperationalRecordKind;
  title: string;
  status: string;
  occurredAt: string;
  fields: Record<string, string | number>;
  createdAt: string;
  updatedAt: string;
}

export interface BusinessTask {
  id: string;
  projectId: string;
  type: BusinessTaskType;
  name: string;
  objective: string;
  status: BusinessTaskStatus;
  sourceTaskIds: string[];
  sourceArtifactIds: string[];
  outputArtifactIds: string[];
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface Artifact {
  id: string;
  projectId: string;
  kind: ArtifactKind;
  purpose?: VideoPurpose;
  prerollType?: PrerollType;
  status: ArtifactStatus;
  content: string;
  sourceJobId?: string;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface GenerationJob {
  id: string;
  projectId: string;
  artifactKind: ArtifactKind;
  purpose?: VideoPurpose;
  prerollType?: PrerollType;
  briefArtifactId?: string;
  status: GenerationJobStatus;
  model?: string;
  providerTaskId?: string;
  diagnostic?: string;
  artifactId?: string;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface ChangeSet {
  id: string;
  projectId: string;
  name: string;
  status: ChangeSetStatus;
  artifactIds: string[];
  budgetLimit?: number;
  preflight?: PreflightResult;
  execution?: SimulationExecution;
  rollback?: SimulationRollback;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface PreflightCheck {
  code: "confirmed_brief" | "ready_creative" | "budget_boundary";
  passed: boolean;
  message: string;
  repair: string;
}

export interface PreflightResult {
  passed: boolean;
  checks: PreflightCheck[];
  checkedAt: string;
}

export interface SimulationEvidence {
  step: "validate_input" | "apply_simulation" | "verify_result";
  status: "completed";
  message: string;
  recordedAt: string;
}

export interface SimulationExecution {
  simulated: true;
  evidence: SimulationEvidence[];
  executedAt: string;
}

export interface SimulationRollback {
  simulated: true;
  reason: string;
  rolledBackAt: string;
}

export interface AuditEvent {
  id: string;
  projectId: string;
  actor: string;
  action: string;
  entityType: "project" | "business_task" | "artifact" | "generation_job" | "change_set";
  entityId: string;
  metadata: Record<string, unknown>;
  createdAt: string;
}

export interface StoreData {
  projects: Project[];
  operationalRecords: OperationalRecord[];
  businessTasks: BusinessTask[];
  artifacts: Artifact[];
  generationJobs: GenerationJob[];
  changeSets: ChangeSet[];
  auditEvents: AuditEvent[];
}

export const emptyStore = (): StoreData => ({
  projects: [],
  operationalRecords: [],
  businessTasks: [],
  artifacts: [],
  generationJobs: [],
  changeSets: [],
  auditEvents: [],
});

const generationJobTransitions: Record<GenerationJobStatus, readonly GenerationJobStatus[]> = {
  queued: ["running", "cancelled"],
  running: ["succeeded", "failed", "cancelled"],
  succeeded: [],
  failed: ["queued"],
  cancelled: ["queued"],
};

const changeSetTransitions: Record<ChangeSetStatus, readonly ChangeSetStatus[]> = {
  draft: ["preflight_passed", "preflight_failed"],
  preflight_passed: ["approved", "rejected"],
  preflight_failed: ["draft"],
  approved: ["executing"],
  rejected: ["draft"],
  executing: ["executed"],
  executed: ["rolled_back"],
  rolled_back: [],
};

function assertTransition<T extends string>(
  entity: string,
  transitions: Record<T, readonly T[]>,
  current: T,
  next: T,
): void {
  if (!transitions[current].includes(next)) {
    throw new DomainError(
      "INVALID_STATE_TRANSITION",
      `Cannot transition ${entity} from ${current} to ${next}`,
    );
  }
}

export function assertGenerationJobTransition(
  current: GenerationJobStatus,
  next: GenerationJobStatus,
): void {
  assertTransition("generation job", generationJobTransitions, current, next);
}

export function assertChangeSetTransition(current: ChangeSetStatus, next: ChangeSetStatus): void {
  assertTransition("change set", changeSetTransitions, current, next);
}

export function isArtifactKind(value: unknown): value is ArtifactKind {
  return value === "brief" || value === "image" || value === "video" || value === "document";
}

export function isVideoPurpose(value: unknown): value is VideoPurpose {
  return typeof value === "string" && VIDEO_PURPOSES.includes(value as VideoPurpose);
}

export function isPrerollType(value: unknown): value is PrerollType {
  return typeof value === "string" && PREROLL_TYPES.includes(value as PrerollType);
}

export function assertVideoMetadata(
  kind: ArtifactKind,
  purpose: VideoPurpose | undefined,
  prerollType: PrerollType | undefined,
): void {
  if (kind !== "video" && (purpose !== undefined || prerollType !== undefined)) {
    throw new DomainError("VALIDATION_ERROR", "Only video resources can have a purpose or preroll type");
  }
  if (purpose === "preroll" && prerollType === undefined) {
    throw new DomainError("VALIDATION_ERROR", "Preroll video resources require a preroll type");
  }
  if (purpose === undefined && prerollType !== undefined) {
    throw new DomainError("VALIDATION_ERROR", "A preroll type requires the preroll purpose");
  }
}

export function isBusinessTaskType(value: unknown): value is BusinessTaskType {
  return typeof value === "string" && BUSINESS_TASK_TYPES.includes(value as BusinessTaskType);
}

export function isOperationalRecordKind(value: unknown): value is OperationalRecordKind {
  return typeof value === "string" && OPERATIONAL_RECORD_KINDS.includes(value as OperationalRecordKind);
}

export function isBusinessTaskStatus(value: unknown): value is BusinessTaskStatus {
  return typeof value === "string" && BUSINESS_TASK_STATUSES.includes(value as BusinessTaskStatus);
}

export function isGenerationJobStatus(value: unknown): value is GenerationJobStatus {
  return typeof value === "string" && GENERATION_JOB_STATUSES.includes(value as GenerationJobStatus);
}

export function isChangeSetStatus(value: unknown): value is ChangeSetStatus {
  return typeof value === "string" && CHANGE_SET_STATUSES.includes(value as ChangeSetStatus);
}
