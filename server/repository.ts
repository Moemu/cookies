import { randomUUID } from "node:crypto";
import { mkdir, readFile, rename, writeFile } from "node:fs/promises";
import { dirname } from "node:path";
import {
  type Artifact,
  type ArtifactKind,
  type ArtifactStatus,
  type AssetFeature,
  type AssetFeatureSimilarityRisk,
  assertAssetFeaturePayload,
  assertVideoMetadata,
  type BusinessTask,
  type BusinessTaskType,
  assertChangeSetTransition,
  assertGenerationJobTransition,
  type AuditEvent,
  type ChangeSet,
  emptyStore,
  type GenerationJob,
  type GenerationJobStatus,
  type OperationalRecord,
  type OperationalRecordKind,
  type PreflightCheck,
  type PreflightResult,
  type Project,
  type ProjectRuntime,
  type PrerollType,
  type ShortDramaPrerollArtifactSnapshot,
  type SimulationEvidence,
  type StoreData,
  type VideoPurpose,
} from "./domain.js";
import { DomainError } from "./errors.js";

export interface CreateProjectInput {
  name: string;
  brand: string;
  objective: string;
  runtime?: Partial<ProjectRuntime>;
  actor?: string;
}

export interface UpsertOperationalRecordInput {
  id: string;
  projectId: string;
  kind: OperationalRecordKind;
  title: string;
  status: string;
  occurredAt: string;
  fields: Record<string, string | number>;
}

export interface CreateArtifactInput {
  projectId: string;
  kind: ArtifactKind;
  purpose?: VideoPurpose;
  prerollType?: PrerollType;
  shortDramaPreroll?: ShortDramaPrerollArtifactSnapshot;
  content: string;
  status?: ArtifactStatus;
  sourceJobId?: string;
  actor?: string;
}

export interface AssetFeatureScope {
  organizationId: string;
  projectId: string;
  assetId?: string;
  assetVersion?: number;
  featureVersion?: string;
}

export interface UpsertAssetFeatureInput {
  organizationId: string;
  projectId: string;
  assetId: string;
  assetVersion: number;
  schemaVersion: "asset_feature_v1";
  featureVersion: string;
  hookStrength: number;
  productVisibility: number;
  sceneTags: string[];
  productTags: string[];
  personTags: string[];
  actionTags: string[];
  emotionTags: string[];
  sellingPoints: string[];
  ctaPresence: boolean;
  similarityGroup?: string;
  similarityRisk: AssetFeatureSimilarityRisk;
  evidence: string[];
  actor?: string;
}

export interface CreateBusinessTaskInput {
  projectId: string;
  type: BusinessTaskType;
  name: string;
  objective: string;
  sourceTaskIds?: string[];
  sourceArtifactIds?: string[];
  actor?: string;
}

export interface CreateGenerationJobInput {
  projectId: string;
  artifactKind: ArtifactKind;
  purpose?: VideoPurpose;
  prerollType?: PrerollType;
  shortDramaPreroll?: ShortDramaPrerollArtifactSnapshot;
  briefArtifactId?: string;
  model?: string;
  actor?: string;
}

export interface CompleteMediaGenerationInput {
  content: string;
  shortDramaPreroll?: ShortDramaPrerollArtifactSnapshot;
  actor?: string;
}

export interface ResourceScope {
  projectId?: string;
  purpose?: VideoPurpose;
  prerollType?: PrerollType;
}

export interface CreateChangeSetInput {
  projectId: string;
  name: string;
  artifactIds?: string[];
  budgetLimit?: number;
  actor?: string;
}

export class FileRepository {
  private mutationQueue: Promise<void> = Promise.resolve();

  private constructor(
    private readonly filePath: string,
    private store: StoreData,
  ) {}

