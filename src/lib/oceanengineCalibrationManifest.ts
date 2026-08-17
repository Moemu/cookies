import manifestSource from '../../docs/delivery/fixtures/oceanengine-calibration-manifest-v1.json'

type ManifestField = {
  key: string
  unit?: string
  page_family: string
  locator?: { value?: string }
  computer_use?: { scope?: { value?: string }; target?: { value?: string }; blocked_state?: string; reason?: string; input_constraints?: Record<string, unknown> }
  condition?: string
  condition_state?: 'evaluable' | 'dependency_only'
  evidence_state?: string
}

type ConsumerMapping = {
  field_key: string
  destination: string
  treatment: 'modelled' | 'dynamic_reference' | 'evidence_only' | 'blocked'
  contract_path: string
}

type ManifestSource = {
  fields: ManifestField[]
  consumer_mappings: ConsumerMapping[]
}

export type VisibleManifestField = {
  key: string
  label: string
  unit?: string
  value: unknown
}

export type CalibrationDisposition = {
  key: string
  label: string
  pageFamily: string
  treatment: ConsumerMapping['treatment']
  state: 'ready' | 'evidence_only' | 'blocked' | 'platform_pending' | 'condition_unmet' | 'missing_value'
  reason: string
}

const manifest = manifestSource as ManifestSource

function toSnakeCase(value: string) {
  return value.replace(/([a-z0-9])([A-Z])/g, '$1_$2').replace(/\[\]$/, '').toLowerCase()
}

function valuesAtPath(root: unknown, contractPath: string) {
  const segments = contractPath.split('.').slice(1)
  return segments.reduce<unknown[]>((values, segment) => values.flatMap(value => {
    if (!value || typeof value !== 'object') return []
    const next = (value as Record<string, unknown>)[toSnakeCase(segment)]
    return Array.isArray(next) ? next : next === undefined || next === null ? [] : [next]
  }), [root])
}

function conditionMatches(condition: string | undefined, facts: Record<string, unknown>) {
  if (!condition) return true
  return condition.split(' and ').every(clause => {
    const equals = clause.match(/^([a-z_]+) == ([a-z_]+)$/)
    if (equals) return facts[equals[1]] === equals[2]
    const inSet = clause.match(/^([a-z_]+) in \[([a-z_,]+)\]$/)
    if (inSet) return inSet[2].split(',').includes(String(facts[inSet[1]] ?? ''))
  const notInSet = clause.match(/^([a-z_]+) not in \[([a-z_,]+)\]$/)
    if (notInSet) {
      const value = facts[notInSet[1]]
      return value !== undefined && value !== null && !notInSet[2].split(',').includes(String(value))
    }
    if (/^[a-z_]+$/.test(clause)) return Boolean(facts[clause])
    return false
  })
}

function configurationFacts(configuration: unknown) {
  const project = valuesAtPath(configuration, 'OceanEngineConfiguration.Project')[0] as Record<string, unknown> | undefined
  const budget = project?.budget_and_bidding as Record<string, unknown> | undefined
  return {
    marketing_purpose: project?.marketing_purpose,
    marketing_scenario: project?.marketing_scenario,
    carrier: project?.carrier,
    delivery_mode: project?.delivery_mode,
    bidding_strategy: budget?.bidding_strategy,
  }
}

function fieldLabel(field: ManifestField) {
  const locator = field.locator?.value ?? ''
  const target = field.computer_use?.target?.value?.replace(/^button:/, '')
  if (locator.startsWith('button:') && target) return target
  return field.computer_use?.scope?.value || locator || field.key
}

function disposition(field: ManifestField, mapping: ConsumerMapping, state: CalibrationDisposition['state'], reason: string): CalibrationDisposition {
  return { key: field.key, label: fieldLabel(field), pageFamily: field.page_family, treatment: mapping.treatment, state, reason }
}

export function visibleOceanEngineManifestFields(configuration: unknown, scope: 'project' | 'promotion', promotion?: unknown) {
  const facts = configurationFacts(configuration)
  return manifest.consumer_mappings
    .filter(mapping => mapping.destination === 'OceanEngineConfiguration' && (mapping.treatment === 'modelled' || mapping.treatment === 'dynamic_reference'))
    .flatMap(mapping => {
      const field = manifest.fields.find(candidate => candidate.key === mapping.field_key)
      if (!field || field.condition_state === 'dependency_only' || !conditionMatches(field.condition, facts)) return []
      const mappingScope = mapping.field_key.startsWith('project.') ? 'project' : 'promotion'
      if (mappingScope !== scope) return []
      const path = scope === 'promotion' && promotion
        ? mapping.contract_path.replace('OceanEngineConfiguration.Promotions[].', 'OceanEngineConfiguration.')
        : mapping.contract_path
      const values = valuesAtPath(scope === 'promotion' && promotion ? promotion : configuration, path)
      return values.length ? [{ key: field.key, label: fieldLabel(field), unit: field.unit, value: values.length === 1 ? values[0] : values }] : []
    })
}

export function oceanEngineCalibrationDispositions(configuration: unknown, scope: 'project' | 'promotion'): CalibrationDisposition[] {
  const facts = configurationFacts(configuration)
  return manifest.consumer_mappings
    .filter(mapping => mapping.destination === 'OceanEngineConfiguration')
    .flatMap<CalibrationDisposition>(mapping => {
      const field = manifest.fields.find(candidate => candidate.key === mapping.field_key)
      if (!field || (mapping.field_key.startsWith('project.') ? 'project' : 'promotion') !== scope) return []
      const values = valuesAtPath(configuration, mapping.contract_path)
      const blockedReason = field.computer_use?.reason
        ?? (field.computer_use?.blocked_state ? `稳定阻断状态：${field.computer_use.blocked_state}` : undefined)
        ?? (field.computer_use?.input_constraints ? '当前页面输入约束不允许生成可执行配置。' : undefined)
        ?? field.condition
        ?? '冻结 Manifest 未提供可执行处置。'
      if (mapping.treatment === 'evidence_only') return [disposition(field, mapping, 'evidence_only', `仅保留校准证据（${field.evidence_state ?? 'unknown'}）。`)]
      if (mapping.treatment === 'blocked') return [disposition(field, mapping, 'blocked', blockedReason)]
      if (field.condition_state === 'dependency_only') return [disposition(field, mapping, 'platform_pending', blockedReason)]
      if (!conditionMatches(field.condition, facts)) return [disposition(field, mapping, 'condition_unmet', field.condition ? `当前条件不满足：${field.condition}` : '当前路径条件无法确认。')]
      if (!values.length) return [disposition(field, mapping, 'missing_value', '当前计划未提供该字段的稳定值。')]
      return [disposition(field, mapping, 'ready', '当前条件满足，且配置值已存在。')]
    })
}
