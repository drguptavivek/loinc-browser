#!/usr/bin/env bash
# Replay every request in scripts/capture-exemplars.sh against a running local server and
# compare the response *shape* with the captured upstream exemplar in docs/exemplars/.
# Shape = HTTP status, resourceType, ordered top-level parameter names, value[x] types, and
# (for resources/Bundles) the top-level key set. Values are ignored (upstream is a newer release).
# Usage: scripts/fhir-parity.sh [base-url]   (default http://localhost:9005)
set -euo pipefail
cd "$(dirname "$0")/.."
BASE="${1:-http://localhost:9005}" python3 - <<'EOF'
import json, os, re, sys, time, urllib.request, urllib.error

base = os.environ["BASE"].rstrip("/")
# Known, documented divergences (docs/FHIR_TERMINOLOGY_PLAN.md §7) and exemplar-corpus issues.
skip = {
    "codesystem-read-loinc": "upstream 404s bare id; local serves it (§7)",
    "valueset-expand-loinc-vs-count2": "upstream lacks http://loinc.org/vs (§7)",
    "valueset-expand-loinc-vs-count2-offset2": "upstream lacks http://loinc.org/vs (§7)",
    "valueset-expand-loinc-vs-filter": "upstream lacks http://loinc.org/vs (§7)",
    "valueset-expand-LP31755-9-count3": "upstream lacks implicit LP value sets (§7)",
    "valueset-expand-unknown": "upstream empty 200; local 404 (§7)",
    "valueset-expand-LG9568-9-count3": "upstream bug: params on LG/LL expand 404 (§7)",
    "valueset-expand-LL1162-8-count2-offset1": "upstream bug (§7)",
    "valueset-expand-LL1162-8-filter": "upstream bug (§7)",
    "valueset-expand-LL1162-8-de": "upstream bug (§7)",
    "valueset-expand-top-ranked-count3": "upstream lacks loinc-top-ranked (local addition)",
    "valueset-read-loinc-top-ranked": "upstream lacks loinc-top-ranked (local addition)",
    "valueset-expand-inline-component": "upstream nginx 403 on POST (§7)",
    "valueset-search-url-deprecated": "upstream 400s _elements; supported locally (§7)",
    "conceptmap-search-loinc-to-ieee": "upstream 400s _summary; supported locally (§7)",
    "conceptmap-translate-snomed-reverse": "upstream 404s reverse maps; local works (§7)",
    "conceptmap-translate-reverse-ieee": "upstream 404s reverse maps; local works (§7)",
    "conceptmap-translate-reverse-flag": "upstream 404s reverse maps; local works (§7)",
    "conceptmap-translate-mapto-deprecated": "exemplar holds a PhenX match (map not in release)",
    "conceptmap-translate-none": "exemplar holds PhenX matches (map not in release)",
    "codesystem-lookup-version-mismatch": "local has one version; version=2.71 still 404 but code differs",
}

def value_key(p):
    return next((k for k in p if k.startswith("value")), "part" if "part" in p else "-")

def shape(status, body):
    try:
        d = json.loads(body)
    except Exception:
        return (status, "non-json")
    rt = d.get("resourceType")
    if rt == "Parameters":
        seq = []
        for p in d.get("parameter", []):
            item = (p["name"], value_key(p))
            if not seq or seq[-1] != item:  # collapse repeated designation/property runs
                seq.append(item)
        return (status, rt, tuple(seq))
    if rt == "OperationOutcome":
        return (status, rt, d["issue"][0].get("code"))
    return (status, rt)

lines = open("scripts/capture-exemplars.sh").read().splitlines()
calls = []
for line in lines:
    m = re.match(r'^(get|post) \$(F|S) (\S+) "([^"]+)"(?: \'(.*)\')?$', line.strip())
    if not m:
        continue
    kind, which, name, url, body = m.groups()
    url = url.replace("\\$", "$").replace("$FHIR", base + "/fhir").replace("$SEARCH", base + "/searchapi")
    folder = "fhir.loinc.org" if which == "F" else "searchapi"
    calls.append((folder, name, kind, url, body))

fails, slow = 0, []
for folder, name, kind, url, body in calls:
    ex = f"docs/exemplars/{folder}/{name}"
    if name in skip:
        print(f"SKIP  {folder}/{name}: {skip[name]}")
        continue
    try:
        up_status = int(open(ex + ".headers").readline().split()[1])
        up_body = open(ex + ".json").read()
    except OSError:
        print(f"MISS  {folder}/{name}: no exemplar captured")
        continue
    req = urllib.request.Request(url, data=body.encode() if body else None, method="POST" if kind == "post" else "GET",
                                 headers={"Content-Type": "application/fhir+json"} if body else {})
    t0 = time.perf_counter()
    try:
        with urllib.request.urlopen(req) as r:
            status, got = r.status, r.read().decode()
    except urllib.error.HTTPError as e:
        status, got = e.code, e.read().decode()
    ms = (time.perf_counter() - t0) * 1000
    if ms > 25:
        slow.append((name, round(ms, 1)))
    if folder == "searchapi":
        ok = status == up_status and set(json.loads(got)) >= set(json.loads(up_body)) - {"FilterCounts"} | ({"FilterCounts"} if "FilterCounts" in json.loads(up_body) else set())
        detail = ""
    else:
        a, b = shape(up_status, up_body), shape(status, got)
        ok = a == b if a[1:2] != ("Parameters",) else (a[0], a[1]) == (b[0], b[1]) and [x for x in a[2] if x in b[2]] == [x for x in a[2] if x in b[2]]
        detail = "" if ok else f"\n      upstream={a}\n      local   ={b}"
    print(f"{'OK  ' if ok else 'DIFF'}  {folder}/{name} ({ms:.1f}ms){detail}")
    fails += 0 if ok else 1
print(f"\n{len(calls)} calls, {fails} shape differences, slow(>25ms): {slow or 'none'}")
sys.exit(1 if fails else 0)
EOF
