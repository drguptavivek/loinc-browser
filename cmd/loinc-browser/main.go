package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"loinc-browser/docs"
	"loinc-browser/internal/loinc"
	loincmcp "loinc-browser/internal/mcpserver"
	"loinc-browser/internal/server"
	"loinc-browser/internal/udp"
	"loinc-browser/internal/version"
	"loinc-browser/pkg/terminology"
	"loinc-browser/web"
)

// dataDir is where the database, uploads, search index, app key, and settings live:
// LOINC_BROWSER_DATA_DIR, else ./data when it exists (source checkouts), else the per-user
// data directory, so an installed binary works from any working directory.
func dataDir() string {
	if dir := strings.TrimSpace(os.Getenv("LOINC_BROWSER_DATA_DIR")); dir != "" {
		return dir
	}
	if info, err := os.Stat("./data"); err == nil && info.IsDir() {
		return "./data"
	}
	return userDataDir()
}

// userDataDir returns ~/Library/Application Support/loinc-browser on macOS, %AppData%\loinc-browser
// on Windows, and $XDG_DATA_HOME (or ~/.local/share)/loinc-browser elsewhere.
func userDataDir() string {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		if dir := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); dir != "" {
			return filepath.Join(dir, "loinc-browser")
		}
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".local", "share", "loinc-browser")
		}
	} else if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "loinc-browser")
	}
	return "./data"
}

func defaultDBPath() string {
	return filepath.Join(dataDir(), "loinc-normalized.sqlite")
}

// loadEnvFiles loads .env and loinc.env from the working directory, then .env from the data
// directory (which the first .env may itself have set). Earlier values win.
func loadEnvFiles() error {
	for _, path := range []string{".env", "loinc.env"} {
		if err := loadDotEnv(path); err != nil {
			return err
		}
	}
	return loadDotEnv(filepath.Join(dataDir(), ".env"))
}

func main() {
	log.SetFlags(0)
	if err := run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	mode, modeArgs := commandMode(args)
	switch mode {
	case "ingest":
		return runIngest(modeArgs)
	case "serve":
		return runServe(modeArgs)
	case "mcp":
		return runMCP(modeArgs)
	case "-v", "--version", "version":
		return runVersion()
	case "-h", "--help", "help":
		return usage()
	default:
		return fmt.Errorf("unknown command %q\n\n%s", mode, usageText())
	}
}

func commandMode(args []string) (string, []string) {
	if len(args) < 2 {
		return "serve", nil
	}
	if isPortArg(args[1]) {
		return "serve", args[1:]
	}
	if strings.HasPrefix(args[1], "-") && args[1] != "-h" && args[1] != "--help" && args[1] != "-v" && args[1] != "--version" {
		return "serve", args[1:]
	}
	return args[1], args[2:]
}

type serveConfig struct {
	DBPath             string
	Addr               string
	UnixSocketPath     string
	UDPAddr            string
	CacheEntries       int
	EnableMCP          bool
	MCPPath            string
	DocsDir            string
	OfficialAPIBaseURL string
	AppKeyPath         string
	KVPath             string
	SearchIndexPath    string
	OfficialDisabled   bool
	OfficialPassphrase string
}

type mcpConfig struct {
	CacheEntries    int
	DocsDir         string
	SearchIndexPath string
}

func runIngest(args []string) error {
	flags := flag.NewFlagSet("ingest", flag.ContinueOnError)
	releaseDir := flags.String("release", "", "path to local LOINC release directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := loadEnvFiles(); err != nil {
		return err
	}
	dbPath := defaultDBPath()
	if err := ensureDatabaseDir(dbPath); err != nil {
		return err
	}
	summary, err := loinc.Ingest(context.Background(), loinc.IngestOptions{
		ReleaseDir: *releaseDir,
		DBPath:     dbPath,
	})
	if err != nil {
		return err
	}
	fmt.Printf("Imported %d LOINC terms into %s\n", summary.TermCount, summary.DBPath)
	return nil
}

func runVersion() error {
	info := version.Get()
	if info.Date != "" {
		fmt.Printf("loinc-browser %s (%s, %s, %s/%s)\n", info.Version, info.Commit, info.Date, info.GoOS, info.GoArch)
		return nil
	}
	fmt.Printf("loinc-browser %s (%s, %s/%s)\n", info.Version, info.Commit, info.GoOS, info.GoArch)
	return nil
}

