// 「记一笔」。这是分析页唯一的写操作：分析页只读结论，把值得留下的那条
// 钉进本轮复盘草稿，复盘页再逐条勾选提交。
//
// 三种状态：可按、已记、不给按。不给按的时候必须说清为什么——一个灰着的按钮
// 不给理由，人只会以为页面坏了，然后去别的地方找一样的东西按。
export function PinFindingButton({ onPin, pinned, reason }: {
  onPin?: () => void
  pinned?: boolean
  /** 不给按的理由。给了它就说明这条不该进结论，会顶掉默认提示。 */
  reason?: string
}) {
  return (
    <button
      type="button"
      className={pinned ? 'pin-finding pinned' : 'pin-finding'}
      disabled={!onPin || pinned}
      onClick={onPin}
      title={reason ?? (pinned ? '已经记进本轮复盘' : '把这条发现记进本轮复盘')}
    >
      {pinned ? '已记一笔' : '记一笔'}
    </button>
  )
}
