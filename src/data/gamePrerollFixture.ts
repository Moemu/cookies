import type { ApiCreateManualGamePrerollInput } from './api'

export const defendSunflowerFixture = {
  id: 'fixture_defend_sunflower_v1',
  version: 1,
  name: '《保卫向日葵》技能选择挑战',
  gameName: '保卫向日葵',
  objective: '用真实技能选择建立挑战感，在 6 秒内让用户理解玩法并点击下载。',
  audience: '喜欢轻策略塔防、技能搭配和竖屏闯关的游戏用户',
  coreMessage: '在波次战斗之间选择技能，决定下一段塔防节奏。',
  gameplaySummary: '竖屏塔防战斗中，玩家在波次间进行技能三选一，选择后继续进入下一波战斗。',
  callToAction: '立即下载',
  sourceGuide: '请选择已授权的 35.74 秒竖屏 MP4 实录；系统只使用 20.292–22.250 秒、29.792–31.375 秒和 34.000–35.500 秒的已核验事实。',
  evidenceMoments: [
    {
      id: 'skill_choice_1',
      kind: 'skill_choice' as const,
      start_milliseconds: 20292,
      end_milliseconds: 22250,
      description: '第 1 次技能三选一，画面展示幻系易伤、怪物易伤、获得格子。',
      verified_copy: ['幻系易伤', '怪物易伤', '获得格子'],
    },
    {
      id: 'skill_choice_2',
      kind: 'skill_choice' as const,
      start_milliseconds: 29792,
      end_milliseconds: 31375,
      description: '第 2 次技能三选一，画面展示激光弹射、格子概率、全体加攻。',
      verified_copy: ['激光弹射', '格子概率', '全体加攻'],
    },
    {
      id: 'wave_2',
      kind: 'wave_progress' as const,
      start_milliseconds: 34000,
      end_milliseconds: 35500,
      description: '技能选择完成后进入第 2/10 波。',
      verified_copy: ['第2/10波'],
    },
  ],
  allowedMechanisms: ['choice_challenge', 'tactical_tradeoff', 'wave_escalation'] as const,
  prohibitedMechanisms: ['failure_reversal', 'merge_upgrade', 'reward_reveal'] as const,
  mandatoryElements: ['保留真实游戏名、技能名、数值和波次', '结尾 CTA 为“立即下载”'],
  prohibitedClaims: [
    '不得虚构失败、复活、合成、升级、奖励或胜利结果',
    '不得出现“观看广告，免费刷新植物”弹窗',
    '不得宣称技能选择产生了素材未证明的结果',
  ],
}

export function buildDefendSunflowerInput(
  sourceVideo: ApiCreateManualGamePrerollInput['sourceVideo'],
): ApiCreateManualGamePrerollInput {
  return {
    briefId: defendSunflowerFixture.id,
    briefVersion: defendSunflowerFixture.version,
    briefName: defendSunflowerFixture.name,
    gameName: defendSunflowerFixture.gameName,
    gameplaySummary: defendSunflowerFixture.gameplaySummary,
    sourceVideo,
    evidenceMoments: defendSunflowerFixture.evidenceMoments.map(moment => ({
      ...moment,
      verified_copy: [...moment.verified_copy],
    })),
    allowedMechanisms: [...defendSunflowerFixture.allowedMechanisms],
    prohibitedMechanisms: [...defendSunflowerFixture.prohibitedMechanisms],
    subtitleStyle: 'high_contrast_dynamic',
    hookStrength: 4,
    paceProfile: 'punchy',
    objective: defendSunflowerFixture.objective,
    audience: defendSunflowerFixture.audience,
    coreMessage: defendSunflowerFixture.coreMessage,
    callToAction: defendSunflowerFixture.callToAction,
    mandatoryElements: [...defendSunflowerFixture.mandatoryElements],
    prohibitedClaims: [...defendSunflowerFixture.prohibitedClaims],
  }
}
