#!/usr/bin/env bash
# Re-download the third-party reference docs used to build and test the local FHIR / Search API /
# MCP features into docs/vendor/ (gitignored: HL7, Regenstrief and MCP content stays local).
# No credentials needed. loinc.org blocks scripted downloads, so its pages come from the Wayback Machine.
set -euo pipefail
cd "$(dirname "$0")/.."
UA='Mozilla/5.0 (loinc-browser dev docs fetch)'
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
mkdir -p docs/vendor/{hl7/r4-operations,loinc,mcp}

fetch() { # url dest
  local code
  code="$(curl -sSL -A "$UA" --max-time 120 -o "$2" -w '%{http_code}' "$1" || echo 000)"
  echo "$code $2"
  [ "$code" = 200 ] || { echo "failed: $1" >&2; return 1; }
}
html_to_text() { # html txt [header]
  python3 - "$1" "$2" "${3:-}" <<'EOF'
import html, re, sys
src, dst, header = sys.argv[1:4]
t = open(src, encoding="utf-8", errors="ignore").read()
t = re.sub(r"<script.*?</script>|<style.*?</style>", "", t, flags=re.S)
t = re.sub(r"<[^>]+>", "\n", t); t = html.unescape(t); t = re.sub(r"\n\s*\n+", "\n", t)
open(dst, "w").write((header + "\n\n" if header else "") + t)
EOF
}

# HL7: "Using LOINC with FHIR" (R4 core + current THO), R4 terminology service page, R4 OperationDefinitions.
fetch https://hl7.org/fhir/R4/loinc.html docs/vendor/hl7/fhir-r4-using-loinc.html
html_to_text docs/vendor/hl7/fhir-r4-using-loinc.html docs/vendor/hl7/fhir-r4-using-loinc.txt
fetch https://terminology.hl7.org/en/LOINC.html docs/vendor/hl7/tho-using-loinc.html
html_to_text docs/vendor/hl7/tho-using-loinc.html docs/vendor/hl7/tho-using-loinc.txt
fetch https://hl7.org/fhir/R4/terminology-service.html docs/vendor/hl7/fhir-r4-terminology-service.html
for op in codesystem-lookup codesystem-validate-code codesystem-subsumes valueset-expand valueset-validate-code conceptmap-translate; do
  fetch "https://hl7.org/fhir/R4/operation-$op.json" "docs/vendor/hl7/r4-operations/operation-$op.json"
done

# HL7: R4 summary elements (isSummary=true) per served resource type, for _summary/_elements.
fetch https://hl7.org/fhir/R4/definitions.json.zip "$tmp/defs.zip"
unzip -o -q "$tmp/defs.zip" profiles-resources.json -d "$tmp"
python3 - "$tmp/profiles-resources.json" docs/vendor/hl7/r4-summary-elements.json <<'EOF'
import json, sys
d = json.load(open(sys.argv[1]))
want = {"CodeSystem", "ValueSet", "ConceptMap", "Questionnaire", "Bundle", "CapabilityStatement",
        "TerminologyCapabilities", "OperationOutcome", "Parameters"}
out = {"_source": "FHIR R4 (4.0.1) profiles-resources.json (https://hl7.org/fhir/R4/definitions.json.zip), "
                   "elements with isSummary=true and top-level min>0 elements"}
for e in d["entry"]:
    r = e["resource"]
    if r.get("resourceType") == "StructureDefinition" and r.get("type") in want \
            and r.get("kind") == "resource" and r.get("derivation") == "specialization":
        els = r["snapshot"]["element"]
        out[r["type"]] = {"summary": [x["path"] for x in els if x.get("isSummary")],
                          "mandatory": [x["path"] for x in els if x.get("min", 0) > 0 and x["path"].count(".") == 1]}
json.dump(out, open(sys.argv[2], "w"), indent=1)
EOF
echo "ok docs/vendor/hl7/r4-summary-elements.json"

# loinc.org (via Wayback): FHIR terminology server guide + Search API doc.
fetch "https://web.archive.org/web/2026/https://loinc.org/fhir/" docs/vendor/loinc/loinc-fhir-terminology-service.wayback.html
html_to_text docs/vendor/loinc/loinc-fhir-terminology-service.wayback.html docs/vendor/loinc/loinc-fhir-terminology-service.txt \
  "Source: https://loinc.org/fhir/ (Wayback Machine snapshot)"
fetch "https://web.archive.org/web/20240420012808/https://loinc.org/kb/search-api/" docs/vendor/loinc/search-api.wayback-20240420.html
html_to_text docs/vendor/loinc/search-api.wayback-20240420.html docs/vendor/loinc/search-api.wayback-20240420.txt \
  "Source: https://loinc.org/kb/search-api/ (Wayback Machine snapshot 2024-04-20)"
# The guide embeds a sample CodeSystem search Bundle (83 property + filter definitions); extract it as JSON.
python3 - docs/vendor/loinc/loinc-fhir-terminology-service.txt docs/vendor/loinc/fhir.loinc.org-codesystem-search-sample.json <<'EOF'
import json, sys
lines = open(sys.argv[1]).read().split("\n")
start = next(i for i, l in enumerate(lines) if l.strip() == "{")
end = next(i for i, l in enumerate(lines) if l.startswith("Get info on an individual"))
json.dump(json.loads("".join(lines[start:end])), open(sys.argv[2], "w"), indent=2)
EOF
echo "ok docs/vendor/loinc/fhir.loinc.org-codesystem-search-sample.json"

# MCP: spec changelog for the protocol version the Go SDK targets, plus the SDK's protocol notes.
fetch https://modelcontextprotocol.io/specification/2026-07-28/changelog.md docs/vendor/mcp/spec-2026-07-28-changelog.md
sdk_dir="$(go list -m -f '{{.Dir}}' github.com/modelcontextprotocol/go-sdk 2>/dev/null || true)"
if [ -n "$sdk_dir" ] && [ -f "$sdk_dir/docs/protocol.md" ]; then
  sdk_ver="$(go list -m -f '{{.Version}}' github.com/modelcontextprotocol/go-sdk)"
  install -m 0644 "$sdk_dir/docs/protocol.md" "docs/vendor/mcp/go-sdk-$sdk_ver-protocol.md"
  echo "ok docs/vendor/mcp/go-sdk-$sdk_ver-protocol.md"
fi
echo "vendor docs refreshed"
