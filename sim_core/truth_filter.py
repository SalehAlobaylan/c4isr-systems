"""Wire-level EMAD 4-partition projection filter (``EXERCISE_OBSERVATION`` vs ``EXERCISE_TRUTH``).

Ensures that when a replay bundle is requested with ``view=operator``, every
``EXERCISE_TRUTH`` and ``EVALUATION`` field (true threat trajectories, decoy
identities, radar LOS ground-truth masks, sealed shrapnel impacts, and hidden
authored raid specs) is stripped *before* bytes leave the server.
"""
from __future__ import annotations

import copy
from typing import Any


OPERATOR_VISIBLE_SALVO_EVENTS = frozenset({"interceptor_launch", "airspace_hold_fire", "operator_command"})


def filter_sim_replay(replay: dict[str, Any], *, view: str = "debrief") -> dict[str, Any]:
    """Project an Intercept & Shrapnel replay bundle for ``operator`` or ``debrief`` scope."""
    if view != "operator":
        out = dict(replay)
        out["viewScope"] = "EXERCISE_TRUTH"
        out["truthRedacted"] = False
        return out

    out = copy.deepcopy(replay)
    out["viewScope"] = "EXERCISE_OBSERVATION"
    out["truthRedacted"] = True
    out["comparison"] = None

    if isinstance(out.get("sourceDraft"), dict):
        out["sourceDraft"].pop("threat", None)

    runs = out.get("runs")
    if isinstance(runs, dict):
        for pol, run in list(runs.items()):
            if not isinstance(run, dict):
                continue
            truth = run.pop("truth", None) or {}
            # Own-force interceptor telemetry is permitted; threat truth & shrapnel are sealed
            own_interceptor = truth.get("interceptorTrajectory") or []
            run["ownForceTelemetry"] = {
                "scope": "EXERCISE_OBSERVATION",
                "interceptorTrajectory": own_interceptor,
            }
            run["threatKind"] = "unknown_contact"
            run["evaluation"] = None
            tl = run.get("timelineEvents")
            if isinstance(tl, dict):
                tl.pop("firstClearLosS", None)
                tl.pop("interceptS", None)
                tl.pop("lastImpactS", None)
            runs[pol] = run
    return out


def filter_salvo_replay(replay: dict[str, Any], *, view: str = "debrief") -> dict[str, Any]:
    """Project a Multi-Threat Salvo replay bundle for ``operator`` or ``debrief`` scope."""
    if view != "operator":
        out = dict(replay)
        out["viewScope"] = "EXERCISE_TRUTH"
        out["truthRedacted"] = False
        return out

    out = copy.deepcopy(replay)
    out["viewScope"] = "EXERCISE_OBSERVATION"
    out["truthRedacted"] = True
    out["comparison"] = None

    if isinstance(out.get("scenario"), dict):
        out["scenario"].pop("threats", None)
    if isinstance(out.get("sourceDraft"), dict):
        out["sourceDraft"].pop("raid", None)

    runs = out.get("runs")
    if isinstance(runs, dict):
        for pol, run in list(runs.items()):
            if not isinstance(run, dict):
                continue
            truth = run.pop("truth", None) or {}
            # Own-force battery launches stay visible; true kill/miss outcomes stay sealed
            own_ints = []
            for f in truth.get("interceptors") or []:
                own_ints.append(
                    {
                        "id": f.get("id"),
                        "batteryId": f.get("batteryId"),
                        "batteryTier": f.get("batteryTier"),
                        "trackId": f.get("trackId"),
                        "launchS": f.get("launchS"),
                        "endS": f.get("endS"),
                        "costKUsd": f.get("costKUsd"),
                        "samples": f.get("samples") or [],
                    }
                )
            own_batts = []
            ev_obj = run.get("evaluation") or {}
            if isinstance(ev_obj, dict):
                for b in ev_obj.get("batteries") or []:
                    if isinstance(b, dict):
                        own_batts.append(
                            {
                                "id": b.get("id"),
                                "tier": b.get("tier"),
                                "label": b.get("label"),
                                "color": b.get("color"),
                                "enu": b.get("enu"),
                                "maxRangeM": b.get("maxRangeM"),
                                "magazineInitial": b.get("magazineInitial"),
                                "magazineRemaining": b.get("magazineRemaining"),
                                "shotsFired": b.get("shotsFired"),
                                "costKUsd": b.get("costKUsd"),
                            }
                        )
            run["ownForceTelemetry"] = {
                "scope": "EXERCISE_OBSERVATION",
                "interceptors": own_ints,
                "batteries": own_batts,
            }
            run["evaluation"] = None
            raw_events = run.get("events") or []
            run["events"] = [ev for ev in raw_events if isinstance(ev, dict) and ev.get("kind") in OPERATOR_VISIBLE_SALVO_EVENTS]
            runs[pol] = run
    return out
