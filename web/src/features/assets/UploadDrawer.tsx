import { useEffect, useRef, useState } from 'react'
import { ApiProblem } from '../../shared/api/client'
import { calculateSHA256, createAssetUpload, finalizeAssetUpload, putAssetContent } from './api'
import { AssetIcon } from './AssetIcon'
import type { UploadSession } from './types'

const MAX_IMAGE_BYTES = 20 * 1024 * 1024
const acceptedTypes = new Set(['image/png', 'image/jpeg'])

type UploadStage = 'idle' | 'hashing' | 'creating' | 'uploading' | 'processing' | 'done' | 'error'

const steps = [
  '创建会话',
  '上传文件',
  '安全处理',
  '完成入库',
] as const

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}

function errorMessage(error: unknown) {
  if (error instanceof ApiProblem) return `${error.problem.error.message}（${error.problem.error.code}）`
  if (error instanceof Error) return error.message
  return '上传失败，请稍后重试。'
}

export function UploadDrawer({
  open,
  projectId,
  onClose,
  onComplete,
}: {
  open: boolean
  projectId: string
  onClose: () => void
  onComplete: (file: File, session: UploadSession) => void
}) {
  const [file, setFile] = useState<File | null>(null)
  const [previewUrl, setPreviewUrl] = useState('')
  const [sha256, setSha256] = useState('')
  const [stage, setStage] = useState<UploadStage>('idle')
  const [error, setError] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const abortRef = useRef<AbortController | null>(null)
  const previewUrlRef = useRef('')

  useEffect(() => () => {
    abortRef.current?.abort()
    if (previewUrlRef.current) URL.revokeObjectURL(previewUrlRef.current)
  }, [])

  if (!open) return null

  const busy = ['hashing', 'creating', 'uploading', 'processing'].includes(stage)
  const activeStep = stage === 'uploading' ? 1 : stage === 'processing' ? 2 : stage === 'done' ? 3 : 0

  function reset() {
    abortRef.current?.abort()
    abortRef.current = null
    clearFile()
    setSha256('')
    setStage('idle')
    setError('')
  }

  function close() {
    reset()
    onClose()
  }

  function selectFile(nextFile?: File) {
    if (!nextFile) return
    if (!acceptedTypes.has(nextFile.type)) {
      setError('仅支持 PNG 或 JPEG 图片。')
      return
    }
    if (nextFile.size < 1 || nextFile.size > MAX_IMAGE_BYTES) {
      setError('图片大小必须在 20 MB 以内。')
      return
    }
    if (previewUrlRef.current) URL.revokeObjectURL(previewUrlRef.current)
    const nextPreviewUrl = URL.createObjectURL(nextFile)
    previewUrlRef.current = nextPreviewUrl
    setFile(nextFile)
    setPreviewUrl(nextPreviewUrl)
    setSha256('')
    setStage('idle')
    setError('')
  }

  function clearFile() {
    if (previewUrlRef.current) URL.revokeObjectURL(previewUrlRef.current)
    previewUrlRef.current = ''
    setPreviewUrl('')
    setFile(null)
  }

  async function upload() {
    if (!file || busy) return
    const controller = new AbortController()
    abortRef.current = controller
    setError('')
    try {
      setStage('hashing')
      const digest = await calculateSHA256(file)
      setSha256(digest)

      setStage('creating')
      const created = await createAssetUpload(projectId, file, digest, controller.signal)
      if (!created.upload) throw new Error('上传会话未返回可用的上传地址。')

      setStage('uploading')
      await putAssetContent(created.upload, file, controller.signal)

      setStage('processing')
      const completed = await finalizeAssetUpload(projectId, created.session.id, controller.signal)
      if (completed.status !== 'succeeded' || !completed.project_asset_ref) {
        throw new Error(completed.error_code || '素材未能完成入库。')
      }
      setStage('done')
      onComplete(file, completed)
    } catch (caught) {
      if (caught instanceof DOMException && caught.name === 'AbortError') return
      setError(errorMessage(caught))
      setStage('error')
    } finally {
      abortRef.current = null
    }
  }

  return (
    <>
      <button className="drawer-scrim" aria-label="关闭上传素材" onClick={close} type="button" />
      <aside className="upload-drawer" aria-label="上传素材" aria-busy={busy}>
        <header className="drawer-header">
          <div>
            <h2>上传素材</h2>
            <p>图片将先进入隔离区，校验完成后再写入项目素材库。</p>
          </div>
          <button className="icon-button" aria-label="关闭" onClick={close} type="button"><AssetIcon name="close" /></button>
        </header>

        <ol className="upload-steps" aria-label="上传进度">
          {steps.map((step, index) => {
            const complete = stage === 'done' || index < activeStep
            const active = index === activeStep && stage !== 'idle' && stage !== 'error'
            return <li className={complete ? 'step step--complete' : active ? 'step step--active' : 'step'} key={step}>
              <span className="step__dot">{complete ? '✓' : index + 1}</span>
              <span>{step}</span>
            </li>
          })}
        </ol>

        <div
          className="dropzone"
          onDragOver={(event) => event.preventDefault()}
          onDrop={(event) => {
            event.preventDefault()
            if (!busy) selectFile(event.dataTransfer.files[0])
          }}
        >
          <AssetIcon name="upload" size={30} />
          <strong>拖拽图片到此处</strong>
          <span>或 <button disabled={busy} onClick={() => inputRef.current?.click()} type="button">点击选择文件</button></span>
          <small>支持 PNG / JPEG，最大 20 MB</small>
          <input
            ref={inputRef}
            aria-label="选择图片文件"
            accept="image/png,image/jpeg"
            disabled={busy}
            hidden
            onChange={(event) => selectFile(event.target.files?.[0])}
            type="file"
          />
        </div>

        {file ? <section className="selected-file" aria-label="已选择文件">
          <div className="selected-file__summary">
            <div className="selected-file__preview">
              {previewUrl ? <img alt="待上传素材预览" src={previewUrl} /> : <AssetIcon name="image" />}
            </div>
            <div><strong>{file.name}</strong><span>{formatBytes(file.size)} · {file.type}</span></div>
            <button className="text-button" disabled={busy} onClick={clearFile} type="button">移除</button>
          </div>
          <dl className="file-metadata">
            <div><dt>校验和（SHA-256）</dt><dd title={sha256}>{sha256 || (stage === 'hashing' ? '计算中…' : '将在上传前计算')}</dd></div>
            <div><dt>项目</dt><dd>{projectId}</dd></div>
          </dl>
        </section> : null}

        {stage === 'processing' ? <div className="processing-note"><span className="spinner" /><div><strong>安全处理与入库中</strong><p>正在校验文件类型、尺寸、摘要和项目上下文。</p></div></div> : null}
        {stage === 'done' ? <div className="success-note"><span>✓</span><div><strong>素材已完成入库</strong><p>资产列表已刷新，可以继续上传下一张图片。</p></div></div> : null}
        {error ? <div className="error-note" role="alert"><strong>上传未完成</strong><span>{error}</span></div> : null}

        <footer className="drawer-actions">
          <button className="button button--secondary" onClick={close} type="button">{busy ? '取消上传' : stage === 'done' ? '完成' : '取消'}</button>
          {stage === 'done'
            ? <button className="button button--primary" onClick={reset} type="button">继续上传</button>
            : <button className="button button--primary" disabled={!file || busy} onClick={upload} type="button">
                {busy ? '处理中…' : stage === 'error' ? '重新上传' : '开始上传'}
              </button>}
        </footer>
      </aside>
    </>
  )
}
