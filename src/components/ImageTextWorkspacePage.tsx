import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react'
import {
  Check,
  CircleAlert,
  Image as ImageIcon,
  RefreshCw,
  Sparkles,
  WandSparkles,
} from 'lucide-react'
import { useProject } from '../context/ProjectContext'
import {
  api,
  type ApiCreateManualImageTextInput,
  type ApiCreativeDirection,
  type ApiCreativeIntakeBootstrap,
  type ApiImageTextAttempt,
  type ApiImageTextWorkspace,
  type ApiCreativeVersion,
} from '../data/api'
import type { DataState } from '../types'
import { StateBoundary } from './StateBoundary'

const roleLabels = {
  cover: '封面主张',
  proof: '核心证据',
  cta: '行动引导',
} as const

const statusLabels: Record<string, string> = {
  queued: '等待生成',
  running: '底图生成中',
  base_asset_ready: '底图已生成',
  rendering: '排版合成中',
  succeeded: '成品已就绪',
  failed: '生成失败',
  cancelled: '已取消',
  stale: '草稿已更新',
}

const activeStatuses = new Set(['queued', 'running', 'base_asset_ready', 'rendering'])
const blockerLabels: Record<string, string> = {
  unsupported_creative_route: '当前任务不是小红书 3:4 图文任务',
  planning_context_invalid: '策略交接内容尚未达到可生成状态',
  direction_not_confirmed: '请先人工确认 Creative Direction',
  input_identity_mismatch: '策略、方向和草稿的版本来源不一致',
  draft_v2_required: '请先生成新版图文方案',
  image_plan_incomplete: '封面、证据和行动三张图的计划不完整',
  visual_brief_missing: '有图片缺少画面说明',
  overlay_copy_invalid: '有图片缺少排版文案或文案过长',
  source_asset_unstable: '必需素材尚未形成稳定版本',
  source_asset_rights_blocked: '来源素材的生成或改编授权不足',
  renderer_unavailable: '服务端尚未配置中文排版字体',
  task_not_authoring: '素材已经进入审核或交付阶段，不能继续生成',
}

type ManualImageTextForm = {
  objective: string
  audience: string
  coreMessage: string
  callToAction: string
  tone: string
  visualKeywords: string
  mandatoryElements: string
  prohibitedClaims: string
}

const emptyManualForm: ManualImageTextForm = {
  objective: '',
  audience: '',
  coreMessage: '',
  callToAction: '',
  tone: '',
  visualKeywords: '',
  mandatoryElements: '',
  prohibitedClaims: '',
}

function parseList(value: string) {
  return value.split(/[，,；;\n]/).map(item => item.trim()).filter(Boolean)
}

