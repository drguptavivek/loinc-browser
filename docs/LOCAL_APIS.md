# Local FHIR + Search API guide

This app serves local endpoints that are wire-compatible with two LOINC services published by Regenstrief (fhir.loinc.org and the LOINC Search API). They are **not** official Regenstrief services and are not affiliated with or endorsed by Regenstrief. Everything is answered entirely from
`./data/loinc-normalized.sqlite`. Serving paths never call the network. See
`docs/FHIR_TERMINOLOGY_PLAN.md` in the repository for the full design; this page is the
consumer-facing quick reference. See `docs/USE_CASES.md` for worked examples of who uses which
route and why.

## Base URL swap

An existing client only changes its base URL:

| Upstream (Regenstrief) base URL | Local base URL |
| --- | --- |
| `https://fhir.loinc.org` | `http://<host>:<port>/fhir` |
| `https://loinc.regenstrief.org/searchapi` | `http://<host>:<port>/searchapi` |

Request building, query parameters, and response parsing stay the same. Basic-auth headers
from existing clients are accepted and ignored — no credentials are required locally.

Try it interactively at [`/api/docs`](/api/docs) (Swagger UI) or in the app's **Local APIs**
console (`?mode=apis`), or read the machine-readable contract at
[`/openapi.json`](/openapi.json).

## Transports

| Mode | How | Notes |
| --- | --- | --- |
| TCP (default) | `curl http://localhost:9005/fhir/metadata` | Same handler tree as the other modes. |
| Unix domain socket | `curl --unix-socket /path/to.sock http://localhost/fhir/metadata` | Enabled with `--unix-socket` / `LOINC_BROWSER_UNIX_SOCKET`. File permissions double as access control. |
| UDP micro-protocol | `echo -n '{"id":"1","op":"lookup","code":"718-7"}' \| nc -u -w1 localhost 8081` | Off by default; enable with `--udp-addr` / `LOINC_BROWSER_UDP_ADDR`. Compact JSON datagrams (ops: lookup, validate, subsumes, translate, expand); oversize answers return `use-http` with the equivalent HTTP path. |
| MCP (agents) | `http://localhost:9005/mcp` (HTTP, on by default) or `loinc-browser mcp` (stdio) | Ten of the tools listed below (`loinc_lookup_code`, `loinc_validate_code`, `loinc_subsumes`, `loinc_expand_value_set`, `loinc_search_value_sets`, `loinc_validate_value_set_membership`, `loinc_translate`, `loinc_list_concept_maps`, `loinc_get_questionnaire`, `loinc_lucene_search`) wrap this same FHIR/Search API surface with compact, agent-friendly output. See `docs/MCP.md`. |

## Mode A: embed as a Go library

Any Go module can read `./data/loinc-normalized.sqlite` in-process, without an HTTP round trip,
via `loinc-browser/pkg/terminology`. This package is public (unlike `internal/loinc`, which Go's
internal-package rule blocks outside this module).

```go
import "loinc-browser/pkg/terminology"

svc, err := terminology.Open("./data/loinc-normalized.sqlite", terminology.OpenOptions{})
if err != nil {
    log.Fatal(err)
}
defer svc.Close()

result, outcome := svc.Lookup(ctx, terminology.LookupParams{Code: "718-7"})
```

`Open` opens the database **read-only** (SQLite `mode=ro`), so it can safely read the same file a
running `loinc-browser` server has open (WAL mode) at the same time. A read-only connection can't
run the lazy `CREATE INDEX IF NOT EXISTS` statements the FHIR queries otherwise use to speed up an
old database, so queries on an old, never-reindexed DB may be slower (a fresh DB already has these
indexes from ingest); results are unaffected either way. Call `(*Service).Close` when done.

`NewService(getStore)` remains available for a caller (such as `cmd/loinc-browser` itself) that
already owns a `*loinc.Store` and wants to hot-swap it across uploads.

## FHIR (`/fhir/...`)

`Content-Type: application/fhir+json;charset=UTF-8`. Every operation accepts GET query
parameters and POST FHIR `Parameters` bodies. `$` is a literal path segment
(`/fhir/CodeSystem/$lookup`, not a route parameter).

### Metadata

```bash
curl 'http://localhost:9005/fhir/metadata'
curl 'http://localhost:9005/fhir/metadata?mode=terminology'
```

Returns a CapabilityStatement (or TerminologyCapabilities with `?mode=terminology`) listing
CodeSystem, ValueSet, ConceptMap, and Questionnaire with only the interactions this server
serves.

### CodeSystem

