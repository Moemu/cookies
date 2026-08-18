import { useEffect, useMemo, useState } from 'react'
import { Archive, Check, Package, Plus, RefreshCw, Save, X } from 'lucide-react'
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

export function ProductsPage() {
  const [products, setProducts] = useState<PlatformProduct[]>([])
  const [selectedId, setSelectedId] = useState('')
  const [form, setForm] = useState<ProductForm>(emptyForm)
  const [showCreate, setShowCreate] = useState(false)
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
      setSelectedId(created.id)
      setForm(emptyForm)
      setShowCreate(false)
      setNotice(`${created.name} 已创建；巨量商品在录入后回绑。`)
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '创建产品失败')
    } finally {
      setBusy(false)
    }
  }

  const bindMapping = async () => {
    if (!selected) return
    setBusy(true)
    try {
      const updated = await platformClient.updateProduct(selected.id, { ocean_engine_product_id: mappingInput.trim() || undefined })
      setProducts(current => current.map(product => product.id === updated.id ? updated : product))
      setMappingInput('')
      setNotice(updated.ocean_engine_product_id ? `已回绑巨量商品 ${updated.ocean_engine_product_id}。` : '已清空巨量商品映射。')
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '回绑巨量商品失败')
    } finally {
      setBusy(false)
    }
  }

  const archiveProduct = async () => {
    if (!selected) return
    setBusy(true)
    try {
      const updated = await platformClient.updateProduct(selected.id, { status: selected.status === 'archived' ? 'active' : 'archived' })
      setProducts(current => current.map(product => product.id === updated.id ? updated : product))
      setNotice(updated.status === 'archived' ? `${updated.name} 已归档。` : `${updated.name} 已恢复为启用。`)
    } catch (error) {
      setNotice(error instanceof Error ? error.message : '更新产品状态失败')
    } finally {
      setBusy(false)
    }
  }

  return <div className="products-surface">
    <section className="products-list-panel">
      <div className="surface-toolbar">
        <div><span className="section-label">PRODUCT CATALOG</span><h3>组织级产品</h3></div>
        <button aria-label="刷新产品列表" onClick={() => void load()} disabled={busy}><RefreshCw size={15}/></button>
        <button className="primary-button" onClick={() => setShowCreate(current => !current)} disabled={busy}><Plus size={15}/>新建产品</button>
      </div>
      {showCreate ? <form className="products-create-form" onSubmit={event => { event.preventDefault(); void createProduct() }}>
        <label>产品名称<input autoFocus required value={form.name} onChange={event => setForm(current => ({ ...current, name: event.target.value }))}/></label>
        <label>活动类型<input placeholder="如 promotion / 新品上市" value={form.activityType} onChange={event => setForm(current => ({ ...current, activityType: event.target.value }))}/></label>
        <label>活动名称<input placeholder="如 双十一活动" value={form.activityName} onChange={event => setForm(current => ({ ...current, activityName: event.target.value }))}/></label>
        <label>品牌名称<input placeholder="如 娇兰" value={form.brandName} onChange={event => setForm(current => ({ ...current, brandName: event.target.value }))}/></label>
        <footer><button className="primary-button" type="submit" disabled={busy}><Save size={14}/>创建</button><button type="button" className="secondary-button" onClick={() => setShowCreate(false)}><X size={14}/>取消</button></footer>
      </form> : null}
      <div className="products-scroll">
        {products.map(product => <button key={product.id} className={product.id === selectedId ? 'products-list-item active' : 'products-list-item'} onClick={() => setSelectedId(product.id)}>
          <span className={`products-status ${product.status}`}>{product.status === 'archived' ? '已归档' : '启用'}</span>
          <b>{product.name}</b>
          <small>{product.ocean_engine_product_id ? `已录入巨量 · ${product.ocean_engine_product_id}` : '待录入巨量'}</small>
        </button>)}
        {!products.length && !busy ? <div className="panel-empty">当前组织还没有产品，点击「新建产品」创建第一个产品对象。</div> : null}
      </div>
    </section>

    <main className="products-detail-panel">
      {selected ? <article className="products-detail">
        <header>
          <div><span className="section-label">{selected.status === 'archived' ? '已归档' : '产品对象'}</span><h2>{selected.name}</h2><p>cookies 产品是事实源；巨量商品 ID 只是平台映射，回绑前视为尚未在巨量创建。</p></div>
          <button className="secondary-button" onClick={() => void archiveProduct()} disabled={busy}><Archive size={14}/>{selected.status === 'archived' ? '恢复启用' : '归档'}</button>
        </header>
        <dl className="products-fields">
          <div><dt>产品 ID</dt><dd>{selected.id}</dd></div>
          <div><dt>活动类型</dt><dd>{selected.activity_type || '—'}</dd></div>
          <div><dt>活动名称</dt><dd>{selected.activity_name || '—'}</dd></div>
          <div><dt>品牌名称</dt><dd>{selected.brand_name || '—'}</dd></div>
          <div><dt>创建时间</dt><dd>{formatTime(selected.created_at)}</dd></div>
          <div><dt>更新时间</dt><dd>{formatTime(selected.updated_at)}</dd></div>
        </dl>
        <section className="products-mapping">
          <div><span className="section-label">OCEAN ENGINE MAPPING</span><h3>巨量商品映射</h3><p>{selected.ocean_engine_product_id ? `已回绑商品 ${selected.ocean_engine_product_id}` : '尚未录入巨量；录入工作流会在投放前创建商品并回绑 ID。'}</p></div>
          <form className="products-mapping-form" onSubmit={event => { event.preventDefault(); void bindMapping() }}>
            <input placeholder="巨量商品 ID" value={mappingInput} onChange={event => setMappingInput(event.target.value)}/>
            <button className="primary-button" type="submit" disabled={busy}><Check size={14}/>回绑</button>
          </form>
        </section>
        <section className="products-projects">
          <div><span className="section-label">USED BY</span><h3>项目关联</h3></div>
          {projectRefs.length ? <ul>{projectRefs.map(ref => <li key={ref.project_id}><b>{ref.name}</b><small>{ref.project_id}</small></li>)}</ul> : <div className="panel-empty">该产品尚未关联到任何项目。</div>}
        </section>
      </article> : <div className="panel-empty"><Package size={22}/><h3>选择或新建一个产品</h3><p>产品目录是组织级业务对象，供投放计划、策略 Brief 与米云素材引用。</p></div>}
    </main>
    {notice ? <div className="inline-notice" role="status">{notice}</div> : null}
  </div>
}
