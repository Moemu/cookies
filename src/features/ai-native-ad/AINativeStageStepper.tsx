import { Check, LoaderCircle } from 'lucide-react'
import type { AINativeStageId, AINativeStageStatus } from './types'

const stages: Array<{ id: AINativeStageId; order: string; label: string; detail: string }> = [
  { id: 'requirement', order: '01', label: '需求分析', detail: '商品与生成要求' },
  { id: 'script', order: '02', label: '脚本生成', detail: '单个完整营销脚本' },
  { id: 'storyboard', order: '03', label: '故事板生成', detail: '素材与详细分镜' },
  { id: 'video', order: '04', label: '视频生成', detail: '渲染进度与成片' },
]

export function AINativeStageStepper({ active, status, onSelect }: {
  active: AINativeStageId
  status: Record<AINativeStageId, AINativeStageStatus>
  onSelect: (stage: AINativeStageId) => void
}) {
  return <div className="ai-native-steps" role="tablist" aria-label="AI 效果广告生成步骤">
    {stages.map(stage => {
      const currentStatus = status[stage.id]
      const className = [active === stage.id ? 'active' : '', currentStatus === 'confirmed' ? 'done' : '', currentStatus === 'invalidated' ? 'invalidated' : ''].filter(Boolean).join(' ')
      return <button
        key={stage.id}
        id={`ai-native-stage-${stage.id}`}
        type="button"
        role="tab"
        aria-selected={active === stage.id}
        aria-controls={`ai-native-panel-${stage.id}`}
        className={className}
        onClick={() => onSelect(stage.id)}
      >
        <span>{currentStatus === 'confirmed' ? <Check size={14}/> : currentStatus === 'generating' ? <LoaderCircle className="spin" size={14}/> : stage.order}</span>
        <span><b>{stage.label}</b><small>{currentStatus === 'invalidated' ? '上游已修改，等待重新生成' : stage.detail}</small></span>
      </button>
    })}
  </div>
}
