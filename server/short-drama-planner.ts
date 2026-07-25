import { DomainError } from "./errors.js";
import type { ShortDramaPrerollArtifactSnapshot } from "./domain.js";

export const SHORT_DRAMA_PREROLL_TYPE = "short_drama";
export const SHORT_DRAMA_PLAN_VERSION = "short_drama_preroll_v1";
export const MIN_SHORT_DRAMA_SYNOPSIS_LENGTH = 40;

export type ShortDramaHookType =
  | "conflict"
  | "reversal"
  | "suspense"
  | "selling_point_bridge";

export interface ShortDramaStoryContext {
  title: string;
  synopsis: string;
  reviewedSellingPoints: string[];
  openingLine?: string;
}

export interface ShortDramaPrerollPlanningInput {
  prerollType: string;
  storyContext: ShortDramaStoryContext;
}

export interface ShortDramaPrerollCandidate {
  id: string;
  hookType: ShortDramaHookType;
  score: number;
  scoreMeaning: "hook_relevance";
  evidence: string[];
  voiceover: string;
  visualIntent: string;
  transitionLine: string;
}

export interface ShortDramaPrerollPlan {
  version: typeof SHORT_DRAMA_PLAN_VERSION;
  candidates: ShortDramaPrerollCandidate[];
}

export interface ShortDramaSelectedCandidateGenerationInput {
  plan: ShortDramaPrerollPlan;
  candidateId: string;
  storyContext: ShortDramaStoryContext;
  confirmedBrief: string;
}

interface ValidatedStoryContext {
  title: string;
  synopsis: string;
  sellingPoints: string[];
  openingLine?: string;
}

interface CandidateTemplate {
  hookType: ShortDramaHookType;
  score: number;
  evidence: string;
  voiceover: string;
  visualIntent: string;
  transitionLine: string;
}

export function planShortDramaPreroll(input: ShortDramaPrerollPlanningInput): ShortDramaPrerollPlan {
  if (input.prerollType !== SHORT_DRAMA_PREROLL_TYPE) {
    throw new DomainError("VALIDATION_ERROR", "Short drama planner only supports the short_drama preroll type");
  }

  const context = validateStoryContext(input.storyContext);
  const primarySellingPoint = context.sellingPoints[0]!;
  const candidates = candidateTemplates(primarySellingPoint).map((template, index) => ({
    id: `short-drama-${template.hookType}-${index + 1}`,
    hookType: template.hookType,
    score: template.score,
    scoreMeaning: "hook_relevance" as const,
    evidence: [template.evidence, `已审核卖点聚焦：${primarySellingPoint}`],
    voiceover: template.voiceover,
    visualIntent: template.visualIntent,
    transitionLine: template.transitionLine,
  }));

  return {
    version: SHORT_DRAMA_PLAN_VERSION,
    candidates: candidates.sort((left, right) => right.score - left.score || left.id.localeCompare(right.id)),
  };
}

export function buildShortDramaPrerollPrompt(
  input: ShortDramaSelectedCandidateGenerationInput,
): string {
  return buildShortDramaPrerollSnapshot(input).prompt;
}

export function buildShortDramaPrerollSnapshot(
  input: ShortDramaSelectedCandidateGenerationInput,
): ShortDramaPrerollArtifactSnapshot {
  if (input.plan.version !== SHORT_DRAMA_PLAN_VERSION) {
    throw new DomainError("VALIDATION_ERROR", "Unsupported short drama plan version");
  }
  const context = validateStoryContext(input.storyContext);
  const candidate = input.plan.candidates.find((item) => item.id === input.candidateId);
  if (!candidate) {
    throw new DomainError("VALIDATION_ERROR", "The selected short drama candidate is not part of this plan");
  }
  const confirmedBrief = redactOpeningLine(requiredText(input.confirmedBrief, "confirmedBrief"), context.openingLine);

  const prompt = [
    "Create a short-drama preroll video.",
    `Story title: ${context.title}.`,
    `Approved synopsis: ${context.synopsis}.`,
    `Confirmed strategy brief: ${confirmedBrief}.`,
    `Selected hook: ${candidate.hookType}.`,
    `Voiceover direction: ${candidate.voiceover}.`,
    `Visual direction: ${candidate.visualIntent}.`,
    `Transition direction: ${candidate.transitionLine}.`,
    "不要逐字复用正片首句。",
  ].join("\n");

  return {
    planVersion: input.plan.version,
    storyContext: {
      title: context.title,
      synopsis: context.synopsis,
      reviewedSellingPoints: context.sellingPoints,
    },
    selectedCandidate: { ...candidate },
    prompt,
  };
}