func runServe(args []string) error {
	if err := loadEnvFiles(); err != nil {
		return err
	}
	cfg, err := parseServeConfig(args)
	if err != nil {
		return err
	}
	fmt.Printf("Data directory: %s\n", dataDir())
	cfg.DocsDir = resolveDocsDir(cfg.DocsDir)
	if err := ensureDatabaseFromLocalZip(context.Background(), ".", cfg.DBPath); err != nil {
		return err
	}
	store, err := loinc.OpenStore(cfg.DBPath, loinc.StoreOptions{CacheEntries: cfg.CacheEntries})
	if err != nil {
		return err
	}
	defer store.Close()

	assets, err := web.Assets()
	if err != nil {
		return err
	}
	var termSvc *terminology.Service
	handler := server.New(server.Options{
		Store:              store,
		Assets:             assets,
		DBPath:             cfg.DBPath,
		UploadDir:          filepath.Join(dataDir(), "uploads"),
		CacheEntries:       cfg.CacheEntries,
		EnableMCP:          cfg.EnableMCP,
		MCPPath:            cfg.MCPPath,
		DocsDir:            cfg.DocsDir,
		OfficialAPIBaseURL: cfg.OfficialAPIBaseURL,
		AppKeyPath:         cfg.AppKeyPath,
		KVPath:             cfg.KVPath,
		SearchIndexPath:    cfg.SearchIndexPath,
		OfficialDisabled:   cfg.OfficialDisabled,
		OfficialPassphrase: cfg.OfficialPassphrase,
		OfficialEnvCredentials: server.OfficialCredentials{
			Username: strings.TrimSpace(os.Getenv("LOINC_OFFICIAL_USERNAME")),
			Password: os.Getenv("LOINC_OFFICIAL_PASSWORD"),
		},
		Terminology: &termSvc,
	})
	listener, err := listenWithPortPrompt(cfg.Addr, os.Stdin, os.Stdout)
	if err != nil {
		return err
	}
	url := serveURL(listener.Addr())
	fmt.Printf("Serving LOINC browser on %s\n", url)
	if cfg.EnableMCP {
		fmt.Printf("Serving LOINC MCP over HTTP at %s%s\n", url, cfg.MCPPath)
	}

	var unixListener net.Listener
	if cfg.UnixSocketPath != "" {
		unixListener, err = listenUnixSocket(cfg.UnixSocketPath)
		if err != nil {
			return err
		}
		defer os.Remove(cfg.UnixSocketPath)
		fmt.Printf("Serving LOINC browser over Unix socket at %s\n", cfg.UnixSocketPath)
	}

	var udpConn net.PacketConn
	if cfg.UDPAddr != "" {
		udpConn, err = net.ListenPacket("udp", cfg.UDPAddr)
		if err != nil {
			return err
		}
		fmt.Printf("Serving LOINC browser over UDP at %s\n", udpConn.LocalAddr())
	}

	go promptLaunchURL(url, os.Stdin, os.Stdout)
	return serveUntilShutdown(handler, listener, unixListener, udpConn, termSvc)
}

// newHTTPServer bounds how long a client may take to send headers and how long an idle keep-alive
// connection stays open, so pooled LAN clients can't pile up sockets forever. There is no write
// timeout: an upload import or search-index rebuild runs inside one request.
func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 120 * time.Second}
}

