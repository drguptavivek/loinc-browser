# In-App LOINC Mapping Agent — Design Plan

Status: **next epic, starting rc7** (scheduled 2026-09-24; see §12). Design only, not implemented.
Drafted 2026-09-23.

## 1. What it is

A chat panel inside the LOINC browser. A user describes a local test, for example "Hgb,
whole blood, g/dL", "serum potassium", or a row from a lab compendium. The agent:

1. **suggests** LOINC codes: a best match plus alternatives, each with its description, an
   axis-by-axis rationale, and a link to the term page in this browser;
2. **converses**: the user asks why, narrows it down ("it's a POC device", "venous"), or
   compares candidates; the agent answers from the local data;
3. **records the user's decision**: the user picks the code, not the model;
4. optionally **works through a CSV** of local terms row by row and writes the decisions
   into a working copy that the user exports.

Everything runs against the local LOINC data; no LOINC call leaves the machine. The only
external dependency is the LLM endpoint, and that can be local too (§3).

## 2. What already exists and gets reused

| Need | Existing piece |
|---|---|
| Term search (Lucene syntax, fielded) | Bleve local search (`internal/server/local_search.go`), `/searchapi` |
| Term detail, fit, relationships, panels, answer lists, hierarchy, parts, groups | `internal/mcpserver.Service` (the MCP tool implementations) |
| Code validation, properties, deprecated → replacement | `pkg/terminology` (`Lookup`, `ValidateCode`, `Translate` with `loinc-map-to`) |
| Abbreviation expansion (Hgb, Bld, SerPl…) | `data/loinc-abbreviations.csv` |
| Links into the UI | `/?term={LOINC}` (existing deep-link state) |
| Credential storage | encrypted KV used by the official-API proxy |
| External agents (Claude Code, Claude Desktop) | the existing HTTP MCP at `/mcp`. The new `suggest_loinc` and CSV tools are added there too, so outside agents get the same abilities. |

## 3. Architecture

```
Browser chat panel ──SSE──▶ POST /api/v1/agent/chat ─▶ internal/agent (loop)
                                                        │  ├─ LLM client (OpenAI-compatible chat/completions + tools)
                                                        │  └─ tool registry ─▶ suggest engine (§4), mcpserver.Service,
                                                        │                      pkg/terminology, csv tools (§6)
                                                        └─ session + decision store: data/agent.sqlite (separate from the release DB)
```

- **LLM client**: speaks the OpenAI-compatible `POST {base}/v1/chat/completions` with
  `tools` and streaming. One client then works with Ollama / vLLM / LM Studio (fully local),
  LiteLLM, OpenAI, and Anthropic's OpenAI-compatible endpoint. Config:
  `LOINC_AGENT_LLM_BASE_URL`, `LOINC_AGENT_LLM_MODEL`, `LOINC_AGENT_LLM_API_KEY`. The key is
  entered in the UI or `.env` and saved encrypted in the KV; it is never sent to the browser.
  Add the keys to `.env.example`.
- **Settings**: see §3.1.
- **Agent loop**: system prompt, then model turn, then tool calls, then tool results, and
  repeat, capped at 8 tool rounds and a token budget per turn. It streams events to the UI:
  `text-delta`, `tool-start`, `tool-end`, `candidates`, `decision`, `error`. It is
  stateless per request: history is loaded from the session store.
- **Also a non-chat endpoint**: `POST /api/v1/suggest` returns the same candidate cards
  without chat, and without an LLM if none is configured. It is documented in OpenAPI so
  programs can call it directly, and it is the MCP tool `suggest_loinc`.

### 3.1 Agent settings (UI panel + API)

A **Settings** panel in the chat view, backed by `GET/PUT /api/v1/agent/settings`. Secrets are
write-only: the GET returns `"clientSecret": "••••set"`, never the value.

