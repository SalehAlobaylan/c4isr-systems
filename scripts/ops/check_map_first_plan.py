"""Read-only checks for the map-first plan; no application qualification implied."""

import argparse
import copy
import hashlib
import json
from pathlib import Path
import re
from urllib.parse import unquote


ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT / "docs/implementation"
LEDGER = BASE / "MAP_FIRST_BACKLOG_2026_10_25.json"
# The gap report, gap analysis, implementation plan and plan audit were merged
# into one record on 11 November 2026. Each original remains a section.
PLAN = BASE / "MAP_FIRST_PLAN.md"
REPORT = PLAN
AUDIT = PLAN
ANALYSIS = PLAN


def ids(prefix, count):
    return {f"{prefix}{number:02}" for number in range(1, count + 1)}


def expand_tasks(value):
    found = set(re.findall(r"T\d{2}", value))
    for first, last in re.findall(r"T(\d{2})[–-]T(\d{2})", value):
        found.update(f"T{n:02}" for n in range(int(first), int(last) + 1))
    return found


def plan_section(plan, ident):
    """Return the body of a plan section, tolerating the merged heading depth."""
    match = re.search(r"^#{3,4} %s — .*$" % ident, plan, re.M)
    if not match:
        return ""
    rest = plan[match.end():]
    following = re.search(r"^#{3,4} ", rest, re.M)
    return rest[:following.start()] if following else rest


