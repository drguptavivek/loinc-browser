# LOINC MCP Server And Agent Skill Design

## Goal

Expose the local LOINC Browser database to agents through Model Context Protocol (MCP) using the Go app itself, with both stdio and HTTP transports, context-optimized tools, and editable Markdown-backed agent guidance.

## Scope

This design covers:

- A shared Go MCP implementation for LOINC tools and resources.
- A stdio MCP command for local agent launch.
- An HTTP MCP endpoint exposed by the existing web/API server.
- Agent-facing Markdown docs served live from disk.
- A repository skill that tells agents how to connect to and use the MCP server.

This design does not cover:

- Online editing of the Markdown docs.
- Remote hosted deployment or public network exposure.
- Copying, committing, or serving licensed LOINC release zip files, generated SQLite databases, WAL/SHM files, or raw release tables.
- Bulk export tools for the full LOINC release.

## Architecture

Create a focused internal MCP package at `internal/mcpserver` that depends on `loinc.Store` and registers MCP tools/resources once. The CLI and HTTP server call into that package so stdio and HTTP transports expose the same behavior.

The existing Go app remains the source of truth:

- `loinc-browser mcp --db ./data/loinc-normalized.sqlite --docs-dir ./docs/agent`
- `loinc-browser serve --db ./data/loinc-normalized.sqlite --addr :8080 --mcp --docs-dir ./docs/agent`

The stdio command opens its own `loinc.Store` and serves MCP on stdin/stdout. The HTTP path reuses the already-open store from `serve` so the browser UI, `/api/v1`, Swagger/OpenAPI, and MCP all read the same database.

## Configuration

Add these flags:

- `mcp --db`: path to the generated SQLite database. Default matches `serve`.
- `mcp --cache-entries`: same cache control as `serve`.
- `mcp --docs-dir`: directory containing agent Markdown docs. Default `./docs/agent`.
- `serve --mcp`: enable HTTP MCP endpoint.
- `serve --mcp-path`: HTTP MCP route. Default `/mcp`.
- `serve --docs-dir`: directory containing agent Markdown docs. Default `./docs/agent`.

Add environment support:

- `LOINC_AGENT_DOCS_DIR`: default docs directory for agent/MCP resources.
- If an MCP path env var is added, document it in `.env.example`.

CLI flags override environment defaults.

## MCP Transports

### Stdio

The stdio server is the default agent integration path. Agent MCP config can launch:

```bash
loinc-browser mcp --db ./data/loinc-normalized.sqlite --docs-dir ./docs/agent
```

It should log operational messages to stderr only, never stdout, so MCP JSON-RPC framing is not corrupted.

### HTTP

When `serve --mcp` is enabled, the existing server exposes the same MCP tools/resources over HTTP at `--mcp-path`, defaulting to `/mcp`. The route is intended for local trusted clients and should not be advertised as a public endpoint.

The HTTP MCP endpoint should not replace `/api/v1`; it is an agent protocol surface over the same Store behavior.

## Context Optimization Requirements

Context optimization is a core requirement, not a polish item.

All MCP tools and resources should default to compact output:

- List tools default to small limits.
- Tools cap `limit` to prevent accidental bulk export.
- Results include stable identifiers for follow-up calls: LOINC numbers, answer list IDs, part numbers, group IDs, hierarchy node IDs.
- Potentially large responses return pagination metadata and a suggested next call.
- Full detail requires an explicit `detail=full` or equivalent.
- Most tools support `detail: "summary" | "standard" | "full"` when the underlying data can be large.
- Term/detail tools support narrow `include` or `fields` options where useful.
- Markdown resources return an index or concise overview by default.
- Markdown-backed tools can return one matching section by heading/topic.
- Responses include short task-oriented notes only where they reduce follow-up context, such as deprecated/inactive warnings or fit caveats.

The MCP layer must not expose tools that dump the full release, raw CSV artifacts, or raw SQLite tables.

## MCP Tools

Tool names should be explicit and stable.

### Concepts And Guidance

`loinc_explain_concepts`

- Inputs: `topic`, `detail`.
- Reads Markdown guidance from `docs/agent/LOINC_CONCEPTS.md`.
- Returns a concise section matching the requested topic, or a compact topic index when no topic is provided.
- Supported topics include `term`, `axes`, `status`, `usage`, `rank`, `panels`, `answer_lists`, `hierarchy`, `parts`, `groups`, `copyright`, and `search_strategy`.

### Term Search And Selection

`loinc_search_terms`

- Inputs mirror the useful subset of `/api/v1/terms/search`: `q`, `status`, `usageType`, `rankMode`, `sort`, `rankedOnly`, `class`, `system`, `timeAspect`, `scale`, `method`, `property`, `orderObs`, `hierarchyNodeId`, `limit`, `offset`, `detail`.
- Default output is compact candidates with LOINC number, display/common name, status, usage types, ranks, class/system/scale/property, and fit warning hints.

`loinc_get_term`

- Inputs: `loincNum`, `detail`, `include`.
- Defaults to standard term fields and omits bulky relationship graphs unless requested.

`loinc_get_term_fit`

- Inputs: `loincNum`.
- Returns compact form-builder suitability flags, status warnings, usage types, rank fields, and links/IDs for follow-up tools.

`loinc_get_term_relationships`

- Inputs: `loincNum`, `include`, `detail`.
- Returns grouped lightweight relationships and IDs for targeted follow-up.

### Panels And Forms

`loinc_search_panels`

- Inputs: search/filter fields similar to `loinc_search_terms`.
- Returns compact panel candidates.