export function ImageTextWorkspacePage({
  state,
  activeTaskId,
  onTaskCreated,
}: {
  state: DataState
  activeTaskId?: string
  onTaskCreated?: (taskId: string) => void
}) {
  const { currentProject } = useProject()
  const [workspace, setWorkspace] = useState<ApiImageTextWorkspace | null>(null)
  const [selectedOrder, setSelectedOrder] = useState(1)
  const [previewURL, setPreviewURL] = useState('')
  const [busyAction, setBusyAction] = useState('')
  const [notice, setNotice] = useState('')
  const [intake, setIntake] = useState<ApiCreativeIntakeBootstrap | null>(null)
  const [directions, setDirections] = useState<ApiCreativeDirection[]>([])
  const [editedTitle, setEditedTitle] = useState('')
  const [editedBody, setEditedBody] = useState('')
  const [editedOverlays, setEditedOverlays] = useState<Record<number, string>>({})
  const [editedVisualBriefs, setEditedVisualBriefs] = useState<Record<number, string>>({})
  const [editedCaptions, setEditedCaptions] = useState<Record<number, string>>({})
  const [creativeVersion, setCreativeVersion] = useState<ApiCreativeVersion | null>(null)
  const [manualForm, setManualForm] = useState<ManualImageTextForm>(emptyManualForm)

  const loadWorkspace = useCallback(async () => {
    if (!activeTaskId) {
      setWorkspace(null)
      return
    }
    try {
      const value = await api.getImageTextWorkspace(currentProject.id, activeTaskId)
      setWorkspace(value)
      setIntake(null)
      setNotice('')
      if (value.readiness.review_ready) {
        try {
          const versions = await api.listImageTextVersions(currentProject.id, activeTaskId)
          setCreativeVersion(versions.items[0] ?? null)
        } catch (cause) {
          setCreativeVersion(null)
          setNotice(cause instanceof Error ? cause.message : '交付版本加载失败')
        }
      } else {
        setCreativeVersion(null)
      }
    } catch (cause) {
      setWorkspace(null)
      try {
        const intakeValue = await api.getCreativeIntake(currentProject.id, activeTaskId)
        setIntake(intakeValue)
        setNotice('')
      } catch {
        setIntake(null)
        setNotice(cause instanceof Error ? cause.message : '图文工作台加载失败')
      }
    }
  }, [activeTaskId, currentProject.id])

  useEffect(() => {
    void loadWorkspace()
  }, [loadWorkspace])

  useEffect(() => {
    if (!workspace) return
    setEditedTitle(workspace.draft.selected_title ?? '')
    setEditedBody(workspace.draft.body)
    setEditedOverlays(Object.fromEntries(
      workspace.draft.image_plan.map(item => [item.order, item.overlay_copy ?? '']),
    ))
    setEditedVisualBriefs(Object.fromEntries(
      workspace.draft.image_plan.map(item => [item.order, item.visual_brief]),
    ))
    setEditedCaptions(Object.fromEntries(
      workspace.draft.image_plan.map(item => [item.order, item.caption]),
    ))
  }, [workspace?.draft.version])

  const selectedSlot = workspace?.slots.find(slot => slot.order === selectedOrder)
  const selectedPlan = workspace?.draft.image_plan.find(item => item.order === selectedOrder)
  const adoptedAttempt = selectedSlot?.attempts.find(
    attempt => attempt.id === selectedSlot.adopted_attempt_id,
  )
  const visibleAttempt = adoptedAttempt
    ?? [...(selectedSlot?.attempts ?? [])].reverse().find(attempt => attempt.status === 'succeeded')

  useEffect(() => {
    let active = true
    setPreviewURL('')
    if (!visibleAttempt?.final_asset_ref) return () => { active = false }
    void api.getProjectAssetPreview(currentProject.id, visibleAttempt.final_asset_ref)
      .then(url => {
        if (active) setPreviewURL(url)
      })
      .catch(() => {
        if (active) setNotice('成品已经生成，但预览地址暂时不可用')
      })
    return () => { active = false }
  }, [currentProject.id, visibleAttempt?.final_asset_ref?.asset_id, visibleAttempt?.final_asset_ref?.version])

  const hasActiveAttempt = useMemo(
    () => workspace?.slots.some(slot => slot.attempts.some(attempt => activeStatuses.has(attempt.status))) ?? false,
    [workspace],
  )

  useEffect(() => {
    if (!hasActiveAttempt) return
    const timer = window.setInterval(() => void loadWorkspace(), 1800)
    return () => window.clearInterval(timer)
  }, [hasActiveAttempt, loadWorkspace])

  const generateDraft = async () => {
    if (!workspace || !activeTaskId) return
    setBusyAction('draft')
    try {
      await api.generateImageTextDraft(
        currentProject.id,
        activeTaskId,
        workspace.task.version,
        workspace.task.direction.direction_version_id,
      )
      await loadWorkspace()
      setNotice('文案和三张图片的创作计划已生成')
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '图文草稿生成失败')
    } finally {
      setBusyAction('')
    }
  }

  const generateDirections = async () => {
    if (!activeTaskId) return
    setBusyAction('directions')
    try {
      const batch = await api.generateCreativeDirections(currentProject.id, activeTaskId)
      setDirections(batch.candidates)
      setNotice('已生成 3 个 Creative Direction，请人工选择一个方向')
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : 'Creative Direction 生成失败')
    } finally {
      setBusyAction('')
    }
  }

  const confirmDirection = async (direction: ApiCreativeDirection) => {
    if (!activeTaskId) return
    setBusyAction(`direction-${direction.direction_id}`)
    try {
      const confirmed = await api.confirmCreativeDirection(currentProject.id, direction.direction_id)
      const task = await api.createImageTextTaskFromDirection(
        currentProject.id,
        activeTaskId,
        confirmed.direction_id,
      )
      setNotice('Creative Direction 已确认，图文任务已创建')
      onTaskCreated?.(task.id)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '确认 Creative Direction 失败')
    } finally {
      setBusyAction('')
    }
  }

  const generateSlot = async (order: number, retry: boolean) => {
    if (!workspace || !activeTaskId) return
    setBusyAction(`slot-${order}`)
    try {
      await api.generateImageTextSlot(
        currentProject.id,
        activeTaskId,
        order,
        workspace.task.version,
        workspace.draft.version,
        retry,
      )
      await loadWorkspace()
      setNotice(`第 ${order} 张图片已进入生成队列`)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '图片生成失败')
    } finally {
      setBusyAction('')
    }
  }

  const saveDraft = async () => {
    if (!workspace || !activeTaskId) return
    setBusyAction('save-draft')
    try {
      await api.updateImageTextDraft(currentProject.id, activeTaskId, workspace, {
        selectedTitle: editedTitle,
        body: editedBody,
        overlayCopy: editedOverlays,
        visualBrief: editedVisualBriefs,
        caption: editedCaptions,
      })
      await loadWorkspace()
      setNotice('图文文案已保存为新的草稿版本')
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '图文文案保存失败')
    } finally {
      setBusyAction('')
    }
  }

  const adoptAttempt = async (attempt: ApiImageTextAttempt) => {
    if (!workspace || !activeTaskId || attempt.status !== 'succeeded') return
    const currentSelectionVersion = selectedSlot?.selection_version ?? 0
    setBusyAction(`adopt-${attempt.id}`)
    try {
      await api.adoptImageTextAttempt(
        currentProject.id,
        activeTaskId,
        attempt.image_plan_order,
        attempt.id,
        workspace.task.version,
        currentSelectionVersion,
      )
      await loadWorkspace()
      setNotice(`已采用第 ${attempt.attempt_no} 次生成结果`)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '采用图片失败')
    } finally {
      setBusyAction('')
    }
  }

  const advanceDelivery = async () => {
    if (!workspace || !activeTaskId) return
    setBusyAction('delivery')
    try {
      let next = creativeVersion
      if (!next) {
        next = await api.freezeImageTextVersion(
          currentProject.id, activeTaskId, workspace.draft.version,
        )
      } else if (next.status === 'created') {
        next = await api.checkImageTextVersion(currentProject.id, next.id)
      } else if (next.status === 'checked') {
        if (next.check && !next.check.passed) {
          throw new Error('交付检查未通过，请先处理检查问题')
        }
        next = await api.approveImageTextVersion(currentProject.id, next.id)
      } else if (next.status === 'approved') {
        await api.deliverImageTextVersion(currentProject.id, next.id)
        next = { ...next, status: 'delivered' }
      }
      setCreativeVersion(next)
      setNotice(next?.status === 'delivered' ? '不可变 CreativePackage 已生成' : '交付流程已推进一步')
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '交付流程推进失败')
    } finally {
      setBusyAction('')
    }
  }

  const updateManualField = (field: keyof ManualImageTextForm, value: string) => {
    setManualForm(current => ({ ...current, [field]: value }))
  }

  const createManualIntake = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setBusyAction('manual-intake')
    setNotice('')
    const input: ApiCreateManualImageTextInput = {
      objective: manualForm.objective,
      audience: manualForm.audience,
      coreMessage: manualForm.coreMessage,
      callToAction: manualForm.callToAction,
      tone: parseList(manualForm.tone),
      visualKeywords: parseList(manualForm.visualKeywords),
      mandatoryElements: parseList(manualForm.mandatoryElements),
      prohibitedClaims: parseList(manualForm.prohibitedClaims),
    }
    try {
      const created = await api.createManualImageTextIntake(currentProject.id, input)
      setNotice('创作需求已保存，请继续确认 Creative Direction')
      onTaskCreated?.(created.id)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '创作需求提交失败')
    } finally {
      setBusyAction('')
    }
  }

  if (!activeTaskId) {
    return <StateBoundary state={state}>
      <form className="image-text-direct-entry" onSubmit={createManualIntake}>
        <header>
          <span className="section-label">图文创作 · 直接输入</span>
          <div className="image-text-direct-title">
            <ImageIcon size={28}/>
            <div>
              <h2>告诉我们这次要创作什么</h2>
              <p>可以直接填写创作需求，也可以继续从策略任务携带已确认的上下文进入。</p>
            </div>
          </div>
        </header>
        <div className="image-text-direct-grid">
          <label className="wide">
            <span>创作目标 <em>必填</em></span>
            <textarea
              value={manualForm.objective}
              onChange={event => updateManualField('objective', event.target.value)}
              placeholder="例如：为新品上市建立第一轮认知"
              maxLength={500}
              required
            />
          </label>
          <label>
            <span>目标受众 <em>必填</em></span>
            <input
              value={manualForm.audience}
              onChange={event => updateManualField('audience', event.target.value)}
              placeholder="例如：关注通勤饮品的年轻用户"
              maxLength={500}
              required
            />
          </label>
          <label>
            <span>行动引导</span>
            <input
              value={manualForm.callToAction}
              onChange={event => updateManualField('callToAction', event.target.value)}
              placeholder="例如：搜索品牌了解更多"
              maxLength={300}
            />
          </label>
          <label className="wide">
            <span>核心信息 <em>必填</em></span>
            <textarea
              value={manualForm.coreMessage}
              onChange={event => updateManualField('coreMessage', event.target.value)}
              placeholder="这篇内容最需要让用户记住什么？"
              maxLength={1000}
              required
            />
          </label>
          <label>
            <span>语气风格</span>
            <input
              value={manualForm.tone}
              onChange={event => updateManualField('tone', event.target.value)}
              placeholder="清爽，克制，可信"
            />
          </label>
          <label>
            <span>视觉关键词</span>
            <input
              value={manualForm.visualKeywords}
              onChange={event => updateManualField('visualKeywords', event.target.value)}
              placeholder="青柠绿，生活方式，留白"
            />
          </label>
          <label>
            <span>必须出现</span>
            <textarea
              value={manualForm.mandatoryElements}
              onChange={event => updateManualField('mandatoryElements', event.target.value)}
              placeholder="品牌名、产品卖点；用逗号或换行分隔"
            />
          </label>
          <label>
            <span>禁止表达</span>
            <textarea
              value={manualForm.prohibitedClaims}
              onChange={event => updateManualField('prohibitedClaims', event.target.value)}
              placeholder="不得虚构功效；用逗号或换行分隔"
            />
          </label>
        </div>
        <footer className="image-text-direct-actions">
          <p>提交后先生成 3 个创意方向，由你确认后再创建图文任务。</p>
          <button
            className="primary-button"
            type="submit"
            disabled={busyAction === 'manual-intake'
              || !manualForm.objective.trim()
              || !manualForm.audience.trim()
              || !manualForm.coreMessage.trim()}
          >
            <Sparkles size={15}/>{busyAction === 'manual-intake' ? '正在创建…' : '创建并规划创意方向'}
          </button>
        </footer>
        {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
      </form>
    </StateBoundary>
  }

  if (!workspace && intake) {
    return <StateBoundary state={state}>
      <div className="image-text-direction-gate">
        <header>
          <span className="section-label">
            {intake.source === 'manual' ? '直接创作 · Creative Intake' : 'Strategy → Creative 交接'}
          </span>
          <h2>先确认创意方向，再开始图文制作</h2>
          <p>{intake.request?.objective
            || intake.request?.core_message
            || intake.base_handoff?.creative_view?.objective?.statement
            || intake.base_handoff?.creative_view?.communication?.single_minded_proposition
            || (intake.source === 'manual'
              ? '创作需求已经保存，Creative 将据此提出可执行的创意方向。'
              : '策略上下文已经冻结，Creative 将据此提出可执行的创意方向。')}</p>
        </header>
        {directions.length === 0 ? <section className="image-text-v2-start">
          <Sparkles size={24}/>
          <div>
            <h3>由 LLM 生成 Creative Direction 候选</h3>
            <p>{intake.source === 'manual'
              ? '系统会基于目标、受众、核心信息与表达边界提出候选；输出只负责创意概念与执行方向。'
              : '输入同时包含通用策略、任务级策略、受众、信息优先级、证据和边界；输出只负责创意概念与执行方向。'}</p>
          </div>
          <button className="primary-button" disabled={busyAction === 'directions'} onClick={() => void generateDirections()}>
            <WandSparkles size={15}/>{busyAction === 'directions' ? '正在生成…' : '生成 3 个候选方向'}
          </button>
        </section> : <>
          <div className="image-text-direction-cards">
          {directions.map((direction, index) => <article key={direction.direction_id}>
            <span>方向 0{index + 1}</span>
            <h3>{direction.concept}</h3>
            <p>{direction.creative_rationale}</p>
            <ul>{direction.execution_outline.map(item => <li key={item}>{item}</li>)}</ul>
            <small>边界：{direction.guardrail_trace.join('；')}</small>
            <button
              className="primary-button full"
              disabled={busyAction.startsWith('direction-')}
              onClick={() => void confirmDirection(direction)}
            >确认并创建图文任务</button>
          </article>)}
          </div>
          <button className="secondary-button" disabled={busyAction === 'directions'} onClick={() => void generateDirections()}>
            <WandSparkles size={15}/>{busyAction === 'directions' ? '正在重新生成…' : '重新生成 3 个方向'}
          </button>
        </>}
        {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
      </div>
    </StateBoundary>
  }

  if (!workspace) {
    return <StateBoundary state={state} onRetry={() => void loadWorkspace()}>
      <div className="image-text-empty">
        <CircleAlert size={28}/>
        <h2>暂时无法打开图文工作台</h2>
        <p>{notice || '正在读取任务数据…'}</p>
        <button className="secondary-button" onClick={() => void loadWorkspace()}>
          <RefreshCw size={15}/>重新加载
        </button>
      </div>
    </StateBoundary>
  }

  const draftReady = workspace.draft.contract_version === 'creative-image-text-draft/v2'

  return <StateBoundary state={state}>
    <div className="image-text-v2-workspace">
      <header className="image-text-v2-header">
        <div>
          <span className="section-label">图文创作 · 小红书 3:4</span>
          <h2>{workspace.draft.selected_title || workspace.task.direction.focus || '从策略生成图文成品'}</h2>
          <p>{workspace.direction.concept} · {workspace.task.direction.audience}</p>
        </div>
        <div className="image-text-v2-readiness">
          <span className={workspace.readiness.image_generation_ready ? 'status success' : 'status warning'}>
            {workspace.readiness.image_generation_ready ? <Check size={14}/> : <CircleAlert size={14}/>}
            {workspace.readiness.image_generation_ready ? '可生成素材' : '尚未满足生成条件'}
          </span>
          <small>任务 v{workspace.task.version} · 草稿 v{workspace.draft.version}</small>
        </div>
      </header>

      {!draftReady ? <section className="image-text-v2-start">
        <Sparkles size={24}/>
        <div>
          <h3>先把策略翻译成可执行的图文方案</h3>
          <p>Creative 会同时读取通用策略、任务级策略和已确认的 Creative Direction，生成文案与封面 / 证据 / CTA 三个图片槽位。</p>
        </div>
        <button
          className="primary-button"
          disabled={!workspace.readiness.draft_generation_ready || busyAction === 'draft'}
          onClick={() => void generateDraft()}
        >
          <WandSparkles size={15}/>{busyAction === 'draft' ? '正在生成…' : '生成图文方案'}
        </button>
      </section> : null}

      {draftReady ? <button
        className="secondary-button"
        disabled={!workspace.readiness.draft_generation_ready || busyAction === 'draft'}
        onClick={() => void generateDraft()}
      >
        <WandSparkles size={15}/>{busyAction === 'draft' ? '正在重新生成…' : '重新生成图文方案'}
      </button> : null}

      {workspace.readiness.blocking_reasons.length > 0 ? <div className="image-text-v2-blockers">
        <CircleAlert size={16}/>
        <span>{workspace.readiness.blocking_reasons.map(reason => blockerLabels[reason] ?? reason).join('；')}</span>
      </div> : null}

      <div className="image-text-v2-grid">
        <aside className="image-text-v2-slots">
          {workspace.slots.map(slot => {
            const plan = workspace.draft.image_plan.find(item => item.order === slot.order)
            const latest = slot.attempts.at(-1)
            return <button
              key={slot.order}
              className={selectedOrder === slot.order ? 'active' : ''}
              onClick={() => setSelectedOrder(slot.order)}
            >
              <span>0{slot.order}</span>
              <div>
                <b>{roleLabels[slot.role]}</b>
                <small>{plan?.purpose || '等待生成图文方案'}</small>
                <em>{latest ? statusLabels[latest.status] : '尚未生成'}</em>
              </div>
            </button>
          })}
        </aside>

        <main className="image-text-v2-preview">
          {previewURL ? <img src={previewURL} alt={selectedPlan?.purpose || `第 ${selectedOrder} 张图`}/> : <div>
            <ImageIcon size={36}/>
            <b>{selectedPlan?.visual_brief || '该槽位还没有成品'}</b>
            <small>{selectedPlan?.overlay_copy || '生成图文方案后，可以逐张生成素材。'}</small>
          </div>}
        </main>

        <aside className="image-text-v2-inspector">
          <span className="section-label">{selectedSlot ? roleLabels[selectedSlot.role] : '图片槽位'}</span>
          {draftReady ? <label>图片文案
            <textarea
              value={editedOverlays[selectedOrder] ?? ''}
              maxLength={120}
              onChange={event => setEditedOverlays(value => ({
                ...value,
                [selectedOrder]: event.target.value,
              }))}
            />
          </label> : <h3>等待创作方案</h3>}
          {draftReady ? <label>画面说明
            <textarea
              value={editedVisualBriefs[selectedOrder] ?? ''}
              maxLength={2000}
              onChange={event => setEditedVisualBriefs(value => ({
                ...value,
                [selectedOrder]: event.target.value,
              }))}
            />
          </label> : null}
          {draftReady ? <label>配图说明
            <textarea
              value={editedCaptions[selectedOrder] ?? ''}
              maxLength={120}
              onChange={event => setEditedCaptions(value => ({
                ...value,
                [selectedOrder]: event.target.value,
              }))}
            />
          </label> : null}
          <p>{selectedPlan?.visual_brief || workspace.direction.creative_rationale}</p>
          <div className="image-text-v2-copy">
            <label>主标题
              <input value={editedTitle} maxLength={80} onChange={event => setEditedTitle(event.target.value)}/>
            </label>
            <label>正文
              <textarea value={editedBody} maxLength={5000} onChange={event => setEditedBody(event.target.value)}/>
            </label>
            <small>{workspace.draft.topics.join(' ')}</small>
          </div>
          <button
            className="secondary-button full"
            disabled={!draftReady || workspace.slots.some(slot => slot.attempts.length > 0) || busyAction === 'save-draft'}
            onClick={() => void saveDraft()}
          >{busyAction === 'save-draft' ? '保存中…' : '保存文案修改'}</button>
          <button
            className="primary-button full"
            disabled={!draftReady || !workspace.readiness.image_generation_ready || Boolean(selectedSlot?.attempts.some(attempt => activeStatuses.has(attempt.status))) || busyAction === `slot-${selectedOrder}`}
            onClick={() => void generateSlot(selectedOrder, Boolean(selectedSlot?.attempts.length))}
          >
            <WandSparkles size={15}/>
            {selectedSlot?.attempts.length ? '重新生成这一张' : '生成这一张'}
          </button>
          <div className="image-text-v2-attempts">
            {(selectedSlot?.attempts ?? []).map(attempt => <div key={attempt.id}>
              <span>第 {attempt.attempt_no} 次 · {statusLabels[attempt.status]}</span>
              {attempt.error_message ? <small>{attempt.error_message}</small> : null}
              {attempt.status === 'succeeded' && attempt.id !== selectedSlot?.adopted_attempt_id
                ? <button
                    className="secondary-button"
                    disabled={busyAction === `adopt-${attempt.id}`}
                    onClick={() => void adoptAttempt(attempt)}
                  >采用</button>
                : attempt.id === selectedSlot?.adopted_attempt_id ? <em>当前采用</em> : null}
            </div>)}
          </div>
          {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
        </aside>
      </div>
      {workspace.readiness.review_ready ? <section className="image-text-v2-delivery">
        <div>
          <span className="section-label">人工审核与交付</span>
          <h3>{creativeVersion
            ? `当前版本状态：${creativeVersion.status}`
            : '三张成品已原子物化，可以冻结版本'}</h3>
          <p>冻结后依次执行确定性检查、人工批准和交付；每一步都保留不可变版本。</p>
        </div>
        <button
          className="primary-button"
          disabled={creativeVersion?.status === 'delivered' || busyAction === 'delivery'}
          onClick={() => void advanceDelivery()}
        >{busyAction === 'delivery' ? '处理中…' : creativeVersion?.status === 'created'
          ? '执行交付检查' : creativeVersion?.status === 'checked'
            ? '人工批准版本' : creativeVersion?.status === 'approved'
              ? '生成交付包' : creativeVersion?.status === 'delivered'
                ? '已交付' : '冻结当前版本'}</button>
      </section> : null}
    </div>
  </StateBoundary>
}
