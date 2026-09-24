# Changelog

## Unreleased

- Added **Find**, the new default screen for people who need a code rather than a tour of LOINC (`docs/FIND_MODE_PLAN.md`). Pick a domain (Lab tests, Lab panels & orders, Radiology, Documents & summaries, Forms & assessments, Vitals, Everything) and type a local name; results show the six axes and the CLCI name. It is keyboard-first: `/` focuses search, ↑/↓ move, Enter opens, `c` copies the code. The term card copies the code as code only, `code | name`, FHIR `Coding` (with the LOINC release as `version`), or HL7 v2 CWE. A deprecated term shows its replacement one click away. **Similar terms** lists the analyte's variants with only the differing axes highlighted, followed by the panels the term is in or its members. A basket collects terms across searches and exports CSV. The hierarchy, facets, relationships, and other explorer modes are unchanged, one tab over; `/?mode=hierarchy` and older deep links still open them.
- Added **My setup** in Find, saved in the browser. Pick a language (any of the 21 LOINC translations in the release) to show its name under each English name; copying still uses the official English name. Show only codes from the loaded common-codes list, show only orderable lab codes (Universal Lab Orders), and choose the default copy format for the `c` key and the term card.
- The Common Lab Codes for India loader now takes any code list: set `LOINC_COMMON_CODES_CSV` (any CSV with a LOINC code column and a name column, header optional) and optionally `LOINC_COMMON_CODES_LABEL`. Its codes rank first, match the list's names, and carry `localName`. CLCI stays the default when present, and `clciName`/`clci=true` are unchanged; `commonCodes=true` is an alias.
- Term search, panel search, term detail, and `/api/v1/terms/match` accept `lang=de-DE` (etc.) and return `localizedName`. With `lang`, word search also matches that language's names ("Natrium" with `lang=de-DE` finds 2951-2), using a translated-names index (about 220 MB, built in about 7 s) that the server adds to the database at startup. `/api/version` lists `languages` and the loaded `commonCodes` label and count.
- Added **Map a list**: paste or upload (CSV/TSV/TXT) a lab test master and get LOINC suggestions grouped as Confident, Needs a look, and No match. Pick among up to 5 candidates for the uncertain names, then download the original columns plus `loinc_code`, `loinc_long_common_name`, and `match_status` (`confident`, `reviewed`, `pending` for names still to look at, `unmapped`). Cells starting with `=`, `+`, `-`, or `@` are exported with a leading `'`, so a spreadsheet can't run them as formulas. Work in progress is kept in the browser. Excel `.xlsx` files are read in the browser, first sheet, with no new dependency; old `.xls` files are not.
- Find keeps its query and domain in the URL (`?mode=find&q=…&domain=…`), so Back, Forward, and shared links restore the search.
- Map a list's **Confident** group is stricter. It now takes a typed LOINC number, a common-codes name match, or a commonly used term clearly ahead of the runner-up. A lone search result no longer counts. On a real 6,761-name hospital test master, a judged sample of confident picks went from 80% to 98% correct; about 12% of names were confident before, and fewer are now. The rest are listed for review with their candidates.
- Added `POST /api/v1/terms/match` (up to 1,000 names per request, the same filters as term search; `q`, paging, `sort`, and `mode` are set per name), which returns up to 5 candidates and a `confident`/`review`/`none` bucket per name. Against all 1,264 CLCI General Names, 1,263 land `confident` with the CLCI code (0.1% wrong-confident).
- Term search drops a leading lone "S."/"S" ("S. Creatinine", "S Sodium"), which Indian test masters put before serum tests: as a word it prefix-matched half of LOINC, so "S. Creatinine" listed GFR first; it now lists 2160-0. Only a leading S is dropped ("Protein S" is unchanged), and it isn't read as "serum", so "S. aureus" still finds Staphylococcus aureus.
- "with contrast" in a request now means LOINC's "W contrast": "ct chest with contrast" lists 24628-0 first. Before, "with" was dropped as a stop word, and the bare "contrast" ranked "CT Chest WO contrast" second.
- Term search: `componentFamily=true` widens `component` to its ratio forms (HbA1c 41995-2 also lists 4548-4, the % form); results now include `timeAspect`. `/api/version` reports `loincVersion` (e.g. `2.82`).
- Added Common Lab Codes for India (CLCI, the NRCeS national LOINC subset) as an optional data file: when `common-lab-codes-for-india.csv` is in the data directory (or `LOINC_CLCI_CSV`), word search ranks its 1,473 codes higher, `clci=true` keeps only them (HTTP and MCP), and results, term detail, and MCP candidates carry `clciName`, the name Indian labs use. A query matching a CLCI General Name (≥75% of words shared) lists that name's codes first and returns them in `clciMatches`. See `docs/agent/LOINC_CLCI.md`.
- "blood" in a search now also matches Serum or Plasma terms, ranked below true blood terms: Indian lab names say "Blood" for serum tests (578 of CLCI's 758 ", Blood" names are Ser/Plas codes). Searching the CLCI names, the right code is in the top 10 for 85% (was 50%) without the CLCI prior, 99% with the prior and name match.
- `widal` now searches "Salmonella typhi" or "Salmonella paratyphi" instead of any "typhi", which also matched Rickettsia typhi (typhus).
- On the external mapper's 6,581-name compendium (rc6 → this build, with the mapper's own unit and meaning-search tiers): exact 84 → 125, unmapped 823 → 593. Exact + high stayed level (566 → 564); the unmapped names mostly gained a low-certainty candidate for review.

