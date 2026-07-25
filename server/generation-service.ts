import type { ArkProvider, MediaGenerationKind } from "./ark-provider.js";
import type {
  Artifact,
  GenerationJob,
  PrerollType,
  ShortDramaPrerollArtifactSnapshot,
  VideoPurpose,
} from "./domain.js";
import { DomainError } from "./errors.js";
import type { FileRepository, ResourceScope } from "./repository.js";
import {
  buildShortDramaPrerollSnapshot,
  planShortDramaPreroll,
  type ShortDramaPrerollPlan,
  type ShortDramaStoryContext,
} from "./short-drama-planner.js";

export interface GenerationService {
  generateBrief(input: GenerationRequest): Promise<{ job: PublicGenerationJob; artifact: Artifact }>;
  planShortDramaPreroll(input: ShortDramaPrerollPlanRequest): Promise<ShortDramaPrerollPlan>;
  createMedia(input: MediaGenerationRequest): Promise<PublicGenerationJob>;
  syncMedia(jobId: string, actor?: string, scope?: ResourceScope): Promise<PublicGenerationJob>;
  cancelMedia(jobId: string, actor?: string, scope?: ResourceScope): Promise<PublicGenerationJob>;
}

export interface GenerationRequest {
  projectId: string;
  prompt: string;
  actor?: string;
}

export interface MediaGenerationRequest {
  projectId: string;
  prompt?: string;
  actor?: string;
  kind: MediaGenerationKind;
  briefId: string;
  purpose?: VideoPurpose;
  prerollType?: PrerollType;
  shortDramaPlanVersion?: string;
  shortDramaCandidateId?: string;
  storyContext?: ShortDramaStoryContext;
}

export interface ShortDramaPrerollPlanRequest {
  projectId: string;
  briefId: string;
  storyContext: ShortDramaStoryContext;
}

export type PublicGenerationJob = Omit<GenerationJob, "providerTaskId" | "shortDramaPreroll">;

export function createGenerationService(
  repository: FileRepository,
  provider: ArkProvider,
): GenerationService {
  return {
    async generateBrief(input) {
      const prompt = requiredPrompt(input.prompt);
      provider.ensureConfigured();
      let job = await repository.createGenerationJob({
        projectId: input.projectId,
        artifactKind: "brief",
        model: provider.config.models.text,
        actor: input.actor,
      });
      job = await repository.transitionGenerationJob(job.id, "running", { actor: input.actor });
      try {
        const content = await provider.generateText(prompt);
        const artifact = await repository.createArtifact({
          projectId: input.projectId,
          kind: "brief",
          content,
          sourceJobId: job.id,
          actor: input.actor,
        });
        job = await repository.transitionGenerationJob(job.id, "succeeded", {
          artifactId: artifact.id,
          actor: input.actor,
        });
        return { job: publicJob(job), artifact };
      } catch (error) {
        await repository.transitionGenerationJob(job.id, "failed", {
          diagnostic: safeDiagnostic(error),
          actor: input.actor,
        });
        throw error;
      }
    },

    async planShortDramaPreroll(input) {
      await requireProject(repository, input.projectId);
      await requireConfirmedBrief(repository, input.projectId, input.briefId);
      return planShortDramaPreroll({ prerollType: "short_drama", storyContext: input.storyContext });
    },

    async createMedia(input) {
      const brief = await requireConfirmedBrief(repository, input.projectId, input.briefId);
      const shortDramaPreroll = createShortDramaSnapshot(input, brief);
      const prompt = shortDramaPreroll?.prompt ?? requiredPrompt(input.prompt ?? "");
      provider.ensureConfigured();
      let job = await repository.createGenerationJob({
        projectId: input.projectId,
        artifactKind: input.kind,
        purpose: input.purpose,
        prerollType: input.prerollType,
        shortDramaPreroll,
        briefArtifactId: input.briefId,
        model: provider.config.models[input.kind],
        actor: input.actor,
      });
      job = await repository.transitionGenerationJob(job.id, "running", { actor: input.actor });
      try {
        const result = await provider.createMedia(input.kind, prompt);
        if (result.providerTaskId) {
          job = await repository.setGenerationJobProviderTask(job.id, result.providerTaskId, input.actor);
        }
        if (result.assetUrl) {
          ({ job } = await repository.completeMediaGenerationJob(job.id, {
            content: result.assetUrl,
            shortDramaPreroll,
            actor: input.actor,
          }));
        }
        return publicJob(job);
      } catch (error) {
        await repository.transitionGenerationJob(job.id, "failed", {
          diagnostic: safeDiagnostic(error),
          actor: input.actor,
        });
        throw error;
      }
    },

    async syncMedia(jobId, actor, scope) {
      const job = await requireMediaJob(repository, jobId, scope);
      if (job.status === "succeeded" || job.status === "failed" || job.status === "cancelled") {
        return publicJob(job);
      }
      if (!job.providerTaskId) {
        return publicJob(await repository.updateGenerationJobDiagnostic(job.id, "PROVIDER_SYNC_UNAVAILABLE", actor));
      }
      try {
        const result = await provider.getMediaTask(job.providerTaskId);
        if (result.status === "queued") {
          return publicJob(await repository.updateGenerationJobDiagnostic(job.id, undefined, actor));
        }
        if (result.status === "running") {
          return publicJob(await repository.updateGenerationJobDiagnostic(job.id, undefined, actor));
        }
        if (result.status === "unknown") {
          return publicJob(await repository.updateGenerationJobDiagnostic(job.id, "PROVIDER_STATUS_UNKNOWN", actor));
        }
        if (result.status === "failed") {
          return publicJob(await repository.transitionGenerationJob(job.id, "failed", {
            diagnostic: result.diagnostic ? "PROVIDER_TASK_FAILED" : "PROVIDER_TASK_FAILED",
            actor,
          }));
        }
        if (result.status === "cancelled") {
          return publicJob(await repository.transitionGenerationJob(job.id, "cancelled", { actor }));
        }
        if (!result.assetUrl) {
          return publicJob(await repository.transitionGenerationJob(job.id, "failed", {
            diagnostic: "PROVIDER_INVALID_RESPONSE",
            actor,
          }));
        }
        const completed = await repository.completeMediaGenerationJob(job.id, {
          content: result.assetUrl,
          actor,
        });
        return publicJob(completed.job);
      } catch (error) {
        const current = await repository.getGenerationJob(job.id, scope);
        if (current?.status === "succeeded" || current?.status === "failed" || current?.status === "cancelled") {
          return publicJob(current);
        }
        return publicJob(await repository.updateGenerationJobDiagnostic(job.id, safeDiagnostic(error), actor));
      }
    },

    async cancelMedia(jobId, actor, scope) {
      const job = await requireMediaJob(repository, jobId, scope);
      if (job.status === "cancelled") return publicJob(job);
      if (job.status === "succeeded" || job.status === "failed") {
        throw new DomainError("INVALID_STATE_TRANSITION", `Cannot cancel a ${job.status} generation job`);
      }
      if (job.providerTaskId) await provider.cancelMedia(job.providerTaskId);
      return publicJob(await repository.transitionGenerationJob(job.id, "cancelled", { actor }));
    },
  };
}

