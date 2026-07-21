export const workItems = [
  { name: '春季新品上市', type: '策略工作区', owner: 'Amelia Meng', status: '待评审', progress: 72, updated: '12 分钟前' },
  { name: '精密制造品牌片', type: '创意任务', owner: 'Lin Wei', status: '生成中', progress: 58, updated: '31 分钟前' },
  { name: 'Q2 素材疲劳研究', type: '分析任务', owner: 'Sofia Chen', status: '需处理', progress: 83, updated: '1 小时前' },
  { name: '线索增长计划 06', type: '投放计划', owner: 'Noah Xu', status: '执行中', progress: 46, updated: '2 小时前' },
  { name: '华东行业受众研究', type: '研究任务', owner: 'Amelia Meng', status: '已完成', progress: 100, updated: '昨天' },
]

export const evidence = [
  { id: '01', title: '行业报告：精密制造市场洞察 2025', source: '艾瑞咨询', confidence: '高', date: '2025-04-28' },
  { id: '02', title: 'B2B 采购行为调研 2025', source: '项目研究库', confidence: '中', date: '2025-05-06' },
  { id: '03', title: '竞品网站与信息分析', source: '网页研究任务', confidence: '高', date: '2025-05-11' },
  { id: '04', title: '客户访谈摘要（10 份）', source: '飞书文档', confidence: '中', date: '2025-05-12' },
]

export const activity = [
  { time: '10:24', title: '策略 v1.2 已提交评审', meta: 'Amelia Meng · 引用 5 条证据' },
  { time: '09:48', title: '创意方向 CR-103 已确认', meta: 'Lin Wei · 进入视频制作' },
  { time: '09:12', title: '投放计划完成预算校验', meta: '系统 · 无阻断风险' },
  { time: '昨天', title: '洞察 INS-208 被策略引用', meta: 'Q2 素材疲劳研究' },
]

export const chartPoints = [32, 39, 36, 49, 54, 51, 63, 67, 72, 78, 75, 86]

export const records = [
  ['WP-2406-18', '春季新品上市', '等待人工确认', 'Amelia Meng', '12 分钟前'],
  ['CR-2406-42', '精密制造品牌片', '生成中 · 6/10', 'Lin Wei', '31 分钟前'],
  ['INS-2406-08', 'Q2 素材疲劳研究', '需处理 · 2 项', 'Sofia Chen', '1 小时前'],
  ['PLAN-2406-03', '线索增长计划 06', '执行中', 'Noah Xu', '2 小时前'],
  ['BRF-2406-11', '华东行业受众研究', '已完成', 'Amelia Meng', '昨天'],
]

export const manhuaMix = [
  { name: '动态漫与仿真人', supply: 14, spend: 38.13, signal: '优先补充验证' },
  { name: '表情包与沙雕漫', supply: 48, spend: 32.8, signal: '提升差异化' },
  { name: '小说漫改与有声书', supply: 36, spend: 25.78, signal: '稳定基础供给' },
  { name: '3D 漫剧', supply: 0, spend: 2.49, signal: '小规模探索' },
]

export const manhuaMethods = [
  { id: 'M1', name: '高光剧情切片', detail: '最低成本验证剧情与角色' },
  { id: 'M3', name: '5–12 秒超短素材', detail: '快速验证钩子与 CTA' },
  { id: 'M4', name: '文字旁白 + BGM', detail: '重构叙事和悬念密度' },
  { id: 'M5', name: '解说 + 原剧情', detail: '兼顾信息密度与沉浸' },
]

export const deliveryDiagnostics = [
  { id: '01', name: '重复组合', value: '12 组', tone: 'danger', detail: '同一商品、素材和转化目标下存在重复广告' },
  { id: '02', name: '新素材覆盖', value: '18%', tone: 'warning', detail: '低于本项目 25% 的探索目标' },
  { id: '03', name: '无消耗广告', value: '7 条', tone: 'warning', detail: '已连续 72 小时无有效消耗' },
]

export const deliveryActions = [
  { priority: 'P0', name: '补充 6 条新素材', detail: '优先改变核心内容，避免只替换边框或文案', impact: '+12–18% 探索覆盖' },
  { priority: 'P1', name: '合并重复广告', detail: '保留有效广告，清理 7 条长期无消耗对象', impact: '减少预算内耗' },
  { priority: 'P2', name: '建立浅层转化实验', detail: '5–10% 预算仅作为来源建议，提交前需人工确认', impact: '扩大探索空间' },
]
