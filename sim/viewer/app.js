import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';

// Coordinate conversion matching EMAD prepare_display_terrain.py:
// ENU [east, north, up] -> Three.js [east, up, -north]
function enuToThree(p) {
  return new THREE.Vector3(p[0], p[2], -p[1]);
}

const WGS84_A = 6378137;
const WGS84_E2 = 6.6943799901413165e-3;
const DEG2RAD = Math.PI / 180;

function threeToLonLat(origin, east, up, south) {
  const lng = origin[0] * DEG2RAD;
  const lat = origin[1] * DEG2RAD;
  const height = origin[2];
  const sin = Math.sin(lat), cos = Math.cos(lat);
  const prime = WGS84_A / Math.sqrt(1 - WGS84_E2 * sin * sin);
  const along = (prime + height) * cos + up * cos + south * sin;
  const x = along * Math.cos(lng) - east * Math.sin(lng);
  const y = along * Math.sin(lng) + east * Math.cos(lng);
  const z = (prime * (1 - WGS84_E2) + height + up) * sin - south * cos;
  const dist = Math.hypot(x, y);
  let angle = Math.atan2(z, dist * (1 - WGS84_E2));
  for (let i = 0; i < 5; i++) {
    const s = Math.sin(angle);
    angle = Math.atan2(z + WGS84_E2 * WGS84_A * s / Math.sqrt(1 - WGS84_E2 * s * s), dist);
  }
  return [Math.atan2(y, x) / DEG2RAD, angle / DEG2RAD];
}

function disposeGroup(group) {
  group.traverse((obj) => {
    if (obj.geometry) obj.geometry.dispose();
    if (obj.material) {
      const mats = Array.isArray(obj.material) ? obj.material : [obj.material];
      for (const m of mats) {
        if (m.map) m.map.dispose();
        m.dispose();
      }
    }
  });
  group.clear();
}

// ── Procedural 3D Tactical Models (oriented along +Z forward, then lookAt'd!) ──
function createThreatModel(kind) {
  const g = new THREE.Group();
  const bodyMat = new THREE.MeshStandardMaterial({ color: 0xff5c72, emissive: 0x6b0b1a, roughness: 0.35, metalness: 0.35 });
  const wingMat = new THREE.MeshStandardMaterial({ color: 0xff8a99, emissive: 0x450610, roughness: 0.45, metalness: 0.25 });
  const glowMat = new THREE.MeshBasicMaterial({ color: 0xffea94 });

  if (kind === 'ballistic_missile') {
    const body = new THREE.Mesh(new THREE.CylinderGeometry(8, 11, 78, 12), bodyMat);
    body.rotation.x = Math.PI / 2;
    g.add(body);
    const nose = new THREE.Mesh(new THREE.ConeGeometry(8, 28, 12), wingMat);
    nose.rotation.x = Math.PI / 2;
    nose.position.z = 52;
    g.add(nose);
    for (let i = 0; i < 4; i++) {
      const fin = new THREE.Mesh(new THREE.BoxGeometry(26, 2.5, 16), wingMat);
      fin.position.z = -30;
      fin.rotation.z = (i * Math.PI) / 4;
      g.add(fin);
    }
    const flame = new THREE.Mesh(new THREE.ConeGeometry(9, 34, 10), glowMat);
    flame.rotation.x = -Math.PI / 2;
    flame.position.z = -54;
    g.add(flame);
  } else if (kind === 'cruise_missile') {
    const body = new THREE.Mesh(new THREE.CylinderGeometry(6, 7, 68, 10), bodyMat);
    body.rotation.x = Math.PI / 2;
    g.add(body);
    const nose = new THREE.Mesh(new THREE.ConeGeometry(6, 18, 10), wingMat);
    nose.rotation.x = Math.PI / 2;
    nose.position.z = 42;
    g.add(nose);
    const wings = new THREE.Mesh(new THREE.BoxGeometry(58, 2.5, 14), wingMat);
    wings.position.z = 2;
    g.add(wings);
    const tail = new THREE.Mesh(new THREE.BoxGeometry(24, 2, 10), wingMat);
    tail.position.z = -28;
    g.add(tail);
    const vfin = new THREE.Mesh(new THREE.BoxGeometry(2, 16, 10), wingMat);
    vfin.position.set(0, 8, -28);
    g.add(vfin);
    const exhaust = new THREE.Mesh(new THREE.SphereGeometry(7, 8, 8), glowMat);
    exhaust.position.z = -37;
    g.add(exhaust);
  } else {
    // Delta-wing One-Way UAV
    const fus = new THREE.Mesh(new THREE.ConeGeometry(8, 56, 8), bodyMat);
    fus.rotation.x = Math.PI / 2;
    g.add(fus);
    const shape = new THREE.Shape();
    shape.moveTo(0, 26);
    shape.lineTo(36, -20);
    shape.lineTo(0, -14);
    shape.lineTo(-36, -20);
    shape.closePath();
    const wingGeom = new THREE.ExtrudeGeometry(shape, { depth: 3, bevelEnabled: false });
    wingGeom.rotateX(Math.PI / 2);
    const wing = new THREE.Mesh(wingGeom, wingMat);
    wing.position.y = 1.5;
    g.add(wing);
    for (const sx of [-35, 35]) {
      const tip = new THREE.Mesh(new THREE.BoxGeometry(2.5, 14, 14), bodyMat);
      tip.position.set(sx, 5, -16);
      g.add(tip);
    }
    const tailGlow = new THREE.Mesh(new THREE.SphereGeometry(6, 8, 8), glowMat);
    tailGlow.position.z = -24;
    g.add(tailGlow);
  }
  return g;
}

function createInterceptorModel() {
  const g = new THREE.Group();
  const bodyMat = new THREE.MeshStandardMaterial({ color: 0xe6fff3, emissive: 0x16804d, roughness: 0.25, metalness: 0.4 });
  const finMat = new THREE.MeshStandardMaterial({ color: 0x69f0ae, emissive: 0x1d9e60 });
  const plumeMat = new THREE.MeshBasicMaterial({ color: 0xa8ffdd });
  const body = new THREE.Mesh(new THREE.CylinderGeometry(5, 6, 56, 10), bodyMat);
  body.rotation.x = Math.PI / 2;
  g.add(body);
  const nose = new THREE.Mesh(new THREE.ConeGeometry(5, 18, 10), finMat);
  nose.rotation.x = Math.PI / 2;
  nose.position.z = 36;
  g.add(nose);
  for (let i = 0; i < 2; i++) {
    const fin = new THREE.Mesh(new THREE.BoxGeometry(22, 2, 14), finMat);
    fin.position.z = -20;
    fin.rotation.z = (i * Math.PI) / 2;
    g.add(fin);
  }
  const plume = new THREE.Mesh(new THREE.ConeGeometry(7, 36, 8), plumeMat);
  plume.rotation.x = -Math.PI / 2;
  plume.position.z = -46;
  g.add(plume);
  return g;
}

function createRadarStation(pos) {
  const root = new THREE.Group();
  root.position.copy(pos);
  const mast = new THREE.Mesh(
    new THREE.CylinderGeometry(10, 18, 52, 8),
    new THREE.MeshStandardMaterial({ color: 0x26485c, roughness: 0.6, metalness: 0.4 })
  );
  mast.position.y = 26;
  root.add(mast);

  const rotor = new THREE.Group();
  rotor.position.y = 56;
  const dish = new THREE.Mesh(
    new THREE.BoxGeometry(46, 16, 6),
    new THREE.MeshStandardMaterial({ color: 0x58d5ff, emissive: 0x105570, roughness: 0.3 })
  );
  const feed = new THREE.Mesh(
    new THREE.ConeGeometry(6, 18, 8),
    new THREE.MeshBasicMaterial({ color: 0xb8f0ff })
  );
  feed.rotation.x = -Math.PI / 2;
  feed.position.z = 12;
  rotor.add(dish, feed);
  root.add(rotor);
  return { root, rotor };
}

function createLauncherStation(pos, targetPos) {
  const root = new THREE.Group();
  root.position.copy(pos);
  const base = new THREE.Mesh(
    new THREE.BoxGeometry(48, 18, 34),
    new THREE.MeshStandardMaterial({ color: 0x244738, roughness: 0.7 })
  );
  base.position.y = 9;
  root.add(base);

  const turret = new THREE.Group();
  turret.position.y = 22;
  const dir = new THREE.Vector3().subVectors(targetPos, pos);
  turret.rotation.y = Math.atan2(dir.x, dir.z);
  const tubeMat = new THREE.MeshStandardMaterial({ color: 0x69f0ae, emissive: 0x114d31, roughness: 0.35 });
  for (const ox of [-10, 10]) {
    for (const oy of [0, 10]) {
      const tube = new THREE.Mesh(new THREE.CylinderGeometry(4.2, 4.2, 44, 8), tubeMat);
      tube.position.set(ox, oy + 10, 4);
      tube.rotation.x = Math.PI * 0.32;
      turret.add(tube);
    }
  }
  root.add(turret);
  return root;
}

