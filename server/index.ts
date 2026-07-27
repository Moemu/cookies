import { createServer, type IncomingMessage, type Server, type ServerResponse } from "node:http";
import { rm } from "node:fs/promises";
import { resolve } from "node:path";
import {
  isArtifactKind,
  isBusinessTaskStatus,
  isBusinessTaskType,
  isGenerationJobStatus,
  isOperationalRecordKind,
  isPrerollType,
  isVideoPurpose,
  type ArtifactStatus,
  type BusinessTaskStatus,
  type PrerollType,
  type VideoPurpose,
} from "./domain.js";
import { createArkProvider, loadArkConfig, publicCapabilities } from "./ark-provider.js";
import { seedDemoProject } from "./demo.js";
import { DomainError, errorStatus, isDomainError } from "./errors.js";
import { createGenerationService, type GenerationService } from "./generation-service.js";
import { FileRepository, type ResourceScope } from "./repository.js";
import type { ShortDramaStoryContext } from "./short-drama-planner.js";

const MAX_BODY_BYTES = 1_000_000;
const artifactStatuses: readonly ArtifactStatus[] = ["draft", "ready", "archived"];

export interface AppOptions {
  repository: FileRepository;
  generationService?: GenerationService;
}

export async function openSeededRepository(filePath: string): Promise<FileRepository> {
  const repository = await FileRepository.open(filePath);
  await seedDemoProject(repository);
  return repository;
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
  setCorsHeaders(request, response);
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
  if (resource === "short-drama-preroll-plans") {
    await shortDramaPrerollPlansRoute(method, request, response, generationService);
    return;
  }
  if (resource === "projects") {
    await projectsRoute(method, id, action, request, response, repository);
    return;
  }
  if (resource === "asset-features") {
    await assetFeaturesRoute(method, request, response, repository, url.searchParams);
    return;
  }
  if (resource === "artifacts") {
    await artifactsRoute(method, id, request, response, repository, resourceScope(url.searchParams));
    return;
  }
  if (resource === "tasks") {
    await businessTasksRoute(method, id, request, response, repository, url.searchParams.get("projectId"));
    return;
  }
  if (resource === "generation-jobs") {
    if (action === "cancel" && method === "POST" && id) {
      const body = await readBody(request);
      return sendJson(response, 200, await generationService.cancelMedia(
        id,
        optionalString(body, "actor"),
        resourceScope(url.searchParams),
      ));
    }
    if (id && method === "GET") {
      return sendJson(response, 200, await generationService.syncMedia(id, undefined, resourceScope(url.searchParams)));
    }
    await jobsRoute(method, id, request, response, repository, resourceScope(url.searchParams));
    return;
  }
  if (resource === "change-sets") {
    if (id && action === "preflight" && method === "POST") {
      const body = await readBody(request);
      return sendJson(response, 200, await repository.preflightChangeSet(id, optionalString(body, "actor")));
    }
    if (id && action === "approve" && method === "POST") {
      const body = await readBody(request);
      return sendJson(response, 200, await repository.approveChangeSet(
        id,
        requiredString(body, "actor"),
        requiredString(body, "role"),
      ));
    }
    if (id && action === "execute" && method === "POST") {
      const body = await readBody(request);
      return sendJson(response, 200, await repository.executeChangeSetSimulation(id, optionalString(body, "actor")));
    }
    if (id && action === "rollback" && method === "POST") {
      const body = await readBody(request);
      return sendJson(response, 200, await repository.rollbackChangeSetSimulation(
        id,
        requiredString(body, "reason"),
        optionalString(body, "actor"),
      ));
    }
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
  const input = { projectId: requiredString(body, "projectId"), actor: optionalString(body, "actor") };
  if (operation === "text") {
    return sendJson(response, 201, await generationService.generateBrief({
      ...input,
      prompt: requiredString(body, "prompt"),
    }));
  }
  const kind = body.kind;
  if (kind !== "image" && kind !== "video") invalidField("kind", "Must be image or video");
  const metadata = videoMetadata(body, kind);
  const isShortDramaPreroll = metadata.prerollType === "short_drama";
  return sendJson(response, 202, await generationService.createMedia({
    ...input,
    kind,
    briefId: requiredString(body, "briefId"),
    prompt: isShortDramaPreroll ? rawString(body, "prompt") : requiredString(body, "prompt"),
    ...metadata,
    ...(isShortDramaPreroll ? {
      shortDramaPlanVersion: requiredString(body, "shortDramaPlanVersion"),
      shortDramaCandidateId: requiredString(body, "shortDramaCandidateId"),
      storyContext: requiredShortDramaStoryContext(body),
    } : {}),
  }));
}

async function shortDramaPrerollPlansRoute(
  method: string,
  request: IncomingMessage,
  response: ServerResponse,
  generationService: GenerationService,
): Promise<void> {
  if (method !== "POST") throw new DomainError("METHOD_NOT_ALLOWED", "Method is not allowed for this route");
  const body = await readBody(request);
  return sendJson(response, 200, await generationService.planShortDramaPreroll({
    projectId: requiredString(body, "projectId"),
    briefId: requiredString(body, "briefId"),
    storyContext: requiredShortDramaStoryContext(body),
  }));
}

async function projectsRoute(
  method: string,
  id: string | undefined,
  action: string | undefined,
  request: IncomingMessage,
  response: ServerResponse,
  repository: FileRepository,
): Promise<void> {
  if (!id && method === "GET") return sendJson(response, 200, await repository.listProjects());
  if (id && action === "operations" && method === "GET") {
    if (!await repository.getProject(id)) {
      throw new DomainError("NOT_FOUND", "Resource was not found");
    }
    return sendJson(response, 200, await repository.listOperationalRecords(id));
  }
  if (id && action === "operations" && method === "POST") {
    const body = await readBody(request);
    return sendJson(response, 201, await repository.upsertOperationalRecord({
      id: requiredString(body, "id"),
      projectId: id,
      kind: requiredOperationalRecordKind(body, "kind"),
      title: requiredString(body, "title"),
      status: requiredString(body, "status"),
      occurredAt: requiredString(body, "occurredAt"),
      fields: requiredFields(body),
    }));
  }
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
  scope: ResourceScope,
): Promise<void> {
  if (!id && method === "GET") return sendJson(response, 200, await repository.listArtifacts(scope));
  if (!id && method === "POST") {
    const body = await readBody(request);
    const status = optionalString(body, "status");
    if (status !== undefined && !artifactStatuses.includes(status as ArtifactStatus)) {
      invalidField("status", "Must be draft, ready, or archived");
    }
    return sendJson(response, 201, await repository.createArtifact({
      projectId: requiredString(body, "projectId"),
      kind: requiredArtifactKind(body, "kind"),
      ...videoMetadata(body, requiredArtifactKind(body, "kind")),
      content: requiredString(body, "content"),
      status: status as ArtifactStatus | undefined,
      sourceJobId: optionalString(body, "sourceJobId"),
      actor: optionalString(body, "actor"),
    }));
  }
  if (id && method === "GET") return sendFound(response, await repository.getArtifact(id, scope));
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
  scope: ResourceScope,
): Promise<void> {
  if (!id && method === "GET") {
    return sendJson(response, 200, (await repository.listGenerationJobs(scope)).map(publicJob));
  }
  if (!id && method === "POST") {
    const body = await readBody(request);
    return sendJson(response, 201, await repository.createGenerationJob({
      projectId: requiredString(body, "projectId"),
      artifactKind: requiredArtifactKind(body, "artifactKind"),
      ...videoMetadata(body, requiredArtifactKind(body, "artifactKind")),
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

async function businessTasksRoute(
  method: string,
  id: string | undefined,
  request: IncomingMessage,
  response: ServerResponse,
  repository: FileRepository,
  projectId: string | null,
): Promise<void> {
  if (!id && method === "GET") {
    return sendJson(response, 200, await repository.listBusinessTasks(projectId ?? undefined));
  }
  if (!id && method === "POST") {
    const body = await readBody(request);
    const type = body.type;
    if (!isBusinessTaskType(type)) invalidField("type", "Must be a supported business task type");
    return sendJson(response, 201, await repository.createBusinessTask({
      projectId: requiredString(body, "projectId"),
      type,
      name: requiredString(body, "name"),
      objective: requiredString(body, "objective"),
      sourceTaskIds: optionalStringArray(body, "sourceTaskIds"),
      sourceArtifactIds: optionalStringArray(body, "sourceArtifactIds"),
      actor: optionalString(body, "actor"),
    }));
  }
  if (id && method === "GET") return sendFound(response, await repository.getBusinessTask(id));
  if (id && method === "PATCH") {
    const body = await readBody(request);
    requireAny(body, ["name", "objective", "status", "sourceTaskIds", "sourceArtifactIds", "outputArtifactIds"]);
    const status = body.status;
    if (status !== undefined && !isBusinessTaskStatus(status)) {
      invalidField("status", "Must be a supported business task status");
    }
    return sendJson(response, 200, await repository.updateBusinessTask(id, {
      name: optionalString(body, "name"),
      objective: optionalString(body, "objective"),
      status: status as BusinessTaskStatus | undefined,
      sourceTaskIds: optionalStringArray(body, "sourceTaskIds"),
      sourceArtifactIds: optionalStringArray(body, "sourceArtifactIds"),
      outputArtifactIds: optionalStringArray(body, "outputArtifactIds"),
      actor: optionalString(body, "actor"),
    }));
  }
  throw new DomainError("METHOD_NOT_ALLOWED", "Method is not allowed for this route");
}

async function assetFeaturesRoute(
  method: string,
  request: IncomingMessage,
  response: ServerResponse,
  repository: FileRepository,
  searchParams: URLSearchParams,
): Promise<void> {
  if (method === "GET") {
    const scope = assetFeatureScope(searchParams);
    const hasExactLookup = scope.assetId !== undefined || scope.assetVersion !== undefined || scope.featureVersion !== undefined;
    if (hasExactLookup) {
      if (scope.assetId === undefined || scope.assetVersion === undefined || scope.featureVersion === undefined) {
        invalidField("assetId", "Must be provided with assetVersion and featureVersion for exact lookup");
      }
      const feature = await repository.getAssetFeature({
        organizationId: scope.organizationId,
        projectId: scope.projectId,
        assetId: scope.assetId,
        assetVersion: scope.assetVersion,
        featureVersion: scope.featureVersion,
      });
      return sendJson(response, 200, { feature: feature ?? null });
    }
    return sendJson(response, 200, { items: await repository.listAssetFeatures(scope) });
  }
  if (method === "PUT") {
    const body = await readBody(request);
    return sendJson(response, 200, await repository.upsertAssetFeature({
      organizationId: requiredString(body, "organizationId"),
      projectId: requiredString(body, "projectId"),
      assetId: requiredString(body, "assetId"),
      assetVersion: requiredPositiveInteger(body, "assetVersion"),
      schemaVersion: requiredString(body, "schemaVersion") as "asset_feature_v1",
      featureVersion: requiredString(body, "featureVersion"),
      hookStrength: requiredNumber(body, "hookStrength"),
      productVisibility: requiredNumber(body, "productVisibility"),
      sceneTags: requiredStringArray(body, "sceneTags"),
      productTags: requiredStringArray(body, "productTags"),
      personTags: requiredStringArray(body, "personTags"),
      actionTags: requiredStringArray(body, "actionTags"),
      emotionTags: requiredStringArray(body, "emotionTags"),
      sellingPoints: requiredStringArray(body, "sellingPoints"),
      ctaPresence: requiredBoolean(body, "ctaPresence"),
      similarityGroup: optionalString(body, "similarityGroup"),
      similarityRisk: requiredString(body, "similarityRisk") as "low" | "medium" | "high",
      evidence: requiredStringArray(body, "evidence"),
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

function rawString(body: Record<string, unknown>, field: string): string | undefined {
  return typeof body[field] === "string" ? body[field] : undefined;
}

function requiredShortDramaStoryContext(body: Record<string, unknown>): ShortDramaStoryContext {
  const value = body.storyContext;
  if (!isRecord(value)) invalidField("storyContext", "Must be an object");
  const reviewedSellingPoints = value.reviewedSellingPoints;
  if (!Array.isArray(reviewedSellingPoints) || reviewedSellingPoints.some((item) => typeof item !== "string")) {
    invalidField("storyContext.reviewedSellingPoints", "Must be an array of strings");
  }
  const openingLine = value.openingLine;
  if (openingLine !== undefined && typeof openingLine !== "string") {
    invalidField("storyContext.openingLine", "Must be a string");
  }
  return {
    title: typeof value.title === "string" ? value.title : "",
    synopsis: typeof value.synopsis === "string" ? value.synopsis : "",
    reviewedSellingPoints,
    ...(openingLine === undefined ? {} : { openingLine }),
  };
}

function videoMetadata(
  body: Record<string, unknown>,
  kind: ReturnType<typeof requiredArtifactKind>,
): { purpose?: VideoPurpose; prerollType?: PrerollType } {
  const purpose = optionalString(body, "purpose");
  const prerollType = optionalString(body, "prerollType");
  if (purpose !== undefined && !isVideoPurpose(purpose)) {
    invalidField("purpose", "Must be a supported video purpose");
  }
  if (prerollType !== undefined && !isPrerollType(prerollType)) {
    invalidField("prerollType", "Must be a supported preroll type");
  }
  return { purpose, prerollType };
}

function resourceScope(searchParams: URLSearchParams): ResourceScope {
  const projectId = searchParams.get("projectId") ?? undefined;
  const purpose = searchParams.get("purpose") ?? undefined;
  const prerollType = searchParams.get("prerollType") ?? undefined;
  if ((purpose !== undefined || prerollType !== undefined) && projectId === undefined) {
    invalidField("projectId", "Is required when filtering by video purpose or preroll type");
  }
  return videoMetadata({ purpose, prerollType }, "video") && {
    projectId,
    purpose: purpose as VideoPurpose | undefined,
    prerollType: prerollType as PrerollType | undefined,
  };
}

function assetFeatureScope(searchParams: URLSearchParams) {
  const organizationId = searchParams.get("organizationId");
  const projectId = searchParams.get("projectId");
  if (!organizationId) invalidField("organizationId", "Required non-empty string");
  if (!projectId) invalidField("projectId", "Required non-empty string");
  return {
    organizationId,
    projectId,
    assetId: searchParams.get("assetId") ?? undefined,
    assetVersion: optionalSearchPositiveInteger(searchParams, "assetVersion"),
    featureVersion: searchParams.get("featureVersion") ?? undefined,
  };
}

function requiredArtifactKind(body: Record<string, unknown>, field: string) {
  if (!isArtifactKind(body[field])) invalidField(field, "Must be brief, image, video, or document");
  return body[field];
}

function requiredOperationalRecordKind(body: Record<string, unknown>, field: string) {
  if (!isOperationalRecordKind(body[field])) invalidField(field, "Must be a supported operational record kind");
  return body[field];
}

function requiredFields(body: Record<string, unknown>): Record<string, string | number> {
  const value = body.fields;
  if (!isRecord(value) || Object.values(value).some((item) => typeof item !== "string" && typeof item !== "number")) {
    invalidField("fields", "Must be an object of strings or numbers");
  }
  return value as Record<string, string | number>;
}

function optionalStringArray(body: Record<string, unknown>, field: string): string[] | undefined {
  const value = body[field];
  if (value === undefined) return undefined;
  if (!Array.isArray(value) || value.some((item) => typeof item !== "string" || !item.trim())) {
    invalidField(field, "Must be an array of non-empty strings");
  }
  return value;
}

function requiredStringArray(body: Record<string, unknown>, field: string): string[] {
  const value = optionalStringArray(body, field);
  if (value === undefined) invalidField(field, "Required array of non-empty strings");
  return value;
}

function requiredNumber(body: Record<string, unknown>, field: string): number {
  const value = body[field];
  if (typeof value !== "number" || !Number.isFinite(value)) {
    invalidField(field, "Must be a finite number");
  }
  return value;
}

function requiredBoolean(body: Record<string, unknown>, field: string): boolean {
  const value = body[field];
  if (typeof value !== "boolean") invalidField(field, "Must be a boolean");
  return value;
}

function requiredPositiveInteger(body: Record<string, unknown>, field: string): number {
  const value = body[field];
  if (!Number.isInteger(value) || typeof value !== "number" || value < 1) {
    invalidField(field, "Must be a positive integer");
  }
  return value;
}

function optionalSearchPositiveInteger(searchParams: URLSearchParams, field: string): number | undefined {
  const value = searchParams.get(field);
  if (value === null) return undefined;
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 1) invalidField(field, "Must be a positive integer");
  return parsed;
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

function setCorsHeaders(request: IncomingMessage, response: ServerResponse): void {
  const origin = request.headers.origin;
  if (origin && /^http:\/\/127\.0\.0\.1:\d+$/.test(origin)) {
    response.setHeader("Access-Control-Allow-Origin", origin);
    response.setHeader("Vary", "Origin");
  }
  response.setHeader("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,OPTIONS");
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
  const dataFile = process.env.DATA_FILE ?? resolve(process.cwd(), "data/mvp-store.json");
  if (process.env.RESET_DATA_FILE === "true") {
    await rm(dataFile, { force: true });
    await rm(`${dataFile}.tmp`, { force: true });
  }
  const repository = process.env.SKIP_DEMO_SEED === "true"
    ? await FileRepository.open(dataFile)
    : await openSeededRepository(dataFile);
  createApp({ repository }).listen(port, "127.0.0.1", () => {
    console.log(`MVP API listening on http://127.0.0.1:${port}`);
  });
}