function validateStoryContext(input: ShortDramaStoryContext): ValidatedStoryContext {
  const openingLine = optionalText(input.openingLine);
  const title = redactOpeningLine(requiredText(input.title, "title"), openingLine);
  const synopsis = redactOpeningLine(requiredText(input.synopsis, "synopsis"), openingLine);
  if (synopsis.length < MIN_SHORT_DRAMA_SYNOPSIS_LENGTH) {
    throw new DomainError(
      "VALIDATION_ERROR",
      `synopsis must be at least ${MIN_SHORT_DRAMA_SYNOPSIS_LENGTH} characters`,
    );
  }
  if (!Array.isArray(input.reviewedSellingPoints) || input.reviewedSellingPoints.length === 0) {
    throw new DomainError("VALIDATION_ERROR", "At least one reviewed selling point is required");
  }

  const sellingPoints = input.reviewedSellingPoints.map((value, index) =>
    redactOpeningLine(requiredText(value, `reviewedSellingPoints[${index}]`), openingLine),
  );
  return { title, synopsis, sellingPoints, openingLine };
}

function candidateTemplates(primarySellingPoint: string): CandidateTemplate[] {
  return [
    {
      hookType: "conflict",
      score: 92,
      evidence: "已审核梗概包含立即对立与时限压力。",
      voiceover: "在关键时刻，主角必须先面对最直接的阻拦。",
      visualIntent: "以人物对峙、压迫式近景和倒计时信息建立冲突。",
      transitionLine: "答案藏在接下来的正片发展里。",
    },
    {
      hookType: "reversal",
      score: 87,
      evidence: "已审核梗概具备身份或局势翻转空间。",
      voiceover: "看似已经注定的局面，下一秒可能完全反转。",
      visualIntent: "用前后反差的表情与场景切换强化反转感。",
      transitionLine: "真正的转折，正片马上揭晓。",
    },
    {
      hookType: "suspense",
      score: 82,
      evidence: "已审核梗概保留了尚未揭开的关键真相。",
      voiceover: "当所有线索指向同一个人，真相却还没有现身。",
      visualIntent: "以证据特写、人物停顿和未完成动作制造悬念。",
      transitionLine: "线索会在正片中继续展开。",
    },
    {
      hookType: "selling_point_bridge",
      score: 77,
      evidence: `已审核卖点可作为前贴与剧情的衔接焦点：${primarySellingPoint}`,
      voiceover: `一场围绕${primarySellingPoint}的较量，才刚刚开始。`,
      visualIntent: "以核心关系或关键物件的连续镜头连接卖点与剧情。",
      transitionLine: "后续剧情将把这场较量推向更深处。",
    },
  ];
}

function requiredText(value: unknown, field: string): string {
  if (typeof value !== "string" || !value.trim()) {
    throw new DomainError("VALIDATION_ERROR", `${field} must be a non-empty string`);
  }
  return value.trim();
}

function optionalText(value: unknown): string | undefined {
  if (value === undefined) return undefined;
  return requiredText(value, "openingLine");
}

function redactOpeningLine(value: string, openingLine: string | undefined): string {
  if (!openingLine) return value;
  return value.split(openingLine).join("").replace(/\s{2,}/g, " ").trim();
}