  static async open(filePath: string): Promise<FileRepository> {
    try {
      const content = await readFile(filePath, "utf8");
      const parsed = JSON.parse(content) as Partial<StoreData>;
      return new FileRepository(filePath, {
        ...emptyStore(),
        ...parsed,
        projects: (parsed.projects ?? []).map((project) => ({
          ...project,
          runtime: normalizeRuntime(project.runtime),
        })),
        operationalRecords: parsed.operationalRecords ?? [],
        businessTasks: (parsed.businessTasks ?? []).map((task) => ({
          ...task,
          sourceTaskIds: task.sourceTaskIds ?? [],
        })),
        artifacts: parsed.artifacts ?? [],
        assetFeatures: parsed.assetFeatures ?? [],
        generationJobs: parsed.generationJobs ?? [],
        changeSets: parsed.changeSets ?? [],
        auditEvents: parsed.auditEvents ?? [],
      });
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
      runtime: normalizeRuntime(input.runtime),
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
    input: Partial<Pick<Project, "name" | "brand" | "objective" | "runtime">> & { actor?: string },
  ): Promise<Project> {
    const project = this.requireProject(id);
    await this.mutate(() => {
      Object.assign(project, withoutUndefined(withoutActor(input)), touch(project));
      this.addAudit(id, input.actor, "project.updated", "project", id, { version: project.version });
    });
    return project;
  }

  async listBusinessTasks(projectId?: string): Promise<BusinessTask[]> {
    return this.store.businessTasks.filter((task) => !projectId || task.projectId === projectId);
  }

  async listOperationalRecords(projectId?: string): Promise<OperationalRecord[]> {
    return this.store.operationalRecords
      .filter((record) => !projectId || record.projectId === projectId)
      .sort((left, right) => right.occurredAt.localeCompare(left.occurredAt) || left.id.localeCompare(right.id));
  }

  async upsertOperationalRecord(input: UpsertOperationalRecordInput): Promise<OperationalRecord> {
    this.requireProject(input.projectId);
    const existing = this.store.operationalRecords.find((record) => record.id === input.id);
    if (existing && existing.projectId !== input.projectId) {
      throw new DomainError("VALIDATION_ERROR", "Operational record ID already belongs to another project", [{
        field: "id",
        message: `Operational record ${input.id} belongs to project ${existing.projectId}`,
      }]);
    }
    const now = new Date().toISOString();
    const record: OperationalRecord = {
      ...input,
      createdAt: existing?.createdAt ?? now,
      updatedAt: now,
    };
    if (existing && operationalRecordEquals(existing, record)) return existing;
    await this.mutate(() => {
      if (existing) {
        Object.assign(existing, record);
      } else {
        this.store.operationalRecords.push(record);
      }
    });
    return record;
  }

  async getBusinessTask(id: string): Promise<BusinessTask | undefined> {
    return this.store.businessTasks.find((task) => task.id === id);
  }

  async createBusinessTask(input: CreateBusinessTaskInput): Promise<BusinessTask> {
    this.requireProject(input.projectId);
    const sourceTaskIds = input.sourceTaskIds ?? [];
    const sourceArtifactIds = input.sourceArtifactIds ?? [];
    this.requireProjectTasks(input.projectId, sourceTaskIds);
    this.requireProjectArtifacts(input.projectId, sourceArtifactIds);
    const now = new Date().toISOString();
    const task: BusinessTask = {
      id: randomUUID(),
      projectId: input.projectId,
      type: input.type,
      name: input.name,
      objective: input.objective,
      status: "draft",
      sourceTaskIds,
      sourceArtifactIds,
      outputArtifactIds: [],
      version: 1,
      createdAt: now,
      updatedAt: now,
    };
    await this.mutate(() => {
      this.store.businessTasks.push(task);
      this.addAudit(task.projectId, input.actor, "business_task.created", "business_task", task.id, {
        type: task.type,
        sourceTaskIds: task.sourceTaskIds,
        sourceArtifactIds: task.sourceArtifactIds,
      });
    });
    return task;
  }

  async updateBusinessTask(
    id: string,
    input: Partial<Pick<BusinessTask, "name" | "objective" | "status" | "sourceTaskIds" | "sourceArtifactIds" | "outputArtifactIds">> & { actor?: string },
  ): Promise<BusinessTask> {
    const task = this.requireBusinessTask(id);
    if (input.sourceTaskIds) this.requireProjectTasks(task.projectId, input.sourceTaskIds);
    if (input.sourceArtifactIds) this.requireProjectArtifacts(task.projectId, input.sourceArtifactIds);
    if (input.outputArtifactIds) this.requireProjectArtifacts(task.projectId, input.outputArtifactIds);
    await this.mutate(() => {
      Object.assign(task, withoutUndefined(withoutActor(input)), touch(task));
      this.addAudit(task.projectId, input.actor, "business_task.updated", "business_task", task.id, {
        status: task.status,
        version: task.version,
      });
    });
    return task;
  }

  async listArtifacts(scope?: string | ResourceScope): Promise<Artifact[]> {
    const filter = resourceScope(scope);
    return this.store.artifacts.filter((artifact) => matchesResourceScope(artifact, filter));
  }

  async getArtifact(id: string, scope?: ResourceScope): Promise<Artifact | undefined> {
    return this.store.artifacts.find((artifact) => artifact.id === id && matchesResourceScope(artifact, scope));
  }

  async createArtifact(input: CreateArtifactInput): Promise<Artifact> {
    this.requireProject(input.projectId);
    assertVideoMetadata(input.kind, input.purpose, input.prerollType);
    if (input.sourceJobId) this.requireMatchingSourceJob(input);
    const now = new Date().toISOString();
    const artifact: Artifact = {
      id: randomUUID(),
      projectId: input.projectId,
      kind: input.kind,
      purpose: input.purpose,
      prerollType: input.prerollType,
      shortDramaPreroll: input.shortDramaPreroll,
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
      Object.assign(artifact, withoutUndefined(withoutActor(input)), touch(artifact));
      this.addAudit(artifact.projectId, input.actor, "artifact.updated", "artifact", artifact.id, {
        version: artifact.version,
      });
    });
    return artifact;
  }

