import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';

const API_BASE = (() => {
  const p = window.location.pathname;
  if (p.startsWith('/sim-viewers/coverage')) return '/api/v1/sim-platform/coverage';
  if (p.startsWith('/viewer/coverage')) return '/api/coverage';
  return '/api';
})();
const USES_GO_GATEWAY = API_BASE.startsWith('/api/v1/');
function apiHeaders(extra = {}) {
  const h = { ...extra };
  const tok = window.__C4ISR_CONFIG__?.apiToken;
  if (USES_GO_GATEWAY && tok) h.Authorization = `Bearer ${tok}`;
  return h;
}
function apiFetch(relPath, opts = {}) {
  return fetch(`${API_BASE}${relPath}`, { ...opts, headers: apiHeaders(opts.headers || {}) });
}
async function loadGlbAuth(url) {
  const loader = new GLTFLoader();
  loader.setRequestHeader(apiHeaders());
  return loader.loadAsync(url);
}

// Coordinate contract matching EMAD prepare_display_terrain.py:
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

function decodeBase64Bytes(b64) {
  const bin = atob(b64);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

function decodeFloat32(b64) {
  const bytes = decodeBase64Bytes(b64);
  return new Float32Array(bytes.buffer, bytes.byteOffset, bytes.byteLength / 4);
}

function decodeUint16(b64) {
  const bytes = decodeBase64Bytes(b64);
  return new Uint16Array(bytes.buffer, bytes.byteOffset, bytes.byteLength / 2);
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

const SENSOR_COLORS = {
  radar: { hex: 0x38bdf8, css: '#38bdf8', name: 'Radar' },
  eo_ir: { hex: 0xfbbf24, css: '#fbbf24', name: 'EO/IR' },
  acoustic: { hex: 0xc084fc, css: '#c084fc', name: 'Acoustic' },
};

// ── Crisp Canvas-Rendered 3D Floating Label Sprite ──
function createLabelSprite(text, accentCss = '#38bdf8', scaleM = 180) {
  const canvas = document.createElement('canvas');
  canvas.width = 512;
  canvas.height = 112;
  const ctx = canvas.getContext('2d');
  ctx.font = '600 38px "Segoe UI", system-ui, sans-serif';
  const metrics = ctx.measureText(text);
  const w = Math.min(490, Math.max(150, Math.ceil(metrics.width) + 52));
  const x = (512 - w) / 2;
  const y = 18;
  const h = 76;
  const r = 18;

  ctx.fillStyle = 'rgba(8, 17, 26, 0.88)';
  ctx.strokeStyle = accentCss;
  ctx.lineWidth = 4;
  ctx.beginPath();
  ctx.roundRect(x, y, w, h, r);
  ctx.fill();
  ctx.stroke();

  ctx.fillStyle = accentCss;
  ctx.beginPath();
  ctx.arc(x + 24, y + h / 2, 8, 0, Math.PI * 2);
  ctx.fill();

  ctx.fillStyle = '#eef6fc';
  ctx.textBaseline = 'middle';
  ctx.fillText(text, x + 42, y + h / 2 + 1, w - 54);

  const tex = new THREE.CanvasTexture(canvas);
  tex.colorSpace = THREE.SRGBColorSpace;
  tex.minFilter = THREE.LinearFilter;
  const mat = new THREE.SpriteMaterial({ map: tex, transparent: true, depthTest: false });
  const sprite = new THREE.Sprite(mat);
  sprite.scale.set(scaleM, scaleM * (112 / 512), 1);
  sprite.renderOrder = 25;
  return sprite;
}

// ── Procedural 3D Models for Multi-Modal Sensors ──
function createSensorModel(sensor, isGhost = false, isPreview = false) {
  const palette = SENSOR_COLORS[sensor.type] || SENSOR_COLORS.radar;
  const g = new THREE.Group();
  const baseMat = new THREE.MeshStandardMaterial({
    color: isGhost ? 0x64748b : isPreview ? 0x065f46 : 0x334155,
    roughness: 0.65,
    metalness: 0.45,
    transparent: isGhost,
    opacity: isGhost ? 0.42 : 1.0,
  });
  const accentHex = isGhost ? 0x94a3b8 : isPreview ? 0x34d399 : palette.hex;
  const accentMat = new THREE.MeshStandardMaterial({
    color: accentHex,
    emissive: isGhost ? 0x1e293b : accentHex,
    emissiveIntensity: isGhost ? 0.15 : isPreview ? 0.55 : 0.35,
    roughness: 0.35,
    metalness: 0.3,
    transparent: isGhost,
    opacity: isGhost ? 0.42 : 1.0,
  });

  const mastH = Math.max(18, sensor.mastAglM);
  const pedestal = new THREE.Mesh(new THREE.CylinderGeometry(7, 13, 10, 8), baseMat);
  pedestal.position.y = 5;
  g.add(pedestal);

  const mast = new THREE.Mesh(new THREE.CylinderGeometry(3.2, 5.2, mastH, 8), baseMat);
  mast.position.y = 10 + mastH * 0.5;
  g.add(mast);

  let rotor = null;
  if (sensor.type === 'radar') {
    rotor = new THREE.Group();
    rotor.position.y = 10 + mastH + 6;
    const dish = new THREE.Mesh(new THREE.BoxGeometry(28, 13, 4.5), accentMat);
    rotor.add(dish);
    const bar = new THREE.Mesh(new THREE.BoxGeometry(34, 2.5, 2.5), baseMat);
    bar.position.y = 8;
    rotor.add(bar);
    g.add(rotor);
  } else if (sensor.type === 'eo_ir') {
    rotor = new THREE.Group();
    rotor.position.y = 10 + mastH + 6;
    const ball = new THREE.Mesh(new THREE.SphereGeometry(10, 14, 10), accentMat);
    rotor.add(ball);
    const lens = new THREE.Mesh(
      new THREE.CylinderGeometry(4.5, 5.5, 9, 10),
      new THREE.MeshBasicMaterial({ color: 0xfff3c4 })
    );
    lens.rotation.x = Math.PI / 2;
    lens.position.z = 8;
    rotor.add(lens);
    g.add(rotor);
  } else {
    const hub = new THREE.Mesh(new THREE.OctahedronGeometry(10, 0), accentMat);
    hub.position.y = 10 + mastH + 5;
    g.add(hub);
    for (let k = 0; k < 4; k++) {
      const a = (k / 4) * Math.PI * 2;
      const arm = new THREE.Mesh(new THREE.CylinderGeometry(1.5, 1.5, 20, 6), baseMat);
      arm.rotation.z = Math.PI / 2;
      arm.rotation.y = a;
      arm.position.set(Math.cos(a) * 10, 10 + mastH + 3, Math.sin(a) * 10);
      g.add(arm);
    }
  }

  const hitMesh = new THREE.Mesh(
    new THREE.SphereGeometry(40, 8, 8),
    new THREE.MeshBasicMaterial({ visible: false })
  );
  hitMesh.position.y = 10 + mastH;
  g.add(hitMesh);

  return { group: g, rotor, hitMesh, topY: 10 + mastH + 42 };
}

function createRangeSectorRing(sensor, groundY, isGhost = false, isPreview = false) {
  const palette = SENSOR_COLORS[sensor.type] || SENSOR_COLORS.radar;
  const r = Math.min(sensor.effectiveRangeM, 18000);
  const full = sensor.sectorWidthDeg >= 359.5;
  const pts = [];
  const steps = full ? 72 : Math.max(24, Math.ceil(sensor.sectorWidthDeg / 4));
  const startDeg = full ? 0 : sensor.sectorCenterDeg - sensor.sectorWidthDeg / 2;
  const spanDeg = full ? 360 : sensor.sectorWidthDeg;

  if (!full) pts.push(new THREE.Vector3(0, 0, 0));
  for (let i = 0; i <= steps; i++) {
    const azRad = (startDeg + (i / steps) * spanDeg) * DEG2RAD;
    pts.push(new THREE.Vector3(Math.sin(azRad) * r, 0, -Math.cos(azRad) * r));
  }
  if (!full) pts.push(new THREE.Vector3(0, 0, 0));

  const geom = new THREE.BufferGeometry().setFromPoints(pts);
  const mat = new THREE.LineBasicMaterial({
    color: isGhost ? 0x94a3b8 : isPreview ? 0x34d399 : palette.hex,
    transparent: true,
    opacity: isGhost ? 0.35 : 0.75,
  });
  const line = new THREE.Line(geom, mat);
  line.position.set(sensor.enu[0], groundY + 6, -sensor.enu[1]);
  return line;
}

function floorColorRGB(floorVal) {
  if (floorVal >= 65535) return [0.16, 0.08, 0.26, 0.82];
  if (floorVal <= 25) return [0.12, 0.90, 0.86, 0.82];
  if (floorVal <= 60) return [0.18, 0.86, 0.48, 0.78];
  if (floorVal <= 120) return [0.96, 0.78, 0.20, 0.80];
  if (floorVal <= 260) return [0.98, 0.44, 0.16, 0.84];
  return [0.92, 0.20, 0.26, 0.90];
}

// ── SVG Chart Builders ──
function renderAltitudeCurveSvg(baseCurve, optCurve, currentAgl) {
  if (!baseCurve || !baseCurve.length) {
    return '<div style="font-size:10.5px;color:var(--muted);">Altitude curve available on v0.2.0 runs.</div>';
  }
  const W = 320, H = 96, padL = 28, padR = 10, padT = 8, padB = 18;
  const maxAgl = 600;
  const sx = (agl) => padL + (Math.min(maxAgl, agl) / maxAgl) * (W - padL - padR);
  const sy = (frac) => padT + (1 - Math.max(0, Math.min(1, frac))) * (H - padT - padB);
  const pathOf = (curve) =>
    curve.map((p, i) => `${i === 0 ? 'M' : 'L'}${sx(p.aglM).toFixed(1)},${sy(p.coverageFraction).toFixed(1)}`).join(' ');

  const cx = sx(currentAgl).toFixed(1);
  return `
    <svg viewBox="0 0 ${W} ${H}" style="width:100%;height:auto;display:block;">
      ${[0, 0.5, 1].map((f) => `
        <line x1="${padL}" y1="${sy(f)}" x2="${W - padR}" y2="${sy(f)}" stroke="rgba(255,255,255,0.08)" stroke-width="1"/>
        <text x="${padL - 4}" y="${sy(f) + 3}" text-anchor="end" fill="#8ea6b8" font-size="8">${Math.round(f * 100)}%</text>
      `).join('')}
      ${[30, 150, 300, 600].map((a) => `
        <text x="${sx(a)}" y="${H - 4}" text-anchor="middle" fill="#8ea6b8" font-size="8">${a}m</text>
      `).join('')}
      <line x1="${cx}" y1="${padT}" x2="${cx}" y2="${H - padB}" stroke="#38bdf8" stroke-width="1.2" stroke-dasharray="3,2"/>
      <path d="${pathOf(baseCurve)}" fill="none" stroke="#fbbf24" stroke-width="1.6" stroke-dasharray="4,2"/>
      ${optCurve ? `<path d="${pathOf(optCurve)}" fill="none" stroke="#34d399" stroke-width="2"/>` : ''}
    </svg>
    <div style="display:flex;gap:10px;font-size:10px;color:var(--muted);margin-top:2px;">
      <span><span style="color:#fbbf24;">━ ━</span> Baseline</span>
      ${optCurve ? '<span><span style="color:#34d399;">━━</span> Optimized</span>' : ''}
      <span style="margin-left:auto;color:#38bdf8;">Slice: ${currentAgl}m</span>
    </div>`;
}

function renderLosProfileSvg(ray) {
  if (!ray || !ray.profile || ray.profile.length < 2) {
    return '<div style="font-size:10.5px;color:var(--muted);">No elevation samples.</div>';
  }
  const prof = ray.profile;
  const W = 340, H = 104, padL = 34, padR = 10, padT = 10, padB = 18;
  const maxD = Math.max(1, prof[prof.length - 1][0]);
  let minZ = Infinity, maxZ = -Infinity;
  for (const [, tz, sz, rz] of prof) {
    if (tz < minZ) minZ = tz;
    if (sz > maxZ) maxZ = sz;
    if (rz > maxZ) maxZ = rz;
    if (rz < minZ) minZ = rz;
  }
  const zSpan = Math.max(25, maxZ - minZ);
  minZ -= zSpan * 0.08;
  maxZ += zSpan * 0.12;

  const sx = (d) => padL + (d / maxD) * (W - padL - padR);
  const sy = (z) => padT + (1 - (z - minZ) / (maxZ - minZ)) * (H - padT - padB);
  const bottomY = H - padB;

  const terrPts = prof.map(([d, tz]) => `${sx(d).toFixed(1)},${sy(tz).toFixed(1)}`).join(' ');
  const terrPoly = `${sx(0).toFixed(1)},${bottomY} ${terrPts} ${sx(maxD).toFixed(1)},${bottomY}`;
  const surfPts = prof.map(([d, , sz]) => `${sx(d).toFixed(1)},${sy(sz).toFixed(1)}`).join(' ');
  const surfPoly = `${sx(0).toFixed(1)},${bottomY} ${surfPts} ${sx(maxD).toFixed(1)},${bottomY}`;
  const rayPath = prof.map(([d, , , rz], i) => `${i === 0 ? 'M' : 'L'}${sx(d).toFixed(1)},${sy(rz).toFixed(1)}`).join(' ');

  let hitDot = '';
  if (ray.blockDistanceM != null && ray.blockPointEnu) {
    const hx = sx(ray.blockDistanceM).toFixed(1);
    const hy = sy(ray.blockPointEnu[2]).toFixed(1);
    hitDot = `<circle cx="${hx}" cy="${hy}" r="4" fill="#f87171" stroke="#fff" stroke-width="1.2"/>`;
  }
  const rayStroke = ray.clear ? '#34d399' : '#f87171';
  const statusTxt = ray.status.replace(/_/g, ' ');
  return `
    <svg viewBox="0 0 ${W} ${H}" style="width:100%;height:auto;display:block;">
      <polygon points="${surfPoly}" fill="rgba(148,163,184,0.45)" stroke="rgba(203,213,225,0.6)" stroke-width="0.7"/>
      <polygon points="${terrPoly}" fill="rgba(100,116,96,0.65)" stroke="rgba(148,163,138,0.8)" stroke-width="1"/>
      <path d="${rayPath}" fill="none" stroke="${rayStroke}" stroke-width="1.8" ${ray.clear ? '' : 'stroke-dasharray="4,2"'}/>
      ${hitDot}
      <text x="${padL - 4}" y="${padT + 6}" text-anchor="end" fill="#8ea6b8" font-size="8">${Math.round(maxZ)}m</text>
      <text x="${padL - 4}" y="${bottomY}" text-anchor="end" fill="#8ea6b8" font-size="8">${Math.round(minZ)}m</text>
      <text x="${padL}" y="${H - 4}" fill="#8ea6b8" font-size="8">Sensor</text>
      <text x="${W - padR}" y="${H - 4}" text-anchor="end" fill="#8ea6b8" font-size="8">${(maxD / 1000).toFixed(2)} km (Target)</text>
    </svg>
    <div style="display:flex;justify-content:space-between;font-size:10px;margin-top:2px;color:var(--muted);">
      <span>Status: <strong style="color:${rayStroke};">${statusTxt}</strong></span>
      <span>${ray.blockDistanceM != null ? `Blocked @ ${(ray.blockDistanceM / 1000).toFixed(2)} km` : ray.minClearanceM != null ? `Min clr: ${ray.minClearanceM} m` : `Range: ${(ray.distanceM / 1000).toFixed(2)} km`}</span>
    </div>`;
}

function compactRunTitle(title) {
  return String(title || 'Study').replace(/^Riyadh Reference Area\s*[—-]\s*/i, '').replace(/^Owned Fixture Verification Environment\s*[—-]\s*/i, '');
}

async function resolveEndpoints(stageEl) {
  const params = new URLSearchParams(window.location.search);
  const mapOrigin = params.get('mapOrigin');
  if (mapOrigin) {
    document.getElementById('back-to-map').href = `${mapOrigin.replace(/\/$/, '')}/map`;
  } else if (window.location.pathname.startsWith('/sim-viewers/')) {
    document.getElementById('back-to-map').href = '/map';
  }
  const runParam = params.get('run');

  try {
    const hRes = await apiFetch('/health', { cache: 'no-store' });
    if (hRes.ok) {
      const health = await hRes.json();
      if (health.status === 'ok') {
        const runsRes = await apiFetch('/runs', { cache: 'no-store' });
        const runsData = runsRes.ok ? await runsRes.json() : { runs: [] };
        const runs = runsData.runs || [];

        let jobId = runParam;
        if (!jobId) {
          const completed = runs.find((r) => r.status === 'completed');
          if (completed) {
            jobId = completed.jobId;
          } else {
            stageEl.textContent = 'No completed study yet — launching default Riyadh coverage study…';
            const envRes = await apiFetch('/environments', { cache: 'no-store' });
            const envData = await envRes.json();
            const firstEnv = (envData.environments || [])[0];
            if (!firstEnv) throw new Error('No verified EMAD environments discovered.');
            const createRes = await apiFetch('/runs', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(firstEnv.defaults),
            });
            const created = await createRes.json();
            jobId = created.jobId;
          }
        }

        let job = null;
        for (let attempt = 0; attempt < 360; attempt++) {
          const jRes = await apiFetch(`/runs/${encodeURIComponent(jobId)}`, { cache: 'no-store' });
          if (jRes.status === 404) {
            stageEl.textContent = 'Waiting for study registration from 2D map…';
            await new Promise((r) => setTimeout(r, 250));
            continue;
          }
          if (!jRes.ok) throw new Error(`Study ${jobId} returned HTTP ${jRes.status}`);
          job = await jRes.json();
          if (job.status === 'completed') break;
          if (job.status === 'failed' || job.status === 'interrupted' || job.status === 'cancelled') {
            throw new Error(job.error || job.stage || 'Coverage study failed.');
          }
          stageEl.textContent = job.stage || 'Computing viewsheds & optimizing placement…';
          await new Promise((r) => setTimeout(r, 280));
        }
        if (!job || job.status !== 'completed') throw new Error('Timed out waiting for coverage study.');

        const freshRunsRes = await apiFetch('/runs', { cache: 'no-store' });
        const freshRuns = freshRunsRes.ok ? ((await freshRunsRes.json()).runs || []) : runs;
        return {
          serviceMode: true,
          jobId,
          runs: freshRuns,
          hasGlb: Boolean(job.hasGlb),
          hasImagery: Boolean(job.hasImagery),
          replayUrl: `${API_BASE}/runs/${encodeURIComponent(jobId)}/replay.json`,
          glbUrl: `${API_BASE}/runs/${encodeURIComponent(jobId)}/terrain.glb`,
          imageryUrl: `${API_BASE}/runs/${encodeURIComponent(jobId)}/imagery.json`,
        };
      }
    }
  } catch (err) {
    if (runParam) throw err;
  }

  const meta = await (await fetch('/data/meta.json')).json();
  return {
    serviceMode: false,
    jobId: null,
    runs: [],
    hasGlb: Boolean(meta.hasGlb),
    hasImagery: Boolean(meta.hasImagery),
    replayUrl: '/data/replay.json',
    glbUrl: '/data/terrain.glb',
    imageryUrl: '/data/imagery.json',
  };
}

async function main() {
  const overlayEl = document.getElementById('loading-overlay');
  const stageEl = document.getElementById('loading-stage');
  const statusEl = document.getElementById('status-line');
  const container = document.getElementById('viewport');

  const renderer = new THREE.WebGLRenderer({ antialias: true, powerPreference: 'high-performance' });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
  renderer.setSize(window.innerWidth, window.innerHeight);
  renderer.setClearColor(0x071018);
  renderer.toneMapping = THREE.ACESFilmicToneMapping;
  renderer.toneMappingExposure = 1.08;
  container.appendChild(renderer.domElement);

  const scene = new THREE.Scene();
  scene.fog = new THREE.FogExp2(0x071018, 0.000038);

  const camera = new THREE.PerspectiveCamera(48, window.innerWidth / window.innerHeight, 2, 75000);
  const controls = new OrbitControls(camera, renderer.domElement);
  controls.enableDamping = true;
  controls.dampingFactor = 0.08;
  controls.maxPolarAngle = Math.PI * 0.495;

  scene.add(new THREE.HemisphereLight(0xd9ecff, 0x3b352a, 1.45));
  const sun = new THREE.DirectionalLight(0xfff4df, 1.75);
  sun.position.set(2500, 6200, 1800);
  scene.add(sun);

  const worldRoot = new THREE.Group();
  scene.add(worldRoot);

  const layerGroups = {
    terrain: new THREE.Group(),
    imagery: new THREE.Group(),
    buildings: new THREE.Group(),
    roads: new THREE.Group(),
    coverage: new THREE.Group(),
    sensors: new THREE.Group(),
    labels: new THREE.Group(),
    moves: new THREE.Group(),
    assets: new THREE.Group(),
    routes: new THREE.Group(),
    probes: new THREE.Group(),
    inspectLos: new THREE.Group(),
    candidates: new THREE.Group(),
    stealth: new THREE.Group(),
    jammers: new THREE.Group(),
  };
  Object.values(layerGroups).forEach((g) => worldRoot.add(g));
  layerGroups.candidates.visible = false;

  let replay = null;
  let serviceMode = false;
  let loadedEnvManifest = null;
  let terrainRef = null;
  let terrainMinY = 0;
  let zScale = 1.0;
  let sceneCenter = new THREE.Vector3();
  let sceneSize = 4000;

  let decodedGrid = null;
  let decodedFloors = { baseline: [], optimized: [] };
  let previewSensors = [];
  let activeConfig = 'optimized';
  let displayMode = 'coverage';
  let sliceAglM = 30;
  let floatAtAgl = false;
  let meshOpacity = 0.68;
  let disabledSensors = new Set();
  let dropSensorMode = false;

  let coverageDrapeGroup = null;
  let coverageFloatMesh = null;
  let coverageTex = null;
  let coverageTexData = null;
  let coverageDrapeMat = null;
  let coverageFloatMat = null;
  let sensorRotors = [];
  let staticPickables = [];
  let routePickables = [];

  let probePlaying = false;
  let probeT = 0;
  let probeMarkers = [];

  let lastInspectedCell = null;
  let lastLosData = null;
  let selectedLosRayIdx = 0;

  let camTransition = null;
  const keysHeld = new Set();

  function applyZScale(val) {
    zScale = val;
    worldRoot.scale.set(1, zScale, 1);
    worldRoot.position.y = terrainMinY * (1 - zScale);
    document.getElementById('zscale-val').textContent = `${zScale.toFixed(2)}×`;
  }

  function toWorldPos(localVec) {
    return new THREE.Vector3(localVec.x, (localVec.y - terrainMinY) * zScale + terrainMinY, localVec.z);
  }

  function animateCameraTo(targetPos, offset, dur = 0.55) {
    const worldT = toWorldPos(targetPos);
    camTransition = {
      startPos: camera.position.clone(),
      endPos: worldT.clone().add(offset),
      startTarget: controls.target.clone(),
      endTarget: worldT,
      t: 0,
      dur,
    };
  }

  function setDropMode(enabled) {
    dropSensorMode = Boolean(enabled && serviceMode);
    container.classList.toggle('drop-cursor', dropSensorMode);
    document.getElementById('drop-mode-banner').classList.toggle('visible', dropSensorMode);
    const btn = document.getElementById('btn-drop-sensor');
    btn.classList.toggle('good-active', dropSensorMode);
    btn.textContent = dropSensorMode ? '📍 Click 3D Terrain…' : '📍 Drop Test Sensor';
  }

  const inspCard = document.getElementById('inspector-card');
  const inspTitle = document.getElementById('insp-title');
  const inspBody = document.getElementById('insp-body');
  const inspLosSec = document.getElementById('insp-los-section');
  const inspLosPills = document.getElementById('insp-los-pills');
  const inspLosSvg = document.getElementById('insp-los-svg');

  function clearInspection() {
    inspCard.classList.remove('visible');
    inspLosSec.style.display = 'none';
    disposeGroup(layerGroups.inspectLos);
    lastInspectedCell = null;
    lastLosData = null;
  }
  document.getElementById('insp-close').onclick = clearInspection;

  function showInspection(info, keepLos = false) {
    inspTitle.textContent = info.title;
    inspBody.innerHTML = info.items
      .map(([k, v]) => `<div class="insp-kv"><span>${k}</span><strong>${v}</strong></div>`)
      .join('');
    if (!keepLos) {
      inspLosSec.style.display = 'none';
      disposeGroup(layerGroups.inspectLos);
      lastInspectedCell = null;
      lastLosData = null;
    }
    inspCard.classList.add('visible');
  }

  function render3dLosRays(losData) {
    disposeGroup(layerGroups.inspectLos);
    if (!losData) return;

    const tp = enuToThree(losData.targetEnu);
    const gp = new THREE.Vector3(tp.x, losData.targetGroundUpM, tp.z);

    const poleGeom = new THREE.BufferGeometry().setFromPoints([gp, tp]);
    layerGroups.inspectLos.add(new THREE.Line(poleGeom, new THREE.LineBasicMaterial({ color: 0xffffff })));
    const orb = new THREE.Mesh(
      new THREE.SphereGeometry(14, 12, 10),
      new THREE.MeshBasicMaterial({ color: 0x38bdf8 })
    );
    orb.position.copy(tp);
    layerGroups.inspectLos.add(orb);

    const groundRingPts = [];
    for (let i = 0; i <= 32; i++) {
      const a = (i / 32) * Math.PI * 2;
      groundRingPts.push(new THREE.Vector3(gp.x + Math.cos(a) * 32, gp.y + 4, gp.z + Math.sin(a) * 32));
    }
    layerGroups.inspectLos.add(
      new THREE.Line(
        new THREE.BufferGeometry().setFromPoints(groundRingPts),
        new THREE.LineBasicMaterial({ color: 0x38bdf8 })
      )
    );

    const visibleRays = (losData.rays || []).filter((r) => !disabledSensors.has(r.sensorId));
    visibleRays.forEach((ray, idx) => {
      const sp = enuToThree(ray.sensorEnu);
      const isSelected = idx === selectedLosRayIdx;

      if (ray.clear) {
        const g = new THREE.BufferGeometry().setFromPoints([sp, tp]);
        layerGroups.inspectLos.add(
          new THREE.Line(
            g,
            new THREE.LineBasicMaterial({ color: isSelected ? 0x6ee7b7 : 0x34d399, transparent: true, opacity: isSelected ? 1.0 : 0.72 })
          )
        );
      } else if (ray.blockPointEnu) {
        const bp = enuToThree(ray.blockPointEnu);
        const g1 = new THREE.BufferGeometry().setFromPoints([sp, bp]);
        layerGroups.inspectLos.add(
          new THREE.Line(g1, new THREE.LineBasicMaterial({ color: 0xf87171, transparent: true, opacity: isSelected ? 0.95 : 0.65 }))
        );
        const g2 = new THREE.BufferGeometry().setFromPoints([bp, tp]);
        const dashed = new THREE.Line(
          g2,
          new THREE.LineDashedMaterial({ color: 0xf87171, dashSize: 35, gapSize: 25, transparent: true, opacity: 0.45 })
        );
        dashed.computeLineDistances();
        layerGroups.inspectLos.add(dashed);

        const hitMarker = new THREE.Mesh(
          new THREE.OctahedronGeometry(isSelected ? 22 : 15, 0),
          new THREE.MeshBasicMaterial({ color: 0xff3b52 })
        );
        hitMarker.position.set(bp.x, bp.y + 10, bp.z);
        layerGroups.inspectLos.add(hitMarker);
      } else {
        const g = new THREE.BufferGeometry().setFromPoints([sp, tp]);
        const dashed = new THREE.Line(
          g,
          new THREE.LineDashedMaterial({ color: 0x94a3b8, dashSize: 45, gapSize: 35, transparent: true, opacity: 0.35 })
        );
        dashed.computeLineDistances();
        layerGroups.inspectLos.add(dashed);
      }
    });
  }

  function renderLosInspectorPanel(losData) {
    lastLosData = losData;
    const rays = (losData?.rays || []).filter((r) => !disabledSensors.has(r.sensorId));
    if (!rays.length) {
      inspLosSec.style.display = 'none';
      return;
    }
    if (selectedLosRayIdx >= rays.length) selectedLosRayIdx = 0;
    inspLosSec.style.display = 'block';
    inspLosPills.innerHTML = rays
      .map(
        (r, i) => `
          <button class="los-pill ${r.clear ? 'clear' : 'blocked'} ${i === selectedLosRayIdx ? 'active' : ''}" data-ray-idx="${i}">
            ${r.clear ? '✓' : '✕'} ${r.label}
          </button>`
      )
      .join('');
    inspLosPills.querySelectorAll('button').forEach((btn) => {
      btn.onclick = () => {
        selectedLosRayIdx = Number(btn.dataset.rayIdx);
        renderLosInspectorPanel(losData);
        render3dLosRays(losData);
      };
    });
    inspLosSvg.innerHTML = renderLosProfileSvg(rays[selectedLosRayIdx]);
    render3dLosRays(losData);
  }

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
      const gltf = await loadGlbAuth(ep.glbUrl);
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
          verts[ptr++] = e;
          verts[ptr++] = grid.up[j * grid.nx + i];
          verts[ptr++] = -n;
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
    terrainRef = terrainObject;

    if (ep.hasImagery && terrainObject) {
      try {
        statusEl.textContent = 'Draping Sentinel-2 optical imagery…';
        const rawImg = await (await fetch(ep.imageryUrl, { headers: apiHeaders() })).json();
        const imgData = rawImg.data || rawImg;
        if (imgData && imgData.texture && imgData.bounds) {
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
        }
      } catch (e) {
        console.warn('Optical drape skipped:', e);
      }
    } else {
      document.getElementById('tog-imagery').checked = false;
      document.getElementById('tog-imagery').disabled = true;
    }

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
          new THREE.MeshStandardMaterial({
            color: 0xa2b8cc, roughness: 0.68, metalness: 0.14, transparent: true, opacity: 0.80, side: THREE.DoubleSide,
          })
        )
      );
    }

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
        new THREE.LineSegments(geom, new THREE.LineBasicMaterial({ color: 0xffd27d, transparent: true, opacity: 0.30 }))
      );
    }
  }

  function buildCoverageMeshShell() {
    disposeGroup(layerGroups.coverage);
    if (coverageTex) {
      coverageTex.dispose();
      coverageTex = null;
    }

    const { nx, ny, eastMin, northMin, cellM, ground, valid } = decodedGrid;
    const spanE = nx * cellM;
    const spanN = ny * cellM;

    coverageTexData = new Uint8Array(nx * ny * 4);
    coverageTex = new THREE.DataTexture(coverageTexData, nx, ny, THREE.RGBAFormat);
    coverageTex.colorSpace = THREE.SRGBColorSpace;
    coverageTex.flipY = false;
    coverageTex.minFilter = THREE.LinearFilter;
    coverageTex.magFilter = THREE.LinearFilter;
    coverageTex.wrapS = THREE.ClampToEdgeWrapping;
    coverageTex.wrapT = THREE.ClampToEdgeWrapping;
    coverageTex.needsUpdate = true;

    coverageDrapeMat = new THREE.MeshBasicMaterial({
      map: coverageTex,
      transparent: true,
      opacity: meshOpacity,
      depthWrite: false,
      polygonOffset: true,
      polygonOffsetFactor: -4,
      polygonOffsetUnits: -4,
    });

    coverageDrapeGroup = new THREE.Group();
    if (terrainRef) {
      terrainRef.traverse((o) => {
        if (!o.isMesh) return;
        const g = o.geometry.clone();
        const pos = g.getAttribute('position');
        const uv = new Float32Array(pos.count * 2);
        for (let i = 0; i < pos.count; i++) {
          const e = pos.getX(i);
          const n = -pos.getZ(i);
          uv[2 * i] = (e - eastMin) / spanE;
          uv[2 * i + 1] = (n - northMin) / spanN;
        }
        g.setAttribute('uv', new THREE.BufferAttribute(uv, 2));
        const m = new THREE.Mesh(g, coverageDrapeMat);
        m.renderOrder = 6;
        m.userData.isCoverageGrid = true;
        coverageDrapeGroup.add(m);
      });
    }
    layerGroups.coverage.add(coverageDrapeGroup);

    const count = nx * ny;
    const pos = new Float32Array(count * 3);
    const uv = new Float32Array(count * 2);
    for (let j = 0; j < ny; j++) {
      const n = northMin + (j + 0.5) * cellM;
      for (let i = 0; i < nx; i++) {
        const idx = j * nx + i;
        const e = eastMin + (i + 0.5) * cellM;
        pos[idx * 3] = e;
        pos[idx * 3 + 1] = ground[idx] + sliceAglM;
        pos[idx * 3 + 2] = -n;
        uv[idx * 2] = (i + 0.5) / nx;
        uv[idx * 2 + 1] = (j + 0.5) / ny;
      }
    }
    const indices = [];
    for (let j = 0; j < ny - 1; j++) {
      for (let i = 0; i < nx - 1; i++) {
        const a = j * nx + i, b = a + 1, c = a + nx, d = c + 1;
        if (valid[a] && valid[b] && valid[c] && valid[d]) {
          indices.push(a, c, b, b, c, d);
        }
      }
    }
    const floatGeom = new THREE.BufferGeometry();
    floatGeom.setAttribute('position', new THREE.BufferAttribute(pos, 3));
    floatGeom.setAttribute('uv', new THREE.BufferAttribute(uv, 2));
    floatGeom.setIndex(indices);
    floatGeom.computeVertexNormals();

    coverageFloatMat = new THREE.MeshBasicMaterial({
      map: coverageTex,
      transparent: true,
      opacity: meshOpacity,
      side: THREE.DoubleSide,
      depthWrite: false,
    });
    coverageFloatMesh = new THREE.Mesh(floatGeom, coverageFloatMat);
    coverageFloatMesh.renderOrder = 7;
    coverageFloatMesh.userData.isCoverageGrid = true;
    coverageFloatMesh.visible = floatAtAgl;
    layerGroups.coverage.add(coverageFloatMesh);
  }

  function getActiveConfigName() {
    return replay?.configs?.[activeConfig] ? activeConfig : 'baseline';
  }

  function getActiveSensorsAndFloors() {
    const cfgName = getActiveConfigName();
    const cfg = replay.configs[cfgName];
    const baseFloors = decodedFloors[cfgName] || [];
    const sensors = [];
    const floors = [];
    cfg.sensors.forEach((s, i) => {
      if (!disabledSensors.has(s.id)) {
        sensors.push(s);
        floors.push(baseFloors[i]);
      }
    });
    previewSensors.forEach(({ sensor, floor }) => {
      if (!disabledSensors.has(sensor.id)) {
        sensors.push(sensor);
        floors.push(floor);
      }
    });
    return { cfgName, cfg, sensors, floors };
  }

  function updateCoverageSurfaceAndMetrics() {
    if (!replay || !decodedGrid || !coverageTex || !coverageTexData) return;

    const { nx, ny, ground, weights, valid, cellAreaKm2 } = decodedGrid;
    const count = nx * ny;
    const { cfgName, cfg, floors: activeFloors } = getActiveSensorsAndFloors();
    const baseOnlyFloors = (decodedFloors[cfgName] || []).filter((_, i) => !disabledSensors.has(cfg.sensors[i].id));

    const hasOpt = Boolean(replay.configs.optimized);
    const effectiveMode = displayMode === 'diff' && !hasOpt ? 'coverage' : displayMode;
    const agl = sliceAglM;
    const tex = coverageTexData;

    if (coverageDrapeGroup) coverageDrapeGroup.visible = !floatAtAgl;
    if (coverageFloatMesh) {
      coverageFloatMesh.visible = floatAtAgl;
      if (floatAtAgl) {
        const posAttr = coverageFloatMesh.geometry.getAttribute('position');
        const pos = posAttr.array;
        for (let idx = 0; idx < count; idx++) pos[idx * 3 + 1] = ground[idx] + agl;
        posAttr.needsUpdate = true;
      }
    }

    let validCount = 0, coveredCount = 0, withoutPreviewCovered = 0;
    let redundantCount = 0, blindCount = 0;
    let wTotal = 0, wCovered = 0, priTotal = 0, priCovered = 0;
    let gainedCount = 0, lostCount = 0;

    const baseFloors = decodedFloors.baseline || [];
    const optFloors = decodedFloors.optimized || [];

    const setRGBA = (idx, r, g, b, a) => {
      const o = idx * 4;
      tex[o] = Math.round(r * 255);
      tex[o + 1] = Math.round(g * 255);
      tex[o + 2] = Math.round(b * 255);
      tex[o + 3] = Math.round(a * 255);
    };

    for (let idx = 0; idx < count; idx++) {
      if (!valid[idx]) {
        setRGBA(idx, 0, 0, 0, 0);
        continue;
      }

      let detCount = 0;
      let minFloor = 65535;
      for (let k = 0; k < activeFloors.length; k++) {
        const f = activeFloors[k][idx];
        if (f < minFloor) minFloor = f;
        if (f <= agl) detCount++;
      }

      if (previewSensors.length > 0) {
        for (let k = 0; k < baseOnlyFloors.length; k++) {
          if (baseOnlyFloors[k][idx] <= agl) { withoutPreviewCovered++; break; }
        }
      }

      validCount++;
      const w = weights[idx];
      wTotal += w;
      if (w > 1.01) priTotal++;
      if (detCount >= 1) {
        coveredCount++;
        wCovered += w;
        if (w > 1.01) priCovered++;
        if (detCount >= 2) redundantCount++;
      } else {
        blindCount++;
      }

      if (effectiveMode === 'coverage') {
        if (detCount === 0) {
          setRGBA(idx, 0.96, 0.18, 0.24, 0.94);
        } else if (detCount === 1) {
          setRGBA(idx, 0.10, 0.84, 0.54, 0.68);
        } else {
          setRGBA(idx, 0.14, 0.68, 0.98, 0.78);
        }
      } else if (effectiveMode === 'floor') {
        const rgba = floorColorRGB(minFloor);
        setRGBA(idx, rgba[0], rgba[1], rgba[2], Math.min(0.95, rgba[3] + 0.12));
      } else {
        let bCov = false;
        for (let k = 0; k < baseFloors.length; k++) {
          if (baseFloors[k][idx] <= agl) { bCov = true; break; }
        }
        let oCov = false;
        for (let k = 0; k < optFloors.length; k++) {
          const sId = replay.configs.optimized.sensors[k].id;
          if (activeConfig === 'optimized' && disabledSensors.has(sId)) continue;
          if (optFloors[k][idx] <= agl) { oCov = true; break; }
        }
        for (let k = 0; k < previewSensors.length; k++) {
          if (!disabledSensors.has(previewSensors[k].sensor.id) && previewSensors[k].floor[idx] <= agl) {
            oCov = true; break;
          }
        }
        if (oCov && !bCov) {
          gainedCount++;
          setRGBA(idx, 0.12, 0.96, 0.48, 0.95);
        } else if (!oCov && bCov) {
          lostCount++;
          setRGBA(idx, 0.99, 0.68, 0.12, 0.92);
        } else if (oCov && bCov) {
          setRGBA(idx, 0.14, 0.52, 0.76, 0.42);
        } else {
          setRGBA(idx, 0.88, 0.18, 0.24, 0.72);
        }
      }
    }

    coverageTex.needsUpdate = true;
    if (coverageDrapeMat) coverageDrapeMat.opacity = meshOpacity;
    if (coverageFloatMat) coverageFloatMat.opacity = meshOpacity;

    const covPct = validCount ? (coveredCount / validCount) * 100 : 0;
    const wCovPct = wTotal ? (wCovered / wTotal) * 100 : 0;
    const redPct = validCount ? (redundantCount / validCount) * 100 : 0;
    const priPct = priTotal ? (priCovered / priTotal) * 100 : null;
    const blindKm2 = blindCount * cellAreaKm2;
    const tagNote =
      (disabledSensors.size ? ` (${disabledSensors.size} off)` : '') +
      (previewSensors.length ? ` (+${previewSensors.length} test)` : '');

    document.getElementById('live-kpis').innerHTML = `
      <div class="kpi">
        <div class="k">Covered @ ${agl}m AGL</div>
        <div class="v ${covPct >= 75 ? 'good' : covPct >= 50 ? 'warn' : 'bad'}">${covPct.toFixed(1)}%</div>
        <div class="s">Weighted ${wCovPct.toFixed(1)}%${tagNote}</div>
      </div>
      <div class="kpi">
        <div class="k">Blind Area</div>
        <div class="v ${blindKm2 <= 1 ? 'good' : blindKm2 <= 8 ? 'warn' : 'bad'}">${blindKm2.toFixed(2)} km²</div>
        <div class="s">${((blindCount / Math.max(validCount, 1)) * 100).toFixed(1)}% of AOI</div>
      </div>
      <div class="kpi">
        <div class="k">Priority &amp; Corridors</div>
        <div class="v ${priPct !== null && priPct >= 80 ? 'good' : 'warn'}">${priPct !== null ? priPct.toFixed(1) + '%' : '—'}</div>
        <div class="s">Assets + ingress zones</div>
      </div>
      <div class="kpi">
        <div class="k">2+ Redundancy</div>
        <div class="v cyan">${redPct.toFixed(1)}%</div>
        <div class="s">Objective ${cfg.metrics.objective.toFixed(4)}</div>
      </div>
    `;

    const prevBox = document.getElementById('preview-summary');
    if (previewSensors.length > 0) {
      prevBox.style.display = 'block';
      const basePct = validCount ? (withoutPreviewCovered / validCount) * 100 : 0;
      const dPct = covPct - basePct;
      const dKm2 = (coveredCount - withoutPreviewCovered) * cellAreaKm2;
      document.getElementById('preview-delta-text').innerHTML =
        `<strong>+${previewSensors.length} test sensor${previewSensors.length > 1 ? 's' : ''}:</strong> ` +
        `<strong>${dPct >= 0 ? '+' : ''}${dPct.toFixed(1)} pp</strong> coverage ` +
        `(<strong>−${Math.max(0, dKm2).toFixed(2)} km²</strong> blind @ ${agl}m)`;
    } else {
      prevBox.style.display = 'none';
    }

    document.getElementById('altitude-curve-box').innerHTML = renderAltitudeCurveSvg(
      replay.configs.baseline?.metrics?.coverageCurve,
      replay.configs.optimized?.metrics?.coverageCurve,
      agl
    );

    const legEl = document.getElementById('surface-legend');
    if (effectiveMode === 'coverage') {
      legEl.innerHTML = `
        <div class="legend-row">
          <span class="legend-item"><span class="dot" style="background:#f52e3d"></span>Blind Spot (0)</span>
          <span class="legend-item"><span class="dot" style="background:#1ad68a"></span>Covered (1)</span>
          <span class="legend-item"><span class="dot" style="background:#24adfa"></span>Redundant (2+)</span>
        </div>`;
    } else if (effectiveMode === 'floor') {
      legEl.innerHTML = `
        <div class="legend-bar" style="background:linear-gradient(90deg,#1fe6db 0%,#2edb7a 24%,#f5c733 48%,#fa7029 72%,#eb3342 88%,#291442 100%);"></div>
        <div class="legend-labels"><span>&lt;25m</span><span>60m</span><span>120m</span><span>&gt;260m</span><span>Masked</span></div>`;
    } else {
      const gKm2 = (gainedCount * cellAreaKm2).toFixed(2);
      const lKm2 = (lostCount * cellAreaKm2).toFixed(2);
      legEl.innerHTML = `
        <div class="legend-row">
          <span class="legend-item"><span class="dot" style="background:#1ff57a"></span>Gained (+${gKm2} km²)</span>
          <span class="legend-item"><span class="dot" style="background:#fcad1f"></span>Tradeoff (-${lKm2} km²)</span>
          <span class="legend-item"><span class="dot" style="background:#2485c2"></span>Both</span>
          <span class="legend-item"><span class="dot" style="background:#e02e3d"></span>Still Blind</span>
        </div>`;
    }

    document.querySelectorAll('#layer-table-body tr').forEach((tr) => {
      tr.classList.toggle('selected', Math.abs(Number(tr.dataset.agl) - agl) < 1);
    });

    rebuildRoutesAndProbes();
  }

  function evaluateRouteLive(route) {
    const { sensors, floors } = getActiveSensorsAndFloors();
    const { nx, ny, eastMin, northMin, cellM } = decodedGrid;

    const detected = [];
    const seenByPerSample = [];
    for (let s = 0; s < route.samples.length; s++) {
      const [e, n, , agl] = route.samples[s];
      const fi = Math.max(0, Math.min(nx - 1, Math.floor((e - eastMin) / cellM)));
      const fj = Math.max(0, Math.min(ny - 1, Math.floor((n - northMin) / cellM)));
      const idx = fj * nx + fi;
      const by = [];
      for (let k = 0; k < sensors.length; k++) {
        if (floors[k][idx] <= agl) by.push(sensors[k]);
      }
      detected.push(by.length > 0);
      seenByPerSample.push(by);
    }

    const totalS = route.lengthM / Math.max(route.speedMps, 1);
    const first = detected.indexOf(true);
    const nPts = Math.max(route.samples.length - 1, 1);
    let warningTimeS = 0, warningDistM = 0, maxGapS = null;
    if (first >= 0) {
      const remFrac = 1 - first / nPts;
      warningDistM = route.lengthM * remFrac;
      warningTimeS = totalS * remFrac;
      let g = 0, bestG = 0;
      for (let i = first; i < detected.length; i++) {
        g = detected[i] ? 0 : g + 1;
        if (g > bestG) bestG = g;
      }
      maxGapS = (bestG / nPts) * totalS;
    }
    const trackedFrac = detected.filter(Boolean).length / Math.max(detected.length, 1);
    return { detected, seenByPerSample, first, warningTimeS, warningDistM, maxGapS, trackedFrac, totalS };
  }

  function rebuildRoutesAndProbes() {
    disposeGroup(layerGroups.routes);
    disposeGroup(layerGroups.probes);
    probeMarkers = [];
    routePickables = [];

    const routesSummaryEl = document.getElementById('routes-summary');
    const routes = replay.routes || [];
    if (!routes.length) {
      routesSummaryEl.innerHTML = '<p style="font-size:11px;color:var(--muted);">No ingress routes defined.</p>';
      return;
    }

    const cards = [];
    routes.forEach((route) => {
      const ev = evaluateRouteLive(route);
      const pts = route.samples;
      const posArr = [], colArr = [];
      for (let i = 0; i < pts.length - 1; i++) {
        const p0 = enuToThree(pts[i]);
        const p1 = enuToThree(pts[i + 1]);
        posArr.push(p0.x, p0.y, p0.z, p1.x, p1.y, p1.z);
        const det = ev.detected[i] || ev.detected[i + 1];
        const r = det ? 0.20 : 0.98, g = det ? 0.90 : 0.30, b = det ? 0.56 : 0.36;
        colArr.push(r, g, b, r, g, b);
      }
      if (posArr.length) {
        const geom = new THREE.BufferGeometry();
        geom.setAttribute('position', new THREE.Float32BufferAttribute(posArr, 3));
        geom.setAttribute('color', new THREE.Float32BufferAttribute(colArr, 3));
        layerGroups.routes.add(new THREE.LineSegments(geom, new THREE.LineBasicMaterial({ vertexColors: true })));
      }

      if (ev.first >= 0) {
        const fp = enuToThree(pts[ev.first]);
        const beacon = new THREE.Mesh(
          new THREE.OctahedronGeometry(26, 0),
          new THREE.MeshBasicMaterial({ color: 0x34d399 })
        );
        beacon.position.copy(fp);
        const byNames = ev.seenByPerSample[ev.first].map((s) => s.label).join(', ');
        beacon.userData.inspect = {
          title: `First Detection — ${route.label}`,
          items: [
            ['Warning Time', `${ev.warningTimeS.toFixed(1)} s`],
            ['Warning Distance', `${Math.round(ev.warningDistM)} m`],
            ['Detected By', byNames],
            ['Altitude AGL', `${pts[ev.first][3].toFixed(0)} m`],
            ['Tracked Fraction', `${(ev.trackedFrac * 100).toFixed(1)}%`],
          ],
        };
        layerGroups.routes.add(beacon);
        routePickables.push(beacon);
      }

      const probeMesh = new THREE.Mesh(
        new THREE.ConeGeometry(16, 42, 8),
        new THREE.MeshStandardMaterial({ color: 0xf87171, emissive: 0x7f1d1d, roughness: 0.35 })
      );
      probeMesh.visible = false;
      const losGroup = new THREE.Group();
      layerGroups.probes.add(probeMesh);
      layerGroups.probes.add(losGroup);
      probeMarkers.push({ route, ev, mesh: probeMesh, losGroup });

      const barSegs = ev.detected.map((d) => `<span style="flex:1;background:${d ? '#34d399' : '#f87171'};"></span>`).join('');
      const warnClass = ev.first < 0 ? 'bad' : ev.warningTimeS >= 25 ? 'good' : 'warn';
      const byFirst = ev.first >= 0 ? ev.seenByPerSample[ev.first].map((s) => s.label).join(', ') : 'Never detected';
      cards.push(`
        <div class="route-card">
          <div class="route-head">
            <span>${route.label}</span>
            <span class="${warnClass}">${ev.first >= 0 ? `${ev.warningTimeS.toFixed(1)} s warning` : 'UNDETECTED'}</span>
          </div>
          <div class="route-bar" title="Green = detected, Red = blind">${barSegs}</div>
          <div class="route-meta">
            <span>${route.speedMps} m/s · ${(route.lengthM / 1000).toFixed(2)} km</span>
            <span>Tracked ${(ev.trackedFrac * 100).toFixed(0)}%</span>
            <span>First: ${byFirst}</span>
            ${ev.maxGapS !== null && ev.maxGapS > 0.5 ? `<span class="warn">Max gap ${ev.maxGapS.toFixed(1)} s</span>` : ''}
          </div>
        </div>
      `);
    });
    routesSummaryEl.innerHTML = cards.join('');
    updateProbePositions();
  }

  function updateProbePositions() {
    const show = probePlaying || probeT > 0.001;
    let totalDet = 0;
    probeMarkers.forEach(({ route, ev, mesh, losGroup }) => {
      disposeGroup(losGroup);
      if (!show || route.samples.length < 2) {
        mesh.visible = false;
        return;
      }
      mesh.visible = true;
      const fIdx = probeT * (route.samples.length - 1);
      const i0 = Math.min(route.samples.length - 2, Math.floor(fIdx));
      const frac = fIdx - i0;
      const p0 = enuToThree(route.samples[i0]);
      const p1 = enuToThree(route.samples[i0 + 1]);
      const pos = p0.clone().lerp(p1, frac);
      mesh.position.copy(pos);
      const dir = p1.clone().sub(p0).normalize();
      mesh.quaternion.setFromUnitVectors(new THREE.Vector3(0, 1, 0), dir);

      const seenBy = ev.seenByPerSample[Math.round(fIdx)] || [];
      const isDet = seenBy.length > 0;
      if (isDet) totalDet++;
      mesh.material.color.setHex(isDet ? 0x34d399 : 0xf87171);
      mesh.material.emissive.setHex(isDet ? 0x065f46 : 0x7f1d1d);

      seenBy.forEach((s) => {
        const sp = enuToThree(s.enu);
        const palette = SENSOR_COLORS[s.type] || SENSOR_COLORS.radar;
        const g = new THREE.BufferGeometry().setFromPoints([sp, pos]);
        losGroup.add(new THREE.Line(g, new THREE.LineBasicMaterial({ color: palette.hex, transparent: true, opacity: 0.9 })));
      });
    });

    const readout = document.getElementById('probe-readout');
    readout.textContent = !show ? '0% · Ready' : `${Math.round(probeT * 100)}% · ${totalDet}/${probeMarkers.length} detected`;
  }

  function rebuildSceneOverlays() {
    disposeGroup(layerGroups.sensors);
    disposeGroup(layerGroups.labels);
    disposeGroup(layerGroups.moves);
    disposeGroup(layerGroups.assets);
    disposeGroup(layerGroups.candidates);
    disposeGroup(layerGroups.jammers);
    sensorRotors = [];
    staticPickables = [];
    if (coverageDrapeGroup) coverageDrapeGroup.traverse((o) => { if (o.isMesh) staticPickables.push(o); });
    if (coverageFloatMesh) staticPickables.push(coverageFloatMesh);

    const cfgName = getActiveConfigName();
    const cfg = replay.configs[cfgName];
    const baseCfg = replay.configs.baseline;
    const baseById = Object.fromEntries((baseCfg?.sensors || []).map((s) => [s.id, s]));
    const allSensors = [...cfg.sensors, ...previewSensors.map((p) => p.sensor)];

    allSensors.forEach((s) => {
      const isPreview = s.origin === 'preview';
      const off = disabledSensors.has(s.id);
      const { group, rotor, hitMesh, topY } = createSensorModel(s, off, isPreview);
      group.position.set(s.enu[0], s.siteUpM, -s.enu[1]);
      layerGroups.sensors.add(group);
      if (rotor && !off) sensorRotors.push(rotor);

      const ring = createRangeSectorRing(s, s.siteUpM, off, isPreview);
      layerGroups.sensors.add(ring);

      const palette = SENSOR_COLORS[s.type] || SENSOR_COLORS.radar;
      const lbl = createLabelSprite(s.label, isPreview ? '#34d399' : off ? '#64748b' : palette.css, 360);
      lbl.position.set(s.enu[0], s.siteUpM + topY + 45, -s.enu[1]);
      layerGroups.labels.add(lbl);

      const sole0 = (s.soleCoverageKm2 || [])[0];
      const cov0 = (s.coverageKm2 || [])[0];
      const baseLayerAgl = (replay.layers[0] || {}).aglM ?? 30;
      hitMesh.userData.inspect = {
        title: `${s.label} (${palette.name})`,
        items: [
          ['Status', off ? 'DISABLED (Outage Sim)' : isPreview ? '3D SANDBOX PREVIEW' : s.locked ? 'Locked' : s.origin],
          ['Site Elevation', `${s.siteUpM.toFixed(1)} m (${s.siteOnRoof ? 'Rooftop' : 'Ground'})`],
          ['Mast Height', `${s.mastAglM} m AGL`],
          ['Effective Range', `${(s.effectiveRangeM / 1000).toFixed(2)} km (nominal ${(s.maxRangeM / 1000).toFixed(1)} km)`],
          ['Sector', s.sectorWidthDeg >= 359 ? '360° Omnidirectional' : `${s.sectorWidthDeg}° @ ${s.sectorCenterDeg}°`],
          ...(cov0 !== undefined ? [[`Coverage @ ${baseLayerAgl}m`, `${cov0.toFixed(2)} km²`]] : []),
          ...(sole0 !== undefined ? [[`Sole Coverage @ ${baseLayerAgl}m`, `${sole0.toFixed(2)} km²`]] : []),
          ['Coordinates', `${s.lat.toFixed(5)}°, ${s.lon.toFixed(5)}°`],
        ],
      };
      staticPickables.push(hitMesh);

      if (cfgName === 'optimized' && !isPreview && baseById[s.id]) {
        const b = baseById[s.id];
        const dist = Math.hypot(s.enu[0] - b.enu[0], s.enu[1] - b.enu[1]);
        if (dist > 15) {
          const ghost = createSensorModel(b, true);
          ghost.group.position.set(b.enu[0], b.siteUpM, -b.enu[1]);
          layerGroups.moves.add(ghost.group);

          const p0 = enuToThree(b.enu);
          const p1 = enuToThree(s.enu);
          const mid = p0.clone().lerp(p1, 0.5);
          mid.y += Math.max(120, Math.min(480, dist * 0.22));
          const curve = new THREE.QuadraticBezierCurve3(p0, mid, p1);
          const arcGeom = new THREE.BufferGeometry().setFromPoints(curve.getPoints(32));
          const arc = new THREE.Line(
            arcGeom,
            new THREE.LineDashedMaterial({ color: 0x34d399, dashSize: 45, gapSize: 22 })
          );
          arc.computeLineDistances();
          layerGroups.moves.add(arc);
        }
      }
    });

    (replay.assets || []).forEach((a) => {
      const pos = enuToThree(a.enu);
      const beacon = new THREE.Mesh(
        new THREE.CylinderGeometry(14, 24, 52, 6),
        new THREE.MeshStandardMaterial({ color: 0xf472b6, emissive: 0x831843, roughness: 0.35 })
      );
      beacon.position.set(pos.x, pos.y + 26, pos.z);
      beacon.userData.inspect = {
        title: `Defended Asset: ${a.label}`,
        items: [
          ['Priority Weight', `${a.priority}×`],
          ['Warning Radius', `${(a.warningRadiusM / 1000).toFixed(2)} km`],
          ['Ground Elevation', `${a.enu[2].toFixed(1)} m`],
          ['Coordinates', `${a.lat.toFixed(5)}°, ${a.lon.toFixed(5)}°`],
        ],
      };
      layerGroups.assets.add(beacon);
      staticPickables.push(beacon);

      const aLbl = createLabelSprite(a.label, '#f472b6', 380);
      aLbl.position.set(pos.x, pos.y + 120, pos.z);
      layerGroups.labels.add(aLbl);

      const ringPts = [];
      for (let i = 0; i <= 72; i++) {
        const th = (i / 72) * Math.PI * 2;
        ringPts.push(new THREE.Vector3(Math.cos(th) * a.warningRadiusM, 0, Math.sin(th) * a.warningRadiusM));
      }
      const ring = new THREE.Line(
        new THREE.BufferGeometry().setFromPoints(ringPts),
        new THREE.LineBasicMaterial({ color: 0xf472b6, transparent: true, opacity: 0.78 })
      );
      ring.position.set(pos.x, pos.y + 8, pos.z);
      layerGroups.assets.add(ring);
    });

    // Stand-off EW Jammers (amber-crimson radar-jamming emitter + directional lobe)
    (replay.jammers || []).forEach((j) => {
      const pos = enuToThree(j.enu);
      const emitter = new THREE.Mesh(
        new THREE.OctahedronGeometry(26, 0),
        new THREE.MeshStandardMaterial({ color: 0xf97316, emissive: 0x9a3412, roughness: 0.3 })
      );
      emitter.position.set(pos.x, pos.y + 12, pos.z);
      emitter.userData.inspect = {
        title: `EW Jammer: ${j.label}`,
        items: [
          ['ERP J/S Factor', `${j.erpRatio}× @ 3 km`],
          ['Jammed Cone Width', `${j.beamWidthDeg}°`],
          ['Altitude AGL', `${j.aglM} m`],
          ['Coordinates', `${j.lat.toFixed(5)}°, ${j.lon.toFixed(5)}°`],
        ],
      };
      layerGroups.jammers.add(emitter);
      staticPickables.push(emitter);
      const jLbl = createLabelSprite(`⚡ ${j.label}`, '#f97316', 360);
      jLbl.position.set(pos.x, pos.y + 95, pos.z);
      layerGroups.labels.add(jLbl);
      // Connect jammer to each radar with an orange-dashed EW jamming beam
      allSensors.filter((s) => s.type === 'radar' && !disabledSensors.has(s.id)).forEach((rad) => {
        const rp = enuToThree(rad.enu);
        const beam = new THREE.Line(
          new THREE.BufferGeometry().setFromPoints([pos, rp]),
          new THREE.LineDashedMaterial({ color: 0xf97316, dashSize: 60, gapSize: 35, transparent: true, opacity: 0.72 })
        );
        beam.computeLineDistances();
        layerGroups.jammers.add(beam);
      });
    });

    const cands = replay.optimizer?.candidates || [];
    cands.forEach((c, idx) => {
      const h = 16 + c.gain * 90;
      const color = c.kind === 'rooftop' ? 0xfbbf24 : c.kind === 'terrain_high' ? 0x34d399 : 0x38bdf8;
      const orb = new THREE.Mesh(
        new THREE.SphereGeometry(10 + c.gain * 14, 10, 8),
        new THREE.MeshBasicMaterial({ color, transparent: true, opacity: 0.45 + c.gain * 0.5 })
      );
      orb.position.set(c.enu[0], c.enu[2] + h, -c.enu[1]);
      orb.userData.inspect = {
        title: `Candidate Site #${idx + 1} (${c.kind.replace('_', ' ')})`,
        items: [
          ['Normalized 1st-Step Gain', `${(c.gain * 100).toFixed(1)}%`],
          ['Site Elevation', `${c.enu[2].toFixed(1)} m`],
          ['ENU Position', `E ${c.enu[0]} m, N ${c.enu[1]} m`],
        ],
      };
      layerGroups.candidates.add(orb);
      staticPickables.push(orb);
    });

    const listEl = document.getElementById('sensor-list');
    listEl.innerHTML = allSensors
      .map((s) => {
        const isPreview = s.origin === 'preview';
        const palette = SENSOR_COLORS[s.type] || SENSOR_COLORS.radar;
        const off = disabledSensors.has(s.id);
        const tag = isPreview ? ' TEST' : s.origin === 'extra' ? ' +extra' : s.origin === 'optimized' ? ' moved' : s.locked ? ' 🔒' : '';
        return `
          <div class="sensor-item ${off ? 'off' : ''} ${isPreview ? 'preview-item' : ''}">
            <div class="sensor-left">
              <label>
                <input type="checkbox" data-sensor-toggle="${s.id}" ${off ? '' : 'checked'} />
                <span class="dot" style="background:${isPreview ? '#34d399' : palette.css}"></span>
                <span>${s.label}</span>
              </label>
            </div>
            <span class="sensor-meta">${(s.effectiveRangeM / 1000).toFixed(1)}km${tag}</span>
          </div>`;
      })
      .join('');

    listEl.querySelectorAll('input[data-sensor-toggle]').forEach((inp) => {
      inp.onchange = () => {
        const id = inp.dataset.sensorToggle;
        if (inp.checked) disabledSensors.delete(id);
        else disabledSensors.add(id);
        rebuildSceneOverlays();
        updateCoverageSurfaceAndMetrics();
        if (lastLosData) renderLosInspectorPanel(lastLosData);
      };
    });

    const causeBox = document.getElementById('blind-cause-box');
    const causes = cfg.metrics.blindCauses;
    if (causes) {
      const tot = Math.max(causes.totalBlindKm2, 0.001);
      const bPct = (causes.buildingBlockedKm2 / tot) * 100;
      const tPct = (causes.terrainBlockedKm2 / tot) * 100;
      const jKm2 = causes.jammedKm2 || 0;
      const jPct = (jKm2 / tot) * 100;
      const rPct = (causes.outOfRangeOrSectorKm2 / tot) * 100;
      causeBox.innerHTML = causes.totalBlindKm2 <= 0.001
        ? `<div class="good" style="font-size:11px;">0.00 km² blind area @ ${causes.aglM}m AGL — full coverage achieved.</div>`
        : `
          <div style="font-size:10.5px;color:var(--muted);">Total @ ${causes.aglM}m AGL: <strong style="color:#fff;">${causes.totalBlindKm2.toFixed(2)} km²</strong></div>
          <div class="cause-bar">
            <span style="width:${bPct}%;background:#fbbf24;" title="Buildings"></span>
            <span style="width:${tPct}%;background:#f87171;" title="Terrain"></span>
            ${jKm2 > 0.001 ? `<span style="width:${jPct}%;background:#f97316;" title="EW Jammed"></span>` : ''}
            <span style="width:${rPct}%;background:#94a3b8;" title="Range / Sector"></span>
          </div>
          <div class="cause-legend">
            <span><span class="dot" style="display:inline-block;background:#fbbf24;margin-right:3px;"></span>Buildings ${causes.buildingBlockedKm2.toFixed(2)} km²</span>
            <span><span class="dot" style="display:inline-block;background:#f87171;margin-right:3px;"></span>Terrain ${causes.terrainBlockedKm2.toFixed(2)} km²</span>
            ${jKm2 > 0.001 ? `<span><span class="dot" style="display:inline-block;background:#f97316;margin-right:3px;"></span>EW Jammed ${jKm2.toFixed(2)} km²</span>` : ''}
            <span><span class="dot" style="display:inline-block;background:#94a3b8;margin-right:3px;"></span>Range ${causes.outOfRangeOrSectorKm2.toFixed(2)} km²</span>
          </div>`;
    } else {
      causeBox.innerHTML = '<div style="font-size:10.5px;color:var(--muted);">Root-cause breakdown available on v0.2.0 runs.</div>';
    }

    const tbody = document.getElementById('layer-table-body');
    tbody.innerHTML = (cfg.metrics.layers || [])
      .map(
        (l) => `
          <tr data-agl="${l.aglM}">
            <td>${l.aglM} m</td>
            <td class="${l.coverageFraction >= 0.75 ? 'good' : l.coverageFraction >= 0.5 ? 'warn' : 'bad'}">${(l.coverageFraction * 100).toFixed(1)}%</td>
            <td>${l.priorityZoneCoverageFraction !== null ? (l.priorityZoneCoverageFraction * 100).toFixed(1) + '%' : '—'}</td>
            <td class="cyan">${(l.redundantFraction * 100).toFixed(1)}%</td>
            <td>${l.blindAreaKm2.toFixed(2)}</td>
          </tr>`
      )
      .join('');
    tbody.querySelectorAll('tr').forEach((tr) => {
      tr.onclick = () => {
        sliceAglM = Number(tr.dataset.agl);
        document.getElementById('agl-slider').value = String(sliceAglM);
        document.getElementById('agl-label').textContent = `${sliceAglM} m`;
        updateCoverageSurfaceAndMetrics();
        if (lastInspectedCell) void inspectCellAt(lastInspectedCell.east, lastInspectedCell.north);
      };
    });

    const cmpEl = document.getElementById('comparison-banner');
    if (replay.comparison && replay.configs.optimized) {
      const c = replay.comparison;
      const l0 = (c.layers || [])[0];
      const gainStr = c.objectiveGainPct !== null ? `${c.objectiveGainPct >= 0 ? '+' : ''}${c.objectiveGainPct.toFixed(1)}%` : '—';
      const covD = l0 ? `${l0.coverageDelta >= 0 ? '+' : ''}${(l0.coverageDelta * 100).toFixed(1)} pp` : '';
      const blindD = l0 ? `${l0.blindAreaDeltaKm2 >= 0 ? '+' : ''}${l0.blindAreaDeltaKm2.toFixed(2)} km²` : '';
      cmpEl.innerHTML = `
        <div class="delta-banner">
          <strong>Optimized vs Baseline:</strong> Objective <strong class="good">${gainStr}</strong> ·
          @ ${l0?.aglM ?? 30}m: <strong>${covD}</strong> coverage, <strong>${blindD}</strong> blind area
        </div>`;
    } else {
      cmpEl.innerHTML = '';
    }
  }

  async function inspectCellAt(eCoord, nCoord) {
    if (!decodedGrid || !replay) return;
    const { nx, ny, eastMin, northMin, cellM, ground, weights, valid } = decodedGrid;
    const fi = Math.max(0, Math.min(nx - 1, Math.floor((eCoord - eastMin) / cellM)));
    const fj = Math.max(0, Math.min(ny - 1, Math.floor((nCoord - northMin) / cellM)));
    const idx = fj * nx + fi;
    if (!valid[idx]) return;

    lastInspectedCell = { east: eCoord, north: nCoord };
    const { cfgName, sensors, floors } = getActiveSensorsAndFloors();

    let minFloor = 65535;
    const seeing = [];
    let bestSensor = 'None';
    sensors.forEach((s, k) => {
      const f = floors[k][idx];
      if (f < minFloor) {
        minFloor = f;
        bestSensor = s.label;
      }
      if (f <= sliceAglM) seeing.push(`${s.label} (≥${f}m)`);
    });

    const [lon, lat] = threeToLonLat(replay.viewerContext.origin, eCoord, ground[idx], -nCoord);
    showInspection(
      {
        title: `Terrain Cell Inspector (${cfgName})`,
        items: [
          ['Coordinates', `${lat.toFixed(5)}°, ${lon.toFixed(5)}°`],
          ['Ground Elevation', `${ground[idx].toFixed(1)} m ENU`],
          ['Min Detect Floor', minFloor >= 65535 ? 'MASKED (Never detected)' : `${minFloor} m AGL (${bestSensor})`],
          [`Status @ ${sliceAglM}m AGL`, seeing.length ? `COVERED (${seeing.length})` : 'BLIND SPOT'],
          ['Priority Weight', `${weights[idx].toFixed(1)}×`],
        ],
      },
      true
    );

    if (serviceMode) {
      try {
        const res = await apiFetch('/los-profile', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            environmentId: replay.environmentId,
            targetEast: eCoord,
            targetNorth: nCoord,
            targetAglM: sliceAglM,
            includeBuildings: replay.scenario?.include_buildings ?? true,
            rcsM2: replay.threat?.rcsM2 ?? 0.1,
            visibilityKm: replay.weather?.visibilityKm ?? 10.0,
            surfaceWindMps: replay.weather?.surfaceWindMps ?? 0.0,
            sensors: sensors.map((s) => ({
              id: s.id,
              label: s.label,
              type: s.type,
              enu: s.enu,
              mastAglM: s.mastAglM,
              maxRangeM: s.maxRangeM,
              minElevationDeg: s.minElevationDeg,
              clutterFloorAglM: s.clutterFloorAglM,
              sectorCenterDeg: s.sectorCenterDeg,
              sectorWidthDeg: s.sectorWidthDeg,
            })),
          }),
        });
        if (res.ok) {
          const losData = await res.json();
          const firstBlocked = (losData.rays || []).findIndex((r) => !r.clear && r.blockPointEnu);
          selectedLosRayIdx = seeing.length === 0 && firstBlocked >= 0 ? firstBlocked : 0;
          renderLosInspectorPanel(losData);
          return;
        }
      } catch (err) {
        console.warn('LOS profile fallback:', err);
      }
    }
    inspLosSec.style.display = 'none';
  }

  async function dropPreviewSensorAt(eCoord, nCoord) {
    if (!serviceMode || !replay || !decodedGrid) return;
    const stype = document.getElementById('drop-sensor-type').value;
    const mastAglM = Math.max(2, Math.min(100, Number(document.getElementById('drop-sensor-mast').value) || 15));
    statusEl.textContent = `Computing live ${stype.toUpperCase()} viewshed at (${Math.round(eCoord)}, ${Math.round(nCoord)})…`;
    try {
      const res = await apiFetch('/preview-sensor', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          environmentId: replay.environmentId,
          id: `preview-${stype}-${previewSensors.length + 1}`,
          label: `Test ${stype === 'eo_ir' ? 'EO/IR' : stype.toUpperCase()} ${previewSensors.length + 1}`,
          type: stype,
          east: eCoord,
          north: nCoord,
          mastAglM,
          gridCellM: decodedGrid.cellM,
          includeBuildings: replay.scenario?.include_buildings ?? true,
          rcsM2: replay.threat?.rcsM2 ?? 0.1,
          visibilityKm: replay.weather?.visibilityKm ?? 10.0,
          surfaceWindMps: replay.weather?.surfaceWindMps ?? 0.0,
        }),
      });
      const text = await res.text();
      let data = null;
      try { data = text ? JSON.parse(text) : null; } catch { /* non-JSON error */ }
      if (!res.ok) throw new Error(data?.error || `HTTP ${res.status} (${text || 'Not found'})`);
      const floor = decodeUint16(data.floorB64);
      if (floor.length !== decodedGrid.nx * decodedGrid.ny) {
        throw new Error('Preview grid dimensions mismatch.');
      }
      previewSensors.push({ sensor: data, floor });
      statusEl.textContent = `Live preview added: ${data.label} (${data.siteOnRoof ? 'Rooftop' : 'Ground'} ${data.siteUpM}m)`;
      rebuildSceneOverlays();
      updateCoverageSurfaceAndMetrics();
      if (lastInspectedCell) void inspectCellAt(lastInspectedCell.east, lastInspectedCell.north);
    } catch (err) {
      statusEl.textContent = `Preview failed: ${err.message}`;
    }
  }

  const raycaster = new THREE.Raycaster();
  const pointerDown = new THREE.Vector2();
  renderer.domElement.addEventListener('pointerdown', (e) => pointerDown.set(e.clientX, e.clientY));
  renderer.domElement.addEventListener('pointerup', (e) => {
    if (Math.hypot(e.clientX - pointerDown.x, e.clientY - pointerDown.y) > 6) return;
    if (!replay) return;
    const allPickables = [...staticPickables, ...routePickables];
    if (!allPickables.length) return;

    const rect = renderer.domElement.getBoundingClientRect();
    const ndc = new THREE.Vector2(
      ((e.clientX - rect.left) / rect.width) * 2 - 1,
      -((e.clientY - rect.top) / rect.height) * 2 + 1
    );
    raycaster.setFromCamera(ndc, camera);
    const hits = raycaster.intersectObjects(allPickables, false);
    if (!hits.length) return;

    const gridHit = hits.find((h) => h.object.visible && h.object.parent?.visible !== false && h.object.userData.isCoverageGrid);

    if (dropSensorMode && gridHit) {
      const localPt = worldRoot.worldToLocal(gridHit.point.clone());
      void dropPreviewSensorAt(localPt.x, -localPt.z);
      return;
    }

    const discrete = hits.find((h) => h.object.userData.inspect);
    if (discrete) {
      showInspection(discrete.object.userData.inspect, false);
      return;
    }

    if (gridHit) {
      const localPt = worldRoot.worldToLocal(gridHit.point.clone());
      void inspectCellAt(localPt.x, -localPt.z);
    }
  });

  function moveCameraPreset(preset) {
    if (!replay) return;
    const cfg = replay.configs[getActiveConfigName()];
    if (preset === 'overview') {
      animateCameraTo(sceneCenter, new THREE.Vector3(sceneSize * 0.55, sceneSize * 0.44, sceneSize * 0.62));
    } else if (preset === 'top') {
      animateCameraTo(sceneCenter, new THREE.Vector3(0, sceneSize * 0.92, 60));
    } else if (preset === 'asset') {
      const a = (replay.assets || [])[0];
      const p = a ? enuToThree(a.enu) : sceneCenter;
      animateCameraTo(p, new THREE.Vector3(750, 560, 820));
    } else if (preset === 'sensor') {
      const s = (cfg.sensors || [])[0];
      const p = s ? enuToThree(s.enu) : sceneCenter;
      animateCameraTo(p, new THREE.Vector3(520, 320, 560));
    } else if (preset === 'route') {
      const r = (replay.routes || [])[0];
      const p = r && r.samples.length ? enuToThree(r.samples[Math.floor(r.samples.length * 0.45)]) : sceneCenter;
      animateCameraTo(p, new THREE.Vector3(850, 480, 900));
    } else if (preset === 'oblique') {
      animateCameraTo(sceneCenter, new THREE.Vector3(-sceneSize * 0.62, sceneSize * 0.16, sceneSize * 0.48));
    }
  }

  async function loadReplayBundle(ep, { resetCamera = true } = {}) {
    stageEl.textContent = 'Fetching coverage replay data…';
    serviceMode = Boolean(ep.serviceMode);
    const newReplay = await (await fetch(ep.replayUrl, { headers: apiHeaders(), cache: 'no-store' })).json();
    replay = newReplay;

    await loadEnvironmentGeometry(newReplay, ep);

    const g = newReplay.grid;
    decodedGrid = {
      nx: g.nx,
      ny: g.ny,
      eastMin: g.eastMin,
      northMin: g.northMin,
      cellM: g.cellM,
      cellAreaKm2: (g.cellM * g.cellM) / 1e6,
      ground: decodeFloat32(g.groundB64),
      weights: decodeFloat32(g.weightsB64),
      valid: decodeBase64Bytes(g.validB64),
    };
    decodedFloors = { baseline: [], optimized: [] };
    ['baseline', 'optimized'].forEach((cfgName) => {
      const c = newReplay.configs[cfgName];
      if (!c) return;
      decodedFloors[cfgName] = c.sensors.map((s) => decodeUint16(s.floorB64));
    });

    disabledSensors.clear();
    previewSensors = [];
    disposeGroup(layerGroups.stealth);
    document.getElementById('stealth-result-box').style.display = 'none';
    document.getElementById('export-net-status').textContent = '';
    setDropMode(false);
    clearInspection();

    activeConfig = newReplay.configs.optimized ? 'optimized' : 'baseline';
    sliceAglM = (newReplay.layers && newReplay.layers[0]) ? newReplay.layers[0].aglM : 30;
    document.getElementById('agl-slider').value = String(sliceAglM);
    document.getElementById('agl-label').textContent = `${sliceAglM} m`;

    const aglPresetsEl = document.getElementById('agl-presets');
    aglPresetsEl.innerHTML = (newReplay.layers || [])
      .map((l) => `<button class="chip" data-agl-preset="${l.aglM}">${l.aglM}m</button>`)
      .join('');
    aglPresetsEl.querySelectorAll('button').forEach((btn) => {
      btn.onclick = () => {
        sliceAglM = Number(btn.dataset.aglPreset);
        document.getElementById('agl-slider').value = String(sliceAglM);
        document.getElementById('agl-label').textContent = `${sliceAglM} m`;
        updateCoverageSurfaceAndMetrics();
        if (lastInspectedCell) void inspectCellAt(lastInspectedCell.east, lastInspectedCell.north);
      };
    });

    document.getElementById('scenario-title').textContent = newReplay.scenario.title || 'Sensor Coverage Study';
    document.getElementById('env-badges').innerHTML =
      `<span class="badge">${newReplay.environment.buildingCount} buildings</span>` +
      `<span class="badge">${g.nx}×${g.ny} (${g.cellM}m)</span>` +
      `<span class="badge warn">${newReplay.derivation}</span>` +
      `<span class="badge good">${newReplay.engineVersion}</span>`;
    statusEl.textContent = `Manifest ${newReplay.environment.manifestSha256.slice(0, 12)}… · Computed in ${newReplay.elapsedS}s`;

    document.getElementById('model-info').innerHTML = `
      <div>Target: <strong style="color:#fff;">${newReplay.threat.label}</strong> (RCS ${newReplay.threat.rcsM2} m²)</div>
      <div>Visibility: <strong style="color:#fff;">${newReplay.weather.visibilityKm} km</strong> · Wind: <strong style="color:#fff;">${newReplay.weather.surfaceWindMps} m/s</strong></div>
      <div style="font-size:10px;margin-top:2px;">Wind source: ${newReplay.weather.windSource}</div>
      <div style="font-size:10px;margin-top:2px;">Obstruction raster: ${newReplay.obstructionGrid.cellM} m (${newReplay.obstructionGrid.buildingCells.toLocaleString()} roof cells) · 4/3-earth refraction</div>
    `;

    const optEl = document.getElementById('optimizer-summary');
    if (newReplay.optimizer?.enabled) {
      const steps = (newReplay.optimizer.trace || [])
        .map((t) => `<div class="insp-kv"><span>${t.phase} · ${t.sensorId}</span><strong>${t.candidateKind} → ${t.objective.toFixed(4)}</strong></div>`)
        .join('');
      optEl.innerHTML = `
        <div style="margin-bottom:5px;">
          ${newReplay.optimizer.method} · <strong>${newReplay.optimizer.evaluations}</strong> evals in ${newReplay.optimizer.elapsedS}s
          ${newReplay.optimizer.note ? `<div class="warn">${newReplay.optimizer.note}</div>` : ''}
        </div>
        ${steps}
      `;
    } else {
      optEl.innerHTML = '<div>Optimizer disabled for this run (baseline evaluation only).</div>';
    }

    const switcher = document.getElementById('run-switcher');
    if (ep.serviceMode && ep.runs.length) {
      switcher.style.display = 'inline-block';
      switcher.innerHTML = ep.runs
        .filter((r) => r.status === 'completed')
        .map((r) => `<option value="${r.jobId}" ${r.jobId === ep.jobId ? 'selected' : ''}>${compactRunTitle(r.title)} (${r.jobId.slice(-6)})</option>`)
        .join('');
    } else {
      switcher.style.display = 'none';
    }

    const draft = newReplay.sourceDraft || {};
    document.getElementById('wi-rcs').value = String(draft.threat?.rcsM2 ?? newReplay.threat.rcsM2 ?? 0.1);
    document.getElementById('wi-vis').value = String(draft.weather?.visibilityKm ?? newReplay.weather.visibilityKm ?? 10);
    document.getElementById('wi-wind').value = String(draft.weather?.surfaceWindMps ?? newReplay.weather.surfaceWindMps ?? 5);
    document.getElementById('wi-sep').value = String(draft.optimizer?.minSeparationM ?? newReplay.optimizer?.minSeparationM ?? 600);
    document.getElementById('wi-extra').value = String(draft.optimizer?.extraSensors ?? newReplay.optimizer?.extraSensors ?? 0);
    document.getElementById('wi-extra-type').value = draft.optimizer?.extraSensorType || 'radar';
    document.getElementById('wi-buildings').checked = draft.includeBuildings ?? newReplay.scenario.include_buildings ?? true;
    document.getElementById('wi-rooftops').checked = draft.optimizer?.allowRooftops ?? true;
    document.getElementById('wi-opt-enabled').checked = draft.optimizer?.enabled ?? true;
    document.getElementById('wi-jammer').checked = Boolean((draft.jammers || newReplay.jammers || []).length);
    document.getElementById('btn-whatif-run').disabled = !ep.serviceMode;
    document.getElementById('btn-drop-sensor').disabled = !ep.serviceMode;
    document.getElementById('btn-find-stealth').disabled = !ep.serviceMode;
    document.getElementById('btn-export-network').disabled = !ep.serviceMode;

    syncConfigAndModeButtons();
    buildCoverageMeshShell();
    rebuildSceneOverlays();
    updateCoverageSurfaceAndMetrics();
    applyZScale(zScale);

    if (resetCamera) moveCameraPreset('overview');
    overlayEl.classList.add('hidden');
  }

  function syncConfigAndModeButtons() {
    const hasOpt = Boolean(replay?.configs?.optimized);
    document.querySelectorAll('#config-buttons button').forEach((btn) => {
      const c = btn.dataset.cfg;
      btn.disabled = c === 'optimized' && !hasOpt;
      btn.classList.toggle('active', c === activeConfig);
      btn.classList.toggle('good-active', c === 'optimized' && activeConfig === 'optimized');
    });
    document.querySelectorAll('#mode-buttons button').forEach((btn) => {
      const m = btn.dataset.mode;
      btn.disabled = m === 'diff' && !hasOpt;
      btn.classList.toggle('active', m === displayMode);
    });
  }

  async function submitAndReloadDraft(draftPayload, statusTarget) {
    const res = await apiFetch('/runs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(draftPayload),
    });
    const created = await res.json();
    if (!res.ok) throw new Error(created.error || `HTTP ${res.status}`);
    for (let i = 0; i < 360; i++) {
      const poll = await (await apiFetch(`/runs/${encodeURIComponent(created.jobId)}`, { cache: 'no-store' })).json();
      if (statusTarget) statusTarget.textContent = poll.stage || 'Running…';
      if (poll.status === 'completed') {
        const url = new URL(window.location.href);
        url.searchParams.set('run', created.jobId);
        window.history.replaceState({}, '', url.toString());
        const ep = await resolveEndpoints(stageEl);
        await loadReplayBundle(ep, { resetCamera: false });
        if (statusTarget) statusTarget.textContent = `Completed (${created.jobId.slice(-6)}) in ${replay.elapsedS}s`;
        return;
      }
      if (poll.status === 'failed' || poll.status === 'cancelled') throw new Error(poll.error || poll.stage || 'Run failed.');
      await new Promise((r) => setTimeout(r, 250));
    }
    throw new Error('Timed out waiting for coverage run.');
  }

  document.querySelectorAll('.collapse-btn[data-target]').forEach((btn) => {
    btn.onclick = () => {
      const panel = document.getElementById(btn.dataset.target);
      panel.classList.toggle('collapsed');
      btn.textContent = panel.classList.contains('collapsed') ? '▸' : '▾';
    };
  });

  const toggleHud = () => document.body.classList.toggle('hud-hidden');
  document.getElementById('btn-hud-hide').onclick = toggleHud;
  document.getElementById('hud-restore-pill').onclick = toggleHud;

  const helpCard = document.getElementById('help-card');
  const toggleHelp = () => helpCard.classList.toggle('visible');
  document.getElementById('btn-help').onclick = toggleHelp;
  document.getElementById('help-close').onclick = () => helpCard.classList.remove('visible');

  document.querySelectorAll('.tab-btn').forEach((btn) => {
    btn.onclick = () => {
      document.querySelectorAll('.tab-btn').forEach((b) => b.classList.toggle('active', b === btn));
      document.querySelectorAll('.tab-pane').forEach((p) => p.classList.toggle('active', p.id === `tab-${btn.dataset.tab}`));
    };
  });

  document.querySelectorAll('#config-buttons button').forEach((btn) => {
    btn.onclick = () => {
      if (btn.disabled) return;
      activeConfig = btn.dataset.cfg;
      disabledSensors.clear();
      syncConfigAndModeButtons();
      rebuildSceneOverlays();
      updateCoverageSurfaceAndMetrics();
      if (lastInspectedCell) void inspectCellAt(lastInspectedCell.east, lastInspectedCell.north);
    };
  });

  document.querySelectorAll('#mode-buttons button').forEach((btn) => {
    btn.onclick = () => {
      if (btn.disabled) return;
      displayMode = btn.dataset.mode;
      syncConfigAndModeButtons();
      updateCoverageSurfaceAndMetrics();
    };
  });

  document.getElementById('agl-slider').oninput = (e) => {
    sliceAglM = Number(e.target.value);
    document.getElementById('agl-label').textContent = `${sliceAglM} m`;
    updateCoverageSurfaceAndMetrics();
  };
  document.getElementById('agl-slider').onchange = () => {
    if (lastInspectedCell) void inspectCellAt(lastInspectedCell.east, lastInspectedCell.north);
  };

  document.getElementById('tog-float-agl').onchange = (e) => {
    floatAtAgl = e.target.checked;
    updateCoverageSurfaceAndMetrics();
  };

  document.getElementById('opacity-slider').oninput = (e) => {
    meshOpacity = Number(e.target.value);
    document.getElementById('opacity-label').textContent = `${Math.round(meshOpacity * 100)}%`;
    if (coverageDrapeMat) coverageDrapeMat.opacity = meshOpacity;
    if (coverageFloatMat) coverageFloatMat.opacity = meshOpacity;
  };

  document.getElementById('btn-drop-sensor').onclick = () => setDropMode(!dropSensorMode);
  document.getElementById('btn-exit-drop').onclick = () => setDropMode(false);
  document.getElementById('btn-clear-preview').onclick = () => {
    previewSensors = [];
    rebuildSceneOverlays();
    updateCoverageSurfaceAndMetrics();
    if (lastInspectedCell) void inspectCellAt(lastInspectedCell.east, lastInspectedCell.north);
  };
  document.getElementById('btn-commit-preview').onclick = async () => {
    if (!previewSensors.length || !replay) return;
    const btn = document.getElementById('btn-commit-preview');
    btn.disabled = true;
    statusEl.textContent = 'Committing preview sensors & re-running full coverage study…';
    try {
      const cfg = replay.configs[getActiveConfigName()];
      const baseDraft = replay.sourceDraft ? JSON.parse(JSON.stringify(replay.sourceDraft)) : { environmentId: replay.environmentId };
      baseDraft.sensors = [...cfg.sensors, ...previewSensors.map((p) => p.sensor)].map((s) => ({
        id: s.id.replace(/^preview-/, 'custom-'),
        type: s.type,
        label: s.label.replace(/^Test /, 'Custom '),
        lon: s.lon,
        lat: s.lat,
        mastAglM: s.mastAglM,
        maxRangeM: s.maxRangeM,
        minElevationDeg: s.minElevationDeg,
        clutterFloorAglM: s.clutterFloorAglM,
        sectorCenterDeg: s.sectorCenterDeg,
        sectorWidthDeg: s.sectorWidthDeg,
        locked: s.origin === 'preview' ? true : Boolean(s.locked),
      }));
      baseDraft.optimizer = { ...(baseDraft.optimizer || {}), extraSensors: 0 };
      await submitAndReloadDraft(baseDraft, statusEl);
    } catch (err) {
      statusEl.textContent = `Commit error: ${err.message}`;
    } finally {
      btn.disabled = false;
    }
  };

  document.getElementById('btn-all-sensors').onclick = () => {
    disabledSensors.clear();
    rebuildSceneOverlays();
    updateCoverageSurfaceAndMetrics();
    if (lastLosData) renderLosInspectorPanel(lastLosData);
  };

  // v0.3 A* Adversarial Least-Detection Stealth Path Finder
  document.getElementById('btn-find-stealth').onclick = async () => {
    if (!serviceMode || !replay || !decodedGrid) return;
    const btn = document.getElementById('btn-find-stealth');
    const box = document.getElementById('stealth-result-box');
    btn.disabled = true;
    box.style.display = 'block';
    box.innerHTML = 'Running A* stealth solver over coverage raster…';
    try {
      const { sensors } = getActiveSensorsAndFloors();
      const asset = (replay.assets || [])[0];
      const res = await apiFetch('/adversarial-route', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          environmentId: replay.environmentId,
          aglM: sliceAglM,
          speedMps: 65,
          gridCellM: decodedGrid.cellM,
          includeBuildings: replay.scenario?.include_buildings ?? true,
          rcsM2: replay.threat?.rcsM2 ?? 0.1,
          visibilityKm: replay.weather?.visibilityKm ?? 10.0,
          surfaceWindMps: replay.weather?.surfaceWindMps ?? 0.0,
          assetEnu: asset ? [asset.enu[0], asset.enu[1]] : undefined,
          jammers: replay.jammers || [],
          sensors: sensors.map((s) => ({
            id: s.id,
            label: s.label,
            type: s.type,
            enu: s.enu,
            mastAglM: s.mastAglM,
            maxRangeM: s.maxRangeM,
            minElevationDeg: s.minElevationDeg,
            clutterFloorAglM: s.clutterFloorAglM,
            sectorCenterDeg: s.sectorCenterDeg,
            sectorWidthDeg: s.sectorWidthDeg,
          })),
        }),
      });
      const route = await res.json();
      if (!res.ok) throw new Error(route.error || `HTTP ${res.status}`);
      disposeGroup(layerGroups.stealth);
      const pts3 = (route.samples || []).map((p) => enuToThree([p[0], p[1], p[2] + 6]));
      if (pts3.length >= 2) {
        const tubeCurve = new THREE.CatmullRomCurve3(pts3);
        const tube = new THREE.Mesh(
          new THREE.TubeGeometry(tubeCurve, Math.min(180, pts3.length * 2), 14, 6, false),
          new THREE.MeshBasicMaterial({ color: 0xc084fc, transparent: true, opacity: 0.92 })
        );
        layerGroups.stealth.add(tube);
        const startLbl = createLabelSprite(`🕵 Stealth Ingress (${Math.round(route.stealthFraction * 100)}% blind)`, '#c084fc', 420);
        startLbl.position.copy(pts3[0]).add(new THREE.Vector3(0, 95, 0));
        layerGroups.stealth.add(startLbl);
      }
      if (route.firstDetectionEnu) {
        const dp = enuToThree(route.firstDetectionEnu);
        const m = new THREE.Mesh(new THREE.OctahedronGeometry(30, 0), new THREE.MeshBasicMaterial({ color: 0xf472b6 }));
        m.position.copy(dp);
        layerGroups.stealth.add(m);
      }
      box.innerHTML =
        `<strong style="color:#e9d5ff;">Adversarial Corridor @ ${route.aglM}m AGL</strong><br>` +
        `Unseen: <strong style="color:#f472b6;">${(route.stealthFraction * 100).toFixed(1)}%</strong> · ` +
        `Warning: <strong style="color:${route.warningTimeS < 15 ? '#f87171' : '#34d399'};">${route.warningTimeS}s</strong> · ` +
        `Len: ${(route.lengthM / 1000).toFixed(2)} km`;
      statusEl.textContent = `Stealth corridor discovered: ${(route.stealthFraction * 100).toFixed(0)}% blind, ${route.warningTimeS}s warning.`;
    } catch (err) {
      box.innerHTML = `<span style="color:#f87171;">Stealth solver failed: ${err.message}</span>`;
    } finally {
      btn.disabled = false;
    }
  };

  // v0.3 Cross-Simulation Sensor Network Export
  document.getElementById('btn-export-network').onclick = async () => {
    if (!serviceMode || !replay) return;
    const st = document.getElementById('export-net-status');
    st.textContent = 'Exporting active sensor layout…';
    try {
      const { cfgName, sensors } = getActiveSensorsAndFloors();
      const res = await apiFetch('/export-network', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          environmentId: replay.environmentId,
          jobId: replay.jobId,
          name: `${compactRunTitle(replay.scenario?.title || replay.environmentId)} (${cfgName})`,
          sensors,
        }),
      });
      const saved = await res.json();
      if (!res.ok) throw new Error(saved.error || `HTTP ${res.status}`);
      st.textContent = `✓ Saved ${saved.layoutId} (${saved.sensorCount} sensors) — ready in Intercept & Salvo!`;
    } catch (err) {
      st.textContent = `Export failed: ${err.message}`;
    }
  };

  document.getElementById('btn-wi-cams-dust').onclick = () => {
    document.getElementById('wi-vis').value = '1.5';
    document.getElementById('wi-wind').value = '14';
    document.getElementById('wi-status').textContent = 'Applied CAMS dust storm preset (1.5 km vis, 14 m/s wind). Click Re-Run.';
  };

  const layerToggleMap = {
    'tog-coverage': layerGroups.coverage,
    'tog-imagery': layerGroups.imagery,
    'tog-buildings': layerGroups.buildings,
    'tog-roads': layerGroups.roads,
    'tog-sensors': layerGroups.sensors,
    'tog-labels': layerGroups.labels,
    'tog-moves': layerGroups.moves,
    'tog-routes': layerGroups.routes,
    'tog-assets': layerGroups.assets,
    'tog-candidates': layerGroups.candidates,
  };
  Object.entries(layerToggleMap).forEach(([id, grp]) => {
    const el = document.getElementById(id);
    if (el) el.onchange = () => { grp.visible = el.checked; };
  });

  document.getElementById('zscale-slider').oninput = (e) => applyZScale(parseFloat(e.target.value));
  document.querySelectorAll('#dock button[data-cam]').forEach((btn) => {
    btn.onclick = () => moveCameraPreset(btn.dataset.cam);
  });

  const probePlayBtn = document.getElementById('btn-probe-play');
  const probeSlider = document.getElementById('probe-slider');
  const toggleProbePlay = () => {
    probePlaying = !probePlaying;
    if (probePlaying && probeT >= 0.995) probeT = 0;
    probePlayBtn.textContent = probePlaying ? '⏸ Pause' : '▶ Fly Ingress';
    probePlayBtn.classList.toggle('active', probePlaying);
    updateProbePositions();
  };
  probePlayBtn.onclick = toggleProbePlay;
  probeSlider.oninput = () => {
    probePlaying = false;
    probePlayBtn.textContent = '▶ Fly Ingress';
    probePlayBtn.classList.remove('active');
    probeT = Number(probeSlider.value);
    updateProbePositions();
  };

  document.getElementById('run-switcher').onchange = async (e) => {
    const newId = e.target.value;
    if (!newId) return;
    const url = new URL(window.location.href);
    url.searchParams.set('run', newId);
    window.history.replaceState({}, '', url.toString());
    overlayEl.classList.remove('hidden');
    const ep = await resolveEndpoints(stageEl);
    await loadReplayBundle(ep, { resetCamera: false });
  };

  document.getElementById('btn-whatif-run').onclick = async () => {
    if (!replay) return;
    const status = document.getElementById('wi-status');
    const btn = document.getElementById('btn-whatif-run');
    btn.disabled = true;
    status.textContent = 'Submitting What-If coverage study…';
    try {
      let baseDraft = replay.sourceDraft ? JSON.parse(JSON.stringify(replay.sourceDraft)) : null;
      if (!baseDraft) {
        const envRes = await apiFetch('/environments');
        const envs = (await envRes.json()).environments || [];
        const match = envs.find((e) => e.id === replay.environmentId) || envs[0];
        baseDraft = match ? JSON.parse(JSON.stringify(match.defaults)) : null;
      }
      if (!baseDraft) throw new Error('Unable to reconstruct draft payload.');

      baseDraft.threat = { ...(baseDraft.threat || {}), rcsM2: Number(document.getElementById('wi-rcs').value) };
      baseDraft.weather = {
        ...(baseDraft.weather || {}),
        visibilityKm: Number(document.getElementById('wi-vis').value),
        useEra5Wind: false,
        surfaceWindMps: Number(document.getElementById('wi-wind').value),
      };
      baseDraft.includeBuildings = document.getElementById('wi-buildings').checked;
      baseDraft.optimizer = {
        ...(baseDraft.optimizer || {}),
        enabled: document.getElementById('wi-opt-enabled').checked,
        extraSensors: Number(document.getElementById('wi-extra').value),
        extraSensorType: document.getElementById('wi-extra-type').value,
        minSeparationM: Number(document.getElementById('wi-sep').value),
        allowRooftops: document.getElementById('wi-rooftops').checked,
      };
      if (document.getElementById('wi-jammer').checked) {
        if (!baseDraft.jammers || !baseDraft.jammers.length) {
          const [w, s, e, n] = replay.environment?.boundsWgs84 || [46.7, 24.68, 46.8, 24.75];
          baseDraft.jammers = [
            {
              id: 'jammer-1',
              label: 'Stand-Off EW Jammer',
              lon: Number((w + (e - w) * 0.16).toFixed(6)),
              lat: Number((s + (n - s) * 0.84).toFixed(6)),
              aglM: 180,
              erpRatio: 16,
              beamWidthDeg: 36,
            },
          ];
        }
      } else {
        baseDraft.jammers = [];
      }
      baseDraft.mapLayerAglM = sliceAglM;

      await submitAndReloadDraft(baseDraft, status);
    } catch (err) {
      status.textContent = `Error: ${err.message}`;
    } finally {
      btn.disabled = false;
    }
  };

  window.addEventListener('keydown', (e) => {
    const tag = e.target?.tagName;
    if (tag === 'INPUT' || tag === 'SELECT' || tag === 'TEXTAREA') return;
    const k = e.key.toLowerCase();
    if (e.key === '?') { toggleHelp(); return; }
    if (e.key === 'Escape') {
      setDropMode(false);
      clearInspection();
      helpCard.classList.remove('visible');
      return;
    }
    if ('wasdqe'.includes(k)) keysHeld.add(k);
    if (k === 'h') toggleHud();
    if (k === 'p') setDropMode(!dropSensorMode);
    if (k === 'r') moveCameraPreset('overview');
    if (k === ' ') { e.preventDefault(); toggleProbePlay(); }
    if (k === 'c' && replay?.configs?.optimized) {
      activeConfig = activeConfig === 'optimized' ? 'baseline' : 'optimized';
      disabledSensors.clear();
      syncConfigAndModeButtons();
      rebuildSceneOverlays();
      updateCoverageSurfaceAndMetrics();
      if (lastInspectedCell) void inspectCellAt(lastInspectedCell.east, lastInspectedCell.north);
    }
    if (k === 'v') {
      floatAtAgl = !floatAtAgl;
      document.getElementById('tog-float-agl').checked = floatAtAgl;
      updateCoverageSurfaceAndMetrics();
    }
    if (k === '1') moveCameraPreset('overview');
    if (k === '2') moveCameraPreset('top');
    if (k === '3') moveCameraPreset('asset');
    if (k === '4') moveCameraPreset('sensor');
    if (k === '5') moveCameraPreset('route');
    if (k === '6') moveCameraPreset('oblique');
  });
  window.addEventListener('keyup', (e) => keysHeld.delete(e.key.toLowerCase()));

  window.addEventListener('resize', () => {
    camera.aspect = window.innerWidth / window.innerHeight;
    camera.updateProjectionMatrix();
    renderer.setSize(window.innerWidth, window.innerHeight);
  });

  const clock = new THREE.Clock();
  function animate() {
    requestAnimationFrame(animate);
    const dt = Math.min(clock.getDelta(), 0.1);

    if (camTransition) {
      camTransition.t += dt / camTransition.dur;
      const u = Math.min(1, camTransition.t);
      const ease = 1 - Math.pow(1 - u, 3);
      camera.position.lerpVectors(camTransition.startPos, camTransition.endPos, ease);
      controls.target.lerpVectors(camTransition.startTarget, camTransition.endTarget, ease);
      if (u >= 1) camTransition = null;
    }

    if (keysHeld.size > 0) {
      camTransition = null;
      const flySpeed = Math.max(350, camera.position.distanceTo(controls.target) * 0.45) * dt;
      const forward = new THREE.Vector3();
      camera.getWorldDirection(forward);
      forward.y = 0;
      if (forward.lengthSq() > 1e-6) forward.normalize();
      const right = new THREE.Vector3().crossVectors(forward, new THREE.Vector3(0, 1, 0)).normalize();
      const delta = new THREE.Vector3();
      if (keysHeld.has('w')) delta.addScaledVector(forward, flySpeed);
      if (keysHeld.has('s')) delta.addScaledVector(forward, -flySpeed);
      if (keysHeld.has('d')) delta.addScaledVector(right, flySpeed);
      if (keysHeld.has('a')) delta.addScaledVector(right, -flySpeed);
      if (keysHeld.has('e')) delta.y += flySpeed;
      if (keysHeld.has('q')) delta.y -= flySpeed;
      camera.position.add(delta);
      controls.target.add(delta);
    }

    for (const r of sensorRotors) r.rotation.y += dt * 1.4;

    if (probePlaying) {
      probeT += dt * 0.085;
      if (probeT >= 1.0) {
        probeT = 1.0;
        probePlaying = false;
        probePlayBtn.textContent = '▶ Fly Ingress';
        probePlayBtn.classList.remove('active');
      }
      probeSlider.value = String(probeT);
      updateProbePositions();
    }

    controls.update();
    renderer.render(scene, camera);
  }
  animate();

  try {
    const ep = await resolveEndpoints(stageEl);
    await loadReplayBundle(ep, { resetCamera: true });
  } catch (err) {
    stageEl.textContent = `Error: ${err.message}`;
    stageEl.style.color = '#f87171';
    console.error(err);
  }
}

main();
