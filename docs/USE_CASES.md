# LOINC Browser — Use Cases

Who should reach for which interface, and how. The single-line `curl` examples are executable:
`make use-cases` runs each one against a running server and fails on any error status
(`scripts/check-use-cases.sh`). See [`LOCAL_APIS.md`](LOCAL_APIS.md) for the full
route list and divergences, [`FHIR_TERMINOLOGY_PLAN.md`](FHIR_TERMINOLOGY_PLAN.md) for the wire
design, [`DEPLOYMENT.md`](DEPLOYMENT.md) for install and service setup, [`MCP.md`](MCP.md) for MCP details, and [`API.md`](API.md) for `/api/v1`.

**Ground rules that apply to every use case below:**

- This is **not** an official Regenstrief service. It is not affiliated with or endorsed by
  Regenstrief. It serves one licensed LOINC release loaded into `loinc-normalized.sqlite` in the data directory
  (whatever version you imported, e.g. 2.82) — not the multi-version window (2.69–2.83) that
  `fhir.loinc.org` serves.
- Everything is answered from the local SQLite database. Serving paths never call the network.
  The only exception is the optional `/api/v1/official/search` proxy, which deliberately calls
  the real Regenstrief Search API when you ask it to.
- Basic-auth headers from existing clients are accepted and ignored; no credentials are needed
  locally.
- Examples assume the default address `:9005`; `LOINC_BROWSER_ADDR`/`PORT` in `.env` or
  `--addr` change it.

**Prerequisites:** a licensed LOINC release loaded into the database (`go run ./cmd/loinc-browser`
auto-ingests a local `Loinc*.zip` when the database is empty; otherwise
`ingest --release ./Loinc_2.82`). `/searchapi` and `loinc_lucene_search` also need the local
search index (`POST /api/v1/local-search/rebuild`).

## Choosing an interface

| Need | Interface | Transport |
| --- | --- | --- |
| Existing FHIR client, minimal change | `/fhir` | HTTP (Mode B/C) |
| Existing Search API client, minimal change | `/searchapi` | HTTP |
| Highest-volume single-code validation | `CodeSystem/$validate-code` | keep-alive HTTP (TCP or UDS); UDP for fire-and-forget fan-out |
| Form pick-lists, panels | `ValueSet/$expand`, `Questionnaire` | HTTP |
| Deprecated-code migration | `$lookup` MAP_TO, `$translate` (`loinc-map-to`) | HTTP |
| Compendium mapping, AI-assisted | term search with `classType=lab`, `$lookup`, MCP tools | HTTP or MCP |
| Lab-only or radiology-only results | `classType=lab`, `class=RAD` | HTTP or MCP |
| Natural-language requests | `mode=hybrid` term search (§16) | HTTP or MCP |
| Cross-terminology mapping | `ConceptMap` + `$translate` | HTTP |
| Hierarchy roll-ups / subsumption | `$subsumes`, implicit `vs/{LP}` | HTTP |
| AI agent integration | MCP tools | HTTP `/mcp` or stdio |
| In-process Go embedding | `pkg/terminology` (Mode A) | direct call |
| EMR form-builder scripting, non-FHIR shape | `/api/v1` | HTTP |
| Interactive exploration | UI (`?mode=apis`, `?mode=advanced`) | browser/HTTP |

## 1. Drop-in replacement for `fhir.loinc.org` in an existing FHIR client

**Who / problem:** A team has a FHIR client, terminology service adapter, or CDS tool already
coded against `https://fhir.loinc.org` and wants LOINC lookups to work air-gapped, or faster, in
a data centre without re-touching client code.

**Interface:** FHIR R4 under `/fhir` (CodeSystem, ValueSet, ConceptMap, Questionnaire).

**Transport:** HTTP over LAN (Mode B/C) for most consumers — it's what the client already
speaks. Nothing else changes.

**Example:**

```bash
curl 'http://localhost:9005/fhir/CodeSystem/$lookup?system=http://loinc.org&code=718-7'
```

Just swap the base URL:

