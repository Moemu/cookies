import assert from "node:assert/strict";
import { once } from "node:events";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { createApp } from "./index.js";
import { FileRepository } from "./repository.js";

async function startApi() {
  const directory = await mkdtemp(join(tmpdir(), "cookies-public-insights-"));
  const repository = await FileRepository.open(join(directory, "store.json"));
  const server = createApp({ repository });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(address && typeof address !== "string");
  return {
    request: async (path: string) => {
      const response = await fetch(`http://127.0.0.1:${address.port}${path}`);
      return { status: response.status, body: await response.json() as any };
    },
    dispose: async () => {
      await new Promise<void>((resolve, reject) => server.close((error) => (error ? reject(error) : resolve())));
      await rm(directory, { recursive: true, force: true });
    },
  };
}

test("public insight sample data is imported from data directory", async () => {
  const api = await startApi();
  try {
    const overview = await api.request("/api/public-insights/overview");
    assert.equal(overview.status, 200);
    assert.equal(overview.body.total_videos, 6);
    assert.equal(overview.body.files[0].filename, "sample_public_insights.csv");

    const filters = await api.request("/api/public-insights/filters");
    assert.equal(filters.status, 200);
    assert.equal(filters.body.industries.some((item: { value: string }) => item.value === "美妆护肤"), true);

    const videos = await api.request("/api/public-insights/videos?page=1&page_size=2&industry=美妆护肤");
    assert.equal(videos.status, 200);
    assert.equal(videos.body.total, 1);
    assert.equal(videos.body.items[0].item_id, "insight-006");
    assert.equal("oral_script" in videos.body.items[0], false);

    const detail = await api.request("/api/public-insights/videos/insight-006");
    assert.equal(detail.status, 200);
    assert.equal(detail.body.item_id, "insight-006");
    assert.equal(detail.body.oral_script.includes("早八通勤"), true);
  } finally {
    await api.dispose();
  }
});
