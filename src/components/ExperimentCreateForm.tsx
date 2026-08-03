import { useMemo, useState } from 'react'
import { CircleAlert } from 'lucide-react'
import {
  api,
  type ApiCreateExperimentInput,
  type ApiCreateVariantInput,
  type ApiExperiment,
  type ApiFeatureField,
  type ApiFeatureSchema,
  type ApiInsightAssetType,
} from '../data/api'

/**
 * 新建实验。整张表单只干一件事：**在看到任何数据之前，把话说死**。
 *
 * 所以这里没有「保存草稿稍后再填变量」这种入口——变量、分组、样本门槛和观察窗口
 * 少任何一项，这次实验事后都能被重新解释成另一次实验。表单一次性收齐，后端再校验
 * 一遍（experiments.go validate）。
 *
 * 自由文本字段不出现在「被测变量」里：文本没有可枚举的取值，分组就无从对齐，
 * 最后一定退化成人事后挑两条素材说「这两条只差文案」。
 */
const assetTypeLabels: Record<ApiInsightAssetType, string> = {
  xiaohongshu_note: '小红书图文',
  wechat_article: '公众号图文',
  brand_ad: '品牌广告',
  digital_human_ad: '数字人效果广告',
  preroll_ad: '广告前贴',
  hit_replica_ad: '爆款复刻',
}

const assetTypeOrder: ApiInsightAssetType[] = [
  'preroll_ad', 'digital_human_ad', 'xiaohongshu_note', 'wechat_article', 'brand_ad', 'hit_replica_ad',
]

// 后端 experiments_service.go 用同一条规则挡自由文本，这里只是提前告诉人为什么选不了。
function comparable(field: ApiFeatureField): boolean {
  return field.kind !== 'text' && field.kind !== 'tags'
}

// 多选特征的取值不是一个词，是一整套词。一条素材的「钩子类型」同时是「问题」和「反差」时，
// 后端读到的取值是「反差、问题」（experiments_service.go featureText 按字典序拼），分组也按这个
// 完整取值对齐。所以这里必须能挑一套词——只能挑一个词的话，这类素材永远进不了任何一组：
// 挑了「问题」，素材报「反差、问题」，入组被硬拦，而人在页面上没有任何办法把这组改成它要的样子。
function composeTerms(terms: string[]): string {
  // 排序口径要和后端一致。中文都在 BMP 内，JS 的默认排序（UTF-16 码元）和 Go 的
  // sort.Strings（UTF-8 字节）在这个范围里同序。
  return Array.from(new Set(terms)).sort().join('、')
}

function splitTerms(value: string): string[] {
  return value ? value.split('、').filter(Boolean) : []
}

type VariantDraft = ApiCreateVariantInput & { key: string }

function blankVariants(): VariantDraft[] {
  return [
    { key: 'v1', name: '基线组', variable_value: '', is_baseline: true },
    { key: 'v2', name: '变体组', variable_value: '', is_baseline: false },
  ]
}

