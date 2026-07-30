import { useEffect, useState } from 'react'
import {
  api,
  type ApiCreativeTaskHandoffDetail,
  type ApiTaskStrategyCreativeIntake,
} from '../../data/api'

export type TaskStrategyPerformanceMode =
  | 'short-drama'
  | 'pre-roll'
  | 'viral-remake'

const performanceModeByBusiness: Readonly<Record<string, TaskStrategyPerformanceMode>> = {
  short_drama_preroll: 'short-drama',
  commerce_preroll: 'pre-roll',
  viral_remake: 'viral-remake',
}

export function taskStrategyPerformanceMode(
  businessCode: string,
): TaskStrategyPerformanceMode | undefined {
  return performanceModeByBusiness[businessCode]
}

export function useTaskStrategyCreativeIntake(
  projectId: string,
  intakeId: string | undefined,
  enabled: boolean,
): ApiTaskStrategyCreativeIntake | null {
  const [intake, setIntake] = useState<ApiTaskStrategyCreativeIntake | null>(null)
  const shouldLoad = enabled && Boolean(intakeId)

  useEffect(() => {
    if (!shouldLoad || !intakeId) {
      setIntake(null)
      return
    }
    let active = true
    setIntake(null)
    void api.getTaskStrategyCreativeIntake(projectId, intakeId)
      .then(value => {
        if (active && value.source === 'task_strategy') setIntake(value)
      })
      .catch(() => {
        if (active) setIntake(null)
      })
    return () => {
      active = false
    }
  }, [intakeId, projectId, shouldLoad])

  return intake
}

export function useTaskStrategyTaskHandoffDetail(
  projectId: string,
  taskId: string | undefined,
): ApiCreativeTaskHandoffDetail | null {
  const [detail, setDetail] = useState<ApiCreativeTaskHandoffDetail | null>(null)

  useEffect(() => {
    if (!taskId) {
      setDetail(null)
      return
    }
    let active = true
    setDetail(null)
    void api.getCreativeTaskHandoffDetail(projectId, taskId)
      .then(value => {
        if (active && value.intake.source === 'task_strategy') setDetail(value)
      })
      .catch(() => {
        if (active) setDetail(null)
      })
    return () => {
      active = false
    }
  }, [projectId, taskId])

  return detail
}

export function TaskStrategyHandoffBanner({
  intake,
}: {
  intake: ApiTaskStrategyCreativeIntake
}) {
  const strategy = intake.request.task_strategy_input
  const pendingConfirmation = strategy.open_questions.slice(0, 3).join('、')
    || '素材、渠道参数与最终生成指令'

  return <div className="task-strategy-handoff-banner" role="status">
    <div>
      <span>已继承创意任务策略</span>
      <b>{intake.request.concept || intake.request.core_message}</b>
      <small>{intake.request.audience} · {strategy.business_code}</small>
    </div>
    <div>
      <small>生产前仍需确认</small>
      <span>{pendingConfirmation}</span>
    </div>
    {strategy.reference_use.locator
      ? <em>参考内容：{strategy.reference_use.rights_status === 'unknown'
        ? '仅可做抽象分析，生产复用权利待确认'
        : strategy.reference_use.rights_status}</em>
      : null}
  </div>
}
