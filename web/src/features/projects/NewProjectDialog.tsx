import { useCallback, useEffect, useRef, useState, type FormEvent } from 'react'
import { ApiProblem } from '../../shared/api/client'
import { createBrand, createProject } from '../platform/api'
import type { Project } from '../platform/types'

function messageFor(error: unknown) {
  if (error instanceof ApiProblem) return `${error.problem.error.message}（${error.problem.error.code}）`
  if (error instanceof Error) return error.message
  return '项目创建失败，请稍后重试。'
}

function CloseIcon() {
  return <svg aria-hidden="true" fill="none" height="20" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" viewBox="0 0 24 24" width="20"><path d="m6 6 12 12M18 6 6 18" /></svg>
}

export function NewProjectDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean
  onClose: () => void
  onCreated: (project: Project) => void
}) {
  const [name, setName] = useState('')
  const [brandName, setBrandName] = useState('')
  const [createdBrand, setCreatedBrand] = useState<{ id: string; name: string } | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const abortRef = useRef<AbortController | null>(null)

  const reset = useCallback(() => {
    abortRef.current?.abort()
    abortRef.current = null
    setName('')
    setBrandName('')
    setCreatedBrand(null)
    setSubmitting(false)
    setError('')
  }, [])

  const close = useCallback(() => {
    reset()
    onClose()
  }, [onClose, reset])

  useEffect(() => {
    if (!open) return
    function closeOnEscape(event: KeyboardEvent) {
      if (event.key === 'Escape' && !submitting) close()
    }
    window.addEventListener('keydown', closeOnEscape)
    return () => window.removeEventListener('keydown', closeOnEscape)
  }, [close, open, submitting])

  useEffect(() => () => abortRef.current?.abort(), [])

  if (!open) return null

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const projectName = name.trim()
    const nextBrandName = brandName.trim()
    if (!projectName) {
      setError('请输入项目名称。')
      return
    }
    if (!nextBrandName) {
      setError('请输入品牌名称。')
      return
    }

    const controller = new AbortController()
    abortRef.current = controller
    setSubmitting(true)
    setError('')
    try {
      let brandId: string
      if (createdBrand?.name === nextBrandName) {
        brandId = createdBrand.id
      } else {
        const brand = await createBrand(nextBrandName, controller.signal)
        brandId = brand.id
        setCreatedBrand({ id: brand.id, name: nextBrandName })
      }
      const project = await createProject({
        name: projectName,
        primary_brand_id: brandId,
        product_ids: [],
        activate: true,
      }, controller.signal)
      reset()
      onCreated(project)
    } catch (caught) {
      if (caught instanceof DOMException && caught.name === 'AbortError') return
      setError(messageFor(caught))
    } finally {
      if (abortRef.current === controller) abortRef.current = null
      setSubmitting(false)
    }
  }

  return (
    <div className="modal-layer">
      <button aria-label="关闭新建项目弹窗" className="modal-scrim" disabled={submitting} onClick={close} type="button" />
      <section aria-labelledby="new-project-title" aria-modal="true" className="project-dialog" role="dialog">
        <header className="project-dialog__header">
          <div>
            <h2 id="new-project-title">新建项目</h2>
            <p>项目会串联品牌、业务模块和独立上下文。</p>
          </div>
          <button aria-label="关闭" className="icon-button" disabled={submitting} onClick={close} type="button"><CloseIcon /></button>
        </header>

        <form onSubmit={submit}>
          <label className="form-field">
            <span>项目名称</span>
            <input autoFocus maxLength={255} onChange={(event) => setName(event.target.value)} placeholder="例如：夏季新品推广" required value={name} />
          </label>

          <label className="form-field">
            <span>品牌名称</span>
            <input maxLength={255} onChange={(event) => {
              setBrandName(event.target.value)
              if (createdBrand?.name !== event.target.value.trim()) setCreatedBrand(null)
            }} placeholder="例如：Cookies Studio" required value={brandName} />
            <small>系统将创建一个新品牌，并将它设为项目主品牌。</small>
          </label>

          <div className="creation-summary">
            <span aria-hidden="true" className="creation-summary__dot" />
            <div><strong>项目创建后立即启用</strong><small>品牌上下文版本从 v1 开始，可直接进入素材库。</small></div>
          </div>

          {error ? <div className="form-error" role="alert">{error}</div> : null}

          <footer className="project-dialog__actions">
            <button className="button button--secondary" disabled={submitting} onClick={close} type="button">取消</button>
            <button className="button button--primary" disabled={submitting} type="submit">{submitting ? '创建中…' : '创建项目'}</button>
          </footer>
        </form>
      </section>
    </div>
  )
}
