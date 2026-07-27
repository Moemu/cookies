import assert from "node:assert/strict";
import test from "node:test";
import { buildShortDramaPrerollPrompt, planShortDramaPreroll } from "./short-drama-planner.js";
import { isDomainError } from "./errors.js";

const storyContext = {
  title: "雨夜归来的继承人",
  synopsis: "被逐出家门的女主在雨夜带着证据归来，发现继母正在转移家产。你以为我今晚回来，是为了求你吗？她必须在家族晚宴前揭开真相。",
  reviewedSellingPoints: ["豪门继承权争夺", "雨夜归来反转"],
  openingLine: "你以为我今晚回来，是为了求你吗？",
};

test("短剧前贴规划器稳定生成按评分排序的四类候选", () => {
  const first = planShortDramaPreroll({ prerollType: "short_drama", storyContext });
  const second = planShortDramaPreroll({ prerollType: "short_drama", storyContext });

  assert.deepEqual(second, first);
  assert.equal(first.candidates.length, 4);
  assert.deepEqual(
    first.candidates.map((candidate) => candidate.hookType),
    ["conflict", "reversal", "suspense", "selling_point_bridge"],
  );
  assert.ok(first.candidates.every((candidate) => candidate.score >= 0 && candidate.score <= 100));
  assert.ok(first.candidates.every((candidate) => candidate.scoreMeaning === "hook_relevance"));
  assert.ok(first.candidates.every((candidate, index, candidates) =>
    index === 0 || candidates[index - 1]!.score >= candidate.score,
  ));
  assert.ok(first.candidates.every((candidate) =>
    candidate.evidence.length > 0
    && candidate.voiceover.length > 0
    && candidate.visualIntent.length > 0
    && candidate.transitionLine.length > 0,
  ));
});

test("短剧前贴规划器拒绝缺失卖点、无效类型和不足长度的梗概", () => {
  assert.throws(
    () => planShortDramaPreroll({
      prerollType: "short_drama",
      storyContext: { ...storyContext, reviewedSellingPoints: [] },
    }),
    (error: unknown) => isDomainError(error, "VALIDATION_ERROR"),
  );
  assert.throws(
    () => planShortDramaPreroll({ prerollType: "game", storyContext }),
    (error: unknown) => isDomainError(error, "VALIDATION_ERROR"),
  );
  assert.throws(
    () => planShortDramaPreroll({
      prerollType: "short_drama",
      storyContext: { ...storyContext, synopsis: "太短" },
    }),
    (error: unknown) => isDomainError(error, "VALIDATION_ERROR"),
  );
});

test("短剧前贴候选和受控 Prompt 不逐字复用正片首句", () => {
  const plan = planShortDramaPreroll({ prerollType: "short_drama", storyContext });
  const prompt = buildShortDramaPrerollPrompt({
    plan,
    candidateId: plan.candidates[0]!.id,
    storyContext,
    confirmedBrief: "突出高冲突开场，并在前贴结尾自然引向正片。",
  });
  const serialized = JSON.stringify({ plan, prompt });

  assert.equal(serialized.includes(storyContext.openingLine), false);
  assert.match(prompt, /不要逐字复用正片首句/);
  assert.throws(
    () => buildShortDramaPrerollPrompt({
      plan,
      candidateId: "forged-candidate",
      storyContext,
      confirmedBrief: "已确认 Brief",
    }),
    (error: unknown) => isDomainError(error, "VALIDATION_ERROR"),
  );
});
