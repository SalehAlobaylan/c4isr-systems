import {
  BookmarkIcon,
  FileTextIcon,
  ImageIcon,
  InfoIcon,
  LayersIcon,
  LayoutGridIcon,
  ListIcon,
  RadarIcon,
  RulerIcon,
  SignalIcon,
} from '@/components/icons'
import { MapWorkbenchChooser } from '../MapWorkbenchChooser'
import type { MapTool } from '../WorkbenchContext'
import type { SupportPanel } from './useMapPanelController'

interface MapRailProps {
  hidden: boolean
  activePanel: SupportPanel
  activeTool?: MapTool
  drawerOpen: boolean
  permissions: string[]
  enabledLayerCount: number
  unavailableLayerCount: number
  caseCitationCount: number
  onTogglePanel: (panel: Exclude<SupportPanel, ''>, triggerLabel: string) => void
  onToggleTool: (tool: MapTool) => void
  onChooseWorkbench: (next: MapTool | 'previews') => void
}

export function MapRail({
  hidden,
  activePanel,
  activeTool,
  drawerOpen,
  permissions,
  enabledLayerCount,
  unavailableLayerCount,
  caseCitationCount,
  onTogglePanel,
  onToggleTool,
  onChooseWorkbench,
}: MapRailProps) {
  const canInvestigations = permissions.includes('investigations.read')
  const canSources = permissions.includes('sources.read')
  const toolActive = (t: MapTool) => drawerOpen && !activePanel && activeTool === t
  const otherToolActive =
    drawerOpen &&
    !activePanel &&
    Boolean(activeTool) &&
    activeTool !== 'investigations' &&
    activeTool !== 'imagery' &&
    activeTool !== 'geography' &&
    activeTool !== 'coverage'

  return (
    <nav className="map-rail" hidden={hidden} aria-label="Map panels">
      {/* Map Controls */}
      <button
        type="button"
        className="map-rail-item"
        aria-label="Map layers"
        aria-pressed={drawerOpen && activePanel === 'layers'}
        onClick={() => onTogglePanel('layers', 'Map layers')}
        title="Map layers, imagery overlays and basemap (L)"
      >
        <LayersIcon className="size-4" />
        <span className="map-rail-label">Layers</span>
        <span
          className="map-rail-badge"
          data-tone={unavailableLayerCount > 0 ? 'critical' : undefined}
        >
          {unavailableLayerCount > 0 ? '!' : enabledLayerCount}
        </span>
      </button>

      <button
        type="button"
        className="map-rail-item"
        aria-label="Map records"
        aria-pressed={drawerOpen && activePanel === 'records'}
        onClick={() => onTogglePanel('records', 'Map records')}
        title="Browse retained records & Data Radar (R)"
      >
        <ListIcon className="size-4" />
        <span className="map-rail-label">Records</span>
      </button>

      <button
        type="button"
        className="map-rail-item"
        aria-label="Map tools"
        aria-pressed={drawerOpen && activePanel === 'tools'}
        onClick={() => onTogglePanel('tools', 'Map tools')}
        title="Measure distance, draw study area, coordinates and buffer (T)"
      >
        <RulerIcon className="size-4" />
        <span className="map-rail-label">Spatial</span>
      </button>

      <button
        type="button"
        className="map-rail-item"
        aria-label="Map views"
        aria-pressed={drawerOpen && activePanel === 'views'}
        onClick={() => onTogglePanel('views', 'Map views')}
        title="Saved map views and authorized view links (V)"
      >
        <BookmarkIcon className="size-4" />
        <span className="map-rail-label">Views</span>
      </button>

      <button
        type="button"
        className="map-rail-item"
        aria-label="3D Simulation"
        aria-pressed={drawerOpen && activePanel === 'simulate'}
        onClick={() => onTogglePanel('simulate', '3D Simulation')}
        title="Configure 3D radar, threat & shrapnel simulation on the map and launch 3D replay"
      >
        <RadarIcon className="size-4" />
        <span className="map-rail-label">Simulate</span>
      </button>

      <button
        type="button"
        className="map-rail-item"
        aria-label="Sensor Coverage"
        aria-pressed={drawerOpen && activePanel === 'coverage'}
        onClick={() => onTogglePanel('coverage', 'Sensor Coverage')}
        title="Place radar, EO/IR & acoustic sensors on the map, analyze 3D blind spots, and optimize siting"
      >
        <SignalIcon className="size-4" />
        <span className="map-rail-label">Coverage</span>
      </button>

      <div className="map-rail-divider" aria-hidden="true" />

      {/* Analysis & Workbenches */}
      {canInvestigations && (
        <button
          type="button"
          className="map-rail-item"
          aria-label="Investigations"
          aria-pressed={toolActive('investigations')}
          onClick={() => onToggleTool('investigations')}
          title="Investigations, evidence citations, review and delivery (I)"
        >
          <FileTextIcon className="size-4" />
          <span className="map-rail-label">Cases</span>
          {caseCitationCount > 0 && (
            <span className="map-rail-badge" data-tone="selected">
              {caseCitationCount}
            </span>
          )}
        </button>
      )}

      {canSources && (
        <button
          type="button"
          className="map-rail-item"
          aria-label="Imagery & analysis"
          aria-pressed={toolActive('imagery')}
          onClick={() => onToggleTool('imagery')}
          title="Satellite imagery comparison and scene catalogue"
        >
          <ImageIcon className="size-4" />
          <span className="map-rail-label">Imagery</span>
        </button>
      )}

      {canSources && (
        <button
          type="button"
          className="map-rail-item"
          aria-label="Evidence inspector"
          aria-pressed={toolActive('geography')}
          onClick={() => onToggleTool('geography')}
          title="Geography references, retained event revisions and source provenance"
        >
          <InfoIcon className="size-4" />
          <span className="map-rail-label">Evidence</span>
        </button>
      )}

      <MapWorkbenchChooser
        value={activeTool}
        permissions={permissions}
        onChoose={onChooseWorkbench}
        className="map-rail-workbench-trigger"
        trigger={
          <>
            <LayoutGridIcon className="size-4" />
            <span className="map-rail-label" data-active={otherToolActive || undefined}>
              More
            </span>
          </>
        }
      />
    </nav>
  )
}
