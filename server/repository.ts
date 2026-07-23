import { randomUUID } from "node:crypto";
import { mkdir, readFile, rename, writeFile } from "node:fs/promises";
import { dirname } from "node:path";
import {
  type Artifact,
  type ArtifactKind,
  type ArtifactStatus,
  assertChangeSetTransition,
  assertGenerationJobTransition,
  type AuditEvent,
  type ChangeSet,
  emptyStore,
  type GenerationJob,
  type GenerationJobStatus,
  type PreflightCheck,
  type PreflightResult,
  type Project,
  type SimulationEvidence,
  type StoreData,
} from "./domain.js";
import { DomainError } from "./errors.js";

export interface CreateProjectInput {
  name: string;
  brand: string;
  objective: string;
  actor?: string;
}

export interface CreateArtifactInput {
  projectId: string;
  kind: ArtifactKind;
  content: string;
  status?: ArtifactStatus;
  sourceJobId?: string;
  actor?: string;
}

export interface CreateGenerationJobInput {
  projectId: string;
  artifactKind: ArtifactKind;
  briefArtifactId?: string;
  model?: string;
  actor?: string;
}

export interface CreateChangeSetInput {
  projectId: string;
  name: string;
  artifactIds?: string[];
  budgetLimit?: number;
  actor?: string;
}

export class FileRepository {
  private constructor(
    private readonly filePath: string,
    private store: StoreData,
  ) {}

  static async open(filePath: string): Promise<FileRepository> {
    try {
      const content = await readFile(filePath, "utf8");
      return new FileRepository(filePath, JSON.parse(content) as StoreData);
    } catch (error: unknown) {
      if ((error as NodeJS.ErrnoException).code === "ENOENT") {
        const repository = new FileRepository(filePath, emptyStore());
        await repository.persist();
        return repository;
      }
      throw error;
    }
  }

  async listProjects(): Promise<Project[]> {
    return [...this.store.projects];
  }

  async getProject(id: string): Promise<Project | undefined> {
    return this.store.projects.find((project) => project.id === id);
  }

  async createProject(input: CreateProjectInput): Promise<Project> {
    const now = new Date().toISOString();
    const project: Project = {
      id: randomUUID(),
      name: input.name,
      brand: input.brand,
      objective: input.objective,
      version: 1,
      createdAt: now,
      updatedAt: now,
    };
    await this.mutate(() => {
      this.store.projects.push(project);
      this.addAudit(project.id, input.actor, "project.created", "project", project.id, {});
    });
    return project;
  }

  async updateProject(
    id: string,
    input: Partial<Pick<Project, "name" | "brand" | "objective">> & { actor?: string },
  ): Promise<Project> {
    const project = this.requireProject(id);
    await this.mutate(() => {
      Object.assign(project, withoutActor(input), touch(project));
      this.addAudit(id, input.actor, "project.updated", "project", id, { version: project.version });
    });
    return project;
  }

  async listArtifacts(projectId?: string): Promise<Artifact[]> {
    return this.store.artifacts.filter((artifact) => !projectId || artifact.projectId === projectId);
  }

  async getArtifact(id: string): Promise<Artifact | undefined> {
    return this.store.artifacts.find((artifact) => artifact.id === id);
  }

  async createArtifact(input: CreateArtifactInput): Promise<Artifact> {
    this.requireProject(input.projectId);
    const now = new Date().toISOString();
    const artifact: Artifact = {
      id: randomUUID(),
      projectId: input.projectId,
      kind: input.kind,
      status: input.status ?? "draft",
      content: input.content,
      sourceJobId: input.sourceJobId,
      version: 1,
      createdAt: now,
      updatedAt: now,
    };
    await this.mutate(() => {
      this.store.artifacts.push(artifact);
      this.addAudit(artifact.projectId, input.actor, "artifact.created", "artifact", artifact.id, {});
    });
    return artifact;
  }