  async listAssetFeatures(scope: AssetFeatureScope): Promise<AssetFeature[]> {
    this.requireProject(scope.projectId);
    if (scope.assetId) this.requireScopedArtifact(scope.projectId, scope.assetId);
    return this.store.assetFeatures.filter((feature) => matchesAssetFeatureScope(feature, scope));
  }

  async getAssetFeature(scope: Required<AssetFeatureScope>): Promise<AssetFeature | undefined> {
    this.requireProject(scope.projectId);
    this.requireScopedArtifact(scope.projectId, scope.assetId);
    return this.store.assetFeatures.find((feature) => matchesAssetFeatureScope(feature, scope));
  }

  async upsertAssetFeature(input: UpsertAssetFeatureInput): Promise<AssetFeature> {
    this.requireProject(input.projectId);
    const asset = this.requireScopedArtifact(input.projectId, input.assetId);
    if ((asset.kind !== "image" && asset.kind !== "video") || asset.version !== input.assetVersion) {
      throw new DomainError("NOT_FOUND", "Resource was not found");
    }
    assertAssetFeaturePayload(input);
    const existing = this.store.assetFeatures.find((feature) => matchesAssetFeatureScope(feature, input));
    const now = new Date().toISOString();
    const feature: AssetFeature = {
      id: existing?.id ?? randomUUID(),
      organizationId: input.organizationId,
      projectId: input.projectId,
      assetId: input.assetId,
      assetVersion: input.assetVersion,
      schemaVersion: input.schemaVersion,
      featureVersion: input.featureVersion,
      hookStrength: input.hookStrength,
      productVisibility: input.productVisibility,
      sceneTags: [...input.sceneTags],
      productTags: [...input.productTags],
      personTags: [...input.personTags],
      actionTags: [...input.actionTags],
      emotionTags: [...input.emotionTags],
      sellingPoints: [...input.sellingPoints],
      ctaPresence: input.ctaPresence,
      similarityGroup: input.similarityGroup,
      similarityRisk: input.similarityRisk,
      evidence: [...input.evidence],
      version: existing ? existing.version + 1 : 1,
      createdAt: existing?.createdAt ?? now,
      updatedAt: now,
    };
    await this.mutate(() => {
      if (existing) {
        Object.assign(existing, feature);
      } else {
        this.store.assetFeatures.push(feature);
      }
      this.addAudit(feature.projectId, input.actor, "asset_feature.upserted", "asset_feature", feature.id, {
        assetId: feature.assetId,
        assetVersion: feature.assetVersion,
        featureVersion: feature.featureVersion,
        version: feature.version,
      });
    });
    return feature;
  }

