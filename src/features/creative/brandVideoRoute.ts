type BrandVideoChannel = 'xiaohongshu' | 'douyin' | 'kuaishou'

type BrandVideoIntake = {
  selected_route_id?: string
  base_handoff?: {
    routes?: Array<{
      route_id: string
      deliverable_type?: string
      purpose?: string
      performance_mode?: string
      channels: string[]
    }>
  }
  request?: {
    selected_route_id?: string
    creative_routes?: Array<{
      route_id: string
      route_type?: string
      channels: string[]
    }>
  }
}

const supportedChannels = new Set<BrandVideoChannel>(['xiaohongshu', 'douyin', 'kuaishou'])

export function resolveBrandVideoRouteTarget(intake: BrandVideoIntake): {
  selectedRouteId: string
  channel: BrandVideoChannel
} {
  const selectedRouteId = intake.selected_route_id?.trim()
    || intake.request?.selected_route_id?.trim()
    || ''
  const frozenRoutes = intake.base_handoff?.routes ?? intake.request?.creative_routes ?? []
  const route = frozenRoutes.find(item => item.route_id === selectedRouteId)
  const isBrandVideo = route && (
    'route_type' in route && route.route_type === 'brand_video'
    || 'performance_mode' in route && route.performance_mode === 'brand_video'
    || 'deliverable_type' in route && route.deliverable_type === 'video' && route.purpose === 'brand'
  )
  if (!selectedRouteId || !route || !isBrandVideo) {
    throw new Error('品牌视频交接缺少已冻结的创意路线，请返回 Strategy 重新交接。')
  }
  const channel = route.channels.find(
    (value): value is BrandVideoChannel => supportedChannels.has(value as BrandVideoChannel),
  )
  if (!channel) {
    throw new Error('当前品牌视频路线没有 Creative 支持的生产渠道。')
  }
  return { selectedRouteId, channel }
}
