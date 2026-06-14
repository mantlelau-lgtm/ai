import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'

import { OverviewPage } from '@/pages/OverviewPage'
import { useAdminStore } from '@/store/useAdminStore'
import { createEmptyBundle } from '@/types'

afterEach(() => {
  useAdminStore.setState({
    loading: false,
    saving: false,
    error: '',
    notice: '',
    overview: null,
    validation: null,
    bundle: createEmptyBundle(),
    agents: [],
    storage: { engine: 'postgresql', database: '' },
  })
})

describe('OverviewPage', () => {
  it('renders service statuses and database summary', () => {
    useAdminStore.setState({
      overview: {
        services: [
          {
            name: 'message-gateway',
            status: 'ok',
            detail: '{"status":"ok"}',
            url: 'http://localhost:50082/admin/healthz',
          },
        ],
        database: {
          engine: 'postgresql',
          database: 'admin_console',
          status: 'ok',
          detail: 'database connected',
        },
        tables: {
          admin_bots: {
            name: 'admin_bots',
            rows: 2,
            updated_at: null,
          },
        },
        summary: {
          bot_count: 2,
          provider_count: 1,
          model_count: 3,
          agent_count: 2,
          message_rule_count: 0,
        },
      },
      storage: {
        engine: 'postgresql',
        database: 'admin_console',
      },
    })

    render(<OverviewPage />)

    expect(screen.getByText('服务状态')).toBeInTheDocument()
    expect(screen.getByText('message-gateway')).toBeInTheDocument()
    expect(screen.getByText('admin_console')).toBeInTheDocument()
    expect(screen.getByText('admin_bots')).toBeInTheDocument()
    expect(screen.getByText('Bot 数量')).toBeInTheDocument()
  })
})
