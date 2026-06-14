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

This starts the UI, `/api/v1`, Swagger/OpenAPI, and HTTP MCP. It uses `./data/loinc-normalized.sqlite` automatically and may auto-ingest a local `Loinc*.zip` when that database is missing or has no `loinc_terms` data. Do not change this to overwrite a populated database.

The serve address may be configured in `.env` with `LOINC_BROWSER_ADDR=:8080` or `PORT=8080`; `--addr` still overrides the default. Keep `.env.example` current when adding environment keys.

Search results hide `STATUS=DEPRECATED` by default. Selecting the `DEPRECATED` status facet should still allow explicitly browsing deprecated terms.

The explicit `ingest --release ./Loinc_2.82` command remains available for manual import into the default normalized SQLite database.

Before claiming completion, run:

```bash
go test ./...
npm --prefix web run check
npm --prefix web run build
```

For rendered browser UI changes, verify behavior with the Codex in-app browser at the running local URL before claiming completion. Do not rely on build output alone for UI interaction fixes.

For Python scripts that create or edit `.docx`/OOXML files, Excel workbooks, PDFs, ODF files, RTF/HTML/Markdown text, YAML/TOML, or PowerPoint files, use:

```bash
/Users/vivekgupta/.codex/.venv/bin/python
```
