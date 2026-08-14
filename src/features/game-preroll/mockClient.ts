import type { AnalysisFact, BriefField, EvidenceMoment, GenerationConfig, GenerationJob, HookCandidate, SourceVideo } from './model'

const wait = (milliseconds: number) => new Promise(resolve => window.setTimeout(resolve, milliseconds))

export interface GamePrerollClient {
  upload(file: File, previewUrl: string): Promise<SourceVideo>
  analyze(source: SourceVideo): Promise<{ facts: AnalysisFact[]; evidence: EvidenceMoment[]; brief: BriefField[] }>
  plan(brief: BriefField[], evidence: EvidenceMoment[], duration: number): Promise<HookCandidate[]>
  generate(candidate: HookCandidate, config: GenerationConfig): Promise<GenerationJob>
}

export class MockGamePrerollClient implements GamePrerollClient {
  async upload(file: File, previewUrl: string) {
    await wait(450)
    return {
      id: `source-${Date.now()}`, name: file.name, sizeBytes: file.size,
      durationSeconds: 42, previewUrl, rightsConfirmed: true,
    }
  }

  async analyze(_source: SourceVideo) {
    await wait(900)
    const evidence: EvidenceMoment[] = [
      { id: 'evidence-choice', kind: 'operation', label: '技能三选一', description: '画面出现三个可选技能，玩家准备进行选择。', startMs: 20200, endMs: 22200, provenance: 'video_evidence' },
      { id: 'evidence-fail', kind: 'result', label: '选择后失败', description: '选择后战局快速恶化，出现明确失败反馈。', startMs: 29800, endMs: 31400, provenance: 'video_evidence' },
      { id: 'evidence-impact', kind: 'gameplay', label: '技能爆发清屏', description: '技能触发后敌人快速减少，画面反馈强烈。', startMs: 34000, endMs: 35500, provenance: 'video_evidence' },
    ]
    const facts: AnalysisFact[] = [
      { id: 'type', label: '游戏类型', value: '轻策略 / 闯关', provenance: 'ai_inference', evidenceRefs: ['evidence-choice'] },
      { id: 'gameplay', label: '核心玩法', value: '技能三选一', provenance: 'video_evidence', evidenceRefs: ['evidence-choice'] },
      { id: 'operation', label: '操作反馈', value: '选择后立即影响战局', provenance: 'video_evidence', evidenceRefs: ['evidence-choice', 'evidence-fail'] },
      { id: 'result', label: '可见结果', value: '失败反馈 / 快速清屏', provenance: 'video_evidence', evidenceRefs: ['evidence-fail', 'evidence-impact'] },
      { id: 'reward', label: '可见奖励', value: '本段未发现', provenance: 'ai_inference', evidenceRefs: [] },
      { id: 'ui', label: '主要 UI', value: '技能卡、战斗区、结算提示', provenance: 'video_evidence', evidenceRefs: ['evidence-choice'] },
    ]
    const brief: BriefField[] = [
      { id: 'objective', label: '广告目标', value: '促进游戏下载', provenance: 'ai_inference', required: true, evidenceRefs: [] },
      { id: 'audience', label: '目标受众', value: '喜欢轻策略、即时反馈和闯关挑战的玩家', provenance: 'ai_inference', required: true, evidenceRefs: ['evidence-choice'] },
      { id: 'selling-point', label: '主推卖点', value: '技能三选一；选错立即失败；选对可快速清屏', provenance: 'video_evidence', required: true, evidenceRefs: evidence.map(item => item.id) },
      { id: 'cta', label: '行动号召 CTA', value: '立即下载', provenance: 'manual', required: true, evidenceRefs: [] },
    ]
    return { facts, evidence, brief }
  }

  async plan(_brief: BriefField[], evidence: EvidenceMoment[], duration: number) {
    await wait(750)
    const end = `${duration}秒`
    return [
      { id: 'candidate-question', mechanism: 'question', name: '悬念提问', hookLine: '这三个技能，你会选哪一个？', audienceFit: '喜欢做选择、参与感强的轻策略玩家', recommendation: '用选择题制造参与感，适合玩法第一次曝光。', recommended: false, score: 88, evidenceRefs: ['evidence-choice'], risk: '避免字幕遮挡原游戏按钮', beats: [{ id: 'q1', range: '0–1秒', copy: '三个技能同时高亮', evidenceRef: 'evidence-choice' }, { id: 'q2', range: '1–3秒', copy: '停顿并抛出选择问题', evidenceRef: 'evidence-choice' }, { id: 'q3', range: `3–${end}`, copy: '展示选择后的战局变化', evidenceRef: 'evidence-impact' }] },
      { id: 'candidate-reversal', mechanism: 'reversal', name: '冲突反转', hookLine: '我选了最强技能，结果反而输了！', audienceFit: '喜欢反转和失败挑战的玩家', recommendation: '原片有清晰的选择失败证据，能够形成最强信息缺口。', recommended: true, score: 95, evidenceRefs: ['evidence-choice', 'evidence-fail'], risk: '不得虚构游戏结算数值', beats: [{ id: 'r1', range: '0–1秒', copy: '“最强技能”选择特写', evidenceRef: 'evidence-choice' }, { id: 'r2', range: '1–3秒', copy: '角色瞬间失败，形成反转', evidenceRef: 'evidence-fail' }, { id: 'r3', range: `3–${end}`, copy: '回到选择前并提示正确解法', evidenceRef: 'evidence-impact' }] },
      { id: 'candidate-impact', mechanism: 'impact', name: '极致爽感', hookLine: '换完这个技能，一波直接清屏！', audienceFit: '追求爽点和强视觉反馈的玩家', recommendation: '用粒子、清屏和数值反馈让用户静音也能理解。', recommended: false, score: 91, evidenceRefs: ['evidence-impact'], risk: '只放大真实反馈，不生成伪按钮', beats: [{ id: 'i1', range: '0–1秒', copy: '敌人压满屏幕', evidenceRef: 'evidence-fail' }, { id: 'i2', range: '1–3秒', copy: '技能点击并爆发', evidenceRef: 'evidence-choice' }, { id: 'i3', range: `3–${end}`, copy: '清屏结果与 CTA', evidenceRef: 'evidence-impact' }] },
    ] satisfies HookCandidate[]
  }

  async generate(candidate: HookCandidate, config: GenerationConfig) {
    await wait(1400)
    return { id: `mock-job-${Date.now()}`, status: 'succeeded', progress: 100, outputUrl: '', diagnostic: `${candidate.name} · ${config.durationSeconds} 秒前贴已完成前端模拟` } satisfies GenerationJob
  }
}
