import assert from "node:assert/strict";
import { once } from "node:events";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import type { ArkProvider } from "./ark-provider.js";
import { createGenerationService } from "./generation-service.js";
import { createApp } from "./index.js";
import { FileRepository } from "./repository.js";

test("前贴视频 API 按项目和用途隔离，并在重启后恢复标签化任务与产物", async () => {
  const directory = await mkdtemp(join(tmpdir(), "cookies-preroll-"));
  let server: ReturnType<typeof createApp> | undefined;
  try {
    const filePath = join(directory, "mvp-store.json");
    const repository = await FileRepository.open(filePath);
    const firstProject = await repository.createProject({
      name: "短剧前贴项目",
      brand: "Cookies",
      objective: "验证短剧前贴隔离",
    });
    const secondProject = await repository.createProject({
      name: "游戏前贴项目",
      brand: "Cookies",
      objective: "验证游戏前贴隔离",
    });
    const firstBrief = await repository.createArtifact({
      projectId: firstProject.id,
      kind: "brief",
      content: "已确认短剧 Brief",
      status: "ready",
    });
    const secondBrief = await repository.createArtifact({
      projectId: secondProject.id,
      kind: "brief",
      content: "已确认游戏 Brief",
      status: "ready",
    });

    const provider = successfulVideoProvider();
    server = createApp({
      repository,
      generationService: createGenerationService(repository, provider),
    });
    const baseUrl = await listen(server);
    const request = async (path: string, body?: Record<string, unknown>) => {
      const response = await fetch(`${baseUrl}${path}`, body ? {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      } : undefined);
      return { status: response.status, body: await response.json() as any };
    };
    const storyContext = {
      title: "雨夜归来的继承人",
      synopsis: "被逐出家门的女主在雨夜带着证据归来，发现继母正在转移家产，并必须在家族晚宴前揭开真相。",
      reviewedSellingPoints: ["豪门继承权争夺"],
      openingLine: "你以为我今晚回来，是为了求你吗？",
    };
    const shortDramaPlan = await request("/api/short-drama-preroll-plans", {
      projectId: firstProject.id,
      briefId: firstBrief.id,
      storyContext,
    });
    assert.equal(shortDramaPlan.status, 200);
    const crossProjectPlan = await request("/api/short-drama-preroll-plans", {
      projectId: firstProject.id,
      briefId: secondBrief.id,
      storyContext,
    });
    assert.equal(crossProjectPlan.status, 400);
    assert.equal(crossProjectPlan.body.error.code, "BRIEF_NOT_CONFIRMED");
    const rejectedSelection = await request("/api/generation/media", {
      projectId: firstProject.id,
      kind: "video",
      purpose: "preroll",
      prerollType: "short_drama",
      briefId: firstBrief.id,
      shortDramaPlanVersion: shortDramaPlan.body.version,
      shortDramaCandidateId: "forged-candidate",
      storyContext,
    });
    assert.equal(rejectedSelection.status, 400);
    assert.equal((await request(`/api/generation-jobs?projectId=${firstProject.id}`)).body.length, 0);

    const firstJob = await request("/api/generation/media", {
      projectId: firstProject.id,
      kind: "video",
      purpose: "preroll",
      prerollType: "short_drama",
      prompt: "客户端伪造的短剧 Prompt",
      briefId: firstBrief.id,
      shortDramaPlanVersion: shortDramaPlan.body.version,
      shortDramaCandidateId: shortDramaPlan.body.candidates[0].id,
      storyContext,
    });
    const secondJob = await request("/api/generation/media", {
      projectId: secondProject.id,
      kind: "video",
      purpose: "preroll",
      prerollType: "game",
      prompt: "游戏前贴",
      briefId: secondBrief.id,
    });
    const commerceJob = await request("/api/generation/media", {
      projectId: secondProject.id,
      kind: "video",
      purpose: "preroll",
      prerollType: "commerce",
      prompt: "电商前贴",
      briefId: secondBrief.id,
    });
    assert.equal(firstJob.status, 202);
    assert.equal(secondJob.status, 202);
    assert.equal(commerceJob.status, 202);
    assert.equal(provider.prompts[0]?.includes("客户端伪造的短剧 Prompt"), false);
    assert.equal(provider.prompts[1], "游戏前贴");
    assert.equal(provider.prompts[2], "电商前贴");

    const missingType = await request("/api/generation/media", {
      projectId: firstProject.id,
      kind: "video",
      purpose: "preroll",
      prompt: "缺少类型的前贴",
      briefId: firstBrief.id,
    });
    assert.equal(missingType.status, 400);
    assert.equal(missingType.body.error.code, "VALIDATION_ERROR");

    const firstJobs = await request(
      `/api/generation-jobs?projectId=${firstProject.id}&purpose=preroll&prerollType=short_drama`,
    );
    assert.equal(firstJobs.status, 200);
    assert.deepEqual(firstJobs.body.map((job: { id: string }) => job.id), [firstJob.body.id]);

    const crossProjectPoll = await request(
      `/api/generation-jobs/${secondJob.body.id}?projectId=${firstProject.id}&purpose=preroll&prerollType=short_drama`,
    );
    assert.equal(crossProjectPoll.status, 404);

    const completed = await request(
      `/api/generation-jobs/${firstJob.body.id}?projectId=${firstProject.id}&purpose=preroll&prerollType=short_drama`,
    );
    assert.equal(completed.status, 200);
    assert.equal(completed.body.status, "succeeded");

    const firstArtifacts = await request(
      `/api/artifacts?projectId=${firstProject.id}&purpose=preroll&prerollType=short_drama`,
    );
    assert.equal(firstArtifacts.status, 200);
    assert.equal(firstArtifacts.body.length, 1);
    assert.equal(firstArtifacts.body[0].sourceJobId, firstJob.body.id);
    assert.equal(firstArtifacts.body[0].purpose, "preroll");
    assert.equal(firstArtifacts.body[0].prerollType, "short_drama");
    assert.equal(firstArtifacts.body[0].shortDramaPreroll.prompt.includes("客户端伪造的短剧 Prompt"), false);
    assert.equal(firstArtifacts.body[0].shortDramaPreroll.prompt.includes(storyContext.openingLine), false);
    const serializedSnapshot = JSON.stringify(firstArtifacts.body[0].shortDramaPreroll);
    assert.equal(serializedSnapshot.includes(storyContext.openingLine), false);
    assert.equal(serializedSnapshot.includes("https://assets.test/preroll.mp4"), false);
    assert.equal(serializedSnapshot.includes("unit-test-credential"), false);

    const reopened = await FileRepository.open(filePath);
    const restoredJobs = await reopened.listGenerationJobs({
      projectId: firstProject.id,
      purpose: "preroll",
      prerollType: "short_drama",
    });
    const restoredArtifacts = await reopened.listArtifacts({
      projectId: firstProject.id,
      purpose: "preroll",
      prerollType: "short_drama",
    });
    assert.deepEqual(restoredJobs.map((job) => job.id), [firstJob.body.id]);
    assert.equal(restoredJobs[0]?.status, "succeeded");
    assert.equal(restoredArtifacts[0]?.id, completed.body.artifactId);
    assert.equal(restoredArtifacts[0]?.shortDramaPreroll?.planVersion, shortDramaPlan.body.version);
    const restoredGameJobs = await reopened.listGenerationJobs({
      projectId: secondProject.id,
      purpose: "preroll",
      prerollType: "game",
    });
    const restoredCommerceJobs = await reopened.listGenerationJobs({
      projectId: secondProject.id,
      purpose: "preroll",
      prerollType: "commerce",
    });
    assert.deepEqual(restoredGameJobs.map((job) => job.id), [secondJob.body.id]);
    assert.deepEqual(restoredCommerceJobs.map((job) => job.id), [commerceJob.body.id]);
  } finally {
    if (server) await close(server);
    await rm(directory, { recursive: true, force: true });
  }
});

function successfulVideoProvider(): ArkProvider & { prompts: string[] } {
  const prompts: string[] = [];
  return {
    config: {
      configured: true,
      apiKey: "unit-test-credential",
      baseUrl: "https://provider.test/v3",
      models: {
        text: "doubao-seed-2-1-pro-260628",
        image: "doubao-seedream-5-0-pro-260628",
        video: "doubao-seedance-2-0-fast-260128",
        embedding: "doubao-embedding-vision-251215",
      },
    },
    ensureConfigured: () => undefined,
    generateText: async () => "Brief",
    prompts,
    createMedia: async (_kind, prompt) => {
      prompts.push(prompt);
      return { providerTaskId: "preroll-provider-task" };
    },
    getMediaTask: async () => ({ status: "succeeded", assetUrl: "https://assets.test/preroll.mp4" }),
    cancelMedia: async () => undefined,
  };
}

async function listen(server: ReturnType<typeof createApp>): Promise<string> {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("Expected a TCP server address");
  return `http://127.0.0.1:${address.port}`;
}

async function close(server: ReturnType<typeof createApp>): Promise<void> {
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
}
