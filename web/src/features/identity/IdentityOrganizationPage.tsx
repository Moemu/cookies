import type { CurrentIdentity, Project } from '../platform/types'

const scopeLabels: Record<string, string> = {
  'project.read': '查看项目',
  'project.write': '管理项目',
  'assets.read': '查看素材',
  'assets.write': '添加素材',
}

function statusLabel(status?: string) {
  if (status === 'active') return '正常'
  if (status === 'suspended') return '已停用'
  if (status === 'archived') return '已归档'
  return status || '未知'
}

export function IdentityOrganizationPage({
  identity,
  projects,
}: {
  identity: CurrentIdentity | null
  projects: Project[]
}) {
  const actor = identity?.actor
  const activeProjects = projects.filter((project) => project.status === 'active').length

  return <section className="access-page">
    <header className="access-page__header">
      <div><h1>组织与访问</h1><p>查看当前可信身份、组织成员关系和平台授权范围。</p></div>
      <span className="access-status"><i />{identity ? '身份已验证' : '正在加载身份'}</span>
    </header>

    <div className="access-layout">
      <section className="access-primary" aria-labelledby="identity-heading">
        <div className="access-section-heading"><h2 id="identity-heading">当前身份</h2><span>{actor?.principal.kind === 'service' ? '服务身份' : '用户身份'}</span></div>
        <dl className="identity-details">
          <div><dt>显示名称</dt><dd>{identity?.user?.display_name || actor?.principal.id || '—'}</dd></div>
          <div><dt>主体 ID</dt><dd className="technical-value">{actor?.principal.id || '—'}</dd></div>
          <div><dt>成员角色</dt><dd>{identity?.membership?.role || (actor?.principal.kind === 'service' ? 'worker' : '—')}</dd></div>
          <div><dt>账号状态</dt><dd>{statusLabel(identity?.user?.status || identity?.membership?.status)}</dd></div>
        </dl>
      </section>

      <aside className="organization-rail" aria-labelledby="organization-heading">
        <div className="organization-mark" aria-hidden="true">{identity?.organization.name.slice(0, 1) || 'O'}</div>
        <h2 id="organization-heading">{identity?.organization.name || '组织信息加载中'}</h2>
        <p className="technical-value">{identity?.organization.id || '—'}</p>
        <div className="organization-facts">
          <div><span>组织状态</span><strong>{statusLabel(identity?.organization.status)}</strong></div>
          <div><span>可见项目</span><strong>{projects.length}</strong></div>
          <div><span>活跃项目</span><strong>{activeProjects}</strong></div>
        </div>
      </aside>
    </div>

    <section className="scope-section" aria-labelledby="scope-heading">
      <div className="access-section-heading"><div><h2 id="scope-heading">授权范围</h2><p>请求由后端可信身份解析并按项目二次授权。</p></div><span>{actor?.scopes.length || 0} 项权限</span></div>
      <div className="scope-list">
        {(actor?.scopes || []).map((scope) => <div className="scope-row" key={scope}><span className="scope-check" aria-hidden="true">✓</span><strong>{scopeLabels[scope] || scope}</strong><code>{scope}</code></div>)}
      </div>
    </section>
  </section>
}
