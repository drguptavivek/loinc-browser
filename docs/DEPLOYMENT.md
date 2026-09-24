# Deployment

How to install, run, and operate `loinc-browser` outside a source checkout. For which interface to
call once it is running, see [`USE_CASES.md`](USE_CASES.md); for transports and latency, see
[`LOCAL_APIS.md`](LOCAL_APIS.md).

`loinc-browser` is a single static Go binary (pure-Go SQLite and Bleve, `CGO_ENABLED=0`) with the
web UI embedded. It needs no runtime, database server, or web server. What it cannot carry is the
licensed LOINC release: every install supplies its own `Loinc_2.xx.zip`.

## Choosing a deployment

| Situation | Choice | Section |
| --- | --- | --- |
| One person, laptop | Run the binary from a terminal | [Run once](#run-once) |
| One person, always on | macOS launchd agent | [macOS](#macos) |
| Shared team/DC service | Linux systemd service | [Linux](#linux-systemd) |
| Windows desktop | Run `loinc-browser.exe` | [Windows](#windows) |
| Go program on the same host | Embed `pkg/terminology`, no server | [`USE_CASES.md` §13](USE_CASES.md#13-embedding-in-a-go-service) |
| Developing the app | `make dev` from source | [README](../README.md#development-mode) |

## Getting the binary

- **Release build (recommended):** download the archive for your platform from GitHub Releases
  (`darwin_arm64`, `darwin_amd64`, `linux_amd64`, `windows_amd64`).
- **From source:** `make build` (needs Go and Node; it builds the web assets first, then
  `./loinc-browser`).

There is no `linux_arm64` build yet; add `"linux/arm64"` to `targets` in
`scripts/build-release.sh` if you need one.

## Data directory

The database, uploads, local search index, app key, and settings live in one directory, resolved
in this order:

1. `LOINC_BROWSER_DATA_DIR`
2. `./data`, when it exists in the working directory (source checkouts)
3. The per-user data directory:
   - macOS: `~/Library/Application Support/loinc-browser`
   - Windows: `%AppData%\loinc-browser`
   - Linux: `$XDG_DATA_HOME/loinc-browser`, or `~/.local/share/loinc-browser`

Startup prints the directory it chose. Configuration is read from `.env` and `loinc.env` in the
working directory, then `.env` in the data directory; earlier values win.

| File in the data directory | What it is | Regenerable? |
| --- | --- | --- |
| `loinc-normalized.sqlite` (+ `-wal`, `-shm`) | Imported release | Yes, re-import the zip |
| `loinc-search.bleve/` | Local Lucene-style search index (~480 MB for 2.82) | Yes, rebuild |
| `uploads/` | Release zips uploaded through the UI | Yes |
| `loinc-browser-app.key`, `loinc-browser-kv.json` | Encrypted saved online-search credentials | No, treat as secrets |

Back up the key and KV files only if you want saved credentials to survive a reinstall.

## First run: loading the release

Pick one:

- Put `Loinc_2.82.zip` in the working directory **or** the data directory and start the server.
  It imports automatically when the database is missing or empty, and never overwrites a
  populated one.
- Start the server with no data and upload the zip in the UI.
- Import explicitly: `loinc-browser ingest --release ./Loinc_2.82` (an unpacked release folder).

A server with no data still starts, so the UI upload works; lookups fail until a release is loaded.

## Search index

`/searchapi`, the UI's Advanced (local) search, and the `loinc_lucene_search` MCP tool need the
Bleve index. A release uploaded through the UI rebuilds it automatically in the background. After
a first-run or `ingest` import, build it once:

```bash
curl -X POST http://localhost:9005/api/v1/local-search/rebuild
```

or with **Build index** in the Advanced search view. On the full 2.82 release a rebuild takes about
25 s (Apple M-series, 195,886 documents). Until it is built, `/searchapi` returns 503 with a
rebuild hint. Every other interface (FHIR, `/api/v1`, MCP lookups, UI term search) reads SQLite
directly and does not need it.

A rebuild writes to a separate folder and swaps it in when finished, so the previous index keeps
answering during the build and an interrupted build never replaces it.
`GET /api/v1/local-search/status` reports one of:

| `state` | Meaning | Queries |
| --- | --- | --- |
| `ready` | Built from the current import | answered |
| `stale` | Built before the current import; results may be outdated | answered |
| `incomplete` | A build left by an older version was interrupted | refused (503) |
| `missing` | Never built | refused (503) |

`building: true` is added while a rebuild runs. The Advanced search view shows a notice for
building, stale, and incomplete indexes, and polls until a build finishes.

The SQLite query indexes are different: import creates them, and startup adds any that are
missing, before the server accepts requests. They never cause partial results.

## Run once

```bash
./loinc-browser                 # :9005, UI + /api/v1 + /fhir + /searchapi + /mcp
./loinc-browser --port 9090
./loinc-browser --addr 127.0.0.1:9005   # this machine only
```

In the background, without a service manager:

```bash
nohup ./loinc-browser > loinc-browser.log 2>&1 &
```

## macOS

```bash
tar -xzf loinc-browser_<version>_darwin_arm64.tar.gz
cd loinc-browser_<version>_darwin_arm64
xattr -d com.apple.quarantine loinc-browser   # release binaries are not signed/notarized
sudo mv loinc-browser /usr/local/bin/
mkdir -p ~/Library/Application\ Support/loinc-browser
cp ~/Downloads/Loinc_2.82.zip ~/Library/Application\ Support/loinc-browser/
loinc-browser
```

To start at login and restart on crash, save `~/Library/LaunchAgents/org.loinc-browser.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>org.loinc-browser</string>
  <key>ProgramArguments</key><array><string>/usr/local/bin/loinc-browser</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>/tmp/loinc-browser.log</string>
  <key>StandardErrorPath</key><string>/tmp/loinc-browser.log</string>
</dict></plist>
```

```bash
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/org.loinc-browser.plist   # start
launchctl bootout   gui/$(id -u) ~/Library/LaunchAgents/org.loinc-browser.plist   # stop
```

## Linux (systemd)

```bash
sudo install -m 755 loinc-browser /usr/local/bin/
sudo useradd --system --no-create-home --shell /usr/sbin/nologin loinc
```

`/etc/systemd/system/loinc-browser.service`:

```ini
[Unit]
Description=LOINC Browser
After=network.target

[Service]
User=loinc
StateDirectory=loinc-browser
WorkingDirectory=/var/lib/loinc-browser
Environment=LOINC_BROWSER_DATA_DIR=/var/lib/loinc-browser
Environment=LOINC_BROWSER_ADDR=:9005
EnvironmentFile=-/etc/loinc-browser.env
ExecStart=/usr/local/bin/loinc-browser
Restart=on-failure
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now loinc-browser        # creates /var/lib/loinc-browser, owned by loinc
sudo cp Loinc_2.82.zip /var/lib/loinc-browser/
sudo chown loinc: /var/lib/loinc-browser/Loinc_2.82.zip
sudo systemctl restart loinc-browser             # imports on this start
curl -X POST http://localhost:9005/api/v1/local-search/rebuild
journalctl -u loinc-browser -f
```

Put secrets (passphrase, online-search credentials) in `/etc/loinc-browser.env`, owned by root with
mode `600`. systemd reads it as root, so the `loinc` user never needs access to it.

## Windows

Unzip and run `loinc-browser.exe` from PowerShell. The data directory is `%AppData%\loinc-browser`
unless a `data` folder exists in the directory you run it from. To run it in the background,
use the Windows Task Scheduler ("At startup", "Run whether user is logged on or not") or a service wrapper such as
NSSM. Nothing in the binary is Windows-service-specific.

## Transports

| Transport | Enable with | Use for |
| --- | --- | --- |
| HTTP over TCP | always on (`--addr`, `--port`, `LOINC_BROWSER_ADDR`, `PORT`) | everything |
| HTTP over a Unix socket | `--unix-socket PATH` / `LOINC_BROWSER_UNIX_SOCKET` | same-host clients; file permissions are the access control |
| UDP micro-protocol | `--udp-addr :8081` / `LOINC_BROWSER_UDP_ADDR` | lossy, same-host/DC fan-out |
| HTTP MCP at `/mcp` | on by default; `--no-mcp` to disable | AI agents over HTTP |
| stdio MCP | `loinc-browser mcp` | an agent config that launches its own process |

## Network exposure and the online search proxy

The default `:9005` listens on **all interfaces**, so anyone who can reach the host can use it.
Everything is read-only and answered locally, with one exception: `/api/v1/official/*` proxies to
the real Regenstrief Search API with a LOINC account. To control that:

| Setting | Effect |
| --- | --- |
| `--addr 127.0.0.1:9005` | Whole server reachable only from this machine |
| `--no-official` / `LOINC_OFFICIAL_DISABLED=true` | Online proxy off; search and credential delete return 403. Use for air-gapped installs |
| `LOINC_OFFICIAL_PASSPHRASE=...` | Online search and credential delete require the `X-Loinc-Passphrase` header (401 otherwise); the UI asks for it |
| `LOINC_OFFICIAL_USERNAME` / `LOINC_OFFICIAL_PASSWORD` | Account from the environment, used for "saved credentials" ahead of the encrypted vault; not deletable from the UI |

Without any of these, credentials entered in the UI can be saved encrypted in the data directory.
The encryption key sits beside the encrypted file, so this protects against the KV file leaking on
its own, not against someone who can read the whole data directory.

The agentic search proxy (`/api/v1/agent/*`, see `docs/API.md`) has its own exposure control:
`LOINC_AGENT_LLM_LOCAL_ONLY` (default `true`) refuses to dial anything but a loopback or private
LLM endpoint, including at connect time, so a configured endpoint can't be redirected or
DNS-rebound to an internal or cloud-metadata address. `LOINC_AGENT_DISABLED=true` turns the whole
feature off.

## CI/CD

- `.github/workflows/ci.yml` runs `go vet`, `go test`, and the web check and build on every push
  to `main` and every pull request.
- `.github/workflows/release.yml` runs on a `v*` tag: it tests, cross-builds, smoke-tests the
  Linux and Windows binaries, and publishes the GitHub release.
- Licensed LOINC data never enters CI, so tests gated on `LOINC_TEST_DB`, `make parity`, and
  `make use-cases` skip there. Run them locally against a loaded server before tagging.
- If a workflow ever needs online-search credentials (for example exemplar capture), pass
  `LOINC_OFFICIAL_USERNAME` / `LOINC_OFFICIAL_PASSWORD` from repository secrets. Never bake them
  into a build.

## Common Lab Codes for India (CLCI), or your own common-codes list

Optional, and one list per deployment. Download the CLCI zip from [NRCeS national releases](https://www.nrces.in/services/national-releases#lab_codes)
and extract it into the data directory, keeping its dated folder:
`<data dir>/common-lab-codes-for-india-20260629/common-lab-codes-for-india.csv` (the newest folder
is used; `LOINC_CLCI_CSV` points elsewhere). Startup prints
`Common codes list "Common Lab Codes for India": 1473 terms (...)`.
Word search then ranks the loaded codes higher, `clci=true` (or `commonCodes=true`) filters to
them, and results carry `localName` (and `clciName` when the loaded list is CLCI). The CLCI file
is C-DAC's (all rights reserved); keep it out of source control. See `docs/agent/LOINC_CLCI.md`.

A non-Indian deployment (US, Australian, hospital-local) can load its own list instead: set
`LOINC_COMMON_CODES_CSV` to any CSV with a LOINC code column and a local-name column (header
row optional), and `LOINC_COMMON_CODES_LABEL` for the label shown in `/api/version` (default the
file name). `LOINC_COMMON_CODES_CSV` takes precedence over `LOINC_CLCI_CSV`.

## Mapper's guide units and comments

Optional. If `<data dir>/common_codes/top2000_mapper_guide.csv` exists (or `LOINC_MAPPER_GUIDE_CSV`
points elsewhere), term results and term detail gain `exampleUcum` (LOINC's example unit) and
`mapperComment` (LOINC's mapping note) for about 2,200 common lab tests. Build the CSV from your
own copy of LOINC's *Mapper's Guide to Top 2000++ US Lab Tests* PDF (from loinc.org/usage):

```bash
python scripts/extract-top2000-mapper-guide.py \
  data/common_codes/LOINC_1.6_Top2000CommonLabResultsUS.pdf data/common_codes/top2000_mapper_guide.csv
```

It needs `pdfplumber`. Both files are LOINC-licensed content; keep them out of git. Restart the
server to load a new CSV.

## Meaning-based search

Optional. Term search can also match by meaning (`mode=hybrid|semantic`, UI **Match**) through
any OpenAI-compatible embeddings endpoint:

```text
LOINC_EMBEDDING_URL=http://127.0.0.1:1234/v1     # LM Studio; Ollama http://127.0.0.1:11434/v1
LOINC_EMBEDDING_MODEL=text-embedding-qwen3-embedding-0.6b
LOINC_EMBEDDING_API_KEY=                          # hosted endpoints only
LOINC_EMBEDDINGS_PATH=<data dir>/loinc-embeddings.sqlite
```

Build the index once after each import with `POST /api/v1/semantic/rebuild` or the UI's **Build
meaning index** link. A 2.82 build embeds 109,325 terms, about 40 minutes with Qwen3-embedding
0.6B in LM Studio on an Apple M-series machine; a stopped build resumes where it left off. The
file is about 112 MB and is loaded into memory on first use. Changing `LOINC_EMBEDDING_MODEL`
requires a rebuild (status says so). The endpoint must be running for searches too, since each
query is embedded when it is asked. With a hosted endpoint, query text leaves the machine.

## Docs in a packaged install

The binary embeds `docs/*.md` and `docs/agent/*.md`. When `./docs/agent` (or `--docs-dir` /
`LOINC_AGENT_DOCS_DIR`) does not exist, startup copies them to `<data dir>/docs/` for the MCP
concept tools and `/docs/*` pages. That copy is refreshed on every start; to customize the docs,
point `--docs-dir` at your own copy.

## Adding it to Claude Code or other MCP clients

See [`MCP.md`](MCP.md#adding-the-server-to-an-mcp-client). In short, with the server running:

```bash
claude mcp add --transport http --scope user loinc http://localhost:9005/mcp
```

## Known gaps

- macOS binaries are unsigned; Gatekeeper blocks them until the quarantine attribute is removed.
