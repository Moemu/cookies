export interface ContentAnalysisCase {
  id: string
  title: string
  category: string
  summary: string
  status: string
  tags: string[]
  metrics: Array<{ label: string; value: string; note: string }>
  dimensions: Array<{ label: string; insight: string; signal: string }>
  timeline: Array<{ time: string; title: string; detail: string }>
  experiments: Array<{ id: string; name: string; detail: string }>
  risks: string[]
  recommendation: string
}

export const contentAnalysisCases: ContentAnalysisCase[] = [
  {
    id: 'taobao-flash-sale',
    title: '淘宝闪购 · 窑鸡拉新短视频',
    category: '外卖拉新 / 信息流',
    summary: '用高食欲特写建立前三秒停留，再以 25.4 元到 3.4 元的订单反差承接新客无门槛红包。',
    status: '案例拆解 · 人工复核待完成',
    tags: ['高食欲钩子', '价格反差', '新人红包', '外卖拉新'],
    metrics: [
      { label: '首屏钩子', value: '0–3 秒', note: '食欲生理刺激' },
      { label: '价格锚点', value: '¥3.4', note: '对比原价 ¥25.4' },
      { label: '权益表达', value: '0 门槛', note: '新人最高 28 元红包' },
      { label: '信息密度', value: '高', note: '价格、费用、CTA 同屏' },
    ],
    dimensions: [
      { label: '视觉与镜头', insight: '第一帧以撕开流油窑鸡、咬下爆汁的近景建立感官刺激；焦香鸡皮、嫩肉与辣椒面共同强化食欲。', signal: '高强度停留钩子' },
      { label: '前三秒留存', insight: '先用可感知的吃播画面留住用户，再进入订单与优惠解释，避免硬广开场直接劝退。', signal: '内容先于广告' },
      { label: '价格与权益', insight: '原价 25.4 元、实付 3.4 元形成明确锚点；配送费和打包费为 0，降低领取与下单的心理成本。', signal: '强优惠反差' },
      { label: '用户痛点', insight: '直击外卖贵、满减复杂、配送费和打包费不透明等普遍抱怨，强调“不用凑单”的确定性。', signal: '全民级痛点' },
      { label: '文案与原生感', insight: '“谁还原价点外卖”使用口语化对话，将平台权益包装为用户主动发现的省钱机会。', signal: '弱硬广感' },
      { label: '转化链路', insight: '食欲画面唤起需求 → 低价订单证明价值 → 新人红包解释资格 → 领券/下单 CTA 收口。', signal: '链路完整' },
      { label: '合规与校验', insight: '价格、红包上限、费用减免与适用门店需按实际活动地域、时段和规则展示，避免绝对化承诺。', signal: '需事实校验' },
    ],
    timeline: [
      { time: '00:00–00:03', title: '食欲钩子', detail: '窑鸡撕开与爆汁咬下特写，先唤起即时进食需求。' },
      { time: '00:03–00:07', title: '订单反差', detail: '展示原价、实付价与免配送/打包费，让优惠可被快速验证。' },
      { time: '00:07–00:11', title: '权益解释', detail: '明确新人无门槛红包，消除满减凑单与规则复杂的顾虑。' },
      { time: '00:11–00:15', title: '行动收口', detail: '以领券或立即点餐 CTA 完成从食欲到下单的转化闭环。' },
    ],
    experiments: [
      { id: 'A1', name: '首帧食欲强度', detail: '对比撕鸡爆汁特写与完整成品俯拍，验证首屏停留差异。' },
      { id: 'A2', name: '价格证据位置', detail: '对比第 3 秒与第 5 秒展示实付价，验证利益点前置程度。' },
      { id: 'A3', name: '权益话术', detail: '对比“无门槛红包”与“新人立减”，验证规则理解与点击意愿。' },
      { id: 'A4', name: '费用信息', detail: '对比同时展示免配送/打包费与仅展示实付价，验证信任增益。' },
    ],
    risks: ['活动价格与红包额度需基于实时规则校验。', '不得以“全网最低”“所有用户可用”等绝对化表述替代适用条件。', '食物画面、商家商品和订单信息需具备使用授权。'],
    recommendation: '首轮只测试“首帧食欲强度”与“实付价出现时机”两项变量；保持商品、地区、人群与权益规则一致，先验证前三秒留存和点击率。',
  },
]

export const manhuaMix = [
  { name: '动态漫', supply: 14, spend: 38.13, signal: '供给少、消耗贡献高' },
  { name: '仿真人', supply: 16, spend: 24.7, signal: '可补充验证' },
  { name: '实拍口播', supply: 42, spend: 22.1, signal: '供给偏多' },
  { name: '剧情混剪', supply: 28, spend: 15.07, signal: '需收敛重复' },
]

export const manhuaMethods = [
  { id: 'M1', name: '动态漫强冲突开场', detail: '只替换前 3 秒冲突钩子，保持题材与人群一致。' },
  { id: 'M2', name: '仿真人身份悬念', detail: '将人物关系与反转前置，验证信息缺口是否提升完播。' },
  { id: 'M3', name: '实拍口播证据化', detail: '用关键证据替代泛化口播，降低首屏流失。' },
  { id: 'M4', name: '剧情混剪节奏压缩', detail: '缩短铺垫，保留单一转折与明确行动点。' },
]