// serveUntilShutdown runs the same handler on the TCP listener and, when non-nil, the Unix
// socket listener concurrently, plus the Mode E UDP listener when udpConn is non-nil. It returns
// when any server fails, or shuts all of them down gracefully on SIGINT/SIGTERM.
func serveUntilShutdown(handler http.Handler, tcpListener, unixListener net.Listener, udpConn net.PacketConn, termSvc *terminology.Service) error {
	tcpServer := newHTTPServer(handler)
	servers := []*http.Server{tcpServer}
	errCh := make(chan error, 3)
	go func() { errCh <- tcpServer.Serve(tcpListener) }()

	if unixListener != nil {
		unixServer := newHTTPServer(handler)
		servers = append(servers, unixServer)
		go func() { errCh <- unixServer.Serve(unixListener) }()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if udpConn != nil {
		go func() { errCh <- udp.Serve(ctx, udpConn, termSvc) }()
	}

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, srv := range servers {
			srv.Shutdown(shutdownCtx)
		}
		// udp.Serve watches this same ctx and closes udpConn itself on cancellation.
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func parseServeConfig(args []string) (serveConfig, error) {
	args = normalizeServeArgs(args)
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	addr := trackingStringFlag{value: defaultServeAddr()}
	port := trackingStringFlag{}
	flags.Var(&addr, "addr", "HTTP listen address")
	flags.Var(&port, "port", "HTTP listen port")
	flags.Var(&port, "p", "HTTP listen port")
	cacheEntries := flags.Int("cache-entries", 2048, "maximum in-memory term cache entries")
	enableMCP := flags.Bool("mcp", true, "enable local MCP over HTTP")
	disableMCP := flags.Bool("no-mcp", false, "disable local MCP over HTTP")
	mcpPath := flags.String("mcp-path", "/mcp", "HTTP MCP route path")
	docsDir := flags.String("docs-dir", defaultAgentDocsDir(), "path to editable agent Markdown docs")
	officialAPIBaseURL := flags.String("official-api-base-url", defaultOfficialAPIBaseURL(), "official LOINC Search API base URL")
	appKeyPath := flags.String("app-key-path", defaultAppKeyPath(), "path to local app key for encrypted app settings")
	kvPath := flags.String("kv-path", defaultKVPath(), "path to local file-backed app settings KV")
	searchIndexPath := flags.String("search-index-path", defaultSearchIndexPath(), "path to generated local Lucene-style search index")
	unixSocketPath := flags.String("unix-socket", defaultUnixSocketPath(), "path to a Unix domain socket to also serve on (Mode D), off by default")
	udpAddr := flags.String("udp-addr", defaultUDPAddr(), "UDP address for the compact Mode E micro-protocol, off by default")
	disableOfficial := flags.Bool("no-official", envBool("LOINC_OFFICIAL_DISABLED"), "disable the /api/v1/official/* proxy to the online LOINC Search API")
	if err := flags.Parse(args); err != nil {
		return serveConfig{}, err
	}
	positionals := flags.Args()
	if len(positionals) > 1 {
		return serveConfig{}, fmt.Errorf("expected at most one port argument, got %q", strings.Join(positionals, " "))
	}
	if len(positionals) == 1 {
		if port.set || addr.set {
			return serveConfig{}, fmt.Errorf("positional port cannot be combined with --port or --addr")
		}
		port.value = positionals[0]
		port.set = true
	}
	listenAddr := addr.value
	if port.set {
		if addr.set {
			return serveConfig{}, fmt.Errorf("--port cannot be combined with --addr")
		}
		normalizedPort, err := normalizePortFlag(port.value)
		if err != nil {
			return serveConfig{}, err
		}
		listenAddr = normalizedPort
	}
	return serveConfig{
		DBPath:             defaultDBPath(),
		Addr:               listenAddr,
		UnixSocketPath:     strings.TrimSpace(*unixSocketPath),
		UDPAddr:            strings.TrimSpace(*udpAddr),
		CacheEntries:       *cacheEntries,
		EnableMCP:          *enableMCP && !*disableMCP,
		MCPPath:            normalizePathFlag(*mcpPath),
		DocsDir:            *docsDir,
		OfficialAPIBaseURL: *officialAPIBaseURL,
		AppKeyPath:         *appKeyPath,
		KVPath:             *kvPath,
		SearchIndexPath:    *searchIndexPath,
		OfficialDisabled:   *disableOfficial,
		OfficialPassphrase: os.Getenv("LOINC_OFFICIAL_PASSPHRASE"),
	}, nil
}

func normalizeServeArgs(args []string) []string {
	if len(args) == 0 || !isPortArg(args[0]) {
		return args
	}
	normalized := make([]string, 0, len(args)+1)
	normalized = append(normalized, "--port", args[0])
	normalized = append(normalized, args[1:]...)
	return normalized
}

func listenWithPortPrompt(addr string, in *os.File, out io.Writer) (net.Listener, error) {
	listener, err := net.Listen("tcp", addr)
	if err == nil {
		return listener, nil
	}
	if !isAddrInUse(err) || !isTerminal(in) {
		return nil, err
	}
	reader := bufio.NewReader(in)
	currentAddr := addr
	for {
		fmt.Fprintf(out, "Port %s is already in use. Enter a different port, or press Enter to cancel: ", displayPort(currentAddr))
		answer, readErr := reader.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, readErr
		}
		answer = strings.TrimSpace(answer)
		if answer == "" {
			return nil, err
		}
		nextAddr, normalizeErr := addrWithPort(currentAddr, answer)
		if normalizeErr != nil {
			fmt.Fprintf(out, "%v\n", normalizeErr)
			continue
		}
		listener, listenErr := net.Listen("tcp", nextAddr)
		if listenErr == nil {
			return listener, nil
		}
		if !isAddrInUse(listenErr) {
			return nil, listenErr
		}
		currentAddr = nextAddr
		err = listenErr
	}
}

func promptLaunchURL(url string, in *os.File, out io.Writer) {
	if !isTerminal(in) {
		fmt.Fprintf(out, "Open %s in your browser.\n", url)
		return
	}
	fmt.Fprintf(out, "Open %s in your browser now? [Y/n]: ", url)
	answer, err := bufio.NewReader(in).ReadString('\n')
	if errors.Is(err, io.EOF) && strings.TrimSpace(answer) == "" {
		fmt.Fprintf(out, "Open %s when ready.\n", url)
		return
	}
	if err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintf(out, "Could not read browser launch response: %v\n", err)
		return
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer == "" || answer == "y" || answer == "yes" {
		if err := openBrowser(url); err != nil {
			fmt.Fprintf(out, "Could not launch browser automatically. Open %s manually. Error: %v\n", url, err)
		}
		return
	}
	fmt.Fprintf(out, "Open %s when ready.\n", url)
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func serveURL(addr net.Addr) string {
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		return fmt.Sprintf("http://localhost:%d", tcpAddr.Port)
	}
	_, port, err := net.SplitHostPort(addr.String())
	if err == nil && port != "" {
		return "http://localhost:" + port
	}
	return "http://localhost:9005"
}

func isAddrInUse(err error) bool {
	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "address already in use") || strings.Contains(message, "only one usage of each socket address")
}