def validate(data, plan, check_files=True):
    errors = []

    def need(condition, message):
        if not condition:
            errors.append(message)

    def indexed(field):
        rows = data[field]
        result = {row["id"]: row for row in rows}
        need(len(rows) == len(result), f"Duplicate IDs: {field}")
        return result

    tasks = indexed("tasks")
    gaps = indexed("gapCoverage")
    scenarios = indexed("acceptanceScenarios")
    requirements = indexed("requirements")
    hardening = indexed("hardeningChecks")
    need(data["schema"] == "emad.map-first-backlog.v2", "Unknown backlog schema")
    need(data["status"] in {"PLANNED", "IN_PROGRESS", "COMPLETE"}, "Invalid overall status")
    need(set(tasks) == ids("T", 24), "Expected T01–T24")
    need(set(gaps) == ids("GAP-", 18), "Expected GAP-01–GAP-18")
    need(set(scenarios) == ids("A", 24), "Expected A01–A24")
    need(set(hardening) == ids("H", 12), "Expected H01–H12")
    need(set(requirements) == {f"R{n:02}.{k}" for n in range(1, 19) for k in range(1, 4)},
         "Expected all 54 atomic requirements")
    # The plan was merged into a larger record, so its own headings sit one
    # level deeper than the standalone document used to carry them.
    headings = re.findall(r"^#{3,4} (T\d{2}) — (.+)$", plan, re.M)
    acceptance = re.findall(r"^#{3,4} (A\d{2}) — (.+)$", plan, re.M)
    need([i for i, _ in headings] == sorted(tasks), "Plan task headings/order drift")
    need([i for i, _ in acceptance] == sorted(scenarios), "Plan scenario headings/order drift")
    for tid, title in headings:
        if tid not in tasks:
            continue
        task = tasks[tid]
        section = plan_section(plan, tid)
        fields = re.search(r"\*\*Gate:\*\* (.*?) \*\*Depends on:\*\* (.*?)\. "
                           r"\*\*Gaps:\*\* (.*?)\. \*\*Acceptance:\*\* ([^\n]+)", section)
        need(fields is not None, f"Missing task contract: {tid}")
        need(task["title"] == title, f"Task title drift: {tid}")
        if fields:
            need(task["gate"] in re.findall(r"MAP-\d", fields[1]), f"Gate drift: {tid}")
            need(set(task["dependsOn"]) == expand_tasks(fields[2]), f"Dependency drift: {tid}")
            need(set(task["gapIds"]) == {"GAP-" + n for n in re.findall(r"\d{2}", fields[3])},
                 f"Gap drift: {tid}")
            need(set(task["acceptanceIds"]) == set(re.findall(r"A\d{2}", fields[4])),
                 f"Acceptance drift: {tid}")
    for aid, title in acceptance:
        if aid in scenarios:
            need(scenarios[aid]["title"] == title, f"Scenario title drift: {aid}")

    visiting, visited = set(), set()

    def visit(tid):
        if tid in visiting:
            errors.append(f"Dependency cycle: {tid}")
            return
        if tid in visited or tid not in tasks:
            return
        visiting.add(tid)
        for parent in tasks[tid]["dependsOn"]:
            need(parent in tasks, f"Unknown dependency: {tid}/{parent}")
            visit(parent)
        visiting.remove(tid)
        visited.add(tid)

    for tid, task in tasks.items():
        visit(tid)
        need(set(task["gapIds"]) <= set(gaps) and bool(task["gapIds"]), f"Invalid gaps: {tid}")
        need(set(task["acceptanceIds"]) <= set(scenarios) and bool(task["acceptanceIds"]),
             f"Invalid acceptance: {tid}")
        need(task["status"] in {"PLANNED", "IN_PROGRESS", "IMPLEMENTED", "BLOCKED", "COMPLETE"}, f"Task status: {tid}")
        if task["status"] == "IMPLEMENTED":
            need(bool(task["evidence"].get("implementation")), f"Implemented without change evidence: {tid}")
        if task["status"] == "COMPLETE":
            need(all(tasks.get(d, {}).get("status") == "COMPLETE" for d in task["dependsOn"]),
                 f"Incomplete prerequisite: {tid}")
            need(bool(task.get("owner")) and bool(task.get("evidenceReviewer")), f"Unassigned completed task: {tid}")
            for dimension in ("implementation", "installation", "functional", "geographic", "userValidation"):
                entries = task["evidence"].get(dimension, [])
                need(bool(entries), f"Missing completed-task evidence: {tid}/{dimension}")
                for entry in entries:
                    valid = isinstance(entry, dict) and bool(entry.get("reference"))
                    valid = valid and (entry.get("outcome") == "PASS" or
                                       entry.get("outcome") == "NOT_APPLICABLE" and bool(entry.get("reason")))
                    need(valid, f"Invalid evidence: {tid}/{dimension}")
    for tid, dependency in (("T13", "T10"), ("T23", "T22")):
        need(dependency in tasks.get(tid, {}).get("dependsOn", []), f"Missing hardened dependency: {tid}/{dependency}")

    mapped_scenarios = {a for t in tasks.values() for a in t["acceptanceIds"]}
    need(mapped_scenarios == set(scenarios), "Unowned acceptance scenario")
    for gid, gap in gaps.items():
        need(gap["status"] in {"OPEN", "IN_PROGRESS", "BLOCKED", "CLOSED"}, f"Gap status: {gid}")
        owners = {tid for tid, task in tasks.items() if gid in task["gapIds"]}
        need(owners == set(gap["tasks"]) and bool(owners), f"Reciprocal gap drift: {gid}")
        need(any(r["gapId"] == gid for r in requirements.values()), f"No requirements: {gid}")
        if gap["status"] == "CLOSED":
            need(all(tasks.get(t, {}).get("status") == "COMPLETE" for t in gap["tasks"]),
                 f"Premature gap closure: {gid}")
            required = {a for r in requirements.values() if r["gapId"] == gid for a in r["acceptanceIds"]}
            need(all(scenarios.get(a, {}).get("status") == "PASS" for a in required),
                 f"Unqualified gap closure: {gid}")
    for row in [*requirements.values(), *hardening.values()]:
        need(bool(row["tasks"]) and set(row["tasks"]) <= set(tasks), f"Invalid owners: {row['id']}")
        need(bool(row["acceptanceIds"]) and set(row["acceptanceIds"]) <= set(scenarios),
             f"Invalid proof IDs: {row['id']}")
        owned = {a for t in row["tasks"] if t in tasks for a in tasks[t]["acceptanceIds"]}
        need(set(row["acceptanceIds"]) <= owned, f"Proof not owned by mapped tasks: {row['id']}")
        if "gapId" in row:
            need(row["gapId"] in gaps, f"Unknown requirement gap: {row['id']}")
            need(all(row["gapId"] in tasks.get(t, {}).get("gapIds", []) for t in row["tasks"]),
                 f"Requirement/task gap mismatch: {row['id']}")
    matrix = re.findall(r"^\| (GAP-\d{2}) .*?\| (P\d) \| (.*?) \| (.*?) \|$", plan, re.M)
    need(len(matrix) == 18, "Missing plan traceability rows")
    for gid, priority, refs, proof in matrix:
        need(gid in gaps and gaps[gid]["priority"] == priority, f"Priority drift: {gid}")
        need(gid in gaps and expand_tasks(refs) == set(gaps[gid]["tasks"]), f"Matrix drift: {gid}")
        need(set(re.findall(r"A\d{2}", proof)) <= set(scenarios), f"Unknown matrix evidence: {gid}")
    for aid, scenario in scenarios.items():
        need(scenario["mandatory"] is True, f"Mandatory scenario weakened: {aid}")
        need(scenario["status"] in {"NOT_RUN", "PASS", "FAIL", "BLOCKED"}, f"Scenario status: {aid}")
        if scenario["status"] == "PASS":
            candidate = data["releaseCandidate"]["identity"]
            need(bool(candidate), f"Pass without candidate identity: {aid}")
            complete = [e for e in scenario["evidence"] if isinstance(e, dict) and
                        e.get("outcome") == "PASS" and e.get("reference") and
                        e.get("candidateIdentity") == candidate and candidate]
            need(bool(complete), f"Pass without complete candidate-bound evidence: {aid}")
    for prerequisite in data["externalPrerequisites"]:
        need(bool(prerequisite["ownerRole"]) and set(prerequisite["tasks"]) <= set(tasks),
             f"Invalid prerequisite: {prerequisite['id']}")
        need(prerequisite["status"] in {"NOT_CONFIRMED", "BLOCKED", "SATISFIED"},
             f"Prerequisite status: {prerequisite['id']}")
    need(data["releaseCandidate"]["status"] in {"NOT_QUALIFIED", "IN_QUALIFICATION", "QUALIFIED"},
         "Invalid release status")
    if data["status"] == "COMPLETE":
        need(data["releaseCandidate"]["status"] == "QUALIFIED", "Completed backlog lacks qualified candidate")
    if data["releaseCandidate"]["status"] == "QUALIFIED":
        need(bool(data["releaseCandidate"]["identity"]) and bool(data["releaseCandidate"]["evidence"]),
             "Qualified release lacks candidate/evidence")
        need(all(t["status"] == "COMPLETE" for t in tasks.values()), "Qualified release has incomplete tasks")
        need(all(s["status"] == "PASS" for s in scenarios.values()), "Qualified release has unpassed scenarios")
        need(all(g["status"] == "CLOSED" for g in gaps.values()), "Qualified release has open gaps")
        need(all(e["status"] == "SATISFIED" for e in data["externalPrerequisites"]), "Qualified release has unresolved prerequisites")
    if check_files:
        expected = {"../../EMAD_CURRENT_SCOPE.md", REPORT.name}
        need({x["path"] for x in data["planningSources"]} == expected, "Missing governing source fingerprint")
        for source in data["planningSources"]:
            path = BASE / source["path"]
            need(path.is_file(), f"Missing planning source: {path}")
            if path.is_file():
                digest = hashlib.sha256(path.read_text(encoding="utf-8").encode()).hexdigest()
                need(digest == source["sha256"], f"Planning source changed; review required: {source['path']}")
        report_gaps = dict(re.findall(r"^#{3,4} (GAP-\d{2}) .*? — (P\d)$", REPORT.read_text(encoding="utf-8"), re.M))
        need(report_gaps == {gid: gap["priority"] for gid, gap in gaps.items()}, "Original report gap/priority drift")
        for key in ("plan", "governingScope", "audit"):
            need((BASE / data[key]).is_file(), f"Missing ledger document: {key}")
        for path in (PLAN, ANALYSIS):
            need(path.is_file(), f"Missing document: {path}")
            if not path.is_file():
                continue
            for link in re.findall(r"\[[^\]]*\]\(([^)]+)\)", path.read_text(encoding="utf-8")):
                target = unquote(link.split("#", 1)[0].strip("<>"))
                if not target or re.match(r"^[a-z]+://", target):
                    continue
                need((path.parent / re.sub(r":\d+$", "", target)).exists(), f"Broken local link: {path.name}: {link}")
    return errors


