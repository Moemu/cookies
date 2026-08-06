import { ChevronDown, Plus } from 'lucide-react'
import type { AINativeAdWorkspaceSummary } from './types'

function statusLabel(item: AINativeAdWorkspaceSummary) {
  switch (item.production_status) {
    case 'completed': return '已完成'
    case 'running':
    case 'rendering': return '视频生成中'
    case 'failed':
    case 'render_failed': return '生成失败'
    case 'cancelled': return '已取消'
  }
  if (item.storyboard_status === 'confirmed') return '故事板已确认'
  if (item.storyboard_status === 'generating') return '故事板生成中'
  if (item.storyboard_status === 'draft') return '故事板待确认'
  if (item.script_status === 'confirmed') return '脚本已确认'
  if (item.script_status === 'generating') return '脚本生成中'
  if (item.script_status === 'draft') return '脚本待确认'
  return item.status === 'confirmed' ? '需求已确认' : '需求编辑中'
}

export function workspaceDisplayName(item: Pick<AINativeAdWorkspaceSummary, 'display_name' | 'product_name'>) {
  return item.display_name.trim() || `${item.product_name || '未命名广告'}（未命名）`
}

export function AINativeAdCatalog({ records, currentId, busy, onSelect, onNew }: {
  records: AINativeAdWorkspaceSummary[]
  currentId: string
  busy: boolean
  onSelect: (workspaceId: string) => void
  onNew: () => void
}) {
  return <div className="ai-native-record-toolbar">
    <label>
      <span>当前广告</span>
      <span className="ai-native-record-select">
        <select aria-label="切换广告记录" value={currentId} disabled={busy || records.length === 0} onChange={event => onSelect(event.target.value)}>
          {records.length === 0 ? <option value="">尚未创建广告</option> : null}
          {!currentId && records.length > 0 ? <option value="">新广告（尚未分析）</option> : null}
          {records.map(item => <option key={item.workspace_id} value={item.workspace_id}>{workspaceDisplayName(item)} · {statusLabel(item)}</option>)}
        </select>
        <ChevronDown size={14}/>
      </span>
    </label>
    <button type="button" className="secondary-button" disabled={busy} onClick={onNew}><Plus size={14}/>新建广告</button>
  </div>
}
