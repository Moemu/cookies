import type {
  ApiArtifact,
  ApiBusinessTask,
  ApiBusinessTaskType,
  ApiGenerationJob,
  ApiOperationalRecord,
  ApiProject,
} from "./api";
import type { DeliveryChangeSet } from "../api/delivery";

type Fetcher = typeof fetch;

export type PlatformProject = {
  id: string;
  organization_id: string;
  name: string;
  status: "draft" | "active" | "archived";
  primary_brand_id: string | null;
  brand_guideline_version_id?: string;
  project_context_version: number;
  created_at: string;
  updated_at: string;
};

export type PlatformProjectRuntime = {
  code: string;
  brand?: string;
  product?: string;
  goal?: string;
  stage: string;
  progress: number;
  status: "active" | "completed" | "blocked" | string;
  owner: string;
  budget: number;
  currency: "CNY";
  timezone: "Asia/Shanghai";
  knowledge_count?: number;
  updated_at: string;
};

export type PlatformProjectAssetRef = {
  project_id: string;
  asset_version: {
    asset_id: string;
    version: number;
  };
};

export type PlatformProjectArtifactSummary = {
  id?: string;
  key: "brief" | "strategy" | "creative" | "insight" | "delivery" | string;
  label: string;
  version: string;
  status: string;
  owner: string;
  updated_at: string;
  summary: string;
  source_version?: string;
  asset_ref?: PlatformProjectAssetRef | null;
};

export type PlatformProjectAsset = {
  ref?: PlatformProjectAssetRef;
  asset?: {
    id: string;
    asset_kind?: string;
    status?: string;
    latest_version?: number;
  };
  version?: {
    version: number;
    status?: string;
    source_type?: string;
    mime_type?: string;
    created_at?: string;
  };
  created_at?: string;
};

export type PlatformBusinessTask = {
  id: string;
  project_id: string;
  type: ApiBusinessTaskType;
  name: string;
  objective: string;
  status: ApiBusinessTask["status"];
  source_task_ids: string[];
  source_artifact_ids: string[];
  output_artifact_ids: string[];
  version: number;
  created_at: string;
  updated_at: string;
};

export type PlatformOperationalRecord = {
  id: string;
  project_id: string;
  kind: ApiOperationalRecord["kind"];
  title: string;
  status: string;
  occurred_at: string;
  fields: Record<string, string | number>;
  created_at: string;
  updated_at: string;
};

export type PlatformChangeSet = {
  id: string;
  project_id: string;
  name: string;
  status: DeliveryChangeSet["status"];
  artifact_refs: PlatformProjectAssetRef[];
  budget_limit?: number;
  preflight: null | {
    passed: boolean;
    checks: Array<{ code: "confirmed_brief" | "ready_creative" | "budget_boundary" | string; passed: boolean; message: string; repair: string }>;
    checked_at: string;
  };
  execution: null | {
    simulated: true;
    evidence: Array<{ step: string; status: string; message: string; recorded_at: string }>;
    executed_at: string;
  };
  rollback: null | {
    simulated: true;
    reason: string;
    rolled_back_at: string;
  };
  audit_events: unknown[];
  version: number;
  created_at: string;
  updated_at: string;
};

export type PlatformProviderJob = {
  id: string;
  kind: string;
  organization_id: string;
  project_id: string;
  execution_status: ApiGenerationJob["status"];
  provider_status: "submitted" | "running" | "outputs_ready" | "ingesting" | "succeeded" | "partially_succeeded" | "failed" | "cancelled" | "expired";
  progress: number;
  project_asset_refs: PlatformProjectAssetRef[];
  error: null | { code: string; message: string; retryable: boolean };
  attempt_count: number;
  max_attempts: number;
  version: number;
  created_at: string;
  updated_at: string;
};

export type PlatformProjectDetail = {
  project: PlatformProject;
  runtime: PlatformProjectRuntime;
  artifacts: PlatformProjectArtifactSummary[];
  assets: PlatformProjectAsset[];
  tasks: PlatformBusinessTask[];
  operations: PlatformOperationalRecord[];
  change_sets: PlatformChangeSet[];
};

type PlatformClientOptions = {
  baseUrl?: string;
  fetcher?: Fetcher;
  idempotencyKey?: () => string;
};

type ItemsResponse<T> = { items?: T[] | null };

const viteEnv = (import.meta as unknown as { env?: { VITE_API_BASE_URL?: string } }).env;
const defaultPlatformBase = `${viteEnv?.VITE_API_BASE_URL ?? ""}/platform/v1`;

