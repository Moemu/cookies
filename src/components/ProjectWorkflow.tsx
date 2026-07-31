import { useEffect, useMemo, useState } from 'react'
import {
  Archive,
  ArrowRight,
  BarChart3,
  Check,
  CircleAlert,
  ClipboardCheck,
  FileText,
  Megaphone,
  Scissors,
  SearchCheck,
  Video,
  type LucideIcon,
} from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import { calculateProjectProgress, progressPercentLabel, progressReasonLabel } from '../lib/project-progress'
import type { BusinessTaskType, ProjectProgressStage, ProjectRecord, SystemKey } from '../types'
import { shortId } from '../data/shortId'

type OpenProject = (id: string, system?: SystemKey, navId?: string, objectId?: string, view?: string) => void
type WorkflowState = 'completed' | 'active' | 'pending' | 'blocked'

type WorkflowStage = {
  id: number
  title: string
  summary: string
  input: string
  action: string
  output: string
  gate: string
  owner: string
  system: SystemKey
  navId: string
  view?: string
  cta: string
  icon: LucideIcon
  rawComplete: boolean
  evidence: string
  state: WorkflowState
}

const generatedVideoTypes = new Set<BusinessTaskType>([
  'video',
  'brand_video',
  'short_drama_preroll',
  'game_preroll',
  'commerce_preroll',
  'viral_remake',
])

const stateLabels: Record<WorkflowState, string> = {
  completed: '已交接',
  active: '当前阶段',
  pending: '等待上游',
  blocked: '存在链路缺口',
}

export function ProjectFlowDashboard({ onOpenProject, onManageProject }: { onOpenProject: OpenProject; onManageProject: (id: string) => void }) {
  const { currentProject } = useProject()
  const stages = useMemo(() => deriveWorkflow(currentProject), [currentProject])
  const projectProgress = useMemo(() => calculateProjectProgress(currentProject), [currentProject])
  const progressStageName = projectProgress.stage
  const progressStage = progressStageName === 'unavailable' ? undefined : stages.find(stage => stage.id === workflowIdForProgressStage(progressStageName))
  const currentStage = progressStage
    ?? stages.find(stage => stage.state === 'active')
    ?? stages.find(stage => stage.state === 'blocked')
    ?? stages.at(-1)!
  const [selectedId, setSelectedId] = useState(currentStage.id)
  const selected = stages.find(stage => stage.id === selectedId) ?? currentStage

  useEffect(() => {
    setSelectedId(currentStage.id)
  }, [currentProject.id, currentStage.id])

  return <div className="project-flow-page">
    <header className="flow-page-heading">
      <div><span>{currentProject.code} · {currentProject.brand}</span><h1>{currentProject.name}</h1><p>从需求到经验沉淀的完整广告生产闭环。每一步都显示上游输入、阶段输出和下一位接手人。</p></div>
      <div className="flow-heading-actions"><button className="secondary-button" onClick={() => onManageProject(currentProject.id)}>项目管理</button><div className="flow-current-state"><small>{projectProgress.available ? '当前推进到' : '进度状态'}</small><b>{projectProgress.available ? `${projectProgress.stageLabel} · ${progressPercentLabel(projectProgress, 'stagePercent')}` : '无法计算'}</b><span>{progressReasonLabel(projectProgress)}</span></div></div>
    </header>

    <section className="workflow-map" aria-label="项目八阶段业务流程">
      <svg className="workflow-connector" viewBox="0 0 1000 390" preserveAspectRatio="none" aria-hidden="true">
        <path d="M115 95 H885 Q930 95 930 140 V250 Q930 295 885 295 H115 Q70 295 70 250 V140 Q70 95 115 95"/>
      </svg>
      {stages.map(stage => {
        const Icon = stage.icon
        return <button
          className={`workflow-stage stage-${stage.id} ${stage.state} ${selected.id === stage.id ? 'selected' : ''}`}
          key={stage.id}
          onClick={() => setSelectedId(stage.id)}
          aria-pressed={selected.id === stage.id}
        >
          <span className="workflow-stage-number">{String(stage.id).padStart(2, '0')}</span>
          <span className="workflow-stage-icon"><Icon size={19}/></span>
          <b>{stage.title}</b>
          <small>{stage.summary}</small>
          <em>{stage.state === 'completed' ? <Check size={12}/> : stage.state === 'blocked' ? <CircleAlert size={12}/> : null}{stateLabels[stage.state]}</em>
        </button>
      })}
    </section>

    <section className="workflow-insight-support" aria-label="素材洞察支持链">
      <button onClick={() => onOpenProject(currentProject.id, 'insight', 'prelaunch')}>
        <SearchCheck size={18}/>
        <span><small>投前支持阶段 01 至 04</small><b>历史素材、经验与证据进入 Brief、策略和创意</b></span>
        <ArrowRight size={15}/>
      </button>
      <button onClick={() => onOpenProject(currentProject.id, 'insight', 'performance')}>
        <BarChart3 size={18}/>
        <span><small>投后承接阶段 06 至 08</small><b>投放数据形成复盘，再沉淀为下一轮经验</b></span>
        <ArrowRight size={15}/>
      </button>
    </section>

    <section className="workflow-stage-detail" aria-live="polite">
      <header>
        <div><span>阶段 {String(selected.id).padStart(2, '0')} · {stateLabels[selected.state]}</span><h2>{selected.title}</h2><p>{selected.evidence}</p></div>
        <div><small>当前负责人</small><b>{selected.owner}</b></div>
      </header>
      <div className="workflow-handoff">
        <article><span>01</span><div><small>上游输入</small><b>{selected.input}</b></div></article>
        <ArrowRight size={18}/>
        <article><span>02</span><div><small>当前动作</small><b>{selected.action}</b></div></article>
        <ArrowRight size={18}/>
        <article><span>03</span><div><small>阶段输出</small><b>{selected.output}</b></div></article>
      </div>
      <footer>
        <div><ClipboardCheck size={16}/><span><small>进入下一阶段的门槛</small><b>{selected.gate}</b></span></div>
        <button className="primary-button" onClick={() => onOpenProject(currentProject.id, selected.system, selected.navId, undefined, selected.view)}>{selected.cta}<ArrowRight size={15}/></button>
      </footer>
    </section>

    <div className="workflow-loop-note"><Archive size={16}/><span><b>经验会回到下一轮需求</b><small>数据复盘确认的经验沉淀为项目资产，新建任务时自动作为证据来源。</small></span></div>
  </div>
}

