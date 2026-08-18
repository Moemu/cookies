import { useEffect, useMemo, useState } from 'react'
import { Archive, Check, Package, Pencil, Plus, RefreshCw, Save, X } from 'lucide-react'
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

type ProductDialog = { mode: 'create' } | { mode: 'edit'; product: PlatformProduct }

function formatTime(value?: string) {
  return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '—'
}

function mappingStatus(product: PlatformProduct) {
  return product.ocean_engine_product_id
    ? <span className="status success"><span/>已录入 · {product.ocean_engine_product_id}</span>
    : <span className="status warning"><span/>待录入</span>
}

export function ProductsPage({ activeView }: { activeView: string }) {
  const isMapping = activeView === '巨量映射'
  const [products, setProducts] = useState<PlatformProduct[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [dialog, setDialog] = useState<ProductDialog | null>(null)
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

  const openCreate = () => {
    setForm(emptyForm)
    setDialog({ mode: 'create' })
  }

  const openEdit = (product: PlatformProduct) => {
    setForm({
      name: product.name,
      activityType: product.activity_type ?? '',
      activityName: product.activity_name ?? '',
      brandName: product.brand_name ?? '',
    })
    setDialog({ mode: 'edit', product })
  }

  const submitDialog = async () => {
    if (!form.name.trim()) {
      setNotice('产品名称不能为空。')
      return
    }
    setBusy(true)
    try {
      if (dialog?.mode === 'edit') {
        const updated = await platformClient.updateProduct(dialog.product.id, {
          name: form.name.trim(),
          activity_type: form.activityType.trim() || undefined,
          activity_name: form.activityName.trim() || undefined,
          brand_name: form.brandName.trim() || undefined,
        })
        setProducts(current => current.map(item => item.id === updated.id ? updated : item))
        setNotice(`${updated.name} 已更新。`)
      } else {
        const created = await platformClient.createProduct({
          name: form.name.trim(),
          activity_type: form.activityType.trim() || undefined,
          activity_name: form.activityName.trim() || undefined,
          brand_name: form.brandName.trim() || undefined,
        })
        setProducts(current => [...current, created])
        setSelectedId(created.id)
        setNotice(`${created.name} 已创建。`)
      }
      setDialog(null)
    } catch (error) {
      setNotice(error instanceof Error ? error.message : dialog?.mode === 'edit' ? '更新产品失败' : '创建产品失败')
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

  return <div className="products-view">
    <div className="table-surface">
      <div className="surface-toolbar">
        <div><span className="section-label">PRODUCT CATALOG</span><h3>{isMapping ? '巨量映射' : '产品列表'}</h3><span className="products-count">{products.length} 个产品</span></div>
        <div className="products-toolbar-actions">
          <button aria-label="刷新" onClick={() => void load()} disabled={busy}><RefreshCw size={15}/></button>
          <button className="primary-button" onClick={openCreate} disabled={busy}><Plus size={15}/>新建产品</button>
        </div>
      </div>

      {isMapping ? <table>
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
      </table> : <table>
        <thead><tr><th>产品</th><th>活动 / 品牌</th><th>巨量映射</th><th>状态</th><th>操作</th></tr></thead>
        <tbody>
          {products.map(product => <tr key={product.id} className={product.id === selectedId ? 'products-row-selected' : undefined} onClick={() => setSelectedId(product.id)}>
            <td><b>{product.name}</b><small className="products-id">{product.id}</small></td>
            <td><b>{[product.activity_name, product.brand_name].filter(Boolean).join(' · ') || '—'}</b><small>{product.activity_type || '未设置活动类型'}</small></td>
            <td>{mappingStatus(product)}</td>
            <td><span className={`status ${product.status === 'archived' ? 'warning' : 'success'}`}><span/>{product.status === 'archived' ? '已归档' : '启用'}</span></td>
            <td><span className="products-row-actions">
              <button className="text-button" onClick={event => { event.stopPropagation(); openEdit(product) }} disabled={busy}><Pencil size={14}/>编辑</button>
              <button className="text-button" onClick={event => { event.stopPropagation(); void toggleArchive(product) }} disabled={busy}><Archive size={14}/>{product.status === 'archived' ? '恢复' : '归档'}</button>
            </span></td>
          </tr>)}
        </tbody>
      </table>}
      {!products.length && !busy ? <div className="panel-empty"><Package size={22}/><h3>当前组织还没有产品</h3><p>点击右上角「新建产品」创建第一个产品对象。</p></div> : null}
    </div>

    {!isMapping && selected ? <div className="products-detail">
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
        <div><span>项目关联</span><em>{projectRefs.length ? projectRefs.map(ref => ref.name).join('、') : '尚未关联项目'}</em></div>
      </footer>
    </div> : null}

    {dialog ? <div className="task-dialog-backdrop" role="dialog" aria-modal="true" onClick={() => { if (!busy) setDialog(null) }}>
      <form className="products-dialog" onSubmit={event => { event.preventDefault(); void submitDialog() }} onClick={event => event.stopPropagation()}>
        <header>
          <div><span className="section-label">{dialog.mode === 'edit' ? 'EDIT PRODUCT' : 'NEW PRODUCT'}</span><h3>{dialog.mode === 'edit' ? `编辑 ${dialog.product.name}` : '新建产品'}</h3></div>
          <button type="button" aria-label="关闭" onClick={() => setDialog(null)} disabled={busy}><X size={16}/></button>
        </header>
        <label>产品名称 <em>必填</em><input autoFocus required placeholder="如 双十一主推款礼盒" value={form.name} onChange={event => setForm(current => ({ ...current, name: event.target.value }))}/></label>
        <label>活动类型<input placeholder="如 promotion / 新品上市" value={form.activityType} onChange={event => setForm(current => ({ ...current, activityType: event.target.value }))}/></label>
        <label>活动名称<input placeholder="如 双十一大促" value={form.activityName} onChange={event => setForm(current => ({ ...current, activityName: event.target.value }))}/></label>
        <label>品牌名称<input placeholder="如 娇兰" value={form.brandName} onChange={event => setForm(current => ({ ...current, brandName: event.target.value }))}/></label>
        <footer>
          <button type="button" className="secondary-button" onClick={() => setDialog(null)} disabled={busy}>取消</button>
          <button type="submit" className="primary-button" disabled={busy}><Save size={14}/>{busy ? '保存中…' : dialog.mode === 'edit' ? '保存修改' : '创建产品'}</button>
        </footer>
      </form>
    </div> : null}

    {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
  </div>
}