| Upstream | Local |
| --- | --- |
| `https://fhir.loinc.org` | `http://<host>:<port>/fhir` |

**Response:** the same `Parameters` resource shape upstream returns (`code`, `system`, `name`,
`version`, `display`, `status`, `designation*`, `property*`), including quirks upstream clients
already parse around, like `system` as `valueString`.

**Caveats:** only one LOINC version is loaded, so `version=` requests for any other release
404 as unknown (upstream serves 2.69–2.83). Resource ids are readable (`loinc-2.82`) rather than
UUIDs, but canonical `url` still matches. See the divergence table in `LOCAL_APIS.md`.

## 2. Drop-in for the LOINC Search API client (`/searchapi`)

**Who / problem:** A team has a client built against
`https://loinc.regenstrief.org/searchapi/{scope}` (the Lucene-style REST search API) and wants
the same request/response shape served locally.

**Interface:** `/searchapi/{loincs|parts|answerlists|groups}`.

**Transport:** HTTP.

**Example:**

```bash
curl 'http://localhost:9005/searchapi/loincs?query=glucose&rows=2'
```

**Response:** `{ResponseSummary: {RecordsFound, StartingOffset, RowsReturned, LoincVersion,
QueryUrl, Next, ...}, Results: [...]}`, with `Results` keyed by release-CSV column names
(`LOINC_NUM`, `COMPONENT`, …).

**Caveats:** `includefiltercounts=true` only applies facet counts on the `loincs` scope.
`language=` swaps in `LinguisticVariants` fields, matching upstream. A missing local search index
returns 503 `{"Message": "local search index not built; POST /api/v1/local-search/rebuild"}`.

## 3. Validating LOINC codes in inbound HL7v2/FHIR lab results at high volume

**Who / problem:** An interface engine or lab-results pipeline needs to validate that inbound
`OBX-3`/`Observation.code` LOINC codes are real and active, at high message volume, with tight
per-message latency.

**Interface:** FHIR `CodeSystem/$validate-code`, or the UDP micro-protocol's `validate` op for a
same-host sidecar.

**Transport:** pick by latency budget. Warm `$lookup 718-7` mean per call, loopback, from §10.1
of the plan (`$validate-code` reuses `$lookup` and runs ~0.1ms cheaper):

| Transport | Mean per call | Notes |
| --- | --- | --- |
| In-process (Mode A, for reference) | ~0.61–0.76ms (`BenchmarkLookupTerm`) | the query work itself; every transport below adds to this |
| HTTP over TCP | ~1.16–1.37ms (`BenchmarkHTTPLookup`) | simplest, works everywhere; reuse keep-alive connections |
| HTTP over Unix socket | not benchmarked separately; same handler, minus the TCP stack | same-host only; file permissions double as access control; opt in with `--unix-socket` / `LOINC_BROWSER_UNIX_SOCKET` |
| UDP micro-protocol | ~0.86ms (`BenchmarkUDPLookup`) | skips HTTP framing; lossy by design; off by default (`--udp-addr` / `LOINC_BROWSER_UDP_ADDR`) |

**Reuse connections.** The server keeps connections open between requests, but only a client
that reuses one session benefits: `requests.Session()` or `httpx.Client()` in Python (not bare
`requests.get`), one shared `http.Client` in Go (raise `Transport.MaxIdleConnsPerHost` above its
default of 2 when running more than two requests in parallel), and `fetch` in Node 19+. Idle
connections close after 120 s. Four to eight parallel requests suit SQLite's concurrent reads; the
same applies to bulk MCP calls, which are plain JSON over HTTP here.

For most pipelines, keep-alive HTTP over TCP or UDS is the right choice. UDP saves roughly 0.3–0.5ms
per call, but only callers that already tolerate loss and retry idempotently should use it.

**Examples:**

```bash
curl 'http://localhost:9005/fhir/CodeSystem/$validate-code?url=http://loinc.org&code=718-7'

# same-host (start the server with --unix-socket ./data/loinc-browser.sock)
curl --unix-socket ./data/loinc-browser.sock 'http://localhost/fhir/CodeSystem/$validate-code?url=http://loinc.org&code=718-7'

# UDP sidecar (start the server with --udp-addr :8081)
echo -n '{"id":"1","op":"validate","code":"718-7"}' | nc -u -w1 localhost 8081
```