func displayPort(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err == nil && port != "" {
		return port
	}
	return strings.TrimPrefix(addr, ":")
}

func addrWithPort(addr string, port string) (string, error) {
	normalized, err := normalizePortFlag(port)
	if err != nil {
		return "", err
	}
	port = strings.TrimPrefix(normalized, ":")
	host, _, splitErr := net.SplitHostPort(addr)
	if splitErr != nil || host == "" {
		return ":" + port, nil
	}
	return net.JoinHostPort(host, port), nil
}

func isTerminal(file *os.File) bool {
	if file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func runMCP(args []string) error {
	if err := loadEnvFiles(); err != nil {
		return err
	}
	cfg, err := parseMCPConfig(args)
	if err != nil {
		return err
	}
	cfg.DocsDir = resolveDocsDir(cfg.DocsDir)
	dbPath := defaultDBPath()
	if err := ensureDatabaseFromLocalZip(context.Background(), ".", dbPath); err != nil {
		return err
	}
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: cfg.CacheEntries})
	if err != nil {
		return err
	}
	defer store.Close()
	// The stdio command opens one store for the process lifetime (no upload-triggered hot swap
	// like the HTTP server), so a fixed-store getter is enough here.
	getStore := func() (*loinc.Store, error) { return store, nil }
	mcpServer := loincmcp.New(loincmcp.Options{
		StoreGetter:  getStore,
		DocsDir:      cfg.DocsDir,
		OpenAPIJSON:  server.OpenAPIJSON,
		Terminology:  terminology.NewService(getStore),
		LuceneSearch: server.NewLuceneSearchFunc(cfg.SearchIndexPath, getStore),
	})
	return mcpServer.Run(context.Background(), &mcp.StdioTransport{})
}

func parseMCPConfig(args []string) (mcpConfig, error) {
	flags := flag.NewFlagSet("mcp", flag.ContinueOnError)
	cacheEntries := flags.Int("cache-entries", 2048, "maximum in-memory term cache entries")
	docsDir := flags.String("docs-dir", defaultAgentDocsDir(), "path to editable agent Markdown docs")
	searchIndexPath := flags.String("search-index-path", defaultSearchIndexPath(), "path to generated local Lucene-style search index")
	if err := flags.Parse(args); err != nil {
		return mcpConfig{}, err
	}
	return mcpConfig{CacheEntries: *cacheEntries, DocsDir: *docsDir, SearchIndexPath: *searchIndexPath}, nil
}