function workflowIdForProgressStage(stage: ProjectProgressStage): number {
  const stageToWorkflowId: Record<ProjectProgressStage, number> = {
    intake: 1,
    strategy: 1,
    creative: 3,
    quality_check: 5,
    human_review: 5,
    delivery: 6,
    completed: 8,
  }
  return stageToWorkflowId[stage]
}

function deriveWorkflow(project: ProjectRecord): WorkflowStage[] {
  const strategyTask = latestTask(project, task => task.type === 'strategy')
  const scriptTask = latestTask(project, task => task.type === 'creative' || task.type === 'brand_video')
  const generationTask = latestTask(project, task => generatedVideoTypes.has(task.type))
  const editTask = latestTask(project, task => task.type === 'video_edit')
  const strategyReady = Boolean(project.artifacts.brief.id)
    && project.artifacts.brief.status === '已确认'
    && Boolean(strategyTask)
  const scriptReady = Boolean(scriptTask && ['ready', 'completed'].includes(scriptTask.status))
  const generationReady = Boolean(generationTask && ['ready', 'completed'].includes(generationTask.status))
  const editReady = Boolean(editTask && ['ready', 'completed'].includes(editTask.status))
  const reviewReady = project.artifacts.creative.status === '已确认' || project.artifacts.creative.status === '已完成'
  const executedChangeSet = project.changeSets.find(changeSet => changeSet.status === '已执行')
  const deliveryReady = Boolean(executedChangeSet)
  const insightReady = project.artifacts.insight.status === '已确认'
  const knowledgeReady = project.knowledgeCount > 0
  const definitions: Array<Omit<WorkflowStage, 'state'>> = [
    {
      id: 1, title: '需求下达', summary: '明确目标 · 拆解任务', icon: FileText,
      input: `${project.brand}的业务目标、边界与投前洞察`, action: '引用历史证据，补齐 Brief，创建策略任务并确认成功指标',
      output: '已确认 Brief + 策略任务', gate: 'Brief 已确认，策略任务已创建',
      owner: '品牌负责人 / 策略', system: 'strategy', navId: 'tasks', cta: '进入需求与策略',
      rawComplete: strategyReady,
      evidence: strategyReady ? `Brief ${project.artifacts.brief.version} 已确认，策略任务 ${shortId(strategyTask?.id)}` : '需要完成 Brief 确认并创建策略任务。',
    },
    {
      id: 2, title: '脚本创作', summary: '输出脚本 · 任务指派', icon: ClipboardCheck,
      input: '已确认 Brief、策略任务、研究证据与投前洞察', action: '形成创意命题、脚本、分镜和渠道规格',
      output: '可制作脚本 + 创意任务', gate: '创意任务进入待评审或已完成',
      owner: '创意策划', system: 'creative', navId: 'tasks', cta: '进入脚本创作',
      rawComplete: scriptReady,
      evidence: scriptTask ? `${scriptTask.name}，当前状态 ${taskStatusLabel(scriptTask.status)}` : '等待从已确认策略创建创意或品牌广告任务。',
    },
    {
      id: 3, title: '广告素材生成', summary: '生成画面 · 同步素材', icon: Video,
      input: '已评审脚本、分镜、品牌约束与授权来源', action: '生成品牌广告、短剧、游戏、电商前贴或爆款复刻素材',
      output: '带版本和来源的广告素材', gate: '至少一个视频生成任务进入待评审或已完成',
      owner: 'AI 制作 / 创意', system: 'creative', navId: 'video', view: '效果广告', cta: '进入广告素材生成',
      rawComplete: generationReady,
      evidence: generationTask ? `${generationTask.name}，当前状态 ${taskStatusLabel(generationTask.status)}` : '等待选择广告类型并创建素材生成任务。',
    },
    {
      id: 4, title: '剪辑出片', summary: '成片剪辑 · 版本管理', icon: Scissors,
      input: '已生成视频、授权素材、字幕和声音', action: '完成混剪、包装、字幕、音频和品牌片尾',
      output: '可审核成片版本', gate: '视频包装任务进入待评审或已完成',
      owner: '剪辑师 / 创意', system: 'creative', navId: 'video', view: '素材剪辑', cta: '进入剪辑出片',
      rawComplete: editReady,
      evidence: editTask ? `${editTask.name}，当前状态 ${taskStatusLabel(editTask.status)}` : '等待把生成素材加入视频包装任务。',
    },
    {
      id: 5, title: '审核协同', summary: '精准批注 · 意见留痕', icon: ClipboardCheck,
      input: '成片、脚本、事实证据和品牌检查结果', action: '完成逐帧批注、合规检查、审批和版本确认',
      output: '已确认创意 + 审批证据', gate: '创意产物状态为已确认或已完成',
      owner: '品牌 / 法务 / 审批人', system: 'creative', navId: 'reviews', cta: '进入审核协同',
      rawComplete: reviewReady,
      evidence: reviewReady ? `创意产物 ${project.artifacts.creative.version} 已通过确认。` : '等待成片进入评审并完成品牌、事实和版权检查。',
    },
    {
      id: 6, title: '广告投放', summary: '受控投放 · 自动盯盘', icon: Megaphone,
      input: '已确认创意、渠道规格、预算和回滚方案', action: '生成 ChangeSet，预检、审批并执行投放',
      output: '执行记录 + 投放证据', gate: 'ChangeSet 已审批并执行',
      owner: '广告投手 / 审批人', system: 'delivery', navId: 'plans', cta: '进入广告投放',
      rawComplete: deliveryReady,
      evidence: executedChangeSet ? `${executedChangeSet.title} 已执行，版本 v${executedChangeSet.version}` : `${project.changeSets.length} 个 ChangeSet，尚未形成已执行记录。`,
    },
    {
      id: 7, title: '数据复盘', summary: '数据分析 · 迭代优化', icon: BarChart3,
      input: '平台消耗、曝光、点击、转化和素材表现', action: '分析发生了什么、为什么发生，并形成下一步建议',
      output: '已确认洞察报告', gate: '效果报告已保存并确认',
      owner: '素材分析 / 投手', system: 'insight', navId: 'performance', cta: '进入数据复盘',
      rawComplete: insightReady,
      evidence: insightReady ? `洞察报告 ${project.artifacts.insight.version} 已确认。` : '广告数据可查看，尚未形成已确认的项目洞察报告。',
    },
    {
      id: 8, title: '经验沉淀', summary: '资产沉淀 · 知识复用', icon: Archive,
      input: '经过样本、置信范围和业务边界验证的洞察', action: '沉淀适用条件、证据和不可复用边界',
      output: '可供下一轮调用的经验资产', gate: '至少一条经验已写入项目知识资产',
      owner: '策略 / 素材分析', system: 'insight', navId: 'knowledge', cta: '进入经验沉淀',
      rawComplete: knowledgeReady,
      evidence: knowledgeReady ? `${project.knowledgeCount} 条经验已沉淀，可供下一轮策略复用。` : '等待把复盘结论确认为带证据和边界的经验。',
    },
  ]

  let foundActive = false
  return definitions.map(stage => {
    let state: WorkflowState
    if (!foundActive && stage.rawComplete) state = 'completed'
    else if (!foundActive) {
      state = 'active'
      foundActive = true
    } else if (stage.rawComplete) state = 'blocked'
    else state = 'pending'
    return { ...stage, state }
  })
}

function latestTask(project: ProjectRecord, predicate: (task: ProjectRecord['tasks'][number]) => boolean) {
  return project.tasks.filter(predicate).slice().sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))[0]
}

function taskStatusLabel(status: ProjectRecord['tasks'][number]['status']): string {
  return ({ draft: '草稿', in_progress: '进行中', ready: '待评审', completed: '已完成', failed: '失败' })[status]
}
