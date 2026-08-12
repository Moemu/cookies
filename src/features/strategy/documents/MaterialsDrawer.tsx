import {
	AlertTriangle, CheckCircle2, Download, Eye, FileText, LoaderCircle,
	RefreshCw, RotateCcw, Sparkles, Upload, XCircle,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { strategyApi } from '../api'
import type { DocumentPreview, DocumentVisionFallbackCapability, KnowledgeDocument, ResearchArtifact } from '../types'

type Props = {
	projectId: string
	documents: KnowledgeDocument[]
	referenceIds: string[]
	researchArtifacts: ResearchArtifact[]
	initialDocumentId?: string
	busy: string
	onCancel: (document: KnowledgeDocument) => Promise<unknown>
	onRetry: (document: KnowledgeDocument) => Promise<unknown>
	onVisualFallback: (document: KnowledgeDocument, pageNumbers?: number[]) => Promise<unknown>
	onUpload: (file: File) => Promise<unknown>
}

export function MaterialsDrawer({
	projectId, documents, referenceIds, researchArtifacts, initialDocumentId, busy,
	onCancel, onRetry, onVisualFallback, onUpload,
}: Props) {
	const [selectedId, setSelectedId] = useState(initialDocumentId ?? '')
	const [preview, setPreview] = useState<DocumentPreview | null>(null)
	const [previewError, setPreviewError] = useState('')
	const [previewLoading, setPreviewLoading] = useState(false)
	const [visionCapability, setVisionCapability] = useState<DocumentVisionFallbackCapability | null>(null)
	const [visionCapabilityLoading, setVisionCapabilityLoading] = useState(false)
	const [visionPages, setVisionPages] = useState('')
	const [visionPageError, setVisionPageError] = useState('')
	const uploadRef = useRef<HTMLInputElement>(null)
	const referenced = useMemo(() => new Set(referenceIds), [referenceIds])
	const selected = documents.find(document => document.id === selectedId) ?? null

	useEffect(() => {
		if (initialDocumentId && documents.some(document => document.id === initialDocumentId)) {
			setSelectedId(initialDocumentId)
			return
		}
		setSelectedId(current => current && documents.some(document => document.id === current)
			? current
			: documents[0]?.id ?? '')
	}, [documents, initialDocumentId])

	useEffect(() => {
		if (!selectedId) {
			setPreview(null)
			return
		}
		const controller = new AbortController()
		setPreviewLoading(true)
		setPreviewError('')
		strategyApi.getDocumentPreview(projectId, selectedId, controller.signal)
			.then(setPreview)
			.catch(error => {
				if (!controller.signal.aborted) setPreviewError(error instanceof Error ? error.message : '预览加载失败')
			})
			.finally(() => {
				if (!controller.signal.aborted) setPreviewLoading(false)
			})
		return () => controller.abort()
	}, [projectId, selectedId, selected?.updated_at])

	useEffect(() => {
		setVisionCapability(null)
		setVisionPages(selected?.vision_selected_pages.join(', ') ?? '')
		setVisionPageError('')
		if (!selected || selected.status !== 'partial' || selected.quality_tier !== 'low') return
		const controller = new AbortController()
		setVisionCapabilityLoading(true)
		strategyApi.getDocumentVisionFallbackCapability(projectId, selected.id, controller.signal)
			.then(setVisionCapability)
			.catch(() => {
				if (!controller.signal.aborted) setVisionCapability({
					contract_version: 'platform-document-vision-fallback/v1', document_id: selected.id,
					eligible: true, recommended: true, available: false,
					reason_code: 'DOCUMENT_VISION_CAPABILITY_UNREACHABLE', model_alias: '',
					conversion_required: false,
					max_pages_per_request: 24, requires_page_selection: true,
				})
			})
			.finally(() => {
				if (!controller.signal.aborted) setVisionCapabilityLoading(false)
			})
		return () => controller.abort()
	}, [projectId, selected?.id, selected?.quality_tier, selected?.status, selected?.updated_at])

	const handleUpload = async (file?: File) => {
		if (!file) return
		await onUpload(file)
		if (uploadRef.current) uploadRef.current.value = ''
	}

	const handleVisionFallback = async () => {
		if (!selected || !visionCapability?.available) return
		let pages: number[] = []
		if (visionCapability.requires_page_selection) {
			const parsed = parsePageSelection(visionPages, selected.total_pages)
			if (parsed.error) {
				setVisionPageError(parsed.error)
				return
			}
			pages = parsed.pages
		}
		setVisionPageError('')
		await onVisualFallback(selected, pages)
	}

	return <section className="strategy-materials" aria-label="项目资料">
		<header className="strategy-materials__summary">
			<div><span>PROJECT MATERIALS</span><b>{documents.length} 份资料</b><small>{referenceIds.length} 份已进入当前 Brief / 策略上下文</small></div>
			<label className="strategy-materials__upload">
				{busy === 'upload-document' ? <LoaderCircle className="spin" size={15}/> : <Upload size={15}/>}
				<span>{busy === 'upload-document' ? '上传中' : '添加资料'}</span>
				<input ref={uploadRef} type="file" accept=".md,.txt,.html,.htm,.docx,.pdf,.pptx" disabled={busy === 'upload-document'} onChange={event => void handleUpload(event.target.files?.[0])}/>
			</label>
		</header>

		<div className="strategy-materials__layout">
			<div className="strategy-materials__queue" aria-label="解析队列">
				{documents.map(document => <button
					className={document.id === selectedId ? 'is-selected' : ''}
					key={document.id}
					onClick={() => setSelectedId(document.id)}
					type="button"
				>
					<DocumentStateIcon document={document}/>
					<span><b>{document.title || document.filename}</b><small>{documentProgressLabel(document)}</small></span>
					{referenced.has(document.id) ? <em>已引用</em> : null}
				</button>)}
				{!documents.length ? <div className="strategy-materials__empty"><FileText size={24}/><b>还没有项目资料</b><p>添加 Brief、报告或演示文稿后，解析状态和质量信号会显示在这里。</p></div> : null}
			</div>

			<div className="strategy-materials__detail">
				{selected ? <>
					<div className="strategy-materials__detail-head">
						<div><span>{selected.mime_type}</span><h3>{selected.title || selected.filename}</h3><p>{formatBytes(selected.size_bytes)} · {selected.chunk_count} 个可追溯片段</p></div>
						<div className="strategy-materials__actions">
							{selected.status === 'parse_queued' || selected.status === 'parsing' ? <button onClick={() => void onCancel(selected)} type="button"><XCircle size={14}/>取消</button> : null}
							{selected.status === 'parse_failed' || selected.status === 'partial' ? <button onClick={() => void onRetry(selected)} type="button"><RotateCcw size={14}/>重新解析</button> : null}
							{preview?.original_available ? <a href={strategyApi.documentContentUrl(projectId, selected.id)} target="_blank" rel="noreferrer"><Download size={14}/>原文件</a> : null}
						</div>
					</div>

					<DocumentProgress document={selected}/>
					<DocumentQuality document={selected}/>
					<DocumentVisionFallback
						busy={busy === `document:vision:${selected.id}`}
						capability={visionCapability}
						capabilityLoading={visionCapabilityLoading}
						document={selected}
						onPageSelectionChange={value => { setVisionPages(value); setVisionPageError('') }}
						onRun={() => void handleVisionFallback()}
						pageError={visionPageError}
						pageSelection={visionPages}
					/>
					{previewLoading ? <div className="strategy-materials__preview-state"><LoaderCircle className="spin" size={18}/>正在准备安全预览…</div> : null}
					{previewError ? <div className="strategy-materials__preview-state is-error"><AlertTriangle size={18}/>{previewError}<button onClick={() => setSelectedId('')} type="button">关闭</button></div> : null}
					{preview && !previewLoading ? <DocumentPreviewPane preview={preview}/> : null}
				</> : <div className="strategy-materials__empty"><Eye size={24}/><b>选择一份资料查看详情</b><p>这里只在选中后加载正文预览，不会在后台批量下载所有材料。</p></div>}
			</div>
		</div>

		{researchArtifacts.some(artifact => referenced.has(artifact.id)) ? <div className="strategy-materials__research">
			<span>RESEARCH EVIDENCE</span>
			{researchArtifacts.filter(artifact => referenced.has(artifact.id)).map(artifact => <article key={artifact.id}><Sparkles size={14}/><div><b>{artifact.title}</b><small>{artifact.citations[0] || '研究产物'}</small></div></article>)}
		</div> : null}
	</section>
}

function DocumentVisionFallback({
	document, capability, capabilityLoading, busy, pageSelection, pageError, onPageSelectionChange, onRun,
}: {
	document: KnowledgeDocument
	capability: DocumentVisionFallbackCapability | null
	capabilityLoading: boolean
	busy: boolean
	pageSelection: string
	pageError: string
	onPageSelectionChange: (value: string) => void
	onRun: () => void
}) {
	if (document.vision_fallback_status === 'queued' || document.vision_fallback_status === 'running') {
		const converting = document.parse_phase === 'visual_conversion'
		return <section className="strategy-materials__vision is-running"><LoaderCircle className="spin" size={17}/><div><b>{converting ? '正在将演示文稿转换为可追溯 PDF' : '视觉解析正在后台执行'}</b><p>{converting ? '转换完成后会自动进入所选页面的视觉解析；原始演示文稿和已有文本都会保留。' : `已选择 ${document.vision_selected_pages.length} 页；可以离开本面板，不会阻塞其他工作。`}</p></div></section>
	}
	if (document.vision_fallback_status === 'succeeded' || document.vision_fallback_status === 'partial') {
		return <section className="strategy-materials__vision is-ready"><CheckCircle2 size={17}/><div><b>视觉补充已写入可追溯片段</b><p>完成 {document.vision_completed_pages.length} / {document.vision_selected_pages.length} 页；原始文本结果仍被保留。</p></div></section>
	}
	if (document.vision_fallback_status === 'failed') {
		const needsReconciliation = visionFailureNeedsReconciliation(document.vision_error_code)
		return <section className="strategy-materials__vision is-failed"><AlertTriangle size={17}/><div><b>视觉解析未完成，文本结果未丢失</b><p>{needsReconciliation ? '外部任务是否已提交尚未确认。为避免重复计费，系统已停止自动和手动重试；请先由运维人员到 LAS 对账。' : document.vision_error_message || '可以稍后重新尝试；当前已有片段仍可使用。'}</p>{capability?.available && !needsReconciliation ? <button type="button" disabled={busy} onClick={onRun}>{busy ? <LoaderCircle className="spin" size={14}/> : <RotateCcw size={14}/>}重试所选页面</button> : null}</div></section>
	}
	if (document.status !== 'partial' || document.quality_tier !== 'low') return null
	return <section className="strategy-materials__vision">
		<Sparkles size={17}/>
		<div>
			<b>可选：用视觉模型补充低质量内容</b>
			{capabilityLoading ? <p>正在检查固定模型路由，不会发起解析或产生费用…</p> : capability?.available ? <>
				<p>只有你确认后才会执行；结果会按页保留 locator，并与原文本并存。</p>
				{capability.conversion_required ? <small>将先在后台通过 {capability.converter_code ?? '固定转换器'} 转为 PDF，再交给固定视觉模型；原始演示文稿不会被替换。</small> : null}
				{capability.requires_page_selection ? <label>选择页面（最多 24 页）<input value={pageSelection} onChange={event => onPageSelectionChange(event.target.value)} placeholder="例如 1, 3-5, 8"/></label> : <small>将处理当前文档的 {document.total_pages ?? 0} 页。</small>}
				{pageError ? <em>{pageError}</em> : null}
				<button type="button" disabled={busy} onClick={onRun}>{busy ? <LoaderCircle className="spin" size={14}/> : <Sparkles size={14}/>}确认并后台解析</button>
			</> : <p>{visionCapabilityReason(capability?.reason_code)} 当前文本结果仍可继续使用。</p>}
		</div>
	</section>
}

function visionFailureNeedsReconciliation(code?: string) {
	return code === 'DOCUMENT_VISION_SUBMISSION_UNKNOWN' ||
		code === 'DOCUMENT_VISION_SUBMISSION_INVALID' ||
		code === 'DOCUMENT_VISION_CHECKPOINT_FAILED'
}

function parsePageSelection(value: string, totalPages: number | null): { pages: number[]; error: string } {
	const pages = new Set<number>()
	for (const part of value.split(',').map(item => item.trim()).filter(Boolean)) {
		const match = part.match(/^(\d+)(?:\s*-\s*(\d+))?$/)
		if (!match) return { pages: [], error: '请使用“1, 3-5, 8”这样的页码格式。' }
		const start = Number(match[1])
		const end = Number(match[2] ?? match[1])
		if (start < 1 || end < start || totalPages != null && end > totalPages) return { pages: [], error: '页码超出当前文档范围。' }
		for (let page = start; page <= end; page += 1) {
			pages.add(page)
			if (pages.size > 24) return { pages: [], error: '一次最多选择 24 页。' }
		}
	}
	return pages.size ? { pages: [...pages].sort((left, right) => left - right), error: '' } : { pages: [], error: '请至少选择 1 页。' }
}

function visionCapabilityReason(code?: string) {
	return ({
		DOCUMENT_VISION_PROVIDER_DISABLED: '视觉文档解析适配器尚未配置，系统不会静默换用其他模型。',
		DOCUMENT_VISION_ROUTE_UNAVAILABLE: '固定视觉模型路由当前不可用，系统不会静默换用其他模型。',
		DOCUMENT_VISION_CONVERTER_DISABLED: '当前格式需要先转换为 PDF；转换器尚未配置，系统不会伪装成可用。',
		DOCUMENT_VISION_STORAGE_SCOPE_INVALID: '文档不在当前项目配置的同一对象存储桶内，系统已阻止跨桶解析。',
		DOCUMENT_VISION_CAPABILITY_UNREACHABLE: '暂时无法确认视觉模型路由状态。',
		DOCUMENT_VISION_INELIGIBLE: '当前文档不需要进入视觉回退。',
	} as Record<string, string>)[code ?? ''] ?? '视觉解析能力暂不可用。'
}

function DocumentStateIcon({ document }: { document: KnowledgeDocument }) {
	if (document.status === 'ready') return <CheckCircle2 className="is-ready" size={17}/>
	if (document.status === 'partial') return <AlertTriangle className="is-partial" size={17}/>
	if (document.status === 'parse_failed') return <XCircle className="is-failed" size={17}/>
	return <LoaderCircle className="spin is-running" size={17}/>
}

function DocumentProgress({ document }: { document: KnowledgeDocument }) {
	const progress = document.parse_progress ?? 0
	const active = document.status === 'parse_queued' || document.status === 'parsing'
	return <section className={`strategy-materials__progress ${active ? 'is-active' : ''}`} aria-label="解析进度">
		<div><span>{phaseLabel(document.parse_phase)}</span><b>{document.parse_progress == null ? '—' : `${progress}%`}</b></div>
		<div className="strategy-materials__progress-track"><i style={{ width: `${progress}%` }}/></div>
		<small>{document.progress_kind === 'pages' && document.total_pages
			? `已处理 ${document.processed_pages ?? 0} / ${document.total_pages} 页`
			: active ? '当前格式不提供可靠逐页回调，显示真实里程碑进度。' : documentProgressLabel(document)}</small>
	</section>
}

function DocumentQuality({ document }: { document: KnowledgeDocument }) {
	if (document.quality_tier === 'unknown') return null
	const summary = document.page_quality_summary
	return <section className={`strategy-materials__quality is-${document.quality_tier}`}>
		<header><div><span>解析质量路由信号</span><b>{qualityLabel(document.quality_tier)}</b></div><strong>{document.quality_score == null ? '—' : Math.round(document.quality_score * 100)}</strong></header>
		<p>用于判断是否需要人工检查或视觉解析，不代表内容事实准确率。</p>
		{summary ? <>
			<dl className="strategy-materials__quality-metrics">
				<div><dt>文字密度</dt><dd>{summary.characters_per_page == null ? '页数未知' : `${Math.round(summary.characters_per_page)} 字/页`}</dd></div>
				<div><dt>来源定位</dt><dd>{formatQualityRatio(summary.locator_coverage)}</dd></div>
				<div><dt>图片信号</dt><dd>{formatMetadataSignal(summary.metadata_image_signal_ratio, summary.metadata_image_signals)}</dd></div>
				<div><dt>表格信号</dt><dd>{formatMetadataSignal(summary.metadata_table_signal_ratio, summary.metadata_table_signals)}</dd></div>
			</dl>
			<small className="strategy-materials__quality-footnote">
				空白页：{summary.empty_pages == null ? '解析器未提供' : `${summary.empty_pages} 页`} · 阅读顺序：{readingOrderLabel(summary.reading_order_signal)}。图片/表格为元数据信号密度，不等同真实页面占比。
			</small>
		</> : null}
		{summary?.signals?.length ? <ul>{summary.signals.slice(0, 3).map(signal => <li key={signal.code}>{qualitySignalLabel(signal.code)}</li>)}</ul> : null}
		{summary?.shadow_fallback_recommended ? <div className="strategy-materials__fallback"><Sparkles size={14}/><span><b>建议检查视觉解析</b><small>当前仅作 shadow 候选，不会自动触发额外模型费用。</small></span></div> : null}
	</section>
}

function DocumentPreviewPane({ preview }: { preview: DocumentPreview }) {
	return <section className="strategy-materials__preview">
		<header><span>TEXT PREVIEW</span><small>{preview.total_characters.toLocaleString('zh-CN')} 字符{preview.text_truncated ? ' · 已截断' : ''}</small></header>
		{preview.text ? <pre>{preview.text}</pre> : <div className="strategy-materials__preview-state">当前还没有可预览的正文。</div>}
		{preview.chunks.length ? <details><summary>查看 {preview.chunks.length} 个来源定位</summary>{preview.chunks.map(chunk => <article key={chunk.id}><b>片段 {chunk.index + 1}{chunk.page_number ? ` · 第 ${chunk.page_number} 页` : ''}</b><small>行 {chunk.start_line}–{chunk.end_line}</small><p>{chunk.snippet}</p></article>)}</details> : null}
	</section>
}

function documentProgressLabel(document: KnowledgeDocument) {
	if (document.status === 'ready') return `${document.chunk_count} 个片段 · 已就绪`
	if (document.status === 'partial') return `${document.chunk_count} 个片段可用 · 建议检查`
	if (document.status === 'parse_failed') return document.parse_error_message || '解析失败，可重试'
	return `${phaseLabel(document.parse_phase)}${document.parse_progress == null ? '' : ` · ${document.parse_progress}%`}`
}

function phaseLabel(phase: KnowledgeDocument['parse_phase']) {
	return ({ queued: '等待解析', scanning: '安全检查', extracting: '提取正文', quality_checking: '质量检查', visual_conversion: '演示文稿转 PDF', visual_fallback: '视觉解析', chunking: '建立来源定位', ready: '解析完成', partial: '部分可用', failed: '解析失败' } as const)[phase] ?? phase
}

function qualityLabel(tier: KnowledgeDocument['quality_tier']) {
	return ({ unknown: '待评估', high: '高', medium: '中', low: '低' } as const)[tier]
}

function formatQualityRatio(value: number) {
	return `${Math.round(Math.max(0, Math.min(1, value)) * 100)}%`
}

function formatMetadataSignal(ratio: number | null | undefined, count: number) {
	if (ratio == null) return count > 0 ? `${count} 个信号` : '未检出'
	return `${formatQualityRatio(ratio)} · ${count} 个`
}

function readingOrderLabel(signal: string) {
	return ({ acceptable: '未见明显异常', suspicious: '建议人工核对', not_assessed: '样本不足' } as Record<string, string>)[signal] ?? '未评估'
}

function qualitySignalLabel(code: string) {
	return ({
		very_low_text_volume: '提取正文过少，可能是扫描件或图片型页面',
		low_text_volume: '提取正文偏少，请检查是否遗漏主要内容',
		replacement_character_rate: '异常替换字符较多，可能存在字体映射或编码问题',
		control_character_rate: '不可见控制字符偏多，阅读顺序可能不稳定',
		meaningful_character_rate: '可识别文字占比较低，可能混入乱码或版式噪声',
		page_character_density: '平均每页文字偏少，图片可能承载主要信息',
		locator_coverage: '部分正文缺少稳定的来源定位',
		fragmented_reading_order: '短碎行较多，建议人工核对阅读顺序',
		blank_pages: '解析器报告空白页，建议确认是否遗漏图片或扫描内容',
	} as Record<string, string>)[code] ?? code
}

function formatBytes(value: number) {
	if (value < 1024) return `${value} B`
	if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
	return `${(value / 1024 / 1024).toFixed(1)} MB`
}
