import { create } from 'zustand'

import type {
  BotConfig,
  BundlePayload,
  DiffResponse,
  LlmKey,
  LlmModel,
  LlmProvider,
  OverviewResponse,
  RoutingEntry,
  ValidationResponse,
} from '@/types'
import { createEmptyBundle } from '@/types'
import { adminApi } from '@/utils/api'

type AdminState = {
  loading: boolean
  saving: boolean
  error: string
  overview: OverviewResponse | null
  bundle: BundlePayload
  validation: ValidationResponse | null
  diff: DiffResponse | null
  storage: {
    engine: string
    database: string
  }
  hydrate: () => Promise<void>
  updateBots: (updater: (bots: BotConfig[]) => BotConfig[]) => void
  updateKeys: (updater: (keys: LlmKey[]) => LlmKey[]) => void
  updateProviders: (updater: (providers: LlmProvider[]) => LlmProvider[]) => void
  updateModels: (updater: (models: LlmModel[]) => LlmModel[]) => void
  updateRoutes: (updater: (routes: RoutingEntry[]) => RoutingEntry[]) => void
  updateDefaultAgent: (value: string) => void
  validateDraft: () => Promise<void>
  previewDiff: () => Promise<void>
  applyDraft: () => Promise<void>
  clearError: () => void
}

export const useAdminStore = create<AdminState>((set, get) => ({
  loading: false,
  saving: false,
  error: '',
  overview: null,
  bundle: createEmptyBundle(),
  validation: null,
  diff: null,
  storage: {
    engine: 'postgresql',
    database: '',
  },
  hydrate: async () => {
    set({ loading: true, error: '' })
    try {
      const [overview, bundleResponse] = await Promise.all([adminApi.getOverview(), adminApi.getBundle()])
      set({
        loading: false,
        overview,
        bundle: bundleResponse.data,
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
  updateKeys: (updater) =>
    set((state) => ({
      bundle: {
        ...state.bundle,
        llm: { ...state.bundle.llm, keys: updater(state.bundle.llm.keys) },
      },
    })),
  updateProviders: (updater) =>
    set((state) => ({
      bundle: {
        ...state.bundle,
        llm: {
          ...state.bundle.llm,
          providers: updater(state.bundle.llm.providers),
        },
      },
    })),
  updateModels: (updater) =>
    set((state) => ({
      bundle: {
        ...state.bundle,
        llm: { ...state.bundle.llm, models: updater(state.bundle.llm.models) },
      },
    })),
  updateRoutes: (updater) =>
    set((state) => ({
      bundle: {
        ...state.bundle,
        routing: { ...state.bundle.routing, bots: updater(state.bundle.routing.bots) },
      },
    })),
  updateDefaultAgent: (value) =>
    set((state) => ({
      bundle: {
        ...state.bundle,
        routing: { ...state.bundle.routing, default_agent: value },
      },
    })),
  validateDraft: async () => {
    set({ loading: true, error: '' })
    try {
      const validation = await adminApi.validate(get().bundle)
      set({ validation, loading: false })
    } catch (error) {
      set({
        loading: false,
        error: error instanceof Error ? error.message : '校验失败',
      })
    }
  },
  previewDiff: async () => {
    set({ loading: true, error: '' })
    try {
      const diff = await adminApi.diff(get().bundle)
      set({ diff, validation: diff.validation, loading: false })
    } catch (error) {
      set({
        loading: false,
        error: error instanceof Error ? error.message : 'Diff 预览失败',
      })
    }
  },
  applyDraft: async () => {
    set({ saving: true, error: '' })
    try {
      const result = await adminApi.apply(get().bundle)
      const overview = await adminApi.getOverview()
      set({
        saving: false,
        validation: result.validation,
        overview,
      })
    } catch (error) {
      set({
        saving: false,
        error: error instanceof Error ? error.message : '保存失败',
      })
    }
  },
  clearError: () => set({ error: '' }),
}))