```bash
curl 'http://localhost:9005/fhir/CodeSystem?url=http://loinc.org'
curl 'http://localhost:9005/fhir/CodeSystem/loinc-2.82'
```

The `http://loinc.org` CodeSystem covers terms, LP parts, LL answer lists, LA answers, and LG
groups.

#### `$lookup`

```bash
curl 'http://localhost:9005/fhir/CodeSystem/$lookup?system=http://loinc.org&code=718-7'
curl 'http://localhost:9005/fhir/CodeSystem/$lookup?system=http://loinc.org&code=LP384441-4'
```

```json
{
  "resourceType": "Parameters",
  "parameter": [
    {"name": "code", "valueCode": "718-7"},
    {"name": "system", "valueString": "http://loinc.org"},
    {"name": "name", "valueString": "LOINC"},
    {"name": "version", "valueString": "2.82"},
    {"name": "display", "valueString": "Hemoglobin [Mass/volume] in Blood"},
    {"name": "status", "valueCode": "active"},
    {"name": "designation", "part": [{"name": "language", "valueCode": "en-US"}, "..."]}
  ]
}
```

Resolves all five code kinds (term, LP part, LL answer list, LA answer, LG group).
`property=` (repeatable) restricts the returned property list. Unknown code or version
mismatch → 404 `OperationOutcome` `not-found`.

#### `$validate-code`

```bash
curl 'http://localhost:9005/fhir/CodeSystem/$validate-code?url=http://loinc.org&code=718-7'
```

`result` is a **valueString** `"true"`/`"false"` (matches upstream, not a real boolean). HTTP
status is always 200.

#### `$subsumes`

```bash
curl 'http://localhost:9005/fhir/CodeSystem/$subsumes?system=http://loinc.org&codeA=LP384441-4&codeB=30064-0'
```

Walks the Component Hierarchy by System. `outcome` (valueString) is one of `equivalent`,
`subsumes`, `subsumed-by`, `not-subsumed`. Unknown code → 400 `invalid`.

### ValueSet

```bash
curl 'http://localhost:9005/fhir/ValueSet?url=http://loinc.org/vs/LL1162-8'
curl 'http://localhost:9005/fhir/ValueSet/LL1162-8'
```

#### `$expand`

```bash
curl 'http://localhost:9005/fhir/ValueSet/$expand?url=http://loinc.org/vs/LL1162-8'
```

```json
{
  "resourceType": "ValueSet",
  "expansion": {
    "total": 5, "offset": 0,
    "parameter": [{"name": "offset", "valueInteger": 0}, {"name": "count", "valueInteger": 100}],
    "contains": [{"system": "http://loinc.org", "code": "LA137-2", "display": "None"}]
  }
}
```

`total` and `offset` are always present. `count` defaults to 100, max 1000. `filter=`,
`activeOnly=true`, and `includeDesignations=true` are supported. POSTing an inline
`valueSet` (Parameters body) expands ad-hoc `compose.include[].filter[]` compositions.

#### `$validate-code`

```bash
curl 'http://localhost:9005/fhir/ValueSet/LL1162-8/$validate-code?system=http://loinc.org&code=LA15679-6'
```

Note `result` is a **valueBoolean** here (unlike CodeSystem `$validate-code`).

### ConceptMap

```bash
curl 'http://localhost:9005/fhir/ConceptMap?url=http://loinc.org/cm/loinc-to-ieee-11073-10101'
curl 'http://localhost:9005/fhir/ConceptMap/loinc-to-ieee-11073-10101'
```

#### `$translate`

```bash
curl 'http://localhost:9005/fhir/ConceptMap/$translate?system=http://loinc.org&code=11556-8'
```

```json
{
  "resourceType": "Parameters",
  "parameter": [
    {"name": "result", "valueBoolean": true},
    {"name": "match", "part": [
      {"name": "equivalence", "valueCode": "equivalent"},
      {"name": "concept", "valueCoding": {"system": "urn:iso:std:iso:11073:10101", "code": "160116", "display": "MDC_CONC_PO2_GEN"}},
      {"name": "source", "valueUri": "http://loinc.org/cm/loinc-to-ieee-11073-10101"}
    ]}
  ]
}
```

Without `url`, every map whose source system matches `system` is searched. `reverse=true`
translates via the map's reverse direction.

### Questionnaire

```bash
curl 'http://localhost:9005/fhir/Questionnaire/89689-4'
```

One Questionnaire per LOINC panel/form term; `item[]` mirrors the panel's child structure
(`linkId`, `code`, `text`, `type`, `answerOption`). A non-panel LOINC number 404s.

### `_summary` and `_elements`

