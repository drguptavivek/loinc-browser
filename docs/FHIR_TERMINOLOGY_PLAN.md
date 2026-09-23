# Local LOINC FHIR + Search API — Implementation Plan

Status: approved plan, being implemented in phases (§8). Revised 2026-09-23 after capturing
authenticated golden responses from the two official LOINC services.

**Primary objective:** fastest possible LOINC lookups served entirely from the local
`data/loinc-normalized.sqlite`. Serving paths never call `fhir.loinc.org` or
`loinc.regenstrief.org`.

**Secondary objective:** wire compatibility. Consumer apps were built against two official APIs:

1. **LOINC FHIR Terminology Server** — `https://fhir.loinc.org` (FHIR R4).
2. **LOINC Search API** — `https://loinc.regenstrief.org/searchapi/{scope}`.

A consumer only changes its base URL; request building and response parsing stay the same.

## 0. Sources of truth

Everything below is grounded in these local copies. Do not design from memory.

| Source | Local copy | What it settles |
|---|---|---|
| LOINC FHIR server guide (loinc.org/fhir, revised 2026-02-24) | `docs/vendor/loinc/loinc-fhir-terminology-service.txt` (+ `.html`) | resource list, canonical URLs, ValueSet/ConceptMap/Questionnaire catalogue |
| fhir.loinc.org CodeSystem definition (83 properties, filters) | `docs/vendor/loinc/fhir.loinc.org-codesystem-search-sample.json` | property codes and types (`Coding` vs `string`), filters |
| LOINC Search API doc (2024-02-29) | `docs/vendor/loinc/search-api.wayback-20240420.txt` | endpoints and parameters (no response schema published) |
| HL7 "Using LOINC with FHIR" (R4 + THO 7.4.0) | `docs/vendor/hl7/fhir-r4-using-loinc.txt`, `docs/vendor/hl7/tho-using-loinc.txt` | inactive = `STATUS=DEPRECATED`, implicit value sets, filter semantics, subsumption via the multiaxial hierarchy |
| FHIR R4 OperationDefinitions | `docs/vendor/hl7/r4-operations/operation-*.json` | the formal in/out parameter set of each operation |
| **Golden responses, fhir.loinc.org** (authenticated, 2026-09-23) | `docs/exemplars/fhir.loinc.org/*.json` + `.headers` | exact wire shapes; this is the parity target |
| **Golden responses, Search API** (authenticated, 2026-09-23) | `docs/exemplars/searchapi/*.json` + `.headers` | exact searchapi response shape |
| Secondary reference, tx.fhir.org | `docs/exemplars/tx.fhir.org/*` | a second server's behaviour; *not* the target |

Refresh the golden responses with `scripts/capture-exemplars.sh`. It reads `username=`/`password=`
from `loinc.env`, never echoes them, and sends them as Basic auth, which both official
services accept. `docs/vendor/` and `docs/exemplars/` hold third-party copyrighted content
(Regenstrief/HL7) and stay local and untracked, like the release data (AGENTS.md).

### 0.1 Facts established from the captures (these corrected the earlier draft)

- `fhir.loinc.org` is "Open Termhub R4 FHIR Terminology Server 1.3.1" on **HAPI FHIR 7.6.1**.
  It currently serves LOINC **2.83**, while the local DB is 2.82. Parity tests therefore compare
  *shape* (parameter names, order, value[x] types, status codes), never values.
- `tx.fhir.org` is HL7's FHIRsmith server (`X-Powered-By: Express`), **not HAPI**. Its
  `valueCode` axis properties and its rejection of LP codes in `$subsumes` are FHIRsmith
  behaviour; fhir.loinc.org behaves differently.
- The `http://loinc.org` CodeSystem covers **terms, LP parts, LL answer lists, LA answers, and LG
  groups**. `$lookup` resolves all five kinds.
- `$subsumes` walks the Component Hierarchy by System, so LP parts subsume terms (`LP384441-4`
  subsumes `30064-0` → `subsumes`; reversed → `subsumed-by`; `LP7846-1` → `LP14542-2` →
  `subsumes`).
- `$lookup` returns the full property set by default, plus designations in every language.
  Axis and part-valued properties are `valueCoding {system, code, display}`.
- Several responses carry a boolean inside `valueString` (for example
  `{"name":"result","valueString":"true"}` from CodeSystem `$validate-code`, and `outcome` from
  `$subsumes`). Consumers parse exactly that.
- DEPRECATED terms: `$lookup` `status` = `retired`; `$validate-code` `result` true with
  `active:false`.
- The Search API returns `{ResponseSummary, Results[, FilterCounts]}`. `Results` rows use
  release-CSV column names (`LOINC_NUM`, `COMPONENT`, …) with JSON `null` for blanks.
- Part-valued Codings (axes, supplementary properties, parent/child) use the Part's **PartName**
  as `display` (e.g. SYSTEM LP7057-5 → "Bld", CLASS LP7803-2 → "HEM/BC", parent LP7846-1 →
  "SPEC"), not PartDisplayName ("Blood", "Hematology and Cell counts"), falling back to the
  hierarchy label for hierarchy-only nodes; verified against codesystem-lookup-718-7/4544-3/
  LP14542-2-parent exemplars and now enforced by `assertSameCodingDisplays` in
  `pkg/terminology/parity_test.go`.

## 1. Served surface

Base paths: FHIR under `/fhir`; Search API under `/searchapi`, mirroring the upstream path so
only the host changes. Every FHIR operation accepts GET query parameters and POST
`Parameters` bodies (`application/fhir+json` or `application/json`).

| # | Route | Upstream parity | Section |
|---|---|---|---|
| 1 | `GET /fhir/metadata` | CapabilityStatement | §4.1 |
| 2 | `GET /fhir/metadata?mode=terminology` | TerminologyCapabilities | §4.1 |
| 3 | `GET /fhir/CodeSystem?url=http://loinc.org[&version=]`, `GET /fhir/CodeSystem/{id}` | search / read | §4.2 |
| 4 | `GET/POST /fhir/CodeSystem/$lookup`, `/fhir/CodeSystem/{id}/$lookup` | terms, LP, LL, LA, LG | §4.3 |
| 5 | `GET/POST /fhir/CodeSystem/$validate-code`, `…/{id}/$validate-code` | | §4.4 |
| 6 | `GET/POST /fhir/CodeSystem/$subsumes`, `…/{id}/$subsumes` | hierarchy-based | §4.5 |
| 7 | `GET /fhir/ValueSet?url=…|name=…|name:in=…|_id=…|_count&_offset` , `GET /fhir/ValueSet/{id}` | search / read (definition) | §4.6 |
| 8 | `GET/POST /fhir/ValueSet/$expand`, `/fhir/ValueSet/{id}/$expand` | incl. POSTed inline `valueSet` | §4.7 |
| 9 | `GET/POST /fhir/ValueSet/$validate-code`, `/fhir/ValueSet/{id}/$validate-code` | | §4.8 |
| 10 | `GET /fhir/ConceptMap?url=…|source-system=…|target-system=…`, `GET /fhir/ConceptMap/{id}` | search / read | §4.9 |
| 11 | `GET/POST /fhir/ConceptMap/$translate`, `/fhir/ConceptMap/{id}/$translate` | | §4.10 |
| 12 | `GET /fhir/Questionnaire?url=http://loinc.org/q/{LOINC}`, `GET /fhir/Questionnaire/{LOINC}` | LOINC panels | §4.11 |
| 13 | `GET /searchapi/{loincs\|parts\|answerlists\|groups}` | Search API | §5 |

The existing `POST /api/v1/official/search` proxy is unchanged. It stays the way to compare
against the live upstream.

Non-goals: XML (`_format=xml` or `Accept: application/fhir+xml` → 406 OperationOutcome),
write interactions (create/update/delete), `vread`/history, multiple loaded LOINC versions,
batch validation, `$closure`, and the upstream "versions" operation. Also out of scope are
ConceptMaps and ValueSets whose source data is not in the release (§4.6.2, §4.9.1).

## 2. Delivery modes

