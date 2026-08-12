import { useCallback, useEffect, useState } from 'react'
import { Plus, RefreshCw } from 'lucide-react'
import { useProject } from '../../../context/ProjectContext'
import { api, type ApiExternalAsset, type ApiExternalPurpose, type ApiInsightAssetType } from '../../../data/api'
import { formatDate } from '../analysis/format'

const purposeLabels: Record<ApiExternalPurpose, string> = {
  benchmark: '同类参照',
  reference: '解释用例',
}

const purposeHints: Record<ApiExternalPurpose, string> = {
  benchmark: '拿来回答「同类素材大概什么水平」。',
  reference: '拿来当正例或反例，解释本轮某一条结论。',
}

const assetTypeLabels: Record<ApiInsightAssetType, string> = {
  xiaohongshu_note: '小红书图文',
  wechat_article: '公众号图文',
  brand_ad: '品牌广告',
  digital_human_ad: '数字人效果广告',
  preroll_ad: '广告前贴',
  hit_replica_ad: '爆款复刻',
}

/**
 * 「外部素材」视图。平台外的素材以**证据**的身份收在这里。
 *
 * `window_end` 不给人手填的口子：它决定留存期限从哪天算起，填错了这条素材会比
 * 该留的时间多留或少留。从页面当前窗口取，和分析页同一套。
 */
export function ExternalView({ window }: { window: { start: string; end: string } }) {
  const { currentProject } = useProject()
  const [items, setItems] = useState<ApiExternalAsset[]>([])
  const [listState, setListState] = useState<'loading' | 'ready' | 'error'>('loading')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)

  const [title, setTitle] = useState('')
  const [sourceNote, setSourceNote] = useState('')
  const [purpose, setPurpose] = useState<ApiExternalPurpose | ''>('')
  const [purposeNote, setPurposeNote] = useState('')
  const [assetType, setAssetType] = useState('')

  const load = useCallback(async () => {
    if (!currentProject.id) return
    setListState('loading')
    try {
      const page = await api.listExternalAssets(currentProject.id)
      setItems(page.items)
      setListState('ready')
    } catch (cause) {
      setItems([])
      setListState('error')
      setNotice(cause instanceof Error ? cause.message : '外部素材读取失败。')
    }
  }, [currentProject.id])

  useEffect(() => { void load() }, [load])

  const submit = async () => {
    if (!currentProject.id || !title.trim() || !purpose) return
    setBusy(true)
    setNotice('')
    try {
      const created = await api.importExternalAsset(currentProject.id, {
        title: title.trim(),
        source_note: sourceNote.trim(),
        purpose,
        purpose_note: purposeNote.trim() || undefined,
        asset_type: assetType || undefined,
        window_end: window.end,
      })
      setTitle('')
      setSourceNote('')
      setPurpose('')
      setPurposeNote('')
      setAssetType('')
      await load()
      setNotice(`已收下「${created.title}」，留到 ${formatDate(created.retention_until)}。`)
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '导入失败，请稍后重试。')
    } finally {
      setBusy(false)
    }
  }

  return <section className="external-view">
    <p className="prelaunch-disclosure">
      这里的素材是<b>证据</b>，不是资产。它们不会进共享素材库、不能被拿去投放，
      也不能改——要改就重新导一份。收它们只有一个用处：解释本轮结果时有个参照。
    </p>

    <div className="external-columns">
      <div className="external-list">
        <div className="prelaunch-filterbar">
          <span>{items.length} 条外部证据 · 留存期从本轮窗口结束日（{formatDate(window.end)}）起算</span>
          <button type="button" className="secondary-button" disabled={listState === 'loading'}
            onClick={() => { void load() }}><RefreshCw size={15}/>刷新</button>
        </div>

        {listState === 'loading' ? <div className="panel-empty">正在读取…</div>
          : listState === 'error' ? <div className="panel-empty">读取失败，请重试。</div>
          : !items.length ? <div className="panel-empty">
            还没有收过外部素材。看到值得当参照的，用右边的表单收一条——记得说清用途。
          </div>
          : <ul className="external-items">
            {items.map(item => <li key={item.id}>
              <div className="external-head">
                <strong>{item.title}</strong>
                <span className="external-purpose">{purposeLabels[item.purpose]}</span>
              </div>
              {/* 到期日和用途要在列表上就能看见：原件被清掉之前，人得有机会
                  把想做的分析做完；清掉之后，「当初为什么收这个」只剩用途这一栏。 */}
              <span className="external-retention">
                留到 {formatDate(item.retention_until)}
                {item.original_purged ? '（原件已删，只剩标注的变量）' : ''}
              </span>
              {item.source_note ? <small>来源：{item.source_note}</small> : null}
              {item.purpose_note ? <small>{item.purpose_note}</small> : null}
              {item.asset_type ? <small>{assetTypeLabels[item.asset_type] ?? item.asset_type}</small> : null}
            </li>)}
          </ul>}
      </div>

      <form className="external-form" onSubmit={event => { event.preventDefault(); void submit() }}>
        <span className="section-label">收一条外部素材</span>

        <label>标题<input value={title} onChange={event => setTitle(event.target.value)}
          placeholder="这条素材叫什么" required/></label>

        <label>来源说明<input value={sourceNote} onChange={event => setSourceNote(event.target.value)}
          placeholder="哪儿看到的，例如「竞品 A 的抖音信息流」"/></label>

        <fieldset className="external-purpose-field">
          <legend>用途</legend>
          {(['benchmark', 'reference'] as ApiExternalPurpose[]).map(value => <label key={value}>
            <input type="radio" name="external-purpose" value={value} checked={purpose === value}
              onChange={() => setPurpose(value)}/>
            {purposeLabels[value]}<small>{purposeHints[value]}</small>
          </label>)}
          <small className="form-hint">
            用途要选，因为留存期到了之后，「当初为什么收这个」只有这一栏说得清。
          </small>
        </fieldset>

        <label>补充说明<textarea value={purposeNote} onChange={event => setPurposeNote(event.target.value)}
          placeholder="为什么这一条值得留（选填）"/></label>

        <label>素材类型<select value={assetType} onChange={event => setAssetType(event.target.value)}>
          <option value="">不指定</option>
          {Object.entries(assetTypeLabels).map(([value, label]) =>
            <option key={value} value={value}>{label}</option>)}
        </select></label>

        <button type="submit" className="secondary-button" disabled={busy || !title.trim() || !purpose}>
          <Plus size={15}/>{busy ? '正在收…' : '收下这条'}
        </button>

        {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
      </form>
    </div>
  </section>
}
