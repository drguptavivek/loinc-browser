# LOINC Browser Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a local LOINC search and browse app that imports a licensed LOINC release into SQLite FTS5 and serves a Svelte browser UI from a Go binary.

**Architecture:** The Go binary has `ingest` and `serve` commands. `ingest` reads local release CSV files and creates a generated SQLite database; `serve` exposes JSON APIs, uses an in-memory object cache for repeated term/facet reads, and serves embedded Svelte assets. The repository contains only code and ignores licensed release files and generated databases.

**Tech Stack:** Go `net/http`, SQLite FTS5 via `modernc.org/sqlite`, Svelte + Vite + TypeScript, shadcn-svelte-inspired local components, Tailwind CSS.

---

### Task 1: Backend Core And FTS Import

**Files:**
- Create: `go.mod`
- Create: `cmd/loinc-browser/main.go`
- Create: `internal/loinc/schema.go`
- Create: `internal/loinc/ingest.go`
- Create: `internal/loinc/search.go`
- Create: `internal/loinc/cache.go`
- Test: `internal/loinc/loinc_test.go`

- [ ] **Step 1: Write failing tests**

Create tests that build a miniature release directory in `t.TempDir()`, ingest it, and verify exact LOINC lookup, FTS search, facets, and cache hits.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/loinc`
Expected: FAIL because the package implementation does not exist yet.

- [ ] **Step 3: Implement minimal backend package**

Create SQLite schema with `loinc_terms`, `loinc_terms_fts`, `import_meta`, indexes, ingestion from `LoincTable/Loinc.csv`, search, detail lookup, facets, and a bounded object cache.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/loinc`
Expected: PASS.

### Task 2: HTTP API And CLI

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/server_test.go`
- Modify: `cmd/loinc-browser/main.go`

- [ ] **Step 1: Write failing API tests**

Test `/api/search`, `/api/terms/{loincNum}`, `/api/facets`, and `/api/health` against an in-memory test server.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/server`
Expected: FAIL because the server package is not implemented yet.

- [ ] **Step 3: Implement API and CLI wiring**

Add `ingest` and `serve` subcommands with flags for `--release`, `--db`, and `--addr`. Add JSON handlers using the backend package.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/server ./internal/loinc`
Expected: PASS.

### Task 3: Svelte Browser UI

**Files:**
- Create: `web/`
- Create: `web/src/App.svelte`
- Create: `web/src/lib/api.ts`
- Create: `web/src/lib/components/*.svelte`
- Modify: `internal/server/server.go`
- Modify: `cmd/loinc-browser/main.go`

- [ ] **Step 1: Scaffold Svelte UI**

Use Vite Svelte TypeScript and local shadcn-style components for button, input, badge, table, tabs, empty state, and skeleton.

- [ ] **Step 2: Implement UI**

Add search input, filters, browse facets, result table, term detail drawer/panel, loading states, empty states, and API error display.

- [ ] **Step 3: Build and embed assets**

Run: `npm --prefix web run build`
Expected: Vite build succeeds and emits `web/dist`.

### Task 4: End-To-End Verification

**Files:**
- Modify: `.gitignore`
- Modify: `README.md`

- [ ] **Step 1: Protect licensed data**

Ignore `Loinc_*`, `*.zip`, `*.sqlite`, `*.db`, and generated data directories.

- [ ] **Step 2: Verify ingest against local release**

Run: `go run ./cmd/loinc-browser ingest --release ./Loinc_2.82 --db ./data/loinc.sqlite`
Expected: imports about 109k terms.

- [ ] **Step 3: Verify API and UI**

Run: `go run ./cmd/loinc-browser serve --db ./data/loinc.sqlite --addr :8080`
Open `http://localhost:8080`, search for terms, open details, and browse facets.

- [ ] **Step 4: Run final checks**

Run: `go test ./...`
Run: `npm --prefix web run build`
Expected: both pass.