"Local" means inside the DC. External-network policy is enforced at the firewall; the app's
only obligation is that serving paths read local SQLite only.

| Mode | Consumer | Transport | Latency budget (p99, warm) |
|---|---|---|---|
| A. In-process library | Go programs on a host that has a copy of the DB file | direct call, `pkg/terminology` | ≤ 100µs lookup |
| B. DC-local service (**primary**) | any language | HTTP over LAN | ≤ 5ms + RTT |
| C. Same-host sidecar | any language | HTTP over 127.0.0.1 | ≤ 5ms |
| D. Unix domain socket | same host | HTTP over UDS (`--unix-socket`, `LOINC_BROWSER_UNIX_SOCKET`) | ≤ 2ms |
| E. UDP datagram | same host/DC | compact JSON micro-protocol (§6), `--udp-addr`, `LOINC_BROWSER_UDP_ADDR` | ≤ 1ms, lossy by design |

- **B** needs only a base-URL change in consumers: `https://fhir.loinc.org` →
  `http://<dc-host>:<port>/fhir`, and `https://loinc.regenstrief.org/searchapi` →
  `http://<dc-host>:<port>/searchapi`. Basic-auth headers from existing clients are accepted and
  ignored.
- **D** serves the identical HTTP handler tree over a socket. File permissions double as access
  control.
- **A** opens the DB read-only (`mode=ro`, WAL), so it can share the file with a running
  server. It needs the 1.6GB DB on that host; a trimmed lookup-only DB is deferred (§10).
- One process serves B/C/D/E concurrently from one store handle. The server hot-swaps its store
  after an upload import, so handlers resolve the store per request (`currentStore()`) and never
  capture a `*loinc.Store`.

## 3. Storage: no schema changes

Everything needed is already in the normalized DB. That includes the `raw_csv_*` tables, which
hold every release CSV verbatim. FHIR resources are projected per request; nothing FHIR-shaped
is persisted. Only additive `CREATE INDEX IF NOT EXISTS` statements run at store open.

| Need | Source |
|---|---|
| Version | `import_meta.release_dir` basename `Loinc_2.82` → `2.82` |
| Terms + all 83 CodeSystem properties | `loinc_terms` + `Term.Fields` (full `Loinc.csv` row) |
| Axis/CLASS properties as Coding | `loinc_part_links` (`property` column holds `http://loinc.org/property/{code}`; `link_set` primary/supplementary) + `parts` for display |
| `parent`/`child` (hierarchy) | `hierarchy_occurrences`, `hierarchy_edges`, `hierarchy_concepts` |
| `$subsumes` | `hierarchy_closure` (ancestor → descendant occurrences), `hierarchy_subtree_terms` |
| LP part `$lookup` | `parts` (+ hierarchy for parent/child) |
| LA answers | `answer_list_answers` (display, sequence, score; parent = LL lists) |
| LL answer lists | `answer_lists`, `answer_list_answers`, `loinc_answer_list_links` (answers-for) |
| LG groups | `loinc_groups`, `parent_groups`, `group_loinc_terms` |
| MAP_TO property / local map-to ConceptMap | `loinc_map_to` |
| Designations (translations) | raw `AccessoryFiles/LinguisticVariants/*LinguisticVariant.csv` + `LinguisticVariants.csv` (ID → `ISO_LANGUAGE-ISO_COUNTRY`) |
| ConsumerName designation | raw `AccessoryFiles/ConsumerName/ConsumerName.csv` (and `CONSUMER_NAME` field) |
| Part → external code ConceptMaps | raw `AccessoryFiles/PartFile/PartRelatedCodeMapping.csv` (`ExtCodeSystem`, `Equivalence`) |
| LOINC ↔ IEEE ConceptMap | raw `AccessoryFiles/LoincIeeeMedicalDeviceCodeMappingTable/…csv` |
| LOINC ↔ RadLex (RPID) ConceptMap, playbook ValueSet | raw `AccessoryFiles/LoincRsnaRadiologyPlaybook/LoincRsnaRadiologyPlaybook.csv` |
| Document Ontology ValueSet | raw `AccessoryFiles/DocumentOntology/DocumentOntology.csv` |
| Universal lab orders ValueSet | raw `AccessoryFiles/LoincUniversalLabOrdersValueSet/…csv` (cross-check against `COMMON_ORDER_RANK>0`) |
| Questionnaire | `panel_items` + `answer_list_answers` (answer lists per child) |

Raw table names are `rawCSVTableName(relativePath)` (`internal/loinc/ingest.go:437`). Export a
store helper, `RawTable(relPath) (string, bool)`, that confirms the table exists; never
hard-code the hash suffix. If a raw table is absent, for example in an older DB, the dependent
resource returns 404 `not-found` with a clear message and `metadata` omits it.

Covering indexes added at open: `answer_list_answers(answer_string_id)`,
`loinc_map_to(loinc_num)`, `hierarchy_concepts(loinc_num)` (exists),
`hierarchy_occurrences(code)` (exists). Raw-table indexes are created lazily on first use
(`CREATE INDEX IF NOT EXISTS idx_raw_<table>_<col>`) for the key columns listed in §4.

## 4. FHIR wire contract (parity target: `docs/exemplars/fhir.loinc.org/`)

### 4.0 Conventions

- Response `Content-Type: application/fhir+json;charset=UTF-8`. Accept `_format=json` or
  `application/json`; `_format=xml` or an XML-only Accept → 406 OperationOutcome.
- Codes are case-insensitive on input and normalized to canonical upper case (`lp14449-0` →
  `LP14449-0`).
- `system`, when supplied, must be `http://loinc.org`; otherwise 404 OperationOutcome
  `not-found`, as upstream.
- `version`: absent, the loaded version (`2.82`), or a prefix of it matches. Any other version
  gives the same `not-found` as an unknown code (see
  `codesystem-lookup-version-mismatch.json`), because only one version is loaded.
- Bundles: `type: searchset`, `total`, `link[self]` plus `link[next]` when paged. Paging uses
  `_count` + `_offset`, as upstream (`valueset-search-name-yes.json`). `fullUrl` is built from
  the request's scheme and host (honour `X-Forwarded-Proto`/`-Host`).
- Every resource carries `publisher: "Regenstrief Institute, Inc."`, the upstream `contact`
  block, and the LOINC `copyright` string verbatim from the exemplars. Resource `version` is the
  loaded LOINC version.
- Resource ids are stable and readable, where upstream uses UUIDs: CodeSystem `loinc` and
  `loinc-2.82`; ValueSet = the LL/LG code or named slug; ConceptMap = the slug; Questionnaire =
  the LOINC number. Upstream consumers address resources by canonical `url`, which matches
  exactly. Read-by-id accepts both the bare id and the `-{version}` suffix form
  (`LG51018-6-2.82`), as upstream versioned ids do.
- Parameter order inside `Parameters` and part order follow the exemplars exactly. Tests assert
  order (§9).

### 4.1 `metadata`

- CapabilityStatement: `fhirVersion: 4.0.1`, `kind: instance`, `format: ["application/fhir+json","json"]`,
  `software {name: "loinc-browser", version}`, `implementation {description, url}`. One `rest`
  entry lists CodeSystem, ValueSet, ConceptMap, and Questionnaire with only the interactions we
  serve (`read`, `search-type`), their search params (§1), and their operations with canonical
  `definition` URLs (`http://hl7.org/fhir/OperationDefinition/CodeSystem-lookup`, …).
- `?mode=terminology`: TerminologyCapabilities in the shape of
  `metadata-terminology.json`, with one `codeSystem` entry
  `{uri: "http://loinc.org", version: [{code: "2.82", isDefault: true, compositional: false}]}`
  plus `expansion`, `validateCode`, and `translation` stanzas.

### 4.2 CodeSystem search and read