## 0.93-rc6 - 2026-09-24

- Added request shorthands LOINC doesn't use, replaced by the LOINC wording before searching: `usg`→US ("usg abdomen" → 24558-9), `ncct`/`nect`→CT WO contrast ("ncct neck" → 36514-8, not the contrast-unspecified 36051-1), `cect`→CT W contrast, `esr`→Sed Rat (→ 4537-7, not the ESR1 gene), `pcv`→hematocrit (→ 4544-3, not penciclovir), `dc`→differential count, `mp`→malaria parasite, `lft`/`rft`/`kft`→liver/renal function panels, `widal`→Typhi, `fungal`→fungus, `sugar`→glucose, `ict`/`dct`→indirect/direct antiglobulin (not "icteric"), `lgm`→IgM (typo), `ada`→adenosine deaminase (the enzyme; "ada gene" now also returns the enzyme tests). Responses list them in `synonyms`.
- Fixed relevance ranking's popularity boost, which SQLite computed with integer division: every term with common rank below 200 got the same full boost, so rank never ordered near-tied results. A bare "urine" now lists Color, Appearance, Ketones, and Culture (ranks 64–85) before rank-161 sediment terms.
- Added mapping context filters on term and panel search (HTTP and MCP): `component` (every variant of one analyte, e.g. TSH 3016-3 / 11579-0 / 11580-8 or quantitative vs qualitative troponin), `contains` (panels holding every listed term, e.g. PT + INR → PT panel 34528-0), `universalLabOrders=true`, and radiology filters on the RSNA playbook (`radModality`, `radRegion`, `radFocus`, `radLaterality`, `radContrast`, `radSubtype`, `radView`).
- Added `docs/agent/LOINC_MAPPING_GUIDANCE.md` (MCP topics such as `units_and_property`, `calculated_vs_measured`, `coagulation_specimens`): lab-mapping rules summarized from LOINC's Top 2000 mapper's guide, with codes checked against 2.82. `scripts/extract-top2000-mapper-guide.py` extracts the guide's per-test example units and comments from a local copy of the PDF; when that CSV is in `<data dir>/common_codes/` (or `LOINC_MAPPER_GUIDE_CSV`), term results, term detail, and MCP candidates carry `exampleUcum` and `mapperComment`.
- Word search maps "phosphorus" to LOINC's "Phosphate" (2777-1).
- Added meaning-based term search: `mode=semantic` (nearest terms by meaning) and `mode=hybrid` (meaning and word rankings merged) on `/api/v1/terms/search`, `/api/search`, MCP `loinc_search_terms`, and the UI (**Match: Words / Both / Meaning**). Uses any OpenAI-compatible embeddings endpoint (`LOINC_EMBEDDING_URL`, e.g. LM Studio); vectors are stored int8 in `<data dir>/loinc-embeddings.sqlite` and searched in memory. `GET /api/v1/semantic/status` and `POST /api/v1/semantic/rebuild` (background, resumable) manage the index. All term filters apply. Meaning ranking adds a popularity prior (common test rank); about 35 common tests also embed hand-written lay phrases ("average blood sugar", "bad cholesterol"); changing them marks the index stale and a rebuild re-embeds only those terms. On 20 held-out plain-language lab queries the right term is in the top 3 for 16 (meaning) and 11 (hybrid) vs 9 (words).
- Term search responses list `ignoredWords` (stop and generic words skipped).
- Measured against a 6,581-name lab/radiology compendium mapped by an external mapper (rc5 → this release): exact + high-certainty picks 531 → 566, low 2,371 → 2,328, unmapped 828 → 823. Two-test orders now resolve to panels (PT + INR → 34528-0 via `contains`) and "S. phosphorus" to 2777-1.

