import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api } from '../data/api'

export type ModelProviderId = 'ark'
export type ModelProviderStatus = '未配置' | '已配置'

export interface ModelProviderConfig {
  id: ModelProviderId
  name: string
  description: string
  status: ModelProviderStatus
  lastVerifiedAt?: string
}

interface ModelConfigValue {
  providers: ModelProviderConfig[]
  configuredCount: number
  isLoading: boolean
  refresh: () => Promise<void>
}

const ModelConfigContext = createContext<ModelConfigValue | null>(null)

export function ModelConfigProvider({ children }: { children: ReactNode }) {
  const [providers, setProviders] = useState<ModelProviderConfig[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const refresh = useCallback(async () => {
    setIsLoading(true)
    try {
      const capabilities = await api.getCapabilities()
      const models = capabilities.capabilities.map(item => `${item.capability}: ${item.model}`).join('；')
      setProviders([{
        id: 'ark',
        name: '火山方舟',
        description: models,
        status: capabilities.status === 'configured' ? '已配置' : '未配置',
        lastVerifiedAt: capabilities.checkedAt,
      }])
    } catch {
      setProviders([{ id: 'ark', name: '火山方舟', description: '无法连接本地 MVP API', status: '未配置' }])
    } finally {
      setIsLoading(false)
    }
  }, [])
  useEffect(() => { void refresh() }, [refresh])
  const value = useMemo(() => ({ providers, configuredCount: providers.filter(provider => provider.status === '已配置').length, isLoading, refresh }), [providers, isLoading, refresh])
  return <ModelConfigContext.Provider value={value}>{children}</ModelConfigContext.Provider>
}

export function useModelConfig() {
  const value = useContext(ModelConfigContext)
  if (!value) throw new Error('useModelConfig must be used inside ModelConfigProvider')
  return value
}
