import type { CSSProperties, ReactNode } from 'react'

export function StrategyWorkspaceShell({ assistant, assistantExpanded = false, assistantOpen, assistantWidth = 356, children, rail, topbar }: {
  assistant: ReactNode
  assistantExpanded?: boolean
  assistantOpen: boolean
  assistantWidth?: number
  children: ReactNode
  rail: ReactNode
  topbar: ReactNode
}) {
  const style = { '--strategy-assistant-width': `${assistantWidth}px` } as CSSProperties
  return <section
    className="strategy-v2-shell"
    data-assistant-expanded={assistantOpen && assistantExpanded ? 'true' : 'false'}
    data-assistant-open={assistantOpen ? 'true' : 'false'}
    style={style}
  >
    {topbar}
    {rail}
    <div className="strategy-v2-body">
      <div className="strategy-v2-stage-region">{children}</div>
      {assistantOpen ? assistant : null}
    </div>
  </section>
}