## 0.93-rc5 - 2026-09-23

- Generic request words (`routine`, `examination`, `exam`, `test`, `level`, `estimation`, `analysis`, `study`, `assay`, `investigation`, `report`) are now ignored like `for`/`the`: "potassium test" returns 2823-3 first instead of odd terms that happen to say "test", and "urine culture routine" returns 630-4.
- The relaxed retry never drops specimen words (blood, urine, serum, plasma, CSF, sputum, stool, fluid, ...), so a query can't silently switch specimens; rc4 turned "urine routine examination" into "routine examination" junk.

## 0.93-rc4 - 2026-09-23

- Added a `classType` filter (LOINC CLASSTYPE: `lab`, `clinical`, `attachment`, `survey`) to term search in the UI (new Type filter), `/api/v1`, `/api/search`, and MCP `loinc_search_terms`; `class` can now repeat to match several classes (previously the second value was silently ignored). Invalid `classType` returns 400.
- Relevance ranking now blends the text match with common test/order rank and demotes TRIAL, DISCOURAGED, and panel terms (unless the query asks for a panel). Checked by `TestRankingAgainstMappingProbe` against 20 real mapping queries (run with `LOINC_TEST_DB`); all put the common LOINC term in the top 3 except where LOINC lacks the synonym (ESR, Widal).
- The relaxed retry now drops the most common words first, so the analyte is kept ("hba1c fasting" drops "fasting"), and returns nothing instead of a generic list when only common words would remain ("widal test").
- Documented lab-compendium mapping patterns (`docs/USE_CASES.md` §7) and lab/radiology narrowing (§15).

## 0.93-rc3 - 2026-09-23

- HTTP servers (TCP and Unix socket) now time out header reads after 10 s and idle keep-alive connections after 120 s; `docs/USE_CASES.md` §3 documents client connection pooling.
- Term search (UI, `/api/v1`, `/api/search`, MCP `loinc_search_terms`) now hides DEPRECATED terms by default, as documented; the old default excluded an `INACTIVE` status that LOINC releases don't use, so 4,991 deprecated terms appeared in every default search. `status=DEPRECATED` and `status=*` still browse them, and a typed LOINC number finds its term whatever its status.
- When no term matches every word, term search drops as few words as possible (keeping the version that finds the most terms) and returns `relaxed`, `droppedWords`, and a `notice`; the UI shows the notice.
- Term search ranks whole-word matches (e.g. an abbreviation like `CRP` in related names) above prefix-only matches, and weights relevance by field (name and component over related names over definition).
- MCP `loinc_search_terms` results include a `relevance` score.

## 0.93-rc2 - 2026-09-23

- Common English words (`for`, `of`, `the`, `in`, …) no longer become required search terms: "glucose for blood" now matches the same 114 terms as "glucose blood" in UI/`/api/v1` term search (previously 29) and `/searchapi`/Advanced search (previously 0). Quoted phrases, field names, and "a"/"no" are untouched.
- Fixed the local search index reporting an interrupted build as ready: rebuilds now build beside the live index and swap it in when finished, so the old index keeps answering and a failed build never replaces it. Status reports `incomplete` (queries refused) for an index without the completion marker, `stale` (queries still answered) for one built before the current import, and `building` while a rebuild runs.
- A release uploaded through the UI now rebuilds the local search index automatically in the background.
- Advanced search shows a notice for building, stale, and incomplete indexes and polls status while a build runs.
- Embedded `docs/*.md` and `docs/agent/*.md` in the binary; when the docs directory is missing (packaged installs), startup copies them to `<data dir>/docs/` so MCP concept tools and `/docs/*` pages work.
- Added a project `.mcp.json` registering the running server with Claude Code, and MCP client setup docs (Claude Code scopes, stdio vs HTTP, Claude Desktop) in `docs/MCP.md`, `docs/DEPLOYMENT.md`, and the README.

