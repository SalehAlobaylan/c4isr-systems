import { XIcon } from '@/components/icons'
import type { ScopeInput } from '@/lib/api'
import type { AtlasLayer, Camera } from '../AtlasCanvas'

export interface MapBookmark {
  id: string
  name: string
  camera: Camera
  region?: string
  publicView?: string
  area?: string
  scope?: ScopeInput
  layers: AtlasLayer[]
}

interface SavedViewsPanelProps {
  bookmarkName: string
  bookmarks: MapBookmark[]
  onBookmarkNameChange: (value: string) => void
  onSaveView: () => void
  onRestoreBookmark: (bookmark: MapBookmark) => void
  onDeleteBookmark: (id: string) => void
  onCopyShareLink: () => void
}

export function SavedViewsPanel({
  bookmarkName,
  bookmarks,
  onBookmarkNameChange,
  onSaveView,
  onRestoreBookmark,
  onDeleteBookmark,
  onCopyShareLink,
}: SavedViewsPanelProps) {
  return (
    <div className="map-drawer-pane">
      <form
        className="map-card"
        onSubmit={(e) => {
          e.preventDefault()
          onSaveView()
        }}
      >
        <div className="map-panel-section-head">
          <h3>Save current view</h3>
        </div>
        <label className="block">
          <span>Name this map view</span>
          <input
            className="atlas-field"
            maxLength={80}
            value={bookmarkName}
            placeholder="e.g. Jeddah March 2025 imagery"
            onChange={(e) => onBookmarkNameChange(e.target.value)}
            required
          />
        </label>
        <div>
          <button className="ux-button" data-primary="true" type="submit">
            Save current view
          </button>
        </div>
        <p className="ux-muted">
          Private to this account and workspace. Settings only, never imagery or credentials.
        </p>
      </form>

      <div className="map-panel-section">
        <div className="map-panel-section-head">
          <h3>Saved views ({bookmarks.length})</h3>
        </div>
        {bookmarks.length === 0 ? (
          <p className="ux-muted">No saved views yet. Save the current scope, camera and visible layers above.</p>
        ) : (
          <div className="space-y-2">
            {bookmarks.map((b) => (
              <article className="atlas-bookmark items-center" key={b.id}>
                <button
                  type="button"
                  className="ux-button flex-1 justify-start text-left"
                  onClick={() => onRestoreBookmark(b)}
                >
                  {b.name}
                </button>
                <button
                  type="button"
                  className="map-drawer-iconbtn"
                  aria-label={`Delete saved view ${b.name}`}
                  onClick={() => onDeleteBookmark(b.id)}
                >
                  <XIcon className="size-4" />
                </button>
              </article>
            ))}
          </div>
        )}
      </div>

      <div className="map-inset-card">
        <div className="map-panel-section-head">
          <h3>Share authorized view</h3>
        </div>
        <div>
          <button type="button" className="ux-button" onClick={onCopyShareLink}>
            Copy view link
          </button>
        </div>
        <p className="ux-muted">
          The link includes area, dates, camera and visible layers. Recipients need their own grants; drafts stay
          private.
        </p>
      </div>
    </div>
  )
}