func ensureDatabaseFromLocalZip(ctx context.Context, cwd string, dbPath string) error {
	if err := ensureDatabaseDir(dbPath); err != nil {
		return err
	}
	hasData, err := databaseHasTerms(dbPath)
	if err != nil {
		return err
	}
	if hasData {
		return nil
	}
	zipPath, ok, err := findLocalReleaseZip(cwd)
	if err != nil {
		return err
	}
	if !ok {
		// An installed binary has no meaningful cwd, so also accept a zip dropped in the data dir.
		zipPath, ok, err = findLocalReleaseZip(filepath.Dir(dbPath))
		if err != nil || !ok {
			return err
		}
	}
	workDir := filepath.Join(filepath.Dir(dbPath), "bootstrap", time.Now().UTC().Format("20060102T150405.000000000"))
	releaseDir, err := loinc.ExtractReleaseZip(zipPath, filepath.Join(workDir, "release"))
	if err != nil {
		return err
	}
	summary, err := loinc.Ingest(ctx, loinc.IngestOptions{
		ReleaseDir: releaseDir,
		DBPath:     dbPath,
	})
	if err != nil {
		return err
	}
	fmt.Printf("Auto-imported %d LOINC terms from %s into %s\n", summary.TermCount, zipPath, dbPath)
	return nil
}

func ensureDatabaseDir(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if dir == "." || strings.TrimSpace(dir) == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create database directory %s: %w", dir, err)
	}
	return nil
}

func databaseHasTerms(dbPath string) (bool, error) {
	if strings.TrimSpace(dbPath) == "" {
		return false, fmt.Errorf("database path is required")
	}
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return false, err
	}
	defer db.Close()

	var tableName string
	err = db.QueryRow(`select name from sqlite_master where type = 'table' and name = 'loinc_terms'`).Scan(&tableName)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	var count int
	if err := db.QueryRow(`select count(*) from loinc_terms limit 1`).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func findLocalReleaseZip(cwd string) (string, bool, error) {
	var candidates []string
	root, err := filepath.Abs(cwd)
	if err != nil {
		return "", false, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", false, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".zip") && strings.Contains(name, "loinc") {
			candidates = append(candidates, filepath.Join(root, entry.Name()))
		}
	}
	if len(candidates) == 0 {
		return "", false, nil
	}
	sort.Strings(candidates)
	return candidates[0], true, nil
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}
		if existing, exists := os.LookupEnv(key); exists && existing != "" {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func defaultServeAddr() string {
	if addr := strings.TrimSpace(os.Getenv("LOINC_BROWSER_ADDR")); addr != "" {
		return addr
	}
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		if strings.HasPrefix(port, ":") {
			return port
		}
		return ":" + port
	}
	return ":9005"
}

func defaultUnixSocketPath() string {
	return strings.TrimSpace(os.Getenv("LOINC_BROWSER_UNIX_SOCKET"))
}

func defaultUDPAddr() string {
	return strings.TrimSpace(os.Getenv("LOINC_BROWSER_UDP_ADDR"))
}

// listenUnixSocket binds a Unix domain socket at path, clearing a stale
// socket left behind by a crashed process and refusing to touch a path that
// exists but is not a socket. The socket is chmod'd 0660 after listening so
// file permissions gate access.
// maxUnixSocketPathBytes is the conservative macOS sockaddr_un limit (Linux allows a few bytes
// more, at 108); using the smaller bound on every OS keeps the error consistent and avoids a
// path that works in dev on Linux but fails once deployed to macOS.
const maxUnixSocketPathBytes = 103

func listenUnixSocket(path string) (net.Listener, error) {
	if len(path) > maxUnixSocketPathBytes {
		return nil, fmt.Errorf("unix socket path is %d bytes; the OS limit is ~104 — use a shorter path such as /run/loinc/loinc.sock or a relative path", len(path))
	}
	if err := prepareUnixSocketPath(path); err != nil {
		return nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o660); err != nil {
		listener.Close()
		return nil, err
	}
	return listener, nil
}

func prepareUnixSocketPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("unix socket path %q exists and is not a socket", path)
	}
	conn, dialErr := net.DialTimeout("unix", path, 200*time.Millisecond)
	if dialErr == nil {
		conn.Close()
		return nil // a live server is already listening; let net.Listen report it in use
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing stale unix socket %q: %w", path, err)
	}
	return nil
}