## 0.93-rc1 - 2026-09-23

- Added a per-user data directory: `LOINC_BROWSER_DATA_DIR`, else `./data` when present, else `~/Library/Application Support/loinc-browser` (macOS), `%AppData%\loinc-browser` (Windows), or `$XDG_DATA_HOME`/`~/.local/share/loinc-browser` (Linux). Database, uploads, search index, app key, and settings all live there; startup prints it, auto-import also looks for `Loinc*.zip` there, and a `.env` there is loaded. `ingest` now loads `.env` too.
- Added `--no-official`/`LOINC_OFFICIAL_DISABLED` to turn off the online Search API proxy (403), an optional `LOINC_OFFICIAL_PASSPHRASE` required as the `X-Loinc-Passphrase` header on official search and credential delete (401 otherwise; the UI prompts for it), and `LOINC_OFFICIAL_USERNAME`/`LOINC_OFFICIAL_PASSWORD` environment credentials that take precedence over the encrypted vault.
- Fixed the server failing to start on an empty (not yet imported) database; store-open FHIR indexes are now skipped until the schema exists.
- Added `.github/workflows/ci.yml` (vet, tests, web check and build on every push and pull request); release tags containing `-rc` publish as GitHub pre-releases.
- Added `docs/DEPLOYMENT.md` (macOS launchd, Linux systemd, Windows, data directory, search index, network exposure, CI/CD) and a README deployment section.
- Added `make use-cases` (`scripts/check-use-cases.sh`), which runs every `curl` example in `docs/USE_CASES.md` against a running server; corrected that doc's latency table, Unix socket example, and hierarchy `$expand` example.
- Added a local FHIR R4 terminology API at `/fhir` wire-compatible with fhir.loinc.org: `metadata` (CapabilityStatement/TerminologyCapabilities), `CodeSystem` `$lookup`/`$validate-code`/`$subsumes` over all five LOINC code kinds (terms, LP parts, LL answer lists, LA answers, LG groups), `ValueSet` catalogue reads plus `$expand`/`$validate-code` including inline `compose` filter expansion backed by real regex evaluation, `ConceptMap` search plus `$translate` (forward and reverse), `Questionnaire` reads for panels/forms, and `_summary`/`_elements` result shaping on every read/search/`$expand`. Answered entirely from the local normalized SQLite database; not affiliated with or endorsed by Regenstrief.
- Added a LOINC Search API-compatible `/searchapi/{scope}` (`loincs`, `parts`, `answerlists`, `groups`) over the local Bleve index, matching upstream's request/response shape (`ResponseSummary`, paging, `sortorder`, `includefiltercounts`).
- Added `--unix-socket`/`LOINC_BROWSER_UNIX_SOCKET` and `--udp-addr`/`LOINC_BROWSER_UDP_ADDR` transports (Modes D/E) alongside the default TCP listener, sharing the same handler tree; the UDP micro-protocol answers compact JSON datagrams for lookup/validate/subsumes/translate/expand and falls back to `use-http` for oversize responses. `serve` now shuts down all listeners gracefully on SIGINT/SIGTERM.
- Added ten MCP tools (`loinc_lookup_code`, `loinc_validate_code`, `loinc_subsumes`, `loinc_expand_value_set`, `loinc_search_value_sets`, `loinc_validate_value_set_membership`, `loinc_translate`, `loinc_list_concept_maps`, `loinc_get_questionnaire`, `loinc_lucene_search`) that wrap the local FHIR terminology service and Search API with compact, agent-friendly output; `loinc_lucene_search` now also works over stdio via `--search-index-path` on the `mcp` subcommand (previously HTTP-only), and `loinc_translate` defaults `system` to `http://loinc.org` when no `url`/`conceptMapId`/`system` is given.
- Changed the MCP server to resolve its LOINC store per request via a getter, so it picks up a store hot-swapped by an upload import instead of serving a stale one; the stdio `mcp` command and existing callers still work unchanged.
- Upgraded `github.com/modelcontextprotocol/go-sdk` to v1.8.0, adding stateless MCP protocol version `2026-07-28` support (server-side, alongside `2025-11-25` and earlier) with typed tool `outputSchema`/`structuredContent`.
- Added a "Local APIs" console (`?mode=apis`) with operation presets, editable params, and copyable curl for the FHIR and Search API routes, distinguishing "Local database" from "Regenstrief upstream" sources and labeled as not an official Regenstrief service.
- Bundled an offline Swagger UI (`/api/docs`, no CDN dependency) served against a relative `/openapi.json`, so API docs work without network access.
- Added `docs/LOCAL_APIS.md` (consumer-facing FHIR + Search API guide), `/docs/{file}.md` cross-links from the running app, and expanded README/agent-KB coverage for the new local APIs and MCP tools.
- Added dev tooling: `scripts/capture-exemplars.sh`, `scripts/fetch-vendor-docs.sh`, `scripts/fhir-parity.sh`, and `make dev-refs`/`make parity` targets for capturing upstream golden responses and diffing them against a running local server; captured docs (`docs/vendor/`, `docs/exemplars/`) are gitignored third-party content, dev-only.
- Fixed a bug where a bare LOINC number (e.g. `718-7`) in local Lucene search was parsed as "718 AND NOT 7" instead of a whole-number phrase match.
- Fixed several `collate nocase` bindings on already-numeric or already-normalized LOINC/part/answer-list/group identifiers that silently defeated their covering indexes and forced full table scans; added indexes for the identifier columns that do need case-insensitive lookups.
- Fixed `$subsumes` to also recognize LP parts that only appear in the hierarchy tables (never as a `$lookup`-style CodeSystem property), instead of reporting them as unknown codes.
- Fixed request bodies exceeding the configured limit to return a proper FHIR `413`/`too-costly` `OperationOutcome` instead of a raw connection error.
- Added `terminology.Open(dbPath, opts) (*terminology.Service, error)` as the public, read-only in-process embedding entry point for `pkg/terminology` (Mode A), for external Go callers that don't run `loinc-browser` itself.
- Added bounded LRU caches for `ValueSet` `$expand` results.
- Fixed `$lookup`/MCP/UDP part-valued Codings (axes, supplementary properties, parent/child) to use the Part's `PartName` as `display`, matching fhir.loinc.org, instead of `PartDisplayName`.
- Added `docs/USE_CASES.md`.

