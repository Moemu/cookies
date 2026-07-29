export type ProjectIndustry = 'short_drama' | 'game' | 'ecommerce' | 'automotive_brand'

export type IndustryProfile = {
  label: string
  shortLabel: string
  strategy: { fields: string[]; format: string }
  creative: { fields: string[]; format: string }
  insight: { fields: string[]; format: string }
  delivery: { fields: string[]; format: string }
}

// Navigation remains universal. These schemas are presentation contracts for
// each Project, so a module can vary without branching the navigation tree.
export const industryProfiles: Record<ProjectIndustry, IndustryProfile> = {
  short_drama: {
    label: '短剧', shortLabel: '短剧',
    strategy: { fields: ['题材与人群', '前 3 秒冲突', '追更转化'], format: '剧情钩子卡 + 集数承接表' },
    creative: { fields: ['角色关系', '反转节点', '追更 CTA'], format: '分集脚本 + 前贴分镜' },
    insight: { fields: ['完播率', '追更率', '角色热度'], format: '集数漏斗 + 剧情节点热力图' },
    delivery: { fields: ['剧集包', '冷启人群', '追更回传'], format: '分集投放矩阵' },
  },
  game: {
    label: '游戏', shortLabel: '游戏',
    strategy: { fields: ['品类赛道', '核心玩法', '预约/下载目标'], format: '卖点优先级 + 人群分层' },
    creative: { fields: ['实机画面', '高光时刻', '下载 CTA'], format: '玩法镜头清单 + 版本脚本' },
    insight: { fields: ['CTR', '下载率', '次日留存'], format: '素材 × 渠道留存矩阵' },
    delivery: { fields: ['素材包', '转化事件', '归因窗口'], format: '渠道 × 出价策略表' },
  },
  ecommerce: {
    label: '电商', shortLabel: '电商',
    strategy: { fields: ['货品机会', '价格权益', '成交目标'], format: '人货场机会表' },
    creative: { fields: ['商品卖点', '使用场景', '下单 CTA'], format: '商品卡 + 短视频脚本' },
    insight: { fields: ['点击率', '加购率', '成交 ROI'], format: '货品 × 素材效率看板' },
    delivery: { fields: ['商品库', '预算', 'ROI 目标'], format: '货品分层投放计划' },
  },
  automotive_brand: {
    label: '品牌（汽车）', shortLabel: '汽车',
    strategy: { fields: ['车型定位', '核心场景', '试驾线索'], format: '场景 × 人群价值主张图' },
    creative: { fields: ['车型卖点', '道路场景', '试驾 CTA'], format: '车型影片分镜 + KV 组合' },
    insight: { fields: ['有效观看', '试驾留资', '品牌搜索'], format: '场景素材 × 线索质量矩阵' },
    delivery: { fields: ['车型素材包', '区域门店', '试驾回传'], format: '区域 × 车型投放矩阵' },
  },
}

export function industryProfile(industry?: string): IndustryProfile {
  return industryProfiles[industry as ProjectIndustry] ?? industryProfiles.ecommerce
}
