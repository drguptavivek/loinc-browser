# Agent Instructions

This repository is code-only. Do not commit or copy licensed LOINC release data, release zip files, generated SQLite databases, or generated WAL/SHM files into source control.

Local release data may exist beside the code, for example:

```bash
./Loinc_2.82/
./Loinc_2.82.zip
./data/loinc-normalized.sqlite
./data/uploads/
```

Use the app through the single Go command:

```bash
go run ./cmd/loinc-browser
```

This starts the UI, `/api/v1`, Swagger/OpenAPI, and HTTP MCP. It uses `./data/loinc-normalized.sqlite` automatically when `./data` exists (otherwise `LOINC_BROWSER_DATA_DIR` or the per-user data directory; see README) and may auto-ingest a local `Loinc*.zip` when that database is missing or has no `loinc_terms` data. Do not change this to overwrite a populated database.

The serve address may be configured in `.env` with `LOINC_BROWSER_ADDR=:8080` or `PORT=8080`; `--addr` still overrides the default. Do NOT read or edit `.env`; keep `.env.example` current when adding environment keys.

The UI opens on **Find** (`?mode=find`), the end-user screen for looking up a code and copying it. **Map a list** (`?mode=map`) maps a whole lab test master and relies on `POST /api/v1/terms/match`. The explorer modes (hierarchy, facets, rank, relationships, Search API, Advanced Search) sit on the other tabs; keep their deep links working. The design and its status are in `docs/FIND_MODE_PLAN.md`. The Find components are `web/src/lib/components/{FindMode,TermCard,BasketPanel,MapList,SetupPanel}.svelte`. Pure helpers live in `web/src/lib/{copy,maplist,xlsx,setup,domains}.ts`; domain presets (the class and classType filters) are in `domains.ts`.

The deployment loads one common-codes list. It is set with `LOINC_COMMON_CODES_CSV` (any CSV with a LOINC code column and a name column) and optionally `LOINC_COMMON_CODES_LABEL`; otherwise `LOINC_CLCI_CSV`, otherwise Common Lab Codes for India auto-discovered in the data directory. Its codes rank first and results carry `localName`. Keep `clciName`, `clciMatches`, and `clci=true` backward compatible, because the external mapper and MCP clients read them. `lang=<code>` (e.g. `de-DE`, from the `languages` list in `/api/version`) adds `localizedName` from the release's linguistic variants. It is display only: search stays English.

Search tweaks for request wording (shorthands such as `usg`/`esr`/`lft`, dropping a leading "S." for serum, "with contrast" meaning "W contrast") live in `internal/loinc/search.go` (`requestSynonyms`, `Store.Search`). Add a probe to `TestRankingAgainstMappingProbe` in `internal/loinc/ranking_eval_test.go` for every such change and run it against the real database (see below).

Search results hide `STATUS=DEPRECATED` by default. Selecting the `DEPRECATED` status facet should still allow explicitly browsing deprecated terms.

The explicit `ingest --release ./Loinc_2.82` command remains available for manual import into the default normalized SQLite database.

Agent-facing LOINC KB docs live in `docs/agent/`. `LOINC_CONCEPTS.md` is the lightweight index; focused docs include `LOINC_TERM_STRUCTURE.md`, `LOINC_NAMES_AND_DISPLAY.md`, `LOINC_SPECIAL_CASES.md`, `LOINC_DATABASE_STRUCTURE.md`, `LOINC_PART_LINKAGES.md`, `LOINC_MAPPING_GUIDANCE.md`, `LOINC_CLCI.md` (Common Lab Codes for India), and `LOINC_LICENSE_NOTE.md`. Keep source-derived KB sections linked to the original LOINC or related source page.

Technical docs are distinct from conceptual docs. Use `LOINC_DATABASE_STRUCTURE.md` for release-file fields, import/schema guidance, `MapTo`, and `SourceOrganization`. Use `LOINC_PART_LINKAGES.md` for `LoincPartLink_Primary.csv`, `LoincPartLink_Supplementary.csv`, `LinkTypeName`, `PartTypeName`, `Property`, and `PartCodeSystem` guidance. When adding technical KB topics, also update the topic map in `LOINC_CONCEPTS.md`, the workflow hints in `LOINC_AGENT_GUIDE.md`, and MCP topic lookup if a new file is introduced.

The local FHIR (`/fhir`) and LOINC Search API–compatible (`/searchapi`) endpoints are specified in `docs/FHIR_TERMINOLOGY_PLAN.md`. Their reference material is gitignored third-party content: `docs/vendor/` (HL7, loinc.org, MCP docs) and `docs/exemplars/` (captured upstream responses). Regenerate both with `make dev-refs`; exemplar capture needs a LOINC account in `loinc.env`. Parity tests skip when these are absent. Compare a running server against the exemplars with `make parity`. Never commit these folders.

Before claiming completion, run:

```bash
go test ./...
npm --prefix web run check
npm --prefix web run build
```

When search ranking, `/api/v1/terms/match`, or the common-codes list changes, also run the ranking and CLCI checks against the real database:

```bash
LOINC_TEST_DB=$(pwd)/data/loinc-normalized.sqlite go test ./internal/loinc/ -run 'TestRanking|TestMatchNamesAgainstCLCI|Localized' -count=1
```

When `web/src/lib/{copy,maplist,xlsx}.ts` change, run their node self-checks, which are kept out of the app tsconfig:

```bash
node --experimental-strip-types web/src/lib/copy.check.ts
node --experimental-strip-types web/src/lib/maplist.check.ts
node --experimental-strip-types web/src/lib/xlsx.check.ts
```

For rendered browser UI changes, verify behavior with the Codex in-app browser at the running local URL before claiming completion. Do not rely on build output alone for UI interaction fixes.

For Python scripts that create or edit `.docx`/OOXML files, Excel workbooks, PDFs, ODF files, RTF/HTML/Markdown text, YAML/TOML, or PowerPoint files, use:

```bash
/Users/vivekgupta/.codex/.venv/bin/python
```
