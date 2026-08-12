// 六个视图共用的格式化。放一处而不是每个视图各写一份：同一个「不可用」在两屏上
// 写成两种样子，人会以为那是两种不同的情况。
//
// 共同的一条规则：undefined 是「算不出来」，不是 0。0 会被读成「表现极差」，
// 那是另一件事。

export function formatRate(value?: number): string {
  return value === undefined ? '不可用' : `${(value * 100).toFixed(2)}%`
}

export function formatSigned(value?: number): string {
  if (value === undefined) return '不可用'
  return `${value > 0 ? '+' : ''}${(value * 100).toFixed(1)}%`
}

export function formatMoney(cents?: number): string {
  // 金额固定两位小数：同一列里 ¥28 和 ¥201.67 并排会让人以为量级差了一位。
  return cents === undefined
    ? '不可用'
    : `¥${(cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

export function formatRatio(value?: number): string {
  return value === undefined ? '不可用' : `${value.toFixed(2)}×`
}

export function formatCount(value: number): string {
  return value.toLocaleString('zh-CN')
}

export function formatNumber(value: number): string {
  return Number.isInteger(value) ? value.toLocaleString('zh-CN') : value.toFixed(4)
}

export function formatDate(value: string): string {
  const time = new Date(value)
  return Number.isNaN(time.valueOf()) ? value : time.toLocaleDateString('zh-CN')
}

/** 后端只收 2006-01-02，带时分秒会被判成参数无效。按本地日历天取，不走 UTC。 */
export function isoDate(value: Date): string {
  return `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, '0')}-${String(value.getDate()).padStart(2, '0')}`
}
