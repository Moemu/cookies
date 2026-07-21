import {
  Activity, Aperture, Archive, BadgeCheck, BarChart3, BookOpenCheck, Bot,
  Boxes, BrainCircuit, ChartNoAxesCombined, CircleGauge, ClipboardCheck,
  Database, FileCheck2, FileSearch, Film, FlaskConical, FolderKanban,
  GalleryHorizontalEnd, Gauge, LayoutDashboard, Library, ListChecks,
  Megaphone, MonitorCog, PackageCheck, PanelTop, PlaySquare, Rocket,
  SearchCheck, Send, Settings2, ShieldCheck, SlidersHorizontal, Sparkles,
  TableProperties, Target, TrendingUp, UsersRound, Video, WandSparkles,
} from 'lucide-react'
import type { SystemDefinition } from '../types'

export const systems: SystemDefinition[] = [
  {
    key: 'strategy', label: '需求与策略', shortLabel: '策略', icon: BrainCircuit,
    statement: '把模糊需求转化为可追溯、可执行的广告策略。',
    nav: [
      { id: 'home', label: '策略工作台', icon: LayoutDashboard, group: '工作', layout: 'dashboard', description: '汇总待办、工作区进度、近期产物与质量状态。', views: ['我的待办', '工作区进度', '最近产物', '质量与用量'] },
      { id: 'workspaces', label: '策略工作区', icon: FolderKanban, group: '工作', layout: 'workspace', description: '在对话、Brief、研究、策略、实验与评审之间保持同一上下文。', views: ['概览', '对话', 'Brief', '研究', '策略', '实验', '评审', '变更记录'] },
      { id: 'briefs', label: '需求中心', icon: ClipboardCheck, group: '资产与方法', layout: 'table', description: '管理 Brief 完整度、来源、冲突、确认与版本。', views: ['Brief 列表', '待补充', '待确认', '版本库', '冲突队列'] },
      { id: 'strategies', label: '策略中心', icon: Target, group: '资产与方法', layout: 'analysis', description: '沉淀方向、受众、主张、渠道预算与实验方案。', views: ['策略库', '渠道策略', '方案对比', '实验方案', '版本库'] },
      { id: 'research', label: '研究洞察', icon: FileSearch, group: '资产与方法', layout: 'analysis', description: '组织受众、竞品、行业研究和可引用证据。', views: ['受众', '竞品', '行业', '资料来源', '研究任务'] },
      { id: 'reviews', label: '评审中心', icon: BadgeCheck, group: '协作', layout: 'table', description: '集中处理待评审内容、评论、审批与变更。', views: ['待我评审', '我发起的', '评论与提及', '已完成', '变更记录'] },
      { id: 'operations', label: '能力运营', icon: Bot, group: '治理', layout: 'operations', description: '治理领域 Skills、模板、字段规则和评测集。', views: ['领域 Skills', '行业模板', '字段规则', '评测集', '质量看板'] },
      { id: 'settings', label: '系统设置', icon: Settings2, group: '治理', layout: 'settings', description: '配置字段、状态、通知和导出规范。', views: ['字段配置', '状态规则', '通知', '导出模板'] },
    ],
  },
  {
    key: 'creative', label: '创意创作', shortLabel: '创意', icon: WandSparkles,
    statement: '把批准策略转化为可评审、可交付的图文与视频作品。',
    nav: [
      { id: 'home', label: '创意工作台', icon: LayoutDashboard, group: '工作', layout: 'dashboard', description: '聚合生产进度、异常、待评审、交付和用量。', views: ['我的任务', '生产进度', '待评审', '最近交付', '用量与质量'] },
      { id: 'tasks', label: '创意任务', icon: ListChecks, group: '工作', layout: 'table', description: '串联策略来源、制作、变体、检查、评审与交付。', views: ['全部', '进行中', '等待输入', '生成中', '待评审', '已完成', '失败', '归档'] },
      { id: 'image-text', label: '图文创作', icon: GalleryHorizontalEnd, group: '创作', layout: 'editor', description: '完成文案、封面、图组、排版、素材和渠道检查。', views: ['小红书', '公众号', '草稿箱', '图文版本'] },
      { id: 'video', label: '视频创作', icon: Video, group: '创作', layout: 'editor', description: '覆盖品牌广告、效果广告和多轨素材剪辑。', views: ['品牌广告制作', '效果广告制作', '素材剪辑', '视频草稿', '视频版本'] },
      { id: 'production', label: '制作中心', icon: Aperture, group: '创作', layout: 'operations', description: '管理图片、视频、音频生成与渲染队列。', views: ['图片生成', '视频生成', '音频生成', '渲染队列', '源素材', '失败任务'] },
      { id: 'reviews', label: '评审中心', icon: BadgeCheck, group: '协作与输出', layout: 'workspace', description: '完成品牌、事实、版权、合规与逐帧评审。', views: ['待我评审', '我发起的', '评论与提及', '退回记录', '已完成'] },
      { id: 'deliveries', label: '交付中心', icon: PackageCheck, group: '协作与输出', layout: 'table', description: '生成稳定发布包、投放包和授权清单。', views: ['待交付', '发布包', '投放包', '下载记录', '停用版本'] },
      { id: 'operations', label: '创意运营', icon: Sparkles, group: '治理', layout: 'operations', description: '治理渠道规则、Skills、模板和品牌规则映射。', views: ['渠道规则', '领域 Skills', '模板', '品牌规则映射', '评测集', '质量看板'] },
      { id: 'settings', label: '系统设置', icon: Settings2, group: '治理', layout: 'settings', description: '配置默认规格、命名、通知、评审与交付规则。', views: ['默认规格', '命名规则', '通知', '评审流', '交付规则'] },
    ],
  },
  {
    key: 'insight', label: '素材洞察', shortLabel: '洞察', icon: ChartNoAxesCombined,
    statement: '连接内容特征与真实效果，形成有边界的可复用经验。',
    nav: [
      { id: 'home', label: '洞察工作台', icon: LayoutDashboard, group: '工作', layout: 'dashboard', description: '优先呈现数据新鲜度、关键机会、疲劳和异常。', views: ['关键机会', '近期洞察', '疲劳预警', '待办', '数据状态'] },
      { id: 'connections', label: '数据接入', icon: Database, group: '数据', layout: 'operations', description: '管理平台数据源、字段映射、素材映射和同步。', views: ['数据源', '导入任务', '字段映射', '素材映射', '同步记录'] },
      { id: 'assets', label: '分析素材库', icon: Library, group: '分析', layout: 'table', description: '建立可分析素材索引、版本、特征与血缘。', views: ['全部素材', '待匹配', '待提取', '特征', '版本与血缘'] },
      { id: 'content', label: '内容分析', icon: Film, group: '分析', layout: 'analysis', description: '分析图文和视频的文本、视觉、音频与节奏。', views: ['漫剧大盘', '制作方法', '包装检查', '图文', '品牌广告'] },
      { id: 'performance', label: '效果分析', icon: TrendingUp, group: '分析', layout: 'analysis', description: '对比素材表现、趋势、疲劳、异常和驱动因素。', views: ['指标总览', '素材对比', 'Cohort', '趋势', '疲劳', '异常', '驱动因素'] },
      { id: 'experiments', label: '实验中心', icon: FlaskConical, group: '分析', layout: 'analysis', description: '管理 A/B 变量、样本检查和可归因结果。', views: ['实验列表', '变量矩阵', '样本检查', '实验结果'] },
      { id: 'knowledge', label: '洞察与经验', icon: BookOpenCheck, group: '沉淀', layout: 'workspace', description: '沉淀结论、适用条件、反例、复审与引用。', views: ['候选洞察', '已确认', '待复审', '已失效', '引用记录'] },
      { id: 'reports', label: '报告中心', icon: PanelTop, group: '沉淀', layout: 'editor', description: '组织任务复盘、周期报告和可引用数据。', views: ['任务复盘', '周期报告', '自定义报告', '协作', '版本与导出'] },
      { id: 'quality', label: '数据质量', icon: ShieldCheck, group: '治理', layout: 'operations', description: '监控新鲜度、缺失、口径、异常和修复队列。', views: ['新鲜度', '缺失', '异常', '口径', '对账', '修复队列'] },
      { id: 'operations', label: '能力运营', icon: SlidersHorizontal, group: '治理', layout: 'operations', description: '治理特征体系、指标字典、Skills 与评测集。', views: ['特征体系', '指标字典', '分析 Skills', '评测集', '版本与质量'] },
      { id: 'settings', label: '系统设置', icon: Settings2, group: '治理', layout: 'settings', description: '配置样本、窗口、通知、确认权限与报告模板。', views: ['样本门槛', '观察窗口', '通知', '确认权限', '报告模板'] },
    ],
  },
  {
    key: 'delivery', label: '智能投放', shortLabel: '投放', icon: Rocket,
    statement: '把批准策略和创意转化为安全、可审计的投放动作。',
    nav: [
      { id: 'home', label: '投放作战台', icon: Gauge, group: '工作', layout: 'dashboard', description: '聚合预算、效果、账户健康、异常、审批和执行。', views: ['今日态势', '预算进度', '核心效果', '异常待办', '最近执行'] },
      { id: 'plans', label: '投放计划', icon: Megaphone, group: '计划与执行', layout: 'workspace', description: '配置目标、预算、受众、版位、创意和校验。', views: ['全部计划', '草稿', '待审批', '执行中', '已完成', '版本'] },
      { id: 'execution', label: '执行中心', icon: PlaySquare, group: '计划与执行', layout: 'operations', description: '管理受控执行、等待用户、接管、恢复和验证。', views: ['待执行', '执行中', '等待用户', '结果未知', '失败', '接管', '完成'] },
      { id: 'monitoring', label: '监控告警', icon: Activity, group: '监控与优化', layout: 'analysis', description: '监控预算、效果、平台、拒审、追踪和素材疲劳。', views: ['需行动', '预算', '效果', '平台状态', '拒审', '追踪', '素材疲劳'] },
      { id: 'optimization', label: '优化中心', icon: TrendingUp, group: '监控与优化', layout: 'analysis', description: '评估建议、预计影响、ChangeSet 和观察结果。', views: ['待处理建议', '已采纳', '观察中', '已拒绝', '效果跟踪'] },
      { id: 'accounts', label: '账户与环境', icon: UsersRound, group: '资源', layout: 'table', description: '管理广告账户、平台资产、权限和执行环境。', views: ['广告账户', '平台资产', '权限', '登录状态', '执行设备'] },
      { id: 'approvals', label: '审批中心', icon: FileCheck2, group: '治理', layout: 'workspace', description: '审查预算、上线、暂停、扩量和紧急动作。', views: ['待我审批', '我发起的', '预算', '上线', '暂停与扩量', '已完成'] },
      { id: 'evidence', label: '证据与审计', icon: Archive, group: '治理', layout: 'operations', description: '保存执行时间线、截图、结构化日志和前后差异。', views: ['执行时间线', '页面截图', '结构化日志', '前后差异', '导出'] },
      { id: 'platform', label: '平台能力运营', icon: MonitorCog, group: '治理', layout: 'operations', description: '治理平台 Skills、适配、回归、版本与故障开关。', views: ['能力矩阵', '页面适配', '回归测试', '版本发布', '故障开关'] },
      { id: 'settings', label: '系统设置', icon: Settings2, group: '治理', layout: 'settings', description: '配置预算护栏、风险等级、通知与监控频率。', views: ['预算护栏', '风险等级', '通知', '监控频率', '命名规则'] },
    ],
  },
]

export const quickActions = [
  { label: '新建策略工作区', detail: '从需求对话开始', system: 'strategy' as const },
  { label: '创建创意任务', detail: '基于已批准策略', system: 'creative' as const },
  { label: '分析一组素材', detail: '连接内容与效果', system: 'insight' as const },
  { label: '创建投放计划', detail: '先校验再执行', system: 'delivery' as const },
]
