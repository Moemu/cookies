import type { ReactNode } from 'react'

// 证据抽屉。任何一条结论都必须能一键翻出它背后的数字——「相信我」不是证据。
export function EvidenceDrawer({ title, open, onClose, children }: {
  title: string
  open: boolean
  onClose: () => void
  children: ReactNode
}) {
  if (!open) return null
  return (
    <aside className="evidence-drawer" role="dialog" aria-label={title}>
      <header className="evidence-drawer-head">
        <h3>{title}</h3>
        <button type="button" onClick={onClose} aria-label="关闭">×</button>
      </header>
      <div className="evidence-drawer-body">{children}</div>
    </aside>
  )
}
