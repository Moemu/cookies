import type { CurrentIdentity } from '../features/platform/types'
import './account.css'

export type AccountPageView = 'profile' | 'security' | 'preferences'

type AccountPageProps = {
  identity: CurrentIdentity | null
  view: AccountPageView
}

const pageCopy: Record<AccountPageView, { title: string; description: string }> = {
  profile: {
    title: '个人资料',
    description: '查看当前登录身份和所属组织。资料编辑能力将在 Identity 服务提供对应接口后接入。',
  },
  security: {
    title: '安全摘要',
    description: '查看当前会话的身份范围；登录、退出和会话管理由 Identity 服务统一负责。',
  },
  preferences: {
    title: '偏好设置',
    description: '当前版本尚未提供可持久化的个人偏好接口，因此不会展示无法保存的表单。',
  },
}

export function AccountPage({ identity, view }: AccountPageProps) {
  const copy = pageCopy[view]
  const userName = identity?.user?.display_name || identity?.actor.principal.id || '本地用户'
  const organizationName = identity?.organization.name || '本地组织'

  return <section className="account-page">
    <header className="account-page__header">
      <p className="account-page__eyebrow">账户中心</p>
      <h1>{copy.title}</h1>
      <p>{copy.description}</p>
    </header>

    {view === 'profile' ? <dl className="account-page__details">
      <div><dt>显示名称</dt><dd>{userName}</dd></div>
      <div><dt>用户 ID</dt><dd>{identity?.user?.id || identity?.actor.principal.id || '未加载'}</dd></div>
      <div><dt>所属组织</dt><dd>{organizationName}</dd></div>
      <div><dt>组织角色</dt><dd>{identity?.membership?.role || '未加载'}</dd></div>
    </dl> : null}

    {view === 'security' ? <section className="account-page__notice">
      <h2>当前访问范围</h2>
      <p>{identity?.actor.scopes?.length ? identity.actor.scopes.join(' · ') : '身份信息加载后显示访问范围。'}</p>
    </section> : null}

    {view === 'preferences' ? <section className="account-page__notice">
      <h2>尚未配置偏好</h2>
      <p>语言、通知和界面偏好需要由 Identity 服务提供保存接口后再启用。</p>
    </section> : null}
  </section>
}