export function createPlatformClient(options: PlatformClientOptions = {}) {
  const baseUrl = (options.baseUrl ?? defaultPlatformBase).replace(/\/$/, "");
  const fetcher = options.fetcher ?? fetch;
  const idempotencyKey = options.idempotencyKey ?? (() => `web-${Date.now()}-${Math.random().toString(36).slice(2)}`);

  async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const headers = new Headers(init.headers);
    if (init.body !== undefined && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
    const response = await fetcher(`${baseUrl}${path}`, {
      credentials: "include",
      ...init,
      headers,
    });
    const payload = await readPayload<T>(response);
    if (!response.ok) {
      const error = payload as { error?: { message?: string }; message?: string };
      throw new Error(error.error?.message ?? error.message ?? "平台 API 请求失败");
    }
    return payload as T;
  }

  function withIdempotency(init: RequestInit = {}): RequestInit {
    const headers = new Headers(init.headers);
    if (!headers.has("Idempotency-Key")) headers.set("Idempotency-Key", idempotencyKey());
    return { ...init, headers };
  }

  return {
    listProjects: async () => asArray((await request<ItemsResponse<PlatformProject>>("/projects")).items).map(toApiProject),
    createProject: (input: Pick<ApiProject, "name" | "brand" | "objective">) =>
      request<PlatformProject>("/projects", {
        method: "POST",
        body: JSON.stringify({ name: input.name, primary_brand_id: null, product_ids: [], activate: false }),
      }).then(project => toApiProject(project)),
    getProjectDetail: (projectId: string) =>
      request<PlatformProjectDetail>(`/projects/${encodeURIComponent(projectId)}`),
    getProjectSnapshot: async (projectId: string) => {
      const detail = await request<PlatformProjectDetail>(`/projects/${encodeURIComponent(projectId)}`);
      return {
        project: toApiProject(detail),
        artifacts: asArray(detail.artifacts).map(summary => toApiArtifact(summary, detail.project.id)),
        jobs: asArray(detail.artifacts).map(summary => toApiGenerationJobFromArtifact(summary, detail.project.id)).filter((job): job is ApiGenerationJob => Boolean(job)),
        tasks: asArray(detail.tasks).map(toApiBusinessTask),
        operations: asArray(detail.operations).map(toApiOperationalRecord),
        changeSets: asArray(detail.change_sets).map(toDeliveryChangeSet),
      };
    },
    listArtifacts: (projectId: string) =>
      request<PlatformProjectDetail>(`/projects/${encodeURIComponent(projectId)}`)
        .then(detail => asArray(detail.artifacts).map(summary => toApiArtifact(summary, detail.project.id))),
    listJobs: (projectId: string) =>
      request<PlatformProjectDetail>(`/projects/${encodeURIComponent(projectId)}`)
        .then(detail => asArray(detail.artifacts).map(summary => toApiGenerationJobFromArtifact(summary, detail.project.id)).filter((job): job is ApiGenerationJob => Boolean(job))),
    listTasks: async (projectId: string) =>
      asArray((await request<ItemsResponse<PlatformBusinessTask>>(`/projects/${encodeURIComponent(projectId)}/tasks`)).items).map(toApiBusinessTask),
    createTask: (projectId: string, input: {
      type: ApiBusinessTaskType;
      name: string;
      objective: string;
      sourceTaskIds?: string[];
      sourceArtifactIds?: string[];
    }) => request<PlatformBusinessTask>(`/projects/${encodeURIComponent(projectId)}/tasks`, withIdempotency({
      method: "POST",
      body: JSON.stringify({
        type: input.type,
        name: input.name,
        objective: input.objective,
        source_task_ids: input.sourceTaskIds ?? [],
        source_artifact_ids: input.sourceArtifactIds ?? [],
      }),
    })).then(toApiBusinessTask),
    updateTask: (projectId: string, taskId: string, input: Partial<Pick<ApiBusinessTask, "name" | "objective" | "status" | "sourceTaskIds" | "sourceArtifactIds" | "outputArtifactIds">>) =>
      request<PlatformBusinessTask>(`/projects/${encodeURIComponent(projectId)}/tasks/${encodeURIComponent(taskId)}`, {
        method: "PATCH",
        body: JSON.stringify({
          name: input.name,
          objective: input.objective,
          status: input.status,
          source_task_ids: input.sourceTaskIds,
          source_artifact_ids: input.sourceArtifactIds,
          output_artifact_ids: input.outputArtifactIds,
        }),
      }).then(toApiBusinessTask),
    listOperations: async (projectId: string) =>
      asArray((await request<ItemsResponse<PlatformOperationalRecord>>(`/projects/${encodeURIComponent(projectId)}/operations`)).items).map(toApiOperationalRecord),
    listChangeSets: async (projectId: string) =>
      asArray((await request<ItemsResponse<PlatformChangeSet>>(`/projects/${encodeURIComponent(projectId)}/change-sets`)).items).map(toDeliveryChangeSet),
    createChangeSet: (projectId: string, input: { name: string; artifactIds: string[]; budgetLimit: number }) =>
      request<PlatformChangeSet>(`/projects/${encodeURIComponent(projectId)}/change-sets`, withIdempotency({
        method: "POST",
        body: JSON.stringify({
          name: input.name,
          artifact_refs: input.artifactIds.map(artifactId => toProjectAssetRef(projectId, artifactId)),
          budget_limit: input.budgetLimit,
        }),
      })).then(toDeliveryChangeSet),
    preflightChangeSet: (projectId: string, changeSetId: string) =>
      request<PlatformChangeSet>(`/projects/${encodeURIComponent(projectId)}/change-sets/${encodeURIComponent(changeSetId)}/preflight`, { method: "POST" }).then(toDeliveryChangeSet),
    approveChangeSet: (projectId: string, changeSetId: string) =>
      request<PlatformChangeSet>(`/projects/${encodeURIComponent(projectId)}/change-sets/${encodeURIComponent(changeSetId)}/approve`, {
        method: "POST",
        body: JSON.stringify({ actor: "Amelia Meng", role: "demo-approver", note: "前端演示审批通过" }),
      }).then(toDeliveryChangeSet),
    executeChangeSet: (projectId: string, changeSetId: string) =>
      request<PlatformChangeSet>(`/projects/${encodeURIComponent(projectId)}/change-sets/${encodeURIComponent(changeSetId)}/execute`, { method: "POST" }).then(toDeliveryChangeSet),
    rollbackChangeSet: (projectId: string, changeSetId: string, reason: string) =>
      request<PlatformChangeSet>(`/projects/${encodeURIComponent(projectId)}/change-sets/${encodeURIComponent(changeSetId)}/rollback`, {
        method: "POST",
        body: JSON.stringify({ actor: "Amelia Meng", reason }),
      }).then(toDeliveryChangeSet),
    createMedia: (projectId: string, kind: "image" | "video", prompt: string) =>
      request<PlatformProviderJob>(`/projects/${encodeURIComponent(projectId)}/model/jobs`, withIdempotency({
        method: "POST",
        body: JSON.stringify({
          capability: kind === "video" ? "video.generate" : "image.generate",
          model_alias: kind === "video" ? "cookies.video.standard" : "cookies.image.standard",
          project_context_version: 1,
          input: kind === "video" ? { prompt, width: 720, height: 1280 } : { prompt, width: 1024, height: 1024 },
        }),
      })).then(toApiGenerationJob),
    getJob: (projectId: string, jobId: string) =>
      request<PlatformProviderJob>(`/projects/${encodeURIComponent(projectId)}/model/jobs/${encodeURIComponent(jobId)}`).then(toApiGenerationJob),
  };
}

