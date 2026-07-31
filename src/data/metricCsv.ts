import type { ApiMetricRow } from './api'

/**
 * 把粘贴进来的报表转成导入行。
 *
 * 这里只做形状检查，业务规则（日期是否越界、数值是否为负、对象是否已登记）留给
 * 后端逐行判断——后端会把被拒的行连同原因一起返回，比在前端拦掉更有用。
 *
 * 但有一件事必须在这里拦死：**认不出的列不能静默填 0**。曝光填成 0 的行在后端
 * 眼里是完全合法的一行，会入库、会计入总盘、界面还会报「被拒 0 行」。那是假装
 * 成功的数据损坏，比导入失败严重得多。
 */

/** 后端认得的七个数值指标（connectors.go MetricCounts）。不在这张表里的列一律视为认不出。 */
const numericMetricColumns = new Set([
  'impressions',
  'clicks',
  'conversions',
  'video_views',
  'video_completions',
  'spend_cents',
  'revenue_cents',
])

/** 非数值的结构列。缺了必需的那两个会在表头校验时单独报错。 */
const structuralColumns = new Set([
  'platform_object_kind',
  'platform_object_id',
  'platform_object_name',
  'stat_date',
])

/**
 * 内置别名。表头允许写中文，省得每次导入前先去改一遍平台导出的文件。
 * 这是**兜底**，不是权威——数据源上配的 field_mapping 优先。
 */
export const columnAliases: Record<string, string> = {
  对象类型: 'platform_object_kind',
  对象ID: 'platform_object_id',
  对象名称: 'platform_object_name',
  日期: 'stat_date',
  曝光: 'impressions',
  展现: 'impressions',
  展现数: 'impressions',
  点击: 'clicks',
  点击数: 'clicks',
  转化: 'conversions',
  转化数: 'conversions',
  播放: 'video_views',
  完播: 'video_completions',
  花费分: 'spend_cents',
  消耗分: 'spend_cents',
  收入分: 'revenue_cents',
}

/**
 * 给「字段映射」界面用的列名清单。
 *
 * 和上面两张集合、这张别名表同源，不另抄一份：抄一份出来，改了解析器忘了改界面，
 * 人照着界面配出来的映射就会被解析器当成认不出的列拦掉，而错在界面上看不出来。
 *
 * `required` 的两个必须露出来。之前界面只列了七个数值指标，人无从知道「广告ID」
 * 该对到什么名字上，而少了它整份表都导不进去——这是卡死，不是不好用。
 */
export type MetricColumn = { key: string; label: string; required?: boolean; note?: string }

export const metricColumns: MetricColumn[] = [
  { key: 'platform_object_id', label: '哪条广告（平台上的 ID）', required: true },
  { key: 'stat_date', label: '哪一天', required: true, note: '写成 2026-07-20 这样' },
  { key: 'platform_object_kind', label: '是广告还是创意', note: '不填按广告算' },
  { key: 'platform_object_name', label: '平台上的名字' },
  { key: 'impressions', label: '曝光' },
  { key: 'clicks', label: '点击' },
  { key: 'conversions', label: '转化' },
  { key: 'video_views', label: '播放', note: '只有视频有' },
  { key: 'video_completions', label: '完播', note: '只有视频有' },
  { key: 'spend_cents', label: '花费', note: '单位是分，报表里是元要先乘 100' },
  { key: 'revenue_cents', label: '收入', note: '单位是分，要电商回传才有' },
]

/** 系统内置认得的中文写法，反查成 canonical 名 → 别名列表。表头写这些就不用配映射。 */
export function builtInAliasesOf(canonical: string): string[] {
  return Object.entries(columnAliases)
    .filter(([, target]) => target === canonical)
    .map(([alias]) => alias)
}

/**
 * 把一个平台列名归一成 canonical 指标名。
 *
 * 数据源上配的映射优先于内置别名：内置别名是猜的，映射是人明确配的。
 * 两者冲突时听人的——否则人在「字段映射」里辛苦配的东西会被一张硬编码表悄悄推翻。
 */
export function canonicalColumn(name: string, fieldMapping: Record<string, string> = {}): string {
  return fieldMapping[name] ?? columnAliases[name] ?? name
}

export function parseMetricCsv(text: string, fieldMapping: Record<string, string> = {}): ApiMetricRow[] {
  const lines = text.split('\n').map(line => line.trim()).filter(Boolean)
  if (lines.length < 2) throw new Error('至少要有表头和一行数据。')

  const rawHeader = lines[0].split(',').map(cell => cell.trim())
  const header = rawHeader.map(cell => canonicalColumn(cell, fieldMapping))

  const required = ['platform_object_id', 'stat_date']
  const missing = required.filter(column => !header.includes(column))
  if (missing.length) throw new Error(`表头缺少必需的列：${missing.join('、')}。`)

  const unknown = header
    .map((column, index) => ({ column, original: rawHeader[index] }))
    .filter(({ column }) => !structuralColumns.has(column) && !numericMetricColumns.has(column))
    .map(({ original }) => original)
  if (unknown.length) {
    throw new Error(
      `这些列对不上任何指标：${unknown.join('、')}。` +
      '去「字段映射」把它们配好，或者从文件里删掉——认不出就导进去会让这几列变成 0，' +
      '而界面还会显示导入成功。',
    )
  }

  return lines.slice(1).map((line, index) => {
    const cells = line.split(',').map(cell => cell.trim())
    if (cells.length !== header.length) {
      throw new Error(`第 ${index + 1} 行有 ${cells.length} 列，表头有 ${header.length} 列，对不上。`)
    }
    const record: Record<string, string> = {}
    const raw: Record<string, string> = {}
    header.forEach((column, position) => { record[column] = cells[position] })
    rawHeader.forEach((column, position) => { raw[column] = cells[position] })
    return {
      platform_object_kind: record.platform_object_kind || 'ad',
      platform_object_id: record.platform_object_id,
      platform_object_name: record.platform_object_name || undefined,
      stat_date: record.stat_date,
      counts: {
        impressions: toInteger(record.impressions),
        clicks: toInteger(record.clicks),
        conversions: toInteger(record.conversions),
        video_views: toInteger(record.video_views),
        video_completions: toInteger(record.video_completions),
        spend_cents: toInteger(record.spend_cents),
        revenue_cents: toInteger(record.revenue_cents),
      },
      // doc10 §4.1「原始平台事实不可被统一指标覆盖」。口径纠纷、对账、字段映射
      // 改错后的重算，三件事都要知道平台报表上原来那行长什么样。
      // 后端 MetricFact.Raw 这一列一直在，只是从来没人填。
      raw,
    }
  })
}

/**
 * 缺的列（比如这份报表没有收入）取 0 是对的——那是「没这个指标」，不是「认不出」。
 * 认不出的列在上面已经拦掉了，走不到这里。
 */
function toInteger(value?: string): number {
  const parsed = Number.parseInt((value ?? '').replace(/[,\s]/g, ''), 10)
  return Number.isFinite(parsed) ? parsed : 0
}

/** djb2。只用来判断「这份文件是不是刚才那份」，不是安全用途。 */
export function hashText(text: string): string {
  let hash = 5381
  for (let index = 0; index < text.length; index += 1) {
    hash = ((hash << 5) + hash + text.charCodeAt(index)) | 0
  }
  return `paste-${(hash >>> 0).toString(16)}-${text.length}`
}
