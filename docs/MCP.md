# LOINC MCP Guide

The LOINC Browser binary exposes a local Model Context Protocol (MCP) server for agents. It uses the same normalized SQLite database and Store behavior as `/api/v1`.

## Transports

### All-in-one HTTP

The default run mode starts the UI, `/api/v1`, Swagger UI, `/openapi.json`, and HTTP MCP together:

```bash
loinc-browser
```

The default MCP HTTP endpoint is:

```text
http://localhost:9005/mcp
```

`loinc-browser serve --addr ...` is equivalent. Use `--mcp-path` to change the HTTP MCP route, or `--no-mcp` to disable HTTP MCP.

### Stdio

Use stdio when an agent should launch a dedicated MCP process instead of connecting to the all-in-one HTTP server:

```bash
loinc-browser mcp --docs-dir ./docs/agent --search-index-path ./data/loinc-search.bleve
```

`--search-index-path` (default `<data dir>/loinc-search.bleve`, env `LOINC_SEARCH_INDEX_PATH`) points
`loinc_lucene_search` at the same local Bleve index the `serve` command builds via
`POST /api/v1/local-search/rebuild`.

## Adding the server to an MCP client

### Claude Code

This repository ships a project-scoped [`.mcp.json`](../.mcp.json) that points Claude Code at the
running all-in-one server:

```json
{
  "mcpServers": {
    "loinc": {
      "type": "http",
      "url": "http://localhost:9005/mcp"
    }
  }
}
```

Start the server (`make serve` or `./loinc-browser`), then open Claude Code in the repository.
Claude Code asks you to approve project servers the first time. Check the connection with `/mcp`
in a session or `claude mcp list` in a shell. Tools appear as `mcp__loinc__<tool>`, for example
`mcp__loinc__loinc_lookup_code`.

The same entry can be added from the CLI. `--scope` picks where it is stored:

```bash
# project: writes .mcp.json, shared with everyone who clones the repo
claude mcp add --transport http --scope project loinc http://localhost:9005/mcp

# local (default): only you, only this project
claude mcp add --transport http loinc http://localhost:9005/mcp

# user: only you, every project (for example an installed binary serving on :9005)
claude mcp add --transport http --scope user loinc http://localhost:9005/mcp
```

Use stdio instead when no server is running and Claude Code should launch its own process:

```bash
claude mcp add --scope user loinc -- loinc-browser mcp
# from a source checkout, without a built binary (compiles on every start)
claude mcp add --scope project loinc -- go run ./cmd/loinc-browser mcp
```

Which transport:

| | HTTP (`/mcp`) | stdio (`loinc-browser mcp`) |
| --- | --- | --- |
| Needs a running server | yes | no |
| Shares the server's live database, upload hot-swap, and search index | yes | same files, own process |
| Startup | instant | process start; `go run` compiles first |
| Works for teammates from `.mcp.json` | if they run the server | if `loinc-browser` is on their `PATH` |

`.mcp.json` supports `${VAR}` and `${VAR:-default}` expansion, for example
`"url": "http://localhost:${LOINC_PORT:-9005}/mcp"`.

### Claude Desktop and other clients

Clients that only speak stdio take a command entry. For Claude Desktop, add to
`claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "loinc": {
      "command": "/usr/local/bin/loinc-browser",
      "args": ["mcp"]
    }
  }
}
```

