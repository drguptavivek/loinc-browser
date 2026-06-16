# Official LOINC API For Agents

## Official API Search

The local app exposes a server-side proxy for the official Regenstrief LOINC Search API at `POST /api/v1/official/search`. Use it when the user wants to compare local imported search with current official LOINC search behavior, or when they explicitly ask for the official API.

Supported official scopes are `loincs`, `answerlists`, `parts`, and `groups`. Supported official parameters are `query`, `rows`, `offset`, `sortorder`, `language`, and `includefiltercounts`.

The response envelope keeps the upstream response under `payload` and adds `local` metadata when LOINC codes can be matched against the local SQLite database. `local.matches[loincNum].found=true` means the official result exists in the imported offline release and can be opened through `/api/v1/terms/{loincNum}`. If the local database is not loaded, the official payload still returns with `local.available=false`.

Local Lucene-style search for the same scopes is documented in `docs/LOCAL_LUCENE_SEARCH.md`. It uses a generated Bleve index built from the local SQLite database and keeps SQLite as the canonical source for hydrated results.

Source: [LOINC Search API](https://loinc.org/kb/api/search-api/).

## Official API Credentials

Credentials are sent to the local app in a JSON `POST` body, never in URL query strings. The app can use direct credentials for one request or saved encrypted credentials with `useSavedCredentials=true`.

Saved credentials are encrypted in the local file-backed KV store `./data/loinc-browser-kv.json` using a random app key at `./data/loinc-browser-app.key`. This prevents casual KV inspection, but anyone with both files can decrypt the saved credentials. Keep both files out of source control.

The app loads `.env` and then `loinc.env` when present. Use `loinc.env` for local test credentials and do not commit it.

## Official API Query Syntax

The `query` value follows official LOINC search syntax. Basic queries use implicit `AND`, quoted phrases, boolean operators, wildcards, and fielded search. Examples include:

```text
glucose blood
"glucose blood"
Component:glucose System:blood
Component:gluco*
```

Advanced syntax supports required and prohibited clauses, grouping, fuzzy and proximity search, ranges, and escaping special characters. Examples include:

```text
+Component:glucose +System:blood
Component:glucose -System:urine
(Component:glucose OR Component:fructose) System:blood
"function panel"~1
```

Part searches and answer-list searches have their own field names. Use official part and answer-list syntax when the scope is `parts` or `answerlists`.

The browser Official API mode includes a query builder for documented official search options: any-field search, fielded search, required `+` clauses, excluded `-` clauses, exact phrases, wildcards, fuzzy search, proximity search, inclusive ranges, and exclusive ranges. It includes the official advanced LOINC fields, part-search fields, and answer-list fields, while keeping the raw query box editable for syntax not captured by a form control.

Sources: [Basic Search Syntax](https://loinc.org/kb/search/basic/), [Advanced Search Syntax](https://loinc.org/kb/search/advanced-search-syntax/), [Part Search](https://loinc.org/kb/search/part-search/), and [Answer List Search](https://loinc.org/kb/search/answer-list-search/).

## Official API Use Guidance

Prefer the local SQLite-backed `/api/v1` routes for fast local browsing, form-builder workflows, relationship inspection, hierarchy navigation, and repeatable tests. Use the official API proxy for official-search comparison, current official search syntax behavior, and scopes that the upstream search service handles directly.

The upstream response schema is treated as unstable by this app. Consumers should read the local envelope fields `scope`, `params`, `upstreamStatus`, `payload`, and `local`, then handle `payload` defensively.