  async listGenerationJobs(scope?: string | ResourceScope): Promise<GenerationJob[]> {
    const filter = resourceScope(scope);
    return this.store.generationJobs.filter((job) => matchesResourceScope(job, filter));
  }

  async getGenerationJob(id: string, scope?: ResourceScope): Promise<GenerationJob | undefined> {
    return this.store.generationJobs.find((job) => job.id === id && matchesResourceScope(job, scope));
  }

  async createGenerationJob(input: CreateGenerationJobInput): Promise<GenerationJob> {
    this.requireProject(input.projectId);
    assertVideoMetadata(input.artifactKind, input.purpose, input.prerollType);
    const now = new Date().toISOString();
    const job: GenerationJob = {
      id: randomUUID(),
      projectId: input.projectId,
      artifactKind: input.artifactKind,
      purpose: input.purpose,
      prerollType: input.prerollType,
      shortDramaPreroll: input.shortDramaPreroll,
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

  async completeMediaGenerationJob(
    id: string,
    input: CompleteMediaGenerationInput,
  ): Promise<{ job: GenerationJob; artifact: Artifact }> {
    const job = this.requireGenerationJob(id);
    assertGenerationJobTransition(job.status, "succeeded");
    const now = new Date().toISOString();
    const artifact: Artifact = {
      id: randomUUID(),
      projectId: job.projectId,
      kind: job.artifactKind,
      purpose: job.purpose,
      prerollType: job.prerollType,
      shortDramaPreroll: input.shortDramaPreroll ?? job.shortDramaPreroll,
      status: "ready",
      content: input.content,
      sourceJobId: job.id,
      version: 1,
      createdAt: now,
      updatedAt: now,
    };
    await this.mutate(() => {
      this.store.artifacts.push(artifact);
      this.addAudit(artifact.projectId, input.actor, "artifact.created", "artifact", artifact.id, {});
      Object.assign(job, { status: "succeeded", artifactId: artifact.id }, touch(job));
      this.addAudit(job.projectId, input.actor, "generation_job.status_changed", "generation_job", job.id, {
        from: "running",
        to: "succeeded",
      });
    });
    return { job, artifact };
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
    this.requireEligibleChangeSetArtifacts(input.projectId, input.artifactIds ?? []);
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

  private requireBusinessTask(id: string): BusinessTask {
    const task = this.store.businessTasks.find((item) => item.id === id);
    if (!task) throw new DomainError("NOT_FOUND", `Business task ${id} was not found`);
    return task;
  }

  private requireProjectArtifacts(projectId: string, artifactIds: string[]): void {
    const invalid = artifactIds.find((id) => !this.store.artifacts.some(
      (artifact) => artifact.id === id && artifact.projectId === projectId,
    ));
    if (invalid) {
      throw new DomainError("VALIDATION_ERROR", "Task artifacts must belong to the same project", [{
        field: "sourceArtifactIds",
        message: `Artifact ${invalid} does not belong to project ${projectId}`,
      }]);
    }
  }

  private requireProjectTasks(projectId: string, taskIds: string[]): void {
    const invalid = taskIds.find((id) => !this.store.businessTasks.some(
      (task) => task.id === id && task.projectId === projectId,
    ));
    if (invalid) {
      throw new DomainError("VALIDATION_ERROR", "Linked tasks must belong to the same project", [{
        field: "sourceTaskIds",
        message: `Task ${invalid} does not belong to project ${projectId}`,
      }]);
    }
  }

  private requireArtifact(id: string): Artifact {
    const artifact = this.store.artifacts.find((item) => item.id === id);
    if (!artifact) throw new DomainError("NOT_FOUND", `Artifact ${id} was not found`);
    return artifact;
  }

  private requireScopedArtifact(projectId: string, id: string): Artifact {
    const artifact = this.store.artifacts.find((item) => item.id === id && item.projectId === projectId);
    if (!artifact) throw new DomainError("NOT_FOUND", "Resource was not found");
    return artifact;
  }

  private requireMatchingSourceJob(input: CreateArtifactInput): void {
    const job = this.requireGenerationJob(input.sourceJobId!);
    if (
      job.projectId !== input.projectId
      || job.artifactKind !== input.kind
      || job.purpose !== input.purpose
      || job.prerollType !== input.prerollType
    ) {
      throw new DomainError("VALIDATION_ERROR", "Artifact source job must belong to the same project and purpose", [{
        field: "sourceJobId",
        message: `Generation job ${job.id} does not match this artifact's project or purpose`,
      }]);
    }
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
          (artifact) => this.isEligibleMainCreative(artifact) && artifact.status === "ready",
        ),
        message: "ChangeSet 包含当前 Project 已确认的主创意",
        repair: "请关联当前 Project 中 status 为 ready 且不属于前贴用途的图片或视频创意后重新预检。",
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

  private requireEligibleChangeSetArtifacts(projectId: string, artifactIds: string[]): void {
    for (const artifactId of artifactIds) {
      const artifact = this.store.artifacts.find((item) => item.id === artifactId);
      if (!artifact || artifact.projectId !== projectId) {
        throw new DomainError("VALIDATION_ERROR", "ChangeSet artifacts must belong to the current project", [{
          field: "artifactIds",
          message: `Artifact ${artifactId} does not belong to project ${projectId}`,
        }]);
      }
      if ((artifact.kind === "image" || artifact.kind === "video") && !this.isEligibleMainCreative(artifact)) {
        throw new DomainError("VALIDATION_ERROR", "Preroll assets cannot be used as ChangeSet main creatives", [{
          field: "artifactIds",
          message: `Artifact ${artifactId} is a preroll asset; select a current project main creative instead`,
        }]);
      }
    }
  }

  private isEligibleMainCreative(artifact: Artifact): boolean {
    return (artifact.kind === "image" || artifact.kind === "video")
      && artifact.purpose === undefined
      && artifact.prerollType === undefined;
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
    const mutation = this.mutationQueue.then(async () => {
      action();
      await this.persist();
    });
    // Keep later writes available when a prior write reports its error to the caller.
    this.mutationQueue = mutation.catch(() => undefined);
    await mutation;
  }

  private async persist(): Promise<void> {
    await mkdir(dirname(this.filePath), { recursive: true });
    const temporaryPath = `${this.filePath}.${randomUUID()}.tmp`;
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

function withoutUndefined<T extends object>(input: T): Partial<T> {
  return Object.fromEntries(
    Object.entries(input).filter(([, value]) => value !== undefined),
  ) as Partial<T>;
}

function normalizeRuntime(runtime: Partial<ProjectRuntime> | undefined): ProjectRuntime {
  return {
    code: runtime?.code ?? "PRJ",
    product: runtime?.product ?? "",
    stage: runtime?.stage ?? "需求梳理",
    progress: runtime?.progress ?? 0,
    status: runtime?.status ?? "active",
    owner: runtime?.owner ?? "demo-user",
    budget: runtime?.budget ?? 0,
    currency: "CNY",
    timezone: "Asia/Shanghai",
  };
}

function operationalRecordEquals(first: OperationalRecord, second: OperationalRecord): boolean {
  return first.projectId === second.projectId
    && first.kind === second.kind
    && first.title === second.title
    && first.status === second.status
    && first.occurredAt === second.occurredAt
    && JSON.stringify(first.fields) === JSON.stringify(second.fields);
}

function resourceScope(scope: string | ResourceScope | undefined): ResourceScope {
  return typeof scope === "string" ? { projectId: scope } : scope ?? {};
}

function matchesResourceScope(
  resource: Pick<Artifact, "projectId" | "purpose" | "prerollType">,
  scope: ResourceScope | undefined,
): boolean {
  return (!scope?.projectId || resource.projectId === scope.projectId)
    && (!scope?.purpose || resource.purpose === scope.purpose)
    && (!scope?.prerollType || resource.prerollType === scope.prerollType);
}

function matchesAssetFeatureScope(feature: AssetFeature, scope: AssetFeatureScope): boolean {
  return feature.organizationId === scope.organizationId
    && feature.projectId === scope.projectId
    && (scope.assetId === undefined || feature.assetId === scope.assetId)
    && (scope.assetVersion === undefined || feature.assetVersion === scope.assetVersion)
    && (scope.featureVersion === undefined || feature.featureVersion === scope.featureVersion);
}
