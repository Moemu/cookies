import type { ProjectRecord } from '../types'

const artifacts = {
  brief: { key: 'brief', label: 'Brief', version: 'v1.3', status: '已确认', owner: 'Amelia Meng', updatedAt: '2026-07-22 09:12', summary: '新品上市与高质量销售线索增长' },
  strategy: { key: 'strategy', label: '策略', version: 'v2.1', status: '已确认', owner: 'Amelia Meng', updatedAt: '2026-07-22 10:24', summary: '精度证据前置，覆盖采购与研发双受众', sourceVersion: 'Brief v1.3' },
  creative: { key: 'creative', label: '创意', version: 'v1.8', status: '制作中', owner: 'Lin Wei', updatedAt: '2026-07-22 13:48', summary: '图文 4 版，视频 3 版，包装检查 8/10', sourceVersion: '策略 v2.1' },
  insight: { key: 'insight', label: '洞察', version: 'v1.4', status: '已确认', owner: 'Sofia Chen', updatedAt: '2026-07-22 15:06', summary: '证据前置版本点击率较基线提升 18%', sourceVersion: '创意 v1.8' },
  delivery: { key: 'delivery', label: '投放', version: 'v0.9', status: '待审批', owner: 'Noah Xu', updatedAt: '2026-07-22 16:30', summary: '计划预算 ¥1,000,000，待审批 ChangeSet 1 项', sourceVersion: '洞察 v1.4' },
} as const

export const initialProjects: ProjectRecord[] = [
  {
    id: 'prj-2607-01', code: 'SP', name: '春季新品上市', brand: '白域精工', product: '高精度 CNC 加工零部件', goal: '新品上市与销售线索增长', stage: '投放审批', progress: 82, status: '进行中', updatedAt: '2026-07-22 16:30', budget: 1000000, currency: 'CNY', timezone: 'Asia/Shanghai',
    artifacts: structuredClone(artifacts),
    changeSets: [{ id: 'CS-2607-018', title: '合并重复广告并补充 6 条新素材', status: '待审批', risk: '中', budgetImpact: 8600, createdAt: '2026-07-22 16:30', createdBy: 'Noah Xu', version: 3, evidenceIds: ['EV-021', 'EV-024', 'INS-014'], rollbackPlan: '恢复 PLAN-2607-06 v0.8，并重新启用原广告组。', changes: [{ field: '重复广告', before: '12 组', after: '保留 5 组有效组合' }, { field: '新素材覆盖', before: '18%', after: '30% 目标' }, { field: '探索预算', before: '¥0', after: '¥8,600' }] }],
  },
  {
    id: 'prj-2607-02', code: 'HG', name: '华东行业增长计划', brand: '白域精工', product: '精密制造解决方案', goal: '重点行业客户拓展', stage: '素材洞察', progress: 64, status: '进行中', updatedAt: '2026-07-21 18:40', budget: 680000, currency: 'CNY', timezone: 'Asia/Shanghai', artifacts: structuredClone(artifacts), changeSets: [],
  },
  {
    id: 'prj-2606-03', code: 'BU', name: '精密制造品牌升级', brand: '白域精工', product: '品牌整合传播', goal: '品牌认知与可信证据建设', stage: '已完成', progress: 100, status: '已完成', updatedAt: '2026-07-18 11:20', budget: 420000, currency: 'CNY', timezone: 'Asia/Shanghai', artifacts: structuredClone(artifacts), changeSets: [],
  },
]

export const projectEvidence = [
  { id: 'EV-021', title: '精密制造行业受众研究', source: '项目研究库', confidence: '高', date: '2026-07-20' },
  { id: 'EV-024', title: '白域精工近 90 天素材表现', source: '广告数据 Connector', confidence: '高', date: '2026-07-22' },
  { id: 'INS-014', title: '证据前置创意实验结论', source: '洞察与经验', confidence: '中', date: '2026-07-22' },
  { id: 'EV-027', title: '10 位采购与研发负责人访谈', source: '飞书文档', confidence: '中', date: '2026-07-19' },
]

export const unifiedRecords = [
  { id: 'BRF-2607-11', name: '春季新品上市 Brief', kind: 'Brief', status: '已确认', owner: 'Amelia Meng', updatedAt: '2026-07-22 09:12' },
  { id: 'STR-2607-08', name: '精度证据增长策略', kind: '策略', status: '已确认', owner: 'Amelia Meng', updatedAt: '2026-07-22 10:24' },
  { id: 'CR-2607-42', name: '精密制造图文与视频', kind: '创意', status: '制作中', owner: 'Lin Wei', updatedAt: '2026-07-22 13:48' },
  { id: 'INS-2607-14', name: '证据前置实验分析', kind: '洞察', status: '待确认', owner: 'Sofia Chen', updatedAt: '2026-07-22 15:06' },
  { id: 'PLAN-2607-06', name: '销售线索增长计划', kind: '投放', status: '待审批', owner: 'Noah Xu', updatedAt: '2026-07-22 16:30' },
  { id: 'CS-2607-018', name: '素材组合优化 ChangeSet', kind: '变更', status: '待审批', owner: 'Noah Xu', updatedAt: '2026-07-22 16:30' },
  { id: 'EV-2607-24', name: '近 90 天素材表现证据', kind: '证据', status: '已确认', owner: 'Sofia Chen', updatedAt: '2026-07-22 14:52' },
  { id: 'VER-2607-19', name: '图文创意 v1.8', kind: '版本', status: '制作中', owner: 'Lin Wei', updatedAt: '2026-07-22 13:48' },
]