The shape comes from `codesystem-search-url.json` and the vendored sample. Fields: `url
http://loinc.org`, `identifier` (OID `urn:oid:2.16.840.1.113883.6.1`), `version`,
`name: LOINC`, `title: "LOINC Code System"`, `status: active`, `experimental: false`,
publisher/contact/description/copyright, `caseSensitive: false`, `valueSet:
http://loinc.org/vs`, `hierarchyMeaning: is-a`, `compositional: false`, `versionNeeded: false`,
`content: not-present`. Also `filter[]` (§4.7.1) and `property[]`: all 83 definitions in
upstream order with `code/uri/description/type`, embedded as a Go table generated from the
vendored sample. Search `?url=http://loinc.org` returns the resource once, as entry id
`loinc-2.82`. Read `/CodeSystem/loinc` and `/CodeSystem/loinc-2.82` both return it; upstream
404s the bare `loinc`, and our 200 is a harmless superset.

### 4.3 `$lookup`

In: `code`, `system`, `version`, `coding`, `displayLanguage`, `property` (repeating), `date`
(ignored).

Out order (verified): `code` (valueCode), `system` (**valueString**), `name` (valueString
`LOINC`), `version` (valueString), `display` (valueString), `status` (valueCode), then
`designation*`, then `property*`.

| Code kind | display | status | designations (`use.code`) | properties |
|---|---|---|---|---|
| Term `^\d+-\d$` | `LONG_COMMON_NAME` | `active`; `retired` if DEPRECATED | en-US `LONG_COMMON_NAME`, `FullySpecifiedName` (`COMPONENT:PROPERTY:TIME_ASPCT:SYSTEM:SCALE_TYP[:METHOD_TYP]`), `ConsumerName`, `SHORTNAME`, `DisplayName`; each linguistic variant `{lang}`: `LONG_COMMON_NAME`, `FullySpecifiedName`, `SHORTNAME`, `LinguisticVariantDisplayName` when non-blank | see below |
| `LP…` | `PartDisplayName` or `PartName` | `active`; `retired` if the part status is DEPRECATED | en-US `PartName`, `PartDisplayName` | `PartTypeName`, `STATUS` (string), `parent`/`child` (Coding, hierarchy) |
| `LL…` | answer-list name | `active` | en-US `AnswerListName` | `answers-for` (Coding, one per linked term) |
| `LA…` | answer display text | `active` | en-US `DisplayText` | `Score`, `SequenceNumber` (strings, first occurrence), `parent` (Coding → each containing LL) |
| `LG…` | group name | `active` | en-US group name | `parent` (parent LG), `child` (member terms), `STATUS` |

Term properties follow the order in `codesystem-lookup-718-7.json`:

1. Primary-link axes as `valueCoding` (`SYSTEM`, `TIME_ASPCT`, `PROPERTY`, `SCALE_TYP`,
   `METHOD_TYP`, `CLASS`, `COMPONENT`, each resolved via `loinc_part_links`, `display` =
   `PartName`).
2. Supplementary-link Coding properties (`category`, `system-core`, `analyte-core`, `analyte`,
   `time-core`, …; the property code is the last path segment of
   `loinc_part_links.property`).
3. Non-blank string fields from `Term.Fields` in fhir.loinc.org property-code spelling
   (`CLASSTYPE`, `COMMON_TEST_RANK`, `SHORTNAME`, `LONG_COMMON_NAME`, `VersionLastChanged`,
   `STATUS`, `VersionFirstReleased`, `CHNG_TYPE`, `UNITSREQUIRED`, `RELATEDNAMES2`,
   `EXAMPLE_UNITS`, `DisplayName`, `EXAMPLE_UCUM_UNITS`, `ORDER_OBS`, …). Map CSV column →
   property code via the 83-entry table; for example `DefinitionDescription`,
   `EXMPL_ANSWERS`.
4. `MAP_TO` (Coding) per `loinc_map_to` target, `answer-list` (Coding) per linked LL.
5. `parent` (Coding) per hierarchy parent occurrence, plus a `parent` Coding per containing LG
   group (`codesystem-lookup-30064-0-parent.json`).

Blank fields are omitted. Integer-looking fields stay `valueString`, as upstream. `property=`
filters to the requested codes, in the same order; unknown property codes are ignored.
`displayLanguage=xx-YY` restricts designations to en-US plus that language; if the language has
a `LONG_COMMON_NAME` variant it becomes `display`.

Errors: unknown code or version mismatch → **404** `OperationOutcome` `issue[0] =
{severity: error, code: not-found, details.text, diagnostics}` with text `Unable to find code for
system/version = {code}`.

### 4.4 CodeSystem `$validate-code`

In: `url` (or `system`), `code`, `version`, `display`, `coding`, `codeableConcept` (first LOINC
coding wins), `displayLanguage`.

Out (verified order): `result` (**valueString** `"true"`/`"false"`), then `message` when false,
`code` (valueString), `display`, `active` (valueBoolean; false for DEPRECATED), `system`
(valueString), `version`. Always HTTP 200.

- Unknown code → `result "false"`, message `The code does not exist for the supplied code system
  and/or version`, then `system`, `version` only.
- Display supplied and not case-insensitively equal to any designation value of the code →
  `result "false"`, message `The code exists but the display is not valid`, plus the other
  parameters (see `codesystem-validate-code-718-7-baddisplay.json`).
- All five code kinds validate.

### 4.5 `$subsumes`

In: `codeA`, `codeB`, `system`, `version`, `codingA`, `codingB`.

Out (verified): `outcome` (**valueString**), `system` (valueString), `version` (valueString).

- `equivalent` when A == B (case-insensitive).
- `subsumes` when some hierarchy occurrence of A is an ancestor of an occurrence of B.
  Terms sit only at leaves: use `hierarchy_subtree_terms` for term B and `hierarchy_closure`
  for part B.
- `subsumed-by` for the reverse; otherwise `not-subsumed`.
- Unknown A or B → **400** OperationOutcome `code: invalid`, text `Code does not exist for code
  system ={code},http://loinc.org`.
- Missing codeA or codeB → 400 `required`.

### 4.6 ValueSet catalogue, search, and read

#### 4.6.1 Served value sets

| Canonical URL | id | Kind | Source | Definition (`compose`) |
|---|---|---|---|---|
| `http://loinc.org/vs` | `loinc-all` | intensional | all terms | `include[{system}]` |
| `http://loinc.org/vs/{LL}` | `{LL}` | extensional | answer list | `include.concept[]` in sequence order |
| `http://loinc.org/vs/{LG}` | `{LG}` | extensional | group members | `include.concept[]` |
| `http://loinc.org/vs/{LP}` (HL7 implicit, multiaxial) | `{LP}` | intensional | hierarchy | `filter ancestor = {LP}`; `name` "LOINC Value Set from Multi-Axial Hierarchy code {LP}" |
| `…/vs/loinc-document-ontology` | same | extensional | DocumentOntology distinct `LoincNumber` | concept list |
| `…/vs/loinc-rsna-radiology-playbook` | same | extensional | Playbook distinct `LoincNumber` | concept list |
| `…/vs/top-lab-orders` | same | extensional | `COMMON_ORDER_RANK > 0` ordered by rank | concept list |
| `…/vs/loinc-top-ranked` | same | intensional | `COMMON_TEST_RANK > 0` (Top 20,000) | filter `COMMON_TEST_RANK` |
| `…/vs/deprecated-loinc-terms` | same | extensional | `STATUS = DEPRECATED` | concept list |
| `…/vs/valid-hl7-attachment-requests` | same | intensional | `ValidHL7AttachmentRequest = Y` | filter |
| `…/vs/valid-hl7-attachment-responses[-ig-exists\|-no-ig-exists]` | same | intensional | `HL7_ATTACHMENT_STRUCTURE` non-blank / `IG exists` / `NoIGexists` | filter |
| `…/vs/loinc-universal-order-set` | same | extensional | LoincUniversalLabOrdersValueSet.csv | concept list |
| `…/vs/loinc-imaging-document-codes` | same | extensional | ImagingDocumentCodes.csv | concept list |

`name`, `description`, `identifier`, and `experimental` for named sets are copied from the
matching exemplar (`valueset-expand-deprecated-count3.json`, `…-top-lab-orders-…`, `…-document-
ontology-…`, `…-rsna-playbook-…`). Named sets without an exemplar get a neutral description.
Answer lists carry their OID `identifier` (`urn:oid:{LoincAnswerListOid}`) and `name` = the
list name, as upstream (`valueset-read-LL1162-8.json`).