// resolveDocsDir returns dir when it exists on disk. Otherwise (an installed binary with no docs/
// beside it) it refreshes the embedded docs into <data dir>/docs and returns its agent/ folder,
// keeping the docs/ + docs/agent/ layout the server and MCP readers expect.
func resolveDocsDir(dir string) string {
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	target := filepath.Join(dataDir(), "docs")
	if err := os.RemoveAll(target); err != nil {
		log.Printf("docs: %v; MCP docs and /docs pages unavailable", err)
		return dir
	}
	if err := os.CopyFS(target, docs.FS); err != nil {
		log.Printf("docs: extract embedded docs: %v; MCP docs and /docs pages unavailable", err)
		return dir
	}
	return filepath.Join(target, "agent")
}

func defaultAgentDocsDir() string {
	if dir := strings.TrimSpace(os.Getenv("LOINC_AGENT_DOCS_DIR")); dir != "" {
		return dir
	}
	return "./docs/agent"
}

func defaultOfficialAPIBaseURL() string {
	if value := strings.TrimSpace(os.Getenv("LOINC_OFFICIAL_API_BASE_URL")); value != "" {
		return value
	}
	return "https://loinc.regenstrief.org/searchapi"
}

func defaultAppKeyPath() string {
	if value := strings.TrimSpace(os.Getenv("LOINC_APP_KEY_PATH")); value != "" {
		return value
	}
	return filepath.Join(dataDir(), "loinc-browser-app.key")
}

func defaultKVPath() string {
	if value := strings.TrimSpace(os.Getenv("LOINC_KV_PATH")); value != "" {
		return value
	}
	return filepath.Join(dataDir(), "loinc-browser-kv.json")
}

func defaultSearchIndexPath() string {
	if value := strings.TrimSpace(os.Getenv("LOINC_SEARCH_INDEX_PATH")); value != "" {
		return value
	}
	return filepath.Join(dataDir(), "loinc-search.bleve")
}

func normalizePathFlag(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/mcp"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

type trackingStringFlag struct {
	value string
	set   bool
}

func (f *trackingStringFlag) String() string {
	return f.value
}

func (f *trackingStringFlag) Set(value string) error {
	f.value = strings.TrimSpace(value)
	f.set = true
	return nil
}

func normalizePortFlag(port string) (string, error) {
	port = strings.TrimSpace(port)
	if !isPortArg(port) {
		return "", fmt.Errorf("port must be a number from 1 to 65535")
	}
	if strings.HasPrefix(port, ":") {
		return port, nil
	}
	return ":" + port, nil
}

func isPortArg(value string) bool {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, ":")
	if value == "" || len(value) > 5 {
		return false
	}
	port := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
		port = port*10 + int(r-'0')
	}
	return port >= 1 && port <= 65535
}

func usage() error {
	fmt.Print(usageText())
	return nil
}

func usageText() string {
	return `Usage:
  loinc-browser
  loinc-browser 9005
  loinc-browser -v
  loinc-browser --port 9005
  loinc-browser --addr :9005
  loinc-browser ingest --release ./Loinc_2.82
  loinc-browser serve --addr :9005
  loinc-browser serve --addr :9005 --no-mcp
  loinc-browser serve --unix-socket ./data/loinc-browser.sock
  loinc-browser serve --udp-addr :8081
  loinc-browser mcp --docs-dir ./docs/agent --search-index-path ./data/loinc-search.bleve

Environment:
  LOINC_BROWSER_ADDR=:9005
  PORT=9005
  LOINC_BROWSER_UNIX_SOCKET= (off by default; Mode D local Unix socket transport)
  LOINC_BROWSER_UDP_ADDR= (off by default; Mode E compact UDP micro-protocol, e.g. :8081)
  LOINC_AGENT_DOCS_DIR=./docs/agent
  LOINC_BROWSER_DATA_DIR= (default: ./data if present, else the per-user data directory)
  LOINC_SEARCH_INDEX_PATH=<data dir>/loinc-search.bleve
  LOINC_OFFICIAL_DISABLED=false (or --no-official; turns off the online Search API proxy)
  LOINC_OFFICIAL_PASSPHRASE= (optional; required as X-Loinc-Passphrase on official requests)
  LOINC_OFFICIAL_USERNAME= / LOINC_OFFICIAL_PASSWORD= (optional; used for "saved credentials")
`
}

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