export const platformClient = createPlatformClient();

export async function readPayload<T>(response: Response): Promise<T | Record<string, never>> {
  if (response.status === 204) return {};
  const text = await response.text();
  if (!text) return {};
  return JSON.parse(text) as T;
}

export function toApiProject(input: PlatformProject | PlatformProjectDetail): ApiProject {
  const project = "project" in input ? input.project : input;
  const runtime = "runtime" in input ? input.runtime : defaultRuntime(project);
  const updatedAt = runtime.updated_at ?? project.updated_at;
  return {
    id: project.id,
    name: project.name,
    brand: runtime.brand ?? "未指定品牌",
    objective: runtime.goal ?? "",
    runtime: {
      code: runtime.code,
      product: runtime.product ?? "",
      stage: runtime.stage,
      progress: runtime.progress,
      status: runtime.status === "completed" ? "completed" : "active",
      owner: runtime.owner,
      budget: runtime.budget,
      currency: runtime.currency,
      timezone: runtime.timezone,
    },
    version: project.project_context_version,
    createdAt: project.created_at,
    updatedAt,
  };
}

export function toApiArtifact(summary: PlatformProjectArtifactSummary, projectId: string): ApiArtifact {
  return {
    id: summary.id ?? summary.asset_ref?.asset_version.asset_id ?? `${projectId}-${summary.key}`,
    projectId,
    kind: summary.key === "creative" ? "image" : summary.key === "brief" ? "brief" : "document",
    status: summary.status === "已确认" || summary.status === "ready" || summary.status === "已完成" ? "ready" : "draft",
    content: summary.summary,
    sourceJobId: summary.source_version,
    version: versionNumber(summary.version),
    createdAt: summary.updated_at,
    updatedAt: summary.updated_at,
  };
}

