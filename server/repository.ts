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
  type ChangeSetStatus,
  emptyStore,
  type GenerationJob,
  type GenerationJobStatus,
  type Project,
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

  async transitionChangeSet(
    id: string,
    status: ChangeSetStatus,
    actor?: string,
  ): Promise<ChangeSet> {
    const changeSet = this.requireChangeSet(id);
    assertChangeSetTransition(changeSet.status, status);
    const previousStatus = changeSet.status;
    await this.mutate(() => {
      Object.assign(changeSet, { status }, touch(changeSet));
      this.addAudit(changeSet.projectId, actor, "change_set.status_changed", "change_set", changeSet.id, {
        from: previousStatus,
        to: status,
      });
    });
    return changeSet;
  }

  async listAuditEvents(projectId?: string): Promise<AuditEvent[]> {
    return this.store.auditEvents.filter((event) => !projectId || event.projectId === projectId);
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