async function requireConfirmedBrief(
  repository: FileRepository,
  projectId: string,
  briefId: string,
): Promise<Artifact> {
  const brief = await repository.getArtifact(briefId);
  if (!brief || brief.projectId !== projectId || brief.kind !== "brief" || brief.status !== "ready") {
    throw new DomainError(
      "BRIEF_NOT_CONFIRMED",
      "A confirmed Brief from this project is required before generating media",
    );
  }
  return brief;
}

async function requireMediaJob(
  repository: FileRepository,
  jobId: string,
  scope?: ResourceScope,
): Promise<GenerationJob> {
  const job = await repository.getGenerationJob(jobId, scope);
  if (!job) throw new DomainError("NOT_FOUND", "Generation job was not found");
  if (job.artifactKind !== "image" && job.artifactKind !== "video") {
    throw new DomainError("VALIDATION_ERROR", "Only media generation jobs can be managed");
  }
  return job;
}

function requiredPrompt(value: string): string {
  if (!value.trim()) throw new DomainError("VALIDATION_ERROR", "prompt must be a non-empty string");
  if (value.length > 12_000) throw new DomainError("VALIDATION_ERROR", "prompt must not exceed 12000 characters");
  return value.trim();
}

function publicJob(job: GenerationJob): PublicGenerationJob {
  const {
    providerTaskId: _providerTaskId,
    shortDramaPreroll: _shortDramaPreroll,
    ...safeJob
  } = job;
  return safeJob;
}

function safeDiagnostic(error: unknown): string {
  return error instanceof DomainError ? error.code : "INTERNAL_ERROR";
}

async function requireProject(repository: FileRepository, projectId: string): Promise<void> {
  if (!await repository.getProject(projectId)) {
    throw new DomainError("NOT_FOUND", "Project was not found");
  }
}

function createShortDramaSnapshot(
  input: MediaGenerationRequest,
  brief: Artifact,
): ShortDramaPrerollArtifactSnapshot | undefined {
  if (input.prerollType !== "short_drama") return undefined;
  if (input.kind !== "video" || input.purpose !== "preroll") {
    throw new DomainError("VALIDATION_ERROR", "Short drama generation requires a preroll video");
  }
  if (!input.shortDramaPlanVersion || !input.shortDramaCandidateId || !input.storyContext) {
    throw new DomainError(
      "VALIDATION_ERROR",
      "Short drama generation requires a plan version, selected candidate, and story context",
    );
  }
  const plan = planShortDramaPreroll({ prerollType: "short_drama", storyContext: input.storyContext });
  if (input.shortDramaPlanVersion !== plan.version) {
    throw new DomainError("VALIDATION_ERROR", "Unsupported short drama plan version");
  }
  return buildShortDramaPrerollSnapshot({
    plan,
    candidateId: input.shortDramaCandidateId,
    storyContext: input.storyContext,
    confirmedBrief: brief.content,
  });
}
