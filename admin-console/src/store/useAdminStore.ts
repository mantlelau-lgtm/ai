import { create } from 'zustand'

import type {
  BotConfig,
  BundlePayload,
  LlmCredential,
  LlmModel,
  OverviewResponse,
  RegisteredAgent,
  ValidationResponse,
} from '@/types'
import { createEmptyBundle } from '@/types'
import { adminApi } from '@/utils/api'

type AdminState = {
  loading: boolean
  saving: boolean
  error: string
  notice: string
  overview: OverviewResponse | null
  bundle: BundlePayload
  agents: RegisteredAgent[]
  validation: ValidationResponse | null
  storage: {
    engine: string
    database: string
  }
  hydrate: () => Promise<void>
  updateBots: (updater: (bots: BotConfig[]) => BotConfig[]) => void
  updateCredentials: (updater: (credentials: LlmCredential[]) => LlmCredential[]) => void
  updateModels: (updater: (models: LlmModel[]) => LlmModel[]) => void
  saveDraft: (message?: string) => Promise<boolean>
  saveModelCard: (model: LlmModel, originalModelId?: string) => Promise<boolean>
  deleteModelCard: (modelId: string) => Promise<boolean>
  saveCredentialCard: (credential: LlmCredential, originalKeyName?: string) => Promise<boolean>
  deleteCredentialCard: (keyName: string) => Promise<boolean>
  updateAgentKey: (name: string, keyName: string) => Promise<boolean>
  clearError: () => void
  clearNotice: () => void
}

export const useAdminStore = create<AdminState>((set, get) => ({
  loading: false,
  saving: false,
  error: '',
  notice: '',
  overview: null,
  bundle: createEmptyBundle(),
  agents: [],
  validation: null,
  storage: {
    engine: 'postgresql',
    database: '',
  },
  hydrate: async () => {
    set({ loading: true, error: '' })
    try {
      const [overview, bundleResponse, agentsResponse] = await Promise.all([
        adminApi.getOverview(),
        adminApi.getBundle(),
        adminApi.listAgents(),
      ])
      set({
        loading: false,
        overview,
        bundle: bundleResponse.data,
        agents: agentsResponse.agents,
        storage: bundleResponse.storage,
      })
    } catch (error) {
      set({
        loading: false,
        error: error instanceof Error ? error.message : '初始化失败',
      })
    }
  },
  updateBots: (updater) =>
    set((state) => ({
      bundle: {
        ...state.bundle,
        bots: { bots: updater(state.bundle.bots.bots) },
      },
    })),
  updateCredentials: (updater) =>
    set((state) => ({
      bundle: {
        ...state.bundle,
        llm: { ...state.bundle.llm, credentials: updater(state.bundle.llm.credentials) },
      },
    })),
  updateModels: (updater) =>
    set((state) => ({
      bundle: {
        ...state.bundle,
        llm: { ...state.bundle.llm, models: updater(state.bundle.llm.models) },
      },
    })),
  saveDraft: async (message) => {
    set({ saving: true, error: '', notice: '' })
    try {
      const result = await adminApi.apply(get().bundle)
      const [overview, bundleResponse, agentsResponse] = await Promise.all([
        adminApi.getOverview(),
        adminApi.getBundle(),
        adminApi.listAgents(),
      ])
      set({
        saving: false,
        validation: result.validation,
        overview,
        bundle: bundleResponse.data,
        agents: agentsResponse.agents,
        storage: bundleResponse.storage,
        notice: message || '已保存并生效',
      })
      return true
    } catch (error) {
      set({
        saving: false,
        error: error instanceof Error ? error.message : '保存失败',
      })
      return false
    }
  },
  saveModelCard: async (model, originalModelId) => {
    set({ saving: true, error: '', notice: '' })
    try {
      if (originalModelId) {
        await adminApi.updateModel(originalModelId, model)
      } else {
        await adminApi.createModel(model)
      }
      await get().hydrate()
      set({ saving: false, notice: '模型已保存并生效' })
      return true
    } catch (error) {
      set({ saving: false, error: error instanceof Error ? error.message : '保存模型失败' })
      return false
    }
  },
  deleteModelCard: async (modelId) => {
    set({ saving: true, error: '', notice: '' })
    try {
      await adminApi.deleteModel(modelId)
      await get().hydrate()
      set({ saving: false, notice: '模型已删除并生效' })
      return true
    } catch (error) {
      set({ saving: false, error: error instanceof Error ? error.message : '删除模型失败' })
      return false
    }
  },
  saveCredentialCard: async (credential, originalKeyName) => {
    set({ saving: true, error: '', notice: '' })
    try {
      if (originalKeyName) {
        await adminApi.updateCredential(originalKeyName, credential)
      } else {
        await adminApi.createCredential(credential)
      }
      await get().hydrate()
      set({ saving: false, notice: '密钥已保存并生效' })
      return true
    } catch (error) {
      set({ saving: false, error: error instanceof Error ? error.message : '保存密钥失败' })
      return false
    }
  },
  deleteCredentialCard: async (keyName) => {
    set({ saving: true, error: '', notice: '' })
    try {
      await adminApi.deleteCredential(keyName)
      await get().hydrate()
      set({ saving: false, notice: '密钥已删除并生效' })
      return true
    } catch (error) {
      set({ saving: false, error: error instanceof Error ? error.message : '删除密钥失败' })
      return false
    }
  },
  clearError: () => set({ error: '' }),
  clearNotice: () => set({ notice: '' }),
  updateAgentKey: async (name, keyName) => {
    set({ saving: true, error: '', notice: '' })
    try {
      const result = await adminApi.updateAgentKey(name, keyName)
      set((state) => ({
        saving: false,
        notice: `Agent ${result.agent.name} 已绑定密钥 ${result.agent.key_name || '未设置'}`,
        agents: state.agents.map((item) => (item.name === result.agent.name ? result.agent : item)),
      }))
      return true
    } catch (error) {
      set({
        saving: false,
        error: error instanceof Error ? error.message : '保存 Agent 密钥失败',
      })
      return false
    }
  },
}))
