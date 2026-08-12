/**
 * 「本次判定使用的阈值」。
 *
 * 阈值可写之后，这个标注就是必需的：不标的话，一条三个月前的结论和一条今天的
 * 结论看起来完全一样，但可能是按两套标准算出来的。
 *
 * **不要每一行都挂。**满屏都是「按第 3 版阈值判定」会把真正要读的结论淹掉；
 * 只在人可能问「这是按什么标准算的」的地方出现——每屏顶部一处，复盘里每条
 * 发现一处，经验卡的「凭什么」展开层里一处。
 */
export function ThresholdStamp({ version }: { version: number | undefined }) {
  // version 缺失（老数据、还没升级的接口）时什么都不显示。显示「按出厂阈值判定」
  // 是在替一条来历不明的结论作保，而这正是这个标注要防的事。
  if (version === undefined || version === null) return null
  return (
    <span className="threshold-stamp" title="判定标准可以在「设置 · 判定阈值」里调整">
      {version === 0 ? '按出厂阈值判定' : `按第 ${version} 版阈值判定`}
    </span>
  )
}
