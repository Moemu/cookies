export type BusinessModule = 'projects' | 'strategy' | 'creative' | 'insights' | 'delivery' | 'admin'

export type CreativeStage =
  | 'direction'
  | 'content'
  | 'production'
  | 'check'
  | 'review'
  | 'delivery'

export const creativeStages: ReadonlyArray<{
  key: CreativeStage
  label: string
  description: string
}> = [
  { key: 'direction', label: '方向', description: '确认内容类型、受众与表达角度' },
  { key: 'content', label: '内容', description: '编辑标题、正文、话题与画面计划' },
  { key: 'production', label: '生产', description: '调用模型并将图片沉淀为项目素材' },
  { key: 'check', label: '检查', description: '冻结版本并执行交付前检查' },
  { key: 'review', label: '评审', description: '审批通过可追溯的创意版本' },
  { key: 'delivery', label: '交付', description: '生成不可变 CreativePackage' },
]

export function projectHomePath(projectId: string) {
  return `/projects/${encodeURIComponent(projectId)}/home`
}

export function projectManagePath(projectId: string) {
  return `/projects/${encodeURIComponent(projectId)}/manage`
}

export function strategyPath(projectId: string) {
  return `/projects/${encodeURIComponent(projectId)}/strategy/workspaces`
}

export function creativeTasksPath(projectId: string) {
  return `/projects/${encodeURIComponent(projectId)}/creative/tasks`
}

export function creativeTaskPath(projectId: string, taskId: string, stage: CreativeStage = 'content') {
  return `${creativeTasksPath(projectId)}/${encodeURIComponent(taskId)}/${stage}`
}

export function assetsPath(projectId: string) {
  return `/projects/${encodeURIComponent(projectId)}/assets`
}

export function providerJobsPath(projectId: string) {
  return `/projects/${encodeURIComponent(projectId)}/provider-jobs`
}

export function deliveryPath(projectId: string, view: 'plans' | 'monitoring' | 'accounts' | 'optimization' = 'plans') {
  return `/projects/${encodeURIComponent(projectId)}/delivery/${view}`
}

export function deliveryPlanPath(projectId: string, planId: string) {
  return `${deliveryPath(projectId)}/${encodeURIComponent(planId)}`
}

export function insightsPath(projectId: string, view: 'prelaunch' | 'performance' | 'reports' | 'experiences' = 'prelaunch') {
  return `/projects/${encodeURIComponent(projectId)}/insights/${view}`
}

export function routeProjectId(pathname: string) {
  return pathname.match(/^\/projects\/([^/]+)/)?.[1]
    || pathname.match(/^\/strategy\/projects\/([^/]+)/)?.[1]
    || ''
}

export function activeBusinessModule(pathname: string): BusinessModule {
  if (pathname.startsWith('/admin') || pathname.startsWith('/account')) return 'admin'
  if (pathname.includes('/strategy')) return 'strategy'
  if (pathname.includes('/insights')) return 'insights'
  if (pathname.includes('/delivery')) return 'delivery'
  if (pathname.includes('/creative') || pathname.includes('/assets') || pathname.includes('/provider-jobs')) return 'creative'
  return 'projects'
}

export function destinationForProject(pathname: string, projectId: string) {
  if (pathname.includes('/strategy')) return strategyPath(projectId)
  if (pathname.includes('/creative')) return creativeTasksPath(projectId)
  if (pathname.includes('/insights')) return insightsPath(projectId)
  if (pathname.includes('/delivery')) return deliveryPath(projectId)
  if (pathname.includes('/provider-jobs')) return providerJobsPath(projectId)
  if (pathname.includes('/assets')) return assetsPath(projectId)
  if (pathname.includes('/manage')) return projectManagePath(projectId)
  return projectHomePath(projectId)
}
