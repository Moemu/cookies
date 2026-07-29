export interface CommerceHookTemplate {
  id: string
  name: string
  category: string
  hook: string
  duration: string
  image: string
  imageLabel: string
  frameStrategy: string
  fidelity: string
  camera: string
  motion: string
  environment: string
  result: string
  guardrails: string
  tags: string[]
}

export type CommerceHookPromptCopy = Pick<
  CommerceHookTemplate,
  'fidelity' | 'camera' | 'motion' | 'environment' | 'result' | 'guardrails'
>

export const commerceHookTemplates: CommerceHookTemplate[] = [
  {
    id: 'product-cut', name: '商品切割', category: '感官冲击', duration: '5–7 秒',
    hook: '用材质反差和切割瞬间制造前三秒停留。', image: '/assets/commerce-hook-cut.jpg', imageLabel: '首帧参考', frameStrategy: '单首帧图生视频',
    fidelity: '保持商品瓶型、比例、标签位置与包装文字不变，不新增品牌信息。',
    camera: '商业摄影质感，9:16 竖版，微距特写，固定机位，柔和侧光。',
    motion: '白色陶瓷刀对准瓶盖平稳下压，动作由慢到快，切割路径清晰。',
    environment: '雪地微缩场景中的人物轻微走动，灯串稳定发光，背景不抢主体。',
    result: '切口出现乳白色质地，停留 0.8 秒展示细腻材质与商品正面。',
    guardrails: '动作连续、无卡顿；手指、刀具和瓶体不畸变；包装文字全程可辨识。', tags: ['美妆个护', 'ASMR', '节日'],
  },
  {
    id: 'window-reveal', name: '雾面橱窗揭幕', category: '仪式展示', duration: '6 秒',
    hook: '从雾面遮挡到清晰揭幕，让娇兰第三代黄金复原蜜完成一次有仪式感的商品登场。', image: '/assets/guerlain-youth-watery-oil-tail.jpg', imageLabel: '娇兰商品尾帧', frameStrategy: '首尾帧生视频',
    fidelity: '保持娇兰第三代黄金复原蜜的修长矩形瓶型、比例、金属滴管瓶盖、透明暖金色液体、悬浮金珠、蜜蜂标识及包装文字不变。',
    camera: '9:16 竖版写实商业摄影，中景，透过雾面玻璃橱窗拍摄，固定机位与构图，使用暖金色侧光。',
    motion: '一只戴浅色手套的手左右擦拭两次玻璃雾气，轨迹连续、幅度克制，全片只执行这一个主动作。',
    environment: '商品和金色蜂巢背景位置保持稳定，仅允许雾气连续消退与轻微光斑变化，背景不得争夺商品注意力。',
    result: '雾气完全消失，完整露出娇兰第三代黄金复原蜜正面，瓶型、蜜蜂标识与标签清晰，最后稳定定格 1 秒。',
    guardrails: '不得生成香水瓶或其他 SKU；不改变瓶型、颜色、金珠、瓶盖、蜜蜂标识及标签位置；不增加字幕、价格、促销信息、水印或第二件商品；手部不畸变，商品不穿模、不闪现、不漂移。', tags: ['娇兰', '护肤', '首尾帧'],
  },
  {
    id: 'one-click', name: '一键取物', category: '动作魔法', duration: '4–6 秒',
    hook: '空手到持物的明确变化，快速交代“想要即得”。', image: '/assets/commerce-hook-reveal.png', imageLabel: '场景质感参考', frameStrategy: '先尾帧、再首帧、首尾帧生视频',
    fidelity: '保持两支产品的颜色、大小、包装文字和相互比例不变。',
    camera: '平视中近景，9:16 竖版，手部位于画面中轴，背景轻微虚化。',
    motion: '首帧为空手舒展，两支产品从画面正下方匀速飞入并被同时握住。',
    environment: '夜景灯光稳定，雪花持续缓慢下落，不改变手部位置。',
    result: '产品在手中停止，正面朝向镜头，停留展示包装与颜色差异。',
    guardrails: '手指数量正确；产品不穿模、不互相遮挡；飞行速度自然且同步。', tags: ['美妆', '转场', '首尾帧'],
  },
  {
    id: 'miniature', name: '微缩功效剧场', category: '功效可视化', duration: '6–9 秒',
    hook: '把抽象功效变成微缩人物正在完成的具体任务。', image: '/assets/commerce-hook-cut.jpg', imageLabel: '微缩比例参考', frameStrategy: '首尾帧生视频',
    fidelity: '商品主体、包装文字与比例保持不变，微缩人物不得覆盖核心标签。',
    camera: '移轴微缩摄影，固定中景，浅景深，商品作为画面最大尺度主体。',
    motion: '微缩工人左右协作喷洒泡沫、刷洗并搬运工具，动作有明确分工。',
    environment: '泡沫从局部扩散后逐步消退，周边工具轻微移动，背景保持稳定。',
    result: '污渍或问题点消失，露出干净表面，以前后差异证明商品功效。',
    guardrails: '变化只发生在问题区域；人物尺度统一；动作顺滑，不闪现、不跳帧。', tags: ['家清', '功效', '微缩'],
  },
  {
    id: 'device-summon', name: '3C 设备召回', category: '场景演示', duration: '5–7 秒',
    hook: '从桌面自动召回到手中，同时点亮核心功能界面。', image: '/assets/commerce-hook-reveal.png', imageLabel: '光影风格参考', frameStrategy: '首尾帧生视频',
    fidelity: '保持设备尺寸、边框、按键、屏幕比例与品牌标识不变。',
    camera: '温暖书房中景，平视固定镜头，自然窗光与台灯共同照明。',
    motion: '手保持空握位置，桌面设备平稳浮起并被吸入手中，随后屏幕亮起。',
    environment: '书页和挂件只有轻微自然运动，桌面陈设位置保持一致。',
    result: '设备正面朝向镜头，屏幕展示核心功能界面并稳定停留。',
    guardrails: '设备不弯曲；手部与设备接触自然；屏幕 UI 不乱码、不漂移。', tags: ['3C', '学习机', '功能展示'],
  },
]