| Setting | Notes |
|---|---|
| Endpoint base URL | e.g. `http://localhost:11434/v1`, `https://api.openai.com/v1`, an internal gateway |
| Model | free text, plus a "list models" button (`GET {base}/models`) when the endpoint supports it |
| Auth mode | `none` · `api-key` · `oauth2-client-credentials` |
| API key + header | default `Authorization: Bearer …`; configurable header name (e.g. `api-key` for Azure-style gateways) |
| Client ID / Client secret / Token URL / Scope | OAuth2 client-credentials flow. The token is fetched server-side, cached until `expires_in` minus 60s, and refreshed on 401 |
| Extra headers | key/value list (e.g. org or project headers some gateways need) |
| Temperature, max tokens, timeout | sensible defaults (0.2, 2048, 60s) |
| Local-only switch | refuses non-loopback/private base URLs (§7) |
| **Test connection** | sends a 1-token completion and one tool-call probe; reports whether tools are supported |

Storage: the non-secret settings live in the file-backed KV (`data/loinc-browser-kv.json`).
The API key and client secret are encrypted with the existing app key
(`OfficialCredentialVault` pattern). Env vars (`LOINC_AGENT_LLM_BASE_URL`, `_MODEL`,
`_API_KEY`, `_CLIENT_ID`, `_CLIENT_SECRET`, `_TOKEN_URL`, `_SCOPE`) seed the defaults; UI-saved
values override them. Keep `.env.example` current.

**System prompt**:

- The **default prompt** ships in the binary (`internal/agent/prompts/default_system.md`,
  embedded). It covers the role, "only codes from tool results", decision rules (the user
  decides; the agent proposes), the axis-by-axis rationale format, link format (`/?term=…`),
  and CSV etiquette. It is visible read-only in the panel.
- The **custom prompt** is an editable textarea with Save and a **Reset to default** button.
  Saved prompts are kept as named versions in the KV (the last 20, with timestamps), and any
  of them can be activated. Which prompt (default vs custom version) is active shows in the
  chat header and is recorded on each decision record.
- Non-negotiable safety rules (only verified codes; decisions need user action) are enforced in
  code (§7), not only in the prompt. Editing the prompt cannot turn them off.
- Optional **prompt variables** substituted at send time: `{{loinc_version}}`, `{{today}}`,
  `{{csv_columns}}` (when a CSV session is active).

## 4. Suggestion engine: deterministic core, LLM on top

The model never invents codes. Codes come only from retrieval, and every shown code is
re-verified with `$lookup` before it reaches the UI.

1. **Normalize the input.** Expand abbreviations (loinc-abbreviations.csv), split units and
   specimen hints, and map units to the likely PROPERTY (g/dL → MCnc, mmol/L → SCnc,
   % → MFr/NFr…). If an LLM is configured, it also returns a structured axis guess
   `{component, property, timing, system, scale, method}` via JSON output. Without one, the
   rule-based parse is used alone.
2. **Retrieve**: several queries against the local index (free text, fielded
   `Component:… System:…`, component synonyms from RELATEDNAMES2), unioned to about 50
   candidates. DEPRECATED terms are replaced by their `MAP_TO` targets and marked as
   such.
3. **Score deterministically**: an axis match grid per candidate (exact, compatible, or
   mismatch per axis, weighted component > system > property > scale > time > method), plus
   COMMON_TEST_RANK as a tiebreaker and methodless preferred unless a method was given.
   Output: a ranked top 5 with a numeric score and the grid.
4. **Explain (LLM)**: the model gets the grid and writes the rationale and the "choose this
   alternative if …" notes. It may reorder only when it gives a reason, and it cannot add codes.
5. **Card** for each candidate: `LOINC_NUM`, long common name, status, class, example
   units, the axis grid (✓ / ~ / ✗ per axis), confidence (high/medium/low derived from the
   score, not model self-report), rationale, `when to pick instead`, and links:
   `/?term=…` (browser), `/fhir/CodeSystem/$lookup?code=…`. Buttons: **Choose**, **Compare**,
   **Open**.

