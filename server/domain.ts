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
  version: number;
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
  businessTasks: BusinessTask[];
  artifacts: Artifact[];
  generationJobs: GenerationJob[];
  changeSets: ChangeSet[];
  auditEvents: AuditEvent[];
}

export const emptyStore = (): StoreData => ({
  projects: [],
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

export function isBusinessTaskType(value: unknown): value is BusinessTaskType {
  return typeof value === "string" && BUSINESS_TASK_TYPES.includes(value as BusinessTaskType);
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