// ── Trajectory interpolation returning both position & forward velocity direction ──
function interpTrajectory(samples, t) {
  if (!samples || samples.length === 0) return null;
  if (t < samples[0].tS) return null;
  if (t >= samples[samples.length - 1].tS) {
    const last = samples[samples.length - 1];
    const prev = samples[Math.max(0, samples.length - 2)];
    const pos = enuToThree(last.enu);
    const vel = enuToThree(last.enu).sub(enuToThree(prev.enu));
    return { pos, vel, ended: true, tS: last.tS };
  }
  let lo = 0, hi = samples.length - 1;
  while (lo + 1 < hi) {
    const mid = (lo + hi) >> 1;
    if (samples[mid].tS <= t) lo = mid; else hi = mid;
  }
  const a = samples[lo], b = samples[hi];
  const f = (t - a.tS) / Math.max(b.tS - a.tS, 1e-6);
  const pa = enuToThree(a.enu), pb = enuToThree(b.enu);
  const vel = pb.clone().sub(pa);
  return { pos: pa.clone().lerp(pb, f), vel, ended: false, tS: t };
}

function interpTrailPoint(points, tRel) {
  if (!points || points.length === 0 || tRel < 0) return null;
  if (tRel >= points[points.length - 1][0]) {
    const last = points[points.length - 1];
    return { pos: enuToThree([last[1], last[2], last[3]]), impacted: true, reachedIndex: points.length - 1 };
  }
  let lo = 0, hi = points.length - 1;
  while (lo + 1 < hi) {
    const mid = (lo + hi) >> 1;
    if (points[mid][0] <= tRel) lo = mid; else hi = mid;
  }
  const a = points[lo], b = points[hi];
  const f = (tRel - a[0]) / Math.max(b[0] - a[0], 1e-6);
  return {
    pos: enuToThree([
      a[1] + f * (b[1] - a[1]),
      a[2] + f * (b[2] - a[2]),
      a[3] + f * (b[3] - a[3]),
    ]),
    impacted: false,
    reachedIndex: lo,
  };
}

function buildEllipseLine(fp, colorHex, dashed = false) {
  const ellPts = [];
  const ang = (fp.ellipseAngleDeg * Math.PI) / 180;
  const ca = Math.cos(ang), sa = Math.sin(ang);
  const cE = fp.centroidEnu[0], cN = fp.centroidEnu[1], cU = fp.centroidEnu[2] + 11;
  for (let i = 0; i <= 72; i++) {
    const th = (i / 72) * Math.PI * 2;
    const lx = fp.ellipse90SemiMajorM * Math.cos(th);
    const ly = fp.ellipse90SemiMinorM * Math.sin(th);
    const e = cE + lx * ca - ly * sa;
    const n = cN + lx * sa + ly * ca;
    ellPts.push(enuToThree([e, n, cU]));
  }
  const geom = new THREE.BufferGeometry().setFromPoints(ellPts);
  const mat = dashed
    ? new THREE.LineDashedMaterial({ color: colorHex, dashSize: 38, gapSize: 22, transparent: true, opacity: 0.85 })
    : new THREE.LineBasicMaterial({ color: colorHex, transparent: true, opacity: 0.95 });
  const line = new THREE.Line(geom, mat);
  if (dashed) line.computeLineDistances();
  return line;
}

function setLoading(title, stage, detail) {
  const overlay = document.getElementById('loading-overlay');
  overlay.classList.remove('hidden');
  if (title) document.getElementById('loading-title').textContent = title;
  if (stage) document.getElementById('loading-stage').textContent = stage;
  if (detail) document.getElementById('loading-detail').textContent = detail;
}

function hideLoading() {
  document.getElementById('loading-overlay').classList.add('hidden');
}

async function resolveDataEndpoints(overrideRunId = null) {
  const params = new URLSearchParams(window.location.search);
  const requestedRun = overrideRunId || params.get('run');
  const mapOrigin = params.get('mapOrigin');
  if (mapOrigin) {
    try {
      const u = new URL(mapOrigin);
      if (['127.0.0.1', 'localhost'].includes(u.hostname)) {
        document.getElementById('map-return-link').href = `${u.origin}/map`;
      }
    } catch { /* ignore */ }
  }

  const meta = await (await fetch('/data/meta.json')).json();
  if (!meta.serviceMode) {
    return {
      serviceMode: false,
      jobId: null,
      hasGlb: meta.hasGlb,
      hasImagery: meta.hasImagery,
      replayUrl: '/data/replay.json',
      glbUrl: '/data/terrain.glb',
      imageryUrl: '/data/imagery.json',
      allRuns: [],
    };
  }

  const runsPayload = await (await fetch('/api/runs')).json();
  const allRuns = runsPayload.runs || [];
  let jobId = requestedRun;

  if (!jobId) {
    const completed = allRuns.find((r) => r.status === 'completed') || allRuns[0];
    if (!completed) {
      setLoading(
        'No Simulation Runs Yet',
        'Waiting for a run from the EMAD Map…',
        'Open the EMAD Map (http://127.0.0.1:5173/map), click Simulate in the left rail, configure radar/threat/wind, and click Run Simulation.'
      );
      return null;
    }
    jobId = completed.jobId;
    const url = new URL(window.location.href);
    url.searchParams.set('run', jobId);
    window.history.replaceState({}, '', url.toString());
  }

  let job = null;
  let notFoundRetries = 0;
  while (true) {
    const res = await fetch(`/api/runs/${encodeURIComponent(jobId)}`);
    if (res.status === 404 && notFoundRetries < 25) {
      notFoundRetries += 1;
      setLoading(
        'Starting EMAD 3D Simulation…',
        'Receiving scenario from EMAD Map…',
        `Waiting for run ${jobId} to initialize on local simulation engine.`
      );
      await new Promise((r) => setTimeout(r, 250));
      continue;
    }
    if (!res.ok) throw new Error(`Simulation run "${jobId}" was not found on this service.`);
    job = await res.json();
    if (job.status === 'completed') break;
    if (job.status === 'failed' || job.status === 'interrupted') {
      throw new Error(job.error || job.stage || 'Simulation failed.');
    }
    setLoading(
      `Running: ${job.title || jobId}`,
      job.stage || 'Running 3-DOF simulation…',
      `Environment: ${job.environmentTitle || job.environmentId} · Mode: ${job.mode} · Seed: ${job.seed}`
    );
    await new Promise((r) => setTimeout(r, 260));
  }

  const latestRuns = (await (await fetch('/api/runs')).json()).runs || allRuns;
  return {
    serviceMode: true,
    jobId,
    hasGlb: Boolean(job.hasGlb),
    hasImagery: Boolean(job.hasImagery),
    replayUrl: `/api/runs/${encodeURIComponent(jobId)}/replay.json`,
    glbUrl: `/api/runs/${encodeURIComponent(jobId)}/terrain.glb`,
    imageryUrl: `/api/runs/${encodeURIComponent(jobId)}/imagery.json`,
    allRuns: latestRuns,
  };
}

