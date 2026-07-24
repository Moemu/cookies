export type ModuleKey = 'strategy' | 'creative' | 'insights' | 'delivery' | 'admin'

export type ShellModule = {
  key: ModuleKey
  label: string
  description: string
  icon: 'target' | 'pen' | 'chart' | 'send' | 'settings'
}

export const shellModules: readonly ShellModule[] = [
  { key: 'strategy', label: '策略', description: '需求与策略系统将在此挂载。', icon: 'target' },
  { key: 'creative', label: '创意', description: '创意创作与视频编辑系统将在此挂载。', icon: 'pen' },
]

export const adminModule: ShellModule = {
  key: 'admin',
  label: '管理',
  description: '共享平台管理能力将在此挂载。',
  icon: 'settings',
}
