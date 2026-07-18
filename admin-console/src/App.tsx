import { useEffect } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'

import { AppShell } from '@/components/AppShell'
import { AgentsPage } from '@/pages/AgentsPage'
import { BotsPage } from '@/pages/BotsPage'
import { LlmKeysPage } from '@/pages/LlmKeysPage'
import { LlmModelsPage } from '@/pages/LlmModelsPage'
import { OverviewPage } from '@/pages/OverviewPage'
import { useAdminStore } from '@/store/useAdminStore'

export default function App() {
  const hydrate = useAdminStore((state) => state.hydrate)

  useEffect(() => {
    void hydrate()
  }, [hydrate])

  return (
    <BrowserRouter>
      <AppShell>
        <Routes>
          <Route path="/" element={<OverviewPage />} />
          <Route path="/bots" element={<BotsPage />} />
          <Route path="/llm" element={<Navigate to="/llm/models" replace />} />
          <Route path="/llm/models" element={<LlmModelsPage />} />
          <Route path="/llm/keys" element={<LlmKeysPage />} />
          <Route path="/agents" element={<AgentsPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AppShell>
    </BrowserRouter>
  )
}
