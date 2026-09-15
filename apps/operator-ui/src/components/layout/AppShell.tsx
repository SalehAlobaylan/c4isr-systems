import { useQuery } from '@tanstack/react-query'
import { Outlet } from '@tanstack/react-router'

import { Sidebar } from '@/components/layout/Sidebar'
import { TopBar } from '@/components/layout/TopBar'
import { ToastViewport } from '@/components/ui/toast'
import { api } from '@/lib/api'
import { queryKeys } from '@/lib/queryKeys'

const ACTIVE_ALERT_FILTER = { state: 'ACTIVE', limit: 1 }

export function AppShell() {
  const alertsQuery = useQuery({
    queryKey: queryKeys.alerts.list(ACTIVE_ALERT_FILTER),
    queryFn: () => api.listAlerts(ACTIVE_ALERT_FILTER),
  })

  return (
    <div className="flex h-screen overflow-hidden bg-bg text-ink">
      <Sidebar alertCount={alertsQuery.data?.total} />
      <div className="flex min-w-0 flex-1 flex-col">
        <TopBar />
        <main className="min-h-0 flex-1 overflow-hidden">
          <Outlet />
        </main>
      </div>
      <ToastViewport />
    </div>
  )
}