export function ExperimentCreateForm({ projectId, schemas, sourceExperienceId, presetHypothesis, onCreated, onCancel }: {
  projectId: string
  schemas: ApiFeatureSchema[]
  /** 从投前洞察的假设卡「拿去验证」带过来的出处；空白新建时为空。 */
  sourceExperienceId?: string
  presetHypothesis?: string
  onCreated: (experiment: ApiExperiment) => void
  onCancel: () => void
}) {
  const [title, setTitle] = useState('')
  const [hypothesis, setHypothesis] = useState(presetHypothesis ?? '')
  const [assetType, setAssetType] = useState<ApiInsightAssetType>('preroll_ad')
  const [variableKey, setVariableKey] = useState('')
  const [controlledKeys, setControlledKeys] = useState<string[]>([])
  const [minImpressions, setMinImpressions] = useState('10000')
  const [windowStart, setWindowStart] = useState(offsetDate(-30))
  const [windowEnd, setWindowEnd] = useState(offsetDate(0))
  const [variants, setVariants] = useState<VariantDraft[]>(blankVariants)
  const [notice, setNotice] = useState('')
  const [saving, setSaving] = useState(false)

  const schema = useMemo(() => schemas.find(item => item.asset_type === assetType), [schemas, assetType])
  const fields = useMemo(() => (schema?.fields ?? []).filter(comparable), [schema])
  const variableField = useMemo(() => fields.find(field => field.key === variableKey), [fields, variableKey])

  function pickAssetType(next: ApiInsightAssetType) {
    setAssetType(next)
    // 换了素材类型，原来那套特征键就不存在了。留着会让人对着一个已经不在的变量提交。
    setVariableKey('')
    setControlledKeys([])
    setVariants(blankVariants())
  }

  function pickVariable(next: string) {
    setVariableKey(next)
    setControlledKeys(keys => keys.filter(key => key !== next))
    setVariants(items => items.map(item => ({ ...item, variable_value: '' })))
  }

  function toggleControlled(key: string) {
    setControlledKeys(keys => keys.includes(key) ? keys.filter(item => item !== key) : [...keys, key])
  }

  function updateVariant(key: string, patch: Partial<ApiCreateVariantInput>) {
    setVariants(items => items.map(item => item.key === key ? { ...item, ...patch } : item))
  }

  // 基线只能有一个，所以设基线是「转移」而不是「勾选」。做成 checkbox 会让人勾出两个基线，
  // 然后在提交时被后端拒绝——那是把一个本来不该发生的状态先允许出来再报错。
  function pickBaseline(key: string) {
    setVariants(items => items.map(item => ({ ...item, is_baseline: item.key === key })))
  }

  function addVariant() {
    setVariants(items => items.length >= 8 ? items : [
      ...items,
      { key: `v${items.length + 1}-${items.length}`, name: `变体组 ${items.length}`, variable_value: '', is_baseline: false },
    ])
  }

  function removeVariant(key: string) {
    setVariants(items => {
      if (items.length <= 2) return items
      const next = items.filter(item => item.key !== key)
      return next.some(item => item.is_baseline) ? next : next.map((item, index) => ({ ...item, is_baseline: index === 0 }))
    })
  }

  async function submit() {
    if (!variableKey) { setNotice('先选被测变量。没有变量，这只是一批素材，不是实验。'); return }
    const threshold = Number(minImpressions)
    if (!Number.isFinite(threshold) || threshold <= 0) { setNotice('每组最低展示量要是一个正整数。'); return }
    const body: ApiCreateExperimentInput = {
      title: title.trim(),
      hypothesis: hypothesis.trim(),
      source_experience_id: sourceExperienceId,
      asset_type: assetType,
      variable_key: variableKey,
      controlled_keys: controlledKeys,
      min_impressions: Math.round(threshold),
      window_start: windowStart,
      window_end: windowEnd,
      variants: variants.map(({ key: _key, ...rest }) => ({ ...rest, name: rest.name.trim(), variable_value: rest.variable_value.trim() })),
    }
    setSaving(true)
    setNotice('')
    try {
      onCreated(await api.createInsightExperiment(projectId, body))
    } catch (cause) {
      setNotice(cause instanceof Error ? cause.message : '实验创建失败。')
    } finally {
      setSaving(false)
    }
  }

  return <div className="experiment-form">
    <div className="experiment-form-head">
      <span className="section-label">新建实验</span>
      <h3>把假设、变量、分组和门槛在开跑之前写下来</h3>
      <p>
        这些字段一旦开跑就锁死。事后再定门槛，等于允许「凑够了就说显著」；事后再改分组，
        等于允许看完数据挑一组好的。
        {sourceExperienceId ? ' 这次实验来自投前洞察的一条假设，有结论后会知道该回头更新哪张卡。' : null}
      </p>
    </div>

    <label className="experiment-field">
      <span>实验名称</span>
      <input value={title} onChange={event => setTitle(event.target.value)} placeholder="例：开场三秒用人脸还是用产品特写"/>
    </label>

    <label className="experiment-field">
      <span>假设（这次想验证什么，以及为什么这么想）</span>
      <textarea value={hypothesis} onChange={event => setHypothesis(event.target.value)}
        placeholder="例：前贴广告开场三秒出现人脸时，点击率高于直接出产品特写。依据是上一轮投放里带人脸的几条都更靠前。"/>
    </label>

    <label className="experiment-field">
      <span>素材类型</span>
      <select value={assetType} onChange={event => pickAssetType(event.target.value as ApiInsightAssetType)}>
        {assetTypeOrder.map(type => <option key={type} value={type}>{assetTypeLabels[type]}</option>)}
      </select>
    </label>

    <label className="experiment-field">
      <span>被测变量（只列可枚举的特征；自由文本没有可对齐的取值，分不了组）</span>
      <select value={variableKey} onChange={event => pickVariable(event.target.value)}>
        <option value="">请选择</option>
        {fields.map(field => <option key={field.key} value={field.key}>{field.label}（{field.group}）</option>)}
      </select>
    </label>
    {!fields.length ? <div className="experiment-hint">
      <CircleAlert size={14}/>
      <span>「{assetTypeLabels[assetType]}」这套特征体系里没有可枚举的字段，这个类型现在做不了实验。</span>
    </div> : null}

    {variableKey ? <div className="experiment-field">
      <span>要控住的其他特征（各组不一致不会被拦，但入组时会给黄牌）</span>
      <div className="experiment-checks">
        {fields.filter(field => field.key !== variableKey).map(field => <label key={field.key}>
          <input type="checkbox" checked={controlledKeys.includes(field.key)} onChange={() => toggleControlled(field.key)}/>
          {field.label}
        </label>)}
      </div>
    </div> : null}

    <div className="experiment-field-row">
      <label className="experiment-field">
        <span>每组最低展示量</span>
        <input type="number" min={1} value={minImpressions} onChange={event => setMinImpressions(event.target.value)}/>
      </label>
      <label className="experiment-field">
        <span>观察窗口起</span>
        <input type="date" value={windowStart} onChange={event => setWindowStart(event.target.value)}/>
      </label>
      <label className="experiment-field">
        <span>观察窗口止</span>
        <input type="date" value={windowEnd} onChange={event => setWindowEnd(event.target.value)}/>
      </label>
    </div>

    <div className="experiment-field">
      <span>分组（至少两组，恰好一组是基线。只有一组就没有对照，也就没有实验）</span>
      {variableField?.kind === 'enum_multi' ? <p className="experiment-field-note">
        「{variableField.label}」一条素材可以同时占多个取值。一组要的是<b>完整</b>的那一套：
        素材同时是「问题」和「反差」时，它属于「反差、问题」这一组，不属于只勾了「问题」的那一组。
      </p> : null}
      <div className="experiment-variant-drafts">
        {variants.map(variant => <div className="experiment-variant-draft" key={variant.key}>
          <input aria-label="分组名称" value={variant.name}
            onChange={event => updateVariant(variant.key, { name: event.target.value })}/>
          {variableField?.kind === 'enum_multi' && variableField.vocabulary?.length
            ? <div className="experiment-variant-terms" role="group" aria-label="这一组的变量取值">
                {variableField.vocabulary.map(term => {
                  const picked = splitTerms(variant.variable_value)
                  const on = picked.includes(term)
                  return <label key={term} className={on ? 'on' : undefined}>
                    <input type="checkbox" checked={on} onChange={() => updateVariant(variant.key, {
                      variable_value: composeTerms(on ? picked.filter(item => item !== term) : [...picked, term]),
                    })}/>
                    {term}
                  </label>
                })}
              </div>
            : variableField?.vocabulary?.length
            ? <select aria-label="这一组的变量取值" value={variant.variable_value}
                onChange={event => updateVariant(variant.key, { variable_value: event.target.value })}>
                <option value="">取值</option>
                {variableField.vocabulary.map(term => <option key={term} value={term}>{term}</option>)}
              </select>
            : <input aria-label="这一组的变量取值" value={variant.variable_value} placeholder={variableField ? `${variableField.label}取值` : '先选被测变量'}
                disabled={!variableField}
                onChange={event => updateVariant(variant.key, { variable_value: event.target.value })}/>}
          <label className="experiment-baseline-pick">
            <input type="radio" name="experiment-baseline" checked={Boolean(variant.is_baseline)}
              onChange={() => pickBaseline(variant.key)}/>
            基线
          </label>
          <button className="text-button" disabled={variants.length <= 2}
            onClick={() => removeVariant(variant.key)}>移除</button>
        </div>)}
      </div>
      <button className="secondary-button" disabled={variants.length >= 8} onClick={addVariant}>加一组</button>
    </div>

    {notice ? <div className="inline-notice" role="status">{notice}</div> : null}

    <div className="experiment-form-actions">
      <button className="primary-button" disabled={saving} onClick={() => { void submit() }}>
        {saving ? '创建中…' : '创建为草稿'}
      </button>
      <button className="secondary-button" onClick={onCancel}>取消</button>
    </div>
    <p className="experiment-form-foot">
      创建出来是草稿。草稿阶段才能往各组里放素材；点「开跑」之后分组就锁死了。
    </p>
  </div>
}

function offsetDate(days: number): string {
  const value = new Date(Date.now() + days * 24 * 60 * 60 * 1000)
  return `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, '0')}-${String(value.getDate()).padStart(2, '0')}`
}