**Response:** `result` is a **valueString** `"true"`/`"false"` (matches upstream, not a real
boolean) via HTTP; the UDP `validate` op returns `{"ok":true,"result":true}`. HTTP status is
always 200 even on `false`.

**Caveats:** UDP responses over 1400 bytes return `{"ok":false,"truncated":true,"error":"use-http",...}`
instead of a silent partial payload — validate is small enough this rarely triggers. UDP is
off by default and has no delivery guarantee; use it only where the caller already tolerates
loss and retries idempotently.

## 4. Order-entry / form pick-lists from answer lists and groups

**Who / problem:** An order-entry or results-entry form needs a coded pick-list — "Positive /
Negative / Indeterminate" for a qualitative result field, or the member set of a LOINC group.

**Interface:** FHIR `ValueSet/$expand` for LL (answer list) and LG (group) value sets, filtered
and paged.

**Transport:** HTTP.

**Example:**

```bash
curl 'http://localhost:9005/fhir/ValueSet/$expand?url=http://loinc.org/vs/LL1162-8&count=100'
curl 'http://localhost:9005/fhir/ValueSet/$expand?url=http://loinc.org/vs/LL1162-8&filter=pos'
```

**Response:** `expansion.total`, `expansion.offset`, and `expansion.contains[]` (`system`,
`code`, `display`). `count` defaults to 100, max 1000. `activeOnly=true` drops DEPRECATED members.

**Caveats:** unknown LL/LG codes 404 (`not-found`) here, where upstream returns an empty 200 —
a documented, spec-correct divergence (§7 of the plan). `/api/v1/answer-lists/{id}/answers` is
the equivalent non-FHIR route if your form layer prefers the normalized API (see `API.md`).

## 5. Rendering LOINC panels as forms

**Who / problem:** A form-builder needs to render a LOINC panel (e.g. a screening battery) as a
structured questionnaire — item order, required flags, answer options.

**Interface:** FHIR `Questionnaire/{LOINC}`.

**Transport:** HTTP.

**Example:**

```bash
curl 'http://localhost:9005/fhir/Questionnaire/89689-4'
```

**Response:** one Questionnaire per panel/form term; `item[]` mirrors the panel's child
structure (`linkId`, `code`, `text`, `type`, `required`, `answerOption`).

**Caveats:** a non-panel LOINC number 404s. Only one level of same-LOINC repeat-group nesting is
reconstructed (see `FHIR_TERMINOLOGY_PLAN.md` §4.11); there is no `enableWhen` skip logic, since
upstream's is hand-curated and not in the release data. `GET /api/v1/panels/{loincNum}/items` is
the non-FHIR equivalent if you prefer authored-sequence rows instead of a Questionnaire tree.

## 6. Migrating deprecated codes

**Who / problem:** A system holding old LOINC codes needs to find current replacements before a
release upgrade, or needs to exclude/include deprecated codes deliberately.

**Interface:** FHIR `CodeSystem/$lookup` (the `MAP_TO` property), `ConceptMap/$translate` via the
local `loinc-map-to` map, and the `deprecated-loinc-terms` ValueSet.

**Transport:** HTTP.

**Examples:**

```bash
# replacement via $lookup's MAP_TO property
curl 'http://localhost:9005/fhir/CodeSystem/$lookup?system=http://loinc.org&code=6796-7&property=MAP_TO'

# replacement via ConceptMap
curl 'http://localhost:9005/fhir/ConceptMap/$translate?url=http://loinc.org/cm/loinc-map-to&code=6796-7'

# enumerate everything deprecated
curl 'http://localhost:9005/fhir/ValueSet/$expand?url=http://loinc.org/vs/deprecated-loinc-terms&count=1000'

# exclude deprecated from any other expansion
curl 'http://localhost:9005/fhir/ValueSet/$expand?url=http://loinc.org/vs&activeOnly=true&count=100'
```