## 0.92 - 2026-06-16

- Added Official API mode for querying Regenstrief's LOINC Search API through a local POST proxy with optional encrypted local credential storage.
- Added local Advanced Search mode backed by a generated search index with scoped LOINC, Parts, Answer Lists, and Groups results.
- Added local advanced-search API endpoints for index status, manual rebuild, and query execution.
- Added support for fielded clauses, boolean grouping, fuzzy terms, proximity syntax, inclusive and exclusive ranges, wildcards, and escaped special characters in local advanced search.
- Preserved original release CSV content in generated `raw_csv_*` SQLite tables during ingest for audit and future field promotion.
- Added focused LOINC agent documentation, official API documentation, local advanced-search documentation, and OpenAPI/Swagger coverage.
- Improved Advanced Search UI with a dedicated mode button, search-term builder, collapsible options row, pagination, resizable result columns, and row-click detail opening.

Full Changelog: https://github.com/drguptavivek/loinc-browser/compare/v0.91...v0.92

## 0.91 - 2026-06-15

- Improved clinical relationship lanes with full LOINC names for panel observations and deduplicated parent containers.
- Added Cytoscape-backed clinical lanes and exploration graph controls with pan, zoom, reset, organize, and more/fewer relationship limits.
- Added copy and TXT export actions for clinical lanes and exploration graph relationship lists, capped at 100 concepts.
- Changed relationship drawer term-opening actions to compact icons that open terms in hierarchy mode in a new tab.
- Improved relationship labels so placeholder `-` concepts fall back to code/type context.

Full Changelog: https://github.com/drguptavivek/loinc-browser/compare/v0.90...v0.91

## 0.90 - 2026-06-14

- Added all-in-one default startup with UI, `/api/v1`, Swagger/OpenAPI, and HTTP MCP at `/mcp`.
- Added first-run auto-import from a local `Loinc*.zip` into `./data/loinc-normalized.sqlite`.
- Added Go-native MCP tools, resources, editable agent docs, and repository skill.
- Added normalized v1 API, Swagger UI, and browser UI for ranked search, hierarchy, panels, answer lists, parts, and groups.
- Added version reporting for CLI, API, and UI.

Full Changelog: https://github.com/drguptavivek/loinc-browser/commits/v0.90
