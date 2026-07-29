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

export function creativePreRollPath(projectId: string) {
  return `/projects/${encodeURIComponent(projectId)}/creative/video/performance/pre-roll`
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

// 素材洞察的一级导航按 docs/19-module-navigation-architecture.md §5 固定为 11 个入口、
// 五个分组。已经能用的入口标 built，其余先占位，点进去说明将来做什么，不放假数据。
export type InsightSection =
  | 'prelaunch'
  | 'performance'
  | 'assets'
  | 'content'
  | 'experiments'
  | 'experiences'
  | 'reports'
  | 'data-sources'
  | 'data-quality'
  | 'operations'
  | 'settings'

export type InsightIcon = 'target' | 'chart' | 'image' | 'pen' | 'package' | 'database' | 'list' | 'send' | 'check' | 'user' | 'settings'

/** 二级视图；built 为假的视图渲染「尚未开放」说明页，不放任何编造的数据。 */
export type InsightViewEntry = { key: string; label: string; built: boolean }

export type InsightEntry = {
  key: InsightSection
  label: string
  icon: InsightIcon
  /** 只要有一个二级视图能用，这个一级入口就算能用。 */
  built: boolean
  views: ReadonlyArray<InsightViewEntry>
  purpose: string
}

function view(key: string, label: string, built = false): InsightViewEntry {
  return { key, label, built }
}

const insightSectionSource: ReadonlyArray<{ label: string; entries: Omit<InsightEntry, 'built'>[] }> = [
  {
    label: '工作',
    entries: [
      {
        key: 'prelaunch', label: '投前洞察', icon: 'target',
        views: [
          view('strategy-evidence', '策略证据', true),
          view('creative-suggestions', '创意建议'),
          view('patterns', '历史模式'),
          view('risks', '风险与反例'),
          view('references', '引用记录', true),
        ],
        purpose: '投放开始前，把已确认的经验按当前项目的情况摆出来供引用。',
      },
      {
        key: 'performance', label: '投后分析', icon: 'chart',
        views: [
          view('metrics', '指标总览', true),
          view('asset-comparison', '素材对比'),
          view('cohort', 'Cohort'),
          view('trends', '趋势'),
          view('fatigue', '疲劳'),
          view('anomalies', '异常'),
        ],
        purpose: '投放结束后，看投放执行留下的指标证据。',
      },
    ],
  },
  {
    label: '数据',
    entries: [
      {
        key: 'data-sources', label: '数据接入', icon: 'send',
        views: [
          view('sources', '数据源'), view('imports', '导入任务'), view('asset-mapping', '素材映射'),
          view('field-mapping', '字段映射'), view('sync-logs', '同步记录'),
        ],
        purpose: '接入外部平台的投放数据，并说明每个字段从哪来。',
      },
    ],
  },
  {
    label: '素材与分析',
    entries: [
      {
        key: 'assets', label: '分析素材库', icon: 'image',
        views: [
          view('all', '全部素材'), view('unmatched', '待匹配'), view('pending-extraction', '待提取'),
          view('features', '特征库'), view('versions', '版本关系'),
        ],
        purpose: '把投出去的素材和它的效果数据对上号，并沉淀可比较的内容特征。',
      },
      {
        key: 'content', label: '内容分析', icon: 'pen',
        views: [
          view('xiaohongshu', '小红书'), view('wechat', '公众号'), view('brand', '品牌广告'),
          view('digital-human', '数字人'), view('pre-roll', '广告前贴'), view('viral', '爆款复刻'),
          view('breakdown', '单素材拆解'),
        ],
        purpose: '按内容形态拆解素材的结构、文本、视觉与转化表现。',
      },
      {
        key: 'experiments', label: '实验中心', icon: 'package',
        views: [
          view('list', '实验列表'), view('variants', 'A/B 变体'), view('matrix', '变量矩阵'),
          view('sample-check', '样本检查'), view('conclusions', '实验结论'),
        ],
        purpose: '用受控实验验证结论，而不是靠一次投放的观察下判断。',
      },
    ],
  },
  {
    label: '经验与输出',
    entries: [
      {
        key: 'experiences', label: '经验库', icon: 'database',
        views: [
          view('pending', '待确认', true), view('confirmed', '已确认', true), view('needs-review', '待复审', true),
          view('retired', '已失效', true), view('references', '引用记录', true),
        ],
        purpose: '经验的生命周期：谁确认的、条件是什么、被谁用过、什么时候失效。',
      },
      {
        key: 'reports', label: '报告中心', icon: 'list',
        views: [
          view('task-review', '任务复盘', true), view('periodic', '周期报告'),
          view('custom', '自定义报告'), view('exports', '导出记录'),
        ],
        purpose: '复盘报告的撰写、确认与沉淀成经验的入口。',
      },
    ],
  },
  {
    label: '治理',
    entries: [
      {
        key: 'data-quality', label: '数据质量', icon: 'check',
        views: [
          view('freshness', '数据新鲜度'), view('missing', '缺失'), view('anomalies', '异常'),
          view('definitions', '口径'), view('reconciliation', '对账'), view('repair-queue', '修复队列'),
        ],
        purpose: '在拿数据下结论之前，先说清楚这批数据能不能信。',
      },
      {
        key: 'operations', label: '能力运营', icon: 'user',
        views: [
          view('feature-system', '特征体系'), view('metric-dictionary', '指标字典'), view('skills', '分析 Skills'),
          view('evaluation-sets', '评测集'), view('quality-board', '质量看板'),
        ],
        purpose: '维护分析口径本身：指标怎么定义、分析能力怎么评测。',
      },
      {
        key: 'settings', label: '系统设置', icon: 'settings',
        views: [
          view('sample-threshold', '样本门槛'), view('observation-window', '观察窗口'), view('notifications', '通知'),
          view('confirm-permissions', '确认权限'), view('report-templates', '报告模板'),
        ],
        purpose: '设定多少样本才算数、观察多久、谁有权确认经验。',
      },
    ],
  },
]

export const insightGroups: ReadonlyArray<{ label: string; entries: ReadonlyArray<InsightEntry> }> =
  insightSectionSource.map((group) => ({
    label: group.label,
    entries: group.entries.map((entry) => ({ ...entry, built: entry.views.some((item) => item.built) })),
  }))

export const insightEntries: ReadonlyArray<InsightEntry> = insightGroups.flatMap((group) => group.entries)

export function insightEntry(section: string) {
  return insightEntries.find((entry) => entry.key === section)
}

export function insightPath(projectId: string, section: InsightSection = 'prelaunch') {
  return `/projects/${encodeURIComponent(projectId)}/insight/${section}`
}

// 二级视图也走 URL：刷新、分享链接、侧栏高亮都停在同一个页签上。
export function insightViewPath(projectId: string, section: InsightSection, view?: string) {
  const key = view || insightEntry(section)?.views[0]?.key || ''
  return `${insightPath(projectId, section)}/${key}`
}

export type InsightExperienceView = 'pending' | 'confirmed' | 'needs-review' | 'retired' | 'references'

export function insightExperiencePath(projectId: string, view: InsightExperienceView = 'pending') {
  return insightViewPath(projectId, 'experiences', view)
}

export function routeProjectId(pathname: string) {
  return pathname.match(/^\/projects\/([^/]+)/)?.[1]
    || pathname.match(/^\/strategy\/projects\/([^/]+)/)?.[1]
    || ''
}

// 模块只看路径里 /projects/:projectId 之后的第一段。
// 不能用 includes 判断：二级视图的名字里也会出现 strategy、creative 这些词，
// 例如 /insight/prelaunch/strategy-evidence 会被误判成需求与策略。
function moduleSegment(pathname: string) {
  const rest = pathname.startsWith('/projects/') ? pathname.replace(/^\/projects\/[^/]*/, '') : pathname
  return rest.split('/').filter(Boolean)[0] || ''
}

export function activeBusinessModule(pathname: string): BusinessModule {
  if (pathname.startsWith('/admin') || pathname.startsWith('/account')) return 'admin'
  switch (moduleSegment(pathname)) {
    case 'strategy': return 'strategy'
    case 'insight': return 'insights'
    case 'delivery': return 'delivery'
    case 'creative': case 'assets': case 'provider-jobs': return 'creative'
    default: return 'projects'
  }
}

export function destinationForProject(pathname: string, projectId: string) {
  switch (moduleSegment(pathname)) {
    case 'strategy': return strategyPath(projectId)
    case 'creative': return creativeTasksPath(projectId)
    case 'insight': return insightPath(projectId)
    case 'delivery': return deliveryPath(projectId)
    case 'provider-jobs': return providerJobsPath(projectId)
    case 'assets': return assetsPath(projectId)
    case 'manage': return projectManagePath(projectId)
    default: return projectHomePath(projectId)
  }
}
