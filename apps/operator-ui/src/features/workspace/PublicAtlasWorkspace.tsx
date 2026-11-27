import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useInfiniteQuery, useQueries, useQuery } from '@tanstack/react-query'
import { useNavigate, useSearch } from '@tanstack/react-router'
import type { Feature, FeatureCollection, Geometry, Position } from 'geojson'
import {
  ApiError,
  LOCAL_BASEMAP_ONLY,
  fixedScopeInput,
  isReadCapacityExhausted,
  retryConcurrentRead,
  useAPI,
  type InventoryFootprint,
  type InventorySource,
  type InventoryTimeBucket,
} from '@/lib/api'
import { longitudeLatitude, pointerLabel } from '@/lib/internationalInput'
import { useWorkspace } from '@/lib/workspaceContext'
import { useRealtime } from '@/realtime/RealtimeProvider'
import { usePublicContext } from '@/features/public-context/usePublicContext'
import { PublicContextPanel } from '@/features/public-context/PublicContextPanel'
import { queryFor } from '@/features/public-context/filters'
import {
  historicalReadOptions,
  monitoringPeriod,
  scopePeriodLabel,
  useAnalystScope,
} from './AnalystScopeProvider'
import {
  AtlasCanvas,
  layerInfo,
  type AtlasHandle,
  type AtlasLayer,
  type Camera,
  type RasterOverlay,
  type ReferenceDrawing,
} from './AtlasCanvas'
import { readSharedView, readStudyArea } from './atlasState'
import { projectContextStatus } from './contextLayerStatus'
import { useCoverage } from './CoveragePanel'
import { fetchDisplayBytes, readSceneFeature } from './displayBytes'
import { MapWorkbenches } from './MapWorkbenches'
import { dateLabel, useWorkspacePreference } from './preferences'
import { ReferenceDepthPanel } from './ReferenceDepthPanel'
import { CatalogPaging, useCatalog } from './useCatalog'
import {
  scopeForBucket,
  scopeForFootprint,
  scopeForInventorySource,
  useCoverageInventory,
} from './useCoverageInventory'
import { useReadSnapshotRelease } from './useReadSnapshotRelease'
import { toolNames, type MapTool, type WorkbenchSearch } from './WorkbenchContext'
import { useCaseCitationBridge, type CaseCitationKind } from './map-panels/caseCitationBridge'
import { ContextualLegend } from './map-panels/ContextualLegend'
import { LayersPanel, type LayerDisplayStatus, type LayersSubTab } from './map-panels/LayersPanel'
import { MapCanvasHud } from './map-panels/MapCanvasHud'
import { MapContextMenu, type MapContextMenuTarget } from './map-panels/MapContextMenu'
import { MapDiscoveryBanner } from './map-panels/MapDiscoveryBanner'
import { MapDrawer, type FullPageDestination } from './map-panels/MapDrawer'
import { MapRail } from './map-panels/MapRail'
import { MapScopeBar, summarizeCoverageReadiness } from './map-panels/MapScopeBar'
import { MapShortcutsDialog } from './map-panels/MapShortcutsDialog'
import type { QuickCommandId } from './map-panels/omnibox'
import { RecordsPanel, type RecordsBrowseMode } from './map-panels/RecordsPanel'
import { SavedViewsPanel, type MapBookmark } from './map-panels/SavedViewsPanel'
import { SelectionPeekCard } from './map-panels/SelectionPeekCard'
import { CoverageSetupPanel } from './map-panels/CoverageSetupPanel'
import type { CovPlacementTool } from './map-panels/coverageClient'
import { SimulationSetupPanel } from './map-panels/SimulationSetupPanel'
import type { SimPlacementTool } from './map-panels/simClient'
import { SpatialToolsPanel } from './map-panels/SpatialToolsPanel'
import { useMapPanelController } from './map-panels/useMapPanelController'

const empty: FeatureCollection = { type: 'FeatureCollection', features: [] }
type MapResponse = {
  type: 'FeatureCollection'
  features: Feature[]
  total?: number | null
  state?: string
  truncated?: boolean
  countMeaning?: string
}

export const collection = (v: unknown): FeatureCollection =>
  v && typeof v === 'object' && Array.isArray((v as FeatureCollection).features)
    ? (v as FeatureCollection)
    : empty

export function boundsOf(f?: Feature): [number, number, number, number] | undefined {
  if (!f) return
  if (Array.isArray(f.properties?.bounds)) return f.properties.bounds as [number, number, number, number]
  let w = 180
  let s = 90
  let e = -180
  let n = -90
  const walk = (c: unknown) => {
    if (!Array.isArray(c)) return
    if (typeof c[0] === 'number' && typeof c[1] === 'number') {
      w = Math.min(w, c[0])
      e = Math.max(e, c[0])
      s = Math.min(s, c[1])
      n = Math.max(n, c[1])
    } else {
      c.forEach(walk)
    }
  }
  if ('coordinates' in f.geometry) walk(f.geometry.coordinates)
  return w <= e ? [w, s, e, n] : undefined
}

const validCamera = (v: unknown): v is Camera =>
  !!v &&
  typeof v === 'object' &&
  ['lng', 'lat', 'zoom', 'bearing', 'pitch'].every((k) => Number.isFinite((v as Record<string, unknown>)[k])) &&
  Math.abs((v as Camera).lng) <= 180 &&
  Math.abs((v as Camera).lat) <= 85 &&
  (v as Camera).zoom >= 0 &&
  (v as Camera).zoom <= 22

const smallBounds = (b?: number[]) =>
  b && b[0] < b[2] && b[1] < b[3] && (b[2] - b[0]) * (b[3] - b[1]) <= 400 ? b.join(',') : undefined

const defaultLayers: AtlasLayer[] = ['earthquakes', 'thermal', 'weather', 'bulletins', 'aircraft', 'imagery']
const allLayers = Object.keys(layerInfo) as AtlasLayer[]
const queryLayers = allLayers
const EMPTY_KEYS: string[] = []

function fullPageForTool(
  tool: MapTool | undefined,
  scopeSearch: Record<string, string | undefined>,
): FullPageDestination | undefined {
  if (!tool) return undefined
  if (tool === 'systems' || tool === 'sources' || tool === 'jobs') {
    return { to: '/systems', search: scopeSearch, label: 'Open Systems full page' }
  }
  if (tool === 'imagery') {
    return { to: '/intelligence', search: { ...scopeSearch, view: 'compare' }, label: 'Open Imagery full page' }
  }
  if (tool === 'review') {
    return { to: '/intelligence', search: { ...scopeSearch, view: 'review' }, label: 'Open Review desk full page' }
  }
  if (tool === 'context') {
    return { to: '/intelligence', search: { ...scopeSearch, view: 'context' }, label: 'Open Context packages full page' }
  }
  if (tool === 'weather') {
    return { to: '/weather-reports', search: scopeSearch, label: 'Open Weather & reports full page' }
  }
  if (tool === 'tracks') {
    return { to: '/external-tracks', search: scopeSearch, label: 'Open Aircraft & vessels full page' }
  }
  return undefined
}