  async updateArtifact(
    id: string,
    input: Partial<Pick<Artifact, "content" | "status" | "sourceJobId">> & { actor?: string },
  ): Promise<Artifact> {
    const artifact = this.requireArtifact(id);
    await this.mutate(() => {
      Object.assign(artifact, withoutActor(input), touch(artifact));
      this.addAudit(artifact.projectId, input.actor, "artifact.updated", "artifact", artifact.id, {
        version: artifact.version,
      });
    });
    return artifact;
  }

  async listGenerationJobs(projectId?: string): Promise<GenerationJob[]> {
    return this.store.generationJobs.filter((job) => !projectId || job.projectId === projectId);
  }

  async getGenerationJob(id: string): Promise<GenerationJob | undefined> {
    return this.store.generationJobs.find((job) => job.id === id);
  }

  async createGenerationJob(input: CreateGenerationJobInput): Promise<GenerationJob> {
    this.requireProject(input.projectId);
    const now = new Date().toISOString();
    const job: GenerationJob = {
      id: randomUUID(),
      projectId: input.projectId,
      artifactKind: input.artifactKind,
      briefArtifactId: input.briefArtifactId,
      status: "queued",
      model: input.model,
      version: 1,
      createdAt: now,
      updatedAt: now,
    };
    await this.mutate(() => {
      this.store.generationJobs.push(job);
      this.addAudit(job.projectId, input.actor, "generation_job.created", "generation_job", job.id, {});
    });
    return job;
  }

  async transitionGenerationJob(
    id: string,
    status: GenerationJobStatus,
    input: Partial<Pick<GenerationJob, "diagnostic" | "artifactId" | "providerTaskId">> & { actor?: string },
  ): Promise<GenerationJob> {
    const job = this.requireGenerationJob(id);
    assertGenerationJobTransition(job.status, status);
    const previousStatus = job.status;
    await this.mutate(() => {
      Object.assign(job, withoutActor(input), { status }, touch(job));
      this.addAudit(job.projectId, input.actor, "generation_job.status_changed", "generation_job", job.id, {
        from: previousStatus,
        to: status,
      });
    });
    return job;
  }

  async setGenerationJobProviderTask(
    id: string,
    providerTaskId: string,
    actor?: string,
  ): Promise<GenerationJob> {
    const job = this.requireGenerationJob(id);
    await this.mutate(() => {
      Object.assign(job, { providerTaskId }, touch(job));
      this.addAudit(job.projectId, actor, "generation_job.provider_task_created", "generation_job", job.id, {});
    });
    return job;
  }

  async updateGenerationJobDiagnostic(
    id: string,
    diagnostic: string | undefined,
    actor?: string,
  ): Promise<GenerationJob> {
    const job = this.requireGenerationJob(id);
    await this.mutate(() => {
      Object.assign(job, { diagnostic }, touch(job));
      this.addAudit(job.projectId, actor, "generation_job.sync_recorded", "generation_job", job.id, {
        diagnostic,
      });
    });
    return job;
  }

  async listChangeSets(projectId?: string): Promise<ChangeSet[]> {
    return this.store.changeSets.filter((changeSet) => !projectId || changeSet.projectId === projectId);
  }

  async getChangeSet(id: string): Promise<ChangeSet | undefined> {
    return this.store.changeSets.find((changeSet) => changeSet.id === id);
  }

  async createChangeSet(input: CreateChangeSetInput): Promise<ChangeSet> {
    this.requireProject(input.projectId);
    const now = new Date().toISOString();
    const changeSet: ChangeSet = {
      id: randomUUID(),
      projectId: input.projectId,
      name: input.name,
      status: "draft",
      artifactIds: input.artifactIds ?? [],
      budgetLimit: input.budgetLimit,
      version: 1,
      createdAt: now,
      updatedAt: now,
    };
    await this.mutate(() => {
      this.store.changeSets.push(changeSet);
      this.addAudit(changeSet.projectId, input.actor, "change_set.created", "change_set", changeSet.id, {});
    });
    return changeSet;
  }

