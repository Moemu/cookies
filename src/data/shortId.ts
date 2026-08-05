/**
 * ID 在界面上的显示口径。
 *
 * 系统生成的 ID 是 `insightreport_` + 32 位十六进制，整条摆在页面上会把一行挤爆，
 * 所以只显示尾部——尾部足够人在两条记录之间对上号，也足够复制去查。
 *
 * 但不能无脑切尾。演示数据和外部导入的 ID 常常本来就很短（`demo_execution_2`），
 * 切掉前 8 位剩下「cution_2」——一个既不是完整 ID 也读不出含义的残词，人对不上任何东西。
 * 短的整条显示，长的才切。
 */
const shortEnough = 16
const tailLength = 8

export function shortId(value: string): string {
  const text = (value ?? '').trim()
  if (text.length <= shortEnough) return text
  return text.slice(-tailLength)
}
