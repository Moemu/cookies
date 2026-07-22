import { createServer, type IncomingMessage, type Server, type ServerResponse } from "node:http";
import { resolve } from "node:path";
import {
  isArtifactKind,
  isChangeSetStatus,
  isGenerationJobStatus,
  type ArtifactStatus,
} from "./domain.js";
import { createArkProvider, loadArkConfig, publicCapabilities } from "./ark-provider.js";
import { DomainError, errorStatus, isDomainError } from "./errors.js";
import { createGenerationService, type GenerationService } from "./generation-service.js";
import { FileRepository } from "./repository.js";

const MAX_BODY_BYTES = 1_000_000;
const artifactStatuses: readonly ArtifactStatus[] = ["draft", "ready", "archived"];

export interface AppOptions {
  repository: FileRepository;
  generationService?: GenerationService;
}

export function createApp({ repository, generationService }: AppOptions): Server {
  const config = loadArkConfig();
  const service = generationService ?? createGenerationService(repository, createArkProvider(config));
  return createServer(async (request, response) => {
    try {
      await route(request, response, repository, service, () => publicCapabilities(config));
    } catch (error) {
      sendError(response, error);
    }
  });
}

async function route(
  request: IncomingMessage,
  response: ServerResponse,
  repository: FileRepository,
  generationService: GenerationService,
  capabilities: () => Record<string, unknown>,
): Promise<void> {
  setCorsHeaders(response);
  if (request.method === "OPTIONS") {
    response.writeHead(204).end();
    return;
  }

  const method = request.method ?? "GET";
  const url = new URL(request.url ?? "/", "http://localhost");
  const segments = url.pathname.split("/").filter(Boolean);
  if (method === "GET" && url.pathname === "/health") {
    sendJson(response, 200, { status: "ok" });
    return;
  }
  if (segments[0] !== "api") throw new DomainError("ROUTE_NOT_FOUND", "Route was not found");

  const resource = segments[1];
  const id = segments[2];
  const action = segments[3];
  if (resource === "provider" && id === "capabilities" && method === "GET") {
    sendJson(response, 200, capabilities());
    return;
  }
  if (resource === "generation") {
    await generationRoute(method, id, request, response, generationService);
    return;
  }
  if (resource === "projects") {
    await projectsRoute(method, id, request, response, repository);
    return;
  }
  if (resource === "artifacts") {
    await artifactsRoute(method, id, request, response, repository, url.searchParams.get("projectId"));
    return;
  }
  if (resource === "generation-jobs") {
    if (action === "cancel" && method === "POST" && id) {
      const body = await readBody(request);
      return sendJson(response, 200, await generationService.cancelMedia(id, optionalString(body, "actor")));
    }
    await jobsRoute(method, id, request, response, repository, url.searchParams.get("projectId"));
    return;
  }
  if (resource === "change-sets") {
    await changeSetsRoute(method, id, request, response, repository, url.searchParams.get("projectId"));
    return;
  }
  if (resource === "audit-events") {
    await auditEventsRoute(method, id, response, repository, url.searchParams.get("projectId"));
    return;
  }
  throw new DomainError("ROUTE_NOT_FOUND", "Route was not found");
}

async function generationRoute(
  method: string,
  operation: string | undefined,
  request: IncomingMessage,
  response: ServerResponse,
  generationService: GenerationService,
): Promise<void> {
  if (method !== "POST" || (operation !== "text" && operation !== "media")) {
    throw new DomainError("METHOD_NOT_ALLOWED", "Method is not allowed for this route");
  }
  const body = await readBody(request);
  const input = {
    projectId: requiredString(body, "projectId"),
    prompt: requiredString(body, "prompt"),
    actor: optionalString(body, "actor"),
  };
  if (operation === "text") {
    return sendJson(response, 201, await generationService.generateBrief(input));
  }
  const kind = body.kind;
  if (kind !== "image" && kind !== "video") invalidField("kind", "Must be image or video");
  return sendJson(response, 202, await generationService.createMedia({ ...input, kind }));
}

async function projectsRoute(
  method: string,
  id: string | undefined,
  request: IncomingMessage,
  response: ServerResponse,
  repository: FileRepository,
): Promise<void> {
  if (!id && method === "GET") return sendJson(response, 200, await repository.listProjects());
  if (!id && method === "POST") {
    const body = await readBody(request);
    return sendJson(response, 201, await repository.createProject({
      name: requiredString(body, "name"),
      brand: requiredString(body, "brand"),
      objective: requiredString(body, "objective"),
      actor: optionalString(body, "actor"),
    }));
  }
  if (id && method === "GET") return sendFound(response, await repository.getProject(id));
  if (id && method === "PATCH") {
    const body = await readBody(request);
    requireAny(body, ["name", "brand", "objective"]);
    return sendJson(response, 200, await repository.updateProject(id, {
      name: optionalString(body, "name"),
      brand: optionalString(body, "brand"),
      objective: optionalString(body, "objective"),
      actor: optionalString(body, "actor"),
    }));
  }
  throw new DomainError("METHOD_NOT_ALLOWED", "Method is not allowed for this route");
}

