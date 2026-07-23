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
    id: 'window-reveal', name: '雾面橱窗揭幕', category: '仪式展示', duration: '6–8 秒',
    hook: '从不可见到清晰可见，让商品登场自带仪式感。', image: '/assets/commerce-hook-reveal.png', imageLabel: '尾帧参考', frameStrategy: '首尾帧生视频',
    fidelity: '保持香水瓶颜色、透明材质、瓶盖造型和标签文字不变。',
    camera: '写实中景，透过玻璃橱窗拍摄，镜头静止，暖金色冬日侧光。',
    motion: '戴手套的手左右擦拭两次玻璃雾气，动作幅度克制且轨迹连续。',
    environment: '橱窗雪花与礼盒保持稳定，背景光斑轻微闪烁，层次由雾到清晰。',
    result: '雾气逐渐消失，完整露出商品正面，最后定格 1 秒呈现玻璃质感。',
    guardrails: '无人物露脸；擦拭不遮挡标签；首尾帧构图、光影和商品位置一致。', tags: ['香氛', '礼赠', '高质感'],
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
