import { Outlet } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'

import { TopBar } from '@/components/layout/TopBar'
import { ToastViewport } from '@/components/ui/toast'
import { API_TOKEN, api } from '@/lib/api'
import { queryKeys } from '@/lib/queryKeys'

export function AppShell() {
  const sessionQuery = useQuery({
    queryKey: queryKeys.currentOperator(),
    queryFn: () => api.getCurrentOperator(),
    retry: false,
    staleTime: 5 * 60_000,
  })
  const operatorID = sessionQuery.data?.operator?.id ?? 'not authenticated'
  const operatorRole = sessionQuery.data?.operator?.role ?? (API_TOKEN ? 'checking' : 'token required')

  return (
    <div className="aegis-app">
      <TopBar />
      <main className="aegis-main">
        <Outlet />
      </main>
      <footer className="aegis-pagefoot">
        <span dir="ltr">
          EMAD | <bdi lang="ar" dir="rtl">عِماد</bdi> · C4ISR operational awareness
        </span>
        <span>Sources → observations → tracks → picture → decision → command</span>
        <span className="aegis-foot-op">Role {operatorRole} · {operatorID.toUpperCase()}</span>
      </footer>
      <ToastViewport />
    </div>
  )
}