export function PublicAtlasWorkspace() {
  const api = useAPI()
  const analyst = useAnalystScope()!
  const activeScope = analyst.snapshot
  const { prefix, session } = useWorkspace()
  const { status: realtimeStatus } = useRealtime()
  const search = useSearch({ from: '/map' })
  const navigate = useNavigate({ from: '/map' })

  const cameraRef = useRef<AtlasHandle>(null)
  const areaRequest = useRef<AbortController | undefined>(undefined)
  const drawerRef = useRef<HTMLElement>(null)
  const searchInputRef = useRef<HTMLInputElement>(null)
  const coordinateLabel = useRef<HTMLSpanElement>(null)

  const [layoutViewport, setLayoutViewport] = useState(() => ({
    width: window.innerWidth,
    height: window.innerHeight,
  }))
  useEffect(() => {
    const onResize = () => setLayoutViewport({ width: window.innerWidth, height: window.innerHeight })
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  useEffect(() => () => areaRequest.current?.abort(), [JSON.stringify(analyst.input)])

  const [camera, setCamera] = useWorkspacePreference<Camera>(
    'atlas-camera',
    { lng: 43.3, lat: 24.7, zoom: 5, bearing: 0, pitch: 0 },
    validCamera,
  )
  const pointer = useRef<[number, number]>([camera.lng, camera.lat])
  const updatePointer = useCallback((p: [number, number]) => {
    pointer.current = p
    if (coordinateLabel.current) coordinateLabel.current.textContent = 'Pointer · ' + pointerLabel(p)
  }, [])

  const regionID = activeScope?.area.geography?.regionId ?? analyst.input?.area.regionId ?? ''
  const [layers, setLayers] = useWorkspacePreference<AtlasLayer[]>(
    'atlas-layers',
    defaultLayers,
    (v): v is AtlasLayer[] => Array.isArray(v) && v.length <= 9 && v.every((k) => k in layerInfo),
  )
  const [basemap, setBasemap] = useWorkspacePreference<'streets' | 'quiet' | 'reference'>(
    'atlas-basemap',
    'quiet',
    (v): v is 'streets' | 'quiet' | 'reference' => ['streets', 'quiet', 'reference'].includes(String(v)),
  )
  const [bookmarks, setBookmarks] = useWorkspacePreference<MapBookmark[]>(
    'atlas-bookmarks',
    [],
    (v): v is MapBookmark[] =>
      Array.isArray(v) &&
      v.length <= 20 &&
      v.every(
        (x) =>
          typeof x.name === 'string' &&
          x.name.length <= 80 &&
          validCamera(x.camera) &&
          Array.isArray(x.layers) &&
          x.layers.every((k: string) => k in layerInfo),
      ),
  )

  const [layerSearch, setLayerSearch] = useState('')
  const [layersInitialTab, setLayersInitialTab] = useState<LayersSubTab>('evidence')
  const [layersTabKey, setLayersTabKey] = useState(0)
  const [recordsInitialMode, setRecordsInitialMode] = useState<RecordsBrowseMode>('view')
  const [recordsModeKey, setRecordsModeKey] = useState(0)
  const [boundaryLevel, setBoundaryLevel] = useState<'auto' | 'country' | 'region'>('auto')
  const [proposalOverlay, setProposalOverlay] = useState<FeatureCollection>(empty)
  const [simOverlay, setSimOverlay] = useState<FeatureCollection>(empty)
  const [simTool, setSimTool] = useState<SimPlacementTool>(null)
  const [covTool, setCovTool] = useState<CovPlacementTool>(null)
  const simClickHandler = useRef<((point: [number, number]) => void) | null>(null)
  const registerSimMapClick = useCallback((handler: ((point: [number, number]) => void) | null) => {
    simClickHandler.current = handler
  }, [])
  const handleSimMapClick = useCallback((point: [number, number]) => {
    simClickHandler.current?.(point)
  }, [])
  const [family, setFamily] = useState<AtlasLayer>('imagery')
  const [query, setQuery] = useState('')
  const [debounced, setDebounced] = useState('')
  const [searchOpen, setSearchOpen] = useState(false)
  const [mapError, setMapError] = useState('')
  const [selected, setSelected] = useState<Feature[]>([])
  const [opacity, setOpacity] = useState(0.75)
  const [referenceOpacity, setReferenceOpacity] = useState(0.7)
  const [mode, setMode] = useState<'inspect' | 'measure' | 'area'>('inspect')
  const [draw, setDraw] = useState<Position[]>([])
  const [sketchArea, setSketchArea] = useState<Feature | undefined>(() =>
    readStudyArea(analyst.input?.area.geometry ? JSON.stringify(analyst.input.area.geometry) : search.area),
  )
  const [measurement, setMeasurement] = useState('')
  const [coordinates, setCoordinates] = useState('')
  const [paused, setPaused] = useState(false)
  const [frozen, setFrozen] = useState<{ key: string; layers: Partial<Record<AtlasLayer, MapResponse>> }>()
  const [pauseTime, setPauseTime] = useState('')
  const [viewport, setViewport] = useState<[number, number, number, number]>()
  const [radius, setRadius] = useState('10000')
  const [bookmarkName, setBookmarkName] = useState('')
  const [captureId, setCaptureId] = useState('')
  const [compareId, setCompareId] = useState('')
  const [quality, setQuality] = useState(false)
  const [swipe, setSwipe] = useState(0.5)
  const [referenceDrawing, setReferenceDrawing] = useState<ReferenceDrawing>()
  const [pinnedPlace, setPinnedPlace] = useState<Feature>()
  const [shortcutsOpen, setShortcutsOpen] = useState(false)
  const [contextMenu, setContextMenu] = useState<MapContextMenuTarget | null>(null)

  const tool =
    search.tool === 'simulation' && session.productProfile === 'map'
      ? undefined
      : (search.tool as MapTool | undefined)

  const wb = useMemo<WorkbenchSearch>(() => {
    try {
      const p = JSON.parse(search.workbench ?? '{}')
      return p && typeof p === 'object' && !Array.isArray(p)
        ? Object.fromEntries(
            Object.entries(p).filter(
              ([k, v]) =>
                [
                  'view',
                  'job',
                  'assessment',
                  'bbox',
                  'before',
                  'after',
                  'id',
                  'kind',
                  'model',
                  'upload',
                  'display',
                  'manifest',
                  'feature',
                  'layers',
                  'basemap',
                  'atlasArea',
                  'terrainPoint',
                ].includes(k) &&
                typeof v === 'string' &&
                v.length <= (k === 'layers' || k === 'feature' ? 512 : 256),
            ),
          )
        : {}
    } catch {
      return {}
    }
  }, [search.workbench])

  const autoCollapseTool = tool === 'environments' && wb.view === 'environment-atlas'
  const panels = useMapPanelController(tool, autoCollapseTool)

  const caseBridgeScope = JSON.stringify(prefix)
  const queueAttachment = useCaseCitationBridge((s) => s.queueAttachment)
  const attachedKeys = useCaseCitationBridge((s) => (s.scopeKey === caseBridgeScope ? s.attachedKeys : EMPTY_KEYS))

  const data = usePublicContext({
    cartography: false,
    loadEvents: search.tool === 'geography',
    regionId: sketchArea ? undefined : regionID,
    geometry: sketchArea ? JSON.stringify(sketchArea.geometry) : undefined,
  })

  useEffect(() => {
    setSketchArea(readStudyArea(analyst.input?.area.geometry ? JSON.stringify(analyst.input.area.geometry) : undefined))
  }, [JSON.stringify(analyst.input?.area)])

  useEffect(() => {
    const shared = readSharedView(search.mapState)
    if (shared) {
      setCamera(shared.camera)
      setLayers(shared.layers)
      cameraRef.current?.restore(shared.camera)
    }
  }, [search.mapState])

  useEffect(() => {
    const timeout = setTimeout(() => setDebounced(query.trim()), 220)
    return () => clearTimeout(timeout)
  }, [query])

  const canReadGeography = session.permissions.includes('sources.read')
  const countries = useInfiniteQuery({
    queryKey: [...prefix, 'geography', 'countries', session.accessRevision],
    enabled: canReadGeography,
    initialPageParam: 0,
    staleTime: 300_000,
    queryFn: ({ signal, pageParam }) => api.listGeographicRegions({ kind: 'country', offset: pageParam }, signal),
    getNextPageParam: (page, _, offset) =>
      page.nextOffset ?? (page.truncated && page.features.length ? offset + page.features.length : undefined),
  })

  const regionQuery = useQuery({
    queryKey: [...prefix, 'geography', 'region', regionID],
    queryFn: ({ signal }) => api.listGeographicRegions({ id: regionID }, signal),
    enabled: !!regionID,
    staleTime: 300_000,
  })

  const scope = collection(regionQuery.data).features[0]
  const scopeBounds = boundsOf(sketchArea ?? scope)
  const scopeBBox = smallBounds(scopeBounds) ?? (regionID ? undefined : data.savedFilters.bbox || undefined)

  const exactSelection = useMemo<Feature | undefined>(
    () =>
      activeScope?.area.geometry
        ? {
            type: 'Feature',
            id: activeScope.area.geography?.regionId ?? activeScope.area.revisionId,
            geometry: activeScope.area.geometry as unknown as Geometry,
            properties: {
              ...scope?.properties,
              name: activeScope.area.name,
              kind: scope?.properties?.kind ?? 'study',
            },
          }
        : undefined,
    [activeScope?.scopeHash, scope],
  )

  const fittedArea = useRef('')
  useEffect(() => {
    if (!exactSelection || fittedArea.current === activeScope?.area.revisionId) return
    const first = !fittedArea.current
    fittedArea.current = activeScope?.area.revisionId ?? ''
    if (first && search.mapState) return
    const b = boundsOf(exactSelection)
    if (b && b[1] < 85 && b[3] > -85) {
      cameraRef.current?.fitBounds([b[0], Math.max(-85, b[1]), b[2], Math.min(85, b[3])])
    }
  }, [activeScope?.area.revisionId, exactSelection])

  const regionName =
    activeScope?.area.name ??
    (sketchArea
      ? 'Drawn study area'
      : scope?.properties?.name ??
        (regionID
          ? regionQuery.isSuccess || regionQuery.isError
            ? 'Selected area unavailable'
            : 'Loading selected area…'
          : data.savedFilters.bbox
            ? 'Bounded investigation'
            : 'World · configured evidence'))

  const children = useCatalog<Feature>(
    'geography',
    undefined,
    !!scope?.properties?.regionKey,
    {
      parent: String(scope?.properties?.regionKey ?? ''),
      sourceId: scope?.properties?.sourceId,
      releaseId: activeScope?.area.geography?.releaseId,
      editionId: activeScope?.area.geography?.releaseId ? undefined : scope?.properties?.editionId,
    },
    300,
  )

  const ancestors = useCatalog<Feature>(
    'geography',
    undefined,
    !!scope?.properties?.parentKey,
    {
      sourceId: scope?.properties?.sourceId,
      releaseId: activeScope?.area.geography?.releaseId,
      editionId: activeScope?.area.geography?.releaseId ? undefined : scope?.properties?.editionId,
    },
    300,
  )

  const searchResults = useInfiniteQuery({
    queryKey: [...prefix, 'geographic-search', debounced],
    enabled: debounced.length > 1 && searchOpen && session.permissions.includes('sources.read'),
    gcTime: 0,
    retry: false,
    initialPageParam: 0,
    queryFn: ({ signal, pageParam }) => api.listGeographicRegions({ q: debounced, offset: pageParam }, signal),
    getNextPageParam: (page) => page.nextOffset ?? undefined,
  })

  useEffect(() => {
    const expiry = pinnedPlace?.properties?.rights?.expiresAt
    const remaining = typeof expiry === 'string' ? Date.parse(expiry) - Date.now() : NaN
    if (!Number.isFinite(remaining)) return
    if (remaining <= 0) {
      setPinnedPlace(undefined)
      return
    }
    if (remaining > 2_147_483_647) return
    const timeout = window.setTimeout(() => setPinnedPlace(undefined), remaining)
    return () => window.clearTimeout(timeout)
  }, [pinnedPlace])

  const searchFeatures = searchResults.data?.pages.flatMap((page) => collection(page).features) ?? []

  const chooseRegion = useCallback(
    (f: Feature) => {
      if (f.geometry.type !== 'Point') setPinnedPlace(undefined)
      if (f.geometry.type === 'Point') {
        setPinnedPlace(f.properties?.family ? f : undefined)
        cameraRef.current?.focus(...(f.geometry.coordinates as [number, number]))
        setSearchOpen(false)
        setMapError(
          `${f.properties?.name} is a place point. The investigation remains ${regionName}; choose a region or draw an area.`,
        )
        return
      }
      const id = String(f.id ?? f.properties?.id)
      setSketchArea(undefined)
      setSelected([])
      setSearchOpen(false)
      setQuery('')
      setPaused(false)
      setFrozen(undefined)
      setDraw([])
      const bounds = boundsOf(f)
      if (bounds && bounds[2] - bounds[0] <= 180) cameraRef.current?.fitBounds(bounds)
      void navigate({
        search: (p) => ({
          ...p,
          ...analyst.searchFor({
            area: {
              kind: 'administrative',
              regionId: id,
              sourceId: f.properties?.sourceId,
              releaseId: f.properties?.releaseId || undefined,
            },
            temporal: analyst.input?.temporal ?? monitoringPeriod(),
          }),
          selected: undefined,
        }),
      })
    },
    [navigate, regionName, analyst],
  )

  const countryFeatures = useMemo(
    () => (canReadGeography ? countries.data?.pages.flatMap((page) => collection(page).features) ?? [] : []),
    [canReadGeography, countries.data],
  )

  const geographicFeatures = useMemo<FeatureCollection>(
    () => ({
      type: 'FeatureCollection',
      features: [
        ...countryFeatures,
        ...collection(children.data).features.filter((f) => f.geometry.type !== 'Point'),
      ],
    }),
    [countryFeatures, children.data],
  )

  const enabledLayers = layers.filter((k) => session.permissions.includes(layerInfo[k].permission))
  const temporal = queryFor(data.savedFilters)
  const filters = {
    region_id: sketchArea ? undefined : regionID || undefined,
    bbox: sketchArea ? undefined : scopeBBox,
    view: temporal.view,
    as_of: temporal.as_of,
    observed_from: temporal.observed_from,
    observed_to: temporal.observed_to,
    source_id: temporal.source_id,
    quality: temporal.quality,
    zoom: Math.floor(camera.zoom),
    geometry: sketchArea ? JSON.stringify(sketchArea.geometry) : undefined,
  }

  const scopeKey = activeScope?.scopeHash ?? JSON.stringify([analyst.input, analyst.error])
  useEffect(() => {
    setSelected([])
    setProposalOverlay(empty)
  }, [scopeKey])

  const scopeCoverage = useCoverage(activeScope, !paused)
  const coverageInventory = useCoverageInventory(!paused)
  const historical = activeScope?.temporal.mode === 'historical'
  const readFamilies = historical
    ? queryLayers.filter((k) => session.permissions.includes(layerInfo[k].permission))
    : enabledLayers
  const inspectingEnvironment = tool === 'environments' && ['environment-3d', 'environment-atlas'].includes(wb.view ?? '')

  const mapBatch = useQuery({
    ...historicalReadOptions(activeScope),
    queryKey: [
      ...prefix,
      'atlas-read',
      scopeKey,
      activeScope?.authorizationRevision,
      temporal.source_id,
      temporal.quality,
      historical ? 'fixed-study' : Math.floor(camera.zoom),
      readFamilies,
    ],
    queryFn: ({ signal }) =>
      api.readAnalystScope(
        {
          ...fixedScopeInput(activeScope!),
          families: readFamilies,
          filters: { sourceId: temporal.source_id, quality: temporal.quality },
          zoom: Math.floor(camera.zoom),
          includeEvents: false,
        },
        signal,
      ),
    enabled: !!activeScope && !analyst.error && enabledLayers.length > 0 && !paused && !inspectingEnvironment,
    retry: retryConcurrentRead,
    retryDelay: (attempt) => 150 * (attempt + 1),
    refetchInterval: (q) => (isReadCapacityExhausted(q.state.error) ? 15_000 : false),
    gcTime: 0,
  })

  useReadSnapshotRelease(inspectingEnvironment ? undefined : mapBatch.data?.id, [...prefix, 'atlas-read'])

  const mapQueries = queryLayers.map((k) => ({
    ...mapBatch,
    isError: mapBatch.isError || mapBatch.data?.layers[k]?.state === 'unavailable',
    data:
      mapBatch.data?.scope.scopeHash === scopeKey
        ? (mapBatch.data.layers[k] as unknown as MapResponse)
        : undefined,
  }))

  const currentLayers = useMemo(
    () => Object.fromEntries(queryLayers.map((k, i) => [k, mapQueries[i].data])) as Partial<Record<AtlasLayer, MapResponse>>,
    [...mapQueries.map((q) => q.data)],
  )
  const lostLayerAccess = mapBatch.error instanceof ApiError && [401, 403].includes(mapBatch.error.status)
  const display = lostLayerAccess ? {} : paused && frozen?.key === scopeKey ? frozen.layers : currentLayers

  useEffect(() => {
    if (lostLayerAccess) {
      setSelected([])
      setFrozen(undefined)
    }
  }, [lostLayerAccess])

  const rendered = useMemo(
    () => Object.fromEntries(enabledLayers.map((k) => [k, collection(display[k])])),
    [display, layers.join(',')],
  )
  const updateLayers = useMemo(
    () => Object.fromEntries(enabledLayers.filter((k) => !!display[k]).map((k) => [k, collection(display[k])])),
    [display, layers.join(',')],
  )

  useEffect(() => {
    if (paused && frozen?.key !== scopeKey) {
      setPaused(false)
      setFrozen(undefined)
    }
  }, [scopeKey, paused, frozen?.key])

  const toggleLayer = (k: AtlasLayer) => setLayers((v) => (v.includes(k) ? v.filter((x) => x !== k) : [...v, k]))

  const openTool = useCallback(
    (next: MapTool, values?: WorkbenchSearch) => {
      panels.openPanel('')
      panels.setDrawerHiddenForTool(false)
      panels.setCleanMap(false)
      panels.setDrawerSize(window.innerWidth <= 760 || window.innerHeight <= 500 ? 'full' : 'normal')
      void navigate({
        search: (p) => ({
          ...p,
          tool: next,
          workbench: JSON.stringify({
            ...values,
            bbox: values?.bbox ?? scopeBBox,
            view:
              values?.view ??
              (next === 'review' ? 'review' : next === 'context' ? 'context' : next === 'jobs' ? 'analysis' : undefined),
          }),
        }),
      })
    },
    [navigate, scopeBBox, panels],
  )

  const updateWorkbench = useCallback(
    (value: WorkbenchSearch, replace = false) => {
      const handoff = value.view === 'environment-3d' || value.view === 'environment-atlas'
      const safeArea =
        activeScope && (activeScope.area.persistent || activeScope.area.geography || activeScope.area.kind === 'world')
      const frozenScope =
        handoff && safeArea && analyst.input?.temporal.rollingSeconds
          ? analyst.searchFor(fixedScopeInput(activeScope))
          : undefined
      void navigate({ search: (p) => ({ ...p, ...frozenScope, workbench: JSON.stringify(value) }), replace })
    },
    [navigate, activeScope, analyst.input?.temporal.rollingSeconds, analyst.searchFor],
  )

  const closeTool = useCallback(() => {
    panels.openPanel('')
    panels.setDrawerHiddenForTool(false)
    void navigate({ search: (p) => ({ ...p, tool: undefined }) })
  }, [navigate, panels])

  const showAtlas = useCallback(() => {
    panels.openPanel('')
    panels.setDrawerHiddenForTool(true)
    panels.setCleanMap(false)
  }, [panels])

  const host = useMemo(
    () => ({
      scope: activeScope,
      scopeInput: activeScope ? fixedScopeInput(activeScope) : analyst.input,
      selectionKey: JSON.stringify(analyst.input),
      previewFootprints: setProposalOverlay,
      search: {
        ...wb,
        bbox: wb.bbox ?? scopeBBox,
        view: wb.view ?? (tool === 'review' ? 'review' : tool === 'context' ? 'context' : undefined),
      },
      regionId: sketchArea ? undefined : regionID || undefined,
      geometry: activeScope?.area.geometry ? JSON.stringify(activeScope.area.geometry) : undefined,
      operationGeometry: activeScope?.area.geometry ? JSON.stringify(activeScope.area.geometry) : undefined,
      areaError: analyst.error ?? (!activeScope ? 'Resolving the selected area and period…' : undefined),
      regionName,
      open: openTool,
      update: updateWorkbench,
      close: closeTool,
      showAtlas,
      showOnMap: (sel: string) => {
        setLayers((v) => [...new Set([...v, ...(sel.startsWith('track:') ? (['aircraft', 'vessels'] as AtlasLayer[]) : [])])])
        void navigate({ search: (p) => ({ ...p, selected: sel }) })
        panels.setCleanMap(false)
      },
    }),
    [
      wb,
      scopeBBox,
      tool,
      regionID,
      filters.geometry,
      scope,
      search.area,
      regionName,
      openTool,
      updateWorkbench,
      closeTool,
      showAtlas,
      activeScope,
      analyst.input,
      analyst.error,
      navigate,
      panels,
    ],
  )

  const environmentHandoff =
    tool === 'environments' &&
    wb.view === 'environment-atlas' &&
    (!wb.atlasArea || wb.atlasArea === activeScope?.area.revisionId) &&
    session.permissions.includes('environments.download')

  const featureLink = environmentHandoff
    ? wb.feature?.match(/^(geodisplay_[a-f0-9]{64}):([a-f0-9]{64}:[A-Za-z0-9_.:-]{1,128})$/)
    : undefined

  const environmentFeature = useQuery({
    queryKey: [...prefix, 'environment-atlas-feature', scopeKey, wb.id, wb.manifest, featureLink?.[0]],
    enabled: !!featureLink && !!wb.id && !!wb.manifest,
    gcTime: 0,
    retry: false,
    refetchInterval: 30_000,
    queryFn: async ({ signal }) => {
      const displayItem = await api.environmentDisplay(featureLink![1], signal)
      if (displayItem.releaseId !== wb.id || displayItem.manifestSha256 !== wb.manifest || displayItem.state !== 'READY') {
        throw new Error('The linked display is unavailable.')
      }
      return readSceneFeature(await fetchDisplayBytes(api.environmentDisplayContent, displayItem, signal), displayItem, featureLink![2])
    },
  })

  const terrainCoordinates = wb.terrainPoint?.match(/^(-?\d{1,3}\.\d{6}),(-?\d{1,2}\.\d{6})$/)
  const terrainID =
    environmentHandoff && terrainCoordinates && /^geodisplay_[a-f0-9]{64}$/.test(wb.display ?? '')
      ? wb.display
      : undefined

  const terrainPoint = useQuery({
    queryKey: [...prefix, 'environment-atlas-terrain', scopeKey, wb.id, wb.manifest, terrainID, wb.terrainPoint],
    enabled: !!terrainID && !!wb.id && !!wb.manifest,
    gcTime: 0,
    retry: false,
    refetchInterval: 30_000,
    queryFn: async ({ signal }): Promise<Feature> => {
      const displayItem = await api.environmentDisplay(terrainID!, signal)
      const [west, south, east, north] = displayItem.bounds
      const lng = Number(terrainCoordinates![1])
      const lat = Number(terrainCoordinates![2])
      if (
        displayItem.releaseId !== wb.id ||
        displayItem.manifestSha256 !== wb.manifest ||
        displayItem.state !== 'READY' ||
        !['terrain-glb-v1', 'terrain-glb-v2', 'ground-glb-v1'].includes(displayItem.recipe) ||
        lng < west - 0.000001 ||
        lng > east + 0.000001 ||
        lat < south - 0.000001 ||
        lat > north + 0.000001
      ) {
        throw new Error('The linked terrain point is unavailable.')
      }
      return {
        type: 'Feature',
        id: `${terrainID}:${wb.terrainPoint}`,
        geometry: { type: 'Point', coordinates: [lng, lat] },
        properties: { name: 'Terrain point' },
      }
    },
  })

  const selectedTerrain = environmentHandoff && terrainID && terrainPoint.isSuccess ? terrainPoint.data : undefined
  const selectedEnvironmentFeature =
    environmentHandoff && featureLink && environmentFeature.isSuccess ? environmentFeature.data : undefined
  const retainedSelection = selectedTerrain ?? selectedEnvironmentFeature

  useEffect(() => {
    if (retainedSelection) {
      const bounds = boundsOf(retainedSelection)
      if (bounds) cameraRef.current?.fitBounds(bounds)
    }
  }, [retainedSelection?.id])

  const streetID =
    environmentHandoff && /^geodisplay_[a-f0-9]{64}$/.test(wb.basemap ?? '') ? wb.basemap : undefined

  const streets = useQuery({
    queryKey: [...prefix, 'environment-street-map', scopeKey, wb.id, wb.manifest, streetID],
    enabled: !!streetID && !!wb.id && !!wb.manifest,
    gcTime: 0,
    retry: false,
    refetchInterval: 30_000,
    queryFn: async ({ signal }) => {
      const displayItem = await api.environmentDisplay(streetID!, signal)
      if (
        displayItem.releaseId !== wb.id ||
        displayItem.manifestSha256 !== wb.manifest ||
        displayItem.recipe !== 'street-pmtiles-v1' ||
        displayItem.state !== 'READY'
      ) {
        throw new Error('The retained street map is unavailable.')
      }
      return { display: displayItem, bytes: await fetchDisplayBytes(api.environmentDisplayContent, displayItem, signal) }
    },
  })

  const streetMap = streetID && streets.isSuccess ? streets.data : undefined
  useEffect(() => {
    if (streetMap && !retainedSelection) {
      cameraRef.current?.fitBounds(streetMap.display.bounds as [number, number, number, number])
    }
  }, [streetMap?.display.id, retainedSelection?.id])

  const locateDeepLink = search.selected?.indexOf(':') ?? -1
  const deepKind = locateDeepLink > 0 ? search.selected!.slice(0, locateDeepLink) : ''
  const deepId = locateDeepLink > 0 ? search.selected!.slice(locateDeepLink + 1) : ''
  const deepFeature = Object.values(display)
    .flatMap((v) => collection(v).features)
    .find((f) => f.properties?.recordId === deepId)

  const externalDetail = useQuery({
    queryKey: [...prefix, 'external-track', deepId],
    queryFn: () => api.getExternalTrack(deepId),
    enabled: deepKind === 'track' && !!deepId && !deepFeature && session.permissions.includes('tracks.read'),
    retry: false,
  })

  useEffect(() => {
    if (!deepId) return
    if (deepFeature) {
      setSelected([deepFeature])
      return
    }
    if (deepKind === 'event' || deepKind === 'earthquakes' || deepKind === 'thermal') {
      data.selectEvidence(deepId)
      const v = data.detail.data?.event
      if (v?.evidenceId === deepId && v.reported.position) {
        setSelected([
          {
            type: 'Feature',
            geometry: { type: 'Point', coordinates: [v.reported.position.lng, v.reported.position.lat] },
            properties: {
              recordId: deepId,
              evidenceId: deepId,
              kind: v.reported.type === 'thermal.detection' ? 'thermal' : 'earthquakes',
              title: v.reported.title,
              date: v.reported.observedAt,
              outsideQuery: true,
              sourceId: v.sourceId,
            },
          },
        ])
      }
    } else if (deepKind === 'track' && externalDetail.data) {
      const v = externalDetail.data
      setSelected([
        {
          type: 'Feature',
          geometry: { type: 'Point', coordinates: [v.reported.position.lng, v.reported.position.lat] },
          properties: {
            recordId: v.id,
            kind: v.kind === 'aircraft' ? 'aircraft' : 'vessels',
            title: v.identityClaim,
            date: v.lastSeenAt,
            sourceId: v.sourceId,
            outsideQuery: true,
          },
        },
      ])
    }
  }, [search.selected, deepFeature, data.detail.data, externalDetail.data, scopeKey])

  // Selecting an object opens the floating SelectionPeekCard — NEVER hijacks the left drawer!
  const inspect = useCallback(
    (hits: Feature[]) => {
      setSelected(hits)
      const p = hits[0]?.properties
      if (p?.evidenceId && (p.kind === 'earthquakes' || p.kind === 'thermal')) {
        data.selectEvidence(String(p.evidenceId))
      }
      if (p?.recordId && !p.aggregate) {
        void navigate({ search: (old) => ({ ...old, selected: `${p.kind}:${p.recordId}` }), replace: true })
      }
    },
    [data, navigate],
  )

  const record = selected[0]
  const props = record?.properties ?? {}
  const locate = (f?: Feature) => {
    const b = boundsOf(f)
    if (b) cameraRef.current?.fitBounds(b)
  }

  const previews = collection(display[family])
    .features.filter((f, i, rows) => rows.findIndex((r) => r.properties?.recordId === f.properties?.recordId) === i)
    .slice(0, 100)

  const captures = collection(display.imagery)
    .features.slice()
    .sort((a, b) => String(a.properties?.date).localeCompare(String(b.properties?.date)))
  const capture = captures.find((f) => f.properties?.recordId === captureId)
  const compatibleCaptures = captures.filter(
    (f) =>
      f.properties?.recordId !== captureId &&
      !!capture &&
      boundsOf(f)?.every((v, i) => Math.abs(v - boundsOf(capture)![i]) < 1e-7),
  )
  const comparison = compatibleCaptures.find((f) => f.properties?.recordId === compareId)
  const rasterCaptures = [capture, comparison].filter((f): f is Feature => !!f)
  const rasterDefs = [false, ...(quality ? [true] : [])].flatMap((mask) =>
    rasterCaptures.map((f, i) => ({
      id: String((mask ? f.properties?.qualityArtifactId : f.properties?.rgbArtifactId) ?? ''),
      bounds: boundsOf(f)!,
      label: `${mask ? 'Quality mask' : 'Retained true color'} · ${dateLabel(f.properties?.date)}`,
      side: comparison ? ((i === 0 ? 'before' : 'after') as 'before' | 'after') : undefined,
      role: mask ? ('quality' as const) : undefined,
    })),
  )

  const rasterQueries = useQueries({
    queries: rasterDefs.map((r) => ({
      queryKey: [...prefix, 'preview-artifact', r.id],
      queryFn: ({ signal }) => api.downloadArtifact(r.id, signal),
      enabled: !!r.id,
      gcTime: 0,
      staleTime: 60_000,
      retry: false,
    })),
  })

  const [rasters, setRasters] = useState<RasterOverlay[]>([])
  useEffect(() => {
    const next = rasterDefs.flatMap((r, i) =>
      rasterQueries[i]?.data
        ? [{ ...r, url: URL.createObjectURL(rasterQueries[i].data!), opacity: r.role === 'quality' ? 1 : opacity }]
        : [],
    )
    setRasters(next)
    return () => next.forEach((r) => URL.revokeObjectURL(r.url))
  }, [
    rasterQueries.map((q) => q.dataUpdatedAt).join(),
    rasterDefs.map((r) => r.id + String(r.side)).join(),
    quality,
  ])

  const visibleRasters = useMemo(
    () => rasters.map((r) => ({ ...r, opacity: r.role === 'quality' ? 1 : opacity })),
    [rasters, opacity],
  )

  const sketch = useMemo<FeatureCollection>(
    () => ({
      type: 'FeatureCollection',
      features: [
        ...(activeScope?.area.geometry
          ? [
              {
                type: 'Feature' as const,
                geometry: activeScope.area.geometry as unknown as Feature['geometry'],
                properties: { name: activeScope.area.name },
              },
            ]
          : sketchArea
            ? [sketchArea]
            : []),
        ...draw.map((p) => ({
          type: 'Feature' as const,
          geometry: { type: 'Point' as const, coordinates: p },
          properties: {},
        })),
        ...(draw.length > 1
          ? [
              {
                type: 'Feature' as const,
                geometry: { type: 'LineString' as const, coordinates: draw },
                properties: {},
              },
            ]
          : []),
      ],
    }),
    [draw, sketchArea, activeScope?.area.revisionId],
  )

  const onDraw = (p: Position) => setDraw((v) => [...v, p].slice(0, 64))

  const finishSketch = async () => {
    setMapError('')
    areaRequest.current?.abort()
    const request = new AbortController()
    areaRequest.current = request
    try {
      if (draw.length < (mode === 'area' ? 3 : 2)) {
        throw new Error(mode === 'area' ? 'Choose at least three points.' : 'Choose at least two points.')
      }
      const geometry =
        mode === 'area'
          ? { type: 'Polygon', coordinates: [[...draw, draw[0]]] }
          : { type: 'LineString', coordinates: draw }
      if (mode === 'area') {
        const r = await api.resolveArea({ kind: 'drawn', name: 'Drawn study area', geometry }, request.signal)
        if (request.signal.aborted) return
        setMeasurement(`${r.areaM2.toLocaleString(undefined, { maximumFractionDigits: 0 })} m² · WGS84 geodesic`)
        analyst.changeArea({ kind: 'drawn', name: 'Drawn study area', geometry })
        setSelected([])
      } else {
        const r = await api.measureMapGeometry(geometry, request.signal)
        if (request.signal.aborted) return
        setMeasurement(`${(r.lengthM / 1000).toFixed(3)} km · WGS84 geodesic`)
      }
      setMode('inspect')
    } catch (e) {
      if (!request.signal.aborted) setMapError(e instanceof Error ? e.message : 'Invalid geometry.')
    }
  }

  const presetTime = (days?: number) => {
    if (days) {
      analyst.changePeriod(monitoringPeriod(days))
    } else {
      const to = new Date()
      const from = new Date(to.getTime() - 366 * 86_400_000)
      analyst.changePeriod({
        mode: 'historical',
        interval: { from: from.toISOString(), to: to.toISOString() },
        revisionView: 'latest',
        timezone: activeScope?.temporal.timezone ?? Intl.DateTimeFormat().resolvedOptions().timeZone,
        backgroundImagery: false,
      })
    }
    setPaused(false)
  }

  const timeLabel = scopePeriodLabel(activeScope)
  const rollingHours = analyst.input?.temporal.rollingSeconds
    ? Math.round(analyst.input.temporal.rollingSeconds / 3600)
    : undefined
  const periodTitle = rollingHours
    ? `Last ${rollingHours} hours`
    : activeScope?.temporal.mode === 'historical'
      ? 'Historical study'
      : 'Selected period'
  const periodDetail = activeScope
    ? `${new Date(activeScope.temporal.interval.from).toLocaleDateString(undefined, {
        timeZone: activeScope.temporal.timezone,
        month: 'short',
        day: 'numeric',
        ...(activeScope.temporal.mode === 'historical' ? { year: 'numeric' as const } : {}),
      })} – ${new Date(activeScope.temporal.interval.to).toLocaleDateString(undefined, {
        timeZone: activeScope.temporal.timezone,
        month: 'short',
        day: 'numeric',
        ...(activeScope.temporal.mode === 'historical' ? { year: 'numeric' as const } : {}),
      })} · ${activeScope.temporal.timezone}`
    : 'Resolving time window…'

  const activeFilterCount =
    (data.savedFilters.sourceId ? 1 : 0) +
    (data.savedFilters.phenomenon ? 1 : 0) +
    (data.savedFilters.quality ? 1 : 0) +
    (data.savedFilters.view !== 'latest' ? 1 : 0) +
    (data.savedFilters.revisions === 'all' ? 1 : 0)

  const readCapacityExhausted = isReadCapacityExhausted(mapBatch.error)
  const unavailableLayers = enabledLayers.filter((k) => mapQueries[queryLayers.indexOf(k)]?.isError)

  const parents: Feature[] = []
  let parentKey = scope?.properties?.parentKey
  for (let i = 0; parentKey && i < 5; i++) {
    const f = collection(ancestors.data).features.find((item) => item.properties?.regionKey === parentKey)
    if (!f || parents.some((p) => p.id === f.id)) break
    parents.unshift(f)
    parentKey = f.properties?.parentKey
  }

  const referenceGroups = useMemo(() => {
    const groups = new Map<string, { name: string; ids: string[]; features: number }>()
    for (const d of data.datasets.data?.items ?? []) {
      if (d.kind !== 'vector') continue
      const key = d.sourceId + '/' + d.datasetKey.replace(/-\d+$/, '')
      const entry = groups.get(key) ?? {
        name: d.datasetKey.startsWith('desert-maps-')
          ? d.label
          : d.datasetKey.replace(/-\d+$/, '').replaceAll('-', ' '),
        ids: [],
        features: 0,
      }
      entry.ids.push(d.id)
      entry.features += d.featureCount
      groups.set(key, entry)
    }
    return [...groups.values()]
  }, [data.datasets.data])

  const saveView = () => {
    if (!bookmarkName.trim() || !analyst.input || analyst.error) return
    setBookmarks((v) =>
      [
        ...v,
        {
          id: crypto.randomUUID(),
          name: bookmarkName.trim().slice(0, 80),
          camera,
          scope: analyst.input,
          publicView: analyst.searchFor().publicView,
          layers,
        },
      ].slice(-20),
    )
    setBookmarkName('')
  }

  const shareLink = () => {
    const link = new URL('/map', window.location.origin)
    if (tool) link.searchParams.set('tool', tool)
    if (search.workbench) link.searchParams.set('workbench', search.workbench)
    if (search.selected) link.searchParams.set('selected', search.selected)
    for (const [key, value] of Object.entries(
      analyst.searchFor(activeScope ? fixedScopeInput(activeScope) : analyst.input),
    )) {
      if (value !== undefined) link.searchParams.set(key, value)
    }
    link.searchParams.set('mapState', JSON.stringify({ camera, layers }))
    return link.href
  }

  const searchViewport = () => {
    if (!viewport) {
      setMapError('The current map view is unavailable.')
      return
    }
    const [w, s, e, n] = viewport
    const width = e - w
    if (width <= 0 || width >= 360 || width * (n - s) > 400) {
      setMapError('Zoom in before using this view as a bounded area.')
      return
    }
    const lng = (x: number) => (((x + 180) % 360) + 360) % 360 - 180
    const geometry = {
      type: 'Polygon',
      coordinates: [
        [
          [lng(w), s],
          [lng(e), s],
          [lng(e), n],
          [lng(w), n],
          [lng(w), s],
        ],
      ],
    }
    panels.openPanel('')
    analyst.changeArea({ kind: 'viewport', name: 'Explicit map view', geometry })
  }

  const createBufferAt = useCallback(
    async (lng: number, lat: number, radiusM: number) => {
      areaRequest.current?.abort()
      const request = new AbortController()
      areaRequest.current = request
      try {
        if (!Number.isFinite(lng) || !Number.isFinite(lat) || !Number.isFinite(radiusM) || radiusM <= 0) {
          throw new Error('Enter WGS84 longitude,latitude and a radius.')
        }
        const feature = await api.createSurroundingArea(
          { longitude: lng, latitude: lat, radiusM },
          request.signal,
        )
        if (request.signal.aborted) return
        const f = feature as unknown as Feature
        locate(f)
        panels.openPanel('')
        analyst.changeArea({
          kind: 'named',
          name: `Deliberate ${radiusM} m context buffer`,
          geometry: f.geometry as unknown as Record<string, unknown>,
        })
      } catch (e) {
        if (!request.signal.aborted) {
          setMapError(e instanceof Error ? e.message : 'Context buffer could not be prepared.')
        }
      }
    },
    [api, analyst, panels],
  )

  const surrounding = async () => {
    const p = coordinates.trim() ? coordinates.split(',').map(Number) : [camera.lng, camera.lat]
    if (p.length !== 2 || !p.every(Number.isFinite) || !radius.trim()) {
      setMapError('Enter WGS84 longitude,latitude and a radius.')
      return
    }
    await createBufferAt(p[0], p[1], Number(radius))
  }

  const keyboardCoordinates = () => {
    const p = longitudeLatitude(coordinates)
    if (!p) {
      setMapError('Enter longitude,latitude in WGS84 decimal degrees. Use signed west/south values.')
      return
    }
    if (mode === 'inspect') cameraRef.current?.focus(p[0], p[1])
    else onDraw(p)
  }

  const inspectRecord = () => {
    const k = props.kind
    if (k === 'weather' || k === 'bulletins') openTool('weather', { id: props.recordId, kind: k === 'weather' ? 'WEATHER' : 'REPORT' })
    else if (k === 'aircraft' || k === 'vessels') openTool('tracks', { id: props.recordId })
    else if (k === 'imagery') {
      setCaptureId(props.recordId)
      openTool('imagery')
    } else if (k === 'findings') openTool('imagery', { view: 'analysis', job: props.jobId })
    else if (k === 'coverage') openTool('sources')
    else if (props.dataset_id) {
      data.selectDataset(props.dataset_id)
      openTool('geography')
    } else {
      data.selectEvidence(props.evidenceId)
      openTool('geography')
    }
  }

  const layerStatus = useMemo(() => {
    const entries = allLayers.map((k) => {
      const q = mapQueries[queryLayers.indexOf(k)]
      const v = display[k]
      const item: LayerDisplayStatus = {
        total: v?.total,
        state: v?.state,
        isError: Boolean(q?.isError),
        countMeaning: v?.countMeaning,
      }
      return [k, item] as const
    })
    return Object.fromEntries(entries) as Record<AtlasLayer, LayerDisplayStatus>
  }, [display, ...mapQueries.map((q) => q.isError)])

  const activeContextStatus = projectContextStatus({
    layer: display[family],
    sources:
      lostLayerAccess || scopeCoverage.isError
        ? []
        : scopeCoverage.data?.sources.filter((s) => s.family === family) ?? [],
    receipts: lostLayerAccess ? [] : mapBatch.data?.coverageReceipts,
    now: Date.now(),
    failed: mapQueries[queryLayers.indexOf(family)]?.isError,
    retained: !!display[family]?.features.length,
  })

  const coverageSummary = summarizeCoverageReadiness(
    scopeCoverage.data,
    scopeCoverage.isPending,
    scopeCoverage.isError,
  )

  const ensureFamiliesEnabled = useCallback(
    (families: string[]) => {
      const additions = families.filter(
        (f): f is AtlasLayer =>
          f in layerInfo && session.permissions.includes(layerInfo[f as AtlasLayer].permission),
      )
      if (additions.length === 0) return
      setLayers((prev) => {
        const next = new Set(prev)
        let changed = false
        for (const f of additions) {
          if (!next.has(f)) {
            next.add(f)
            changed = true
          }
        }
        return changed ? [...next] : prev
      })
    },
    [session.permissions, setLayers],
  )

  const jumpToBucket = useCallback(
    (bucket: InventoryTimeBucket) => {
      const rankedFamilies = Object.entries(bucket.byFamily ?? {})
        .sort((a, b) => b[1] - a[1])
        .map(([k]) => k)
      ensureFamiliesEnabled(rankedFamilies)
      const topLayer = rankedFamilies.find((f): f is AtlasLayer => f in layerInfo)
      if (topLayer) setFamily(topLayer)
      setPaused(false)
      setSelected([])
      analyst.setScope(scopeForBucket(bucket, analyst.input?.area, activeScope?.temporal.timezone))
    },
    [ensureFamiliesEnabled, analyst, activeScope?.temporal.timezone],
  )

  const jumpToFootprint = useCallback(
    (fp: InventoryFootprint) => {
      ensureFamiliesEnabled(fp.families)
      setPaused(false)
      setSelected([])
      analyst.setScope(scopeForFootprint(fp, analyst.input?.temporal))
    },
    [ensureFamiliesEnabled, analyst],
  )

  const jumpToInventorySource = useCallback(
    (src: InventorySource) => {
      ensureFamiliesEnabled([src.family])
      if (src.family in layerInfo) setFamily(src.family as AtlasLayer)
      setPaused(false)
      setSelected([])
      analyst.setScope(scopeForInventorySource(src, analyst.input, activeScope?.temporal.timezone))
    },
    [ensureFamiliesEnabled, analyst, activeScope?.temporal.timezone],
  )

  const openBrowseAllRetained = useCallback(() => {
    setRecordsInitialMode('all')
    setRecordsModeKey((k) => k + 1)
    panels.openPanel('records', 'Map records')
  }, [panels])

  const evidenceLayersForEmptyCheck = enabledLayers.filter((k) => k !== 'coverage')
  const countedLayersForEmptyCheck =
    evidenceLayersForEmptyCheck.length > 0 ? evidenceLayersForEmptyCheck : enabledLayers
  const visibleFeatureCount = countedLayersForEmptyCheck.reduce(
    (sum, k) => sum + (rendered[k]?.features.length ?? 0),
    0,
  )
  const canToggleCoverageFootprints = session.permissions.includes(layerInfo.coverage.permission)
  const footprintCount = coverageInventory.data?.footprints.length ?? 0

  const handleCloseDrawer = useCallback(() => {
    if (panels.panel) {
      const triggerLabel = panels.panelTriggerRef.current
      panels.openPanel('')
      requestAnimationFrame(() => {
        const target = triggerLabel
          ? document.querySelector<HTMLElement>(`[aria-label="${CSS.escape(triggerLabel)}"]`)
          : document.querySelector<HTMLElement>('.map-rail-item')
        if (target?.getClientRects().length) target.focus()
      })
    } else if (tool) {
      closeTool()
    }
  }, [panels, tool, closeTool])

  // Focus first control inside drawer when a support panel opens
  useEffect(() => {
    if (!panels.panel) return
    drawerRef.current?.querySelector<HTMLButtonElement>('button')?.focus()
  }, [panels.panel])

  const selectWorldScope = useCallback(() => {
    setSketchArea(undefined)
    setSelected([])
    analyst.changeArea({ kind: 'world' })
    cameraRef.current?.fitBounds([-165, -55, 165, 75])
  }, [analyst])

  const toggleCoverageDrawer = useCallback(() => {
    if (!session.permissions.includes('sources.read')) return
    if (panels.hasDrawerOpen && !panels.panel && tool === 'coverage') closeTool()
    else openTool('coverage')
  }, [session.permissions, panels.hasDrawerOpen, panels.panel, tool, closeTool, openTool])

  const toggleInvestigationsDrawer = useCallback(() => {
    if (!session.permissions.includes('investigations.read')) return
    if (panels.hasDrawerOpen && !panels.panel && tool === 'investigations') closeTool()
    else openTool('investigations')
  }, [session.permissions, panels.hasDrawerOpen, panels.panel, tool, closeTool, openTool])

  const handleRunOmniboxCommand = useCallback(
    (cmd: QuickCommandId) => {
      if (cmd === 'layers') panels.openPanel('layers', 'Map layers')
      else if (cmd === 'records') panels.openPanel('records', 'Map records')
      else if (cmd === 'coverage') openTool('coverage')
      else if (cmd === 'investigations') openTool('investigations')
      else if (cmd === 'imagery') openTool('imagery')
      else if (cmd === 'measure') {
        setMode('measure')
        setDraw([])
        setMeasurement('')
        panels.openPanel('')
      } else if (cmd === 'area') {
        setMode('area')
        setDraw([])
        setMeasurement('')
        panels.openPanel('')
      } else if (cmd === 'focus') panels.toggleCleanMap()
      else if (cmd === 'world') selectWorldScope()
      else if (cmd === 'shortcuts') setShortcutsOpen(true)
    },
    [panels, openTool, selectWorldScope],
  )

  const handleJumpToCoordinates = useCallback(
    (lng: number, lat: number, withBuffer = false) => {
      if (withBuffer) {
        void createBufferAt(lng, lat, 10000)
        return
      }
      cameraRef.current?.focus(lng, lat)
      setMapError(`Centered camera at ${lat.toFixed(4)}°, ${lng.toFixed(4)}° (WGS84).`)
    },
    [createBufferAt],
  )

  const handleSelectLoadedRecord = useCallback(
    (layer: AtlasLayer, feature: Feature) => {
      setFamily(layer)
      inspect([feature])
      locate(feature)
    },
    [inspect],
  )

  // Global guarded keyboard shortcuts for /map (L, R/B, C, I, T, V, F, /, Ctrl+K, [, ?, Esc)
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented) return

      // Ctrl+K / Cmd+K always focuses Omnibox unless a modal dialog is open
      if ((event.ctrlKey || event.metaKey) && !event.altKey && event.key.toLowerCase() === 'k') {
        if (document.querySelector('[role="dialog"][aria-modal="true"], dialog[open]')) return
        event.preventDefault()
        if (panels.cleanMap) panels.setCleanMap(false)
        setSearchOpen(true)
        requestAnimationFrame(() => {
          searchInputRef.current?.focus()
          searchInputRef.current?.select()
        })
        return
      }

      if (event.metaKey || event.ctrlKey || event.altKey) return

      const target = event.target as HTMLElement | null
      const tag = target?.tagName.toLowerCase() ?? ''
      const inDialog = Boolean(
        target?.closest('dialog, [role="dialog"]') ||
          document.querySelector('[role="dialog"][aria-modal="true"], dialog[open]'),
      )

      if (event.key === 'Escape') {
        if (inDialog) return
        if (target instanceof HTMLInputElement && target.type === 'search' && target.value) return
        if (contextMenu) {
          event.preventDefault()
          setContextMenu(null)
          return
        }
        if (searchOpen) {
          event.preventDefault()
          setSearchOpen(false)
          return
        }
        if (mode !== 'inspect') {
          event.preventDefault()
          setMode('inspect')
          setDraw([])
          return
        }
        if (panels.hasDrawerOpen) {
          event.preventDefault()
          handleCloseDrawer()
          return
        }
        if (selected.length > 0) {
          event.preventDefault()
          setSelected([])
          void navigate({ search: (p) => ({ ...p, selected: undefined }), replace: true })
        }
        return
      }

      if (
        inDialog ||
        tag === 'input' ||
        tag === 'textarea' ||
        tag === 'select' ||
        target?.isContentEditable
      ) {
        return
      }

      const key = event.key
      const lower = key.toLowerCase()

      if (key === '/') {
        event.preventDefault()
        if (panels.cleanMap) panels.setCleanMap(false)
        setSearchOpen(true)
        requestAnimationFrame(() => {
          searchInputRef.current?.focus()
          searchInputRef.current?.select()
        })
      } else if (key === '?') {
        event.preventDefault()
        setShortcutsOpen((v) => !v)
      } else if (key === '[') {
        event.preventDefault()
        if (panels.hasDrawerOpen) {
          handleCloseDrawer()
        } else {
          panels.openPanel(panels.lastPanelRef.current)
        }
      } else if (lower === 'l') {
        event.preventDefault()
        panels.togglePanel('layers', 'Map layers')
      } else if (lower === 'r' || lower === 'b') {
        event.preventDefault()
        panels.togglePanel('records', 'Map records')
      } else if (lower === 't') {
        event.preventDefault()
        panels.togglePanel('tools', 'Map tools')
      } else if (lower === 'v') {
        event.preventDefault()
        panels.togglePanel('views', 'Map views')
      } else if (lower === 'c') {
        event.preventDefault()
        toggleCoverageDrawer()
      } else if (lower === 'i') {
        event.preventDefault()
        toggleInvestigationsDrawer()
      } else if (lower === 'f') {
        event.preventDefault()
        panels.toggleCleanMap()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [
    panels,
    contextMenu,
    searchOpen,
    mode,
    selected.length,
    handleCloseDrawer,
    navigate,
    toggleCoverageDrawer,
    toggleInvestigationsDrawer,
  ])

  const openLayersAt = (tab: LayersSubTab) => {
    setLayersInitialTab(tab)
    setLayersTabKey((v) => v + 1)
    panels.openPanel('layers', 'Map layers')
  }

  const handleAttachToCase = (citation: { kind: CaseCitationKind; id: string; label?: string }) => {
    queueAttachment(caseBridgeScope, citation)
    if (tool !== 'investigations' || !panels.hasDrawerOpen) {
      openTool('investigations')
    }
    setMapError(`Queued "${citation.label || citation.id}" for the active investigation.`)
  }

  const validReferenceDrawing =
    canReadGeography &&
    referenceDrawing &&
    data.showReference &&
    data.selectedLayers.includes(referenceDrawing.datasetId) &&
    data.datasets.data?.items.some((item) => item.id === referenceDrawing.datasetId)
      ? referenceDrawing
      : undefined

  const showDiscoveryBanner =
    !panels.cleanMap &&
    mode === 'inspect' &&
    !inspectingEnvironment &&
    !environmentHandoff &&
    unavailableLayers.length === 0 &&
    selected.length === 0 &&
    !pinnedPlace &&
    !validReferenceDrawing &&
    Boolean(activeScope) &&
    !mapBatch.isPending &&
    !mapBatch.isError &&
    enabledLayers.length > 0 &&
    visibleFeatureCount === 0

  // Single unified drawer metadata — never displays "Workbenches" above "Map layers"
  const drawerMeta = useMemo(() => {
    if (panels.panel === 'layers') {
      return {
        title: 'Map layers',
        subtitle: 'Evidence visibility, imagery overlays and basemap',
        regionLabel: 'Map layers',
      }
    }
    if (panels.panel === 'records') {
      return {
        title: 'Scoped records',
        subtitle: `${layerInfo[family].name} · ${regionName}`,
        regionLabel: 'Map records',
      }
    }
    if (panels.panel === 'tools') {
      return {
        title: 'Spatial tools',
        subtitle: 'Measure, draw study polygon, viewport scope and buffer',
        regionLabel: 'Investigation tools',
      }
    }
    if (panels.panel === 'views') {
      return {
        title: 'Saved views',
        subtitle: 'Private bookmarks and authorized view links',
        regionLabel: 'Saved map views',
      }
    }
    if (panels.panel === 'simulate') {
      return {
        title: '3D Simulation Setup',
        subtitle: 'Place radar, threat & wind on the map, then launch 3D replay',
        regionLabel: '3D simulation setup',
      }
    }
    if (panels.panel === 'coverage') {
      return {
        title: 'Sensor Coverage & Blind Spots',
        subtitle: 'Place radar, EO/IR & acoustic sensors on terrain, then optimize siting in 3D',
        regionLabel: 'Sensor coverage setup',
      }
    }
    if (tool) {
      return {
        title: toolNames[tool],
        subtitle: regionName,
        regionLabel: 'Map preview dock',
      }
    }
    return { title: '', subtitle: '', regionLabel: 'Map preview dock' }
  }, [panels.panel, tool, family, regionName])

  const canAttachToCase =
    session.permissions.includes('investigations.edit') || session.permissions.includes('investigations.create')

  const fullPageLink = !panels.panel
    ? fullPageForTool(tool, analyst.searchFor(activeScope ? fixedScopeInput(activeScope) : analyst.input))
    : undefined

  return (
    <div className="public-map-workspace map-workspace atlas-workspace" aria-label="Public Context map workspace">
      {analyst.error && (
        <div role="alert" className="ux-notice">
          {analyst.error}{' '}
          <button
            type="button"
            className="ux-button"
            onClick={() => analyst.setScope({ area: { kind: 'world' }, temporal: monitoringPeriod() })}
          >
            Start a new view
          </button>
        </div>
      )}
      {activeScope?.area.display.state === 'partial' && (
        <p role="status" className="ux-notice">
          {activeScope.area.display.message}
        </p>
      )}
      {(data.filterWarning || regionQuery.isError || (!!regionID && regionQuery.isSuccess && !scope)) && (
        <p className="ux-notice" role="alert">
          {data.filterWarning || 'Selected geography unavailable. Choose another admitted region.'}
        </p>
      )}

      {/* ROLE 1: Unified 44px Scope, Smart Omnibox & Readiness Bar */}
      {!panels.cleanMap && (
        <MapScopeBar
          ariaLabel={data.savedFilters.view !== 'latest' ? 'Historical report map' : 'Public map scope'}
          searchInputRef={searchInputRef}
          query={query}
          searchOpen={searchOpen}
          searchPending={searchResults.isPending}
          searchError={searchResults.isError}
          searchFeatures={searchFeatures}
          searchHasNextPage={Boolean(searchResults.hasNextPage)}
          searchFetchingNext={searchResults.isFetchingNextPage}
          renderedFeatures={rendered}
          permissions={session.permissions}
          onQueryChange={setQuery}
          onSearchOpenChange={setSearchOpen}
          onSearchFetchNext={() => void searchResults.fetchNextPage()}
          onChooseRegion={chooseRegion}
          onJumpToCoordinates={handleJumpToCoordinates}
          onSelectLoadedRecord={handleSelectLoadedRecord}
          onRunCommand={handleRunOmniboxCommand}
          parents={parents}
          regionName={regionName}
          canFitArea={Boolean(activeScope?.area.components.length)}
          onSelectWorld={selectWorldScope}
          onFitArea={() => {
            const parts = activeScope?.area.components
            if (parts?.length) {
              cameraRef.current?.fitBounds(parts[0].bounds as [number, number, number, number])
              if (parts.length > 1) {
                setMapError(
                  `Showing component 1 of ${parts.length}. All components remain in the analytical scope; choose another in Geography.`,
                )
              }
            } else {
              locate(sketchArea ?? scope)
            }
          }}
          timeOpen={panels.timePopoverOpen}
          onTimeOpenChange={panels.setTimePopoverOpen}
          periodTitle={periodTitle}
          periodDetail={periodDetail}
          timeLabel={timeLabel}
          historical={historical}
          rollingHours={rollingHours}
          activeFilterCount={activeFilterCount}
          publicData={data}
          onPresetTime={presetTime}
          timeBuckets={coverageInventory.data?.timeBuckets}
          onJumpToBucket={jumpToBucket}
          canReadSources={session.permissions.includes('sources.read')}
          coverageOpen={panels.hasDrawerOpen && !panels.panel && tool === 'coverage'}
          coverageSummary={coverageSummary}
          onToggleCoverage={toggleCoverageDrawer}
        />
      )}

      {/* ROLE 2–4: Fixed 58px Left Rail + Content-Aware Resizable Drawer + Unobstructed Map Canvas */}
      <div className="map-stage">
        <MapRail
          hidden={panels.cleanMap}
          activePanel={panels.panel}
          activeTool={tool}
          drawerOpen={panels.hasDrawerOpen}
          permissions={session.permissions}
          enabledLayerCount={enabledLayers.length}
          unavailableLayerCount={unavailableLayers.length}
          caseCitationCount={attachedKeys.length}
          onTogglePanel={(target, triggerLabel) => panels.togglePanel(target, triggerLabel)}
          onToggleTool={(nextTool) => {
            if (panels.hasDrawerOpen && !panels.panel && tool === nextTool) {
              if (panels.timePopoverOpen) {
                panels.setTimePopoverOpen(false)
              } else {
                closeTool()
              }
            } else {
              openTool(nextTool)
            }
          }}
          onChooseWorkbench={(next) => {
            if (next === 'previews') {
              if (tool) {
                panels.openPanel('records', 'Map records', { keepOnToolClear: true })
                void navigate({ search: (p) => ({ ...p, tool: undefined }) })
              } else {
                panels.openPanel('records', 'Map records')
              }
            } else {
              openTool(next)
            }
          }}
        />

        <MapDrawer
          drawerRef={drawerRef}
          open={panels.hasDrawerOpen}
          title={drawerMeta.title}
          subtitle={drawerMeta.subtitle}
          regionLabel={drawerMeta.regionLabel}
          closeLabel={panels.panel ? 'Close map tool' : 'Close workbench'}
          width={panels.drawerWidth}
          isWideWidth={panels.isWideWidth}
          size={panels.drawerSize}
          fullPageLink={fullPageLink}
          onResizeWidth={(nextW) => panels.setBoundedWidth(nextW, layoutViewport.width)}
          onToggleMaximize={panels.toggleMaximize}
          onToggleWide={panels.toggleWideWidth}
          onClose={handleCloseDrawer}
        >
          {/* Preserved mounted workbench instances (hidden while a support panel is in front) */}
          <div hidden={Boolean(panels.panel)}>
            <MapWorkbenches
              tool={tool}
              host={host}
              routeKey={search.workbench}
              geography={
                <div className="ux-page space-y-3">
                  <header className="space-y-1">
                    <h2 className="text-base font-semibold">Geography &amp; reference</h2>
                    <p className="ux-muted">{regionName}</p>
                  </header>
                  <ReferenceDepthPanel />
                  <div className="catalog-paging">
                    <p role="status">
                      {countries.isError
                        ? 'Country boundaries could not load.'
                        : countries.isPending
                          ? 'Loading country boundaries…'
                          : `${countryFeatures.length} country boundaries loaded`}
                    </p>
                    {countries.hasNextPage && (
                      <button
                        type="button"
                        className="ux-button"
                        disabled={countries.isFetching}
                        onClick={() => void countries.fetchNextPage()}
                      >
                        More countries
                      </button>
                    )}
                    <button
                      type="button"
                      className="ux-button"
                      disabled={countries.isFetching}
                      onClick={() => void countries.refetch()}
                    >
                      Refresh countries
                    </button>
                  </div>
                  {scope && <CatalogPaging catalog={children} label="subdivisions" />}
                  {scope && (
                    <details className="atlas-region-provenance" open>
                      <summary>Selected area &amp; source</summary>
                      <dl>
                        <dt>Meaning</dt>
                        <dd>{scope.properties?.meaning}</dd>
                        <dt>Version / edition</dt>
                        <dd>
                          {scope.properties?.version} · {scope.properties?.editionId}
                        </dd>
                        <dt>Attribution</dt>
                        <dd>{scope.properties?.attribution}</dd>
                        <dt>Canonical area geometry SHA-256</dt>
                        <dd>{activeScope?.area.geometrySha256}</dd>
                        <dt>Source edition artifact SHA-256</dt>
                        <dd>{scope.properties?.sha256}</dd>
                      </dl>
                      <p>Generalized reference geometry does not certify jurisdiction or current source coverage.</p>
                    </details>
                  )}
                  {activeScope && activeScope.area.components.length > 1 && (
                    <div aria-label="Area components" className="flex flex-wrap gap-2">
                      {activeScope.area.components.map((part, i) => (
                        <button
                          type="button"
                          className="ux-button"
                          key={i}
                          onClick={() =>
                            cameraRef.current?.fitBounds(part.bounds as [number, number, number, number])
                          }
                        >
                          Fit component {i + 1}
                        </button>
                      ))}
                    </div>
                  )}
                  <div className="flex flex-wrap gap-2">
                    <button type="button" className="ux-button" onClick={() => openLayersAt('evidence')}>
                      Manage layers
                    </button>
                    <button type="button" className="ux-button" onClick={() => panels.openPanel('tools', 'Map tools')}>
                      Subdivisions &amp; study areas
                    </button>
                  </div>
                  <PublicContextPanel
                    embedded
                    data={data}
                    focus={(lng, lat) => cameraRef.current?.focus(lng, lat)}
                    replaying={false}
                  />
                </div>
              }
            />
          </div>

          {panels.panel === 'layers' && (
            <LayersPanel
              key={layersTabKey}
              initialTab={layersInitialTab}
              layers={layers}
              defaultLayers={defaultLayers}
              permissions={session.permissions}
              layerStatus={layerStatus}
              onToggleLayer={toggleLayer}
              onApplyPreset={setLayers}
              onBrowseLayer={(layer) => {
                setFamily(layer)
                setRecordsInitialMode('view')
                setRecordsModeKey((n) => n + 1)
                panels.openPanel('records', 'Map records')
              }}
              boundaryLevel={boundaryLevel}
              onBoundaryLevelChange={setBoundaryLevel}
              basemap={basemap}
              onBasemapChange={setBasemap}
              captures={captures}
              compatibleCaptures={compatibleCaptures}
              captureId={captureId}
              compareId={compareId}
              onCaptureIdChange={setCaptureId}
              onCompareIdChange={setCompareId}
              quality={quality}
              onQualityChange={setQuality}
              opacity={opacity}
              onOpacityChange={setOpacity}
              swipe={swipe}
              onSwipeChange={setSwipe}
              hasComparison={Boolean(comparison)}
              rasterError={rasterQueries.some((q) => q.isError)}
              rasters={rasters}
              layerSearch={layerSearch}
              onLayerSearchChange={setLayerSearch}
              referenceGroups={referenceGroups}
              referenceOpacity={referenceOpacity}
              onReferenceOpacityChange={setReferenceOpacity}
              publicData={data}
              footprintCount={footprintCount}
            />
          )}

          {panels.panel === 'records' && (
            <RecordsPanel
              family={family}
              allLayers={allLayers}
              enabledLayers={layers}
              layerStatus={layerStatus}
              records={previews}
              selectedRecordId={props.recordId ? String(props.recordId) : undefined}
              selectedFeature={record}
              timeLabel={timeLabel}
              coverage={scopeCoverage.data}
              activeContextStatus={activeContextStatus}
              inventory={coverageInventory.data}
              inventoryLoading={coverageInventory.isPending}
              initialBrowseMode={recordsInitialMode}
              browseModeKey={recordsModeKey}
              onChooseFamily={setFamily}
              onEnableLayer={toggleLayer}
              onSelectFeature={(f) => inspect([f])}
              onLocateSelected={() => locate(record)}
              onOpenTool={openTool}
              onOpenCoverage={() => openTool('coverage')}
              onPresetHistory={() => presetTime()}
              onJumpToBucket={jumpToBucket}
              onJumpToSource={jumpToInventorySource}
              onJumpToFootprint={jumpToFootprint}
            />
          )}

          {panels.panel === 'tools' && (
            <SpatialToolsPanel
              mode={mode}
              drawCount={draw.length}
              measurement={measurement}
              hasSketchArea={Boolean(sketchArea)}
              coordinates={coordinates}
              radius={radius}
              subdivisions={collection(children.data).features}
              onSetMode={(nextMode) => {
                setMode(nextMode)
                setDraw([])
                if (nextMode !== 'inspect') {
                  setMeasurement('')
                  panels.openPanel('')
                }
              }}
              onCoordinatesChange={setCoordinates}
              onRadiusChange={setRadius}
              onKeyboardCoordinates={keyboardCoordinates}
              onFinishSketch={() => void finishSketch()}
              onReturnNamedArea={() => {
                analyst.changeArea({ kind: 'world' })
                setDraw([])
              }}
              onCreateBuffer={() => void surrounding()}
              onUseViewportArea={searchViewport}
              onChooseSubdivision={chooseRegion}
            />
          )}

          {panels.panel === 'views' && (
            <SavedViewsPanel
              bookmarkName={bookmarkName}
              bookmarks={bookmarks}
              onBookmarkNameChange={setBookmarkName}
              onSaveView={saveView}
              onRestoreBookmark={(b) => {
                setLayers(b.layers)
                setCamera(b.camera)
                cameraRef.current?.restore(b.camera)
                void navigate({
                  search: (p) => ({
                    ...p,
                    ...(b.scope
                      ? analyst.searchFor(b.scope)
                      : { scope: undefined, region: b.region || 'world', area: b.area, publicView: b.publicView }),
                  }),
                })
                panels.openPanel('')
              }}
              onDeleteBookmark={(id) => setBookmarks((v) => v.filter((x) => x.id !== id))}
              onCopyShareLink={() => {
                void navigator.clipboard
                  ?.writeText(shareLink())
                  .then(() => setMapError('View link copied. The recipient needs their own grants.'))
                  .catch(() => setMapError('Copy the address bar to share this authorized view.'))
              }}
            />
          )}

          {panels.panel === 'simulate' && (
            <SimulationSetupPanel
              activeTool={simTool}
              onSelectTool={setSimTool}
              onOverlayChange={setSimOverlay}
              onFitBounds={(bounds) => cameraRef.current?.fitBounds(bounds)}
              onNotice={setMapError}
              registerMapClick={registerSimMapClick}
            />
          )}

          {panels.panel === 'coverage' && (
            <CoverageSetupPanel
              activeTool={covTool}
              onSelectTool={setCovTool}
              onOverlayChange={setSimOverlay}
              onFitBounds={(bounds) => cameraRef.current?.fitBounds(bounds)}
              onNotice={setMapError}
              registerMapClick={registerSimMapClick}
            />
          )}
        </MapDrawer>

        {/* MAP CANVAS REGION */}
        <div className="public-map-canvas atlas-map-region">
          <AtlasCanvas
            proposalOverlay={proposalOverlay}
            simOverlay={panels.panel === 'simulate' || panels.panel === 'coverage' ? simOverlay : empty}
            simClickActive={
              (panels.panel === 'simulate' && simTool !== null) ||
              (panels.panel === 'coverage' && covTool !== null)
            }
            onSimClick={handleSimMapClick}
            cameraRef={cameraRef}
            initialCamera={camera}
            onCamera={setCamera}
            onPointer={updatePointer}
            onBounds={setViewport}
            regions={geographicFeatures}
            scope={exactSelection ?? sketchArea ?? scope}
            selection={retainedSelection ?? record}
            referencePlace={canReadGeography ? pinnedPlace : undefined}
            boundaryLevel={boundaryLevel}
            features={rendered}
            datasets={data.showReference ? data.selectedLayers : []}
            referenceOpacity={referenceOpacity}
            basemap={LOCAL_BASEMAP_ONLY ? 'reference' : basemap}
            streetArchive={streetMap?.bytes}
            onSelect={inspect}
            onRegion={chooseRegion}
            onReference={setReferenceDrawing}
            onContextMenu={setContextMenu}
            onMoveStart={() => setContextMenu(null)}
            mode={mode}
            onDraw={onDraw}
            sketch={sketch}
            rasters={visibleRasters}
            swipe={swipe}
            onError={setMapError}
          />

          <MapCanvasHud
            cleanMap={panels.cleanMap}
            onToggleCleanMap={panels.toggleCleanMap}
            onRestoreWorkspace={() => panels.setCleanMap(false)}
            mode={mode}
            drawCount={draw.length}
            onOpenCoordinates={() => panels.openPanel('tools', 'Map tools')}
            onUndoDraw={() => setDraw((v) => v.slice(0, -1))}
            onFinishDraw={() => void finishSketch()}
            onCancelDraw={() => {
              setMode('inspect')
              setDraw([])
            }}
            paused={paused}
            pauseTime={pauseTime}
            bearing={camera.bearing}
            scopeKey={scopeKey}
            updateLayers={updateLayers}
            onInspectUpdate={(kind, f) => {
              setFamily(kind)
              inspect([f])
            }}
            onRefreshDisplay={() => {
              void mapBatch.refetch()
              void scopeCoverage.refetch()
              void coverageInventory.refetch()
              analyst.refresh()
            }}
            onPreviousCamera={() => cameraRef.current?.previous()}
            onTogglePause={() => {
              setPaused((v) => !v)
              if (!paused) {
                setFrozen({ key: scopeKey, layers: currentLayers })
                setPauseTime(new Date().toISOString())
              }
            }}
            onZoomIn={() => cameraRef.current?.zoomIn()}
            onZoomOut={() => cameraRef.current?.zoomOut()}
            onResetNorth={() => cameraRef.current?.north()}
          />

          {/* Zero-Collision Top-Left Flex Notice Stack */}
          {(showDiscoveryBanner ||
            environmentHandoff ||
            (unavailableLayers.length > 0 && !panels.cleanMap && !inspectingEnvironment)) && (
            <div className="map-top-left-stack">
              {showDiscoveryBanner && (
                <MapDiscoveryBanner
                  scopeKey={scopeKey}
                  regionName={regionName}
                  periodTitle={periodTitle}
                  inventory={coverageInventory.data}
                  coverage={scopeCoverage.data}
                  coverageFootprintsOn={layers.includes('coverage')}
                  canToggleCoverageFootprints={canToggleCoverageFootprints}
                  canRequestAcquisition={session.permissions.includes('acquisitions.request')}
                  onJumpToBucket={jumpToBucket}
                  onJumpToFootprint={jumpToFootprint}
                  onToggleCoverageFootprints={() => toggleLayer('coverage')}
                  onOpenSmartFetch={() => openTool('coverage')}
                  onBrowseAllRetained={openBrowseAllRetained}
                />
              )}

              {environmentHandoff && (
                <aside className="atlas-data-notice" aria-label="Environment selection">
                  {selectedEnvironmentFeature && (
                    <p>
                      <bdi dir="auto">
                        {selectedEnvironmentFeature.properties.tags.name || selectedEnvironmentFeature.properties.sourceId}
                      </bdi>{' '}
                      · {selectedEnvironmentFeature.properties.sourceId}
                    </p>
                  )}
                  {selectedTerrain && <p>Terrain point · {wb.terrainPoint?.replace(',', '°, ')}°</p>}
                  {terrainID && terrainPoint.isError && (
                    <p role="status">The linked terrain point is unavailable in this workspace.</p>
                  )}
                  {featureLink && environmentFeature.isPending && <p role="status">Resolving the retained feature…</p>}
                  {featureLink && environmentFeature.isError && (
                    <p role="status">The retained feature is unavailable in this workspace.</p>
                  )}
                  {streetID && streets.isPending && <p role="status">Verifying the retained street map…</p>}
                  {streetID && streets.isError && (
                    <p role="status">
                      Retained street detail is unavailable. Regional reference geography remains available.
                    </p>
                  )}
                  {streetMap && (
                    <p>
                      Retained street detail · zoom 12–16 · {streetMap.display.bounds.map((v) => v.toFixed(3)).join(', ')} ·{' '}
                      <a href="https://www.openstreetmap.org/copyright">© OpenStreetMap contributors</a>
                    </p>
                  )}
                  <div className="flex flex-wrap gap-2">
                    {wb.display && (
                      <button
                        type="button"
                        className="ux-button"
                        onClick={() => openTool('environments', { ...wb, view: 'environment-3d' })}
                      >
                        Reopen 3D preview
                      </button>
                    )}
                    <button type="button" className="ux-button" onClick={closeTool}>
                      Clear environment selection
                    </button>
                  </div>
                </aside>
              )}

              {unavailableLayers.length > 0 && !panels.cleanMap && !inspectingEnvironment && (
                <aside className="atlas-data-notice" aria-label="Map data availability">
                  <strong>
                    {readCapacityExhausted ? 'Temporary evidence read limit reached' : 'Some evidence layers could not load'}
                  </strong>
                  <details className="atlas-recovery-details" open={layoutViewport.width > 760}>
                    <summary>Details &amp; recovery</summary>
                    <p>
                      {readCapacityExhausted
                        ? 'The map will retry as temporary reads expire. You can also close unused EMAD tabs and select Retry layers.'
                        : `${unavailableLayers.map((k) => layerInfo[k].name).join(', ')}. Retry these layers to check for retained records.`}
                    </p>
                    <div className="flex flex-wrap gap-2">
                      <button
                        type="button"
                        className="ux-button"
                        disabled={mapBatch.isFetching}
                        onClick={() => void mapBatch.refetch()}
                      >
                        {mapBatch.isFetching ? 'Retrying…' : 'Retry layers'}
                      </button>
                      <button type="button" className="ux-button" onClick={() => openTool('sources')}>
                        Check sources
                      </button>
                    </div>
                    <details>
                      <summary>Technical details</summary>
                      <p>{mapBatch.error?.message ?? 'One or more retained evidence queries are unavailable.'}</p>
                    </details>
                  </details>
                </aside>
              )}
            </div>
          )}

          {/* ROLE 3: Compact Floating Selection Peek Card (never steals the left drawer!) */}
          <SelectionPeekCard
            selected={selected}
            pinnedPlace={pinnedPlace}
            referenceDrawing={validReferenceDrawing}
            canReadGeography={canReadGeography}
            canAttachToCase={canAttachToCase}
            attachedKeys={attachedKeys}
            timeLabel={timeLabel}
            readId={mapBatch.data?.id}
            readScope={mapBatch.data?.scope}
            onClearSelection={() => {
              setSelected([])
              void navigate({ search: (p) => ({ ...p, selected: undefined }), replace: true })
            }}
            onSelectOverlapping={(value) =>
              setSelected((v) => [
                ...v.filter((f) => String(f.id ?? f.properties?.recordId ?? 0) === value),
                ...v.filter((f) => String(f.id ?? f.properties?.recordId ?? 0) !== value),
              ])
            }
            onLocate={locate}
            onInspect={inspectRecord}
            onShowPixels={(id) => {
              setCaptureId(id)
              openLayersAt('imagery')
            }}
            onAttachToCase={handleAttachToCase}
            onChooseAggregateRecord={(v) => {
              data.selectEvidence(String(v.evidenceId))
              if (!v.displayPosition) {
                setMapError(
                  'This retained report has no usable map locator. Inspect its source location and uncertainty.',
                )
                return
              }
              setSelected([
                {
                  type: 'Feature',
                  geometry: v.displayPosition as Geometry,
                  properties: {
                    ...v,
                    kind: props.kind,
                    evidenceId: v.evidenceId,
                    recordId: v.evidenceId,
                    title: (v.reported as { title?: string })?.title,
                  },
                },
              ])
            }}
            onClearPinnedPlace={() => setPinnedPlace(undefined)}
            onClearReferenceDrawing={() => setReferenceDrawing(undefined)}
          />

          {/* ROLE 4: Contextual Expandable Map Legend with inline layer visibility toggles */}
          {!panels.cleanMap && (
            <ContextualLegend
              open={panels.legendOpen}
              onToggle={() => panels.setLegendOpen((v) => !v)}
              enabledLayers={enabledLayers}
              layerStatus={layerStatus}
              regionLoading={Boolean(regionID && !scope)}
              rasters={rasters}
              qualityMask={quality}
              onSelectLayer={(k) => {
                setFamily(k)
                setRecordsInitialMode('view')
                setRecordsModeKey((n) => n + 1)
                panels.openPanel('records', 'Map records')
              }}
              onToggleLayer={toggleLayer}
              onOpenImageryOverlay={() => openLayersAt('imagery')}
              onOpenLayersPanel={() => openLayersAt('evidence')}
              coverageFootprintsOn={layers.includes('coverage')}
              canToggleCoverageFootprints={canToggleCoverageFootprints}
              footprintCount={footprintCount}
              onToggleCoverageFootprints={() => toggleLayer('coverage')}
            />
          )}

          {/* Right-Click Canvas Context Menu */}
          <MapContextMenu
            target={contextMenu}
            canReadSources={session.permissions.includes('sources.read')}
            onClose={() => setContextMenu(null)}
            onCopyCoords={(lng, lat) => {
              const text = `${lat.toFixed(5)}, ${lng.toFixed(5)}`
              void navigator.clipboard
                ?.writeText(text)
                .then(() => setMapError(`Copied WGS84 coordinates: ${text}`))
                .catch(() => setMapError(`WGS84 coordinates: ${text}`))
            }}
            onMeasureFromHere={(lng, lat) => {
              setMode('measure')
              setDraw([[lng, lat]])
              setMeasurement('')
              panels.openPanel('')
            }}
            onDrawAreaFromHere={(lng, lat) => {
              setMode('area')
              setDraw([[lng, lat]])
              setMeasurement('')
              panels.openPanel('')
            }}
            onBuffer10Km={(lng, lat) => {
              void createBufferAt(lng, lat, 10000)
            }}
            onCheckCoverage={() => openTool('coverage')}
            onZoomHere={(lng, lat) => cameraRef.current?.zoomToPoint(lng, lat, 2)}
          />

          {/* Bottom-Center Non-Colliding Map Notice Toast */}
          {mapError && (
            <div className="map-toast atlas-map-warning" role="status">
              <span>{mapError}</span>
              <button type="button" aria-label="Dismiss map notice" onClick={() => setMapError('')}>
                Dismiss
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Unified 28px In-Flow Telemetry, Trust & Shortcuts Strip (12px compliant) */}
      <footer className="map-telemetry-footer">
        <div className="map-footer-left">
          <span ref={coordinateLabel} className="map-footer-mono">
            Pointer · {pointerLabel(pointer.current)}
          </span>
          <span className="map-footer-mono map-footer-zoom" aria-label="Map zoom level">
            Z {camera.zoom.toFixed(1)}
          </span>
          {rasters.length > 0 && (
            <button
              type="button"
              className="map-footer-link"
              aria-label="Retained raster acquisitions"
              onClick={() => openLayersAt('imagery')}
            >
              Sentinel-2 · {dateLabel(capture?.properties?.date)}
              {comparison ? ` ↔ ${dateLabel(comparison.properties?.date)}` : ''}
              {quality ? ' · SCL exclusions' : ''}
            </button>
          )}
          {(paused || boundaryLevel !== 'auto' || measurement) && (
            <span className="map-footer-state" data-active-state={paused || boundaryLevel !== 'auto'}>
              {paused
                ? `Display paused · ${new Date(pauseTime).toLocaleTimeString()}`
                : boundaryLevel !== 'auto'
                  ? `Selecting ${boundaryLevel === 'country' ? 'countries' : 'subdivisions'}`
                  : measurement}
              {boundaryLevel !== 'auto' && (
                <button type="button" className="map-footer-link" onClick={() => setBoundaryLevel('auto')}>
                  Return to objects
                </button>
              )}
            </span>
          )}
        </div>
        <div className="map-footer-right">
          <span data-testid="workspace-basis" className="map-footer-basis">
            Public sources · retained evidence &amp; provider uncertainty
          </span>
          <span
            className="workspace-connection"
            data-connected={realtimeStatus === 'connected'}
            title="Event connection only. Source freshness is reported separately."
          >
            {realtimeStatus === 'connected' ? 'Events connected' : `Events ${realtimeStatus}`}
          </span>
          <button
            type="button"
            className="map-footer-link map-footer-keys-btn"
            onClick={() => setShortcutsOpen(true)}
            title="View keyboard shortcuts (?)"
          >
            Shortcuts (?)
          </button>
        </div>
      </footer>

      <MapShortcutsDialog open={shortcutsOpen} onClose={() => setShortcutsOpen(false)} />
    </div>
  )
}