export const hookStoryboard = [
  { time: '00:00–00:01.5', name: '建立异常', detail: '先给动作起点或未完成状态，第一眼制造信息缺口。' },
  { time: '00:01.5–00:04.0', name: '完成变化', detail: '只执行一个主动作，环境运动作为辅助信号。' },
  { time: '00:04.0–00:06.0', name: '商品定格', detail: '正面、文字和结果清晰可见，为后续正片留出拼接点。' },
]

const guerlainFidelity = '保持娇兰第三代黄金复原蜜的修长矩形瓶型、比例、金属滴管瓶盖、透明暖金色液体、悬浮金珠、蜜蜂标识及包装文字不变。'
const guerlainGuardrails = '不得生成香水瓶或其他 SKU；不改变瓶型、颜色、金珠、瓶盖、蜜蜂标识及标签位置；不增加字幕、价格、促销信息、水印或第二件商品；手部不畸变，商品不穿模、不闪现、不漂移。'

export function commerceTemplateApiId(id: string) {
  return `commerce.${id}` as
    | 'commerce.product-cut'
    | 'commerce.window-reveal'
    | 'commerce.one-click'
    | 'commerce.miniature'
    | 'commerce.device-summon'
}

export function guerlainPromptCopy(templateId: string): CommerceHookPromptCopy {
  switch (templateId) {
    case 'product-cut':
      return {
        fidelity: guerlainFidelity,
        camera: '9:16 竖版写实商业微距摄影，娇兰商品固定居中，暖金色侧光突出透明瓶身和悬浮金珠。',
        motion: '刀具只切开商品旁侧的半透明蜂蜜凝胶介质，展示金色细腻截面，全程不接触、不切割商品瓶身。',
        environment: '金色蜂巢背景和商品位置保持稳定，只允许凝胶产生少量真实切面变化。',
        result: '刀具退出，娇兰第三代黄金复原蜜正面与蜂蜜凝胶截面共同清晰定格。',
        guardrails: `${guerlainGuardrails} 不得切割、击碎或打开商品包装。`,
      }
    case 'one-click':
      return {
        fidelity: guerlainFidelity,
        camera: '9:16 竖版中近景，暖金色梳妆台场景，固定机位，商品出现路径连续可读。',
        motion: '一只戴浅色手套的手完成一次按压，闭合展示位平稳打开，娇兰商品沿固定路径升起。',
        environment: '梳妆台与蜂巢装饰位置保持稳定，不生成手机界面或不存在的电子功能。',
        result: '手部离开，娇兰第三代黄金复原蜜回到画面中心，正面朝向镜头稳定定格。',
        guardrails: guerlainGuardrails,
      }
    case 'miniature':
      return {
        fidelity: guerlainFidelity,
        camera: '9:16 竖版移轴微缩摄影，商品为最大尺度主体，暖金色浅景深，微缩元素不遮挡标签。',
        motion: '微缩角色围绕瓶身完成一次连续的金色光泽与蜂蜜质感演示，只表现 Brief 已确认的修护与焕亮卖点。',
        environment: '微缩活动只发生在瓶身周边安全区域，不触碰、不覆盖商品和品牌标识。',
        result: '微缩动作停止并退居辅助位置，娇兰第三代黄金复原蜜完整正面稳定定格。',
        guardrails: `${guerlainGuardrails} 不生成医疗结果、绝对功效或 Brief 未确认的数据。`,
      }
    case 'device-summon':
      return {
        fidelity: guerlainFidelity,
        camera: '9:16 竖版中景，暖金色梳妆台与展示柜场景，固定机位，商品出现过程无遮挡。',
        motion: '梳妆台展示抽屉完成一次机械滑出，娇兰商品随托台平稳升起并转向正面。',
        environment: '只使用符合美妆品类的真实梳妆台装置，不生成电子屏幕、应用界面或虚构功能。',
        result: '装置停止，娇兰第三代黄金复原蜜完整正面朝向镜头并稳定定格。',
        guardrails: guerlainGuardrails,
      }
    default:
      return {
        fidelity: guerlainFidelity,
        camera: '9:16 竖版写实商业摄影，中景，透过雾面玻璃橱窗拍摄，固定机位与构图，使用暖金色侧光。',
        motion: '一只戴浅色手套的手左右擦拭玻璃，雾气连续消退，全片只执行这一个主动作。',
        environment: '商品和金色蜂巢背景位置保持稳定，仅允许雾气连续消退与轻微光斑变化。',
        result: '雾气完全消失，完整露出娇兰第三代黄金复原蜜正面，瓶型、蜜蜂标识与标签清晰，最后稳定定格。',
        guardrails: guerlainGuardrails,
      }
  }
}
