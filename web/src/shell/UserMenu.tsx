import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { logout } from '../auth/api'
import type { CurrentIdentity } from '../features/platform/types'

export function UserMenu({ identity }: { identity: CurrentIdentity | null }) {
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const displayName = identity?.user?.display_name || identity?.actor.principal.id || '本地用户'
  const initial = displayName.slice(0, 1).toUpperCase()
  const canManageOrganization = identity?.membership?.role === 'owner'

  return <div className="user-menu">
    <button
      aria-expanded={open}
      aria-haspopup="menu"
      aria-label={`打开 ${displayName} 的用户菜单`}
      className="user-menu__trigger"
      onClick={() => setOpen((current) => !current)}
      type="button"
    >
      <span className="avatar" aria-hidden="true">{initial}</span>
      <span className="user-menu__name">{displayName}</span>
    </button>
    {open ? <div aria-label="用户菜单" className="user-menu__panel" role="menu">
      <div className="user-menu__identity">
        <strong>{displayName}</strong>
        <span>{identity?.organization.name || '本地组织'}</span>
      </div>
      <Link onClick={() => setOpen(false)} role="menuitem" to="/account/profile">个人资料</Link>
      <Link onClick={() => setOpen(false)} role="menuitem" to="/account/security">安全摘要</Link>
      <Link onClick={() => setOpen(false)} role="menuitem" to="/account/preferences">偏好设置</Link>
      {canManageOrganization ? <Link onClick={() => setOpen(false)} role="menuitem" to="/admin">组织与访问</Link> : null}
      <button className="user-menu__logout" onClick={() => {
        setOpen(false)
        logout().finally(() => navigate('/login', { replace: true }))
      }} role="menuitem" type="button">退出登录</button>
    </div> : null}
  </div>
}