  async preflightChangeSet(id: string, actor?: string): Promise<ChangeSet> {
    const changeSet = this.requireChangeSet(id);
    if (changeSet.status !== "draft") {
      throw new DomainError("INVALID_STATE_TRANSITION", `Cannot preflight change set from ${changeSet.status}`);
    }
    const preflight = this.evaluatePreflight(changeSet);
    const nextStatus = preflight.passed ? "preflight_passed" : "preflight_failed";
    await this.mutate(() => {
      Object.assign(changeSet, { status: nextStatus, preflight }, touch(changeSet));
      this.addAudit(changeSet.projectId, actor, "change_set.preflight_completed", "change_set", changeSet.id, {
        passed: preflight.passed,
        checks: preflight.checks.map((check) => ({ code: check.code, passed: check.passed })),
      });
    });
    return changeSet;
  }

  async approveChangeSet(id: string, actor: string, role: string): Promise<ChangeSet> {
    if (role !== "demo-approver") {
      throw new DomainError("FORBIDDEN", "Only a demo approver can approve a change set");
    }
    const changeSet = this.requireChangeSet(id);
    assertChangeSetTransition(changeSet.status, "approved");
    const preflight = this.requireCurrentPreflight(changeSet, "approve");
    await this.mutate(() => {
      Object.assign(changeSet, { status: "approved" }, touch(changeSet));
      this.addAudit(changeSet.projectId, actor, "change_set.approved", "change_set", changeSet.id, {
        simulated: true,
        role,
        preflightCheckedAt: preflight.checkedAt,
      });
    });
    return changeSet;
  }

  async executeChangeSetSimulation(id: string, actor?: string): Promise<ChangeSet> {
    const changeSet = this.requireChangeSet(id);
    assertChangeSetTransition(changeSet.status, "executing");
    this.requireCurrentPreflight(changeSet, "execute");
    const executedAt = new Date().toISOString();
    const evidence: SimulationEvidence[] = [
      { step: "validate_input", status: "completed", message: "已复核预检通过的输入和预算边界。", recordedAt: executedAt },
      { step: "apply_simulation", status: "completed", message: "已在本地模拟环境应用变更；未写入真实广告平台。", recordedAt: executedAt },
      { step: "verify_result", status: "completed", message: "已验证模拟结果，可随时执行回滚。", recordedAt: executedAt },
    ];
    await this.mutate(() => {
      Object.assign(changeSet, { status: "executing" }, touch(changeSet));
      this.addAudit(changeSet.projectId, actor, "change_set.simulation_started", "change_set", changeSet.id, { simulated: true });
      Object.assign(changeSet, {
        status: "executed",
        execution: { simulated: true, evidence, executedAt },
      }, touch(changeSet));
      this.addAudit(changeSet.projectId, actor, "change_set.simulation_completed", "change_set", changeSet.id, {
        simulated: true,
        evidenceCount: evidence.length,
      });
    });
    return changeSet;
  }

  async rollbackChangeSetSimulation(id: string, reason: string, actor?: string): Promise<ChangeSet> {
    const changeSet = this.requireChangeSet(id);
    assertChangeSetTransition(changeSet.status, "rolled_back");
    const rolledBackAt = new Date().toISOString();
    await this.mutate(() => {
      this.addAudit(changeSet.projectId, actor, "change_set.rollback_started", "change_set", changeSet.id, {
        simulated: true,
        reason,
      });
      Object.assign(changeSet, {
        status: "rolled_back",
        rollback: { simulated: true, reason, rolledBackAt },
      }, touch(changeSet));
      this.addAudit(changeSet.projectId, actor, "change_set.rolled_back", "change_set", changeSet.id, {
        simulated: true,
        reason,
      });
    });
    return changeSet;
  }

  async listAuditEvents(projectId?: string): Promise<AuditEvent[]> {
    return this.store.auditEvents.filter((event) => !projectId || event.projectId === projectId);
  }

  async recordAuditEvent(
    projectId: string,
    action: string,
    entityType: AuditEvent["entityType"],
    entityId: string,
    actor?: string,
    metadata: Record<string, unknown> = {},
  ): Promise<void> {
    this.requireProject(projectId);
    await this.mutate(() => {
      this.addAudit(projectId, actor, action, entityType, entityId, metadata);
    });
  }

