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
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"loinc-browser/internal/loinc"
	loincmcp "loinc-browser/internal/mcpserver"
	"loinc-browser/internal/server"
	"loinc-browser/internal/version"
	"loinc-browser/web"
)

const defaultDBPath = "./data/loinc-normalized.sqlite"

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
	DBPath       string
	Addr         string
	CacheEntries int
	EnableMCP    bool
	MCPPath      string
	DocsDir      string
}

type mcpConfig struct {
	CacheEntries int
	DocsDir      string
}

func runIngest(args []string) error {
	flags := flag.NewFlagSet("ingest", flag.ContinueOnError)
	releaseDir := flags.String("release", "", "path to local LOINC release directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	summary, err := loinc.Ingest(context.Background(), loinc.IngestOptions{
		ReleaseDir: *releaseDir,
		DBPath:     defaultDBPath,
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
	if err := loadDotEnv(".env"); err != nil {
		return err
	}
	cfg, err := parseServeConfig(args)
	if err != nil {
		return err
	}
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
	handler := server.New(server.Options{
		Store:        store,
		Assets:       assets,
		DBPath:       cfg.DBPath,
		UploadDir:    "./data/uploads",
		CacheEntries: cfg.CacheEntries,
		EnableMCP:    cfg.EnableMCP,
		MCPPath:      cfg.MCPPath,
		DocsDir:      cfg.DocsDir,
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
	go promptLaunchURL(url, os.Stdin, os.Stdout)
	return http.Serve(listener, handler)
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
		DBPath:       defaultDBPath,
		Addr:         listenAddr,
		CacheEntries: *cacheEntries,
		EnableMCP:    *enableMCP && !*disableMCP,
		MCPPath:      normalizePathFlag(*mcpPath),
		DocsDir:      *docsDir,
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
	if err := loadDotEnv(".env"); err != nil {
		return err
	}
	cfg, err := parseMCPConfig(args)
	if err != nil {
		return err
	}
	if err := ensureDatabaseFromLocalZip(context.Background(), ".", defaultDBPath); err != nil {
		return err
	}
	store, err := loinc.OpenStore(defaultDBPath, loinc.StoreOptions{CacheEntries: cfg.CacheEntries})
	if err != nil {
		return err
	}
	defer store.Close()
	mcpServer := loincmcp.New(loincmcp.Options{
		Store:       store,
		DocsDir:     cfg.DocsDir,
		OpenAPIJSON: server.OpenAPIJSON,
	})
	return mcpServer.Run(context.Background(), &mcp.StdioTransport{})
}

func parseMCPConfig(args []string) (mcpConfig, error) {
	flags := flag.NewFlagSet("mcp", flag.ContinueOnError)
	cacheEntries := flags.Int("cache-entries", 2048, "maximum in-memory term cache entries")
	docsDir := flags.String("docs-dir", defaultAgentDocsDir(), "path to editable agent Markdown docs")
	if err := flags.Parse(args); err != nil {
		return mcpConfig{}, err
	}
	return mcpConfig{CacheEntries: *cacheEntries, DocsDir: *docsDir}, nil
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
		return nil
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
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if rel == "data" || strings.HasPrefix(rel, "data"+string(filepath.Separator)) {
				return filepath.SkipDir
			}
			if rel != "." && strings.Count(rel, string(filepath.Separator)) >= 2 {
				return filepath.SkipDir
			}
			return nil
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".zip") && strings.Contains(name, "loinc") {
			candidates = append(candidates, path)
		}
		return nil
	})
	if err != nil {
		return "", false, err
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

func defaultAgentDocsDir() string {
	if dir := strings.TrimSpace(os.Getenv("LOINC_AGENT_DOCS_DIR")); dir != "" {
		return dir
	}
	return "./docs/agent"
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
  loinc-browser mcp --docs-dir ./docs/agent

Environment:
  LOINC_BROWSER_ADDR=:9005
  PORT=9005
  LOINC_AGENT_DOCS_DIR=./docs/agent
`
}
