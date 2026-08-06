import type { FirstFrameCandidate, HookDirection, StoryAnalysis } from './types'

export const fixtureAnalysis: StoryAnalysis = {
  title: '消失的第七份证词',
  episode: '第 1 集',
  synopsis: '林惠整理父亲遗物时，发现一份从未出现在案件卷宗里的录音。录音中的时间与六年前的事故记录完全矛盾，而当年所有知情人都声称没有见过这份证词。',
  openingBeat: '深夜书房里，旧录音机突然播放出一句被刻意剪断的话，林惠停在门口回头。',
  characters: ['林惠｜调查真相的女儿', '父亲｜已故案件记录人'],
  visualKeywords: ['深夜书房', '旧录音机', '红色指示灯', '悬疑冷光'],
}

export const fixtureHooks: HookDirection[] = [
  {
    id: 'curiosity-evidence', category: 'curiosity', eyebrow: '猎奇吸睛 01', title: '不存在的证词',
    description: '用“档案中不存在的录音”制造信息缺口，把真相留到正片。',
    hookCopy: '所有人都说没见过它——可录音里，偏偏有第七个人的声音。',
    rationale: '核心变量：证据异常；适合用特写和声音停顿快速抓住注意力。',
  },
  {
    id: 'curiosity-time', category: 'curiosity', eyebrow: '猎奇吸睛 02', title: '时间对不上',
    description: '从六年前记录中的时间矛盾切入，让观众主动追问谁在说谎。',
    hookCopy: '录音发生在事故之后——可那个人，明明早就不在了。',
    rationale: '核心变量：时间悖论；强悬念，但不提前揭示案件答案。',
  },
  {
    id: 'summary-tape', category: 'summary', eyebrow: '剧情总结 01', title: '一盘录音，重启旧案',
    description: '概括女主从遗物中发现新证据的关键动作，快速交代故事入口。',
    hookCopy: '整理父亲遗物时，她找到一盘足以推翻六年前结论的录音。',
    rationale: '核心变量：剧情起因；信息清楚，适合首次接触该短剧的观众。',
  },
  {
    id: 'summary-truth', category: 'summary', eyebrow: '剧情总结 02', title: '所有人都隐瞒了真相',
    description: '聚焦知情人的集体沉默，突出女主即将追查的核心冲突。',
    hookCopy: '六年前，所有人给出了同一个答案。六年后，她发现他们都在说谎。',
    rationale: '核心变量：人物关系；更偏情绪推动，利于自然进入悬疑正片。',
  },
]

export const fixtureImages: FirstFrameCandidate[] = [
  { id: 'frame-1', label: '证据特写', imageUrl: '/assets/short-drama-preroll-v2/frame-evidence.svg', composition: '录音机红灯与磁带标签近景' },
  { id: 'frame-2', label: '人物反应', imageUrl: '/assets/short-drama-preroll-v2/frame-reaction.svg', composition: '女主回头，前景保留录音机轮廓' },
  { id: 'frame-3', label: '空间悬念', imageUrl: '/assets/short-drama-preroll-v2/frame-room.svg', composition: '深夜书房广角，门缝透出冷光' },
]

export function fixtureImagePrompt(analysis: StoryAnalysis, hook: HookDirection): string {
  return `竖屏 9:16，短剧前贴首帧。${analysis.openingBeat} 主体清晰，第一视觉焦点是旧录音机亮起的红色指示灯，女主位于中景并产生警觉反应。低饱和蓝黑色调，局部暖光，写实电影质感，保留画面上方字幕安全区。对应钩子：${hook.hookCopy} 不出现文字、Logo、水印。`
}

export function fixtureVideoPrompt(analysis: StoryAnalysis, hook: HookDirection, duration: number): string {
  return `${duration} 秒竖屏 9:16 独立短剧前贴。0-1.5 秒：录音机红灯突然亮起，磁带轻微转动，快速建立异常。1.5-${Math.max(3, duration - 1.5)} 秒：镜头缓慢后移，${analysis.openingBeat}，用环境音和人物反应维持悬念。最后 1.5 秒定格关键信息缺口并出现 CTA“点击正片揭开真相”。钩子文案：${hook.hookCopy}。节奏克制但紧张，人物与空间连续，不解释真相，不拼接正片。`
}
