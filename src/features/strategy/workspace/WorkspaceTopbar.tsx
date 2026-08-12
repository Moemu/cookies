import Activity from 'lucide-react/dist/esm/icons/activity.js'
import Bot from 'lucide-react/dist/esm/icons/bot.js'
import LockKeyhole from 'lucide-react/dist/esm/icons/lock-keyhole.js'
import RefreshCw from 'lucide-react/dist/esm/icons/refresh-cw.js'
import type { Workspace } from '../types'

export function WorkspaceTopbar({
  activeWorkspaceId,
  activityDisabled = false,
  assistantDisabled = false,
  assistantOpen,
  assistantUnread = false,
  backgroundTaskCount,
  busy,
  onAssistantToggle,
  onOpenActivity,
  onRefresh,
  onWorkspaceChange,
  workspaceName,
  workspaces,
}: {
  activeWorkspaceId: string
  activityDisabled?: boolean
  assistantDisabled?: boolean
  assistantOpen: boolean
  assistantUnread?: boolean
  backgroundTaskCount: number
  busy: boolean
  onAssistantToggle: () => void
  onOpenActivity: () => void
  onRefresh: () => void
  onWorkspaceChange: (workspaceId: string) => void
  workspaceName: string
  workspaces: Workspace[]
}) {
  return <header className="strategy-workspace-topbar">
    <div className="strategy-workspace-identity">
      <span>当前工作链</span>
      <strong>{workspaceName}</strong>
      <small><LockKeyhole size={13}/>锁定到当前 Project</small>
    </div>
    <label className="strategy-workspace-switcher">
      <span>切换工作区</span>
      <select
        aria-label="切换策略工作区"
        value={activeWorkspaceId}
        onChange={event => onWorkspaceChange(event.target.value)}
      >
        {workspaces.map(workspace => <option key={workspace.id} value={workspace.id}>
          {workspace.name}{workspace.is_primary ? ' · 主工作区' : ''}
        </option>)}
      </select>
    </label>
    <div className="strategy-workspace-tools">
      <button
        type="button"
        className="strategy-activity-trigger"
        aria-label={backgroundTaskCount > 0 ? `后台任务，${backgroundTaskCount} 个正在运行` : '后台任务'}
        data-running={backgroundTaskCount > 0 ? 'true' : 'false'}
        disabled={activityDisabled}
        onClick={onOpenActivity}
      >
        <Activity size={15}/>
        <span>{backgroundTaskCount > 0 ? `${backgroundTaskCount} 个后台任务` : '后台任务'}</span>
      </button>
      <button
        type="button"
        className="strategy-assistant-trigger"
        aria-label={assistantUnread ? 'AI 助手，有未读更新' : 'AI 助手'}
        aria-expanded={assistantOpen}
        data-unread={assistantUnread ? 'true' : 'false'}
        disabled={assistantDisabled}
        onClick={onAssistantToggle}
      >
        <Bot size={16}/><span>AI 助手</span>{assistantUnread ? <i aria-hidden="true"/> : null}
      </button>
      <button type="button" className="strategy-icon-button" aria-label="刷新当前策略工作区" disabled={busy} onClick={onRefresh}>
        <RefreshCw className={busy ? 'spin' : undefined} size={16}/>
      </button>
    </div>
  </header>
}
