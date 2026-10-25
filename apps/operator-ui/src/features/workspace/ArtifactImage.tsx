import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useAPI } from '@/lib/api'
import { useWorkspace } from '@/lib/workspaceContext'

/** Authorized retained bytes only, shared within a session and revoked on unmount. */
export function ArtifactImage({ id, alt, className = '' }: { id?: string; alt: string; className?: string }) {
  const api = useAPI(), { prefix } = useWorkspace()
  const blob = useQuery({ queryKey: [...prefix, 'preview-artifact', id], queryFn: () => api.downloadArtifact(id!), enabled: !!id, staleTime: 60_000, gcTime: 0, retry: false })
  const [url, setURL] = useState('')
  useEffect(() => { if (!blob.data) { setURL(''); return }; const next = URL.createObjectURL(blob.data); setURL(next); return () => URL.revokeObjectURL(next) }, [blob.data])
  return url ? <img className={className} src={url} alt={alt} /> : <div className={`preview-placeholder artifact-image-state ${className}`} role="status">{!id ? 'No retained preview' : blob.isError ? 'Preview unavailable' : 'Loading image…'}</div>
}
