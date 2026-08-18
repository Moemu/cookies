import { useEffect, useMemo, useState } from 'react'
import { Archive, Check, Package, Plus, RefreshCw, Save } from 'lucide-react'
import {
  platformClient,
  type PlatformProduct,
  type PlatformProductProjectRef,
} from '../data/platformClient'

type ProductForm = {
  name: string
  activityType: string
  activityName: string
  brandName: string
}

const emptyForm: ProductForm = { name: '', activityType: '', activityName: '', brandName: '' }

function formatTime(value?: string) {
  return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—'
}

export function ProductsPage({ activeView }: { activeView: string }) {
  const view = activeView === '新建产品' ? 'create' : activeView === '巨量映射' ? 'mapping' : 'list'
  const [products, setProducts] = useState<PlatformProduct[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [form, setForm] = useState<ProductForm>(emptyForm)
  const [mappingInput, setMappingInput] = useState('')
  const [projectRefs, setProjectRefs] = useState<PlatformProductProjectRef[]>([])
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')

  const selected = useMemo(() => products.find(product => product.id === selectedId), [products, selectedId])

  const load = async () => {
    setBusy(true)
    try {
      const values = await platformClient.listProducts()
      setProducts(values)
      setSelectedId(current => (current && values.some(product => product.id === current) ? current : values[0]?.id ?? ''))
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '加载产品目录失败')
    } finally {
      setBusy(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  useEffect(() => {
    let active = true
    setProjectRefs([])
    if (!selectedId) return () => { active = false }
    void platformClient.listProductProjects(selectedId).then(refs => {
      if (active) setProjectRefs(refs)
    }).catch(() => {
      if (active) setProjectRefs([])
    })
    return () => { active = false }
  }, [selectedId])

  const createProduct = async () => {
    if (!form.name.trim()) {
      setNotice('产品名称不能为空。')
      return
    }
    setBusy(true)
    try {
      const created = await platformClient.createProduct({
        name: form.name.trim(),
        activity_type: form.activityType.trim() || undefined,
        activity_name: form.activityName.trim() || undefined,
        brand_name: form.brandName.trim() || undefined,
      })
      setProducts(current => [...current, created])
      setForm(emptyForm)
      setNotice(`${created.name} 已创建，可在「产品列表」查看并回绑巨量商品。`)
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '创建产品失败')
    } finally {
      setBusy(false)
    }
  }

  const bindMapping = async (product: PlatformProduct) => {
    const value = mappingInput.trim()
    if (!value) return
    setBusy(true)
    try {
      const updated = await platformClient.updateProduct(product.id, { ocean_engine_product_id: value })
      setProducts(current => current.map(item => item.id === updated.id ? updated : item))
      setMappingInput('')
      setNotice(`${updated.name} 已回绑巨量商品 ${updated.ocean_engine_product_id}。`)
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '回绑巨量商品失败')
    } finally {
      setBusy(false)
    }
  }

  const clearMapping = async (product: PlatformProduct) => {
    setBusy(true)
    try {
      const updated = await platformClient.updateProduct(product.id, { ocean_engine_product_id: undefined })
      setProducts(current => current.map(item => item.id === updated.id ? updated : item))
      setNotice(`${updated.name} 已清空巨量商品映射。`)
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '清空映射失败')
    } finally {
      setBusy(false)
    }
  }

  const toggleArchive = async (product: PlatformProduct) => {
    setBusy(true)
    try {
      const updated = await platformClient.updateProduct(product.id, { status: product.status === 'archived' ? 'active' : 'archived' })
      setProducts(current => current.map(item => item.id === updated.id ? updated : item))
      setNotice(updated.status === 'archived' ? `${updated.name} 已归档。` : `${updated.name} 已恢复为启用。`)
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '更新产品状态失败')
    } finally {
      setBusy(false)
    }
  }

  const mappingStatus = (product: PlatformProduct) => product.ocean_engine_product_id
    ? <span className="status success"><span/>已录入 · {product.ocean_engine_product_id}</span>
    : <span className="status warning"><span/>待录入</span>

  const toolbar = (title: string, sub: string) => <div className="surface-toolbar">
    <div><span className="section-label">PRODUCT CATALOG</span><h3>{title}</h3><span>{sub}</span></div>
    <button aria-label="刷新产品列表" onClick={() => void load()} disabled={busy}><RefreshCw size={15}/></button>
  </div>

  if (view === 'create') {
    return <div className="products-view">
      <div className="table-surface">
        {toolbar('新建产品', '创建组织级业务产品对象；巨量映射在录入后回绑。')}
        <form className="products-create-form" onSubmit={event => { event.preventDefault(); void createProduct() }}>
          <label>产品名称 <em>必填</em><input autoFocus required placeholder="如 双十一主推款礼盒" value={form.name} onChange={event => setForm(current => ({ ...current, name: event.target.value }))}/></label>
          <label>活动类型<input placeholder="如 promotion / 新品上市" value={form.activityType} onChange={event => setForm(current => ({ ...current, activityType: event.target.value }))}/></label>
          <label>活动名称<input placeholder="如 双十一大促" value={form.activityName} onChange={event => setForm(current => ({ ...current, activityName: event.target.value }))}/></label>
          <label>品牌名称<input placeholder="如 娇兰" value={form.brandName} onChange={event => setForm(current => ({ ...current, brandName: event.target.value }))}/></label>
          <footer><button className="primary-button" type="submit" disabled={busy}><Save size={14}/>{busy ? '创建中…' : '创建产品'}</button></footer>
        </form>
        <div className="products-create-note"><Package size={16}/><span>cookies 产品是事实源：投放下拉、策略 Brief 与米云素材都引用这里的对象。创建后进入「巨量映射」完成平台录入与回绑。</span></div>
      </div>
      {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
    </div>
  }

  if (view === 'mapping') {
    return <div className="products-view">
      <div className="table-surface">
        {toolbar('巨量映射', '商品在巨量平台录入后回绑商品 ID；未回绑视为尚未在巨量创建。')}
        <table>
          <thead><tr><th>产品</th><th>活动</th><th>映射状态</th><th>巨量商品 ID</th></tr></thead>
          <tbody>
            {products.map(product => <tr key={product.id}>
              <td><b>{product.name}</b><small>{product.id}</small></td>
              <td><b>{[product.activity_name, product.brand_name].filter(Boolean).join(' · ') || '—'}</b><small>{product.activity_type || '未设置活动类型'}</small></td>
              <td>{mappingStatus(product)}</td>
              <td>{product.ocean_engine_product_id
                ? <span className="products-mapping-value"><code>{product.ocean_engine_product_id}</code><button className="text-button" onClick={() => void clearMapping(product)} disabled={busy}>清空</button></span>
                : <form className="products-mapping-form" onSubmit={event => { event.preventDefault(); void bindMapping(product) }}>
                    <input placeholder="粘贴巨量商品 ID" value={mappingInput} onChange={event => setMappingInput(event.target.value)}/>
                    <button className="primary-button" type="submit" disabled={busy || !mappingInput.trim()}><Check size={14}/>回绑</button>
                  </form>}
              </td>
            </tr>)}
          </tbody>
        </table>
        {!products.length && !busy ? <div className="panel-empty">当前组织还没有产品，先去「新建产品」创建一个。</div> : null}
      </div>
      {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
    </div>
  }

  return <div className="products-view">
    <div className="table-surface">
      {toolbar('产品列表', `${products.length} 个产品`) }
      <table>
        <thead><tr><th>产品</th><th>活动 / 品牌</th><th>巨量映射</th><th>状态</th><th>项目关联</th></tr></thead>
        <tbody>
          {products.map(product => <tr key={product.id} className={product.id === selectedId ? 'products-row-selected' : undefined} onClick={() => setSelectedId(product.id)}>
            <td><b>{product.name}</b><small>{product.id}</small></td>
            <td><b>{[product.activity_name, product.brand_name].filter(Boolean).join(' · ') || '—'}</b><small>{product.activity_type || '未设置活动类型'}</small></td>
            <td>{mappingStatus(product)}</td>
            <td><span className={`status ${product.status === 'archived' ? 'warning' : 'success'}`}><span/>{product.status === 'archived' ? '已归档' : '启用'}</span></td>
            <td><button className="text-button" onClick={event => { event.stopPropagation(); void toggleArchive(product) }} disabled={busy}><Archive size={14}/>{product.status === 'archived' ? '恢复' : '归档'}</button></td>
          </tr>)}
        </tbody>
      </table>
      {!products.length && !busy ? <div className="panel-empty">当前组织还没有产品，先去「新建产品」创建第一个产品对象。</div> : null}
    </div>

    {selected ? <div className="products-detail">
      <header>
        <div><span className="section-label">产品对象</span><h2>{selected.name}</h2><p>cookies 产品是事实源；巨量商品 ID 只是平台映射。</p></div>
        <span className="products-detail-id">{selected.id}</span>
      </header>
      <dl>
        <div><dt>活动类型</dt><dd>{selected.activity_type || '—'}</dd></div>
        <div><dt>活动名称</dt><dd>{selected.activity_name || '—'}</dd></div>
        <div><dt>品牌名称</dt><dd>{selected.brand_name || '—'}</dd></div>
        <div><dt>创建时间</dt><dd>{formatTime(selected.created_at)}</dd></div>
        <div><dt>更新时间</dt><dd>{formatTime(selected.updated_at)}</dd></div>
      </dl>
      <footer>
        <div><span>巨量映射</span>{mappingStatus(selected)}</div>
        <div><span>项目关联</span>{projectRefs.length ? <em>{projectRefs.map(ref => ref.name).join('、')}</em> : <em>尚未关联项目</em>}</div>
      </footer>
    </div> : null}

    {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
  </div>
}
