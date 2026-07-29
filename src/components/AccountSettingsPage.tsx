import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { Building2, KeyRound, Save, ShieldCheck, UserPlus, UsersRound } from 'lucide-react'
import { accountApi, type BackendOrganizationAccess, type BackendOrganizationMember } from '../backend/platform'
import { useAuth } from '../context/AuthContext'

type AccountPage = 'profile' | 'security' | 'organization' | 'organization-members'

export function AccountSettingsPage({ page }: { page: AccountPage }) {
  const { session, refresh, switchOrganization } = useAuth()
  const [organizations, setOrganizations] = useState<BackendOrganizationAccess[]>([])
  const [members, setMembers] = useState<BackendOrganizationMember[]>([])
  const [displayName, setDisplayName] = useState(session.user?.displayName ?? '')
  const [newUserId, setNewUserId] = useState('')
  const [newRole, setNewRole] = useState('member')
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const organizationId = session.organization?.id ?? ''
  const canManageMembers = session.scopes?.includes('organization.members.manage') ?? false
  const currentOrganizationRole = session.membership?.role ?? ''
  const canGrantOwner = currentOrganizationRole === 'owner'

  useEffect(() => {
    setDisplayName(session.user?.displayName ?? '')
  }, [session.user?.displayName])

  useEffect(() => {
    if (page === 'organization') {
      void accountApi.listOrganizations().then(result => setOrganizations(result.items)).catch(showError(setError))
    }
    if (page === 'organization-members' && organizationId) {
      void reloadMembers()
    }
  }, [page, organizationId])

  const reloadMembers = async () => {
    try {
      setError('')
      setMembers((await accountApi.listOrganizationMembers(organizationId)).items)
    } catch (cause) {
      setError(errorText(cause))
    }
  }

  const saveProfile = async (event: FormEvent) => {
    event.preventDefault()
    setBusy(true)
    setError('')
    setNotice('')
    try {
      await accountApi.updateProfile(displayName)
      await refresh()
      setNotice('个人资料已更新。')
    } catch (cause) {
      setError(errorText(cause))
    } finally {
      setBusy(false)
    }
  }

  const addMember = async (event: FormEvent) => {
    event.preventDefault()
    setBusy(true)
    setError('')
    setNotice('')
    try {
      await accountApi.addOrganizationMember(organizationId, newUserId.trim(), newRole)
      setNewUserId('')
      setNotice('已有用户已加入当前组织。')
      await reloadMembers()
    } catch (cause) {
      setError(errorText(cause))
    } finally {
      setBusy(false)
    }
  }

  const updateMember = async (member: BackendOrganizationMember, role: string, status: string) => {
    setBusy(true)
    setError('')
    setNotice('')
    try {
      await accountApi.updateOrganizationMember(organizationId, member.user.id, {
        role,
        status,
        expected_updated_at: member.membership.updated_at,
      })
      setNotice('成员权限已更新。')
      if (member.user.id === session.user?.id) {
        await refresh()
        return
      }
      await reloadMembers()
    } catch (cause) {
      setError(errorText(cause))
    } finally {
      setBusy(false)
    }
  }

  const changeOrganization = async (nextOrganizationId: string) => {
    setBusy(true)
    setError('')
    setNotice('')
    try {
      await switchOrganization(nextOrganizationId)
    } catch (cause) {
      setError(errorText(cause))
      setBusy(false)
    }
  }

  if (page === 'profile') return <SettingsFrame icon={<ShieldCheck/>} eyebrow="ACCOUNT" title="个人资料" description="管理全局用户身份；组织角色和项目权限由管理员单独维护。">
    <form className="account-form" onSubmit={saveProfile}>
      <label>用户 ID<input value={session.user?.id ?? ''} disabled/></label>
      <label>显示名称<input value={displayName} onChange={event => setDisplayName(event.target.value)} maxLength={80} required/></label>
      <button className="primary-button" disabled={busy || !displayName.trim()}><Save size={15}/>{busy ? '保存中…' : '保存资料'}</button>
    </form>
    <Feedback notice={notice} error={error}/>
  </SettingsFrame>

  if (page === 'security') return <SettingsFrame icon={<KeyRound/>} eyebrow="SECURITY" title="安全与登录" description="Cookies 使用服务端 opaque session，浏览器脚本无法读取认证 token。">
    <dl className="account-facts">
      <div><dt>认证方式</dt><dd>平台密码会话（当前环境）</dd></div>
      <div><dt>Cookie</dt><dd>HttpOnly · SameSite=Strict</dd></div>
      <div><dt>当前作用域</dt><dd>{session.organization?.name} · {session.membership?.role}</dd></div>
      <div><dt>权限数量</dt><dd>{session.scopes?.length ?? 0}</dd></div>
    </dl>
  </SettingsFrame>

  if (page === 'organization') return <SettingsFrame icon={<Building2/>} eyebrow="ORGANIZATION" title="组织管理" description="每个会话只绑定一个组织；切换时会轮换会话并清空项目缓存。">
    <div className="organization-list">
      {organizations.map(item => <article key={item.organization.id} className={item.organization.id === organizationId ? 'active' : ''}>
        <div><b>{item.organization.name}</b><small>{item.organization.id} · {item.membership.role}</small></div>
        {item.organization.id === organizationId ? <span>当前组织</span> : <button className="secondary-button" disabled={busy} onClick={() => void changeOrganization(item.organization.id)}>切换</button>}
      </article>)}
    </div>
    <Feedback notice={notice} error={error}/>
  </SettingsFrame>

  return <SettingsFrame icon={<UsersRound/>} eyebrow="ORGANIZATION MEMBERS" title="组织成员" description="组织角色控制组织级管理能力；项目访问仍需要显式项目成员关系。">
    {canManageMembers ? <form className="member-add-form" onSubmit={addMember}>
      <label>已有用户 ID<input value={newUserId} onChange={event => setNewUserId(event.target.value)} placeholder="usr_…" required/></label>
      <label>组织角色<select value={newRole} onChange={event => setNewRole(event.target.value)}><option value="member">member</option><option value="auditor">auditor</option><option value="admin">admin</option>{canGrantOwner ? <option value="owner">owner</option> : null}</select></label>
      <button className="primary-button" disabled={busy || !newUserId.trim()}><UserPlus size={15}/>添加已有用户</button>
    </form> : null}
    <div className="member-table">
      {members.map(member => {
        const adminCannotManage = currentOrganizationRole === 'admin' &&
          (member.membership.role === 'owner' || member.user.id === session.user?.id)
        const canEditMember = canManageMembers && !adminCannotManage
        const roleOptions = canGrantOwner || member.membership.role === 'owner'
          ? ['owner', 'admin', 'member', 'auditor']
          : ['admin', 'member', 'auditor']
        return <article key={member.user.id}>
          <div><b>{member.user.display_name}</b><small>{member.user.id}</small></div>
          <select aria-label={`${member.user.display_name}的组织角色`} value={member.membership.role} disabled={!canEditMember || busy} onChange={event => void updateMember(member, event.target.value, member.membership.status)}>
            {roleOptions.map(role => <option key={role}>{role}</option>)}
          </select>
          <button className="secondary-button" disabled={!canEditMember || busy} onClick={() => void updateMember(member, member.membership.role, member.membership.status === 'active' ? 'suspended' : 'active')}>{member.membership.status === 'active' ? '停用' : '启用'}</button>
          <span className={`member-status ${member.membership.status}`}>{member.membership.status}</span>
        </article>
      })}
      {!members.length && !error ? <div className="panel-empty">当前组织暂无可显示成员。</div> : null}
    </div>
    <Feedback notice={notice} error={error}/>
  </SettingsFrame>
}

function SettingsFrame({ icon, eyebrow, title, description, children }: { icon: ReactNode; eyebrow: string; title: string; description: string; children: ReactNode }) {
  return <main className="account-settings-page" id="main-content"><header><span>{icon}</span><div><small>{eyebrow}</small><h1>{title}</h1><p>{description}</p></div></header>{children}</main>
}

function Feedback({ notice, error }: { notice: string; error: string }) {
  return <>{error ? <div className="config-notice error" role="alert">{error}</div> : null}{notice ? <div className="inline-notice" role="status">{notice}</div> : null}</>
}

function errorText(cause: unknown) {
  return cause instanceof Error ? cause.message : '请求失败，请稍后重试。'
}

function showError(setter: (value: string) => void) {
  return (cause: unknown) => setter(errorText(cause))
}
