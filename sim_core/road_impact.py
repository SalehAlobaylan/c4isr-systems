"""OSM Road-Network Disruption & Emergency-Response Detour Analyzer.

Builds a lightweight topologically snapped road graph from an
``EnvironmentModel``'s retained OSM road geometries and evaluates how falling
hazardous intercept debris affects arterial mobility and emergency-response
routing (pure-Python ``heapq`` Dijkstra, zero external graph dependencies).
"""
from __future__ import annotations

from dataclasses import dataclass
import heapq
import math
from typing import Any

import numpy as np
from shapely.geometry import Point

from .environment import EnvironmentModel

# Conservative urban speed assumptions (m/s) by OSM highway class (explicitly derived)
ROAD_SPEED_MPS: dict[str, float] = {
    "motorway": 27.8,
    "trunk": 22.2,
    "primary": 16.7,
    "secondary": 13.9,
    "tertiary": 11.1,
    "residential": 8.3,
    "unclassified": 9.7,
}


@dataclass(frozen=True)
class _RoadEdge:
    edge_id: int
    u: int
    v: int
    length_m: float
    travel_s: float
    source_id: str
    highway: str
    name: str
    mid_e: float
    mid_n: float
    p0: tuple[float, float]
    p1: tuple[float, float]


class RoadImpactAnalyzer:
    """Fast OSM road disruption & emergency detour scorer built directly from EnvironmentModel."""

    def __init__(self, env: EnvironmentModel, *, snap_m: float = 28.0) -> None:
        self.env = env
        self.snap_m = float(snap_m)
        self.nodes: list[tuple[float, float]] = []
        self.edges: list[_RoadEdge] = []
        self.adj: dict[int, list[tuple[int, int]]] = {}  # node -> [(neighbor, edge_id)]
        self._node_grid: dict[tuple[int, int], int] = {}
        self._build_graph()

    def _get_or_add_node(self, e: float, n: float) -> int:
        key = (int(round(e / self.snap_m)), int(round(n / self.snap_m)))
        existing = self._node_grid.get(key)
        if existing is not None:
            return existing
        nid = len(self.nodes)
        self.nodes.append((e, n))
        self._node_grid[key] = nid
        self.adj[nid] = []
        return nid

    def _build_graph(self) -> None:
        eid = 0
        for road in self.env.roads:
            geom = road.geometry_enu
            if geom.geom_type == "LineString":
                lines = [geom]
            elif geom.geom_type == "MultiLineString":
                lines = list(geom.geoms)
            else:
                continue
            hw = str(road.highway or "unclassified")
            name = str(getattr(road, "name", "") or "")
            spd = ROAD_SPEED_MPS.get(hw, 10.0)
            for line in lines:
                coords = list(line.coords)
                if len(coords) < 2:
                    continue
                # Collapse intermediate vertices along straight roads while keeping endpoints & ~180m hops
                prev_e, prev_n = float(coords[0][0]), float(coords[0][1])
                u = self._get_or_add_node(prev_e, prev_n)
                acc_len = 0.0
                for k in range(1, len(coords)):
                    ce, cn = float(coords[k][0]), float(coords[k][1])
                    seg = math.hypot(ce - prev_e, cn - prev_n)
                    acc_len += seg
                    is_last = k == len(coords) - 1
                    if is_last or acc_len >= 180.0:
                        v = self._get_or_add_node(ce, cn)
                        if u != v and acc_len >= 4.0:
                            pe, pn = self.nodes[u]
                            qe, qn = self.nodes[v]
                            edge = _RoadEdge(
                                edge_id=eid,
                                u=u,
                                v=v,
                                length_m=acc_len,
                                travel_s=acc_len / spd,
                                source_id=road.source_id,
                                highway=hw,
                                name=name,
                                mid_e=0.5 * (pe + qe),
                                mid_n=0.5 * (pn + qn),
                                p0=(pe, pn),
                                p1=(qe, qn),
                            )
                            self.edges.append(edge)
                            self.adj[u].append((v, eid))
                            self.adj[v].append((u, eid))
                            eid += 1
                            u = v
                            acc_len = 0.0
                    prev_e, prev_n = ce, cn

        self._edge_mid_e = np.array([e.mid_e for e in self.edges], dtype=np.float64) if self.edges else np.zeros(0)
        self._edge_mid_n = np.array([e.mid_n for e in self.edges], dtype=np.float64) if self.edges else np.zeros(0)
        self._node_e = np.array([p[0] for p in self.nodes], dtype=np.float64) if self.nodes else np.zeros(0)
        self._node_n = np.array([p[1] for p in self.nodes], dtype=np.float64) if self.nodes else np.zeros(0)

        # Keep largest connected component anchor nodes for emergency response routing
        self._main_nodes = self._largest_component()

    def _largest_component(self) -> list[int]:
        if not self.nodes:
            return []
        visited = [False] * len(self.nodes)
        best: list[int] = []
        for start in range(len(self.nodes)):
            if visited[start] or not self.adj.get(start):
                continue
            comp = []
            stack = [start]
            visited[start] = True
            while stack:
                u = stack.pop()
                comp.append(u)
                for nb, _ in self.adj[u]:
                    if not visited[nb]:
                        visited[nb] = True
                        stack.append(nb)
            if len(comp) > len(best):
                best = comp
        return best

    def _nearest_main_node(self, east: float, north: float) -> int | None:
        if not self._main_nodes:
            return None
        idx = np.asarray(self._main_nodes, dtype=np.int32)
        d2 = (self._node_e[idx] - east) ** 2 + (self._node_n[idx] - north) ** 2
        return int(idx[int(np.argmin(d2))])

    def _shortest_path_s(self, start: int, goal: int, blocked_edges: set[int]) -> float | None:
        if start == goal:
            return 0.0
        dist: dict[int, float] = {start: 0.0}
        heap: list[tuple[float, int]] = [(0.0, start)]
        while heap:
            d, u = heapq.heappop(heap)
            if u == goal:
                return d
            if d > dist.get(u, float("inf")) + 1e-6:
                continue
            for nb, eid in self.adj[u]:
                if eid in blocked_edges:
                    continue
                nd = d + self.edges[eid].travel_s
                if nd < dist.get(nb, float("inf")):
                    dist[nb] = nd
                    heapq.heappush(heap, (nd, nb))
        return None

    def evaluate_debris_impact(
        self,
        impact_east: np.ndarray,
        impact_north: np.ndarray,
        hazardous_mask: np.ndarray | None = None,
        *,
        block_radius_m: float = 36.0,
        protected_center: tuple[float, float] | None = None,
    ) -> dict[str, Any]:
        """Compute blocked road segments, km, and emergency-response detour time from debris impacts."""
        if not self.edges or len(impact_east) == 0:
            return {
                "derivation": "derived/osm-road-graph-dijkstra",
                "blockedEdgeCount": 0,
                "blockedRoadKm": 0.0,
                "affectedArterials": [],
                "baselineEmergencyS": None,
                "detourEmergencyS": None,
                "emergencyDetourDelayS": 0.0,
                "routeSevered": False,
            }

        e_arr = np.asarray(impact_east, dtype=np.float64).ravel()
        n_arr = np.asarray(impact_north, dtype=np.float64).ravel()
        if hazardous_mask is not None:
            hz = np.asarray(hazardous_mask, dtype=bool).ravel()
            e_arr = e_arr[hz]
            n_arr = n_arr[hz]

        if len(e_arr) == 0:
            return {
                "derivation": "derived/osm-road-graph-dijkstra",
                "blockedEdgeCount": 0,
                "blockedRoadKm": 0.0,
                "affectedArterials": [],
                "baselineEmergencyS": None,
                "detourEmergencyS": None,
                "emergencyDetourDelayS": 0.0,
                "routeSevered": False,
            }

        # Bounding-box prefilter + fast distance check to edge midpoints
        pad = float(block_radius_m) + 120.0
        emin, emax = float(np.min(e_arr)) - pad, float(np.max(e_arr)) + pad
        nmin, nmax = float(np.min(n_arr)) - pad, float(np.max(n_arr)) + pad
        cand_idx = np.nonzero(
            (self._edge_mid_e >= emin)
            & (self._edge_mid_e <= emax)
            & (self._edge_mid_n >= nmin)
            & (self._edge_mid_n <= nmax)
        )[0]

        blocked: set[int] = set()
        r2 = float(block_radius_m) ** 2
        for ei in cand_idx:
            ed = self.edges[int(ei)]
            # Point-to-segment distance squared across all hazardous impacts
            x0, y0 = ed.p0
            x1, y1 = ed.p1
            dx, dy = x1 - x0, y1 - y0
            l2 = dx * dx + dy * dy
            if l2 < 1e-6:
                d2 = (e_arr - x0) ** 2 + (n_arr - y0) ** 2
            else:
                t = np.clip(((e_arr - x0) * dx + (n_arr - y0) * dy) / l2, 0.0, 1.0)
                d2 = (e_arr - (x0 + t * dx)) ** 2 + (n_arr - (y0 + t * dy)) ** 2
            if float(np.min(d2)) <= r2:
                blocked.add(int(ei))

        blocked_km = sum(self.edges[ei].length_m for ei in blocked) / 1000.0
        arterials = sorted(
            {
                self.edges[ei].name or self.edges[ei].highway
                for ei in blocked
                if self.edges[ei].highway in ("motorway", "trunk", "primary", "secondary")
            }
        )[:8]

        # Emergency-response corridor test: nearest node to SW depot -> protected zone / debris centroid
        cx = float(np.mean(e_arr))
        cy = float(np.mean(n_arr))
        target_e, target_n = protected_center if protected_center is not None else (cx, cy)
        span_e = self.env.east_max - self.env.east_min
        span_n = self.env.north_max - self.env.north_min

        # Place emergency origin across the debris zone from the target so debris genuinely forces detours
        dir_e = cx - target_e
        dir_n = cy - target_n
        d_mag = math.hypot(dir_e, dir_n)
        if d_mag < 100.0:
            dir_e, dir_n, d_mag = -1.0, -1.0, math.sqrt(2.0)
        origin_e = cx + (dir_e / d_mag) * min(1800.0, 0.30 * span_e)
        origin_n = cy + (dir_n / d_mag) * min(1800.0, 0.30 * span_n)

        start_node = self._nearest_main_node(origin_e, origin_n)
        goal_node = self._nearest_main_node(target_e, target_n)

        base_s = None
        detour_s = None
        delay_s = 0.0
        severed = False
        if start_node is not None and goal_node is not None and start_node != goal_node:
            base_s = self._shortest_path_s(start_node, goal_node, set())
            if base_s is not None:
                if blocked:
                    detour_s = self._shortest_path_s(start_node, goal_node, blocked)
                    if detour_s is None:
                        severed = True
                        delay_s = round(max(180.0, base_s * 1.5), 1)
                    else:
                        delay_s = round(max(0.0, detour_s - base_s), 1)
                else:
                    detour_s = base_s

        return {
            "derivation": "derived/osm-road-graph-dijkstra",
            "blockedEdgeCount": len(blocked),
            "blockedRoadKm": round(blocked_km, 3),
            "affectedArterials": arterials,
            "baselineEmergencyS": round(base_s, 1) if base_s is not None else None,
            "detourEmergencyS": round(detour_s, 1) if detour_s is not None else None,
            "emergencyDetourDelayS": delay_s,
            "routeSevered": severed,
        }


def get_road_analyzer(env: EnvironmentModel) -> RoadImpactAnalyzer:
    """Lazy-cached per-environment road graph builder."""
    cached = getattr(env, "_road_impact_analyzer", None)
    if cached is None:
        cached = RoadImpactAnalyzer(env)
        setattr(env, "_road_impact_analyzer", cached)
    return cached
