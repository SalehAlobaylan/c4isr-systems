import { Outlet } from '@tanstack/react-router'

import { TopBar } from '@/components/layout/TopBar'
import { ToastViewport } from '@/components/ui/toast'
import { OPERATOR_ID } from '@/lib/api'

export function AppShell() {
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
        <span className="aegis-foot-op">Role operator · {OPERATOR_ID.toUpperCase()}</span>
      </footer>
      <ToastViewport />
    </div>
  )
}
