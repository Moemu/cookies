// 「记一笔」。这是分析页唯一的写操作：分析页只读结论，把值得留下的那条
// 钉进本轮复盘草稿，复盘页再逐条勾选提交。
//
// 本期只出壳：onPin 不给就是禁用态，真正的写入在 P1 分析页计划里接。
export function PinFindingButton({ onPin, pinned }: {
  onPin?: () => void
  pinned?: boolean
}) {
  return (
    <button
      type="button"
      className={pinned ? 'pin-finding pinned' : 'pin-finding'}
      disabled={!onPin || pinned}
      onClick={onPin}
      title={pinned ? '已经记进本轮复盘' : '把这条发现记进本轮复盘'}
    >
      {pinned ? '已记一笔' : '记一笔'}
    </button>
  )
}