export function toApiBusinessTask(task: PlatformBusinessTask): ApiBusinessTask {
  return {
    id: task.id,
    projectId: task.project_id,
    type: task.type,
    name: task.name,
    objective: task.objective,
    status: task.status,
    sourceTaskIds: task.source_task_ids ?? [],
    sourceArtifactIds: task.source_artifact_ids ?? [],
    outputArtifactIds: task.output_artifact_ids ?? [],
    version: task.version,
    createdAt: task.created_at,
    updatedAt: task.updated_at,
  };
}

export function toApiOperationalRecord(record: PlatformOperationalRecord): ApiOperationalRecord {
  return {
    id: record.id,
    projectId: record.project_id,
    kind: record.kind,
    title: record.title,
    status: record.status,
    occurredAt: record.occurred_at,
    fields: record.fields,
    createdAt: record.created_at,
    updatedAt: record.updated_at,
  };
}

export function toDeliveryChangeSet(changeSet: PlatformChangeSet): DeliveryChangeSet {
  return {
    id: changeSet.id,
    projectId: changeSet.project_id,
    name: changeSet.name,
    status: changeSet.status,
    artifactIds: asArray(changeSet.artifact_refs).map(ref => ref.asset_version.asset_id),
    budgetLimit: changeSet.budget_limit,
    preflight: changeSet.preflight ? {
      passed: changeSet.preflight.passed,
      checks: asArray(changeSet.preflight.checks).map(check => ({
        code: check.code as DeliveryChangeSet["preflight"] extends { checks: Array<infer Check> } ? Check extends { code: infer Code } ? Code : never : never,
        passed: check.passed,
        message: check.message,
        repair: check.repair,
      })),
      checkedAt: changeSet.preflight.checked_at,
    } : undefined,
    execution: changeSet.execution ? {
      simulated: true,
      evidence: asArray(changeSet.execution.evidence).map(item => ({
        step: item.step,
        status: item.status,
        message: item.message,
        recordedAt: item.recorded_at,
      })),
      executedAt: changeSet.execution.executed_at,
    } : undefined,
    rollback: changeSet.rollback ? {
      simulated: true,
      reason: changeSet.rollback.reason,
      rolledBackAt: changeSet.rollback.rolled_back_at,
    } : undefined,
    version: changeSet.version,
    createdAt: changeSet.created_at,
    updatedAt: changeSet.updated_at,
  };
}

export function toApiGenerationJob(job: PlatformProviderJob): ApiGenerationJob {
  const artifactKind = job.kind.includes("video") ? "video" : "image";
  const firstAssetRef = asArray(job.project_asset_refs)[0];
  return {
    id: job.id,
    projectId: job.project_id,
    artifactKind,
    status: job.execution_status,
    model: artifactKind === "video" ? "cookies.video.standard" : "cookies.image.standard",
    diagnostic: job.error?.message,
    artifactId: firstAssetRef?.asset_version.asset_id,
    version: job.version,
    createdAt: job.created_at,
    updatedAt: job.updated_at,
  };
}

function asArray<T>(items: T[] | null | undefined): T[] {
  return items ?? [];
}

function toApiGenerationJobFromArtifact(summary: PlatformProjectArtifactSummary, projectId: string): ApiGenerationJob | null {
  if (summary.key !== "creative" || !summary.asset_ref) return null;
  return {
    id: summary.source_version ?? `asset-${summary.asset_ref.asset_version.asset_id}`,
    projectId,
    artifactKind: "image",
    status: summary.status === "已完成" || summary.status === "已确认" || summary.status === "ready" ? "succeeded" : "running",
    model: "cookies.image.standard",
    artifactId: summary.asset_ref.asset_version.asset_id,
    version: summary.asset_ref.asset_version.version,
    createdAt: summary.updated_at,
    updatedAt: summary.updated_at,
  };
}

function defaultRuntime(project: PlatformProject): PlatformProjectRuntime {
  return {
    code: project.id,
    brand: "",
    product: "",
    goal: "",
    stage: project.status === "archived" ? "已归档" : "项目初始化",
    progress: project.status === "active" ? 10 : 0,
    status: project.status === "archived" ? "completed" : "active",
    owner: "服务端演示用户",
    budget: 0,
    currency: "CNY",
    timezone: "Asia/Shanghai",
    updated_at: project.updated_at,
  };
}

function toProjectAssetRef(projectId: string, artifactId: string): PlatformProjectAssetRef {
  const match = artifactId.match(/^(.*):v(\d+)$/);
  return {
    project_id: projectId,
    asset_version: {
      asset_id: match?.[1] ?? artifactId,
      version: match?.[2] ? Number(match[2]) : 1,
    },
  };
}

function versionNumber(version: string): number {
  const match = version.match(/\d+/);
  return match ? Number(match[0]) : 1;
}
