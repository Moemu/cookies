import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { once } from "node:events";
import { createServer, type Server } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import {
  ARK_MODELS,
  createArkProvider,
  loadArkConfig,
  publicCapabilities,
  type ArkProvider,
} from "./ark-provider.js";
import { DomainError, isDomainError } from "./errors.js";
import { createGenerationService } from "./generation-service.js";
import { createApp } from "./index.js";
import { FileRepository } from "./repository.js";

const testConfig = loadArkConfig({
  ARK_API_KEY: "unit-test-credential",
  ARK_BASE_URL: "https://provider.test/v3",
});

test("安全配置仅公开能力和固定模型映射", () => {
  const unconfigured = loadArkConfig({});
  const payload = JSON.stringify(publicCapabilities(testConfig));

  assert.equal(unconfigured.configured, false);
  assert.equal(testConfig.models.text, "doubao-seed-2-1-pro-260628");
  assert.equal(testConfig.models.image, "doubao-seedream-5-0-pro-260628");
  assert.equal(testConfig.models.video, "doubao-seedance-2-0-fast-260128");
  assert.equal(testConfig.models.embedding, "doubao-embedding-vision-251215");
  assert.equal(payload.includes("apiKey"), false);
  assert.equal(payload.includes("unit-test-credential"), false);
  assert.throws(() => loadArkConfig({ ARK_BASE_URL: "http://provider.test" }), /HTTPS/);
});

test("Provider 使用指定模型且将上游失败标准化", async () => {
  const requests: Array<{ path: string; body: Record<string, unknown>; authorization: string | null }> = [];
  const provider = createArkProvider(testConfig, async (input, init) => {
    requests.push({
      path: String(input),
      body: JSON.parse(String(init?.body)) as Record<string, unknown>,
      authorization: new Headers(init?.headers).get("Authorization"),
    });
    return new Response(JSON.stringify({ choices: [{ message: { content: "策略结果" } }] }), { status: 200 });
  });

  assert.equal(await provider.generateText("生成策略"), "策略结果");
  assert.equal(requests[0]?.body.model, ARK_MODELS.text);
  assert.equal(requests[0]?.authorization, "Bearer unit-test-credential");

  const failed = createArkProvider(testConfig, async () => new Response("unavailable", { status: 503 }));
  await assert.rejects(
    () => failed.createMedia("video", "生成视频"),
    (error: unknown) => isDomainError(error, "PROVIDER_REQUEST_FAILED"),
  );
});

test("文本和媒体任务复用持久化状态机且不泄露上游任务标识", async () => {
  const directory = await mkdtemp(join(tmpdir(), "cookies-provider-"));
  const provider = fakeProvider();
  try {
    const repository = await FileRepository.open(join(directory, "mvp-store.json"));
    const project = await repository.createProject({ name: "Demo", brand: "Cookies", objective: "Grow" });
    const service = createGenerationService(repository, provider);

    const brief = await service.generateBrief({ projectId: project.id, prompt: "生成策略" });
    const media = await service.createMedia({ projectId: project.id, kind: "video", prompt: "生成视频" });
    const stored = await repository.getGenerationJob(media.id);
    const cancelled = await service.cancelMedia(media.id);

    assert.equal(brief.job.model, ARK_MODELS.text);
    assert.equal(brief.job.status, "succeeded");
    assert.equal(brief.artifact.sourceJobId, brief.job.id);
    assert.equal(media.model, ARK_MODELS.video);
    assert.equal(media.status, "running");
    assert.equal("providerTaskId" in media, false);
    assert.equal(stored?.providerTaskId, "provider-task-for-test");
    assert.equal(cancelled.status, "cancelled");
    assert.deepEqual(provider.cancelledTaskIds, ["provider-task-for-test"]);
    assert.equal((await repository.listAuditEvents(project.id)).some((event) => event.action === "generation_job.status_changed"), true);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});

test("HTTP 契约对未配置和 Provider 失败返回安全错误，不回显凭据", async () => {
  const directory = await mkdtemp(join(tmpdir(), "cookies-provider-http-"));
  let server: Server | undefined;
  try {
    const repository = await FileRepository.open(join(directory, "mvp-store.json"));
    const project = await repository.createProject({ name: "Demo", brand: "Cookies", objective: "Grow" });
    const unavailableProvider: ArkProvider = {
      ...fakeProvider(),
      ensureConfigured: () => {
        throw new DomainError("PROVIDER_NOT_CONFIGURED", "Model provider is not configured on this server");
      },
    };
    server = createApp({ repository, generationService: createGenerationService(repository, unavailableProvider) });
    const url = await listen(server);
    const response = await fetch(`${url}/api/generation/text`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ projectId: project.id, prompt: "生成策略" }),
    });
    const body = await response.json() as { error: { code: string; message: string } };
    const serialized = JSON.stringify(body);

    assert.equal(response.status, 503);
    assert.equal(body.error.code, "PROVIDER_NOT_CONFIGURED");
    assert.equal(serialized.includes("unit-test-credential"), false);
    assert.equal(serialized.includes("Authorization"), false);
  } finally {
    if (server) await close(server);
    await rm(directory, { recursive: true, force: true });
  }
});

function fakeProvider(): ArkProvider & { cancelledTaskIds: string[] } {
  const cancelledTaskIds: string[] = [];
  return {
    config: testConfig,
    cancelledTaskIds,
    ensureConfigured: () => undefined,
    generateText: async () => "生成的策略 Brief",
    createMedia: async () => ({ providerTaskId: "provider-task-for-test" }),
    cancelMedia: async (providerTaskId) => {
      cancelledTaskIds.push(providerTaskId);
    },
  };
}

async function listen(server: Server): Promise<string> {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("Expected a TCP server address");
  return `http://127.0.0.1:${address.port}`;
}

async function close(server: Server): Promise<void> {
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
}
