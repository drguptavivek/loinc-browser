# Agent Instructions

This repository is code-only. Do not commit or copy licensed LOINC release data, release zip files, generated SQLite databases, or generated WAL/SHM files into source control.

Local release data may exist beside the code, for example:

```bash
./Loinc_2.82/
./Loinc_2.82.zip
./data/loinc.sqlite
./data/uploads/
```

Use the app through the Go commands:

```bash
go run ./cmd/loinc-browser ingest --release ./Loinc_2.82 --db ./data/loinc.sqlite
go run ./cmd/loinc-browser serve --db ./data/loinc.sqlite --addr :8080
```

The serve address may be configured in `.env` with `LOINC_BROWSER_ADDR=:8080` or `PORT=8080`; CLI flags still override the default. Keep `.env.example` current when adding environment keys.

Search results hide `STATUS=DEPRECATED` by default. Selecting the `DEPRECATED` status facet should still allow explicitly browsing deprecated terms.

On `serve`, startup may auto-ingest a local `Loinc*.zip` only when the configured database is missing or has no `loinc_terms` data. Do not change this to overwrite a populated database.

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
