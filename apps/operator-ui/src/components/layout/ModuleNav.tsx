import { Link } from '@tanstack/react-router'
import { useWorkspace } from '@/lib/workspaceContext'
import { SessionLogin } from '@/components/SessionLogin'

/** Product capabilities stay visible alongside the existing exercise views. */
export function ModuleNav() {
  const { session } = useWorkspace()
  const publicContext = session.resource.partition === 'PUBLIC_EVIDENCE'
  const author = session.permissions.includes('sim.scenarios.author')
  const can = (permission: string) => session.permissions.includes(permission)

  return <nav className="emad-modules" aria-label="EMAD modules">
    <div className="emad-module-links">
    <Link to="/" activeOptions={{ exact: true }} activeProps={{ 'aria-current': 'page' }}>Home</Link>
    <Link to="/simulation-studio" activeProps={{ 'aria-current': 'page' }}>Simulation Studio</Link>
    {publicContext ? <>
      {can('sources.read') && <Link to="/intelligence" activeProps={{ 'aria-current': 'page' }}>Imagery & assessments</Link>}
      {can('sources.read') && <Link to="/weather-reports" activeProps={{ 'aria-current': 'page' }}>Weather & reports</Link>}
      {can('tracks.read') && <Link to="/external-tracks" activeProps={{ 'aria-current': 'page' }}>Moving tracks</Link>}
    </> : <>
      {author && <Link to="/simulation-assets" activeProps={{ 'aria-current': 'page' }}>3D assets</Link>}
      <Link to="/recordings" search={{ run: undefined, ms: 0, sequence: 0 }} activeProps={{ 'aria-current': 'page' }}>Recording & replay</Link>
    </>}
    </div>
    <SessionLogin compact />
  </nav>
}