`loinc_get_panel_items`

- Inputs: `loincNum`, `limit`, `offset`, `detail`.
- Defaults to authored sequence, child LOINC number, display label, required flag, entry type, datatype, and answer list override ID.

### Answer Lists

`loinc_search_answer_lists`

- Inputs: `q`, `limit`, `offset`, `detail`.
- Returns answer list IDs, names, OIDs, and external-definition state.

`loinc_get_answer_list_answers`

- Inputs: `answerListId`, `limit`, `offset`, `detail`.
- Defaults to sequence number, display text, local code, external code, and score when present.

### Hierarchy

`loinc_browse_hierarchy`

- Inputs: `nodeId`, `q`, `limit`, `offset`, `detail`.
- With no `nodeId`, returns root nodes. With `nodeId`, returns immediate children and breadcrumb/context hints.
- Always uses hierarchy occurrence `nodeId`, not hierarchy concept code, as the stable browse identifier.

`loinc_get_hierarchy_terms`

- Inputs: `nodeId`, term-list filters, `limit`, `offset`, `detail`.
- Returns terms scoped to a hierarchy occurrence subtree.

### Parts And Groups

`loinc_search_parts`

- Inputs: `q`, `limit`, `offset`, `detail`.
- Returns part number, type, name, display name, and status.

`loinc_search_groups`

- Inputs: `q`, `limit`, `offset`, `detail`.
- Returns group ID, group name, parent group, archetype, and status.

## MCP Resources

Resources are file-backed or generated on demand:

- `loinc://concepts`: reads `docs/agent/LOINC_CONCEPTS.md`.
- `loinc://agent-guide`: reads `docs/agent/LOINC_AGENT_GUIDE.md`.
- `loinc://license-note`: reads `docs/agent/LOINC_LICENSE_NOTE.md`.
- `loinc://api-guide`: reads `docs/API.md`.
- `loinc://openapi`: returns the live OpenAPI JSON structure already served by the app.

Markdown files are read at request time so edits made in code editors are visible on the next MCP call. Missing docs should return a clear MCP error naming the expected file path and the `--docs-dir` setting.

Default resource reads should return the Markdown file content as a direct resource read. Topic-scoped and compact section retrieval belongs in the companion `loinc_explain_concepts` tool, which prevents agents from loading whole docs when a narrow answer is enough.

## Editable Agent Docs

Create:

- `docs/agent/LOINC_CONCEPTS.md`
- `docs/agent/LOINC_AGENT_GUIDE.md`
- `docs/agent/LOINC_LICENSE_NOTE.md`

`LOINC_CONCEPTS.md` should explain key LOINC concepts for agents:

- Term anatomy and six axes.
- Status values and why deprecated, discouraged, and inactive terms need caution.
- Order vs observation usage.
- Common test/order ranks and ranked-only search.
- Answer lists.
- Panels and questionnaire forms.
- Hierarchy occurrence node IDs.
- Parts and groups.
- Copyright/source metadata.
- Search strategy for form-builder workflows.

`LOINC_AGENT_GUIDE.md` should explain common workflows:

- Search for an observation term.
- Validate a selected term using fit metadata.
- Inspect answer lists.
- Inspect panels/forms.
- Browse hierarchy and get subtree terms.
- Compare candidates without overloading context.

`LOINC_LICENSE_NOTE.md` should remind agents:

- LOINC release data is licensed.
- Do not copy release files, generated SQLite databases, or bulk term dumps into source control or external prompts.
- Prefer narrow search/detail calls.

## Repository Skill

Add a skill directory such as `skills/loinc-mcp/SKILL.md`.

The skill should tell agents:

- How to connect over stdio.
- How to connect over HTTP when `serve --mcp` is running.
- Which tools to use for common tasks.
- To start by reading `loinc://concepts` or calling `loinc_explain_concepts` for unfamiliar topics.
- To prefer compact searches and narrow follow-up calls.
- To avoid exposing licensed release data or generated DB files.
- To use `status=INACTIVE` or `status=*` only when explicitly needed.

The skill should include sample agent MCP configuration snippets for stdio and HTTP clients.

## Error Handling

The MCP layer should map existing Store errors to clear MCP errors:

- Missing or unreadable database: explain the expected `--db` path and suggest running ingest.
- Missing term, answer list, part, group, or hierarchy node: return a not-found message with the identifier.
- Missing docs file: name the missing file and current docs directory.
- Invalid arguments: name the invalid field and acceptable values.

Errors should be concise and avoid dumping stack traces into agent context.

## Testing

Tests should cover:

- MCP server construction registers expected tools and resources.
- Representative tool handlers call the existing Store behavior correctly.
- Context defaults are compact and capped.
- `detail=full` is explicit.
- Missing docs file returns a useful error.
- `loinc_explain_concepts` can return a matching Markdown section.
- CLI accepts `mcp` flags.
- `serve --mcp` registers the HTTP MCP route without breaking `/api/v1`, Swagger, or frontend serving.

Final verification remains:

```bash
go test ./...
npm --prefix web run check
npm --prefix web run build
```

Rendered browser verification is only required if implementation changes browser UI behavior.

## Documentation Updates

Update:

- `README.md`: MCP overview, stdio command, HTTP command, and skill location.
- `docs/API.md` or new `docs/MCP.md`: tool/resource reference and examples.
- `.env.example`: new docs directory or MCP settings.
- Release packaging docs/scripts if needed so source docs and skill files are included, but licensed release data and generated databases remain excluded.
