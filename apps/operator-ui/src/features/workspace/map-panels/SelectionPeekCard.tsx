import type { Feature, Geometry } from 'geojson'
import { CheckIcon, PlusIcon, XIcon } from '@/components/icons'
import type { ScopeSnapshot } from '@/lib/api'
import { layerInfo, type AtlasLayer, type ReferenceDrawing } from '../AtlasCanvas'
import { ReadRecords } from '../ReadRecords'
import { citationKey, type CaseCitationKind } from './caseCitationBridge'

interface SelectionPeekCardProps {
  selected: Feature[]
  pinnedPlace?: Feature
  referenceDrawing?: ReferenceDrawing
  canReadGeography: boolean
  canAttachToCase: boolean
  attachedKeys: string[]
  timeLabel: string
  readId?: string
  readScope?: ScopeSnapshot
  onClearSelection: () => void
  onSelectOverlapping: (id: string) => void
  onLocate: (feature: Feature) => void
  onInspect: () => void
  onShowPixels: (recordId: string) => void
  onAttachToCase: (citation: { kind: CaseCitationKind; id: string; label?: string }) => void
  onChooseAggregateRecord: (record: Record<string, unknown>) => void
  onClearPinnedPlace: () => void
  onClearReferenceDrawing: () => void
}

function citationForFeature(feature: Feature): { kind: CaseCitationKind; id: string; label?: string } | null {
  const p = feature.properties ?? {}
  if (p.aggregate) return null
  const label = String(p.title || p.name || p.recordId || '')
  if (p.kind === 'imagery' && p.recordId) return { kind: 'capture', id: String(p.recordId), label }
  if (p.kind === 'findings' && p.jobId) return { kind: 'analysis', id: String(p.jobId), label }
  if (p.dataset_id) return { kind: 'reference', id: String(p.dataset_id), label }
  if (p.evidenceId) return { kind: 'observation', id: String(p.evidenceId), label }
  if (p.recordId && ['earthquakes', 'thermal', 'weather', 'bulletins'].includes(String(p.kind))) {
    return { kind: 'observation', id: String(p.recordId), label }
  }
  return null
}