def self_test(data, plan):
    faults = {
        "missing_gap": lambda d: d["gapCoverage"].pop(),
        "cycle": lambda d: d["tasks"][0]["dependsOn"].append("T24"),
        "unknown_dependency": lambda d: d["tasks"][0]["dependsOn"].append("T99"),
        "missing_requirement": lambda d: d["requirements"].pop(),
        "unknown_acceptance": lambda d: d["requirements"][0]["acceptanceIds"].append("A99"),
        "title_drift": lambda d: d["tasks"][0].update(title="Wrong title"),
        "unsafe_order": lambda d: d["tasks"][12]["dependsOn"].remove("T10"),
        "weakened_gate": lambda d: d["acceptanceScenarios"][0].update(mandatory=False),
        "false_pass": lambda d: d["acceptanceScenarios"][0].update(status="PASS"),
        "false_completion": lambda d: d["tasks"][0].update(status="COMPLETE"),
        "false_release": lambda d: d["releaseCandidate"].update(status="QUALIFIED"),
        "source_drift": lambda d: d["planningSources"][0].update(sha256="0" * 64),
    }
    missed = []
    for name, mutate in faults.items():
        broken = copy.deepcopy(data)
        mutate(broken)
        if not validate(broken, plan):
            missed.append(name)
    return len(faults), missed


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--self-test", action="store_true", help="Inject broken planning states in memory")
    args = parser.parse_args()
    try:
        data = json.loads(LEDGER.read_text(encoding="utf-8"))
        plan = PLAN.read_text(encoding="utf-8")
        errors = validate(data, plan)
        result = {"status": "FAIL" if errors else "PASS", "tasks": len(data["tasks"]),
                  "gaps": len(data["gapCoverage"]), "requirements": len(data["requirements"]),
                  "acceptanceScenarios": len(data["acceptanceScenarios"]),
                  "hardeningChecks": len(data["hardeningChecks"]),
                  "qualification": "Planning consistency only; this checker does not execute application acceptance.", "errors": errors}
        if args.self_test and not errors:
            count, missed = self_test(data, plan)
            result["faultsInjected"] = count
            result["faultsMissed"] = missed
            if missed:
                errors.append("Validator missed injected faults: " + ", ".join(missed))
                result["status"] = "FAIL"
        print(json.dumps(result, indent=2))
        return int(bool(errors))
    except (KeyError, TypeError, ValueError, OSError) as error:
        print(json.dumps({"status": "FAIL", "errors": [str(error)]}, indent=2))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
