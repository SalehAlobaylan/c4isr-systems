import { Outlet } from '@tanstack/react-router'

import { ChainRibbon } from '@/components/layout/ChainRibbon'
import { TopBar } from '@/components/layout/TopBar'
import { ToastViewport } from '@/components/ui/toast'

/**
 * The shell.
 *
 * The operator's identity is read once, by the header, which is the only
 * place that shows it. The footer used to fetch the same session a second
 * time to print the role and the id again; two owners for one fact, and a
 * duplicate request on every page.
 */
export function AppShell() {
  return (
    <div className="aegis-app">
      <TopBar />
      <main className="aegis-main">
        <Outlet />
      </main>
      <footer className="aegis-pagefoot">
        <ChainRibbon />
        <span className="aegis-foot-op">C4ISR Operational Awareness</span>
      </footer>
      <ToastViewport />
    </div>
  )
}