  async getAuditEvent(id: string): Promise<AuditEvent | undefined> {
    return this.store.auditEvents.find((event) => event.id === id);
  }

  private requireProject(id: string): Project {
    const project = this.store.projects.find((item) => item.id === id);
    if (!project) throw new DomainError("NOT_FOUND", `Project ${id} was not found`);
    return project;
  }

  private requireGenerationJob(id: string): GenerationJob {
    const job = this.store.generationJobs.find((item) => item.id === id);
    if (!job) throw new DomainError("NOT_FOUND", `Generation job ${id} was not found`);
    return job;
  }

  private requireArtifact(id: string): Artifact {
    const artifact = this.store.artifacts.find((item) => item.id === id);
    if (!artifact) throw new DomainError("NOT_FOUND", `Artifact ${id} was not found`);
    return artifact;
  }

  private requireChangeSet(id: string): ChangeSet {
    const changeSet = this.store.changeSets.find((item) => item.id === id);
    if (!changeSet) throw new DomainError("NOT_FOUND", `Change set ${id} was not found`);
    return changeSet;
  }

  private evaluatePreflight(changeSet: ChangeSet): PreflightResult {
    const artifacts = changeSet.artifactIds
      .map((artifactId) => this.store.artifacts.find((artifact) => artifact.id === artifactId))
      .filter((artifact): artifact is Artifact => artifact !== undefined && artifact.projectId === changeSet.projectId);
    const checks: PreflightCheck[] = [
      {
        code: "confirmed_brief",
        passed: artifacts.some((artifact) => artifact.kind === "brief" && artifact.status === "ready"),
        message: "ChangeSet 包含已确认的 Brief",
        repair: "请关联状态为 ready 的 Brief 后重新预检。",
      },
      {
        code: "ready_creative",
        passed: artifacts.some(
          (artifact) => (artifact.kind === "image" || artifact.kind === "video") && artifact.status === "ready",
        ),
        message: "ChangeSet 包含已确认的图片或视频创意",
        repair: "请关联状态为 ready 的图片或视频创意后重新预检。",
      },
      {
        code: "budget_boundary",
        passed: typeof changeSet.budgetLimit === "number" && changeSet.budgetLimit > 0,
        message: "ChangeSet 设置了正数预算边界",
        repair: "请设置大于 0 的 budgetLimit 后重新预检。",
      },
    ];
    return { passed: checks.every((check) => check.passed), checks, checkedAt: new Date().toISOString() };
  }

  private requireCurrentPreflight(changeSet: ChangeSet, action: "approve" | "execute"): PreflightResult {
    const preflight = this.evaluatePreflight(changeSet);
    if (!preflight.passed) {
      throw new DomainError(
        "INVALID_STATE_TRANSITION",
        `Cannot ${action} change set because server preflight checks no longer pass`,
      );
    }
    return preflight;
  }

  private addAudit(
    projectId: string,
    actor: string | undefined,
    action: string,
    entityType: AuditEvent["entityType"],
    entityId: string,
    metadata: Record<string, unknown>,
  ): void {
    this.store.auditEvents.push({
      id: randomUUID(),
      projectId,
      actor: actor || "demo-user",
      action,
      entityType,
      entityId,
      metadata,
      createdAt: new Date().toISOString(),
    });
  }

  private async mutate(action: () => void): Promise<void> {
    action();
    await this.persist();
  }

  private async persist(): Promise<void> {
    await mkdir(dirname(this.filePath), { recursive: true });
    const temporaryPath = `${this.filePath}.tmp`;
    await writeFile(temporaryPath, JSON.stringify(this.store, null, 2), "utf8");
    await rename(temporaryPath, this.filePath);
  }
}

function touch(entity: { version: number; updatedAt: string }): Pick<Project, "version" | "updatedAt"> {
  return { version: entity.version + 1, updatedAt: new Date().toISOString() };
}

function withoutActor<T extends { actor?: string }>(input: T): Omit<T, "actor"> {
  const { actor: _actor, ...values } = input;
  return values;
}