Use an absolute path: desktop apps do not inherit your shell `PATH`. The stdio process uses the same
[data directory](DEPLOYMENT.md#data-directory) as the server, so it finds the database the server
imported. Clients that support streamable HTTP can use `http://localhost:9005/mcp` directly.

## Editable Agent Docs

MCP resources read Markdown files from disk at request time. Edits made in a code editor are visible on the next resource/tool call.

`loinc://concepts` returns the lightweight concept index at `docs/agent/LOINC_CONCEPTS.md`. The `loinc_explain_concepts` tool searches topic sections across the structured agent KB files in `docs/agent/`, including term structure, names/display, special cases, database structure, part linkages, and license notes.

Default docs directory:

```text
./docs/agent
```

The binary also embeds these docs. When the docs directory does not exist (an installed binary
with no `docs/` beside it), startup copies the embedded docs to `<data dir>/docs/` and serves them
from there. That copy is refreshed on every start, so edit docs in a source checkout or in a
directory set with `--docs-dir`, not in the data directory.

Override with:

```bash
--docs-dir ./docs/agent
LOINC_AGENT_DOCS_DIR=./docs/agent
```

## Protocol versions

The server negotiates every MCP protocol version the SDK (`github.com/modelcontextprotocol/go-sdk`
v1.8.0) supports: `2026-07-28` (stateless — no `initialize` handshake; every request carries its
protocol version and client capabilities in `_meta`; the HTTP transport requires `Mcp-Method` and
`Mcp-Name` headers and answers `server/discover`), `2025-11-25`, and `2025-06-18` (classic
`initialize` → `notifications/initialized` → `tools/list`/`tools/call` handshake). Claude Code and
most current clients negotiate `2025-11-25`. The HTTP handler runs `Stateless: true,
JSONResponse: true`, which `2026-07-28` requires; stdio serves both lifecycle models
transparently. Tool output includes typed `outputSchema` (`tools/list`) and `structuredContent`
(`tools/call`) alongside the text-content fallback older clients read.

## Tools

Context is capped by default. Use small limits and follow-up calls by stable ID.

### Local normalized-database tools

| Tool | Purpose |
| --- | --- |
| `loinc_explain_concepts` | Return a compact explanation for one LOINC topic from editable Markdown. |
| `loinc_search_terms` | Search compact LOINC term candidates. Hides deprecated terms unless `status` asks for them. Filter with `classType` (`lab`, `clinical`, `attachment`, `survey`; use `lab` when mapping lab tests) and `class` or `classes`. `mode: "hybrid"` merges meaning-based and word search for natural-language requests (HTTP MCP only, needs the meaning index; see USE_CASES §16). Each result has `relevance` (text-match strength, higher is better; compare within one call, and use `sort: "relevance"` to order by it). When no term matches every word, returns `relaxed: true` with `droppedWords`. `universalLabOrders: true` keeps orderable lab tests; `radModality`, `radRegion`, `radFocus`, `radLaterality`, `radContrast` (`W`, `WO`, `WO & W`), `radSubtype`, `radView` filter radiology terms by RSNA playbook parts. |
| `loinc_get_term` | Get one selected LOINC term. |
| `loinc_get_term_fit` | Get compact form-builder suitability metadata. |
| `loinc_get_term_relationships` | Get grouped lightweight relationships. |
| `loinc_search_panels` | Search panels and forms. `contains: ["5902-2", "6301-6"]` keeps only panels holding every listed term (here the PT panel 34528-0), for mapping combined requests. |
| `loinc_get_panel_items` | List panel/form items in authored sequence. |
| `loinc_search_answer_lists` | Search answer lists. |
| `loinc_get_answer_list_answers` | List answer choices in sequence. |
| `loinc_browse_hierarchy` | Browse hierarchy roots or children by occurrence `nodeId`. |
| `loinc_get_hierarchy_terms` | List terms under a hierarchy node. |
| `loinc_search_parts` | Search LOINC parts. |
| `loinc_search_groups` | Search LOINC groups. |

### FHIR terminology + Search API tools

These wrap the local `/fhir/...` terminology service (`pkg/terminology`, see `docs/LOCAL_APIS.md`)
and the local Bleve search index (`/searchapi`, `/api/v1/local-search/query`). Each returns a
compact summary plus a `browserUrl` (`/?term={code}`) and/or `fhirUrl` an agent can hand to a
user; pass `rawFhir: true` on `loinc_lookup_code`/`loinc_get_questionnaire` for the full FHIR
resource. A bad code or unknown ValueSet/ConceptMap/panel returns a tool error (`isError: true`
with the message), not a transport error.

| Tool | Purpose | Example |
| --- | --- | --- |
| `loinc_lookup_code` | Look up any code kind (term/LP/LL/LA/LG): display, status, axes, key properties, related codes. | `{"code":"718-7"}` |
| `loinc_validate_code` | Check whether a code (optionally with a display) is valid and active. | `{"code":"718-7"}` |
| `loinc_subsumes` | Check subsumption between two codes via the Component Hierarchy by System. | `{"codeA":"LP14559-6","codeB":"718-7"}` |
| `loinc_expand_value_set` | Expand a named/LL/LG/implicit-LP ValueSet into paginated member codes. | `{"url":"http://loinc.org/vs/LL1162-8"}` |
| `loinc_search_value_sets` | Search served ValueSets by name. | `{"nameContains":"positive"}` |
| `loinc_validate_value_set_membership` | Check whether a code is a member of a ValueSet. | `{"url":"http://loinc.org/vs/LL1162-8","code":"LA6576-8"}` |
| `loinc_translate` | Translate a code through a ConceptMap (deprecated-LOINC replacement, or a third-party mapping). | `{"code":"11556-8"}` |
| `loinc_list_concept_maps` | List the ConceptMaps this server serves. | `{}` |
| `loinc_get_questionnaire` | Get a LOINC panel as a compact FHIR Questionnaire item tree. | `{"loincNum":"24357-6"}` |
| `loinc_lucene_search` | Run a Lucene-style query against the local Bleve index (loincs/parts/answerlists/groups). | `{"scope":"loincs","query":"Component:glucose System:bld"}` |

`loinc_translate` defaults `system` to `http://loinc.org` when neither `url`/`conceptMapId` nor
`system` is given, so a bare `{"code":"..."}` searches every served map from LOINC — unlike FHIR's
own `ConceptMap/$translate` route, which still requires an explicit `system` or `url`/`id`.
`loinc_lucene_search` requires the local search index to be built
(`POST /api/v1/local-search/rebuild`, or the `mcp` subcommand's `--search-index-path`); when the
index has not been built at that path, it reports a clear error instead of guessing.

## Resources

| Resource | Source |
| --- | --- |
| `loinc://concepts` | `docs/agent/LOINC_CONCEPTS.md` |
| `loinc://agent-guide` | `docs/agent/LOINC_AGENT_GUIDE.md` |
| `loinc://license-note` | `docs/agent/LOINC_LICENSE_NOTE.md` |
| `loinc://api-guide` | `docs/API.md` |
| `loinc://openapi` | live OpenAPI JSON from the app |

## Context Discipline

- Search first with compact results.
- Keep `limit` small; MCP tools cap large limits.
- Use `loinc_get_term_fit` before recommending a selected code.
- Use focused follow-up tools for answer lists, panels, hierarchy, parts, and groups.
- Request full detail only when needed.
- Do not use MCP to produce bulk dumps of the LOINC release.

## License Discipline

LOINC release data is licensed. Do not commit or copy release zip files, extracted release directories, generated SQLite databases, WAL/SHM files, or bulk release dumps into source control or external prompts.