**Response:** `$lookup` returns `status: retired` for DEPRECATED terms; `$translate` returns
`result: true` with the replacement `concept` and an optional `comment` part when
`loinc_map_to.comment` is non-blank.

**Caveats:** `activeOnly=true` treats DEPRECATED as the only inactive status (TRIAL/DISCOURAGED
stay active), matching HL7's "Using LOINC with FHIR" guidance, not the UI's search-hide default.

## 7. Mapping a local lab compendium to LOINC

**Who / problem:** A lab or LIS team has a local test compendium (CSV of local test
names/codes) and needs to bulk-map each row to a LOINC code by matching Component, Property,
System, Scale, and Method.

**Interface:** term search (`/api/v1/terms/search`, or MCP `loinc_search_terms` for an agent) for
candidates, `/searchapi` Lucene-style queries when you need fielded axis matching, and FHIR
`$lookup` or `loinc_get_term_fit` to check a candidate before committing it.

**Transport:** HTTP, reusing one connection (see §3), or MCP over HTTP.

**Examples:**

```bash
curl 'http://localhost:9005/api/v1/terms/search?q=potassium%20serum&classType=lab&limit=5'
curl 'http://localhost:9005/searchapi/loincs?query=Component:glucose+AND+System:bld&rows=25'
curl 'http://localhost:9005/fhir/CodeSystem/$lookup?system=http://loinc.org&code=2345-7'
```

MCP tool calls for an agent working row by row:

```json
{"tool": "loinc_search_terms", "arguments": {"q": "hba1c fasting", "classType": "lab", "limit": 5}}
{"tool": "loinc_get_term_fit", "arguments": {"loincNum": "4548-4"}}
```

**Response:** ranked candidates. For text queries the order blends the text match with how
commonly the term is used, pushes TRIAL and DISCOURAGED terms down, and ranks panels below single
tests unless the query says "panel". Each MCP candidate carries `relevance`.

**Patterns that work** (from mapping a 15,099-row hospital compendium):

