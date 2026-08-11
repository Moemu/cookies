import Check from 'lucide-react/dist/esm/icons/check.js'
import type { StrategyStage } from './workspaceRoute'

export type StageProgress = 'complete' | 'current' | 'available' | 'blocked'

export type StageRailItem = {
  stage: StrategyStage
  status: StageProgress
  detail: string
}

const stageLabels: Readonly<Record<StrategyStage, { index: string; label: string }>> = {
  intake: { index: '01', label: '理解需求' },
  brief: { index: '02', label: 'Brief' },
  strategy: { index: '03', label: '策略' },
  review: { index: '04', label: '确认 / 评审' },
  handoff: { index: '05', label: '创意交接' },
}

export function strategyStageLabel(stage: StrategyStage): string {
  return stageLabels[stage].label
}

export function StageRail({ activeStage, items, onSelect }: {
  activeStage: StrategyStage
  items: StageRailItem[]
  onSelect: (stage: StrategyStage) => void
}) {
  return <nav className="strategy-stage-rail" aria-label="策略工作阶段">
    <ol>
      {items.map(item => {
        const label = stageLabels[item.stage]
        const active = item.stage === activeStage
        return <li key={item.stage} data-status={item.status}>
          <button
            type="button"
            aria-current={active ? 'step' : undefined}
            aria-label={`${label.label}：${item.detail}`}
            onClick={() => onSelect(item.stage)}
          >
            <span className="strategy-stage-index" aria-hidden="true">
              {item.status === 'complete' ? <Check size={14}/> : label.index}
            </span>
            <span className="strategy-stage-copy">
              <b>{label.label}</b>
              <small>{item.detail}</small>
            </span>
          </button>
        </li>
      })}
    </ol>
  </nav>
}
