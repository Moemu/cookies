import { AlertCircle, ShieldCheck } from 'lucide-react'
import type { PackageVersion, StrategyCreativeHandoff, StrategyDraft } from './types'

export function CreativeHandoffPackageSummary({ draft, handoff, strategyPackage }: {
  draft: StrategyDraft | null
  handoff: StrategyCreativeHandoff | null
  strategyPackage: PackageVersion | null
}) {
  if (!strategyPackage) {
    return <section className="creative-handoff-package missing" role="status">
      <AlertCircle size={18}/>
      <div><b>等待不可变 StrategyPackage</b><small>{draft?.status === 'approved'
        ? '当前策略已确认，正在等待对应 Package 可读取；刷新后仍无结果时请检查发布记录。'
        : '确认并发布当前策略 Revision 后，才能选择冻结 Route 并创建创意任务。'}</small></div>
    </section>
  }

  const ready = handoff?.upstream_readiness.status === 'ready'
  return <section className="creative-handoff-package" data-ready={ready} aria-label="创意交接来源包">
    <header>
      <span><ShieldCheck size={18}/></span>
      <div><small>只读来源</small><b>StrategyPackage v{strategyPackage.version}</b></div>
      <em>{ready ? '交接就绪' : '存在阻断项'}</em>
    </header>
    <dl>
      <div><dt>Strategy Revision</dt><dd>{strategyPackage.snapshot.strategy_revision}</dd></div>
      <div><dt>Package</dt><dd title={strategyPackage.package_id}>{strategyPackage.package_id}</dd></div>
      <div><dt>Package hash</dt><dd title={strategyPackage.content_hash}>{shortHash(strategyPackage.content_hash)}</dd></div>
      <div><dt>Handoff hash</dt><dd title={handoff?.handoff_content_hash}>{handoff ? shortHash(handoff.handoff_content_hash) : '尚未生成'}</dd></div>
    </dl>
    <p>任务答案和 Overlay 只会创建下游版本，不会改变以上 Package 内容或哈希。</p>
  </section>
}

function shortHash(value: string) {
  return value.length > 24 ? `${value.slice(0, 18)}…${value.slice(-6)}` : value
}
