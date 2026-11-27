import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';

const API_BASE = (() => {
  const p = window.location.pathname;
  if (p.startsWith('/sim-viewers/salvo')) return '/api/v1/sim-platform/salvo';
  if (p.startsWith('/viewer/salvo')) return '/api/salvo';
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

const THREAT_COLORS = {
  ballistic_missile: 0xff3b5c,
  cruise_missile: 0xfb923c,
  one_way_uav: 0xf472b6,
  decoy: 0xfde047,
};

const TIER_COLORS = {
  tier1_sam: 0x38bdf8,
  tier2_shorad: 0x34d399,
  tier3_point_defense: 0xfbbf24,
};

const POLICY_LABELS = {
  greedy_earliest: 'Greedy Baseline',
  combinatorial_wta: 'Combinatorial WTA',
  ai_policy: 'AI RL Policy',
};

function interpSampleList(samples, t) {
  if (!samples || samples.length === 0) return null;
  if (t < samples[0][0] || t > samples[samples.length - 1][0]) return null;
  let lo = 0, hi = samples.length - 1;
  while (lo + 1 < hi) {
    const mid = (lo + hi) >> 1;
    if (samples[mid][0] <= t) lo = mid;
    else hi = mid;
  }
  const a = samples[lo], b = samples[hi];
  const f = b[0] > a[0] ? Math.max(0, Math.min(1, (t - a[0]) / (b[0] - a[0]))) : 0;
  const p = new THREE.Vector3(
    a[1] + f * (b[1] - a[1]),
    a[3] + f * (b[3] - a[3]),
    -(a[2] + f * (b[2] - a[2]))
  );
  const nxt = new THREE.Vector3(b[1], b[3], -b[2]);
  return { pos: p, next: nxt };
}

async function resolveEndpoint(stageEl) {
  const params = new URLSearchParams(window.location.search);
  const mapOrigin = params.get('mapOrigin');
  if (mapOrigin) document.getElementById('back-to-map').href = `${mapOrigin.replace(/\/$/, '')}/map`;
  else if (window.location.pathname.startsWith('/sim-viewers/')) document.getElementById('back-to-map').href = '/map';
  const runParam = params.get('run');

  const runsRes = await apiFetch('/runs', { cache: 'no-store' });
  const runs = runsRes.ok ? ((await runsRes.json()).runs || []) : [];
  let jobId = runParam;
  if (!jobId) {
    const done = runs.find((r) => r.status === 'completed');
    if (done) {
      jobId = done.jobId;
    } else {
      stageEl.textContent = 'No completed salvo yet — launching default layered defense salvo…';
      const envs = ((await (await apiFetch('/environments', { cache: 'no-store' })).json()).environments || []);
      if (!envs.length) throw new Error('No verified environments found.');
      const cr = await apiFetch('/runs', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(envs[0].defaults),
      });
      const created = await cr.json();
      jobId = created.jobId;
    }
  }

  let job = null;
  for (let i = 0; i < 360; i++) {
    const res = await apiFetch(`/runs/${encodeURIComponent(jobId)}`, { cache: 'no-store' });
    if (res.status === 404) {
      stageEl.textContent = 'Waiting for salvo run registration…';
      await new Promise((r) => setTimeout(r, 250));
      continue;
    }
    if (!res.ok) throw new Error(`Salvo job HTTP ${res.status}`);
    job = await res.json();
    if (job.status === 'completed') break;
    if (job.status === 'failed' || job.status === 'interrupted' || job.status === 'cancelled') throw new Error(job.error || job.stage || 'Salvo failed.');
    stageEl.textContent = job.stage || 'Running salvo simulation…';
    await new Promise((r) => setTimeout(r, 250));
  }
  if (!job || job.status !== 'completed') throw new Error('Timed out waiting for salvo run.');
  const freshRuns = ((await (await apiFetch('/runs', { cache: 'no-store' })).json()).runs || runs);
  return {
    jobId,
    runs: freshRuns,
    hasGlb: Boolean(job.hasGlb),
    hasImagery: Boolean(job.hasImagery),
    replayUrl: `${API_BASE}/runs/${encodeURIComponent(jobId)}/replay.json`,
    glbUrl: `${API_BASE}/runs/${encodeURIComponent(jobId)}/terrain.glb`,
    imageryUrl: `${API_BASE}/runs/${encodeURIComponent(jobId)}/imagery.json`,
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
  scene.fog = new THREE.FogExp2(0x071018, 0.00003);

  const camera = new THREE.PerspectiveCamera(48, window.innerWidth / window.innerHeight, 2, 95000);
  const controls = new OrbitControls(camera, renderer.domElement);
  controls.enableDamping = true;
  controls.dampingFactor = 0.08;
  controls.maxPolarAngle = Math.PI * 0.495;

  scene.add(new THREE.HemisphereLight(0xd9ecff, 0x3b352a, 1.5));
  const sun = new THREE.DirectionalLight(0xfff4df, 1.75);
  sun.position.set(2800, 6500, 2200);
  scene.add(sun);

  const groups = {
    terrain: new THREE.Group(),
    imagery: new THREE.Group(),
    buildings: new THREE.Group(),
    roads: new THREE.Group(),
    sites: new THREE.Group(),
    rings: new THREE.Group(),
    civil: new THREE.Group(),
    truth: new THREE.Group(),
    obs: new THREE.Group(),
    debris: new THREE.Group(),
  };
  Object.values(groups).forEach((g) => scene.add(g));

  let replay = null;
  let loadedManifest = null;
  let activePolicy = 'combinatorial_wta';
  let sceneCenter = new THREE.Vector3();
  let sceneSize = 6000;
  let simTime = 0;
  let maxTime = 115;
  let playing = true;
  let playSpeed = 1.0;

  let threatActors = [];
  let intActors = [];
  let burstActors = [];
  let debrisActors = [];
  let civilActors = [];
  let obsDotPool = [];
  const keysHeld = new Set();

  async function loadEnvGeometry(rep, ep) {
    const sha = rep.environment.manifestSha256;
    if (loadedManifest === sha) return;
    loadedManifest = sha;
    disposeGroup(groups.terrain);
    disposeGroup(groups.imagery);
    disposeGroup(groups.buildings);
    disposeGroup(groups.roads);

    let terrainObj = null;
    if (ep.hasGlb) {
      const gltf = await loadGlbAuth(ep.glbUrl);
      terrainObj = gltf.scene;
      terrainObj.traverse((o) => {
        if (o.isMesh) o.material = new THREE.MeshStandardMaterial({ color: 0x6f6858, roughness: 0.92, metalness: 0.05 });
      });
      groups.terrain.add(terrainObj);
    } else {
      const g = rep.viewerContext.fallbackTerrainGrid;
      const geom = new THREE.BufferGeometry();
      const verts = new Float32Array(g.nx * g.ny * 3);
      let ptr = 0;
      for (let j = 0; j < g.ny; j++) {
        const n = g.northMin + (j / (g.ny - 1)) * (g.northMax - g.northMin);
        for (let i = 0; i < g.nx; i++) {
          const e = g.eastMin + (i / (g.nx - 1)) * (g.eastMax - g.eastMin);
          verts[ptr++] = e; verts[ptr++] = g.up[j * g.nx + i]; verts[ptr++] = -n;
        }
      }
      const idx = [];
      for (let j = 0; j < g.ny - 1; j++) {
        for (let i = 0; i < g.nx - 1; i++) {
          const a = j * g.nx + i, b = a + 1, c = a + g.nx, d = c + 1;
          idx.push(a, c, b, b, c, d);
        }
      }
      geom.setAttribute('position', new THREE.BufferAttribute(verts, 3));
      geom.setIndex(idx);
      geom.computeVertexNormals();
      terrainObj = new THREE.Mesh(geom, new THREE.MeshStandardMaterial({ color: 0x5c685e, roughness: 0.9, side: THREE.DoubleSide }));
      groups.terrain.add(terrainObj);
    }

    const box = new THREE.Box3().setFromObject(groups.terrain);
    sceneCenter = box.getCenter(new THREE.Vector3());
    sceneSize = Math.max(2000, box.getSize(new THREE.Vector3()).length());

    if (ep.hasImagery && terrainObj) {
      try {
        const rawImg = await (await fetch(ep.imageryUrl, { headers: apiHeaders() })).json();
        const img = rawImg.data || rawImg;
        if (img?.texture && img?.bounds) {
          const rawBytes = Uint8Array.from(atob(img.texture), (c) => c.charCodeAt(0));
          const bitmap = await createImageBitmap(new Blob([rawBytes], { type: 'image/png' }), { imageOrientation: 'flipY' });
          const tex = new THREE.Texture(bitmap);
          tex.colorSpace = THREE.SRGBColorSpace;
          tex.flipY = false;
          tex.needsUpdate = true;
          const [west, south, east, north] = img.bounds;
          const origin = rep.viewerContext.origin;
          const mat = new THREE.MeshBasicMaterial({ map: tex, alphaTest: 0.4, polygonOffset: true, polygonOffsetFactor: -1, polygonOffsetUnits: -1 });
          terrainObj.traverse((o) => {
            if (!o.isMesh) return;
            const cg = o.geometry.clone();
            const pos = cg.getAttribute('position');
            const uv = new Float32Array(pos.count * 2);
            for (let i = 0; i < pos.count; i++) {
              const [lon, lat] = threeToLonLat(origin, pos.getX(i), pos.getY(i), pos.getZ(i));
              uv[2 * i] = (lon - west) / (east - west);
              uv[2 * i + 1] = 1 - (north - lat) / (north - south);
            }
            cg.setAttribute('uv', new THREE.BufferAttribute(uv, 2));
            groups.imagery.add(new THREE.Mesh(cg, mat));
          });
        }
      } catch {
        /* ignore imagery errors */
      }
    }

    const bldgs = rep.viewerContext.buildings || [];
    const pos = [], norm = [];
    for (const b of bldgs) {
      const ring = b.ring, y0 = b.baseUp, y1 = b.baseUp + b.heightM;
      for (let i = 0; i < ring.length - 1; i++) {
        const [e0, n0] = ring[i], [e1, n1] = ring[i + 1];
        const dx = e1 - e0, dz = -(n1 - n0), l = Math.hypot(dx, dz) || 1;
        pos.push(e0, y0, -n0, e1, y0, -n1, e1, y1, -n1, e0, y0, -n0, e1, y1, -n1, e0, y1, -n0);
        for (let k = 0; k < 6; k++) norm.push(-dz / l, 0, dx / l);
      }
    }
    if (pos.length) {
      const bg = new THREE.BufferGeometry();
      bg.setAttribute('position', new THREE.Float32BufferAttribute(pos, 3));
      bg.setAttribute('normal', new THREE.Float32BufferAttribute(norm, 3));
      groups.buildings.add(new THREE.Mesh(bg, new THREE.MeshStandardMaterial({ color: 0x9aaec2, roughness: 0.7, transparent: true, opacity: 0.72, side: THREE.DoubleSide })));
    }

    const rSegs = [];
    for (const rd of rep.viewerContext.roads || []) {
      const pts = rd.points;
      for (let i = 0; i < pts.length - 1; i++) rSegs.push(pts[i][0], pts[i][2], -pts[i][1], pts[i + 1][0], pts[i + 1][2], -pts[i + 1][1]);
    }
    if (rSegs.length) {
      const rg = new THREE.BufferGeometry();
      rg.setAttribute('position', new THREE.Float32BufferAttribute(rSegs, 3));
      groups.roads.add(new THREE.LineSegments(rg, new THREE.LineBasicMaterial({ color: 0xffd27d, transparent: true, opacity: 0.28 })));
    }
  }

  function makeCircle(radiusM, colorHex, yOffset = 10) {
    const pts = [];
    for (let i = 0; i <= 64; i++) {
      const a = (i / 64) * Math.PI * 2;
      pts.push(new THREE.Vector3(Math.cos(a) * radiusM, yOffset, Math.sin(a) * radiusM));
    }
    return new THREE.Line(new THREE.BufferGeometry().setFromPoints(pts), new THREE.LineBasicMaterial({ color: colorHex, transparent: true, opacity: 0.6 }));
  }

  function buildPolicyScene(polName) {
    activePolicy = polName;
    const run = replay.runs[polName];
    if (!run) return;
    const ev = run.evaluation;

    disposeGroup(groups.sites);
    disposeGroup(groups.rings);
    disposeGroup(groups.civil);
    disposeGroup(groups.truth);
    disposeGroup(groups.obs);
    disposeGroup(groups.debris);
    threatActors = [];
    intActors = [];
    burstActors = [];
    debrisActors = [];
    civilActors = [];
    obsDotPool = [];

    // Assets
    for (const a of replay.assets || []) {
      const p = enuToThree(a.enu);
      const hub = new THREE.Mesh(new THREE.CylinderGeometry(24, 34, 65, 6), new THREE.MeshStandardMaterial({ color: 0xf472b6, emissive: 0x831843, roughness: 0.3 }));
      hub.position.set(p.x, p.y + 32, p.z);
      groups.sites.add(hub);
      const ring = makeCircle(a.radiusM, 0xf472b6, 12);
      ring.position.copy(p);
      groups.rings.add(ring);
    }

    // Sensors
    for (const s of replay.sensors || []) {
      const p = enuToThree(s.enu);
      const col = s.type === 'eo_ir' ? 0xfbbf24 : s.type === 'acoustic' ? 0xc084fc : 0x38bdf8;
      const m = new THREE.Mesh(new THREE.OctahedronGeometry(22, 0), new THREE.MeshStandardMaterial({ color: col, emissive: col, emissiveIntensity: 0.4 }));
      m.position.copy(p);
      groups.sites.add(m);
    }

    // Batteries & range rings (available in both Debrief & Operator COP via ownForceTelemetry)
    const siteBatts = ev?.batteries || run.ownForceTelemetry?.batteries || [];
    for (const b of siteBatts) {
      if (!b.enu) continue;
      const p = enuToThree(b.enu);
      const col = TIER_COLORS[b.tier] || 0x38bdf8;
      const base = new THREE.Mesh(new THREE.BoxGeometry(40, 26, 40), new THREE.MeshStandardMaterial({ color: col, emissive: col, emissiveIntensity: 0.35 }));
      base.position.set(p.x, p.y + 13, p.z);
      groups.sites.add(base);
      const ring = makeCircle(Math.min(b.maxRangeM || 12000, 18000), col, 8);
      ring.position.copy(p);
      groups.rings.add(ring);
    }

    // Civilian Airliners & Safety Cylinders
    for (const ct of replay.civilianTraffic || []) {
      const grp = new THREE.Group();
      const body = new THREE.Mesh(new THREE.ConeGeometry(28, 100, 6), new THREE.MeshStandardMaterial({ color: 0xe0f2fe, emissive: 0x0284c7, emissiveIntensity: 0.5 }));
      body.rotation.x = Math.PI / 2;
      grp.add(body);
      if (ct.kind === 'civilian_airliner') {
        const wing = new THREE.Mesh(new THREE.BoxGeometry(120, 4, 24), new THREE.MeshStandardMaterial({ color: 0xbae6fd }));
        grp.add(wing);
        const cyl = new THREE.Mesh(
          new THREE.CylinderGeometry(ct.safety_radius_m || 1500, ct.safety_radius_m || 1500, (ct.safety_half_height_m || 450) * 2, 32, 1, true),
          new THREE.MeshBasicMaterial({ color: 0x38bdf8, transparent: true, opacity: 0.08, side: THREE.DoubleSide, depthWrite: false })
        );
        grp.add(cyl);
      }
      groups.civil.add(grp);
      civilActors.push({ track: ct, group: grp });
    }

    const tr = run.truth || null;
    const ownInts = tr?.interceptors || run.ownForceTelemetry?.interceptors || [];

    // Truth Threats (stripped on server in Operator COP mode)
    for (const th of tr?.threats || []) {
      const col = THREAT_COLORS[th.kind] || 0xff5c72;
      const pts = th.samples.map((s) => enuToThree([s[1], s[2], s[3]]));
      const line = new THREE.Line(
        new THREE.BufferGeometry().setFromPoints(pts),
        new THREE.LineBasicMaterial({ color: col, transparent: true, opacity: th.lethal ? 0.55 : 0.35 })
      );
      groups.truth.add(line);
      const geom = th.kind === 'ballistic_missile'
        ? new THREE.ConeGeometry(28, 105, 8)
        : th.kind === 'decoy'
          ? new THREE.OctahedronGeometry(26, 0)
          : new THREE.ConeGeometry(22, 72, 6);
      const mesh = new THREE.Mesh(geom, new THREE.MeshStandardMaterial({ color: col, emissive: col, emissiveIntensity: 0.5 }));
      if (th.kind !== 'decoy') mesh.rotation.x = Math.PI / 2;
      mesh.visible = false;
      groups.truth.add(mesh);
      threatActors.push({ th, mesh });
    }

    // Own-force Interceptors (permitted in both Operator COP and Debrief)
    for (const flt of ownInts) {
      if (!flt.samples || flt.samples.length < 2) continue;
      const col = TIER_COLORS[flt.batteryTier] || 0x34d399;
      const pts = flt.samples.map((s) => enuToThree([s[1], s[2], s[3]]));
      const line = new THREE.Line(
        new THREE.BufferGeometry().setFromPoints(pts),
        new THREE.LineBasicMaterial({ color: col, transparent: true, opacity: 0.85 })
      );
      line.geometry.setDrawRange(0, 0);
      groups.obs.add(line);
      const mesh = new THREE.Mesh(new THREE.ConeGeometry(16, 56, 6), new THREE.MeshBasicMaterial({ color: 0xffffff }));
      mesh.visible = false;
      groups.obs.add(mesh);
      intActors.push({ flt, mesh, line });
    }

    // Intercept & Leaker Bursts (only present in Debrief)
    for (const evItem of run.events || []) {
      if ((evItem.kind === 'intercept_kill' || evItem.kind === 'leaker_impact') && evItem.enu) {
        const isKill = evItem.kind === 'intercept_kill';
        const orb = new THREE.Mesh(
          new THREE.SphereGeometry(isKill ? 75 : 95, 14, 10),
          new THREE.MeshBasicMaterial({ color: isKill ? 0xffca63 : 0xff2e4c, transparent: true, opacity: 0.75 })
        );
        orb.position.copy(enuToThree(evItem.enu));
        orb.visible = false;
        groups.truth.add(orb);
        burstActors.push({ tS: evItem.tS, mesh: orb });
      }
    }

    // Shrapnel ground impacts (only present in Debrief)
    const catCol = { building: 0xff2e4c, road: 0xff9e3d, protected_zone: 0xf065d8, open: 0x58e08e };
    for (const imp of (tr?.shrapnelImpacts || []).slice(0, 600)) {
      const m = new THREE.Mesh(
        new THREE.SphereGeometry(imp.hazardous ? 18 : 11, 6, 6),
        new THREE.MeshBasicMaterial({ color: catCol[imp.category] || 0x58e08e })
      );
      const p = enuToThree(imp.enu);
      p.y += 8;
      m.position.copy(p);
      m.visible = false;
      groups.debris.add(m);
      debrisActors.push({ tS: imp.tS, mesh: m });
    }

    // Observation fused track pool (strictly EXERCISE_OBSERVATION!)
    for (let i = 0; i < 28; i++) {
      const wire = new THREE.Mesh(
        new THREE.OctahedronGeometry(38, 0),
        new THREE.MeshBasicMaterial({ color: 0x38bdf8, wireframe: true })
      );
      wire.visible = false;
      groups.obs.add(wire);
      obsDotPool.push(wire);
    }

    // Update UI
    document.querySelectorAll('#policy-buttons button').forEach((btn) => {
      const p = btn.dataset.policy;
      btn.classList.toggle('active', p === activePolicy);
      btn.classList.toggle('good-active', p === activePolicy && p !== 'greedy_earliest');
    });

    if (ev) {
      document.getElementById('kpi-grid').innerHTML = `
        <div class="kpi"><div class="k">Defense Score</div><div class="v ${ev.compositeScore >= 75 ? 'good' : ev.compositeScore >= 45 ? 'warn' : 'bad'}">${ev.compositeScore.toFixed(1)}/100</div></div>
        <div class="kpi"><div class="k">Lethal Killed / Leakers</div><div class="v ${ev.leakers === 0 ? 'good' : 'bad'}">${ev.lethalIntercepted}/${ev.lethalThreats} (${ev.leakers} leak)</div></div>
        <div class="kpi"><div class="k">Interceptor Spend</div><div class="v cyan">$${ev.totalCostKUsd.toLocaleString()}k (${ev.shotsFired} shots)</div></div>
        <div class="kpi"><div class="k">Decoy Waste / Air Viol.</div><div class="v ${ev.decoyShotsWasted === 0 && ev.airspaceViolations === 0 ? 'good' : 'warn'}">${ev.decoyShotsWasted} decoy · ${ev.airspaceViolations} air</div></div>
        <div class="kpi"><div class="k">Debris Bldgs / WorldPop</div><div class="v">${ev.debrisBuildingHits} bldg · ~${ev.estimatedPopulationExposed} pop</div></div>
        <div class="kpi"><div class="k">Road Blocked / Detour</div><div class="v">${ev.roadImpact.blockedRoadKm.toFixed(2)}km (+${Math.round(ev.roadImpact.emergencyDetourDelayS || 0)}s)</div></div>
      `;
    } else {
      const launchCount = ownInts.length;
      const trackFramesCount = (run.observations?.trackFrames || []).length;
      document.getElementById('kpi-grid').innerHTML = `
        <div class="kpi"><div class="k">View Partition</div><div class="v warn">OBSERVATION</div></div>
        <div class="kpi"><div class="k">Own Launches</div><div class="v cyan">${launchCount} missiles</div></div>
        <div class="kpi"><div class="k">Fused Frames</div><div class="v">${trackFramesCount} ticks</div></div>
        <div class="kpi"><div class="k">Truth &amp; Eval</div><div class="v warn">SEALED</div></div>
      `;
    }

    const d = replay.comparison?.wtaVsGreedy;
    const db = document.getElementById('delta-banner');
    if (d && Object.keys(d).length) {
      db.innerHTML = `<div class="delta-banner"><strong>WTA vs Greedy:</strong> Score <strong class="${d.scoreDelta >= 0 ? 'good' : 'bad'}">${d.scoreDelta >= 0 ? '+' : ''}${d.scoreDelta}</strong> · Cost Saved <strong class="good">$${d.costSavedKUsd.toLocaleString()}k (${d.costSavedPct}%)</strong> · Decoy Δ <strong>${d.decoyWasteDelta}</strong></div>`;
    } else {
      db.innerHTML = '';
    }

    const rows = Object.entries(replay.comparison?.policies || {}).map(([p, r]) =>
      `<div class="track-row"><span>${POLICY_LABELS[p] || p}</span><span>${r.compositeScore} pts · ${r.leakers}L · $${r.totalCostKUsd}k</span></div>`
    );
    document.getElementById('policy-table').innerHTML = rows.join('') || '<div style="font-size:10.5px;color:var(--muted);">Unlock Evaluator Debrief to view policy scoring.</div>';

    const evHtml = (run.events || [])
      .filter((x) => x.kind !== 'launch')
      .slice(0, 40)
      .map((x) => {
        const cls = x.kind === 'intercept_kill' ? 'good' : x.kind === 'leaker_impact' ? 'bad' : 'warn';
        return `<div class="event-item"><span class="cyan">t=${x.tS}s</span> <span class="${cls}">${x.kind.replace('_', ' ')}</span> ${x.threatId || ''} ${x.batteryId ? `← ${x.batteryId}` : ''}</div>`;
      })
      .join('');
    document.getElementById('event-log').innerHTML = evHtml || '<span style="color:var(--muted);">No operator-visible events.</span>';

    const trEnds = (tr?.threats || []).map((t) => t.endS + 6);
    const tfEnds = (run.observations?.trackFrames || []).map((f) => f.tS + 4);
    maxTime = Math.max(25, Number(replay.scenario?.durationS || 60), ...trEnds, ...tfEnds);
    document.getElementById('time-slider').max = String(Math.ceil(maxTime));
    simTime = 0;
    updateAtTime(0);
  }

  function updateAtTime(t) {
    if (!replay) return;
    const run = replay.runs[activePolicy];
    if (!run) return;
    document.getElementById('time-slider').value = String(t);
    document.getElementById('time-readout').textContent = `t = ${t.toFixed(1)}s`;

    // Civilian airliners
    for (const a of civilActors) {
      const s = a.track.start_enu, v = a.track.vel_enu;
      const pos = enuToThree([s[0] + v[0] * t, s[1] + v[1] * t, s[2] + v[2] * t]);
      const nxt = enuToThree([s[0] + v[0] * (t + 1), s[1] + v[1] * (t + 1), s[2] + v[2] * (t + 1)]);
      a.group.position.copy(pos);
      if (pos.distanceToSquared(nxt) > 1e-4) a.group.lookAt(nxt);
    }

    // Threats
    for (const a of threatActors) {
      const st = interpSampleList(a.th.samples, t);
      if (st) {
        a.mesh.visible = true;
        a.mesh.position.copy(st.pos);
        if (st.pos.distanceToSquared(st.next) > 1e-4) a.mesh.lookAt(st.next);
      } else {
        a.mesh.visible = false;
      }
    }

    // Interceptors
    for (const a of intActors) {
      const st = interpSampleList(a.flt.samples, t);
      let cnt = 0;
      for (let i = 0; i < a.flt.samples.length; i++) {
        if (a.flt.samples[i][0] <= t) cnt = i + 1;
        else break;
      }
      a.line.geometry.setDrawRange(0, cnt);
      if (st) {
        a.mesh.visible = true;
        a.mesh.position.copy(st.pos);
        if (st.pos.distanceToSquared(st.next) > 1e-4) a.mesh.lookAt(st.next);
      } else {
        a.mesh.visible = false;
      }
    }

    // Bursts
    for (const b of burstActors) {
      const age = t - b.tS;
      if (age >= 0 && age <= 3.0) {
        b.mesh.visible = true;
        const u = age / 3.0;
        b.mesh.scale.setScalar(0.4 + u * 2.2);
        b.mesh.material.opacity = (1 - u) * 0.85;
      } else {
        b.mesh.visible = false;
      }
    }

    // Debris ground hits
    for (const d of debrisActors) d.mesh.visible = t >= d.tS;

    // Live Magazine HUD
    const magFrames = run.magazineTimeline || [];
    let curMags = null;
    for (const mf of magFrames) {
      if (mf.tS <= t) curMags = mf.magazines;
      else break;
    }
    const batts = run.evaluation?.batteries || run.ownForceTelemetry?.batteries || (replay.scenario?.batteries || []).map((b) => ({
      id: b.id,
      label: b.label || b.id,
      color: b.tier === 'tier1_sam' ? '#38bdf8' : b.tier === 'tier2_shorad' ? '#34d399' : '#fbbf24',
      magazineInitial: b.magazine || 8,
      magazineRemaining: b.magazine || 8,
      costKUsd: b.tier === 'tier1_sam' ? 1200 : b.tier === 'tier2_shorad' ? 180 : 18,
      shotsFired: '—',
      kills: '—',
    }));
    document.getElementById('magazine-bars').innerHTML = batts.map((b) => {
      const rem = curMags ? (curMags[b.id] ?? b.magazineRemaining) : b.magazineInitial;
      const pct = Math.round((rem / Math.max(1, b.magazineInitial)) * 100);
      const killsLabel = b.kills !== undefined && b.kills !== '—' ? `${b.kills} kills` : 'COP';
      return `
        <div class="mag-card">
          <div class="mag-head"><span style="color:${b.color};">${b.label}</span><span>${rem}/${b.magazineInitial}</span></div>
          <div class="mag-bar"><div class="mag-fill" style="width:${pct}%;background:${b.color};"></div></div>
          <div class="mag-meta"><span>$${b.costKUsd}k/shot</span><span>${b.shotsFired} fired · ${killsLabel}</span></div>
        </div>`;
    }).join('');

    // Fused Track COP (strictly EXERCISE_OBSERVATION)
    const tFrames = run.observations?.trackFrames || [];
    let curTracks = [];
    for (const tf of tFrames) {
      if (tf.tS <= t) curTracks = tf.tracks || [];
      else break;
    }
    obsDotPool.forEach((m, i) => {
      const tr = curTracks[i];
      if (tr) {
        m.visible = true;
        m.position.copy(enuToThree(tr.posEnu));
        m.material.color.setHex(tr.iffCooperative ? 0x7dd3fc : tr.topClass === 'decoy' ? 0xfde047 : 0x38bdf8);
      } else {
        m.visible = false;
      }
    });
    document.getElementById('track-list').innerHTML = curTracks.length
      ? curTracks.map((tr) => `<div class="track-row"><span>${tr.trackId} (${tr.topClass.replace('_', ' ')})</span><span>${tr.speedMps}m/s · p=${Math.round((tr.classProbs[tr.topClass] || 0) * 100)}%</span></div>`).join('')
      : '<div style="font-size:11px;color:var(--muted);">No active fused tracks at this instant.</div>';
  }

  // --- Phase 4: Live Interactive Command-in-the-Loop Exercise (EXERCISE_OBSERVATION only) ---
  const SESSIONS_BASE = USES_GO_GATEWAY ? '/api/v1/sim-platform/sessions' : `${API_BASE}/sessions`;
  let liveSession = null;
  let liveAbort = null;
  let liveStepTimer = null;
  let livePaused = false;
  let liveIntPool = [];

  function ensureLivePools() {
    while (liveIntPool.length < 32) {
      const cone = new THREE.Mesh(
        new THREE.ConeGeometry(18, 60, 6),
        new THREE.MeshBasicMaterial({ color: 0x34d399 })
      );
      cone.visible = false;
      groups.obs.add(cone);
      liveIntPool.push(cone);
    }
    while (obsDotPool.length < 28) {
      const wire = new THREE.Mesh(
        new THREE.OctahedronGeometry(38, 0),
        new THREE.MeshBasicMaterial({ color: 0x38bdf8, wireframe: true })
      );
      wire.visible = false;
      groups.obs.add(wire);
      obsDotPool.push(wire);
    }
  }

  function syncLiveButtons() {
    const toggleBtn = document.getElementById('btn-live-toggle');
    const hfBtn = document.getElementById('btn-live-holdfire');
    const badge = document.getElementById('live-badge');
    const isLive = Boolean(liveSession);
    if (toggleBtn) {
      toggleBtn.textContent = isLive ? '⏹ Stop Live' : '▶ Start Live';
      toggleBtn.classList.toggle('primary', !isLive);
      toggleBtn.classList.toggle('active', isLive);
    }
    if (hfBtn) {
      hfBtn.disabled = !isLive;
      hfBtn.classList.toggle('active', Boolean(liveSession?.holdFire));
      hfBtn.textContent = liveSession?.holdFire ? '✅ Clear Hold Fire' : '🛑 Hold Fire';
    }
    if (badge) {
      if (!isLive) {
        badge.textContent = 'REPLAY';
        badge.className = 'badge';
      } else if (liveSession.status === 'completed') {
        badge.textContent = 'LIVE DONE';
        badge.className = 'badge good';
      } else if (livePaused) {
        badge.textContent = 'LIVE PAUSED';
        badge.className = 'badge warn';
      } else {
        badge.textContent = 'LIVE COP';
        badge.className = 'badge good';
      }
    }
  }

  async function sendLiveCommand(kind, payload = {}) {
    if (!liveSession) return;
    try {
      const res = await fetch(`${SESSIONS_BASE}/${encodeURIComponent(liveSession.sessionId)}/commands`, {
        method: 'POST',
        headers: apiHeaders({ 'Content-Type': 'application/json' }),
        body: JSON.stringify({ kind, payload }),
      });
      if (res.ok && !USES_GO_GATEWAY) {
        const snap = await res.json();
        if (snap.observation) applyLiveSnapshot(snap);
      }
    } catch (err) {
      console.warn('Live command error:', err);
    }
  }

  async function controlLiveSession(action, timeScale = playSpeed) {
    if (!liveSession) return;
    if (action === 'pause') livePaused = true;
    if (action === 'resume') livePaused = false;
    syncLiveButtons();
    if (USES_GO_GATEWAY) {
      await fetch(`${SESSIONS_BASE}/${encodeURIComponent(liveSession.sessionId)}/control`, {
        method: 'POST',
        headers: apiHeaders({ 'Content-Type': 'application/json' }),
        body: JSON.stringify({ action, timeScale }),
      }).catch(() => {});
    }
  }

  function applyLiveSnapshot(snap) {
    if (!snap || !liveSession) return;
    liveSession = { ...liveSession, ...snap };
    ensureLivePools();

    // Hide sealed truth/debris layers during live EXERCISE_OBSERVATION COP!
    groups.truth.visible = false;
    groups.debris.visible = false;
    intActors.forEach((a) => { a.mesh.visible = false; a.line.visible = false; });

    const obs = snap.observation || {};
    const vt = Number(snap.virtualTimeS || 0);
    const dur = Number(snap.durationS || 115);
    document.getElementById('time-slider').max = String(Math.ceil(dur));
    document.getElementById('time-slider').value = String(vt);
    document.getElementById('time-readout').textContent = `LIVE t = ${vt.toFixed(1)}s`;

    // Sync active policy buttons
    document.querySelectorAll('#policy-buttons button').forEach((btn) => {
      const p = btn.dataset.policy;
      btn.classList.toggle('active', p === snap.policy);
      btn.classList.toggle('good-active', p === snap.policy && p !== 'greedy_earliest');
    });

    // Live fused tracks (strictly EXERCISE_OBSERVATION)
    const tracks = obs.tracks || [];
    obsDotPool.forEach((m, i) => {
      const tr = tracks[i];
      if (tr) {
        m.visible = true;
        m.position.copy(enuToThree(tr.posEnu));
        m.material.color.setHex(tr.iffCooperative ? 0x7dd3fc : tr.topClass === 'decoy' ? 0xfde047 : 0x38bdf8);
      } else {
        m.visible = false;
      }
    });

    // Live own-force airborne interceptors
    const activeInts = obs.activeInterceptors || [];
    liveIntPool.forEach((m, i) => {
      const f = activeInts[i];
      if (f && f.enu) {
        m.visible = true;
        m.position.copy(enuToThree(f.enu));
        m.material.color.setHex(TIER_COLORS[f.batteryTier] || 0x34d399);
      } else {
        m.visible = false;
      }
    });

    // Live civilian airliners
    const civById = new Map((obs.civilianTraffic || []).map((c) => [c.id, c]));
    for (const a of civilActors) {
      const liveC = civById.get(a.track.id);
      if (liveC && liveC.enu) a.group.position.copy(enuToThree(liveC.enu));
    }

    // Live battery magazines with interactive TOGGLE_BATTERY
    const batts = obs.batteries || [];
    document.getElementById('magazine-bars').innerHTML = batts.map((b) => {
      const pct = Math.round((b.magazineRemaining / Math.max(1, b.magazineInitial)) * 100);
      const stateBadge = b.enabled ? 'ONLINE' : 'OFFLINE';
      return `
        <div class="mag-card" style="opacity:${b.enabled ? 1 : 0.5};">
          <div class="mag-head">
            <span style="color:${b.color};">${b.label}</span>
            <span>${b.magazineRemaining}/${b.magazineInitial} <button class="chip" data-toggle-batt="${b.id}" data-next-enabled="${!b.enabled}" style="padding:1px 5px;font-size:9.5px;">${stateBadge}</button></span>
          </div>
          <div class="mag-bar"><div class="mag-fill" style="width:${pct}%;background:${b.color};"></div></div>
          <div class="mag-meta"><span>$${b.costKUsd}k/shot</span><span>${b.shotsFired} fired · ${b.activeInterceptors} in air</span></div>
        </div>`;
    }).join('');
    document.querySelectorAll('#magazine-bars button[data-toggle-batt]').forEach((btn) => {
      btn.onclick = () => sendLiveCommand('TOGGLE_BATTERY', { batteryId: btn.dataset.toggleBatt, enabled: btn.dataset.nextEnabled === 'true' });
    });

    // Live Fused Track COP with interactive ENGAGE_TRACK
    document.getElementById('track-list').innerHTML = tracks.length
      ? tracks.map((tr) => `
          <div class="track-row">
            <span>${tr.trackId} (${tr.topClass.replace('_', ' ')})</span>
            <span style="display:flex;align-items:center;gap:4px;">
              ${tr.speedMps}m/s
              ${tr.iffCooperative ? '<span class="cyan">IFF</span>' : `<button class="chip" data-engage-track="${tr.trackId}" style="padding:1px 5px;font-size:9.5px;">🎯 Engage</button>`}
            </span>
          </div>`).join('')
      : '<div style="font-size:11px;color:var(--muted);">Scanning airspace — no fused tracks yet.</div>';
    document.querySelectorAll('#track-list button[data-engage-track]').forEach((btn) => {
      btn.onclick = () => sendLiveCommand('ENGAGE_TRACK', { trackId: btn.dataset.engageTrack });
    });

    // Live KPIs & Operator Events
    const totalShots = batts.reduce((s, b) => s + Number(b.shotsFired || 0), 0);
    document.getElementById('kpi-grid').innerHTML = `
      <div class="kpi"><div class="k">Live Mode</div><div class="v good">INTERACTIVE COP</div></div>
      <div class="kpi"><div class="k">Weapons Control</div><div class="v ${snap.holdFire ? 'bad' : 'good'}">${snap.holdFire ? 'HOLD FIRE' : 'WEAPONS FREE'}</div></div>
      <div class="kpi"><div class="k">Fused Tracks</div><div class="v cyan">${tracks.length} active</div></div>
      <div class="kpi"><div class="k">Own Launches</div><div class="v">${totalShots} (${activeInts.length} in flight)</div></div>
    `;
    const evs = (obs.recentEvents || []).slice().reverse();
    document.getElementById('event-log').innerHTML = evs.length
      ? evs.map((x) => `<div class="event-item"><span class="cyan">t=${x.tS}s</span> <span class="warn">${x.kind.replace(/_/g, ' ')}</span> ${x.command || x.trackId || ''} ${x.batteryId ? `← ${x.batteryId}` : ''}</div>`).join('')
      : '<span style="color:var(--muted);">Waiting for live operator or sensor events…</span>';

    if (snap.completed || snap.status === 'completed' || snap.status === 'stopped') {
      liveSession.status = snap.completed ? 'completed' : 'stopped';
      if (liveStepTimer) { clearInterval(liveStepTimer); liveStepTimer = null; }
      statusEl.textContent = `Live Exercise ${liveSession.status} at t=${vt.toFixed(1)}s (EXERCISE_OBSERVATION).`;
    }
    syncLiveButtons();
  }

  async function stopLiveSession() {
    if (!liveSession) return;
    const sid = liveSession.sessionId;
    if (liveAbort) { liveAbort.abort(); liveAbort = null; }
    if (liveStepTimer) { clearInterval(liveStepTimer); liveStepTimer = null; }
    liveIntPool.forEach((m) => { m.visible = false; });
    liveSession = null;
    livePaused = false;
    syncLiveButtons();
    await fetch(`${SESSIONS_BASE}/${encodeURIComponent(sid)}/stop`, {
      method: 'POST',
      headers: apiHeaders(),
    }).catch(() => {});
    // Restore replay view
    groups.truth.visible = document.getElementById('tog-truth')?.checked ?? true;
    groups.debris.visible = document.getElementById('tog-debris')?.checked ?? true;
    intActors.forEach((a) => { a.line.visible = true; });
    if (replay) buildPolicyScene(activePolicy);
    statusEl.textContent = 'Returned to replay mode.';
  }

  async function startLiveSession() {
    if (!replay) return;
    if (liveSession) { await stopLiveSession(); return; }
    const base = replay.sourceDraft ? JSON.parse(JSON.stringify(replay.sourceDraft)) : { environmentId: replay.environmentId };
    base.policy = activePolicy;
    base.timeScale = playSpeed;
    statusEl.textContent = 'Starting live interactive Salvo exercise (EXERCISE_OBSERVATION)…';
    const res = await fetch(SESSIONS_BASE, {
      method: 'POST',
      headers: apiHeaders({ 'Content-Type': 'application/json' }),
      body: JSON.stringify(base),
    });
    const initSnap = await res.json();
    if (!res.ok) throw new Error(initSnap.error || `Live session HTTP ${res.status}`);

    liveSession = initSnap;
    livePaused = false;
    playing = true;
    playBtn.textContent = '⏸ Pause';
    playBtn.classList.add('active');
    syncViewScopeUI('LIVE EXERCISE_OBSERVATION COP — Sealed truth hidden while exercise is active.');
    applyLiveSnapshot(initSnap);
    statusEl.textContent = `Live session ${initSnap.sessionId} active (${USES_GO_GATEWAY ? 'Go SSE clock' : 'Python step loop'}).`;

    if (USES_GO_GATEWAY) {
      liveAbort = new AbortController();
      (async () => {
        try {
          const evRes = await fetch(`${SESSIONS_BASE}/${encodeURIComponent(initSnap.sessionId)}/events`, {
            headers: apiHeaders({ Accept: 'text/event-stream' }),
            signal: liveAbort.signal,
          });
          if (!evRes.ok || !evRes.body) return;
          const reader = evRes.body.getReader();
          const dec = new TextDecoder();
          let buf = '';
          while (true) {
            const { value, done } = await reader.read();
            if (done) break;
            buf += dec.decode(value, { stream: true });
            const chunks = buf.split('\n\n');
            buf = chunks.pop() || '';
            for (const chunk of chunks) {
              const dataLine = chunk.split('\n').find((l) => l.startsWith('data:'));
              if (dataLine) {
                try { applyLiveSnapshot(JSON.parse(dataLine.slice(5).trim())); } catch {}
              }
            }
          }
        } catch (err) {
          if (err?.name !== 'AbortError') console.warn('Live SSE closed:', err);
        }
      })();
    } else {
      let inFlight = false;
      liveStepTimer = setInterval(async () => {
        if (!liveSession || livePaused || inFlight || liveSession.completed || liveSession.stopped) return;
        inFlight = true;
        try {
          const ticks = Math.max(1, Math.round(playSpeed));
          const stRes = await fetch(`${SESSIONS_BASE}/${encodeURIComponent(liveSession.sessionId)}/step`, {
            method: 'POST',
            headers: apiHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({ ticks }),
          });
          if (stRes.ok) applyLiveSnapshot(await stRes.json());
        } finally {
          inFlight = false;
        }
      }, 250);
    }
  }

  function moveCamera(preset) {
    const a0 = (replay?.assets || [])[0];
    const hub = a0 ? enuToThree(a0.enu) : sceneCenter;
    if (preset === 'overview') {
      controls.target.copy(sceneCenter);
      camera.position.copy(sceneCenter).add(new THREE.Vector3(sceneSize * 0.56, sceneSize * 0.42, sceneSize * 0.58));
    } else if (preset === 'top') {
      controls.target.copy(sceneCenter);
      camera.position.copy(sceneCenter).add(new THREE.Vector3(0, sceneSize * 0.95, 60));
    } else if (preset === 'hub') {
      controls.target.copy(hub);
      camera.position.copy(hub).add(new THREE.Vector3(1200, 900, 1400));
    } else if (preset === 'ingress') {
      controls.target.copy(sceneCenter);
      camera.position.copy(sceneCenter).add(new THREE.Vector3(-sceneSize * 0.52, sceneSize * 0.25, sceneSize * 0.48));
    }
  }

  let activeViewScope = new URLSearchParams(window.location.search).get('view') === 'operator' ? 'operator' : 'debrief';
  let currentEp = null;

  function syncViewScopeUI(noteText = '') {
    const opBtn = document.getElementById('btn-view-operator');
    const debBtn = document.getElementById('btn-view-debrief');
    const noteEl = document.getElementById('view-scope-note');
    if (opBtn) opBtn.classList.toggle('active', activeViewScope === 'operator');
    if (debBtn) debBtn.classList.toggle('active', activeViewScope === 'debrief');
    if (noteEl) {
      if (noteText) {
        noteEl.textContent = noteText;
        noteEl.style.display = 'block';
      } else if (activeViewScope === 'operator') {
        noteEl.textContent = 'EXERCISE_OBSERVATION ONLY — Sealed raid truth, decoy flags & evaluation stripped on server.';
        noteEl.style.display = 'block';
      } else {
        noteEl.style.display = 'none';
      }
    }
  }

  async function loadBundle(ep, resetCam = true) {
    currentEp = ep;
    stageEl.textContent = 'Fetching 3D Salvo replay bundle…';
    const sep = ep.replayUrl.includes('?') ? '&' : '?';
    let repRes = await fetch(`${ep.replayUrl}${sep}view=${encodeURIComponent(activeViewScope)}`, { headers: apiHeaders(), cache: 'no-store' });
    let scopeNote = '';
    if (repRes.status === 403 && activeViewScope === 'debrief') {
      activeViewScope = 'operator';
      scopeNote = 'Debrief truth requires sim.truth.read authorization — showing Operator COP (EXERCISE_OBSERVATION).';
      repRes = await fetch(`${ep.replayUrl}${sep}view=operator`, { headers: apiHeaders(), cache: 'no-store' });
    }
    if (!repRes.ok) throw new Error(`Salvo replay HTTP ${repRes.status}`);
    replay = await repRes.json();
    if (replay.truthRedacted) activeViewScope = 'operator';
    syncViewScopeUI(scopeNote);
    await loadEnvGeometry(replay, ep);

    document.getElementById('scenario-title').textContent = replay.scenario.title || 'Multi-Threat Salvo';
    const threatBadge = replay.truthRedacted
      ? '<span class="badge warn">EXERCISE_OBSERVATION COP</span>'
      : `<span class="badge">${(replay.scenario.threats || []).length} threats</span>`;
    document.getElementById('env-badges').innerHTML =
      threatBadge +
      `<span class="badge">${(replay.scenario.batteries || []).length} batteries</span>` +
      `<span class="badge good">${replay.engineVersion}</span>`;
    statusEl.textContent = `Manifest ${replay.environment.manifestSha256.slice(0, 12)}… · Computed in ${replay.elapsedS}s`;

    const sw = document.getElementById('run-switcher');
    sw.innerHTML = ep.runs.filter((r) => r.status === 'completed')
      .map((r) => `<option value="${r.jobId}" ${r.jobId === ep.jobId ? 'selected' : ''}>${r.jobId.slice(-6)} · ${r.threatCount}T</option>`)
      .join('');

    const polBtns = document.getElementById('policy-buttons');
    polBtns.innerHTML = (replay.policies || ['greedy_earliest', 'combinatorial_wta'])
      .map((p) => `<button class="chip" data-policy="${p}">${POLICY_LABELS[p] || p}</button>`)
      .join('');
    polBtns.querySelectorAll('button').forEach((b) => {
      b.onclick = () => {
        if (liveSession) sendLiveCommand('SET_POLICY', { policy: b.dataset.policy });
        else buildPolicyScene(b.dataset.policy);
      };
    });

    const sd = replay.sourceDraft || {};
    document.getElementById('wi-decoys').value = String(sd.raid?.decoys ?? 3);
    document.getElementById('wi-uavs').value = String(sd.raid?.uavs ?? 5);
    document.getElementById('wi-cruise').value = String(sd.raid?.cruiseMissiles ?? 2);
    document.getElementById('wi-ballistic').value = String(sd.raid?.ballisticMissiles ?? 1);
    document.getElementById('wi-vis').value = String(sd.visibilityKm ?? 10);
    document.getElementById('wi-seed').value = String(sd.seed ?? replay.scenario.seed ?? 20261127);
    const wiBtn = document.getElementById('btn-whatif-run');
    const wiSt = document.getElementById('wi-status');
    if (wiBtn) wiBtn.disabled = Boolean(replay.truthRedacted);
    if (wiSt) wiSt.textContent = replay.truthRedacted ? 'Switch to Evaluator Debrief to edit sealed raid composition.' : '';

    const pref = replay.runs.combinatorial_wta ? 'combinatorial_wta' : replay.policies[0];
    buildPolicyScene(pref);
    if (resetCam) moveCamera('overview');
    overlayEl.classList.add('hidden');
  }

  // Controls & bindings
  document.querySelectorAll('.collapse-btn[data-target]').forEach((b) => {
    b.onclick = () => {
      const p = document.getElementById(b.dataset.target);
      p.classList.toggle('collapsed');
      b.textContent = p.classList.contains('collapsed') ? '▸' : '▾';
    };
  });
  const toggleHud = () => document.body.classList.toggle('hud-hidden');
  document.getElementById('btn-hud-hide').onclick = toggleHud;
  document.getElementById('hud-restore-pill').onclick = toggleHud;

  document.querySelectorAll('.tab-btn').forEach((b) => {
    b.onclick = () => {
      document.querySelectorAll('.tab-btn').forEach((x) => x.classList.toggle('active', x === b));
      document.querySelectorAll('.tab-pane').forEach((p) => p.classList.toggle('active', p.id === `tab-${b.dataset.tab}`));
    };
  });

  const togMap = { 'tog-truth': groups.truth, 'tog-obs': groups.obs, 'tog-civil': groups.civil, 'tog-debris': groups.debris, 'tog-rings': groups.rings, 'tog-buildings': groups.buildings, 'tog-imagery': groups.imagery };
  Object.entries(togMap).forEach(([id, g]) => {
    const el = document.getElementById(id);
    if (el) el.onchange = () => { g.visible = el.checked; };
  });

  const playBtn = document.getElementById('btn-play');
  const togglePlay = () => {
    if (liveSession) {
      livePaused = !livePaused;
      playBtn.textContent = livePaused ? '▶ Resume' : '⏸ Pause';
      playBtn.classList.toggle('active', !livePaused);
      void controlLiveSession(livePaused ? 'pause' : 'resume', playSpeed);
      return;
    }
    playing = !playing;
    playBtn.textContent = playing ? '⏸ Pause' : '▶ Play';
    playBtn.classList.toggle('active', playing);
  };
  playBtn.onclick = togglePlay;
  document.getElementById('btn-live-toggle').onclick = () => {
    startLiveSession().catch((err) => {
      statusEl.textContent = `Live error: ${err.message}`;
    });
  };
  document.getElementById('btn-live-holdfire').onclick = () => {
    if (!liveSession) return;
    if (liveSession.holdFire) void sendLiveCommand('CLEAR_HOLD_FIRE', {});
    else void sendLiveCommand('HOLD_FIRE_ZONE', { enabled: true });
  };
  document.getElementById('time-slider').oninput = (e) => {
    if (liveSession) return;
    playing = false;
    playBtn.textContent = '▶ Play';
    playBtn.classList.remove('active');
    simTime = Number(e.target.value);
    updateAtTime(simTime);
  };
  document.getElementById('speed-select').onchange = (e) => {
    playSpeed = Number(e.target.value) || 1;
    if (liveSession) void controlLiveSession('speed', playSpeed);
  };
  document.querySelectorAll('#dock button[data-cam]').forEach((b) => { b.onclick = () => moveCamera(b.dataset.cam); });

  document.getElementById('btn-view-operator').onclick = async () => {
    if (liveSession) await stopLiveSession();
    if (!currentEp || activeViewScope === 'operator') return;
    activeViewScope = 'operator';
    const url = new URL(window.location.href);
    url.searchParams.set('view', 'operator');
    window.history.replaceState({}, '', url.toString());
    await loadBundle(currentEp, false);
  };
  document.getElementById('btn-view-debrief').onclick = async () => {
    if (liveSession) await stopLiveSession();
    if (!currentEp || activeViewScope === 'debrief') return;
    activeViewScope = 'debrief';
    const url = new URL(window.location.href);
    url.searchParams.set('view', 'debrief');
    window.history.replaceState({}, '', url.toString());
    await loadBundle(currentEp, false);
  };

  document.getElementById('run-switcher').onchange = async (e) => {
    if (liveSession) await stopLiveSession();
    const url = new URL(window.location.href);
    url.searchParams.set('run', e.target.value);
    window.history.replaceState({}, '', url.toString());
    overlayEl.classList.remove('hidden');
    await loadBundle(await resolveEndpoint(stageEl), false);
  };

  document.getElementById('btn-whatif-run').onclick = async () => {
    if (!replay) return;
    if (liveSession) await stopLiveSession();
    const st = document.getElementById('wi-status');
    const btn = document.getElementById('btn-whatif-run');
    btn.disabled = true;
    st.textContent = 'Running Salvo What-If…';
    try {
      const base = replay.sourceDraft ? JSON.parse(JSON.stringify(replay.sourceDraft)) : { environmentId: replay.environmentId };
      base.raid = {
        ...(base.raid || {}),
        decoys: Number(document.getElementById('wi-decoys').value),
        uavs: Number(document.getElementById('wi-uavs').value),
        cruiseMissiles: Number(document.getElementById('wi-cruise').value),
        ballisticMissiles: Number(document.getElementById('wi-ballistic').value),
      };
      base.visibilityKm = Number(document.getElementById('wi-vis').value);
      base.seed = Number(document.getElementById('wi-seed').value);
      const res = await apiFetch('/runs', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(base) });
      const created = await res.json();
      if (!res.ok) throw new Error(created.error || `HTTP ${res.status}`);
      const url = new URL(window.location.href);
      url.searchParams.set('run', created.jobId);
      window.history.replaceState({}, '', url.toString());
      await loadBundle(await resolveEndpoint(stageEl), false);
      st.textContent = `Completed (${created.jobId.slice(-6)}) in ${replay.elapsedS}s`;
    } catch (err) {
      st.textContent = `Error: ${err.message}`;
    } finally {
      btn.disabled = false;
    }
  };

  window.addEventListener('keydown', (e) => {
    if (['INPUT', 'SELECT', 'TEXTAREA'].includes(e.target?.tagName)) return;
    const k = e.key.toLowerCase();
    if ('wasdqe'.includes(k)) keysHeld.add(k);
    if (k === ' ') { e.preventDefault(); togglePlay(); }
    if (k === 'h') toggleHud();
    if (k === '1') moveCamera('overview');
    if (k === '2') moveCamera('top');
    if (k === '3') moveCamera('hub');
    if (k === '4') moveCamera('ingress');
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
    if (keysHeld.size > 0) {
      const spd = Math.max(450, camera.position.distanceTo(controls.target) * 0.45) * dt;
      const fwd = new THREE.Vector3();
      camera.getWorldDirection(fwd); fwd.y = 0; if (fwd.lengthSq() > 1e-6) fwd.normalize();
      const right = new THREE.Vector3().crossVectors(fwd, new THREE.Vector3(0, 1, 0)).normalize();
      const d = new THREE.Vector3();
      if (keysHeld.has('w')) d.addScaledVector(fwd, spd);
      if (keysHeld.has('s')) d.addScaledVector(fwd, -spd);
      if (keysHeld.has('d')) d.addScaledVector(right, spd);
      if (keysHeld.has('a')) d.addScaledVector(right, -spd);
      if (keysHeld.has('e')) d.y += spd;
      if (keysHeld.has('q')) d.y -= spd;
      camera.position.add(d); controls.target.add(d);
    }
    if (playing && replay && !liveSession) {
      simTime += dt * playSpeed * 3.5;
      if (simTime > maxTime) simTime = 0;
      updateAtTime(simTime);
    }
    controls.update();
    renderer.render(scene, camera);
  }
  animate();

  try {
    await loadBundle(await resolveEndpoint(stageEl), true);
  } catch (err) {
    stageEl.textContent = `Error: ${err.message}`;
    stageEl.style.color = '#f87171';
    console.error(err);
  }
}

main();
