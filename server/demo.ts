import type { Artifact, BusinessTask, BusinessTaskType, ChangeSet, OperationalRecordKind, Project } from "./domain.js";
import { FileRepository, type UpsertOperationalRecordInput } from "./repository.js";

export const DEMO_PROJECT_NAME = "投资人路演：精度证据增长";
// Task26 audit: this is the only canonical investor demo Project seed.
// It is identified by name + brand + objective so reruns reuse the same Project
// instead of leaking core demo records into user-created Projects.
const DEMO_PROJECT_IDENTITY = {
  name: DEMO_PROJECT_NAME,
  brand: "白域精工",
  objective: "向采购与研发负责人展示精度证据，获取高质量销售线索。",
  runtime: {
    code: "SP",
    product: "高精度 CNC 加工零部件",
    stage: "投放审批",
    progress: 82,
    status: "active" as const,
    owner: "Noah Xu",
    budget: 1_000_000,
    currency: "CNY" as const,
    timezone: "Asia/Shanghai" as const,
  },
};
const DEMO_ACTOR = "demo-seeder";
const DEMO_BRIEF_CONTENT = "已确认 Brief：以 ±0.01mm 精度、98%+ 准时交付和真实制造场景为核心证据，面向采购与研发负责人获取销售线索。";
const DEMO_CREATIVE_CONTENT = "预置 AI 图文创意：高精度 CNC 加工画面，展示精度与交期证据；仅作路演资产，不代表真实广告素材。";
const DEMO_VIDEO_CONTENT = "预置 AI 视频创意：15 秒精密制造品牌片，按问题、证据、交付承诺和 CTA 串联完整路演故事线。";
const DEMO_STRATEGY_CONTENT = "[strategy] 精度证据增长策略：以研发负责人和采购负责人为核心受众，先用公差、良率和交期证据建立信任，再用真实制造场景引导销售线索。";
const DEMO_INSIGHT_CONTENT = "[insight] 素材洞察报告：证据前置版本在 48 个素材样本中表现稳定，研发负责人素材 CTR 与线索质量均优于纯产品特写对照组。";
const DEMO_DELIVERY_CONTENT = "[delivery] 投放模拟计划：保留人工审批、预算边界、重复广告清理和回滚入口；默认只执行本地模拟，不连接真实广告平台。";
const DEMO_CHANGE_SET_NAME = "精度证据创意与探索预算模拟";
const DEMO_TASKS: Array<{
  type: BusinessTaskType;
  name: string;
  objective: string;
  status: BusinessTask["status"];
}> = [
  {
    type: "strategy",
    name: "精度证据增长策略评审",
    objective: "整合行业访谈、历史素材表现和公开洞察样本，形成可向投资人复演的策略 Brief 与证据链。",
    status: "ready",
  },
  {
    type: "creative",
    name: "精密制造主创意组合",
    objective: "基于已确认策略制作图文与视频主创意，突出 ±0.01mm 精度、交付稳定性和真实制造画面。",
    status: "in_progress",
  },
  {
    type: "short_drama_preroll",
    name: "短剧前贴：交期冲突钩子",
    objective: "用交付延误冲突切入，生成可人工选择的短剧前贴候选并保留候选证据。",
    status: "ready",
  },
  {
    type: "viral_remake",
    name: "高表现结构复刻验证",
    objective: "拆解公开样本的钩子、证明与 CTA 结构，生成白域精工可用的原创复刻草案。",
    status: "draft",
  },
  {
    type: "video_edit",
    name: "品牌片包装与交付版本",
    objective: "把主视频、字幕、CTA 和审计说明整理成可进入投放模拟审批的交付版本。",
    status: "in_progress",
  },
];
const DEMO_OPERATIONAL_RECORD_IDS = [
  "WORK-2607-01",
  "WORK-2607-02",
  "WORK-2607-03",
  "WORK-2607-04",
  "WORK-2607-05",
  "EV-021",
  "EV-024",
  "INS-014",
  "EV-027",
  "ACT-2607-01",
  "ACT-2607-02",
  "ACT-2607-03",
  "ACT-2607-04",
  "METRIC-2607-01",
  "AD-2607-031",
  "AD-2607-028",
  "AD-2607-019",
  "AD-2607-014",
  "AD-2607-008",
  "MIX-2607-01",
  "MIX-2607-02",
  "MIX-2607-03",
  "MIX-2607-04",
  "METHOD-M1",
  "METHOD-M3",
  "METHOD-M4",
  "METHOD-M5",
  "DIAG-2607-01",
  "DIAG-2607-02",
  "DIAG-2607-03",
  "ACTION-P0",
  "ACTION-P1",
  "ACTION-P2",
  "REC-BRF-2607-11",
  "REC-STR-2607-08",
  "REC-CR-2607-42",
  "REC-INS-2607-14",
  "REC-PLAN-2607-06",
  "REC-CS-2607-018",
  "REC-EV-2607-24",
  "REC-VER-2607-19",
  "WORK-2607-06",
  "EV-031",
  "EV-034",
  "ACT-2607-05",
  "ACT-2607-06",
  "METRIC-2607-02",
  "AD-2607-036",
  "MIX-2607-05",
  "METHOD-M6",
  "DIAG-2607-04",
  "ACTION-P3",
  "REC-ASSET-2607-31",
  "REC-AUDIT-2607-04",
];

