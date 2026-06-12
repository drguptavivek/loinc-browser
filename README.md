# LOINC Browser

Local browser for a licensed LOINC release. The code imports every term row from `LoincTable/Loinc.csv` into SQLite, preserves all columns from each row for the term detail view, imports useful relationship/accessory artifacts when present, builds an FTS5 index over the searchable LOINC fields, and serves a Svelte search UI from a Go binary.

Licensed LOINC release files and generated SQLite databases must stay out of git.

## Quick Start

```bash
make install
make ingest RELEASE=./Loinc_2.82
make serve
```

Open `http://localhost:8080`.

The default database is `./data/loinc.sqlite` and the default address is `:8080`. Override either value when needed:

```bash
make ingest RELEASE=./Loinc_2.82 DB=./data/loinc.sqlite
make serve DB=./data/loinc.sqlite ADDR=:9090
```

The serve address can also come from `.env`:

```bash
LOINC_BROWSER_ADDR=:8080
# or
PORT=8080
```

See `.env.example` for the supported keys.

The browser includes a local loader for uploading a licensed LOINC release zip. Uploaded releases are extracted under `data/uploads/`, ingested into the configured SQLite database, and remain outside git. Search results hide `STATUS=DEPRECATED` by default unless the `DEPRECATED` status facet is explicitly selected.

On `serve`, if the configured database is missing or has no `loinc_terms` data, the app looks for a local `Loinc*.zip` within the project directory or one nested release directory and imports it automatically. Existing populated databases are not overwritten.

## Installed Binary

Release packages contain only the application binary and documentation. They do not contain LOINC release data or a generated database.

```bash
./loinc-browser ingest --release ./Loinc_2.82 --db ./data/loinc.sqlite
./loinc-browser serve --db ./data/loinc.sqlite --addr :8080
```

If a local `Loinc*.zip` is present and the database is empty, `serve` can auto-ingest it on first run.

When available in the release, the importer also loads:

- `LoincTable/MapTo.csv` for deprecated-term replacement mappings
- `LoincTable/SourceOrganization.csv` for source/copyright metadata
- `AccessoryFiles/PartFile/LoincPartLink_Primary.csv`
- `AccessoryFiles/PartFile/LoincPartLink_Supplementary.csv`
- `AccessoryFiles/AnswerFile/LoincAnswerListLink.csv`
- `AccessoryFiles/PanelsAndForms/PanelsAndForms.csv`
- `AccessoryFiles/GroupFile/GroupLoincTerms.csv`
- `AccessoryFiles/ComponentHierarchyBySystem/ComponentHierarchyBySystem.csv`

Term detail loads the core LOINC fields first. Relationship details are lazy-loaded on demand with `include=relationships` or through the relationship graph endpoint. Loaded relationship details include `mapTo`, `parts`, `answerLists`, `panels`, `groups`, and `hierarchy`; the relationship graph endpoint also exposes incoming `MapTo` links and shared concept neighborhoods so a term can be explored in either direction. Lazy-loaded relationship graphs are cached in memory, so reopening the same term's graph avoids repeating the heavier SQLite relationship traversal.

## High-Level Relationship Model

The browser treats `LoincTable/Loinc.csv` as the canonical term table. Everything else enriches those terms with relationships or source metadata:

- `MapTo.csv`: direct term-to-term replacement links, mainly for deprecated terms. The app exposes both directions: this term maps to another term, and other terms map to this term.
- LOINC parts: term-to-concept links for component, property, system, method, and other semantic parts. Shared parts are used as neighborhoods so a selected term can reveal other terms using the same concept.
- Answer list links: term-to-answer-list relationships for terms with coded response options.
- Panels and forms: parent-child term relationships for panels, forms, and their member observations.
- Groups: value-set style groupings that collect related LOINC terms under a group concept.
- Component hierarchy by system: hierarchy rows that support tree-style exploration and broader/narrower navigation.
- Source organizations: copyright, source, and terms-of-use metadata for imported source references.

In the UI, the term detail drawer shows the direct relationships for the selected term plus shared concept neighborhoods. The relationship browser can also browse relationship rows independently by kind, code, title, or LOINC number.

See `ERD.md` for the fuller relationship diagram and storage model.

## API

The same server exposes JSON endpoints for scripts and other apps.

```bash
curl 'http://localhost:8080/api/search?q=glucose%20plasma&status=ACTIVE&limit=5'
curl 'http://localhost:8080/api/terms/14749-6'
curl 'http://localhost:8080/api/terms/14749-6?include=relationships'
curl 'http://localhost:8080/api/terms/14749-6/relationships'
curl 'http://localhost:8080/api/accessories?kind=part-primary&q=glucose&limit=5'
curl 'http://localhost:8080/api/facets'
curl 'http://localhost:8080/api/source-organizations'
curl -F 'releaseZip=@./Loinc_2.82.zip' 'http://localhost:8080/api/import/upload'
curl 'http://localhost:8080/openapi.json'
```

The OpenAPI 3.1 spec is served at `http://localhost:8080/openapi.json`.

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