The FHIR R4 search result parameters ([`_summary`](https://hl7.org/fhir/R4/search.html#summary),
[`_elements`](https://hl7.org/fhir/R4/search.html#elements)) are supported on every read, search,
and `$expand` (a ValueSet). `$lookup`, `$validate-code`, `$subsumes`, and `$translate` return a
`Parameters` resource and are unaffected.

```bash
curl 'http://localhost:9005/fhir/ValueSet?url=http://loinc.org/vs/LL1162-8&_summary=true'
curl 'http://localhost:9005/fhir/ValueSet/LL1162-8?_elements=url,name'
curl 'http://localhost:9005/fhir/ConceptMap?_summary=count'
```

- `_summary=true` keeps only the R4 `isSummary=true` elements for that resource type
  (`docs/vendor/hl7/r4-summary-elements.json`), plus `resourceType`/`id`/`meta`.
- `_summary=text` keeps `text`, `id`, `meta`, and mandatory elements; `_summary=data` keeps
  everything except `text`; `_summary=false` is the default full resource.
- `_summary=count` (search only) returns a Bundle with `type`/`total`/`link[self]` and no
  `entry`; on a read or `$expand` it is a 400 `invalid`, since neither returns a searchset.
- `_elements=a,b` keeps those top-level elements plus `resourceType`/`id`/`meta`/mandatory
  elements; unknown names are ignored. Combining `_summary` with `_elements` is 400 `invalid`,
  as is an unrecognized `_summary` value.
- A filtered resource gets `meta.tag` `SUBSETTED`
  (`http://terminology.hl7.org/CodeSystem/v3-ObservationValue`). On a search, filtering applies
  to each `entry.resource`; the Bundle envelope itself is untouched.
- Upstream fhir.loinc.org rejects both parameters with 400 `not-supported`; supporting them here
  is a documented divergence (plan §7).

## LOINC Search API (`/searchapi/{scope}`)

```bash
curl 'http://localhost:9005/searchapi/loincs?query=glucose&rows=2'
curl 'http://localhost:9005/searchapi/parts?query=glucose&rows=2'
curl 'http://localhost:9005/searchapi/answerlists?query=yes&rows=2'
curl 'http://localhost:9005/searchapi/groups?query=glucose&rows=2'
```

```json
{
  "ResponseSummary": {
    "RecordsFound": 1027, "StartingOffset": 0, "RowsReturned": 2, "LoincVersion": "2.82",
    "QueryUrl": "http://localhost:9005/searchapi/loincs?query=glucose&rows=2",
    "Next": "http://localhost:9005/searchapi/loincs?query=glucose&rows=2&offset=2"
  },
  "Results": ["..."]
}
```

Scopes are `loincs`, `parts`, `answerlists`, `groups`. Parameters: `query`, `rows` (default
20, max 500), `offset`, `sortorder` (field name plus optional ` asc`/` desc`), `language`
(LinguisticVariants ID; swaps translated fields on `loincs` rows), and
`includefiltercounts=true` (facet counts, `loincs` scope only). Basic-auth headers are
accepted and ignored.

## Documented divergences from fhir.loinc.org

| Area | Upstream | Local | Why |
| --- | --- | --- | --- |
| Resource ids | UUIDs | readable ids (`LL1162-8`, `loinc-2.82`) | canonical `url` still matches; upstream also accepts these as read ids |
| `CodeSystem/loinc` read | 404 | 200 | harmless superset |
| `http://loinc.org/vs` and implicit `vs/{LP}` | 404 "Failed to find matching value set" | served | defined by the CodeSystem and by HL7 |
| `$expand` of unknown LL | empty 200 | 404 `not-found` | spec-correct |
| `$expand` with `filter`/`offset` on LL | 404 (upstream bug) | works | spec-correct |
| POST inline `valueSet` `$expand` | nginx 403 | works | spec-correct |
| Reverse `$translate` (`…-to-loinc` maps, `reverse=true`) | 404 | works | maps are published as bidirectional |
| Loaded versions | 2.69–2.83 | the one imported release | single-DB design |
| `_summary` / `_elements` | 400 `not-supported` | supported | FHIR R4 search result params; clients that never send them see no change |

## Reference

- Machine-readable contract: [`/openapi.json`](/openapi.json), browsable at [`/api/docs`](/api/docs).
- Full design and open decisions: `docs/FHIR_TERMINOLOGY_PLAN.md` in the repository.
- `/api/v1` (this app's own normalized API, not upstream-compatible): [`API.md`](API.md).
- Golden response captures used as the parity target: `docs/exemplars/fhir.loinc.org/` and
  `docs/exemplars/searchapi/` (local, untracked — see `AGENTS.md`).
