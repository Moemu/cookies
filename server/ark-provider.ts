import { DomainError } from "./errors.js";

export const ARK_MODELS = Object.freeze({
  text: "doubao-seed-2-1-pro-260628",
  image: "doubao-seedream-5-0-pro-260628",
  video: "doubao-seedance-2-0-fast-260128",
  embedding: "doubao-embedding-vision-251215",
});

const PROVIDER_TIMEOUT_MS = 15_000;

export type MediaGenerationKind = "image" | "video";
export type ProviderMediaStatus = "queued" | "running" | "succeeded" | "failed" | "cancelled" | "unknown";

export interface ProviderMediaTask {
  status: ProviderMediaStatus;
  assetUrl?: string;
  diagnostic?: string;
}

export interface ArkConfig {
  readonly apiKey: string;
  readonly baseUrl: string;
  readonly configured: boolean;
  readonly models: typeof ARK_MODELS;
  readonly source?: "environment" | "workspace";
  readonly maskedApiKey?: string;
  readonly updatedAt?: string;
}

export interface ArkProvider {
  readonly config: ArkConfig;
  ensureConfigured(): void;
  generateText(prompt: string): Promise<string>;
  createMedia(kind: MediaGenerationKind, prompt: string): Promise<{ providerTaskId?: string; assetUrl?: string }>;
  getMediaTask(providerTaskId: string): Promise<ProviderMediaTask>;
  cancelMedia(providerTaskId: string): Promise<void>;
}

export function loadArkConfig(environment: NodeJS.ProcessEnv = process.env): ArkConfig {
  const apiKey = environment.ARK_API_KEY?.trim() ?? "";
  const baseUrl = (environment.ARK_BASE_URL?.trim() || "https://ark.cn-beijing.volces.com/api/v3").replace(/\/+$/, "");
  let url: URL;

  try {
    url = new URL(baseUrl);
  } catch {
    throw new Error("ARK_BASE_URL must be a valid HTTPS URL");
  }
  if (url.protocol !== "https:") throw new Error("ARK_BASE_URL must use HTTPS");

  return { apiKey, baseUrl, configured: apiKey.length > 0, models: ARK_MODELS };
}

export function publicCapabilities(config: ArkConfig): Record<string, unknown> {
  return {
    provider: "ark",
    status: config.configured ? "configured" : "not_configured",
    capabilities: Object.entries(config.models).map(([capability, model]) => ({
      capability,
      model,
      available: config.configured,
    })),
    credential: config.configured
      ? {
        source: config.source ?? "environment",
        maskedApiKey: config.maskedApiKey ?? maskSecret(config.apiKey),
        updatedAt: config.updatedAt,
      }
      : undefined,
    checkedAt: new Date().toISOString(),
  };
}

export function createArkProvider(
  config: ArkConfig,
  fetchImpl: typeof fetch = fetch,
): ArkProvider {
  async function request(
    path: string,
    body?: Record<string, unknown>,
    method = "POST",
  ): Promise<Record<string, unknown>> {
    ensureConfigured();
    let response: Response;
    try {
      response = await fetchImpl(`${config.baseUrl}${path}`, {
        method,
        signal: AbortSignal.timeout(PROVIDER_TIMEOUT_MS),
        headers: {
          Authorization: `Bearer ${config.apiKey}`,
          "Content-Type": "application/json",
        },
        body: body === undefined ? undefined : JSON.stringify(body),
      });
    } catch {
      throw new DomainError("PROVIDER_UNAVAILABLE", "Model provider is temporarily unavailable");
    }
    if (!response.ok) {
      throw new DomainError("PROVIDER_REQUEST_FAILED", "Model provider could not complete the request");
    }
    try {
      const payload: unknown = await response.json();
      return isRecord(payload) ? payload : {};
    } catch {
      throw new DomainError("PROVIDER_INVALID_RESPONSE", "Model provider returned an invalid response");
    }
  }

  function ensureConfigured(): void {
    if (!config.configured) {
      throw new DomainError("PROVIDER_NOT_CONFIGURED", "Model provider is not configured on this server");
    }
  }

  return {
    config,
    ensureConfigured,
    async generateText(prompt) {
      const payload = await request("/chat/completions", {
        model: config.models.text,
        messages: [{ role: "user", content: prompt }],
      });
      const choices = payload.choices;
      const first = Array.isArray(choices) ? choices[0] : undefined;
      const message = isRecord(first) && isRecord(first.message) ? first.message : undefined;
      if (typeof message?.content !== "string" || !message.content.trim()) {
        throw new DomainError("PROVIDER_INVALID_RESPONSE", "Model provider returned an invalid text result");
      }
      return message.content.trim();
    },
    async createMedia(kind, prompt) {
      const path = kind === "image" ? "/images/generations" : "/contents/generations/tasks";
      const payload = await request(path, { model: config.models[kind], prompt });
      const data = Array.isArray(payload.data) && isRecord(payload.data[0]) ? payload.data[0] : undefined;
      const providerTaskId = stringValue(payload.id) ?? stringValue(payload.task_id) ?? stringValue(data?.id);
      const assetUrl = stringValue(payload.url) ?? stringValue(data?.url);
      if (!providerTaskId && !assetUrl) {
        throw new DomainError("PROVIDER_INVALID_RESPONSE", "Model provider returned no media task or asset");
      }
      return { providerTaskId, assetUrl };
    },
    async getMediaTask(providerTaskId) {
      const payload = await request(
        `/contents/generations/tasks/${encodeURIComponent(providerTaskId)}`,
        undefined,
        "GET",
      );
      const data = isRecord(payload.data) ? payload.data : Array.isArray(payload.data) && isRecord(payload.data[0]) ? payload.data[0] : {};
      const rawStatus = stringValue(payload.status) ?? stringValue(payload.state) ?? stringValue(data.status) ?? stringValue(data.state);
      const status = normalizeMediaStatus(rawStatus);
      const assetUrl = stringValue(payload.url) ?? stringValue(data.url) ?? stringValue(data.output_url);
      const payloadError = isRecord(payload.error) ? payload.error : {};
      const dataError = isRecord(data.error) ? data.error : {};
      const diagnostic = stringValue(payloadError.message) ?? stringValue(dataError.message);
      return { status, assetUrl, diagnostic };
    },
    async cancelMedia(providerTaskId) {
      await request(`/contents/generations/tasks/${encodeURIComponent(providerTaskId)}/cancel`, {});
    },
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringValue(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value : undefined;
}

function normalizeMediaStatus(value: string | undefined): ProviderMediaStatus {
  switch (value?.toLowerCase()) {
    case "queued":
    case "pending":
    case "created":
      return "queued";
    case "running":
    case "processing":
    case "in_progress":
      return "running";
    case "succeeded":
    case "success":
    case "completed":
      return "succeeded";
    case "failed":
    case "error":
      return "failed";
    case "cancelled":
    case "canceled":
      return "cancelled";
    default:
      return "unknown";
  }
}

export function maskSecret(value: string): string {
  const secret = value.trim();
  if (secret.length <= 8) return "••••••••";
  return `••••••••${secret.slice(-4)}`;
}