#### 4.6.2 Not served

The data for these is not in the release: `functional-status`,
`loincs-related-to-public-health-case-reporting`, `loinc-rsna-radiology-playbook-core`.
Requests return 404 `not-found` naming the set. Parent-group (`LG…` from ParentGroup.csv)
ValueSets are served as `compose.include.valueSet[]` of their child groups, and expanding them
flattens to member terms. Upstream lists this as a known bug; ours works.

#### 4.6.3 Search and read

- Search supports `url`, `name` (prefix, case-insensitive), `name:in` / `name:contains`
  (substring — upstream's `name:in=Yes` behaves as contains), `_id`, `_count` (default 20, max
  100), and `_offset`.
- Name search covers answer lists and groups (the large catalogues) plus named sets.
- Read returns the definition only. The size policy mirrors upstream, which embeds the full
  concept list even for 6k-member sets: embed `compose.include.concept` when the member count is
  ≤ 10,000; otherwise emit the intensional `filter` form.
- `_summary` and `_elements` are supported (§4.13). Upstream rejects both with 400; that is a
  divergence (§7). Other unknown `_`-prefixed result parameters → 400 `not-supported` with text
  `Input parameter '{p}' is not supported`, as upstream.

### 4.7 `$expand`

In: `url`, `valueSet` (POST inline resource), `valueSetVersion`, `filter`, `offset`, `count`,
`activeOnly`, `includeDesignations`, `displayLanguage`, `excludeNested` (ignored; flat always),
`date` (ignored).

Out (verified, `valueset-expand-LL1162-8.json`): the ValueSet definition fields, including
`compose` when the definition embeds it, plus:

```json
"expansion": {
  "id": "<uuid>", "identifier": "<uuid>", "timestamp": "<RFC3339 seconds, +00:00>",
  "total": 5, "offset": 0,
  "parameter": [ {"name":"offset","valueInteger":0}, {"name":"count","valueInteger":100} ],
  "contains": [ {"system":"http://loinc.org","code":"LA137-2","display":"None"} ]
}
```

- `total` is always present, including on count-truncated pages, as upstream
  (`valueset-expand-deprecated-count3.json`: `total 6018`, 3 `contains`). `offset` is always
  present. `parameter` echoes the effective `offset` and `count`.
- `count` default 100 (upstream default), max 1000; `count=0` → no `contains`, `total` still
  set; negative or non-integer → 400 `invalid`.
