import { AssetIcon } from './AssetIcon'
import type { ProjectAsset } from './types'

export function RemoveAssetDialog({
  asset,
  busy,
  error,
  onClose,
  onConfirm,
}: {
  asset: ProjectAsset | null
  busy: boolean
  error: string
  onClose: () => void
  onConfirm: () => void
}) {
  if (!asset) return null
  const label = `${asset.asset.id} · v${asset.version.version}`

  return <div className="modal-layer">
    <button aria-label="关闭删除素材弹窗" className="modal-scrim" disabled={busy} onClick={onClose} type="button" />
    <section aria-labelledby="remove-asset-title" aria-modal="true" className="project-dialog remove-asset-dialog" role="dialog">
      <header className="project-dialog__header">
        <div><h2 id="remove-asset-title">从项目中删除素材？</h2><p>删除后，这个素材版本将不再出现在当前项目素材库。</p></div>
        <button aria-label="关闭" className="icon-button" disabled={busy} onClick={onClose} type="button"><AssetIcon name="close" /></button>
      </header>
      <div className="remove-asset-target">
        <AssetIcon name="image" size={22} />
        <div><strong title={label}>{label}</strong><span>{asset.version.mime_type} · {asset.version.source_type === 'provider_generated' ? 'Provider 生成' : '用户素材'}</span></div>
      </div>
      <p className="remove-asset-note">底层不可变版本、文件和引用记录仍会保留，用于审计和 Provider 任务溯源。</p>
      {error ? <div className="form-error" role="alert">{error}</div> : null}
      <footer className="project-dialog__actions">
        <button className="button button--secondary" disabled={busy} onClick={onClose} type="button">取消</button>
        <button className="button button--danger" disabled={busy} onClick={onConfirm} type="button">{busy ? '删除中…' : '确认删除'}</button>
      </footer>
    </section>
  </div>
}