export function SelectionPeekCard({
  selected,
  pinnedPlace,
  referenceDrawing,
  canReadGeography,
  canAttachToCase,
  attachedKeys,
  timeLabel,
  readId,
  readScope,
  onClearSelection,
  onSelectOverlapping,
  onLocate,
  onInspect,
  onShowPixels,
  onAttachToCase,
  onChooseAggregateRecord,
  onClearPinnedPlace,
  onClearReferenceDrawing,
}: SelectionPeekCardProps) {
  const record = selected[0]

  // 1. Primary selected map feature (evidence, imagery, track, reference)
  if (record) {
    const props = record.properties ?? {}
    const kindKey = String(props.kind ?? '') as AtlasLayer
    const info = kindKey in layerInfo ? layerInfo[kindKey] : undefined
    const citation = citationForFeature(record)
    const alreadyAttached = citation ? attachedKeys.includes(citationKey(citation.kind, citation.id)) : false

    return (
      <section className="map-peek-card atlas-peek" aria-label="Selected map object">
        <header className="map-peek-head">
          <span className="map-peek-kind">
            {info && (
              <span style={{ color: `var(${info.token})` }} aria-hidden="true">
                {info.glyph}
              </span>
            )}
            <span>{info?.name ?? props.kind ?? 'Reference feature'}</span>
          </span>
          <button
            type="button"
            className="map-drawer-iconbtn"
            aria-label="Clear selected map object"
            onClick={onClearSelection}
          >
            <XIcon className="size-4" />
          </button>
        </header>

        <div className="map-peek-body">
          {selected.length > 1 && (
            <label className="block">
              <span className="ux-muted">Overlapping objects ({selected.length})</span>
              <select
                className="atlas-field mt-1"
                value={String(record.id ?? props.recordId ?? 0)}
                onChange={(e) => onSelectOverlapping(e.target.value)}
              >
                {selected.map((f, i) => (
                  <option key={i} value={String(f.id ?? f.properties?.recordId ?? 0)}>
                    {f.properties?.title ?? f.properties?.name ?? f.properties?.kind ?? 'Reference'}
                  </option>
                ))}
              </select>
            </label>
          )}

          <h2>
            <bdi dir="auto">{props.title || props.name || 'Retained reference feature'}</bdi>
          </h2>

          {props.temperature != null && (
            <p className="atlas-weather-value font-mono">
              {Number(props.temperature).toFixed(1)} °C{' '}
              <span className="ux-muted">
                Wind {props.wind == null ? 'unknown' : `${Number(props.wind).toFixed(1)} m/s`}
              </span>
            </p>
          )}

          {(kindKey === 'aircraft' || kindKey === 'vessels') && !props.aggregate && (
            <p className="atlas-weather-value font-mono">
              {props.onGround === true
                ? 'On ground'
                : props.altitude != null
                  ? `${Math.round(Number(props.altitude)).toLocaleString()} m`
                  : 'Alt unknown'}
              {' · '}
              {props.speedMps != null
                ? `${Math.round(Number(props.speedMps) * 1.94384)} kt`
                : 'Speed unknown'}
              {' · '}
              {props.course != null ? `${Math.round(Number(props.course))}°` : 'Hdg unknown'}
              {props.originCountry ? (
                <span className="ux-muted"> · {String(props.originCountry)}</span>
              ) : null}
            </p>
          )}

          <p className="map-peek-meta">
            {props.date
              ? new Date(props.date).toLocaleString()
              : props.meaning ?? 'Source version retained; observation time not supplied.'}
          </p>

          <p className="map-peek-meta">
            {props.source ||
              props.sourceId ||
              (props.aggregate ? 'Multiple retained records; inspect individual sources' : 'Reference geography')}
            {props.uncertainty ? ` · ${props.uncertainty}` : ''}
          </p>

          <p className="ux-muted">{timeLabel}</p>

          {props.validityStatus && (
            <p role="status" className="ux-notice">
              Current retained provider status: {String(props.validityStatus).replaceAll('_', ' ')}. This is
              separate from the selected historical interpretation and is not independent verification.
            </p>
          )}
          {props.outsideQuery && (
            <p className="ux-notice">
              Pinned record outside the selected query. Its own date and location are preserved.
            </p>
          )}
          {props.reviewStatus && (
            <p className="ux-notice">Workflow: {props.reviewStatus} · algorithmic geometry; inspect human review</p>
          )}
          {props.synthetic === true && <p className="ux-notice">Synthetic development inputs</p>}
          {props.status && (
            <p className="ux-status w-fit">
              {props.status} · {Math.round(Number(props.ageSeconds ?? 0) / 60)} minutes since reported validity
            </p>
          )}
          {props.aggregate && <p className="ux-muted">Display aggregate · {props.count} records. Zoom for reported positions.</p>}
          {props.aggregate &&
            readId &&
            readScope &&
            ['earthquakes', 'thermal', 'events'].includes(String(props.kind)) && (
              <ReadRecords
                scope={readScope}
                readId={readId}
                family={String(props.kind)}
                aggregate={String(props.aggregateId ?? '')}
                onChoose={(v) => onChooseAggregateRecord(v)}
              />
            )}
        </div>

        <div className="map-peek-actions">
          <button type="button" className="ux-button" onClick={() => onLocate(record)}>
            Locate
          </button>
          {canAttachToCase && citation && (
            <button
              type="button"
              className="ux-button"
              disabled={alreadyAttached}
              onClick={() => onAttachToCase(citation)}
            >
              {alreadyAttached ? (
                <>
                  <CheckIcon className="size-3.5" /> In case
                </>
              ) : (
                <>
                  <PlusIcon className="size-3.5" /> Attach to case
                </>
              )}
            </button>
          )}
          {props.kind === 'imagery' && props.recordId && (
            <button
              type="button"
              className="ux-button"
              data-primary="true"
              onClick={() => onShowPixels(String(props.recordId))}
            >
              Show pixels
            </button>
          )}
          <button
            type="button"
            className="ux-button ml-auto"
            onClick={onInspect}
            disabled={Boolean(props.aggregate)}
          >
            Inspect
          </button>
        </div>
      </section>
    )
  }

  // 2. Pinned Arabic / geographic place point (previously an unstyled top banner)
  if (canReadGeography && pinnedPlace) {
    return (
      <section className="map-peek-card" aria-label="Selected place reference">
        <header className="map-peek-head">
          <span className="map-peek-kind">Place point</span>
        </header>
        <div className="map-peek-body">
          <h2>
            <bdi dir="auto">{pinnedPlace.properties?.name}</bdi>
          </h2>
          <p className="map-peek-meta">
            <bdi dir="auto">{pinnedPlace.properties?.publisherRegion}</bdi>
          </p>
          {pinnedPlace.properties?.allowTerrainDisplay === true && (
            <p className="ux-muted">
              3D place reference <code>{String(pinnedPlace.id)}</code>
            </p>
          )}
        </div>
        <div className="map-peek-actions">
          <button type="button" className="ux-button" onClick={() => onLocate(pinnedPlace)}>
            Locate
          </button>
          <button type="button" className="ux-button ml-auto" onClick={onClearPinnedPlace}>
            Clear place selection
          </button>
        </div>
      </section>
    )
  }

  // 3. Selected vector reference drawing (previously an unstyled top banner)
  if (canReadGeography && referenceDrawing) {
    return (
      <section className="map-peek-card" aria-label="Selected reference drawing">
        <header className="map-peek-head">
          <span className="map-peek-kind">{referenceDrawing.family.replaceAll('-', ' ')}</span>
        </header>
        <div className="map-peek-body">
          <h2>
            <bdi dir="auto">{referenceDrawing.name || 'Unnamed drawing'}</bdi>
          </h2>
          <p className="map-peek-meta">
            <bdi dir="auto">{referenceDrawing.attribution}</bdi>
          </p>
          <p className="ux-muted">
            <code>{referenceDrawing.id}</code>
          </p>
        </div>
        <div className="map-peek-actions">
          <button type="button" className="ux-button ml-auto" onClick={onClearReferenceDrawing}>
            Clear drawing selection
          </button>
        </div>
      </section>
    )
  }

  return null
}

export type { Geometry }