- Order: answer lists by sequence; named sets, all-LOINC, and LP implicit sets ascending by
  numeric LOINC (`CAST(substr(...))`, so `2-1 < 10-1`). Stable across pages.
  **Correction (found implementing P2):** LG groups are the one exception — captured
  `valueset-expand-LG9568-9.json` orders `contains` `25681-8, 40635-5, 6785-0, 6787-6`, which is
  plain **text** `loinc_num` order (matching `group_loinc_terms`'s primary-key order), not numeric
  (`6785-0 < 6787-6 < 25681-8 < 40635-5`). The original bullet's blanket "groups ... ascending by
  numeric LOINC" was wrong; groups use text order for both `compose.include.concept` and
  `expansion.contains`.
- `filter`: case-insensitive, word-prefix match on display (all tokens). Use `loinc_terms_fts`
  for term sources and `LIKE` for small sources (answer lists, groups).
- `activeOnly=true` excludes DEPRECATED (HL7: DEPRECATED is the only inactive status;
  TRIAL/DISCOURAGED stay active). Default is false, so the full set expands. With
  `activeOnly=false`, a DEPRECATED member carries `"inactive": true` in `contains`.
  This resolves the old draft's contradiction: FHIR semantics, not the UI search default,
  govern `$expand`.
- `includeDesignations=true` / `displayLanguage` add `contains[].designation[]` from §4.3.
- Unknown LL/LG/named URL → 404 `not-found`, text `Failed to find matching value set`.
  Upstream returns an empty 200 for unknown LL codes; the spec-correct 404 is a documented
  divergence (§7).

#### 4.7.1 Inline / intensional composition (POST `valueSet`)

Supports `compose.include[]` / `exclude[]` with `system: http://loinc.org`, and either
`concept[]` or `filter[]`. Filters, per HL7 and the upstream CodeSystem `filter` list:

| property | op | value |
|---|---|---|
| any of the 83 property codes (e.g. `COMPONENT`, `CLASS`, `SCALE_TYP`, `STATUS`, `ORDER_OBS`) | `=` / `in` / `regex` | Coding-typed: LP code (`=`) or part name; string-typed: field value |
| `parent` | `=` / `in` | LP code(s): immediate hierarchy children |
| `ancestor` | `=` / `in` | LP code(s): transitive (closure) |
| `child` | `=` / `in` | codes whose child is the value |
| `concept` | `is-a` / `descendent-of` / `is-not-a` | LP code (hierarchy) |
| `copyright` | `=` | `LOINC` (blank `EXTERNAL_COPYRIGHT_NOTICE`) / `3rdParty` (non-blank) |

Include `valueSet[]` references resolve through the catalogue. An unsupported filter gives 400
`not-supported`. Upstream nginx 403s POST `$expand`; the spec behaviour is ours (§7).

### 4.8 ValueSet `$validate-code`

In: `url` / instance id / `valueSet`, `code`, `system`, `display`, `coding`,
`codeableConcept`, `activeOnly`.

Out (verified, note **valueBoolean** here, unlike CodeSystem): found →
`[result true, display]`; not found → `[result false, message "The code '{code}' was not found in
this value set"]`. Display mismatch → `result false`, message
`The code exists but the display is not valid`, `display`. HTTP 200; an unknown value set → 404.

### 4.9 ConceptMap catalogue, search, and read

Direction pairs, all with `url http://loinc.org/cm/{id}`:

| id (forward / reverse) | Source → target system | Source |
|---|---|---|
| `loinc-to-ieee-11073-10101` / `ieee-11073-10101-to-loinc` | `http://loinc.org` ↔ `urn:iso:std:iso:11073:10101` (code = `IEEE_CF_CODE10`, display = `IEEE_REFID`) | IEEE table (`EQUIVALENCE`) |
| `loinc-to-radlex` / `radlex-to-loinc` | ↔ `http://radlex.org` (code = `RPID`, display = `LongName`) | Playbook rows with RPID |
| `loinc-parts-to-radlex` / `radlex-to-loinc-parts` | ↔ `http://www.radlex.org` | PartRelatedCodeMapping |
| `loinc-parts-to-rxnorm` / `rxnorm-to-loinc-parts` | ↔ `http://www.nlm.nih.gov/research/umls/rxnorm` | " |
| `loinc-parts-to-pubchem` / `pubchem-to-loinc-parts` | ↔ `http://pubchem.ncbi.nlm.nih.gov` | " |
| `loinc-parts-to-snomed-ct` / `snomed-ct-to-loinc-parts` | ↔ `http://snomed.info/sct` | " |
| `loinc-parts-to-chebi` / `chebi-to-loinc-parts` | ↔ `https://www.ebi.ac.uk/chebi` | " |
| `loinc-parts-to-ncbi-clinvar` / `ncbi-clinvar-to-loinc-parts` | ↔ `https://www.ncbi.nlm.nih.gov/clinvar` | " |
| `loinc-parts-to-ncbi-gene` / `ncbi-gene-to-loinc-parts` | ↔ `https://www.ncbi.nlm.nih.gov/gene` | " |
| `loinc-parts-to-ncbi-taxonomy` / `ncbi-taxonomy-to-loinc-parts` | ↔ `https://www.ncbi.nlm.nih.gov/taxonomy` | " |
| `loinc-map-to` (local extension, url `http://loinc.org/cm/loinc-map-to`) | LOINC → LOINC replacement | `loinc_map_to` |

- Not served (no release data): `loinc-to-phenx`, the CMS maps (`loinc-to-cms-irf-pai`,
  `-lcds`, `-mds`, `-oasis`), and their reverses. PartRelatedCodeMapping systems
  `fdasis.nlm.nih.gov` and `genenames.org` have no upstream map id; do not mint any.
- Search shape (verified, `conceptmap-search-url-loinc-to-ieee.json`): entries carry `url`,
  `version`, `title` (= id), `status`, publisher/contact, `sourceUri`, `targetUri`. Search
  results never embed `group`.
- Read `/ConceptMap/{id}` embeds `group[{source, target, element[{code, display,
  target[{code, display, equivalence}]}]}]`, capped at 1000 elements (`ponytail:` cap). Beyond
  the cap, it adds the extension note "use $translate". Search params: `url`, `source-system`,
  `target-system`, `source-code`, `target-code`, `_count`, `_offset`.

### 4.10 `$translate`

In: `url` / instance, `conceptMap`, `code`, `system`, `version`, `source`, `coding`,
`codeableConcept`, `target`, `targetsystem`, `reverse`.

Out (verified): `result` (valueBoolean), then per match `match.part[equivalence (valueCode),
concept (valueCoding {system, code, display}), source (valueUri = map url)]`. When `result` is
false, `message` is `No mapping found matching specified criteria`. Always HTTP 200 when the
map exists.

- Without `url`: search every map whose source system equals `system`, with matches in map
  order. For example `system=http://loinc.org&code=11556-8` → the IEEE match
  (`conceptmap-translate-11556-8.json`), and `code=30657-1` → the RadLex match.
- `reverse=true` on a forward map is identical to translating via its reverse map.
- Equivalence comes from the CSV column (`equivalent`, `narrower`, `wider`, `relatedto`, …).
  RadLex playbook rows have no equivalence column, so emit `relatedto`, as upstream does
  (`conceptmap-translate-30657-1.json`). `loinc-map-to` emits `equivalent` and adds a `comment`
  part when `loinc_map_to.comment` is non-blank.
- An unknown `system` with no map → 404 `not-found`, text `Code system not found matching
  'system' parameter`, as upstream.

### 4.11 Questionnaire (LOINC panels)

Verified shape (`questionnaire-89689-4.json`): `url http://loinc.org/q/{LOINC}`, `version`,
`name` (title sanitized — see below), `title` = panel `LONG_COMMON_NAME`,
`status: draft`, `subjectType ["Patient"]`, publisher/contact, `description`, `copyright` (LOINC
copyright plus `\r\n` plus a PanelsAndForms.csv `EXTERNAL_COPYRIGHT_NOTICE` when present, else its
`AdditionalCopyright` — the exemplar's PROMIS attribution is carried in `AdditionalCopyright`, not
`EXTERNAL_COPYRIGHT_NOTICE`, correcting the earlier draft of this line), `code[{system, code,
display}]`, and `item[]`.

`name` derivation, verified against the exemplar (title "PROMIS cancer item bank - physical
function - version 1.1" → name "PROMIS_cancer_item_bank_physical_function_version"): each run of
non-alphanumeric characters becomes one `_`, the result is capped at 50 characters, and a trailing
`_` left by the cap is trimmed (the cap cuts the exemplar's title mid-word, at "version_1_1" →
"version").

Each `panel_items` child becomes an item:

- `linkId` = PanelsAndForms `ID`; `code[]` = the child term; `text` = `DisplayNameForForm` or the
  child `LONG_COMMON_NAME`.
- `type`: `choice` if an answer list applies, `group` if the child is itself a panel (items
  nest recursively via `parent_id`), `decimal` for `SCALE_TYP=Qn` (the exemplar's own T-score item,
  89690-2, is Qn and comes back `"type": "decimal"`, not `"quantity"` as the earlier draft of this
  line said), otherwise `string`. Only one level of nesting is reconstructed: a child that is
  itself a panel (a distinct LOINC that is some other panel's own parent) recurses via
  `FHIRPanelItems`; a same-LOINC repeat-group (`panel_items` rows whose own container row was
  dropped at ingest because `parent_loinc_num == child_loinc_num`) is not distinguished from a
  top-level item — ponytail: flattened rather than reconstructed, revisit if a served panel needs
  real repeat-group nesting.
- `required` from `ObservationRequiredInPanel` (R → true), `repeats: false`.
- `answerOption[].valueCoding` from `answer_list_id_override` or else the child's linked answer
  list.

No `enableWhen`: upstream skip logic is hand-curated and not in the release. A non-panel LOINC
→ 404.

### 4.12 Errors (all FHIR routes)

`OperationOutcome {issue[{severity: "error", code, details: {text}, diagnostics: text}]}`.
Codes and status as verified: `not-found` 404, `invalid` 400, `required` 400,
`not-supported` 400, `not-supported` 406 for XML. A store that is not loaded → 503 `exception`.

### 4.13 `_summary` and `_elements` (all resource-returning interactions)

These are the FHIR R4 search result parameters
([search.html#summary](https://hl7.org/fhir/R4/search.html#summary),
[#elements](https://hl7.org/fhir/R4/search.html#elements)). They apply to read, search, and
the resource returned by `$expand` (a ValueSet). `Parameters` results ($lookup, $validate-code,
$subsumes, $translate) are returned unchanged, as the spec leaves them out of scope.

- The summary element set per resource type is taken verbatim from R4 `isSummary=true`
  flags, vendored at `docs/vendor/hl7/r4-summary-elements.json`. That file also lists the
  mandatory (`min>0`) top-level elements. Embed it as a Go table and never hand-pick elements.
- `_summary=true`: keep only summary elements. `resourceType`, `id`, and `meta` are always kept.
  Note that ValueSet `compose`/`expansion` and ConceptMap `group` are *not* summary elements.
- `_summary=text`: `text`, `id`, `meta`, and mandatory top-level elements.
- `_summary=data`: everything except `text`.
- `_summary=count`: searches only. A Bundle with `type`, `total`, and `link[self]`, and no
  `entry`. On read this is a 400 `invalid`.
- `_summary=false`: full resource (the default).
- `_elements=a,b,c`: keep the named top-level elements plus `resourceType`, `id`, `meta`, and
  the mandatory elements. Unknown names are ignored. Combining it with `_summary` → 400
  `invalid`.
- Bundles: the parameters apply to each `entry.resource`; the Bundle envelope is untouched.
  Filtered resources get
  `meta.tag[{system: "http://terminology.hl7.org/CodeSystem/v3-ObservationValue", code: "SUBSETTED"}]`,
  as the spec requires.
- Implementation: one post-processing step in `internal/fhirhttp` on the marshalled resource
  (top-level key filter; a nested `_elements` path is not supported). Apply it before writing
  and push nothing into `pkg/terminology`. For large `$expand` pages, `_elements=url,name`
  skips the `contains` serialization cost only if filtering happens before marshal. That's
  acceptable either way (ponytail: filter after marshal; optimize if measured).

## 5. Local Search API (`/searchapi/{scope}`)

Parity target: `docs/exemplars/searchapi/`. Params: `query`, `rows` (default 20, max 500),
`offset`, `sortorder` (field name plus optional ` asc`/` desc`; `loinc_num` etc.), `language`
(LinguisticVariants `ID`), and `includefiltercounts`. Execution reuses the existing Bleve
local-search service (`internal/server/local_search.go`, same parser and field aliases).
A missing index → 503 JSON `{"Message": "local search index not built; POST /api/v1/local-search/rebuild"}`.

Response (verified):

```json
{
  "ResponseSummary": {
    "RecordsFound": 1027, "StartingOffset": 0, "RowsReturned": 2, "LoincVersion": "2.82",
    "Copyright": "…", "QueryUrl": "<this request URL>",
    "Next": "<url offset+rows, when more>", "Previous": "<url offset-rows, when offset>0>",
    "QueryExecutionTime": "<RFC3339 local>", "QueryDuration": 0.0070191
  },
  "Results": [ … ],
  "FilterCounts": { … }   // only with includefiltercounts=true
}
```

- `loincs` rows: every `Loinc.csv` column in release order, keyed exactly as in
  `loincs-glucose-rows2.json` (`LOINC_NUM` … `COMMON_SI_TEST_RANK` …). Blank → `null`;
  `CLASSTYPE` and ranks as JSON numbers; plus the trailing derived keys present in the exemplar
  (copy the key list from the exemplar; the implementer diffs the key sets).
- `language=N` replaces COMPONENT…LONG_COMMON_NAME and RELATEDNAMES2 with the linguistic
  variant row (`loincs-hemoglobin-lang15.json`).
- `parts` rows: `PartNumber, PartTypeName, PartName, PartDisplayName, Status, Classlist, Link`
  (`https://loinc.org/{PartNumber}`).
- `answerlists` rows: `AnswerListId, Name, Description, LoincAnswerListOid, ExtDefinedYN,
  ExtDefinedAnswerListCodeSystem, ExtDefinedAnswerListLink, Answers[…]`, with each answer's keys
  as in `answerlists-yes-rows2.json`.
- `groups` rows: `ParentGroupId, ParentGroup, GroupId, Group, Archetype, STATUS,
  VersionFirstReleased, UsageNotes, MolecularWeightOfAnalyte, Category, Loincs[{LoincNumber,
  LongCommonName}], Link`. Attribute fields come from raw `GroupAttributes.csv`.
- `FilterCounts`: facet arrays `[{Label, Search, Count}]` per key in
  `loincs-glucose-filtercounts.json` (Status, Class, … ), computed over the full hit set.
  Implement the keys the local index can supply and document any omitted ones.
- Basic-auth headers are accepted and ignored. The response is `application/json;
  charset=utf-8`.

## 6. UDP micro-protocol (Mode E)

One JSON request datagram → zero or one JSON response datagram (≤ 1400 B each). The client
supplies `id`; the server echoes it. Every op is an idempotent read, so blind retries are safe.

```json
{"id":"42","op":"lookup","code":"718-7"}
{"id":"43","op":"validate","code":"718-7","display":"Hemoglobin [Mass/volume] in Blood"}
{"id":"44","op":"subsumes","codeA":"LP384441-4","codeB":"30064-0"}
{"id":"45","op":"translate","code":"11556-8"}
{"id":"46","op":"expand","url":"http://loinc.org/vs/LL1162-8"}
```

```json
{"id":"42","ok":true,"code":"718-7","display":"Hemoglobin [Mass/volume] in Blood","status":"ACTIVE","class":"HEM/BC","version":"2.82"}
{"id":"43","ok":true,"result":true}
{"id":"44","ok":true,"outcome":"subsumes"}
{"id":"45","ok":true,"matches":[{"system":"urn:iso:std:iso:11073:10101","code":"160116","display":"MDC_CONC_PO2_GEN","equivalence":"equivalent"}]}
{"id":"46","ok":true,"total":5,"contains":[{"code":"LA137-2","display":"None"}]}
{"id":"47","ok":false,"error":"not-found","code":"9999-9"}
{"id":"48","ok":false,"truncated":true,"error":"use-http","href":"/fhir/ValueSet/$expand?url=http%3A%2F%2Floinc.org%2Fvs%2FLL1000-0"}
```

- Ops and outcome vocabulary match §4.3–§4.10 exactly, via the same `pkg/terminology` calls.
- A response over 1400 B → `truncated:true, error:"use-http"` plus the URL-encoded HTTP path.
  Never send a silent partial payload.
- Malformed JSON → no response. Unknown op or bad params → `{"id":…,"ok":false,"error":"bad-request"}`.
- Off by default. `--udp-addr` / `LOINC_BROWSER_UDP_ADDR` (e.g. `:8081`).

## 7. Documented divergences from fhir.loinc.org

| Area | Upstream | Local | Why |
|---|---|---|---|
| Resource ids | UUIDs | readable ids (`LL1162-8`, `loinc-2.82`) | canonical `url` matches; upstream also accepts these as read ids |
| `CodeSystem/loinc` read | 404 | 200 | harmless superset |
| `http://loinc.org/vs` (all LOINC) and implicit `vs/{LP}` | 404 "Failed to find matching value set" | served | defined by the CodeSystem (`valueSet`) and by HL7 |
| `$expand` of unknown LL | empty 200 | 404 `not-found` | spec-correct |
| `$expand` with `filter` / `offset` on LL | 404 (upstream bug) | works | spec-correct |
| POST inline `valueSet` `$expand` | nginx 403 | works | spec-correct |
| Reverse `$translate` (`…-to-loinc` maps, `reverse=true`) | 404 | works | maps are published as bidirectional |
| Parent-group expansion | known failing | works | |
| Loaded versions | 2.69–2.83 | the one imported release | single-DB design |
| Designation languages | from the upstream DB | from the local release LinguisticVariants | same source files |
| `_summary` / `_elements` | 400 `not-supported` | supported (§4.13) | FHIR R4 search result params; clients that never send them see no change |

Upstream quirks we **keep** for client compatibility: `valueString` booleans (CodeSystem
`$validate-code` `result`, `$subsumes` `outcome`), `system` as `valueString` in `$lookup`,
parameter orders, and error texts.

## 8. Implementation phases (agent-sized, file ownership)

Package layout:

- `pkg/terminology`: the Mode A public API and the single code path. A `Service` wraps a store
  getter func. `Lookup`, `ValidateCode`, `Subsumes`, `Expand`, `ValidateValueSetCode`,
  `Translate`, `Questionnaire`, `ReadValueSet`, `SearchValueSets`, `ReadConceptMap`,
  `SearchConceptMaps`, `CodeSystemResource`, `Capabilities` return FHIR-shaped Go values (`fhir.go`: `Parameters`,
  `Parameter{Name, Value*, Part}` with ordered `MarshalJSON`, and `Resource map`-free structs).
  Errors are typed (`*OutcomeError{Status, Code, Text}`).
- `internal/loinc/fhir_queries.go`: read-only store additions (version, raw-table resolver,
  part/answer/group/hierarchy/linguistic-variant/raw-map queries, paged expansion queries).
- `internal/fhirhttp`: HTTP adapters (param parsing from GET and POST `Parameters`,
  `writeFHIR`, OperationOutcome, Bundles, route registration via `Register(mux, svc)`), plus one
  line in `server.New`.
- `internal/server/searchapi.go`: the `/searchapi/*` handlers over the existing local-search
  service.
- `internal/udp` (or `cmd/loinc-browser/udp.go`): the Mode E listener. The UDS listener lives in
  `cmd/loinc-browser`.

| Phase | Scope | Owner files | Done when |
|---|---|---|---|
| P1 | Foundation + CodeSystem: store queries, `pkg/terminology` types, `internal/fhirhttp` skeleton, metadata, CodeSystem search/read, `$lookup` (all five kinds + designations), `$validate-code`, `$subsumes`, errors | `internal/loinc/fhir_queries.go`, `pkg/terminology/{fhir,service,codesystem,properties}.go`, `internal/fhirhttp/*`, 1 line in `internal/server/server.go` | fixture tests + exemplar shape tests green |
| P1b (parallel with P1) | Local Search API | `internal/server/searchapi.go`, `_test.go`, 1 route line in `server.go` | exemplar key-set tests green |
| P2 | ValueSet catalogue, search/read, `$expand` (incl. inline compose + filters), `$validate-code` | `pkg/terminology/valueset*.go`, `internal/fhirhttp/valueset.go` | tests green |
| P3 | ConceptMap catalogue, search/read, `$translate`; Questionnaire | `pkg/terminology/{conceptmap,questionnaire}.go`, `internal/fhirhttp/{conceptmap,questionnaire}.go` | tests green |
| P4 | Transports: `--unix-socket`, `--udp-addr`, env keys, `.env.example`; benchmarks (`go test -bench` over full DB when `LOINC_BENCH_DB` set) | `cmd/loinc-browser/*`, `internal/udp/*`, `pkg/terminology/bench_test.go` | UDS/UDP round-trip tests green; bench numbers recorded in this doc |
| P5 | Docs: `docs/API.md` FHIR + searchapi section, README (base-URL swap, UDS/UDP), CHANGELOG, `docs/agent/LOINC_OFFICIAL_API.md` (local searchapi), `LOINC_AGENT_GUIDE.md` workflow hint, OpenAPI entries for `/searchapi` | docs files, `internal/server/openapi.go` | reviewed |
| P5b | UI exposure: (1) FHIR + `/searchapi` paths in `internal/server/openapi.go` so Swagger (`/api/docs`) lists and tries them; (2) a "Local APIs" view in the web app: operation presets for every §1 route, editable params, GET/POST toggle, shows request URL + copyable curl, pretty JSON response with status/timing, links to the matching exemplar shape; (3) Official API view gets a "Local /searchapi" source toggle for side-by-side comparison | `web/src/lib/components/ApiConsole.svelte`, `web/src/App.svelte` (view wiring only), `web/src/lib/api.ts`, `internal/server/openapi.go` | `npm --prefix web run check && build` green; verified in a browser against the running app |
| P6 | Independent review, fixes, live verification (`go run ./cmd/loinc-browser` + `scripts/fhir-parity.sh` against the real DB) | — | §11 checklist ticked |

## 9. Testing

1. **Fixture tests** extend the existing `writeServerTestRelease` pattern. The fixture gains
   minimal Part, PartLink, hierarchy, answer list, group, MapTo, LinguisticVariants, IEEE,
   PartRelatedCodeMapping, and PanelsAndForms rows. They assert exact values and order.
2. **Exemplar shape-parity tests** (`pkg/terminology/parity_test.go` and
   `internal/server/searchapi_test.go`) run when `docs/exemplars/` and the real DB
   (`LOINC_TEST_DB=./data/loinc-normalized.sqlite`) are present, and `t.Skip` otherwise. For
   each captured call they compare:
   - the ordered list of top-level parameter names;
   - the value[x] key of each parameter and part;
   - the HTTP status and `resourceType`;
   - for Bundles and resources, the key set (JSON-path skeleton).

   Values are ignored except `system` and `name`, because upstream is on 2.83. Divergences in
   §7 are listed in a skip table with the reason.
3. **Live parity script** `scripts/fhir-parity.sh`: replays every URL in
   `capture-exemplars.sh` against `http://localhost:$PORT` and diffs shape skeletons with `jq`.
   This is the manual acceptance step.
4. **Benchmarks** against the full DB, gated by `LOINC_BENCH_DB`.

## 10. Open decisions (defaults chosen; revisit only with evidence)

1. DuckDB mirror: not built. **Re-measured (2026-09-23, post-review-fixes, real 2.82 DB,
   in-process, N=500 warm calls, Apple M5 Pro): not tripped, and the one previously-missed budget
   is now met too.** `$expand http://loinc.org/vs count=1000` p95 = 21.9µs, p99 = 80.3µs — down
   from 8.9ms/9.1ms (§11's named/dynamic ValueSet sources now cache their unfiltered member list
   per Store instead of re-running SQLite's join+dedupe+sort on every request; see
   `loinc.Store.CachedExpandMembers`, `internal/loinc/fhir_valueset_queries.go`). `$expand
   count=100` p95 = 10.6µs, also down from 6.8ms. `$lookup` p95 = 666µs / p99 = 848µs against the
   ≤100µs in-process target — improved from 908µs/1.19ms (root cause fixed: `FHIRLinguisticVariants`,
   `internal/loinc/fhir_queries.go`, now issues one `UNION ALL` query across every
   LinguisticVariants per-language raw table instead of ~20 separate round trips) but still over
   budget; the remaining cost is spread across `$lookup`'s other per-term queries (part links,
   hierarchy parents, groups, MAP_TO, the raw Loinc.csv row), each already using its own index, so
   there is no further single-query fix available without restructuring `TermWithAccessories`
   itself (P1's code, out of scope here). Filed as a remaining report; see the results table below
   for every measured op.

### 10.1 Benchmark results

**Re-measured 2026-09-23 after the review-fix pass** (real 2.82 DB, `data/loinc-normalized.sqlite`,
Apple M5 Pro, `go test -bench . -benchtime=50x -benchmem`, in-process unless noted). Original
2026-09-23 numbers are struck through for comparison; see the review items cited per row for what
changed.

| Benchmark | ns/op | allocs/op | Budget (§2/§9/§11) | Verdict |
|---|---:|---:|---|---|
| `BenchmarkLookupTerm` (718-7) | ~~830,178~~ → 609,738–764,735 | ~~2,669~~ → 1,375 | ≤100µs p99 in-process | improved (item 7: `FHIRLinguisticVariants` UNION ALL), still **missed** — remaining cost is spread across `$lookup`'s other per-term queries, not a single fixable hot spot |
| `BenchmarkLookupPart` (LP14542-2) | 59,288–70,329 | 227 | ≤100µs p99 in-process | met |
| `BenchmarkValidateCode` (718-7) | ~~634,265~~ → 471,922–548,015 | ~~2,691~~ → 1,397 | — | tracks `$lookup` (reuses it) |
| `BenchmarkSubsumes` (LP384441-4/30064-0)* | 25,318–62,495 | 88–89 | — | met, well under lookup's cost |
| `BenchmarkTranslateWithURL` (11556-8, named map) | 16,672–18,166 | 76 | — | met |
| `BenchmarkTranslateWithoutURL` (11556-8, catalogue search) | 184,553–201,262 | 539 | — | met (no hard budget); probes every served map |
| `BenchmarkExpandAnswerList` (LL1162-8) | 33,944–36,363 | 224 | — | met |
| `BenchmarkExpandAllCount100` (vs, count=100) | ~~5,513,152~~ → 7,363–8,312 | ~~1,390~~ → 56 | ≤25ms p95 (§11) | met, **~700x faster** (item 6: `loinc-all` is `NoDuplicates` + cached, so a warm request pages an in-memory slice instead of re-running the join) |
| `BenchmarkExpandAllCount1000` (vs, count=1000) | ~~7,073,892~~ → 20,246–25,268 | ~~13,071~~ → 56 | ≤50ms p95 (§10.1 DuckDB gate) | met, not tripped, **~300x faster** |
| `BenchmarkExpandAllCount1000Offset50000` (vs, count=1000, offset=50000) | ~~57,269,323~~ → 21,887–22,124 | ~~12,871~~ → 56 | — | **~2,600x faster**: a deep OFFSET is now a slice of the cached member list, not a SQL `OFFSET` scan |
| `BenchmarkExpandFilterGlucose` (vs, filter=glucose) | 3,425,886–3,997,098 | 1,400–1,401 | — | met; unaffected (a `filter` bypasses the cached-member-list path by design, since results vary per filter) |
| `BenchmarkQuestionnaire` (89689-4) | 5,747,480–6,000,885 | 1,714 | — | met (no hard budget) |
| `BenchmarkHTTPLookup` (HTTP, httptest) | 1,158,136–1,367,549 | 3,886–5,482 | ≤5ms warm (§2 Mode B/C) | met |
| `BenchmarkHTTPExpandCount100` (HTTP, httptest) | ~~5,766,172~~ → 201,338 | ~~1,565~~ → 212 | ≤25ms (§11, HTTP overhead on top) | met, **~28x faster** |
| `BenchmarkUDPLookup` (UDP round trip, loopback) | ~~1,051,890~~ → 864,401 | ~~3,096~~ → 1,496 | ≤1ms (§2 Mode E) | now **met** (was missed by ~52µs); tracks the underlying `$lookup` improvement above |

\* Previously used `LP15946-4` ("Valine", a real `Part.csv` ancestor of `30064-0`) as a
workaround: `LP384441-4` is a hierarchy-only node in the local 2.82 release (no `Part.csv` row —
see `lookupHierarchyOnlyPart`) and `Subsumes.assertCodeExists` didn't resolve that case, 400ing
with "code does not exist". **Fixed** (review item 1): `assertCodeExists` now shares
`codeResolves`'/`partCodeResolves`'s per-kind resolution with `$lookup`, so `LP384441-4` resolves
and the benchmark uses §11's own named pair directly.

Percentile detail (`LOINC_TEST_DB=... go test ./pkg/terminology -run TestTimingPercentiles -v`,
N=500 warm in-process calls; re-measured after the fixes above):

| Op | p50 | p95 | p99 | max |
|---|---:|---:|---:|---:|
| `$lookup 718-7` | 469.9µs–592.5µs | 666.4µs–907.9µs | 847.8µs–1.193ms | 1.184ms–1.470ms |
| `$expand vs count=1000` | ~~7.082ms~~ → 9.958µs | ~~8.943ms~~ → 21.9µs | ~~9.110ms~~ → 80.3µs | ~~9.346ms~~ → 148.6µs |
| `$expand vs count=100` | ~~5.541ms~~ → 5.875µs | ~~6.837ms~~ → 10.6µs | ~~7.027ms~~ → 19.5µs | ~~7.156ms~~ → 117.1µs |
| `$validate-code 718-7` | 478.3µs–570.9µs | 669µs–781.4µs | 852.8µs–944.3µs | 894.6µs–1.382ms |
| `$subsumes LP384441-4/30064-0` | 27µs | 34.1µs | 40.3µs | 65.7µs |

2. Trimmed lookup-only DB for Mode A: deferred until a consumer asks.
3. UDP framing: JSON (debuggable with `nc -u`). Binary is deferred.
4. `$lookup` designation volume: all languages by default, as upstream (~15KB for 718-7);
   `displayLanguage` narrows it.
5. Named value sets without upstream exemplars (`loinc-universal-order-set`,
   `loinc-imaging-document-codes`) are HAPI-loader conventions; keep them, flagged as local.
7. WebSocket transport: **not implemented** (decided 2026-09-23). The lookups are
   request/response. HTTP keep-alive and HTTP/2 already remove connection setup, and UDS/UDP
   cover lowest latency. A WebSocket binding would be a custom protocol that no FHIR client
   speaks. The agent chat streams over SSE with a separate cancel POST
   (`docs/AGENT_CHAT_PLAN.md`).
6. `ConceptMap/$closure`: **not implemented** (decided 2026-09-23). It is stateful: the server
   must remember, per client-named table, which concepts it has already returned. That
   conflicts with the stateless read-only design, and fhir.loinc.org does not offer it either.
   `$subsumes` (pairwise) and `$expand` with `ancestor` / `concept is-a` filters (the whole
   subtree) cover the same need. Revisit only if a consumer needs incremental closure
   maintenance. The cost would be a small closure-table store keyed by `name`, in a separate
   SQLite file so the release DB stays read-only.

### 10.1a Cold-start measurement (2026-09-23)

Measured whether a genuinely cold DB (fresh ingest, none of the lazy `CREATE INDEX IF NOT EXISTS`
indexes yet created by any prior process) makes the *first* `$lookup`/`$expand`/`$translate`/
`/searchapi` call slow, since the real `data/loinc-normalized.sqlite` already carries those indexes
from earlier runs and can't measure this directly. Method: copied the real 2.82 DB (1.98GB) to a
scratch path, dropped every lazily-created index (`ensureFHIRIndexes`, `ensureFHIRValueSetIndexes`,
`ensureRawIndex`'s `idx_raw_csv_*`/`idx_raw_ieee_*`/`idx_raw_playbook_*`/`idx_raw_partmap_*`,
47 indexes total), opened a fresh writable `Store` against the copy (in-process
`go test`, `LOINC_COLDSTART_DB=<copy>`, `pkg/terminology/zz_coldstart_timing_test.go`, since
removed), and timed each op's single first call:

| Op | first-call latency |
|---|---:|
| `$lookup 718-7` | 788–819ms |
| `$expand loinc-rsna-radiology-playbook count=3` | 132–136ms |
| `$translate 11556-8` | 57–89ms |
| `/searchapi` parts query (`SearchAPIPartRow`) | 17µs (no raw-table dependency) |

All four are well under the 2s threshold this item set for moving index creation into ingest, so
**no code change was made**: `createPostImportIndexes` (`internal/loinc/ingest.go`) keeps the
non-FHIR indexes it already builds at ingest time, and the FHIR-specific ones stay lazy
(`ensureFHIRIndexes`/`ensureFHIRValueSetIndexes` at every `OpenStore`, `ensureRawIndex`/
`ensureConceptMapIndexes` on first per-table use) as designed. The ~216s regression this item's
brief cites predates the `idx_*_nc` covering-index fix already recorded in §10.1 above (item 7);
that fix, not index timing, was the actual root cause, and it's already shipped.

## 11. Acceptance checklist

- [ ] Every route in §1 answers. Errors are OperationOutcome with the verified codes and statuses.
- [ ] Shape-parity tests pass for every captured fhir.loinc.org call, except the §7 skip list.
- [ ] `$lookup 718-7` parameter order `code, system, name, version, display, status`,
      designations (≥ en-US five uses + linguistic variants), and axis properties as
      `valueCoding` with LP codes (`COMPONENT → LP14449-0`, `CLASS → LP7803-2`).
- [x] `$subsumes LP384441-4 / 30064-0` → `subsumes`; reversed → `subsumed-by`;
      `718-7 / 718-7` → `equivalent`; unknown → 400 `invalid`. (Fixed 2026-09-23: `LP384441-4` is
      hierarchy-only, and `assertCodeExists` now resolves it via the shared `codeResolves`/
      `partCodeResolves` path; see `TestSubsumesHierarchyOnlyPart`, `BenchmarkSubsumes`.)
- [ ] `$validate-code` (CodeSystem) returns `result` as valueString and `active:false` for
      DEPRECATED `6796-7`.
- [ ] `$expand LL1162-8` → 5 concepts in sequence order, `total 5`, `offset 0`, parameter echo
      `offset`/`count`. `deprecated-loinc-terms count=3` → `total` = local deprecated count,
      with 3 `contains`.
- [ ] `$expand http://loinc.org/vs` paging: disjoint and order-stable across pages, constant
      `total`; `activeOnly=true` excludes DEPRECATED only.
- [ ] `$translate 11556-8` → IEEE `160116`; `30657-1` → RadLex `relatedto`; reverse maps work.
- [ ] `Questionnaire/89689-4` items match the exemplar skeleton (`linkId, code, text, type,
      repeats, answerOption`).
- [ ] `/searchapi/*` response key sets match `docs/exemplars/searchapi/` for all four scopes,
      with filter counts and language.
- [x] One process serves TCP, UDS, and UDP simultaneously, from one store (verified:
      `internal/server/udp_wiring_test.go` `TestServeAllThreeTransportsFromOneStore`,
      `cmd/loinc-browser` wires `--udp-addr`/`LOINC_BROWSER_UDP_ADDR` into `serveUntilShutdown`
      alongside the existing TCP/`--unix-socket` listeners). UDP ops match HTTP outcomes for all
      five ops (`internal/udp/udp_test.go`); `id` is echoed; oversize results give `use-http`
      with a URL-encoded href (`TestExpandTruncatesOversizeResponse`). Not independently
      re-verified here: a UDS `$lookup` being byte-identical to TCP (§1/§4.3 parity is P1's, and
      Mode D already shares the same `http.Handler`, so this is unchanged by P4b).
- [x] Benchmarks recorded (§10.1 results table, re-measured 2026-09-23 after the review-fix
      pass): `count=100` expansion p95 = 10.6µs (was 6.8ms), `count=1000` p95 = 21.9µs (was
      8.9ms) — both far under the 25ms/50ms budgets, via cached named/dynamic ValueSet member
      lists (item 6). Lookup p95/p99 (666µs/848µs in-process, down from 908µs/1.19ms after item
      7's `FHIRLinguisticVariants` fix) is still over the 100µs budget — remaining cost is spread
      across `$lookup`'s other per-term queries, filed as a report rather than fixed in this
      phase (P1's `TermWithAccessories` code). HTTP warm lookup = 1.16ms, under the 5ms Mode B/C
      budget; UDP warm lookup = 864µs, now under the 1ms Mode E budget (was missed by ~52µs).
- [ ] `go test ./...`, `npm --prefix web run check`, and `npm --prefix web run build` are green.
      A live curl pass over all routes against the 2.82 DB passes.