export async function seedDemoProject(repository: FileRepository): Promise<Project> {
  let project = (await repository.listProjects()).find(isDemoProject)
    ?? await repository.createProject({ ...DEMO_PROJECT_IDENTITY, actor: DEMO_ACTOR });
  if (!sameRuntime(project.runtime, DEMO_PROJECT_IDENTITY.runtime)) {
    project = await repository.updateProject(project.id, {
      runtime: DEMO_PROJECT_IDENTITY.runtime,
      actor: DEMO_ACTOR,
    });
  }
  await repository.deleteSeedDataOutsideProject({
    projectId: project.id,
    operationalRecordIds: DEMO_OPERATIONAL_RECORD_IDS,
    artifactContents: [
      DEMO_BRIEF_CONTENT,
      DEMO_CREATIVE_CONTENT,
      DEMO_VIDEO_CONTENT,
      DEMO_STRATEGY_CONTENT,
      DEMO_INSIGHT_CONTENT,
      DEMO_DELIVERY_CONTENT,
    ],
    changeSetNames: [DEMO_CHANGE_SET_NAME],
    auditActions: ["demo.seed_verified"],
  });
  const artifacts = await repository.listArtifacts(project.id);
  const brief = artifacts.find(isReadyBrief)
    ?? await repository.createArtifact({
      projectId: project.id,
      kind: "brief",
      status: "ready",
      content: DEMO_BRIEF_CONTENT,
      actor: DEMO_ACTOR,
    });
  const creative = artifacts.find(isReadyCreative)
    ?? await repository.createArtifact({
      projectId: project.id,
      kind: "image",
      status: "ready",
      content: DEMO_CREATIVE_CONTENT,
      actor: DEMO_ACTOR,
    });
  const video = artifacts.find((artifact) => artifact.kind === "video" && artifact.content === DEMO_VIDEO_CONTENT)
    ?? await repository.createArtifact({
      projectId: project.id,
      kind: "video",
      status: "ready",
      content: DEMO_VIDEO_CONTENT,
      actor: DEMO_ACTOR,
    });
  const strategy = await ensureDocument(repository, artifacts, project.id, DEMO_STRATEGY_CONTENT);
  const insight = await ensureDocument(repository, artifacts, project.id, DEMO_INSIGHT_CONTENT);
  const delivery = await ensureDocument(repository, artifacts, project.id, DEMO_DELIVERY_CONTENT);
  const completeArtifacts = [...artifacts, brief, creative, video, strategy, insight, delivery];
  await seedDemoTasks(repository, project.id, { brief, creative, video, strategy, insight, delivery });
  const existingChangeSet = (await repository.listChangeSets(project.id))
    .find((changeSet) => isApprovableChangeSet(changeSet, completeArtifacts));
  if (!existingChangeSet) {
    const changeSet = await repository.createChangeSet({
      projectId: project.id,
      name: DEMO_CHANGE_SET_NAME,
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
  for (const record of demoOperationalRecords(project.id)) {
    await repository.upsertOperationalRecord(record);
  }
  return project;
}

function sameRuntime(left: Project["runtime"], right: Project["runtime"]): boolean {
  return left.code === right.code
    && left.product === right.product
    && left.stage === right.stage
    && left.progress === right.progress
    && left.status === right.status
    && left.owner === right.owner
    && left.budget === right.budget
    && left.currency === right.currency
    && left.timezone === right.timezone;
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

async function ensureDocument(
  repository: FileRepository,
  artifacts: Artifact[],
  projectId: string,
  content: string,
): Promise<Artifact> {
  return artifacts.find((artifact) => artifact.kind === "document" && artifact.content === content)
    ?? await repository.createArtifact({
      projectId,
      kind: "document",
      status: "ready",
      content,
      actor: DEMO_ACTOR,
    });
}

async function seedDemoTasks(
  repository: FileRepository,
  projectId: string,
  artifacts: {
    brief: Artifact;
    creative: Artifact;
    video: Artifact;
    strategy: Artifact;
    insight: Artifact;
    delivery: Artifact;
  },
): Promise<void> {
  const existingTasks = await repository.listBusinessTasks(projectId);
  const sourceByType = (type: BusinessTaskType) => {
    if (type === "strategy") return {
      sourceTaskIds: [] as string[],
      sourceArtifactIds: [artifacts.brief.id, artifacts.insight.id],
      outputArtifactIds: [artifacts.strategy.id],
    };
    if (type === "creative") return {
      sourceTaskIds: demoTaskIds(existingTasks, ["strategy"]),
      sourceArtifactIds: [artifacts.brief.id, artifacts.strategy.id, artifacts.insight.id],
      outputArtifactIds: [artifacts.creative.id, artifacts.video.id],
    };
    if (type === "video_edit") return {
      sourceTaskIds: demoTaskIds(existingTasks, ["strategy", "creative"]),
      sourceArtifactIds: [artifacts.brief.id, artifacts.strategy.id, artifacts.creative.id, artifacts.video.id],
      outputArtifactIds: [artifacts.delivery.id],
    };
    return {
      sourceTaskIds: demoTaskIds(existingTasks, ["strategy", "creative"]),
      sourceArtifactIds: [artifacts.brief.id, artifacts.strategy.id, artifacts.insight.id],
      outputArtifactIds: [artifacts.video.id],
    };
  };
  for (const demoTask of DEMO_TASKS) {
    const existing = existingTasks.find((task) => task.type === demoTask.type && task.name === demoTask.name);
    const links = sourceByType(demoTask.type);
    if (!existing) {
      const created = await repository.createBusinessTask({
        projectId,
        type: demoTask.type,
        name: demoTask.name,
        objective: demoTask.objective,
        sourceTaskIds: links.sourceTaskIds,
        sourceArtifactIds: links.sourceArtifactIds,
        actor: DEMO_ACTOR,
      });
      existingTasks.push(await repository.updateBusinessTask(created.id, {
        status: demoTask.status,
        outputArtifactIds: links.outputArtifactIds,
        actor: DEMO_ACTOR,
      }));
      continue;
    }
    const needsUpdate = existing.objective !== demoTask.objective
      || existing.status !== demoTask.status
      || !sameStringList(existing.sourceTaskIds, links.sourceTaskIds)
      || !sameStringList(existing.sourceArtifactIds, links.sourceArtifactIds)
      || !sameStringList(existing.outputArtifactIds, links.outputArtifactIds);
    if (needsUpdate) {
      await repository.updateBusinessTask(existing.id, {
        objective: demoTask.objective,
        status: demoTask.status,
        sourceTaskIds: links.sourceTaskIds,
        sourceArtifactIds: links.sourceArtifactIds,
        outputArtifactIds: links.outputArtifactIds,
        actor: DEMO_ACTOR,
      });
    }
  }
}

function demoTaskIds(tasks: BusinessTask[], types: BusinessTaskType[]): string[] {
  return DEMO_TASKS
    .filter((task) => types.includes(task.type))
    .map((demoTask) => tasks.find((task) => task.type === demoTask.type && task.name === demoTask.name)?.id)
    .filter((id): id is string => Boolean(id));
}

function sameStringList(left: string[], right: string[]): boolean {
  return left.length === right.length && left.every((value, index) => value === right[index]);
}

function demoOperationalRecords(projectId: string): UpsertOperationalRecordInput[] {
  return [
    record("WORK-2607-01", projectId, "work_item", "春季新品上市", "待评审", "2026-07-22T08:30:00.000Z", { type: "策略工作区", owner: "Amelia Meng", progress: 82 }),
    record("WORK-2607-02", projectId, "work_item", "精密制造品牌片", "生成中", "2026-07-22T05:48:00.000Z", { type: "创意任务", owner: "Lin Wei", progress: 68 }),
    record("WORK-2607-03", projectId, "work_item", "证据前置实验分析", "需处理", "2026-07-22T07:06:00.000Z", { type: "分析任务", owner: "Sofia Chen", progress: 86 }),
    record("WORK-2607-04", projectId, "work_item", "销售线索增长计划 06", "待审批", "2026-07-22T08:30:00.000Z", { type: "投放计划", owner: "Noah Xu", progress: 82 }),
    record("WORK-2607-05", projectId, "work_item", "华东行业受众研究", "已完成", "2026-07-21T10:40:00.000Z", { type: "研究任务", owner: "Amelia Meng", progress: 100 }),
    record("EV-021", projectId, "evidence", "精密制造行业受众研究", "已确认", "2026-07-20T00:00:00.000Z", { source: "项目研究库", confidence: "高" }),
    record("EV-024", projectId, "evidence", "白域精工近 90 天素材表现", "已确认", "2026-07-22T00:00:00.000Z", { source: "广告数据 Connector", confidence: "高" }),
    record("INS-014", projectId, "evidence", "证据前置创意实验结论", "已确认", "2026-07-22T00:00:00.000Z", { source: "洞察与经验", confidence: "中" }),
    record("EV-027", projectId, "evidence", "10 位采购与研发负责人访谈", "已确认", "2026-07-19T00:00:00.000Z", { source: "飞书文档", confidence: "中" }),
    record("ACT-2607-01", projectId, "activity", "策略 v1.2 已提交评审", "completed", "2026-07-22T02:24:00.000Z", { actor: "Amelia Meng", detail: "引用 5 条证据" }),
    record("ACT-2607-02", projectId, "activity", "创意方向 CR-103 已确认", "completed", "2026-07-22T01:48:00.000Z", { actor: "Lin Wei", detail: "进入视频制作" }),
    record("ACT-2607-03", projectId, "activity", "投放计划完成预算校验", "completed", "2026-07-22T01:12:00.000Z", { actor: "系统", detail: "无阻断风险" }),
    record("ACT-2607-04", projectId, "activity", "洞察 INS-208 被策略引用", "completed", "2026-07-21T08:30:00.000Z", { actor: "系统", detail: "Q2 素材疲劳研究" }),
    record("ACT-2607-05", projectId, "activity", "短剧前贴候选已完成人审", "completed", "2026-07-22T06:36:00.000Z", { actor: "Noah Xu", detail: "选择交期冲突钩子" }),
    record("ACT-2607-06", projectId, "activity", "投放模拟证据包已归档", "completed", "2026-07-22T09:06:00.000Z", { actor: "系统", detail: "写入审批与回滚说明" }),
    record("METRIC-2607-01", projectId, "metric", "证据前置创意表现趋势", "ready", "2026-07-22T08:30:00.000Z", {
      points: "32,39,36,49,54,51,63,67,72,78,75,86",
      unit: "index",
      summary: "证据前置版本，正在形成稳定增量。",
      latest: "86%",
      comparison: "较基线 +18%",
      sample: "48 个有效素材版本",
      confidence: "95% · 差异 +12% 至 +23%",
      scope: "该结论适用于白域精工在中国大陆的销售线索广告。样本仍集中于精密制造题材，跨区域复用前需重新验证。",
      recommendation: "下一轮将证据前置版本扩大到 30% 素材覆盖，并保留纯产品特写作为对照组。",
    }),
    record("METRIC-2607-02", projectId, "metric", "线索质量与表单完成趋势", "ready", "2026-07-22T09:00:00.000Z", {
      points: "41,44,47,52,58,61,66,69,73,76,80,84",
      unit: "score",
      summary: "高意向表单占比持续提升，研发负责人样本贡献最高。",
      latest: "84",
      comparison: "较基线 +21%",
      sample: "312 条模拟线索",
      confidence: "92% · 仍需真实 CRM 回传验证",
      scope: "用于部署后演示线索质量分析，不代表真实投放效果预测。",
      recommendation: "保留研发负责人定向，同时用 10% 预算探索采购负责人长尾关键词。",
    }),
    record("AD-2607-031", projectId, "performance_ad", "精度证据·研发负责人", "持续放量", "2026-07-22T08:30:00.000Z", { platform: "巨量引擎", format: "视频", spend: 28640, impressions: 682400, ctr: 4.18, cpa: 54.2 }),
    record("AD-2607-028", projectId, "performance_ad", "真实制造场景·采购线", "稳定", "2026-07-21T08:30:00.000Z", { platform: "腾讯广告", format: "图文", spend: 21800, impressions: 486200, ctr: 3.74, cpa: 61.8 }),
    record("AD-2607-019", projectId, "performance_ad", "短剧前贴·交期冲突", "优先扩量", "2026-07-18T08:30:00.000Z", { platform: "巨量引擎", format: "视频", spend: 18420, impressions: 438900, ctr: 4.62, cpa: 49.6 }),
    record("AD-2607-014", projectId, "performance_ad", "游戏前贴·精度挑战", "观察", "2026-07-14T08:30:00.000Z", { platform: "快手磁力", format: "视频", spend: 15680, impressions: 326800, ctr: 3.26, cpa: 68.4 }),
    record("AD-2607-008", projectId, "performance_ad", "纯产品特写·对照组", "建议降量", "2026-07-04T08:30:00.000Z", { platform: "腾讯广告", format: "图文", spend: 13320, impressions: 312100, ctr: 2.41, cpa: 82.7 }),
    record("AD-2607-036", projectId, "performance_ad", "工程师证言·良率证明", "新建观察", "2026-07-22T09:00:00.000Z", { platform: "巨量引擎", format: "视频", spend: 9200, impressions: 184600, ctr: 3.96, cpa: 57.4 }),
    record("MIX-2607-01", projectId, "audience_mix", "动态漫与仿真人", "优先补充验证", "2026-07-22T08:30:00.000Z", { supply: 14, spend: 38.13 }),
    record("MIX-2607-02", projectId, "audience_mix", "表情包与沙雕漫", "提升差异化", "2026-07-22T08:30:00.000Z", { supply: 48, spend: 32.8 }),
    record("MIX-2607-03", projectId, "audience_mix", "小说漫改与有声书", "稳定基础供给", "2026-07-22T08:30:00.000Z", { supply: 36, spend: 25.78 }),
    record("MIX-2607-04", projectId, "audience_mix", "3D 漫剧", "小规模探索", "2026-07-22T08:30:00.000Z", { supply: 0, spend: 2.49 }),
    record("MIX-2607-05", projectId, "audience_mix", "工程师证言与工厂实拍", "建议扩充", "2026-07-22T09:00:00.000Z", { supply: 22, spend: 41.6 }),
    record("METHOD-M1", projectId, "method", "高光剧情切片", "ready", "2026-07-22T08:30:00.000Z", { detail: "最低成本验证剧情与角色" }),
    record("METHOD-M3", projectId, "method", "5-12 秒超短素材", "ready", "2026-07-22T08:30:00.000Z", { detail: "快速验证钩子与 CTA" }),
    record("METHOD-M4", projectId, "method", "文字旁白 + BGM", "ready", "2026-07-22T08:30:00.000Z", { detail: "重构叙事和悬念密度" }),
    record("METHOD-M5", projectId, "method", "解说 + 原剧情", "ready", "2026-07-22T08:30:00.000Z", { detail: "兼顾信息密度与沉浸" }),
    record("METHOD-M6", projectId, "method", "工程师证言 + 微距实拍", "ready", "2026-07-22T09:00:00.000Z", { detail: "补足精度、良率和可制造性证据" }),
    record("DIAG-2607-01", projectId, "delivery_diagnostic", "重复组合", "danger", "2026-07-22T08:30:00.000Z", { value: "12 组", detail: "同一商品、素材和转化目标下存在重复广告" }),
    record("DIAG-2607-02", projectId, "delivery_diagnostic", "新素材覆盖", "warning", "2026-07-22T08:30:00.000Z", { value: "18%", detail: "低于本项目 25% 的探索目标" }),
    record("DIAG-2607-03", projectId, "delivery_diagnostic", "无消耗广告", "warning", "2026-07-22T08:30:00.000Z", { value: "7 条", detail: "已连续 72 小时无有效消耗" }),
    record("DIAG-2607-04", projectId, "delivery_diagnostic", "审批证据完整度", "success", "2026-07-22T09:00:00.000Z", { value: "92%", detail: "Brief、创意、预算和回滚说明均已归档" }),
    record("ACTION-P0", projectId, "delivery_action", "补充 6 条新素材", "P0", "2026-07-22T08:30:00.000Z", { detail: "优先改变核心内容，避免只替换边框或文案", impact: "+12-18% 探索覆盖" }),
    record("ACTION-P1", projectId, "delivery_action", "合并重复广告", "P1", "2026-07-22T08:30:00.000Z", { detail: "保留有效广告，清理 7 条长期无消耗对象", impact: "减少预算内耗" }),
    record("ACTION-P2", projectId, "delivery_action", "建立浅层转化实验", "P2", "2026-07-22T08:30:00.000Z", { detail: "5-10% 预算仅作为来源建议，提交前需人工确认", impact: "扩大探索空间" }),
    record("ACTION-P3", projectId, "delivery_action", "补齐 CRM 回传校验", "P3", "2026-07-22T09:00:00.000Z", { detail: "用线索质量字段校验高意向样本，不把点击率直接等同成交", impact: "提升演示可信度" }),
    record("REC-BRF-2607-11", projectId, "unified_record", "春季新品上市 Brief", "已确认", "2026-07-22T01:12:00.000Z", { kind: "Brief", owner: "Amelia Meng" }),
    record("REC-STR-2607-08", projectId, "unified_record", "精度证据增长策略", "已确认", "2026-07-22T02:24:00.000Z", { kind: "策略", owner: "Amelia Meng" }),
    record("REC-CR-2607-42", projectId, "unified_record", "精密制造图文与视频", "制作中", "2026-07-22T05:48:00.000Z", { kind: "创意", owner: "Lin Wei" }),
    record("REC-INS-2607-14", projectId, "unified_record", "证据前置实验分析", "待确认", "2026-07-22T07:06:00.000Z", { kind: "洞察", owner: "Sofia Chen" }),
    record("REC-PLAN-2607-06", projectId, "unified_record", "销售线索增长计划", "待审批", "2026-07-22T08:30:00.000Z", { kind: "投放", owner: "Noah Xu" }),
    record("REC-CS-2607-018", projectId, "unified_record", "素材组合优化 ChangeSet", "待审批", "2026-07-22T08:30:00.000Z", { kind: "变更", owner: "Noah Xu" }),
    record("REC-EV-2607-24", projectId, "unified_record", "近 90 天素材表现证据", "已确认", "2026-07-22T06:52:00.000Z", { kind: "证据", owner: "Sofia Chen" }),
    record("REC-VER-2607-19", projectId, "unified_record", "图文创意 v1.8", "制作中", "2026-07-22T05:48:00.000Z", { kind: "版本", owner: "Lin Wei" }),
    record("REC-ASSET-2607-31", projectId, "unified_record", "品牌片视频 v1.3", "待评审", "2026-07-22T09:00:00.000Z", { kind: "视频资产", owner: "Lin Wei" }),
    record("REC-AUDIT-2607-04", projectId, "unified_record", "投放模拟审计证据包", "已归档", "2026-07-22T09:06:00.000Z", { kind: "审计", owner: "系统" }),
    record("WORK-2607-06", projectId, "work_item", "部署默认演示数据巡检", "已完成", "2026-07-22T09:10:00.000Z", { type: "上线检查", owner: "系统", progress: 100 }),
    record("EV-031", projectId, "evidence", "工程师证言素材脚本", "已确认", "2026-07-22T09:00:00.000Z", { source: "演示种子", confidence: "中" }),
    record("EV-034", projectId, "evidence", "投放模拟回滚说明", "已确认", "2026-07-22T09:06:00.000Z", { source: "本地审计", confidence: "高" }),
  ];
}

function record(
  id: string,
  projectId: string,
  kind: OperationalRecordKind,
  title: string,
  status: string,
  occurredAt: string,
  fields: Record<string, string | number>,
): UpsertOperationalRecordInput {
  return { id, projectId, kind, title, status, occurredAt, fields };
}
