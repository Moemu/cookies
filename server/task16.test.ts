import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { FileRepository } from "./repository.js";

test("ChangeSet 仅接受当前 Project 的主创意并在预检时排除前贴", async () => {
  const directory = await mkdtemp(join(tmpdir(), "cookies-task16-"));
  try {
    const repository = await FileRepository.open(join(directory, "mvp-store.json"));
    const project = await repository.createProject({
      name: "主创意项目",
      brand: "Cookies",
      objective: "验证 ChangeSet 创意边界",
    });
    const otherProject = await repository.createProject({
      name: "其他项目",
      brand: "Cookies",
      objective: "验证跨项目边界",
    });
    const brief = await repository.createArtifact({
      projectId: project.id,
      kind: "brief",
      status: "ready",
      content: "已确认 Brief",
    });
    const mainCreative = await repository.createArtifact({
      projectId: project.id,
      kind: "image",
      status: "ready",
      content: "当前项目主创意",
    });
    const preroll = await repository.createArtifact({
      projectId: project.id,
      kind: "video",
      purpose: "preroll",
      prerollType: "short_drama",
      status: "ready",
      content: "前贴视频",
    });
    const foreignCreative = await repository.createArtifact({
      projectId: otherProject.id,
      kind: "image",
      status: "ready",
      content: "其他项目主创意",
    });

    await assert.rejects(
      repository.createChangeSet({
        projectId: project.id,
        name: "不允许前贴",
        artifactIds: [brief.id, preroll.id],
        budgetLimit: 100,
      }),
      /Preroll assets cannot be used/,
    );
    await assert.rejects(
      repository.createChangeSet({
        projectId: project.id,
        name: "不允许跨项目",
        artifactIds: [brief.id, foreignCreative.id],
        budgetLimit: 100,
      }),
      /must belong to the current project/,
    );

    const changeSet = await repository.createChangeSet({
      projectId: project.id,
      name: "只使用主创意",
      artifactIds: [brief.id, mainCreative.id],
      budgetLimit: 100,
    });
    const preflight = await repository.preflightChangeSet(changeSet.id);
    assert.equal(preflight.status, "preflight_passed");
    assert.equal(preflight.preflight?.checks.find((check) => check.code === "ready_creative")?.passed, true);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});
