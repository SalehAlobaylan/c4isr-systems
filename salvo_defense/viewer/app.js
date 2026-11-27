import * as THREE from 'three';
import { OrbitControls } from 'three/addons/controls/OrbitControls.js';
import { GLTFLoader } from 'three/addons/loaders/GLTFLoader.js';

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
  const runParam = params.get('run');

  const runsRes = await fetch('/api/runs', { cache: 'no-store' });
  const runs = runsRes.ok ? ((await runsRes.json()).runs || []) : [];
  let jobId = runParam;
  if (!jobId) {
    const done = runs.find((r) => r.status === 'completed');
    if (done) {
      jobId = done.jobId;
    } else {
      stageEl.textContent = 'No completed salvo yet — launching default layered defense salvo…';
      const envs = ((await (await fetch('/api/environments', { cache: 'no-store' })).json()).environments || []);
      if (!envs.length) throw new Error('No verified environments found.');
      const cr = await fetch('/api/runs', {
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
    const res = await fetch(`/api/runs/${encodeURIComponent(jobId)}`, { cache: 'no-store' });
    if (res.status === 404) {
      stageEl.textContent = 'Waiting for salvo run registration…';
      await new Promise((r) => setTimeout(r, 250));
      continue;
    }
    if (!res.ok) throw new Error(`Salvo job HTTP ${res.status}`);
    job = await res.json();
    if (job.status === 'completed') break;
    if (job.status === 'failed' || job.status === 'interrupted') throw new Error(job.error || job.stage || 'Salvo failed.');
    stageEl.textContent = job.stage || 'Running salvo simulation…';
    await new Promise((r) => setTimeout(r, 250));
  }
  if (!job || job.status !== 'completed') throw new Error('Timed out waiting for salvo run.');
  const freshRuns = ((await (await fetch('/api/runs', { cache: 'no-store' })).json()).runs || runs);
  return {
    jobId,
    runs: freshRuns,
    hasGlb: Boolean(job.hasGlb),
    hasImagery: Boolean(job.hasImagery),
    replayUrl: `/api/runs/${encodeURIComponent(jobId)}/replay.json`,
    glbUrl: `/api/runs/${encodeURIComponent(jobId)}/terrain.glb`,
    imageryUrl: `/api/runs/${encodeURIComponent(jobId)}/imagery.json`,
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
      const gltf = await new GLTFLoader().loadAsync(ep.glbUrl);
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
        const rawImg = await (await fetch(ep.imageryUrl)).json();
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

    // Batteries & range rings
    for (const b of ev.batteries || []) {
      const p = enuToThree(b.enu);
      const col = TIER_COLORS[b.tier] || 0x38bdf8;
      const base = new THREE.Mesh(new THREE.BoxGeometry(40, 26, 40), new THREE.MeshStandardMaterial({ color: col, emissive: col, emissiveIntensity: 0.35 }));
      base.position.set(p.x, p.y + 13, p.z);
      groups.sites.add(base);
      const ring = makeCircle(Math.min(b.maxRangeM, 18000), col, 8);
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

    // Truth Threats
    for (const th of run.truth.threats || []) {
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

    // Interceptors
    for (const flt of run.truth.interceptors || []) {
      if (!flt.samples || flt.samples.length < 2) continue;
      const col = TIER_COLORS[flt.batteryTier] || 0x34d399;
      const pts = flt.samples.map((s) => enuToThree([s[1], s[2], s[3]]));
      const line = new THREE.Line(
        new THREE.BufferGeometry().setFromPoints(pts),
        new THREE.LineBasicMaterial({ color: col, transparent: true, opacity: 0.85 })
      );
      line.geometry.setDrawRange(0, 0);
      groups.truth.add(line);
      const mesh = new THREE.Mesh(new THREE.ConeGeometry(16, 56, 6), new THREE.MeshBasicMaterial({ color: 0xffffff }));
      mesh.visible = false;
      groups.truth.add(mesh);
      intActors.push({ flt, mesh, line });
    }

    // Intercept & Leaker Bursts
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

    // Shrapnel ground impacts
    const catCol = { building: 0xff2e4c, road: 0xff9e3d, protected_zone: 0xf065d8, open: 0x58e08e };
    for (const imp of (run.truth.shrapnelImpacts || []).slice(0, 600)) {
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

    document.getElementById('kpi-grid').innerHTML = `
      <div class="kpi"><div class="k">Defense Score</div><div class="v ${ev.compositeScore >= 75 ? 'good' : ev.compositeScore >= 45 ? 'warn' : 'bad'}">${ev.compositeScore.toFixed(1)}/100</div></div>
      <div class="kpi"><div class="k">Lethal Killed / Leakers</div><div class="v ${ev.leakers === 0 ? 'good' : 'bad'}">${ev.lethalIntercepted}/${ev.lethalThreats} (${ev.leakers} leak)</div></div>
      <div class="kpi"><div class="k">Interceptor Spend</div><div class="v cyan">$${ev.totalCostKUsd.toLocaleString()}k (${ev.shotsFired} shots)</div></div>
      <div class="kpi"><div class="k">Decoy Waste / Air Viol.</div><div class="v ${ev.decoyShotsWasted === 0 && ev.airspaceViolations === 0 ? 'good' : 'warn'}">${ev.decoyShotsWasted} decoy · ${ev.airspaceViolations} air</div></div>
      <div class="kpi"><div class="k">Debris Bldgs / WorldPop</div><div class="v">${ev.debrisBuildingHits} bldg · ~${ev.estimatedPopulationExposed} pop</div></div>
      <div class="kpi"><div class="k">Road Blocked / Detour</div><div class="v">${ev.roadImpact.blockedRoadKm.toFixed(2)}km (+${Math.round(ev.roadImpact.emergencyDetourDelayS || 0)}s)</div></div>
    `;

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
    document.getElementById('policy-table').innerHTML = rows.join('');

    const evHtml = (run.events || [])
      .filter((x) => x.kind !== 'launch')
      .slice(0, 40)
      .map((x) => {
        const cls = x.kind === 'intercept_kill' ? 'good' : x.kind === 'leaker_impact' ? 'bad' : 'warn';
        return `<div class="event-item"><span class="cyan">t=${x.tS}s</span> <span class="${cls}">${x.kind.replace('_', ' ')}</span> ${x.threatId || ''} ${x.batteryId ? `← ${x.batteryId}` : ''}</div>`;
      })
      .join('');
    document.getElementById('event-log').innerHTML = evHtml || '<span style="color:var(--muted);">No terminal events.</span>';

    maxTime = Math.max(25, ...((run.truth.threats || []).map((t) => t.endS + 6)));
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
    const batts = run.evaluation.batteries || [];
    document.getElementById('magazine-bars').innerHTML = batts.map((b) => {
      const rem = curMags ? (curMags[b.id] ?? b.magazineRemaining) : b.magazineInitial;
      const pct = Math.round((rem / Math.max(1, b.magazineInitial)) * 100);
      return `
        <div class="mag-card">
          <div class="mag-head"><span style="color:${b.color};">${b.label}</span><span>${rem}/${b.magazineInitial}</span></div>
          <div class="mag-bar"><div class="mag-fill" style="width:${pct}%;background:${b.color};"></div></div>
          <div class="mag-meta"><span>$${b.costKUsd}k/shot</span><span>${b.shotsFired} fired · ${b.kills} kills</span></div>
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

  async function loadBundle(ep, resetCam = true) {
    stageEl.textContent = 'Fetching 3D Salvo replay bundle…';
    replay = await (await fetch(ep.replayUrl, { cache: 'no-store' })).json();
    await loadEnvGeometry(replay, ep);

    document.getElementById('scenario-title').textContent = replay.scenario.title || 'Multi-Threat Salvo';
    document.getElementById('env-badges').innerHTML =
      `<span class="badge">${replay.scenario.threats.length} threats</span>` +
      `<span class="badge">${replay.scenario.batteries.length} batteries</span>` +
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
    polBtns.querySelectorAll('button').forEach((b) => { b.onclick = () => buildPolicyScene(b.dataset.policy); });

    const sd = replay.sourceDraft || {};
    document.getElementById('wi-decoys').value = String(sd.raid?.decoys ?? 3);
    document.getElementById('wi-uavs').value = String(sd.raid?.uavs ?? 5);
    document.getElementById('wi-cruise').value = String(sd.raid?.cruiseMissiles ?? 2);
    document.getElementById('wi-ballistic').value = String(sd.raid?.ballisticMissiles ?? 1);
    document.getElementById('wi-vis').value = String(sd.visibilityKm ?? 10);
    document.getElementById('wi-seed').value = String(sd.seed ?? replay.scenario.seed ?? 20261127);

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
    playing = !playing;
    playBtn.textContent = playing ? '⏸ Pause' : '▶ Play';
    playBtn.classList.toggle('active', playing);
  };
  playBtn.onclick = togglePlay;
  document.getElementById('time-slider').oninput = (e) => {
    playing = false;
    playBtn.textContent = '▶ Play';
    playBtn.classList.remove('active');
    simTime = Number(e.target.value);
    updateAtTime(simTime);
  };
  document.getElementById('speed-select').onchange = (e) => { playSpeed = Number(e.target.value) || 1; };
  document.querySelectorAll('#dock button[data-cam]').forEach((b) => { b.onclick = () => moveCamera(b.dataset.cam); });

  document.getElementById('run-switcher').onchange = async (e) => {
    const url = new URL(window.location.href);
    url.searchParams.set('run', e.target.value);
    window.history.replaceState({}, '', url.toString());
    overlayEl.classList.remove('hidden');
    await loadBundle(await resolveEndpoint(stageEl), false);
  };

  document.getElementById('btn-whatif-run').onclick = async () => {
    if (!replay) return;
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
      const res = await fetch('/api/runs', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(base) });
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
    if (playing && replay) {
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
