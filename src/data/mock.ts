export const workItems = [
  { name: '春季新品上市', type: '策略工作区', owner: 'Amelia Meng', status: '待评审', progress: 82, updated: '2026-07-22 16:30' },
  { name: '精密制造品牌片', type: '创意任务', owner: 'Lin Wei', status: '生成中', progress: 68, updated: '2026-07-22 13:48' },
  { name: '证据前置实验分析', type: '分析任务', owner: 'Sofia Chen', status: '需处理', progress: 86, updated: '2026-07-22 15:06' },
  { name: '销售线索增长计划 06', type: '投放计划', owner: 'Noah Xu', status: '待审批', progress: 82, updated: '2026-07-22 16:30' },
  { name: '华东行业受众研究', type: '研究任务', owner: 'Amelia Meng', status: '已完成', progress: 100, updated: '2026-07-21 18:40' },
]

export const evidence = [
  { id: '01', title: '精密制造行业受众研究', source: '项目研究库', confidence: '高', date: '2026-07-20' },
  { id: '02', title: '白域精工近 90 天素材表现', source: '广告数据 Connector', confidence: '高', date: '2026-07-22' },
  { id: '03', title: '证据前置创意实验结论', source: '洞察与经验', confidence: '中', date: '2026-07-22' },
  { id: '04', title: '采购与研发负责人访谈（10 份）', source: '飞书文档', confidence: '中', date: '2026-07-19' },
]

export const activity = [
  { time: '10:24', title: '策略 v1.2 已提交评审', meta: 'Amelia Meng · 引用 5 条证据' },
  { time: '09:48', title: '创意方向 CR-103 已确认', meta: 'Lin Wei · 进入视频制作' },
  { time: '09:12', title: '投放计划完成预算校验', meta: '系统 · 无阻断风险' },
  { time: '昨天', title: '洞察 INS-208 被策略引用', meta: 'Q2 素材疲劳研究' },
]

export const chartPoints = [32, 39, 36, 49, 54, 51, 63, 67, 72, 78, 75, 86]

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