- **Scope the search.** Pass `classType=lab` for lab tests; survey (PhenX) and attachment terms
  otherwise compete ("vitamin d" ranks a PhenX protocol #2 without it). For radiology use
  `class=RAD`. Several classes can be combined: `class=CHEM&class=SERO`. See §15.
- **Send plain words.** Common words (for, of, the, in, ...) and generic request words (routine,
  examination, test, level, ...) are ignored, and a whole-word match,
  such as an abbreviation in LOINC's synonyms (CRP, HBsAg, TSH), outranks a prefix match. Strip
  local noise yourself: parenthesized codes ("BLOOD GROUP (BG)"), department prefixes.
- **Handle `relaxed`.** When no term has every word, the search drops the most common words first
  and returns `relaxed: true` with `droppedWords` ("hba1c fasting" drops "fasting", keeps the
  analyte). Specimen words (blood, urine, serum, CSF, fluid, ...) are never dropped: a result for
  the wrong specimen is worse than none. Record the dropped words with the mapping and treat relaxed results as lower
  confidence. When only generic words would be left (a word LOINC never uses, such as "widal"),
  the search returns nothing rather than thousands of unrelated terms.
- **Use `relevance` within one call only.** It depends on the query words, so compare candidates
  from the same search, and keep your own cross-row confidence score.
- **Deprecated terms are hidden** by default. Pass `status=DEPRECATED` or `status=*` only to map
  legacy codes (§6).
- **Bring your own synonyms** for local terms LOINC doesn't use: ESR (LOINC says "Sed Rat"), USG
  (LOINC says "US"), Widal (LOINC names the S. Typhi antibodies), "urine routine examination"
  (urinalysis panel), "fungal" (LOINC says "Fungus"). The server does not expand
  synonyms beyond LOINC's related names.

**Caveats:** compare against the Fully-Specified Name and its major axes, not display-name
similarity alone (`docs/agent/LOINC_CONCEPTS.md`, Search Strategy). There is no batch endpoint,
so a 5,000-row compendium means at least 5,000 calls: script them over one reused HTTP session
with four to eight in parallel, or use `pkg/terminology` in-process (§13). MCP over HTTP works for
this too; send `Accept: application/json, text/event-stream` (both types, or the server answers
400) and expect a plain JSON body. A guided, in-app mapping chat agent that records per-row
decisions is a **future epic** (`docs/AGENT_CHAT_PLAN.md`).

## 8. Cross-terminology mapping

**Who / problem:** An integration needs LOINC parts mapped to SNOMED CT, RxNorm, ChEBI, or
similar, or LOINC codes mapped to IEEE 11073 device codes or the RSNA RadLex playbook.

**Interface:** FHIR `ConceptMap` catalogue + `$translate`.

**Transport:** HTTP.

**Examples:**

```bash
curl 'http://localhost:9005/fhir/ConceptMap?url=http://loinc.org/cm/loinc-to-ieee-11073-10101'
curl 'http://localhost:9005/fhir/ConceptMap/$translate?system=http://loinc.org&code=11556-8'
curl 'http://localhost:9005/fhir/ConceptMap/$translate?system=http://loinc.org&code=30657-1'
```

**Response:** `11556-8` → an IEEE 11073 match (`urn:iso:std:iso:11073:10101`); `30657-1` → a
RadLex match with `equivalence: relatedto` (RadLex playbook rows have no equivalence column, so
`relatedto` is used, matching upstream).

**Caveats:** only maps with release data are served — `loinc-to-phenx` and the CMS maps
(`-cms-irf-pai`, `-lcds`, `-mds`, `-oasis`) are **not** served (no source data in the release).
See the full map id table in `FHIR_TERMINOLOGY_PLAN.md` §4.9. `reverse=true` translates via a
map's reverse direction, and reverse maps that upstream 404s (`…-to-loinc`) work here.

## 9. Hierarchy-based queries and analytics roll-ups

**Who / problem:** Analytics or a dashboard needs "all tests under this LOINC part hierarchy
node" for roll-up reporting, or needs to check subsumption between two codes.

**Interface:** FHIR `CodeSystem/$subsumes`, implicit `ValueSet/vs/{LP}` expansion, and
`compose.include[].filter[]` with `ancestor`/`concept is-a`.

**Transport:** HTTP, or `/api/v1/hierarchy/*` if you want non-FHIR occurrence-node browsing
(see `API.md`).

**Examples:**

```bash
curl 'http://localhost:9005/fhir/CodeSystem/$subsumes?system=http://loinc.org&codeA=LP15946-4&codeB=30064-0'
curl 'http://localhost:9005/fhir/ValueSet/$expand?url=http://loinc.org/vs/LP15946-4&count=1000'
```

**Response:** `$subsumes` gives `outcome` (valueString: `equivalent`, `subsumes`,
`subsumed-by`, `not-subsumed`). The implicit `vs/{LP}` expansion returns every term under that
hierarchy node — the same set `$subsumes` reasons over pairwise.

**Caveats:** `$closure` (stateful incremental closure) is **not implemented** by design —
`$subsumes` (pairwise) plus `ancestor`/`is-a` filter expansion cover the same need without
server-side session state (plan §10, decision 6). Implicit `vs/{LP}` expansion only works for
parts with a `Part.csv` row. Hierarchy-only nodes such as `LP384441-4` return a 404 from `$expand`,
even though `$subsumes` accepts them. LG groups use plain text `loinc_num` order in
`expansion.contains`, not numeric order, unlike every other served set — a captured upstream
quirk, not a bug.

## 10. Multilingual display

**Who / problem:** A UI or report needs LOINC display text in a language other than English.

**Interface:** FHIR `$lookup`/`$expand` `displayLanguage=`, or `/searchapi` `language=`.

**Transport:** HTTP.

**Examples:**

```bash
curl 'http://localhost:9005/fhir/CodeSystem/$lookup?system=http://loinc.org&code=718-7&displayLanguage=es-AR'
curl 'http://localhost:9005/searchapi/loincs?query=hemoglobin&language=15'
```

**Response:** `$lookup` adds a `designation` entry for the requested language (falling back to
en-US if that language has no `LONG_COMMON_NAME` variant); `/searchapi` swaps
`COMPONENT`…`LONG_COMMON_NAME` and `RELATEDNAMES2` for the LinguisticVariants row identified by
the numeric `language` ID.

**Caveats:** designations come from the local release's `LinguisticVariants` files — only
languages present in your imported release are available; `language=` in `/searchapi` is a
numeric LinguisticVariants `ID`, not a BCP-47 tag, matching upstream's own convention.

## 11. AI agents via MCP

**Who / problem:** An agent (Claude Code, Claude Desktop, or another MCP client) needs
programmatic, context-capped access to LOINC lookups, search, and FHIR operations without
parsing full FHIR resources by hand.

**Interface:** MCP tools — normalized-database tools (`loinc_search_terms`, `loinc_get_term`,
`loinc_get_term_fit`, …) and FHIR/Search-API-backed tools (`loinc_lookup_code`,
`loinc_validate_code`, `loinc_subsumes`, `loinc_expand_value_set`, `loinc_search_value_sets`,
`loinc_validate_value_set_membership`, `loinc_translate`, `loinc_list_concept_maps`,
`loinc_get_questionnaire`, `loinc_lucene_search`). Full list in [`MCP.md`](MCP.md).

**Transport:** HTTP (`/mcp`, on by default, negotiates protocol versions `2026-07-28`,
`2025-11-25`, `2025-06-18`) for an agent already talking to the running all-in-one server, or
stdio for an agent config that launches a dedicated process.

**Examples:**

```bash
# HTTP MCP (default run)
curl -X POST http://localhost:9005/mcp -H 'content-type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"loinc_lookup_code","arguments":{"code":"718-7"}}}'
```

```bash
# stdio MCP (dedicated process, e.g. an agent config launches this)
loinc-browser mcp --docs-dir ./docs/agent --search-index-path ./data/loinc-search.bleve
```

**Response:** each tool returns a compact summary plus a `browserUrl` (`/?term={code}`) and/or
`fhirUrl`; pass `rawFhir: true` on `loinc_lookup_code`/`loinc_get_questionnaire` for the full FHIR
resource. Bad codes return a tool error (`isError: true`), not a transport error.

**Caveats:** `loinc_lucene_search` needs the local Bleve index built first
(`POST /api/v1/local-search/rebuild`, or `--search-index-path` for stdio). Keep `limit` small —
MCP tools cap large limits, and the guidance in `MCP.md` explicitly says not to use MCP for bulk
dumps of the release.

## 12. Air-gapped / offline data-centre deployment

Every interface above already runs offline (see the ground rules), and so do Swagger UI at
`/api/docs` and `/openapi.json`, because their assets are bundled rather than loaded from a CDN.

```bash
curl 'http://localhost:9005/api/docs'
```

The **only** feature that calls the network is the optional
`POST /api/v1/official/search` proxy to the real Regenstrief Search API — do not use it in an
air-gapped environment, or firewall it off deliberately. `make dev-refs` / `scripts/capture-exemplars.sh`
(vendored docs and golden-response capture) also need network and are development-only, not
required to run the app.

## 13. Embedding in a Go service

**Who / problem:** A Go program on a host that already has a copy of the release SQLite file
wants LOINC lookups in-process, with no HTTP hop.

**Interface:** Mode A, the in-process `pkg/terminology` library:
`terminology.Open(dbPath string, opts terminology.OpenOptions) (*terminology.Service, error)`,
plus `(*Service).Close() error`. `Open` opens the database read-only (SQLite `mode=ro`), so it
can safely share a WAL-mode file with a running `loinc-browser` server; it skips the lazy
`CREATE INDEX IF NOT EXISTS` statements the FHIR queries otherwise use on a read-only connection
(logged once), and still answers correctly, just without that speed-up on an old, never-reindexed
database. See [`LOCAL_APIS.md`](LOCAL_APIS.md) Mode A for the full walkthrough.

**Transport:** direct Go function call, no network at all.

**Example:**

```go
import "loinc-browser/pkg/terminology"

svc, err := terminology.Open("./data/loinc-normalized.sqlite", terminology.OpenOptions{})
if err != nil {
    log.Fatal(err)
}
defer svc.Close()

term, _ := svc.Lookup(ctx, terminology.LookupParams{Code: "718-7"})
vs, _ := svc.Expand(ctx, terminology.ExpandParams{URL: "http://loinc.org/vs/LL1162-8"})
```

**Response:** term lookup costs ~0.61–0.76ms (`BenchmarkLookupTerm`). That saves about 0.5ms
per call compared with HTTP (§3), but it still misses the plan's ≤100µs Mode A budget, because the
cost is in the SQL, not the transport. Part lookups (~60–70µs), `$subsumes` (~25–60µs), and cached
expansions (`BenchmarkExpandAnswerList` ~34µs) do meet it.

**Caveats:** Mode A needs the full release DB on that host. A trimmed lookup-only DB is deferred
until a consumer asks for one (plan §10, decision 2).

## 14. Browsing/exploring in the UI

**Who / problem:** A clinical-informatics reviewer wants to explore terms, try FHIR/searchapi
calls interactively, or browse the hierarchy/relationships without writing a client.

**Interface:** the Svelte UI's Local APIs console, Advanced (local) Search view, and
hierarchy/relationships browsing.

**Transport:** browser, over HTTP.

**Examples:**

```text
http://localhost:9005/?mode=apis        # Local APIs console: operation presets, editable params, copyable curl
http://localhost:9005/?mode=advanced    # Advanced (local) Search view over /searchapi + /api/v1/local-search
http://localhost:9005/?mode=hierarchy   # hierarchy browsing
http://localhost:9005/?mode=relationships
```

**Response:** the Local APIs console shows the request URL, a copyable curl, and pretty JSON with
status/timing for every FHIR/searchapi route in §1 of the plan. The Official API view
(`?mode=official`) also has a "Local /searchapi" toggle for side-by-side comparison against the
real upstream.

**Caveats:** this is for interactive exploration, not automation — script against the HTTP
routes or MCP tools directly for anything repeated.

## 15. Narrowing searches to lab tests or radiology

**Who / problem:** A user or integration wants lab results only, or radiology only, without
survey, attachment, or clinical-document terms mixed in.

**Interface:** the `classType` and `class` filters on term search (UI Type filter,
`/api/v1/terms/search`, MCP `loinc_search_terms`), or `Class:` clauses in `/searchapi`.

**Transport:** HTTP or MCP.

**Examples:**

```bash
curl 'http://localhost:9005/api/v1/terms/search?q=ferritin&classType=lab'
curl 'http://localhost:9005/api/v1/terms/search?q=chest&class=RAD'
curl 'http://localhost:9005/api/v1/terms/search?q=glucose&class=CHEM&class=UA'
curl 'http://localhost:9005/searchapi/loincs?query=(Class:CHEM%20OR%20Class:SERO)%20AND%20glucose'
```

**Response:** the usual term list, restricted to the chosen type or classes.

| `classType` | LOINC CLASSTYPE | Terms in 2.82 |
| --- | --- | --- |
| `lab` | 1, Laboratory | 66,861 |
| `clinical` | 2, Clinical (includes radiology, `class=RAD`) | 28,635 |
| `attachment` | 3, Claims attachments | 1,161 |
| `survey` | 4, Surveys (PhenX, MDS, ...) | 12,668 |

Lab classes include `CHEM`, `HEM/BC`, `MICRO`, `SERO`, `UA`, `COAG`, `BLDBK`, `DRUG/TOX`,
`ALLERGY`, `ABXBACT`, `CELLMARK`, `PATH`, and `SPEC`.

**Caveats:** `classType` reads CLASSTYPE from the raw `Loinc.csv` table kept at import; a database
imported by a very old version without raw tables returns 400 until re-imported. An unknown
`classType` value returns 400. `/searchapi` has no class-type field; combine `Class:` clauses.

## 16. Natural-language search by meaning

**Who / problem:** A user or agent describes a test in their own words ("sugar in blood after
fasting", "kidney function", "hepatitis B surface antigen") that may share few or no words with
LOINC's names.

**Interface:** term search with `mode=hybrid` (meaning and words merged; the better default) or
`mode=semantic` (meaning only): UI **Match: Both / Meaning**, `/api/v1/terms/search`, and MCP
`loinc_search_terms` with `"mode"`.

**Transport:** HTTP or MCP over HTTP. The stdio MCP command has no meaning index.

**Setup:** run an OpenAI-compatible embeddings endpoint and point the server at it, then build the
meaning index once per import:

```text
LOINC_EMBEDDING_URL=http://127.0.0.1:1234/v1        # LM Studio; Ollama/vLLM/hosted also work
LOINC_EMBEDDING_MODEL=text-embedding-qwen3-embedding-0.6b
POST /api/v1/semantic/rebuild                        # ~40 min for 2.82; resumable
GET  /api/v1/semantic/status                         # building 12,345 of 109,325 ... ready
```

**Examples** (need the meaning index, so not run by `make use-cases`):

```text
/api/v1/terms/search?q=sugar%20in%20blood%20after%20fasting&mode=hybrid&classType=lab
/api/v1/terms/search?q=kidney%20function&mode=semantic&classType=lab
```

**Response:** the usual term list with `"mode"`; every filter (`status`, `class`, `classType`)
applies. `hybrid` merges the two rankings by reciprocal rank fusion, so a term found both ways
rises to the top.

**Quality** (2.82, Qwen3-embedding 0.6B, `classType=lab`; right term at #1 / in top 3 / in top 10):

| Query set | Words | Meaning | Hybrid |
|---|---|---|---|
| 20 lab probes (used to tune; lay phrases overlap them) | 11 / 12 / 14 | 14 / 20 / 20 | 12 / 20 / 20 |
| 20 held-out plain-language queries (15 lay-phrase tests, 5 controls) | 9 / 9 / 10 | 12 / 16 / 18 | 11 / 11 / 15 |

Word search is best for exact codes and names; meaning finds plain-language requests. Meaning
ranking adds the same kind of popularity prior as word search, so common tests beat obscure
near-synonyms. About 35 common tests also carry hand-written lay phrases ("average blood sugar",
"bad cholesterol") in their embedded text, since LOINC's names never use them; on the held-out
set they moved meaning from 6 / 12 / 14 to 12 / 16 / 18 with the controls unchanged. Changing
the phrases marks the index stale, and a rebuild re-embeds only those terms.

**Caveats:** prefer `hybrid` over `semantic` for mapping; it keeps word search's exact-name hits. Each query is embedded by the endpoint at search time,
so a hosted endpoint sees query text; LM Studio or Ollama keeps everything on the machine. The
index is about 112 MB in memory (int8 vectors) and is marked `stale` after a new import. See
[`DEPLOYMENT.md`](DEPLOYMENT.md#meaning-based-search).

## Not a fit

Per the plan's explicit non-goals (`FHIR_TERMINOLOGY_PLAN.md` §1, §10):

- **Write operations** — create/update/delete, `vread`/history. This server is read-only.
- **SNOMED CT (or other terminologies) as first-class code systems** — they appear only as
  `ConceptMap` targets from LOINC parts, never as a served `CodeSystem`/`ValueSet` of their own.
- **Multiple loaded LOINC versions** — one release is loaded at a time; `version=` for any other
  release 404s.
- **`ConceptMap/$closure`** — stateful incremental closure maintenance is not implemented;
  use `$subsumes` or an `ancestor`/`is-a` filter expansion instead.
- **WebSocket transport** — not implemented; lookups are request/response, and HTTP keep-alive,
  UDS, and UDP already cover the relevant latency tiers. The agent chat feature (future epic,
  `docs/AGENT_CHAT_PLAN.md`) streams over SSE instead, when it lands.
- **Batch validation, `$format=xml`** — not served; XML Accept/`_format` gets a 406
  `OperationOutcome`.
