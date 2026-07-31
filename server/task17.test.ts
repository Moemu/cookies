import assert from "node:assert/strict";
import { readdir, readFile, mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { FileRepository } from "./repository.js";

test("文件仓储会串行化并发变更，并留下可恢复的完整存储文件", async () => {
  const directory = await mkdtemp(join(tmpdir(), "cookies-task17-"));
  const filePath = join(directory, "mvp-store.json");
  try {
    const repository = await FileRepository.open(filePath);
    const projects = await Promise.all(Array.from({ length: 24 }, (_, index) => repository.createProject({
      name: `并发项目 ${index}`,
      brand: "Cookies",
      objective: "验证写入不会丢失",
    })));

    await Promise.all(projects.map((project, index) => repository.updateProject(project.id, {
      objective: `更新后的目标 ${index}`,
    })));

    const persisted = JSON.parse(await readFile(filePath, "utf8")) as { projects: Array<{ id: string }> };
    const reopened = await FileRepository.open(filePath);
    const restored = await reopened.listProjects();
    const files = await readdir(directory);

    assert.equal(persisted.projects.length, projects.length);
    assert.deepEqual(new Set(restored.map((project) => project.id)), new Set(projects.map((project) => project.id)));
    assert.deepEqual(
      restored.map((project) => project.objective).sort(),
      projects.map((_, index) => `更新后的目标 ${index}`).sort(),
    );
    assert.deepEqual(files.filter((file) => file.includes(".tmp")), []);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});