async function init() {
  const statusEl = document.getElementById('status-line');
  let endpoints = await resolveDataEndpoints();
  if (!endpoints) return;

  setLoading('Loading 3D Scene…', 'Fetching simulation replay & terrain assets…');

  // Three.js Renderer, Scene, Camera, Controls (created once, reused across What-If hot reloads!)
  const container = document.getElementById('viewport');
  const renderer = new THREE.WebGLRenderer({ antialias: true, powerPreference: 'high-performance' });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 1.5));
  renderer.setSize(window.innerWidth, window.innerHeight);
  renderer.setClearColor(0x081016);
  renderer.toneMapping = THREE.ACESFilmicToneMapping;
  renderer.toneMappingExposure = 1.08;
  container.appendChild(renderer.domElement);

  const scene = new THREE.Scene();
  scene.fog = new THREE.FogExp2(0x081016, 0.000042);

  const camera = new THREE.PerspectiveCamera(48, window.innerWidth / window.innerHeight, 2, 70000);
  const controls = new OrbitControls(camera, renderer.domElement);
  controls.enableDamping = true;
  controls.dampingFactor = 0.08;
  controls.maxPolarAngle = Math.PI * 0.495;

  scene.add(new THREE.HemisphereLight(0xd9ecff, 0x3b352a, 1.45));
  const sun = new THREE.DirectionalLight(0xfff4df, 1.75);
  sun.position.set(2500, 6200, 1800);
  scene.add(sun);

  // World root supports clean visual vertical exaggeration (zScale) pivoted at terrainMinY
  const worldRoot = new THREE.Group();
  scene.add(worldRoot);

  const layerGroups = {
    terrain: new THREE.Group(),
    imagery: new THREE.Group(),
    buildings: new THREE.Group(),
    roads: new THREE.Group(),
    coverage: new THREE.Group(),
    wind: new THREE.Group(),
    staticSites: new THREE.Group(),
    optimizer: new THREE.Group(),
    ghostCompare: new THREE.Group(),
    guides: new THREE.Group(),
    truth: new THREE.Group(),
    obs: new THREE.Group(),
    shrapnel: new THREE.Group(),
  };
  Object.values(layerGroups).forEach((g) => worldRoot.add(g));

  // Mutable state shared across hot-reloaded runs
  let replay = null;
  let loadedEnvManifest = null;
  let terrainMinY = 0;
  let zScale = 1.0;
  let sceneCenter = new THREE.Vector3();
  let sceneSize = 4000;
  let radarPos = new THREE.Vector3();
  let radarRotor = null;

  let policyNames = [];
  let activePolicy = 'safe_window';
  let currentRun = null;
  let simTime = 0;
  let maxTime = 60;
  let playing = true;
  let speed = 1.0;
  let followMode = null; // null | 'threat' | 'interceptor'

  // Smooth camera transition target
  let camTransition = null; // { startPos, endPos, startTarget, endTarget, t, dur }
  const keysHeld = new Set();

  // Dynamic objects for current policy
  let threatModel, interceptorModel, burstMesh, shockRingMesh, losLine, trackHeadMesh;
  let activeThreatLine, activeIntLine;
  let threatLineCoords = [], intLineCoords = [];
  let fragMeshes = [];
  let impactMeshes = [];
  let obsDotObjects = [];
  let pickableObjects = [];

  function applyZScale(scaleVal) {
    zScale = scaleVal;
    worldRoot.scale.set(1, zScale, 1);
    worldRoot.position.y = terrainMinY * (1 - zScale);
    document.getElementById('zscale-val').textContent = `${zScale.toFixed(2)}×`;
  }

  function toWorldPos(localVec) {
    return new THREE.Vector3(localVec.x, (localVec.y - terrainMinY) * zScale + terrainMinY, localVec.z);
  }

  function animateCameraTo(targetPos, camOffset, dur = 0.55) {
    followMode = null;
    const worldT = toWorldPos(targetPos);
    camTransition = {
      startPos: camera.position.clone(),
      endPos: worldT.clone().add(camOffset),
      startTarget: controls.target.clone(),
      endTarget: worldT,
      t: 0,
      dur,
    };
  }

  // Click-to-Inspect Raycaster
  const raycaster = new THREE.Raycaster();
  const pointerDown = new THREE.Vector2();
  const inspCard = document.getElementById('inspector-card');
  const inspTitle = document.getElementById('insp-title');
  const inspBody = document.getElementById('insp-body');
  document.getElementById('insp-close').onclick = () => inspCard.classList.remove('visible');

  function showInspection(info) {
    inspTitle.textContent = info.title;
    inspBody.innerHTML = info.items
      .map(([k, v]) => `<div class="insp-kv"><span>${k}</span><strong>${v}</strong></div>`)
      .join('');
    inspCard.classList.add('visible');
  }

  renderer.domElement.addEventListener('pointerdown', (e) => {
    pointerDown.set(e.clientX, e.clientY);
  });
  renderer.domElement.addEventListener('pointerup', (e) => {
    if (Math.hypot(e.clientX - pointerDown.x, e.clientY - pointerDown.y) > 6) return;
    if (!pickableObjects.length) return;
    const rect = renderer.domElement.getBoundingClientRect();
    const ndc = new THREE.Vector2(
      ((e.clientX - rect.left) / rect.width) * 2 - 1,
      -((e.clientY - rect.top) / rect.height) * 2 + 1
    );
    raycaster.setFromCamera(ndc, camera);
    const hits = raycaster.intersectObjects(pickableObjects, false);
    if (hits.length > 0 && hits[0].object.userData.inspect) {
      showInspection(hits[0].object.userData.inspect);
    }
  });

  // ── Load / Reload Terrain & Static Environment Layers (cached if same manifest!) ──
  async function loadEnvironmentGeometry(newReplay, ep) {
    const manifestSha = newReplay.environment.manifestSha256;
    if (loadedEnvManifest === manifestSha) return;
    loadedEnvManifest = manifestSha;

    disposeGroup(layerGroups.terrain);
    disposeGroup(layerGroups.imagery);
    disposeGroup(layerGroups.buildings);
    disposeGroup(layerGroups.roads);

    let terrainObject = null;
    if (ep.hasGlb) {
      statusEl.textContent = 'Loading verified EMAD terrain GLB…';
      const gltf = await new GLTFLoader().loadAsync(ep.glbUrl);
      terrainObject = gltf.scene;
      terrainObject.traverse((o) => {
        if (o.isMesh) {
          o.material = new THREE.MeshStandardMaterial({ color: 0x6f6858, roughness: 0.92, metalness: 0.05 });
        }
      });
      layerGroups.terrain.add(terrainObject);
    } else {
      const grid = newReplay.viewerContext.fallbackTerrainGrid;
      const geom = new THREE.BufferGeometry();
      const verts = new Float32Array(grid.nx * grid.ny * 3);
      let ptr = 0;
      for (let j = 0; j < grid.ny; j++) {
        const n = grid.northMin + (j / (grid.ny - 1)) * (grid.northMax - grid.northMin);
        for (let i = 0; i < grid.nx; i++) {
          const e = grid.eastMin + (i / (grid.nx - 1)) * (grid.eastMax - grid.eastMin);
          const u = grid.up[j * grid.nx + i];
          verts[ptr++] = e; verts[ptr++] = u; verts[ptr++] = -n;
        }
      }
      const indices = [];
      for (let j = 0; j < grid.ny - 1; j++) {
        for (let i = 0; i < grid.nx - 1; i++) {
          const a = j * grid.nx + i, b = a + 1, c = a + grid.nx, d = c + 1;
          indices.push(a, c, b, b, c, d);
        }
      }
      geom.setAttribute('position', new THREE.BufferAttribute(verts, 3));
      geom.setIndex(indices);
      geom.computeVertexNormals();
      terrainObject = new THREE.Mesh(
        geom,
        new THREE.MeshStandardMaterial({ color: 0x5c685e, roughness: 0.9, metalness: 0.05, side: THREE.DoubleSide })
      );
      layerGroups.terrain.add(terrainObject);
    }

    const box = new THREE.Box3().setFromObject(layerGroups.terrain);
    sceneCenter = box.getCenter(new THREE.Vector3());
    sceneSize = Math.max(1500, box.getSize(new THREE.Vector3()).length());
    terrainMinY = box.min.y;

    // Optional Sentinel-2 optical texture drape
    if (ep.hasImagery && terrainObject) {
      try {
        statusEl.textContent = 'Draping Sentinel-2 optical imagery…';
        const imgData = await (await fetch(ep.imageryUrl)).json();
        const rawBytes = Uint8Array.from(atob(imgData.texture), (c) => c.charCodeAt(0));
        const bitmap = await createImageBitmap(new Blob([rawBytes], { type: 'image/png' }), { imageOrientation: 'flipY' });
        const texture = new THREE.Texture(bitmap);
        texture.colorSpace = THREE.SRGBColorSpace;
        texture.flipY = false;
        texture.minFilter = THREE.NearestFilter;
        texture.magFilter = THREE.NearestFilter;
        texture.generateMipmaps = false;
        texture.needsUpdate = true;

        const [west, south, east, north] = imgData.bounds;
        const origin = newReplay.viewerContext.origin;
        const mat = new THREE.MeshBasicMaterial({
          map: texture, alphaTest: 0.4, polygonOffset: true, polygonOffsetFactor: -1, polygonOffsetUnits: -1,
        });
        terrainObject.traverse((o) => {
          if (!o.isMesh) return;
          const g = o.geometry.clone();
          const pos = g.getAttribute('position');
          const uv = new Float32Array(pos.count * 2);
          for (let i = 0; i < pos.count; i++) {
            const [lon, lat] = threeToLonLat(origin, pos.getX(i), pos.getY(i), pos.getZ(i));
            uv[2 * i] = (lon - west) / (east - west);
            uv[2 * i + 1] = 1 - (north - lat) / (north - south);
          }
          g.setAttribute('uv', new THREE.BufferAttribute(uv, 2));
          layerGroups.imagery.add(new THREE.Mesh(g, mat));
        });
        document.getElementById('tog-imagery').disabled = false;
      } catch (e) {
        console.warn('Optical drape skipped:', e);
      }
    } else {
      document.getElementById('tog-imagery').checked = false;
      document.getElementById('tog-imagery').disabled = true;
    }

    // Derived 3D Building volumes
    const bldgs = newReplay.viewerContext.buildings || [];
    const positions = [], normals = [];
    const addQuad = (a, b, c, d, nx, ny, nz) => {
      positions.push(...a, ...b, ...c, ...a, ...c, ...d);
      for (let k = 0; k < 6; k++) normals.push(nx, ny, nz);
    };
    for (const b of bldgs) {
      const ring = b.ring, y0 = b.baseUp, y1 = b.baseUp + b.heightM;
      for (let i = 0; i < ring.length - 1; i++) {
        const [e0, n0] = ring[i], [e1, n1] = ring[i + 1];
        const dx = e1 - e0, dz = -(n1 - n0), len = Math.hypot(dx, dz) || 1;
        addQuad([e0, y0, -n0], [e1, y0, -n1], [e1, y1, -n1], [e0, y1, -n0], -dz / len, 0, dx / len);
      }
      let ce = 0, cn = 0;
      const m = ring.length - 1;
      for (let i = 0; i < m; i++) { ce += ring[i][0]; cn += ring[i][1]; }
      ce /= m; cn /= m;
      for (let i = 0; i < m; i++) {
        const [e0, n0] = ring[i], [e1, n1] = ring[i + 1];
        positions.push(ce, y1, -cn, e0, y1, -n0, e1, y1, -n1);
        normals.push(0, 1, 0, 0, 1, 0, 0, 1, 0);
      }
    }
    if (positions.length > 0) {
      const geom = new THREE.BufferGeometry();
      geom.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
      geom.setAttribute('normal', new THREE.Float32BufferAttribute(normals, 3));
      layerGroups.buildings.add(
        new THREE.Mesh(
          geom,
          new THREE.MeshStandardMaterial({ color: 0x9aaec2, roughness: 0.68, metalness: 0.12, transparent: true, opacity: 0.74, side: THREE.DoubleSide })
        )
      );
    }

    // Draped OSM roads
    const segs = [];
    for (const road of newReplay.viewerContext.roads || []) {
      const pts = road.points;
      for (let i = 0; i < pts.length - 1; i++) {
        segs.push(pts[i][0], pts[i][2], -pts[i][1], pts[i + 1][0], pts[i + 1][2], -pts[i + 1][1]);
      }
    }
    if (segs.length > 0) {
      const geom = new THREE.BufferGeometry();
      geom.setAttribute('position', new THREE.Float32BufferAttribute(segs, 3));
      layerGroups.roads.add(
        new THREE.LineSegments(geom, new THREE.LineBasicMaterial({ color: 0xffd27d, transparent: true, opacity: 0.34 }))
      );
    }
  }

  // ── Load a Replay Bundle (supports initial load AND live What-If hot reloads!) ──
  async function loadRunBundle(ep, preserveCamera = false) {
    endpoints = ep;
    replay = await (await fetch(ep.replayUrl)).json();
    await loadEnvironmentGeometry(replay, ep);

    // Populate run switcher
    if (ep.serviceMode && ep.allRuns.length > 0) {
      const sel = document.getElementById('run-switcher');
      sel.style.display = 'block';
      sel.innerHTML = '';
      for (const r of ep.allRuns.filter((x) => x.status === 'completed')) {
        const opt = document.createElement('option');
        opt.value = r.jobId;
        const pct = r.comparison?.safeWindowVsEarliest?.urbanExposureReductionPct;
        const tag = pct != null ? ` (${pct >= 0 ? '-' : '+'}${Math.abs(pct).toFixed(0)}% exp)` : '';
        opt.textContent = `${r.jobId.slice(-6)} · ${r.mode} · seed ${r.seed}${tag}`;
        if (r.jobId === ep.jobId) opt.selected = true;
        sel.appendChild(opt);
      }
      sel.onchange = async () => {
        const nextUrl = new URL(window.location.href);
        nextUrl.searchParams.set('run', sel.value);
        window.history.pushState({}, '', nextUrl.toString());
        const nextEp = await resolveDataEndpoints(sel.value);
        if (nextEp) {
          await loadRunBundle(nextEp, true);
          hideLoading();
        }
      };
    }

    // Populate header & What-If inputs from replay.sourceDraft
    document.getElementById('scenario-title').textContent = replay.title;
    document.getElementById('scenario-meta').textContent =
      `Env ${replay.environment.profileId} · manifest ${replay.environment.manifestSha256.slice(0, 12)}… · ${ep.jobId || 'static'}`;

    const firstRunObj = replay.runs[Object.keys(replay.runs)[0]];
    const threatKind = firstRunObj?.threatKind || 'one_way_uav';
    document.getElementById('badges').innerHTML =
      `<span class="badge">${threatKind.replaceAll('_', ' ')}</span>` +
      `<span class="badge">${replay.environment.buildingCount} buildings</span>` +
      `<span class="badge warn">${replay.atmosphere.derivedOrigin}</span>`;

    let draftForWhatIf = replay.sourceDraft || null;
    const wiStatus = document.getElementById('whatif-status');
    if (!draftForWhatIf && ep.serviceMode) {
      try {
        const [envRes, healthRes] = await Promise.all([fetch('/api/environments'), fetch('/api/health')]);
        const envList = envRes.ok ? (await envRes.json()).environments || [] : [];
        const healthDoc = healthRes.ok ? await healthRes.json() : null;
        const matched = envList.find((e) => e.manifestSha256 === replay.environment?.manifestSha256) || envList[0];
        if (matched?.defaults) {
          draftForWhatIf = { ...matched.defaults, environmentId: matched.id };
          replay.sourceDraft = draftForWhatIf;
        }
        if (healthDoc?.engineVersion === 'emad-sim-py-0.1.0') {
          wiStatus.textContent = 'Running engine v0.1.0 — restart task dev for v0.2.0 ballistic, kinetic hazard & optimizer chart.';
        }
      } catch {
        // ignore fallback fetch error
      }
    }
    if (draftForWhatIf) {
      const sd = draftForWhatIf;
      document.getElementById('wi-kind').value = sd.threat?.kind || 'one_way_uav';
      document.getElementById('wi-speed').value = sd.threat?.speedMps ?? 68;
      document.getElementById('wi-apogee').value = sd.threat?.ballisticApogeeM ?? 1400;
      document.getElementById('wi-int-speed').value = sd.launcher?.speedMps ?? 295;
      document.getElementById('wi-frags').value = sd.shrapnel?.fragmentCount ?? 220;
      document.getElementById('wi-wind-scale').value = sd.atmosphere?.windScale ?? 1.0;
      document.getElementById('wi-wind-e').value = sd.atmosphere?.windAddEnuMps?.[0] ?? 0;
      document.getElementById('wi-wind-n').value = sd.atmosphere?.windAddEnuMps?.[1] ?? 0;
      document.getElementById('wi-seed').value = sd.seed ?? 20261125;
      document.getElementById('btn-whatif-run').disabled = !ep.serviceMode;
    } else {
      document.getElementById('btn-whatif-run').disabled = true;
    }

    // Wind table
    document.getElementById('wind-table').innerHTML = replay.windProfile
      .map(
        (w) =>
          `${String(w.altitudeAglM).padStart(4, ' ')}m AGL: ` +
          `E ${w.windEnuMps[0] >= 0 ? '+' : ''}${w.windEnuMps[0].toFixed(1)}, ` +
          `N ${w.windEnuMps[1] >= 0 ? '+' : ''}${w.windEnuMps[1].toFixed(1)} m/s (${w.speedMps.toFixed(1)} m/s)`
      )
      .join('<br/>');

    // Rebuild static scenario sites (radar, launcher, protected zone, coverage fan, wind arrows)
    disposeGroup(layerGroups.staticSites);
    disposeGroup(layerGroups.coverage);
    disposeGroup(layerGroups.wind);

    radarPos = enuToThree(replay.radarPositionEnu);
    const launcherPos = enuToThree(replay.launcherPositionEnu);
    const protCenter = enuToThree(replay.protectedZone.centerEnu);
    const protRadius = replay.protectedZone.radiusM;

    const firstThreatStart = firstRunObj?.truth?.threatTrajectory?.[0]
      ? enuToThree(firstRunObj.truth.threatTrajectory[0].enu)
      : sceneCenter;

    const radarObj = createRadarStation(radarPos);
    radarRotor = radarObj.rotor;
    layerGroups.staticSites.add(radarObj.root);
    layerGroups.staticSites.add(createLauncherStation(launcherPos, firstThreatStart));

    const ring = new THREE.Mesh(
      new THREE.RingGeometry(protRadius - 10, protRadius + 10, 64),
      new THREE.MeshBasicMaterial({ color: 0xf065d8, side: THREE.DoubleSide, transparent: true, opacity: 0.8 })
    );
    ring.rotation.x = -Math.PI / 2;
    ring.position.copy(protCenter).add(new THREE.Vector3(0, 12, 0));
    layerGroups.staticSites.add(ring);

    const cyl = new THREE.Mesh(
      new THREE.CylinderGeometry(protRadius, protRadius, 125, 48, 1, true),
      new THREE.MeshBasicMaterial({ color: 0xf065d8, side: THREE.DoubleSide, transparent: true, opacity: 0.11 })
    );
    cyl.position.copy(protCenter).add(new THREE.Vector3(0, 62, 0));
    layerGroups.staticSites.add(cyl);

    // Radar LOS fan
    const fanPts = [], ringPts = [];
    for (const ray of replay.radarCoverageRing || []) {
      const edge = enuToThree(ray.edgeEnu);
      ringPts.push(edge);
      fanPts.push(radarPos.x, radarPos.y, radarPos.z, edge.x, edge.y, edge.z);
    }
    if (ringPts.length > 2) {
      ringPts.push(ringPts[0]);
      layerGroups.coverage.add(
        new THREE.Line(
          new THREE.BufferGeometry().setFromPoints(ringPts),
          new THREE.LineBasicMaterial({ color: 0x58d5ff, transparent: true, opacity: 0.65 })
        )
      );
      const fanGeom = new THREE.BufferGeometry();
      fanGeom.setAttribute('position', new THREE.Float32BufferAttribute(fanPts, 3));
      layerGroups.coverage.add(
        new THREE.LineSegments(fanGeom, new THREE.LineBasicMaterial({ color: 0x58d5ff, transparent: true, opacity: 0.14 }))
      );
    }

    // 3D ERA5 Wind arrows
    const bounds = replay.environment.boundsEnu;
    const anchorE = (bounds.eastMin + bounds.eastMax) * 0.5;
    const anchorN = (bounds.northMin + bounds.northMax) * 0.5;
    for (const wp of replay.windProfile || []) {
      const originV = enuToThree([anchorE, anchorN, wp.upM]);
      const dir = enuToThree(wp.windEnuMps);
      const spd = dir.length();
      if (spd < 0.05) continue;
      dir.normalize();
      const arrowLen = Math.min(950, Math.max(180, spd * 80));
      layerGroups.wind.add(new THREE.ArrowHelper(dir, originV, arrowLen, 0x8be9ff, 75, 44));
    }

    policyNames = Object.keys(replay.runs);
    const nextPolicy = policyNames.includes(activePolicy)
      ? activePolicy
      : (policyNames.includes('safe_window') ? 'safe_window' : policyNames[0]);
    buildPolicyScene(nextPolicy);
    applyZScale(zScale);

    if (!preserveCamera) {
      setCamOverview(false);
    }
    statusEl.textContent = ep.hasGlb
      ? 'Verified terrain GLB loaded · click impacts or optimizer orbs to inspect'
      : 'Heightfield terrain loaded · click impacts to inspect';
  }

  function buildPolicyScene(policyKey) {
    activePolicy = policyKey;
    currentRun = replay.runs[policyKey];
    disposeGroup(layerGroups.truth);
    disposeGroup(layerGroups.obs);
    disposeGroup(layerGroups.shrapnel);
    disposeGroup(layerGroups.optimizer);
    disposeGroup(layerGroups.ghostCompare);
    disposeGroup(layerGroups.guides);
    inspCard.classList.remove('visible');

    fragMeshes = [];
    impactMeshes = [];
    obsDotObjects = [];
    pickableObjects = [];

    const tr = currentRun.truth;
    const obs = currentRun.observations;
    const ev = currentRun.evaluation;
    const rawEvents = currentRun.timelineEvents || {};
    const events = {
      firstDetectionS: rawEvents.firstDetectionS ?? (obs.detections?.length ? obs.detections[0].tS : null),
      launchS: rawEvents.launchS ?? (tr.intercept?.launchTimeS ?? null),
      interceptS: rawEvents.interceptS ?? (tr.intercept?.intercepted ? tr.intercept.interceptTimeS : null),
      lastImpactS:
        rawEvents.lastImpactS ??
        (tr.intercept?.intercepted && tr.shrapnel?.footprint?.maxFallTimeS != null
          ? Number((tr.intercept.interceptTimeS + tr.shrapnel.footprint.maxFallTimeS).toFixed(1))
          : null),
    };

    const lastThreatT = tr.threatTrajectory.length ? tr.threatTrajectory[tr.threatTrajectory.length - 1].tS : 30;
    const fallT = tr.shrapnel ? tr.shrapnel.footprint.maxFallTimeS : 0;
    const intT = tr.intercept.intercepted ? tr.intercept.interceptTimeS : lastThreatT;
    maxTime = Math.max(lastThreatT, intT + fallT) + 2.0;

    const slider = document.getElementById('time-slider');
    slider.max = maxTime.toFixed(1);
    simTime = 0;
    slider.value = '0';

    // 1. Faint full path guides + bright progressive animated trail lines
    threatLineCoords = tr.threatTrajectory.map((s) => enuToThree(s.enu));
    if (threatLineCoords.length > 1) {
      const guide = new THREE.Line(
        new THREE.BufferGeometry().setFromPoints(threatLineCoords),
        new THREE.LineDashedMaterial({ color: 0xff5c72, dashSize: 30, gapSize: 24, transparent: true, opacity: 0.3 })
      );
      guide.computeLineDistances();
      layerGroups.guides.add(guide);

      const progGeom = new THREE.BufferGeometry().setFromPoints(threatLineCoords);
      progGeom.setDrawRange(0, 0);
      activeThreatLine = new THREE.Line(progGeom, new THREE.LineBasicMaterial({ color: 0xff5c72 }));
      layerGroups.truth.add(activeThreatLine);
    } else {
      activeThreatLine = null;
    }

    threatModel = createThreatModel(currentRun.threatKind || 'one_way_uav');
    layerGroups.truth.add(threatModel);

    // Radar LOS beam
    losLine = new THREE.Line(
      new THREE.BufferGeometry().setFromPoints([radarPos.clone(), radarPos.clone()]),
      new THREE.LineBasicMaterial({ color: 0x58d5ff, transparent: true, opacity: 0.85 })
    );
    layerGroups.truth.add(losLine);

    // Interceptor trail & oriented missile model
    intLineCoords = tr.interceptorTrajectory.map((s) => enuToThree(s.enu));
    if (intLineCoords.length > 1) {
      const guide = new THREE.Line(
        new THREE.BufferGeometry().setFromPoints(intLineCoords),
        new THREE.LineDashedMaterial({ color: 0x69f0ae, dashSize: 26, gapSize: 20, transparent: true, opacity: 0.28 })
      );
      guide.computeLineDistances();
      layerGroups.guides.add(guide);

      const progGeom = new THREE.BufferGeometry().setFromPoints(intLineCoords);
      progGeom.setDrawRange(0, 0);
      activeIntLine = new THREE.Line(progGeom, new THREE.LineBasicMaterial({ color: 0x69f0ae }));
      layerGroups.truth.add(activeIntLine);
    } else {
      activeIntLine = null;
    }
    interceptorModel = createInterceptorModel();
    interceptorModel.visible = false;
    layerGroups.truth.add(interceptorModel);

    // Intercept burst sphere + expanding shockwave ring
    burstMesh = new THREE.Mesh(
      new THREE.SphereGeometry(60, 20, 16),
      new THREE.MeshBasicMaterial({ color: 0xffea94, transparent: true, opacity: 0.0 })
    );
    shockRingMesh = new THREE.Mesh(
      new THREE.RingGeometry(48, 62, 48),
      new THREE.MeshBasicMaterial({ color: 0xfff3c4, side: THREE.DoubleSide, transparent: true, opacity: 0.0 })
    );
    shockRingMesh.rotation.x = -Math.PI / 2;
    if (tr.intercept.intercepted) {
      const ip = enuToThree(tr.intercept.interceptEnu);
      burstMesh.position.copy(ip);
      shockRingMesh.position.copy(ip);
    }
    burstMesh.visible = false;
    shockRingMesh.visible = false;
    layerGroups.shrapnel.add(burstMesh, shockRingMesh);

    // 2. EXERCISE_OBSERVATION: noisy detections & estimated track
    for (const d of obs.detections) {
      const m = new THREE.Mesh(
        new THREE.SphereGeometry(13, 8, 8),
        new THREE.MeshBasicMaterial({ color: 0x58d5ff, transparent: true, opacity: 0.72 })
      );
      m.position.copy(enuToThree(d.measuredEnu));
      m.visible = false;
      layerGroups.obs.add(m);
      obsDotObjects.push({ mesh: m, deliverS: d.deliverS });
    }
    if (obs.tracks.length > 1) {
      layerGroups.obs.add(
        new THREE.Line(
          new THREE.BufferGeometry().setFromPoints(obs.tracks.map((tk) => enuToThree(tk.estimatedEnu))),
          new THREE.LineBasicMaterial({ color: 0x58d5ff, transparent: true, opacity: 0.55 })
        )
      );
    }
    trackHeadMesh = new THREE.Mesh(
      new THREE.OctahedronGeometry(24, 0),
      new THREE.MeshBasicMaterial({ color: 0x58d5ff, wireframe: true })
    );
    trackHeadMesh.visible = false;
    layerGroups.obs.add(trackHeadMesh);

    // 3. Safe-Window Optimizer Candidate Orbs in 3D + Cost Mini-Chart
    const feasibleCands = (currentRun.optimizerCandidates || []).filter((c) => c.feasible && c.predictedInterceptEnu);
    const optSection = document.getElementById('optimizer-section');
    const candChart = document.getElementById('cand-chart');
    if (feasibleCands.length > 0) {
      optSection.style.display = 'block';
      candChart.innerHTML = '';
      const maxCost = Math.max(...feasibleCands.map((c) => c.objectiveCost), 0.1);
      for (const c of feasibleCands) {
        const norm = c.objectiveCost / maxCost;
        const colHex = c.selected ? 0x69f0ae : (norm > 0.66 ? 0xff5c72 : (norm > 0.33 ? 0xffca63 : 0x58d5ff));
        const pos = enuToThree(c.predictedInterceptEnu);
        const orb = new THREE.Mesh(
          new THREE.SphereGeometry(c.selected ? 26 : 15, 12, 10),
          new THREE.MeshBasicMaterial({ color: colHex, transparent: true, opacity: c.selected ? 0.92 : 0.55, wireframe: !c.selected })
        );
        orb.position.copy(pos);
        orb.userData.inspect = {
          title: c.selected ? `★ Selected Safe Window (t=${c.predictedInterceptS}s)` : `Candidate Window (t=${c.predictedInterceptS}s)`,
          items: [
            ['Launch Time', `${c.candidateLaunchS}s`],
            ['Est. Intercept', `${c.predictedInterceptS}s`],
            ['Cost Score', `${c.objectiveCost.toFixed(3)}`],
            ['Preview Bldg Hits', `${c.previewBuildingHits}`],
            ['Preview Road Hits', `${c.previewRoadHits}`],
            ['Wind Drift', `${c.previewDriftM.toFixed(0)} m`],
          ],
        };
        layerGroups.optimizer.add(orb);
        pickableObjects.push(orb);

        // Bar in HUD mini-chart
        const bar = document.createElement('div');
        bar.className = `cand-bar ${c.selected ? 'selected' : ''}`;
        bar.style.height = `${Math.max(16, Math.round((c.objectiveCost / maxCost) * 100))}%`;
        bar.style.background = `#${colHex.toString(16).padStart(6, '0')}`;
        bar.title = `Launch t=${c.candidateLaunchS}s · Intercept t=${c.predictedInterceptS}s · Cost ${c.objectiveCost.toFixed(3)} · Preview bldg=${c.previewBuildingHits}`;
        bar.onclick = () => {
          animateCameraTo(pos, new THREE.Vector3(700, 620, 800));
          showInspection(orb.userData.inspect);
        };
        candChart.appendChild(bar);
      }
    } else {
      optSection.style.display = 'none';
    }

    // 4. Dual-Policy Ghost Comparison Overlay (shows the OTHER policy's 90% ellipse & intercept!)
    for (const otherName of policyNames) {
      if (otherName === activePolicy) continue;
      const otherRun = replay.runs[otherName];
      const oIntr = otherRun?.truth?.intercept;
      const oFp = otherRun?.truth?.shrapnel?.footprint;
      const ghostCol = otherName === 'earliest' ? 0xff425c : 0x45f09e;
      if (oFp) {
        layerGroups.ghostCompare.add(buildEllipseLine(oFp, ghostCol, true));
      }
      if (oIntr?.intercepted && oIntr.interceptEnu) {
        const marker = new THREE.Mesh(
          new THREE.OctahedronGeometry(30, 0),
          new THREE.MeshBasicMaterial({ color: ghostCol, wireframe: true })
        );
        marker.position.copy(enuToThree(oIntr.interceptEnu));
        marker.userData.inspect = {
          title: `Comparison Ghost: ${otherName}`,
          items: [
            ['Intercept', `t=${oIntr.interceptTimeS}s`],
            ['Alt AGL', `${oIntr.interceptAglM?.toFixed(0) ?? '—'} m`],
            ['Bldg Hits', `${otherRun.evaluation.shrapnelExposure?.buildingHits ?? '—'}`],
          ],
        };
        layerGroups.ghostCompare.add(marker);
        pickableObjects.push(marker);
      }
    }

    // 5. Shrapnel trails, animated fragments, clickable impacts, and active 90% ellipse
    if (tr.intercept.intercepted && tr.shrapnel) {
      const classColor = { heavy: 0xff5c72, medium: 0xffca63, light: 0x8be9ff };
      for (const trail of tr.shrapnel.trails) {
        const pts = trail.points.map((p) => enuToThree([p[1], p[2], p[3]]));
        const col = classColor[trail.class] || 0xffca63;
        const lineGeom = new THREE.BufferGeometry().setFromPoints(pts);
        lineGeom.setDrawRange(0, 0);
        const trailLine = new THREE.Line(
          lineGeom,
          new THREE.LineBasicMaterial({ color: col, transparent: true, opacity: trail.hazardous ? 0.58 : 0.28 })
        );
        layerGroups.shrapnel.add(trailLine);

        const fMesh = new THREE.Mesh(
          new THREE.SphereGeometry(trail.class === 'heavy' ? 12 : 8.5, 8, 6),
          new THREE.MeshBasicMaterial({ color: col })
        );
        fMesh.visible = false;
        layerGroups.shrapnel.add(fMesh);
        fragMeshes.push({ mesh: fMesh, line: trailLine, points: trail.points });
      }

      const catColor = {
        building: 0xff2e4c,
        road: 0xff9e3d,
        protected_zone: 0xf065d8,
        open: 0x58e08e,
      };
      for (const imp of tr.shrapnel.impacts) {
        const radius = imp.hazardous ? (imp.category === 'open' ? 13 : 19) : (imp.category === 'open' ? 9 : 13);
        const m = new THREE.Mesh(
          new THREE.SphereGeometry(radius, 10, 8),
          new THREE.MeshBasicMaterial({ color: catColor[imp.category] || 0xffffff })
        );
        const p = enuToThree(imp.enu);
        p.y += 5;
        m.position.copy(p);
        m.visible = false;
        m.userData.inspect = {
          title: `${imp.id.toUpperCase()} · ${imp.hazardous ? 'HAZARDOUS IMPACT (≥80 J)' : 'Light Debris'}`,
          items: [
            ['Origin', imp.origin === 'interceptor' ? 'Interceptor' : 'Threat'],
            ['Class / Mass', `${imp.class} (${imp.massKg ?? '?'} kg)`],
            ['Impact Speed', `${imp.impactSpeedMps ?? '?'} m/s`],
            ['Kinetic Energy', `${imp.impactEnergyJ != null ? imp.impactEnergyJ.toLocaleString() : '?'} J`],
            ['Surface Hit', imp.category.replace('_', ' ')],
            ['OSM Feature', imp.buildingSourceId || '—'],
          ],
        };
        layerGroups.shrapnel.add(m);
        pickableObjects.push(m);
        impactMeshes.push({ mesh: m, impactAtS: tr.intercept.interceptTimeS + imp.tAfterInterceptS });
      }

      layerGroups.shrapnel.add(buildEllipseLine(tr.shrapnel.footprint, 0xffca63, false));
    }

    // 6. Populate Policy Tabs, Metrics HUD, and Timeline Event Bookmarks
    const tabs = document.getElementById('policy-tabs');
    tabs.innerHTML = '';
    for (const pName of policyNames) {
      const b = document.createElement('button');
      b.className = `btn ${pName === activePolicy ? 'active' : ''}`;
      b.style.flex = '1';
      b.textContent = pName === 'earliest' ? 'Earliest (Baseline)' : 'Safe-Window (Wind-Aware)';
      b.onclick = () => buildPolicyScene(pName);
      tabs.appendChild(b);
    }

    const exp = ev.shrapnelExposure || {};
    const fp = ev.shrapnelFootprint || {};
    const intr = ev.interception;
    const sens = ev.sensing;
    const expPct = exp.urbanExposureFraction != null ? (exp.urbanExposureFraction * 100).toFixed(1) + '%' : 'N/A';
    const expCls = exp.urbanExposureFraction != null ? (exp.urbanExposureFraction < 0.15 ? 'good' : exp.urbanExposureFraction > 0.35 ? 'bad' : 'warn') : '';

    document.getElementById('metrics-grid').innerHTML = `
      <div class="metric"><div class="label">Intercept</div><div class="val ${intr.intercepted ? 'good' : 'bad'}">${intr.intercepted ? `t=${intr.interceptTimeS}s` : 'MISS'}</div></div>
      <div class="metric"><div class="label">Intercept Alt</div><div class="val">${intr.interceptAglM != null ? intr.interceptAglM.toFixed(0) + 'm AGL' : '—'}</div></div>
      <div class="metric"><div class="label">Building Hits (≥80J)</div><div class="val ${exp.buildingHits > 10 ? 'bad' : exp.buildingHits > 0 ? 'warn' : 'good'}">${exp.buildingHits ?? '—'}${exp.hazardousBuildingHits != null ? ` (${exp.hazardousBuildingHits})` : ''}</div></div>
      <div class="metric"><div class="label">Road Hits</div><div class="val ${exp.roadHits > 20 ? 'warn' : ''}">${exp.roadHits ?? '—'}</div></div>
      <div class="metric"><div class="label">Urban Exposure</div><div class="val ${expCls}">${expPct}</div></div>
      <div class="metric"><div class="label">Wind Drift</div><div class="val">${fp.driftDistanceM != null ? fp.driftDistanceM.toFixed(0) + 'm' : '—'}</div></div>
      <div class="metric"><div class="label">Hazardous Debris</div><div class="val warn">${exp.hazardousHits != null ? `${exp.hazardousHits}/${exp.total}` : '—'}</div></div>
      <div class="metric"><div class="label">Max Impact Energy</div><div class="val">${exp.maxImpactEnergyJ != null ? (exp.maxImpactEnergyJ >= 1000 ? (exp.maxImpactEnergyJ / 1000).toFixed(1) + ' kJ' : exp.maxImpactEnergyJ.toFixed(0) + ' J') : '—'}</div></div>
      <div class="metric"><div class="label">Radar Occlusion</div><div class="val">${(sens.occlusionFraction * 100).toFixed(0)}%</div></div>
      <div class="metric"><div class="label">Mean Track Err</div><div class="val">${sens.meanTrackErrorM != null ? sens.meanTrackErrorM.toFixed(1) + 'm' : '—'}</div></div>
    `;

    const deltaBox = document.getElementById('delta-box');
    const d = replay.comparison?.safeWindowVsEarliest;
    if (d && d.urbanExposureReductionPct != null) {
      deltaBox.style.display = 'block';
      deltaBox.innerHTML =
        `<strong>Safe-Window vs Earliest:</strong> ` +
        `Buildings <b>${d.buildingHitsDelta >= 0 ? '+' : ''}${d.buildingHitsDelta}</b> · ` +
        `Roads <b>${d.roadHitsDelta >= 0 ? '+' : ''}${d.roadHitsDelta}</b> · ` +
        `Exposure <b>${d.urbanExposureReductionPct >= 0 ? '-' : '+'}${Math.abs(d.urbanExposureReductionPct).toFixed(1)}%</b>`;
    } else {
      deltaBox.style.display = 'none';
    }

    // Event bookmark pills + colored ticks on the timeline track
    const pillsDiv = document.getElementById('bookmark-pills');
    const ticksDiv = document.getElementById('timeline-ticks');
    pillsDiv.innerHTML = '';
    ticksDiv.innerHTML = '';
    const marks = [
      { label: 'T0 Ingress', t: 0, color: '#8ea6b8', cls: '' },
      events.firstDetectionS != null ? { label: `Detect ${events.firstDetectionS.toFixed(1)}s`, t: events.firstDetectionS, color: '#58d5ff', cls: '' } : null,
      events.launchS != null ? { label: `Launch ${events.launchS.toFixed(1)}s`, t: events.launchS, color: '#69f0ae', cls: '' } : null,
      events.interceptS != null ? { label: `💥 Intercept ${events.interceptS.toFixed(1)}s`, t: Math.max(0, events.interceptS - 0.8), tickT: events.interceptS, color: '#ffca63', cls: 'intercept' } : null,
      events.lastImpactS != null ? { label: `All Impact ${events.lastImpactS.toFixed(1)}s`, t: events.lastImpactS, color: '#ff5c72', cls: '' } : null,
    ].filter(Boolean);

    for (const mk of marks) {
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = `ev-pill ${mk.cls}`;
      btn.textContent = mk.label;
      btn.onclick = () => {
        simTime = mk.t;
        updateAtTime(simTime);
      };
      pillsDiv.appendChild(btn);

      const tick = document.createElement('div');
      tick.className = 'tl-tick';
      tick.style.left = `${Math.min(100, Math.max(0, ((mk.tickT ?? mk.t) / maxTime) * 100))}%`;
      tick.style.background = mk.color;
      ticksDiv.appendChild(tick);
    }

    updateAtTime(0);
  }

  function updateAtTime(t) {
    if (!currentRun) return;
    const tr = currentRun.truth;
    const obs = currentRun.observations;
    const intT = tr.intercept.intercepted ? tr.intercept.interceptTimeS : Infinity;

    // Threat position, orientation & progressive trail
    const th = interpTrajectory(tr.threatTrajectory, Math.min(t, intT));
    if (th && t <= intT) {
      threatModel.visible = true;
      threatModel.position.copy(th.pos);
      if (th.vel.lengthSq() > 1e-4) {
        threatModel.lookAt(th.pos.clone().add(th.vel));
      }
      if (activeThreatLine) {
        let count = 0;
        for (let i = 0; i < tr.threatTrajectory.length; i++) {
          if (tr.threatTrajectory[i].tS <= t) count = i + 1;
          else break;
        }
        activeThreatLine.geometry.setDrawRange(0, count);
      }

      let clear = true;
      for (const scan of tr.radarLosTruth) {
        if (scan.sampleS <= t) clear = scan.losClear;
        else break;
      }
      losLine.visible = true;
      losLine.geometry.setFromPoints([radarPos, th.pos]);
      losLine.material.color.setHex(clear ? 0x58d5ff : 0xff3b5c);

      if (followMode === 'threat') {
        const wt = toWorldPos(th.pos);
        controls.target.lerp(wt, 0.14);
      }
    } else {
      threatModel.visible = false;
      losLine.visible = false;
      if (activeThreatLine) activeThreatLine.geometry.setDrawRange(0, threatLineCoords.length);
    }

    // Interceptor position, orientation & progressive trail
    const ic = interpTrajectory(tr.interceptorTrajectory, t);
    if (ic && t <= intT) {
      interceptorModel.visible = true;
      interceptorModel.position.copy(ic.pos);
      if (ic.vel.lengthSq() > 1e-4) {
        interceptorModel.lookAt(ic.pos.clone().add(ic.vel));
      }
      if (activeIntLine) {
        let count = 0;
        for (let i = 0; i < tr.interceptorTrajectory.length; i++) {
          if (tr.interceptorTrajectory[i].tS <= t) count = i + 1;
          else break;
        }
        activeIntLine.geometry.setDrawRange(0, count);
      }
    } else {
      interceptorModel.visible = false;
      if (activeIntLine) {
        activeIntLine.geometry.setDrawRange(0, t > intT ? intLineCoords.length : 0);
      }
    }

    // Burst flash + shockwave ring
    if (tr.intercept.intercepted && t >= intT && t <= intT + 2.8) {
      const age = (t - intT) / 2.8;
      burstMesh.visible = true;
      burstMesh.scale.setScalar(0.35 + age * 2.5);
      burstMesh.material.opacity = (1 - age) * 0.78;
      shockRingMesh.visible = true;
      shockRingMesh.scale.setScalar(0.5 + age * 6.5);
      shockRingMesh.material.opacity = (1 - age) * 0.65;
    } else {
      burstMesh.visible = false;
      shockRingMesh.visible = false;
    }

    // Delivered observations & observation-only track head
    for (const item of obsDotObjects) item.mesh.visible = item.deliverS <= t;
    let latestTrack = null;
    for (const tk of obs.tracks) {
      if (tk.tS <= t) latestTrack = tk;
      else break;
    }
    if (latestTrack && t <= intT + 1.0) {
      trackHeadMesh.visible = true;
      trackHeadMesh.position.copy(enuToThree(latestTrack.estimatedEnu));
    } else {
      trackHeadMesh.visible = false;
    }

    // Animated shrapnel fragments, progressive trails & ground impacts
    if (tr.intercept.intercepted && t >= intT) {
      const tRel = t - intT;
      for (const fm of fragMeshes) {
        const res = interpTrailPoint(fm.points, tRel);
        if (res) {
          fm.line.geometry.setDrawRange(0, res.reachedIndex + 2);
          if (!res.impacted) {
            fm.mesh.visible = true;
            fm.mesh.position.copy(res.pos);
          } else {
            fm.mesh.visible = false;
          }
        } else {
          fm.line.geometry.setDrawRange(0, 0);
          fm.mesh.visible = false;
        }
      }
      for (const im of impactMeshes) im.mesh.visible = t >= im.impactAtS;
    } else {
      for (const fm of fragMeshes) {
        fm.mesh.visible = false;
        fm.line.geometry.setDrawRange(0, 0);
      }
      for (const im of impactMeshes) im.mesh.visible = false;
    }

    let phase = 'Threat Approaching';
    if (tr.intercept.intercepted && t >= intT) phase = 'Post-Intercept Shrapnel Drift & Impact';
    else if (ic && t <= intT) phase = 'Interceptor Pursuing Threat';
    else if (latestTrack) phase = `Radar Tracking (${latestTrack.updateCount} obs)`;

    document.getElementById('live-status').textContent = `t = ${t.toFixed(1)}s · ${phase}`;
    document.getElementById('time-readout').textContent = `${t.toFixed(1)}s / ${maxTime.toFixed(1)}s`;
    document.getElementById('time-slider').value = t.toFixed(1);
  }

  // ── Camera Presets (smoothly eased!) ──
  function setCamOverview(smooth = true) {
    const off = new THREE.Vector3(sceneSize * 0.44, sceneSize * 0.46, sceneSize * 0.54);
    if (smooth) animateCameraTo(sceneCenter, off);
    else {
      followMode = null;
      const wt = toWorldPos(sceneCenter);
      camera.position.copy(wt).add(off);
      controls.target.copy(wt);
      controls.update();
    }
  }
  function setCamIntercept() {
    const tr = currentRun?.truth;
    const focus = tr?.intercept?.intercepted ? enuToThree(tr.intercept.interceptEnu) : sceneCenter;
    animateCameraTo(focus, new THREE.Vector3(950, 820, 1050));
  }
  function setCamRadar() {
    animateCameraTo(radarPos, new THREE.Vector3(650, 480, 720));
  }
  function setCamImpact() {
    const fp = currentRun?.truth?.shrapnel?.footprint;
    const focus = fp ? enuToThree(fp.centroidEnu) : sceneCenter;
    animateCameraTo(focus, new THREE.Vector3(550, 780, 620));
  }
  function setCamTopDown() {
    animateCameraTo(sceneCenter, new THREE.Vector3(0, sceneSize * 0.92, 60));
  }
  function toggleFollowThreat() {
    followMode = followMode === 'threat' ? null : 'threat';
    document.getElementById('cam-follow').classList.toggle('active', followMode === 'threat');
  }

  // ── Bind UI Controls ──
  document.getElementById('cam-overview').onclick = () => setCamOverview(true);
  document.getElementById('cam-intercept').onclick = setCamIntercept;
  document.getElementById('cam-radar').onclick = setCamRadar;
  document.getElementById('cam-follow').onclick = toggleFollowThreat;
  document.getElementById('cam-impact').onclick = setCamImpact;
  document.getElementById('cam-topdown').onclick = setCamTopDown;

  const playBtn = document.getElementById('btn-play');
  function setPlaying(next) {
    playing = next;
    playBtn.textContent = playing ? 'Pause' : 'Play';
    playBtn.classList.toggle('active', playing);
  }
  playBtn.onclick = () => setPlaying(!playing);
  document.getElementById('btn-reset').onclick = () => { simTime = 0; updateAtTime(0); };
  document.getElementById('btn-step-back').onclick = () => {
    setPlaying(false);
    simTime = Math.max(0, simTime - 0.2);
    updateAtTime(simTime);
  };
  document.getElementById('btn-step-fwd').onclick = () => {
    setPlaying(false);
    simTime = Math.min(maxTime, simTime + 0.2);
    updateAtTime(simTime);
  };
  document.getElementById('speed-select').onchange = (e) => { speed = parseFloat(e.target.value); };
  document.getElementById('time-slider').oninput = (e) => {
    setPlaying(false);
    simTime = parseFloat(e.target.value);
    updateAtTime(simTime);
  };
  document.getElementById('zscale-slider').oninput = (e) => applyZScale(parseFloat(e.target.value));

  // Panel tabs & collapse toggles
  const tabTel = document.getElementById('tab-telemetry');
  const tabWi = document.getElementById('tab-whatif');
  const paneTel = document.getElementById('pane-telemetry');
  const paneWi = document.getElementById('pane-whatif');
  tabTel.onclick = () => { tabTel.classList.add('active'); tabWi.classList.remove('active'); paneTel.style.display = 'block'; paneWi.style.display = 'none'; };
  tabWi.onclick = () => { tabWi.classList.add('active'); tabTel.classList.remove('active'); paneWi.style.display = 'block'; paneTel.style.display = 'none'; };

  document.getElementById('collapse-left').onclick = (e) => {
    const p = document.getElementById('top-left');
    p.classList.toggle('collapsed');
    e.currentTarget.textContent = p.classList.contains('collapsed') ? '+' : '−';
  };
  document.getElementById('collapse-right').onclick = (e) => {
    const p = document.getElementById('top-right');
    p.classList.toggle('collapsed');
    e.currentTarget.textContent = p.classList.contains('collapsed') ? '+' : '−';
  };
  const toggleHud = () => document.body.classList.toggle('hud-hidden');
  document.getElementById('btn-hide-hud').onclick = toggleHud;
  document.getElementById('hud-restore-pill').onclick = toggleHud;

  // Layer toggles
  const toggleMap = {
    'tog-imagery': layerGroups.imagery,
    'tog-buildings': layerGroups.buildings,
    'tog-roads': layerGroups.roads,
    'tog-coverage': layerGroups.coverage,
    'tog-wind': layerGroups.wind,
    'tog-optimizer': layerGroups.optimizer,
    'tog-ghost-compare': layerGroups.ghostCompare,
    'tog-guides': layerGroups.guides,
    'tog-truth': layerGroups.truth,
    'tog-obs': layerGroups.obs,
    'tog-shrapnel': layerGroups.shrapnel,
  };
  for (const [id, grp] of Object.entries(toggleMap)) {
    const el = document.getElementById(id);
    el.onchange = () => { grp.visible = el.checked; };
  }

  // What-If Threat Kind auto-defaults & wind presets
  document.getElementById('wi-kind').onchange = (e) => {
    const k = e.target.value;
    if (k === 'one_way_uav') {
      document.getElementById('wi-speed').value = 68;
      document.getElementById('wi-int-speed').value = 295;
    } else if (k === 'cruise_missile') {
      document.getElementById('wi-speed').value = 230;
      document.getElementById('wi-int-speed').value = 380;
    } else if (k === 'ballistic_missile') {
      document.getElementById('wi-speed').value = 340;
      document.getElementById('wi-apogee').value = 1500;
      document.getElementById('wi-int-speed').value = 500;
    }
  };
  document.querySelectorAll('[data-wind-preset]').forEach((btn) => {
    btn.onclick = () => {
      const p = btn.dataset.windPreset;
      if (p === 'era5') { document.getElementById('wi-wind-scale').value = 1; document.getElementById('wi-wind-e').value = 0; document.getElementById('wi-wind-n').value = 0; }
      else if (p === 'shamal') { document.getElementById('wi-wind-scale').value = 1; document.getElementById('wi-wind-e').value = 14; document.getElementById('wi-wind-n').value = -5; }
      else if (p === 'gale') { document.getElementById('wi-wind-scale').value = 1; document.getElementById('wi-wind-e').value = 22; document.getElementById('wi-wind-n').value = 0; }
      else if (p === 'calm') { document.getElementById('wi-wind-scale').value = 0; document.getElementById('wi-wind-e').value = 0; document.getElementById('wi-wind-n').value = 0; }
    };
  });
  document.getElementById('wi-rand-seed').onclick = () => {
    document.getElementById('wi-seed').value = Math.floor(Math.random() * 90000000) + 10000000;
  };

  // Live What-If Re-Run In Place!
  document.getElementById('btn-whatif-run').onclick = async () => {
    if (!replay?.sourceDraft || !endpoints.serviceMode) return;
    const wiStatus = document.getElementById('whatif-status');
    const runBtn = document.getElementById('btn-whatif-run');
    runBtn.disabled = true;
    wiStatus.textContent = 'Submitting What-If scenario…';
    try {
      const base = structuredClone(replay.sourceDraft);
      base.seed = Number(document.getElementById('wi-seed').value) || 20261125;
      base.mode = 'compare';
      base.threat = {
        ...(base.threat || {}),
        kind: document.getElementById('wi-kind').value,
        speedMps: Number(document.getElementById('wi-speed').value) || 68,
        ballisticApogeeM: Number(document.getElementById('wi-apogee').value) || 1400,
      };
      base.launcher = {
        ...(base.launcher || {}),
        speedMps: Number(document.getElementById('wi-int-speed').value) || 295,
      };
      base.shrapnel = {
        ...(base.shrapnel || {}),
        fragmentCount: Number(document.getElementById('wi-frags').value) || 220,
      };
      base.atmosphere = {
        windScale: Number(document.getElementById('wi-wind-scale').value) || 0,
        windAddEnuMps: [
          Number(document.getElementById('wi-wind-e').value) || 0,
          Number(document.getElementById('wi-wind-n').value) || 0,
          0,
        ],
      };

      const res = await fetch('/api/runs', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(base),
      });
      const created = await res.json();
      if (!res.ok) throw new Error(created.error || `HTTP ${res.status}`);

      wiStatus.textContent = `Simulating ${created.jobId.slice(-6)}…`;
      const nextUrl = new URL(window.location.href);
      nextUrl.searchParams.set('run', created.jobId);
      window.history.pushState({}, '', nextUrl.toString());

      const nextEp = await resolveDataEndpoints(created.jobId);
      if (nextEp) {
        await loadRunBundle(nextEp, true);
        hideLoading();
        wiStatus.textContent = `Loaded ${created.jobId.slice(-6)}!`;
        tabTel.click();
      }
    } catch (err) {
      wiStatus.textContent = `Error: ${err.message}`;
    } finally {
      runBtn.disabled = false;
    }
  };

  // Keyboard shortcuts & smooth WASD/QE free-fly
  window.addEventListener('keydown', (e) => {
    const tag = e.target?.tagName?.toLowerCase();
    if (tag === 'input' || tag === 'select' || tag === 'textarea') return;
    const k = e.key.toLowerCase();
    if (['w', 'a', 's', 'd', 'q', 'e'].includes(k)) {
      keysHeld.add(k);
      followMode = null;
      camTransition = null;
      return;
    }
    if (e.code === 'Space') { e.preventDefault(); setPlaying(!playing); }
    else if (e.key === 'ArrowLeft') { e.preventDefault(); setPlaying(false); simTime = Math.max(0, simTime - (e.shiftKey ? 2.0 : 0.5)); updateAtTime(simTime); }
    else if (e.key === 'ArrowRight') { e.preventDefault(); setPlaying(false); simTime = Math.min(maxTime, simTime + (e.shiftKey ? 2.0 : 0.5)); updateAtTime(simTime); }
    else if (k === 'r') { simTime = 0; updateAtTime(0); }
    else if (k === 'h') { toggleHud(); }
    else if (k === 'c') { const el = document.getElementById('tog-ghost-compare'); el.checked = !el.checked; layerGroups.ghostCompare.visible = el.checked; }
    else if (k === '1') setCamOverview(true);
    else if (k === '2') setCamIntercept();
    else if (k === '3') setCamRadar();
    else if (k === '4') toggleFollowThreat();
    else if (k === '5') setCamImpact();
    else if (k === '6') setCamTopDown();
  });
  window.addEventListener('keyup', (e) => keysHeld.delete(e.key.toLowerCase()));

  window.addEventListener('resize', () => {
    camera.aspect = window.innerWidth / window.innerHeight;
    camera.updateProjectionMatrix();
    renderer.setSize(window.innerWidth, window.innerHeight);
  });

  // Initial bundle load
  await loadRunBundle(endpoints, false);
  hideLoading();

  // Main render loop
  let lastFrame = performance.now();
  const fwdVec = new THREE.Vector3();
  const rightVec = new THREE.Vector3();
  function animate(now) {
    requestAnimationFrame(animate);
    const dtWall = Math.min(0.1, (now - lastFrame) / 1000);
    lastFrame = now;

    if (radarRotor) radarRotor.rotation.y += dtWall * 1.9;

    // Smooth eased camera preset transitions
    if (camTransition) {
      camTransition.t += dtWall;
      const u = Math.min(1, camTransition.t / camTransition.dur);
      const ease = 1 - Math.pow(1 - u, 3);
      camera.position.lerpVectors(camTransition.startPos, camTransition.endPos, ease);
      controls.target.lerpVectors(camTransition.startTarget, camTransition.endTarget, ease);
      if (u >= 1) camTransition = null;
    }

    // WASD / QE free-fly pan
    if (keysHeld.size > 0) {
      const flySpeed = Math.max(450, camera.position.distanceTo(controls.target) * 0.65) * dtWall;
      camera.getWorldDirection(fwdVec);
      fwdVec.y = 0;
      if (fwdVec.lengthSq() > 1e-6) fwdVec.normalize();
      rightVec.crossVectors(fwdVec, new THREE.Vector3(0, 1, 0)).normalize();
      const move = new THREE.Vector3();
      if (keysHeld.has('w')) move.addScaledVector(fwdVec, flySpeed);
      if (keysHeld.has('s')) move.addScaledVector(fwdVec, -flySpeed);
      if (keysHeld.has('d')) move.addScaledVector(rightVec, flySpeed);
      if (keysHeld.has('a')) move.addScaledVector(rightVec, -flySpeed);
      if (keysHeld.has('e')) move.y += flySpeed * 0.7;
      if (keysHeld.has('q')) move.y -= flySpeed * 0.7;
      camera.position.add(move);
      controls.target.add(move);
    }

    if (playing && currentRun) {
      simTime += dtWall * speed;
      if (simTime > maxTime) simTime = 0;
      updateAtTime(simTime);
    }
    controls.update();
    renderer.render(scene, camera);
  }
  requestAnimationFrame(animate);
}

init().catch((err) => {
  setLoading('Simulation Viewer Unavailable', err.message, 'Ensure the local EMAD simulation service is running (task dev or task dev:sim).');
  document.getElementById('status-line').textContent = `Viewer error: ${err.message}`;
  console.error(err);
});
