import { KanonStrategyWorkspace } from '../KanonStrategyWorkspace'
import { WorkspaceProvider } from './WorkspaceProvider'
import type { StrategyWorkspaceLocation } from './workspaceRoute'
import '../styles/tokens.css'
import '../styles/shell.css'
import '../styles/research.css'
import '../styles/documents.css'

export function StrategyWorkspaceRoute({
  location,
  onNavigate,
  onOpenCreative,
  projectId,
  workspaceId,
}: {
  location: StrategyWorkspaceLocation
  onNavigate: (workspaceId: string, location: StrategyWorkspaceLocation, replace?: boolean) => void
  onOpenCreative: (navId: string, view: string, contextId: string) => void
  projectId: string
  workspaceId?: string
}) {
  return <WorkspaceProvider
    location={location}
    onNavigate={onNavigate}
    projectId={projectId}
    workspaceId={workspaceId}
  >
    <KanonStrategyWorkspace onOpenCreative={onOpenCreative}/>
  </WorkspaceProvider>
}