async function artifactsRoute(
  method: string,
  id: string | undefined,
  request: IncomingMessage,
  response: ServerResponse,
  repository: FileRepository,
  projectId: string | null,
): Promise<void> {
  if (!id && method === "GET") return sendJson(response, 200, await repository.listArtifacts(projectId ?? undefined));
  if (!id && method === "POST") {
    const body = await readBody(request);
    const status = optionalString(body, "status");
    if (status !== undefined && !artifactStatuses.includes(status as ArtifactStatus)) {
      invalidField("status", "Must be draft, ready, or archived");
    }
    return sendJson(response, 201, await repository.createArtifact({
      projectId: requiredString(body, "projectId"),
      kind: requiredArtifactKind(body, "kind"),
      content: requiredString(body, "content"),
      status: status as ArtifactStatus | undefined,
      sourceJobId: optionalString(body, "sourceJobId"),
      actor: optionalString(body, "actor"),
    }));
  }
  if (id && method === "GET") return sendFound(response, await repository.getArtifact(id));
  if (id && method === "PATCH") {
    const body = await readBody(request);
    requireAny(body, ["content", "status", "sourceJobId"]);
    const status = optionalString(body, "status");
    if (status !== undefined && !artifactStatuses.includes(status as ArtifactStatus)) {
      invalidField("status", "Must be draft, ready, or archived");
    }
    return sendJson(response, 200, await repository.updateArtifact(id, {
      content: optionalString(body, "content"),
      status: status as ArtifactStatus | undefined,
      sourceJobId: optionalString(body, "sourceJobId"),
      actor: optionalString(body, "actor"),
    }));
  }
  throw new DomainError("METHOD_NOT_ALLOWED", "Method is not allowed for this route");
}

async function jobsRoute(
  method: string,
  id: string | undefined,
  request: IncomingMessage,
  response: ServerResponse,
  repository: FileRepository,
  projectId: string | null,
): Promise<void> {
  if (!id && method === "GET") {
    return sendJson(response, 200, (await repository.listGenerationJobs(projectId ?? undefined)).map(publicJob));
  }
  if (!id && method === "POST") {
    const body = await readBody(request);
    return sendJson(response, 201, await repository.createGenerationJob({
      projectId: requiredString(body, "projectId"),
      artifactKind: requiredArtifactKind(body, "artifactKind"),
      model: optionalString(body, "model"),
      actor: optionalString(body, "actor"),
    }));
  }
  if (id && method === "GET") {
    const job = await repository.getGenerationJob(id);
    return sendFound(response, job === undefined ? undefined : publicJob(job));
  }
  if (id && method === "PATCH") {
    const body = await readBody(request);
    const status = body.status;
    if (!isGenerationJobStatus(status)) invalidField("status", "Must be a generation job status");
    return sendJson(response, 200, await repository.transitionGenerationJob(id, status, {
      diagnostic: optionalString(body, "diagnostic"),
      artifactId: optionalString(body, "artifactId"),
      actor: optionalString(body, "actor"),
    }));
  }
  throw new DomainError("METHOD_NOT_ALLOWED", "Method is not allowed for this route");
}

function publicJob<T extends { providerTaskId?: string }>(job: T): Omit<T, "providerTaskId"> {
  const { providerTaskId: _providerTaskId, ...safeJob } = job;
  return safeJob;
}

async function changeSetsRoute(
  method: string,
  id: string | undefined,
  request: IncomingMessage,
  response: ServerResponse,
  repository: FileRepository,
  projectId: string | null,
): Promise<void> {
  if (!id && method === "GET") return sendJson(response, 200, await repository.listChangeSets(projectId ?? undefined));
  if (!id && method === "POST") {
    const body = await readBody(request);
    return sendJson(response, 201, await repository.createChangeSet({
      projectId: requiredString(body, "projectId"),
      name: requiredString(body, "name"),
      artifactIds: optionalStringArray(body, "artifactIds"),
      budgetLimit: optionalNonNegativeNumber(body, "budgetLimit"),
      actor: optionalString(body, "actor"),
    }));
  }
  if (id && method === "GET") return sendFound(response, await repository.getChangeSet(id));
  if (id && method === "PATCH") {
    const body = await readBody(request);
    if (!isChangeSetStatus(body.status)) invalidField("status", "Must be a change set status");
    return sendJson(response, 200, await repository.transitionChangeSet(id, body.status, optionalString(body, "actor")));
  }
  throw new DomainError("METHOD_NOT_ALLOWED", "Method is not allowed for this route");
}