## 5. Conversation and decisions

- The user can follow up in chat ("why not 30350-3?", "is there a POC version?"). The agent
  answers using tools such as `get_term_fit`, relationships, and panels.
- A decision is made only by an explicit user action: the **Choose** button, or a chat
  confirmation the UI shows as a confirm chip. The agent may propose a choice but cannot
  record one on its own.
- The decision record goes to `data/agent.sqlite`. It holds the input, the chosen LOINC,
  the candidates shown with scores, the rationale, the model name, the user note, and a
  timestamp. It exports as JSON/CSV, and past decisions are searchable ("what did we map
  'K+' to last time?"). The agent reads them as a tool (`find_prior_decisions`) for
  consistency.

## 6. CSV mapping workflow

- **Import**: upload a CSV (or pick one from `data/uploads/`), then choose the columns that
  describe a row (local code, name, units, specimen, method…).
- **Batch suggest**: run §4 per row with bounded concurrency; LLM rationale is optional per
  batch (it costs tokens). A progress bar shows status.
- **Review grid**: one row per local term with the top suggestion, confidence, and an
  alternatives dropdown. Accept, reject, skip, or add a note. "Discuss" opens the chat
  scoped to that row. Filter by low confidence first.
- **Agent CSV tools** (for chat-driven edits, e.g. "map rows 12–20 like row 11"):
  `csv_describe`, `csv_read_rows(from,to)`, `csv_find_rows(query)`,
  `csv_propose_mapping(row, loinc, rationale)`. Proposals appear in the grid as pending;
  the user confirms them there.
- **Write-back**: never modify the original. The work lives in a working copy
  (`data/agent/csv/{id}/`) with an append-only change log (undo). Export appends the columns
  `LOINC_NUM, LOINC_LONG_COMMON_NAME, LOINC_STATUS, MATCH_CONFIDENCE, RATIONALE,
  ALTERNATIVES, DECIDED_BY, DECIDED_AT, NOTE`.
- **Scope guard**: CSV tools can only reach files inside the app's upload/agent directories
  (resolved-path check); no arbitrary filesystem access.

## 7. Safety and privacy

- **Data leaving the host**: with a remote LLM, the user's input text and CSV cell values go
  to that provider. Lab compendia are usually not PHI, but patient-level CSVs would be.
  The UI shows which LLM endpoint is active. Allow-listing a local model only is a config
  switch (`LOINC_AGENT_LLM_LOCAL_ONLY=true` refuses non-loopback/non-private base URLs).
- **Prompt injection**: CSV cells and user text are data. Tools are read-only except
  `csv_propose_mapping`, which only creates pending proposals. Nothing the model outputs
  can write a decision.
- **Hallucination guard**: codes not present in the tool results for the session are stripped
  from the model's text before display, and flagged.
- Caps: tool rounds, tokens per turn, batch concurrency, request body size.

## 8. Quality measurement

- A gold set of local term → LOINC pairs, starting from the LOINC mapping examples plus
  the user's own past decisions.
- Metrics: top-1 and top-5 hit rate for the deterministic engine alone and with each LLM;
  the rate of "no good candidate" correctly flagged.
- `go test` runs the deterministic engine on the gold set. An eval command runs the LLM variant
  against the configured endpoint.

## 9. Phases

| Phase | Scope | Done when |
|---|---|---|
| A1 | Suggest engine (§4 steps 1–3, 5) + `POST /api/v1/suggest` + MCP tool `suggest_loinc` + gold-set test | top-5 hit rate measured; OpenAPI documented |
| A2 | LLM client (api-key + OAuth2 client-credentials) + settings panel/API with encrypted secrets and test-connection + default/editable/versioned system prompt + agent loop + SSE `/api/v1/agent/chat` + chat panel UI with candidate cards and links | a conversation works against a local Ollama and a hosted endpoint; settings persist; secrets never returned |
| A3 | Decision store + Choose/confirm flow + prior-decision tool + export | decisions persist across restarts |
| A4 | CSV import / batch / review grid / agent CSV tools / export | round trip: CSV in → reviewed → CSV out, original untouched |
| A5 | Safety switches (local-only, stripping unknown codes), eval command, docs | checklist in §7 verified |

## 10. Decisions (2026-09-23)

- Endpoints to support and test first: **local OpenAI-compatible (Ollama/vLLM)** and **hosted
  OpenAI-compatible with API key**. The OAuth2 client-credentials mode is still built (§3.1),
  but is tested only against a mock token server until a real gateway is named.
- Data policy: the **local-only switch is on by default**. An admin turns it off in settings to
  allow a hosted endpoint, and the chat header always shows the active endpoint.
- Timing: **deferred to the next epic**. This epic ends with the local FHIR + Search API work.

## 11. Open questions (resolved above, kept for context)

1. The LLM endpoint: "OpenAPI-compatible" is read here as **OpenAI-compatible chat
   completions** (the de-facto standard that local and hosted LLM servers speak), configured
   in the §3.1 settings panel. Which provider or model is intended first (a local
   Ollama/vLLM model, or a hosted one)? Does the client ID/secret belong to a specific
   gateway (Azure OpenAI with Entra ID, an internal LiteLLM, etc.)? That decides the token
   URL/scope defaults.
2. May CSV/user text go to a hosted LLM, or must the default be local-only?
3. Is the decision history per user? There are no user accounts today; a single shared local
   history is the default.

## 12. Lessons from the external lab-compendium mapper (2026-09-24)

A separate Python mapper (`ehospital-labs/loinc_mapper/app.py`, not in this repo) mapped a
hospital compendium (15,099 rows, 6,581 distinct names, 13% with units) through `/mcp`. It is
a working prototype of A1 and A4 and changes the plan as follows.

**A1: ranking.** Port its scoring to Go instead of the axis-weight scoring in §4 step 3.
Keep the axis grid only to explain each candidate. What it scores:

- token coverage both ways (local name vs candidate), with abbreviation groups;
- unit agreement: unit → LOINC property (`mg/dL` → MCnc, `%` → MFr, `sec` → Time), and unit
  vs `exampleUcum` after normalizing UCUM syntax (a missing `exampleUcum` is neutral);
- contrast intent for imaging (NCCT/NECT → WO, CECT → W; contrast-unstated terms demoted);
- specimen mismatch penalty, common-rank bonus, deprecated/non-result terms dropped;
- combined orders: map each part, then prefer a panel containing all parts (`contains`);
- meaning search (`mode=semantic`) only as a last resort, capped at LOW.

Certainty levels: EXACT / HIGH / MEDIUM / LOW / UNMAPPED. The server should own every rewrite
(`requestSynonyms`); the mapper's duplicate synonym lists already drifted once (NCCT → the
contrast-unstated 36051-1 instead of 36514-8).

**Gold set.** Before porting, export the mapper's human-confirmed rows and accepted mappings.
The Go engine must reproduce the mapper's certainty counts on it (rc6 baseline: EXACT 84,
HIGH 482, MEDIUM 2,864, LOW 2,328, UNMAPPED 823).

**A3: decisions.** Confirmed rows must survive every re-run (the mapper's MANUAL rows). Add a
reject list ("never pick X for name Y"), which the mapper designed but did not build.

**A4: CSV session.** A session is stateful: upload, map, review and discuss with the agent,
export. Changes to §6:

- map each distinct normalized name once, then fan the result out to every row that has it;
- the unit column is a first-class input, not optional context;
- the review grid opens on LOW and UNMAPPED rows;
- export also carries `METHOD` (word / panel / meaning / manual), `SCORE`, and `SEARCH_QUERY`.

Doing the ranking server-side replaces the mapper's 5 to 15 remote searches per name with one
call, which a live review session needs.
