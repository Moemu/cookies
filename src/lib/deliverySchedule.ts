const shanghaiDateFormatter = new Intl.DateTimeFormat('en-CA', {
  timeZone: 'Asia/Shanghai',
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
})

export function toShanghaiDateInput(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const parts = Object.fromEntries(shanghaiDateFormatter.formatToParts(date).map(part => [part.type, part.value]))
  return `${parts.year}-${parts.month}-${parts.day}`
}

export function fromShanghaiStartDate(value: string) {
  return value ? `${value}T00:00:00+08:00` : ''
}

export function fromShanghaiEndDate(value: string) {
  return value ? `${value}T23:59:59+08:00` : ''
}
