# LOINC Browser

Local browser for a licensed LOINC release. The code imports every term row from `LoincTable/Loinc.csv` into typed SQLite columns, imports required relationship/accessory artifacts into normalized foreign-key tables, builds an FTS5 index over the searchable LOINC fields, and serves a Svelte search UI from a Go binary.

Licensed LOINC release files and generated SQLite databases must stay out of git.

## License and Attribution

This repository contains application code and documentation only. It does not include the LOINC release, generated SQLite databases, or redistributed LOINC Licensed Materials.

LOINC content is owned by its respective rights holders and remains governed by the [LOINC Copyright Notice and License](https://loinc.org/kb/license/). When you use this browser with a local LOINC release, the following LOINC notice applies:

> This material contains content from LOINC (http://loinc.org). LOINC is Copyright © Regenstrief Institute, Inc. and the Logical Observation Identifiers Names and Codes (LOINC) Committee and is available at no cost under the license at http://loinc.org/license. LOINC® is a registered United States trademark of Regenstrief Institute, Inc.

Third-party content surfaced from LOINC release fields, including `EXTERNAL_COPYRIGHT_NOTICE`, remains subject to the relevant third-party copyright and terms. Project documentation and non-LOINC explanatory text may be reused with attribution under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/). Project source: [GitHub](https://github.com/drguptavivek/LOINC).

## Quick Start

```bash
make install
make ingest RELEASE=./Loinc_2.82
make serve
```

Open `http://localhost:8080`.

The default database is `./data/loinc-normalized.sqlite` and the default address is `:8080`. Override either value when needed:

```bash
make ingest RELEASE=./Loinc_2.82 DB=./data/loinc-normalized.sqlite
make serve DB=./data/loinc-normalized.sqlite ADDR=:9090
```

The serve address can also come from `.env`:

```bash
LOINC_BROWSER_ADDR=:8080
# or
PORT=8080
```

See `.env.example` for the supported keys.

The browser includes a local loader for uploading a licensed LOINC release zip. Uploaded releases are extracted under `data/uploads/`, ingested into the configured SQLite database, and remain outside git. v1 term lists exclude `STATUS=INACTIVE` by default; pass `status=INACTIVE` to search inactive terms, or `status=*` to include every status.

On `serve`, if the configured database is missing or has no `loinc_terms` data, the app looks for a local `Loinc*.zip` within the project directory or one nested release directory and imports it automatically. Existing populated databases are not overwritten.

## Installed Binary

Release packages contain only the application binary and documentation. They do not contain LOINC release data or a generated database.

```bash
./loinc-browser ingest --release ./Loinc_2.82 --db ./data/loinc-normalized.sqlite
./loinc-browser serve --db ./data/loinc-normalized.sqlite --addr :8080
```

If a local `Loinc*.zip` is present and the database is empty, `serve` can auto-ingest it on first run.

## Development Mode

Use Vite during UI work so Svelte changes hot-reload without rebuilding embedded assets:

```bash
make dev
```

This starts two processes:

- Go API on `http://localhost:8080`, restarted when Go source files change
- Vite HMR UI on `http://localhost:5173`, proxying `/api` and `/openapi.json` to the Go API

You can also run them separately:

```bash
make dev-api
make dev-web
```

Override ports when needed:

```bash
make dev ADDR=:9090 DEV_WEB_PORT=5174
```

The importer requires these release files and fails if any are missing:

- `LoincTable/MapTo.csv` for deprecated-term replacement mappings
- `LoincTable/SourceOrganization.csv` for source/copyright metadata
- `AccessoryFiles/PartFile/Part.csv`
- `AccessoryFiles/PartFile/LoincPartLink_Primary.csv`
- `AccessoryFiles/PartFile/LoincPartLink_Supplementary.csv`
- `AccessoryFiles/AnswerFile/AnswerList.csv`
- `AccessoryFiles/AnswerFile/LoincAnswerListLink.csv`
- `AccessoryFiles/PanelsAndForms/PanelsAndForms.csv`
- `AccessoryFiles/GroupFile/ParentGroup.csv`
- `AccessoryFiles/GroupFile/Group.csv`
- `AccessoryFiles/GroupFile/GroupLoincTerms.csv`
- `AccessoryFiles/ComponentHierarchyBySystem/ComponentHierarchyBySystem.csv`

The ingest schema is normalized-only for relationship data. v1 API endpoints read these tables directly; there are no compatibility relationship tables or raw JSON fallback columns.

## High-Level Relationship Model

The browser treats `LoincTable/Loinc.csv` as the canonical term table. Everything else enriches those terms with normalized relationships or source metadata:

- `loinc_map_to`: direct term-to-term replacement links, mainly for deprecated terms.
- `parts` and `loinc_part_links`: term-to-concept links for component, property, system, method, and other semantic parts.
- `answer_lists`, `answer_list_answers`, and `loinc_answer_list_links`: answer-list identity, answer rows, and term usage.
- `panel_items`: parent-child term relationships for panels, forms, and their member observations.
- `parent_groups`, `loinc_groups`, and `group_loinc_terms`: value-set style groupings that collect related LOINC terms.
- `hierarchy_concepts`, `hierarchy_occurrences`, `hierarchy_edges`, `hierarchy_closure`, and `hierarchy_subtree_terms`: path-preserving hierarchy browsing and fast branch-scoped term queries.
- Source organizations: copyright, source, and terms-of-use metadata for imported source references.

The v1 API exposes focused resources for term search, term detail, grouped relationships, hierarchy nodes, panel items, answer lists, parts, groups, and source/copyright metadata. Hierarchy browsing uses `hierarchy_occurrences.node_id` so duplicate hierarchy concept codes are safe to browse.

The app also supports **Browse by rank**, based on LOINC's `COMMON_TEST_RANK` and `COMMON_ORDER_RANK` fields. Ranked browsing can use observation or order rank mode, limits results to positive ranks when requested, and orders the most frequently used LOINC codes first. Unranked terms remain searchable through normal search and facet browsing.

See `ERD.md` for the fuller relationship diagram and storage model.

## API

The same server exposes JSON endpoints for scripts and other apps. The `/api/v1` routes are the normalized API surface for EMR form-builder workflows.
See `docs/API.md` for the structured v1 API guide, including route groups, shared filters, pagination, HATEOAS links, hierarchy browsing, panels, answer lists, parts, groups, and copyright/source workflows.

```bash
curl 'http://localhost:8080/api/v1/terms/search?q=glucose&usageType=observation&rankMode=observation&sort=relevance'
curl 'http://localhost:8080/api/v1/terms/top?rankMode=observation&limit=10'
curl 'http://localhost:8080/api/v1/terms/14749-6'
curl 'http://localhost:8080/api/v1/terms/14749-6/relationships'
curl 'http://localhost:8080/api/v1/hierarchy/roots'
curl 'http://localhost:8080/api/v1/answer-lists/search?q=positive'
curl 'http://localhost:8080/api/v1/source-organizations'
curl -F 'releaseZip=@./Loinc_2.82.zip' 'http://localhost:8080/api/import/upload'
```

Swagger UI is served at `http://localhost:8080/api/docs`. The underlying OpenAPI 3.1 spec is served at `http://localhost:8080/openapi.json`.

## Check

```bash
go test ./...
npm --prefix web run check
npm --prefix web run build
```

## Build Installable Binaries

Build code-only release packages for macOS, Linux, and Windows:

```bash
make release VERSION=2.82.0
```

Packages are written under `dist/`:

- `loinc-browser_<version>_darwin_amd64.tar.gz`
- `loinc-browser_<version>_darwin_arm64.tar.gz`
- `loinc-browser_<version>_linux_amd64.tar.gz`
- `loinc-browser_<version>_linux_arm64.tar.gz`
- `loinc-browser_<version>_linux_armv7.tar.gz`
- `loinc-browser_<version>_windows_amd64.zip`
- `loinc-browser_<version>_windows_arm64.zip`

These packages include only the app binary and docs. They do not include licensed LOINC release files or generated SQLite databases.
