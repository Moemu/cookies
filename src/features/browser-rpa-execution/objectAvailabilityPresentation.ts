import type { RunnerV3Plan } from './model'

type ObjectAvailabilityItem = NonNullable<RunnerV3Plan['object_availability']>[number]

export type ObjectAvailabilityPresentation = {
  available: boolean
  kindLabel: string
  name: string
  scopeLabel: string
  statusLabel: string
  statusDetail: string
  platformId?: string
  technicalType: string
}

const OBJECT_KIND_LABELS: Record<string, string> = {
  application: '应用',
  brand: '品牌',
  category: '行业分类',
  creative_component: '创意组件',
  delivery_identity: '投放身份',
  direct_link: '直达链接',
  industry_category: '行业分类',
  landing_page: '落地页',
  material: '视频素材',
  native_anchor: '原生锚点',
  owned_landing_page: '落地页',
  product: '营销商品',
  product_catalog: '商品库',
  product_image: '商品图片',
  video_material: '视频素材',
}

function isUrl(value: string | undefined): boolean {
  if (!value) return false
  try {
    const url = new URL(value)
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}

function urlHost(value: string): string {
  try {
    return new URL(value).hostname
  } catch {
    return ''
  }
}

function kindLabel(item: ObjectAvailabilityItem): string {
  if (item.field_key === 'project.marketing_product_reference') return '营销商品'
  if (item.field_key.includes('base_material_references')) return '视频素材'
  return OBJECT_KIND_LABELS[item.object_kind] || '平台对象'
}

function scopeLabel(fieldKey: string): string {
  const promotion = /^promotions\.(\d+)\./.exec(fieldKey)
  if (promotion) return `单元 ${Number(promotion[1]) + 1}`
  if (fieldKey.startsWith('project.')) return '项目'
  return '投放计划'
}

function displayName(item: ObjectAvailabilityItem, label: string): string {
  const candidate = item.display_name || item.internal_object_id
  if (isUrl(candidate)) {
    const host = urlHost(candidate)
    return host ? `${host} ${label}` : label
  }
  return candidate || label
}

export function presentObjectAvailability(item: ObjectAvailabilityItem): ObjectAvailabilityPresentation {
  const label = kindLabel(item)
  const name = displayName(item, label)
  const publicPlatformId = isUrl(item.platform_object_id) ? undefined : item.platform_object_id

  if (item.available) {
    return {
      available: true,
      kindLabel: label,
      name,
      scopeLabel: scopeLabel(item.field_key),
      statusLabel: '已绑定',
      statusDetail: publicPlatformId ? '平台对象可用。' : `${label}可用。`,
      platformId: publicPlatformId,
      technicalType: item.object_kind,
    }
  }

  return {
    available: false,
    kindLabel: label,
    name,
    scopeLabel: scopeLabel(item.field_key),
    statusLabel: '需处理',
    statusDetail: `请先为“${name}”绑定巨量${label} ID。`,
    technicalType: item.object_kind,
  }
}

export function presentPlanBlockedReason(reason: string): string {
  if (reason === 'PLATFORM_OBJECTS_UNAVAILABLE') {
    return '有巨量对象尚未绑定。请处理下方标记为“需处理”的对象。'
  }
  return reason
}
