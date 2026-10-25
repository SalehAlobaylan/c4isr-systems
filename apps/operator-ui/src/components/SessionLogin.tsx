import { useEffect, useRef, useState } from 'react'
import { switchSession } from '@/lib/api'

export function SessionLogin({ compact = false }: { compact?: boolean }) {
  const popover=useRef<HTMLDetailsElement>(null)
  useEffect(()=>{const dismiss=(event:PointerEvent)=>{if(popover.current?.open&&!popover.current.contains(event.target as Node))popover.current.open=false};const escape=(event:KeyboardEvent)=>{if(event.key==='Escape'&&popover.current?.open){popover.current.open=false;popover.current.querySelector('summary')?.focus()}};document.addEventListener('pointerdown',dismiss);document.addEventListener('keydown',escape);return()=>{document.removeEventListener('pointerdown',dismiss);document.removeEventListener('keydown',escape)}},[])
  const [token, setToken] = useState(''), [error, setError] = useState('')
  const form = <form className="space-y-3" onSubmit={e => { e.preventDefault(); try { switchSession(token) } catch (failure) { setError(failure instanceof Error ? failure.message : 'Session could not be opened.') } }}>
    <label className="block text-sm font-medium">Account credential<input className="mt-1 min-h-11 w-full rounded border border-edge bg-canvas px-3" type="password" autoComplete="off" maxLength={4096} value={token} onChange={e => setToken(e.target.value)} required /></label>
    <p className="text-xs text-ink-muted">Use your issued credential. Switching accounts closes this session and clears its cached data.</p>
    {error && <p role="alert">{error}</p>}
    <button className="min-h-11 rounded border border-edge px-4 py-2" type="submit">Open account</button>
  </form>
  if (compact) return <details ref={popover} onToggle={e=>{if(!e.currentTarget.open)setToken('')}} className="relative ml-auto shrink-0"><summary className="min-h-11 cursor-pointer whitespace-nowrap rounded px-3 py-2">Account</summary><div className="absolute right-0 z-50 mt-2 w-72 max-w-[90vw] rounded border border-edge bg-panel p-4 shadow-xl">{form}</div></details>
  return <section className="mx-auto mt-6 w-full max-w-md rounded border border-edge bg-panel p-5" aria-label="Open an EMAD account"><h2 className="mb-3 text-lg font-semibold">Open your workspace</h2>{form}</section>
}
