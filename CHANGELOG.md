# Changelog

## Unreleased

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
