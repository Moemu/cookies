import type { ArkProvider, MediaGenerationKind } from "./ark-provider.js";
import type { Artifact, GenerationJob } from "./domain.js";
import { DomainError } from "./errors.js";
import type { FileRepository } from "./repository.js";

export interface GenerationService {
  generateBrief(input: GenerationRequest): Promise<{ job: PublicGenerationJob; artifact: Artifact }>;
  createMedia(input: MediaGenerationRequest): Promise<PublicGenerationJob>;
  syncMedia(jobId: string, actor?: string): Promise<PublicGenerationJob>;
  cancelMedia(jobId: string, actor?: string): Promise<PublicGenerationJob>;
}

export interface GenerationRequest {
  projectId: string;
  prompt: string;
  actor?: string;
}

export interface MediaGenerationRequest extends GenerationRequest {
  kind: MediaGenerationKind;
  briefId: string;
}

export type PublicGenerationJob = Omit<GenerationJob, "providerTaskId">;

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

    async createMedia(input) {
      const prompt = requiredPrompt(input.prompt);
      provider.ensureConfigured();
      await requireConfirmedBrief(repository, input.projectId, input.briefId);
      let job = await repository.createGenerationJob({
        projectId: input.projectId,
        artifactKind: input.kind,
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
          const artifact = await repository.createArtifact({
            projectId: input.projectId,
            kind: input.kind,
            content: result.assetUrl,
            status: "ready",
            sourceJobId: job.id,
            actor: input.actor,
          });
          job = await repository.transitionGenerationJob(job.id, "succeeded", {
            artifactId: artifact.id,
            actor: input.actor,
          });
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

    async syncMedia(jobId, actor) {
      const job = await requireMediaJob(repository, jobId);
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
        const artifact = await repository.createArtifact({
          projectId: job.projectId,
          kind: job.artifactKind,
          content: result.assetUrl,
          status: "ready",
          sourceJobId: job.id,
          actor,
        });
        return publicJob(await repository.transitionGenerationJob(job.id, "succeeded", {
          artifactId: artifact.id,
          actor,
        }));
      } catch (error) {
        return publicJob(await repository.updateGenerationJobDiagnostic(job.id, safeDiagnostic(error), actor));
      }
    },

    async cancelMedia(jobId, actor) {
      const job = await requireMediaJob(repository, jobId);
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

async function requireMediaJob(repository: FileRepository, jobId: string): Promise<GenerationJob> {
  const job = await repository.getGenerationJob(jobId);
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
  const { providerTaskId: _providerTaskId, ...safeJob } = job;
  return safeJob;
}

function safeDiagnostic(error: unknown): string {
  return error instanceof DomainError ? error.code : "INTERNAL_ERROR";
}