async function auditEventsRoute(
  method: string,
  id: string | undefined,
  response: ServerResponse,
  repository: FileRepository,
  projectId: string | null,
): Promise<void> {
  if (!id && method === "GET") return sendJson(response, 200, await repository.listAuditEvents(projectId ?? undefined));
  if (id && method === "GET") return sendFound(response, await repository.getAuditEvent(id));
  throw new DomainError("METHOD_NOT_ALLOWED", "Method is not allowed for this route");
}

async function readBody(request: IncomingMessage): Promise<Record<string, unknown>> {
  let body = "";
  for await (const chunk of request) {
    body += chunk;
    if (Buffer.byteLength(body) > MAX_BODY_BYTES) {
      throw new DomainError("PAYLOAD_TOO_LARGE", "Request body exceeds 1 MB");
    }
  }
  if (!body) return {};
  try {
    const parsed: unknown = JSON.parse(body);
    if (!isRecord(parsed)) throw new Error("Expected an object");
    return parsed;
  } catch {
    throw new DomainError("INVALID_JSON", "Request body must be a JSON object");
  }
}

function requiredString(body: Record<string, unknown>, field: string): string {
  const value = optionalString(body, field);
  if (value === undefined) invalidField(field, "Required non-empty string");
  return value;
}

function optionalString(body: Record<string, unknown>, field: string): string | undefined {
  const value = body[field];
  if (value === undefined) return undefined;
  if (typeof value !== "string" || !value.trim()) invalidField(field, "Must be a non-empty string");
  return value.trim();
}

function requiredArtifactKind(body: Record<string, unknown>, field: string) {
  if (!isArtifactKind(body[field])) invalidField(field, "Must be brief, image, video, or document");
  return body[field];
}

function optionalStringArray(body: Record<string, unknown>, field: string): string[] | undefined {
  const value = body[field];
  if (value === undefined) return undefined;
  if (!Array.isArray(value) || value.some((item) => typeof item !== "string" || !item.trim())) {
    invalidField(field, "Must be an array of non-empty strings");
  }
  return value;
}

function optionalNonNegativeNumber(body: Record<string, unknown>, field: string): number | undefined {
  const value = body[field];
  if (value === undefined) return undefined;
  if (typeof value !== "number" || !Number.isFinite(value) || value < 0) {
    invalidField(field, "Must be a non-negative finite number");
  }
  return value;
}

function requireAny(body: Record<string, unknown>, fields: string[]): void {
  if (!fields.some((field) => body[field] !== undefined)) {
    throw new DomainError("VALIDATION_ERROR", "At least one updatable field is required", [{
      message: `Provide one of: ${fields.join(", ")}`,
    }]);
  }
}

function invalidField(field: string, message: string): never {
  throw new DomainError("VALIDATION_ERROR", `${field} ${message.toLowerCase()}`, [{ field, message }]);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function sendFound<T>(response: ServerResponse, entity: T | undefined): void {
  if (entity === undefined) throw new DomainError("NOT_FOUND", "Resource was not found");
  sendJson(response, 200, entity);
}

function setCorsHeaders(response: ServerResponse): void {
  response.setHeader("Access-Control-Allow-Origin", "http://127.0.0.1:5173");
  response.setHeader("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS");
  response.setHeader("Access-Control-Allow-Headers", "Content-Type");
}

function sendJson(response: ServerResponse, status: number, body: unknown): void {
  response.writeHead(status, { "Content-Type": "application/json; charset=utf-8" });
  response.end(JSON.stringify(body));
}

function sendError(response: ServerResponse, error: unknown): void {
  const domainError = isDomainError(error)
    ? error
    : new DomainError("INTERNAL_ERROR", "An unexpected server error occurred");
  sendJson(response, errorStatus(domainError.code), {
    error: {
      code: domainError.code,
      message: domainError.message,
      ...(domainError.details ? { details: domainError.details } : {}),
    },
  });
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(new URL(import.meta.url).pathname)) {
  const port = Number(process.env.PORT ?? 8787);
  const repository = await FileRepository.open(resolve(process.cwd(), "data/mvp-store.json"));
  createApp({ repository }).listen(port, "127.0.0.1", () => {
    console.log(`MVP API listening on http://127.0.0.1:${port}`);
  });
}
