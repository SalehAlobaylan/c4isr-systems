/* Must come first: it fixes the cascade layer order for the whole document. */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from '@tanstack/react-router'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

/*
 * Approved typefaces, bundled locally rather than fetched at runtime.
 *
 * IBM Plex Sans / Sans Arabic / Mono are SIL Open Font License 1.1, and shipping
 * the files with the application means operator legibility never depends on a
 * third-party CDN being reachable (DESIGN.md §5.1). Only the subsets and weights
 * the interface actually uses are imported.
 */
import '@fontsource/ibm-plex-sans/latin-400.css'
import '@fontsource/ibm-plex-sans/latin-500.css'
import '@fontsource/ibm-plex-sans/latin-600.css'
import '@fontsource/ibm-plex-sans/latin-700.css'
import '@fontsource/ibm-plex-sans-arabic/arabic-400.css'
import '@fontsource/ibm-plex-sans-arabic/arabic-500.css'
import '@fontsource/ibm-plex-sans-arabic/arabic-600.css'
import '@fontsource/ibm-plex-mono/latin-400.css'
import '@fontsource/ibm-plex-mono/latin-500.css'
import '@fontsource/ibm-plex-mono/latin-600.css'

import { RealtimeProvider } from '@/realtime/RealtimeProvider'
import { router } from '@/router'

/*
 * The one stylesheet. It imports the design tokens, the shell and component
 * vocabulary, and the board and picture stylesheets, and it fixes the cascade
 * layer order. See `src/index.css`.
 */
import '@/index.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 10_000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
})

const rootElement = document.getElementById('root')
if (!rootElement) throw new Error('Root element #root not found')

createRoot(rootElement).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RealtimeProvider>
        <RouterProvider router={router} />
      </RealtimeProvider>
    </QueryClientProvider>
  </StrictMode>,
)
