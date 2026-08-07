export type BrandVideoChannel = 'xiaohongshu' | 'douyin' | 'kuaishou'

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

export function resolveBrandVideoRouteOptions(intake: BrandVideoIntake): {
  selectedRouteId: string
  channels: BrandVideoChannel[]
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
  const channels = route.channels.filter(
    (value): value is BrandVideoChannel => supportedChannels.has(value as BrandVideoChannel),
  )
  if (!channels.length) {
    throw new Error('当前品牌视频路线没有 Creative 支持的生产渠道。')
  }
  return { selectedRouteId, channels: [...new Set(channels)] }
}

export function resolveBrandVideoRouteTarget(
  intake: BrandVideoIntake,
  selectedChannel?: BrandVideoChannel,
): {
  selectedRouteId: string
  channel: BrandVideoChannel
} {
  const options = resolveBrandVideoRouteOptions(intake)
  if (selectedChannel && !options.channels.includes(selectedChannel)) {
    throw new Error('选择的生产渠道不在已冻结的品牌视频路线中。')
  }
  if (options.channels.length > 1 && !selectedChannel) {
    throw new Error('该品牌视频路线包含多个渠道，请先明确选择本次生产渠道。')
  }
  return {
    selectedRouteId: options.selectedRouteId,
    channel: selectedChannel ?? options.channels[0],
  }
}

export function resolveBrandVideoRouteTargets(
  intake: BrandVideoIntake,
  selectedChannels: BrandVideoChannel[],
): Array<{
  selectedRouteId: string
  channel: BrandVideoChannel
}> {
  const options = resolveBrandVideoRouteOptions(intake)
  const channels = [...new Set(selectedChannels)]
  if (!channels.length) {
    throw new Error('请至少选择一个渠道适配。')
  }
  const unsupportedChannel = channels.find(channel => !options.channels.includes(channel))
  if (unsupportedChannel) {
    throw new Error('选择的渠道适配不在已冻结的品牌视频路线中。')
  }
  return channels.map(channel => ({
    selectedRouteId: options.selectedRouteId,
    channel,
  }))
}

export function toggleBrandVideoChannel(
  selectedChannels: BrandVideoChannel[],
  channel: BrandVideoChannel,
): BrandVideoChannel[] {
  return selectedChannels.includes(channel)
    ? selectedChannels.filter(value => value !== channel)
    : [...selectedChannels, channel]
}
