import type { Artifact, ChangeSet, Project } from "./domain.js";
import { FileRepository } from "./repository.js";

export const DEMO_PROJECT_NAME = "投资人路演：精度证据增长";
const DEMO_PROJECT_IDENTITY = {
  name: DEMO_PROJECT_NAME,
  brand: "白域精工",
  objective: "向采购与研发负责人展示精度证据，获取高质量销售线索。",
};
const DEMO_ACTOR = "demo-seeder";

export async function seedDemoProject(repository: FileRepository): Promise<Project> {
  const project = (await repository.listProjects()).find(isDemoProject)
    ?? await repository.createProject({ ...DEMO_PROJECT_IDENTITY, actor: DEMO_ACTOR });
  const artifacts = await repository.listArtifacts(project.id);
  const brief = artifacts.find(isReadyBrief)
    ?? await repository.createArtifact({
      projectId: project.id,
      kind: "brief",
      status: "ready",
      content: "已确认 Brief：以 ±0.01mm 精度、98%+ 准时交付和真实制造场景为核心证据，面向采购与研发负责人获取销售线索。",
      actor: DEMO_ACTOR,
    });
  const creative = artifacts.find(isReadyCreative)
    ?? await repository.createArtifact({
      projectId: project.id,
      kind: "image",
      status: "ready",
      content: "预置 AI 图文创意：高精度 CNC 加工画面，展示精度与交期证据；仅作路演资产，不代表真实广告素材。",
      actor: DEMO_ACTOR,
    });
  const completeArtifacts = [...artifacts, brief, creative];
  const existingChangeSet = (await repository.listChangeSets(project.id))
    .find((changeSet) => isApprovableChangeSet(changeSet, completeArtifacts));
  if (!existingChangeSet) {
    const changeSet = await repository.createChangeSet({
      projectId: project.id,
      name: "精度证据创意与探索预算模拟",
      artifactIds: [brief.id, creative.id],
      budgetLimit: 8600,
      actor: DEMO_ACTOR,
    });
    await repository.preflightChangeSet(changeSet.id, DEMO_ACTOR);
  }
  if ((await repository.listAuditEvents(project.id)).length === 0) {
    await repository.recordAuditEvent(project.id, "demo.seed_verified", "project", project.id, DEMO_ACTOR, {
      source: "startup",
    });
  }
  return project;
}

function isDemoProject(project: Project): boolean {
  return project.name === DEMO_PROJECT_IDENTITY.name
    && project.brand === DEMO_PROJECT_IDENTITY.brand
    && project.objective === DEMO_PROJECT_IDENTITY.objective;
}

function isReadyBrief(artifact: Artifact): boolean {
  return artifact.kind === "brief" && artifact.status === "ready";
}

function isReadyCreative(artifact: Artifact): boolean {
  return (artifact.kind === "image" || artifact.kind === "video") && artifact.status === "ready";
}

function isApprovableChangeSet(changeSet: ChangeSet, artifacts: Artifact[]): boolean {
  if (changeSet.status !== "preflight_passed" || !changeSet.preflight?.passed || !changeSet.budgetLimit || changeSet.budgetLimit <= 0) {
    return false;
  }
  const linkedArtifacts = artifacts.filter((artifact) => changeSet.artifactIds.includes(artifact.id));
  return linkedArtifacts.some(isReadyBrief) && linkedArtifacts.some(isReadyCreative);
}
