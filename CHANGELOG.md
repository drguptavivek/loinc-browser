# Changelog

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
