import { useEffect } from 'react'
import { BrowserRouter, Route, Routes } from 'react-router-dom'

import { AppShell } from '@/components/AppShell'
import { BotsPage } from '@/pages/BotsPage'
import { LlmPage } from '@/pages/LlmPage'
import { OverviewPage } from '@/pages/OverviewPage'
import { PublishPage } from '@/pages/PublishPage'
import { RoutingPage } from '@/pages/RoutingPage'
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
          <Route path="/llm" element={<LlmPage />} />
          <Route path="/routing" element={<RoutingPage />} />
          <Route path="/publish" element={<PublishPage />} />
        </Routes>
      </AppShell>
    </BrowserRouter>
  )
}
