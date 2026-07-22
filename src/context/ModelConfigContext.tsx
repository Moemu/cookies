import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react'

export type ModelProviderId = 'openai' | 'anthropic' | 'gemini' | 'volcengine' | 'custom'
export type ModelProviderStatus = '未配置' | '已配置' | '校验失败'

export interface ModelProviderConfig {
  id: ModelProviderId
  name: string
  description: string
  endpoint: string
  defaultModel: string
  status: ModelProviderStatus
  keyHint?: string
  lastVerifiedAt?: string
}

interface SaveProviderInput {
  endpoint: string
  defaultModel: string
  apiKey?: string
}

interface ModelConfigValue {
  providers: ModelProviderConfig[]
  configuredCount: number
  saveProvider: (id: ModelProviderId, input: SaveProviderInput) => ModelProviderConfig
  verifyProvider: (id: ModelProviderId) => boolean
  removeProvider: (id: ModelProviderId) => void
}

const STORAGE_KEY = 'cookies.model-provider-metadata.v1'

const seedProviders: ModelProviderConfig[] = [
  { id: 'openai', name: 'OpenAI', description: '文本、推理、图像与多模态模型', endpoint: 'https://api.openai.com/v1', defaultModel: 'gpt-5', status: '未配置' },
  { id: 'anthropic', name: 'Anthropic', description: '长文本、策略分析与内容评审', endpoint: 'https://api.anthropic.com', defaultModel: 'claude-sonnet-4', status: '未配置' },
  { id: 'gemini', name: 'Google Gemini', description: '多模态理解与素材分析', endpoint: 'https://generativelanguage.googleapis.com', defaultModel: 'gemini-2.5-pro', status: '未配置' },
  { id: 'volcengine', name: '火山引擎 / 豆包', description: '中文生成、图像与视频能力', endpoint: 'https://ark.cn-beijing.volces.com/api/v3', defaultModel: 'doubao-seed-1-6', status: '未配置' },
  { id: 'custom', name: 'OpenAI 兼容服务', description: '私有部署或兼容 OpenAI 协议的模型服务', endpoint: '', defaultModel: '', status: '未配置' },
]

function loadProviderMetadata() {
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY)
    if (!stored) return seedProviders
    const metadata = JSON.parse(stored) as Partial<ModelProviderConfig>[]
    return seedProviders.map(provider => ({ ...provider, ...metadata.find(item => item.id === provider.id), description: provider.description }))
  } catch {
    return seedProviders
  }
}

const ModelConfigContext = createContext<ModelConfigValue | null>(null)

export function ModelConfigProvider({ children }: { children: ReactNode }) {
  const [providers, setProviders] = useState<ModelProviderConfig[]>(loadProviderMetadata)

  const persist = useCallback((next: ModelProviderConfig[]) => {
    setProviders(next)
    // 前端 Mock 只保存连接元数据。API Key 明文必须由生产环境的后端密钥服务接收与保存。
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(next))
  }, [])

  const saveProvider = useCallback((id: ModelProviderId, input: SaveProviderInput) => {
    const current = providers.find(provider => provider.id === id) ?? seedProviders[0]
    const saved: ModelProviderConfig = {
      ...current,
      endpoint: input.endpoint.trim(),
      defaultModel: input.defaultModel.trim(),
      status: input.apiKey || current.keyHint ? '已配置' : '未配置',
      keyHint: input.apiKey ? `••••••••${input.apiKey.slice(-4)}` : current.keyHint,
      lastVerifiedAt: input.apiKey || current.keyHint ? '2026-07-22 17:30' : undefined,
    }
    persist(providers.map(provider => provider.id === id ? saved : provider))
    return saved
  }, [persist, providers])

  const verifyProvider = useCallback((id: ModelProviderId) => {
    const provider = providers.find(item => item.id === id)
    if (!provider?.keyHint || !provider.endpoint || !provider.defaultModel) return false
    persist(providers.map(item => item.id === id ? { ...item, status: '已配置', lastVerifiedAt: '2026-07-22 17:31' } : item))
    return true
  }, [persist, providers])

  const removeProvider = useCallback((id: ModelProviderId) => {
    persist(providers.map(provider => provider.id === id ? { ...provider, status: '未配置', keyHint: undefined, lastVerifiedAt: undefined } : provider))
  }, [persist, providers])

  const value = useMemo(() => ({ providers, configuredCount: providers.filter(provider => provider.status === '已配置').length, saveProvider, verifyProvider, removeProvider }), [providers, removeProvider, saveProvider, verifyProvider])
  return <ModelConfigContext.Provider value={value}>{children}</ModelConfigContext.Provider>
}

export function useModelConfig() {
  const value = useContext(ModelConfigContext)
  if (!value) throw new Error('useModelConfig must be used inside ModelConfigProvider')
  return value
}
