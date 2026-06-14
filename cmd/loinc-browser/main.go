package main

import (
	"bufio"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"loinc-browser/internal/loinc"
	loincmcp "loinc-browser/internal/mcpserver"
	"loinc-browser/internal/server"
	"loinc-browser/web"
)

func main() {
	log.SetFlags(0)
	if err := run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	if len(args) < 2 {
		return usage()
	}
	switch args[1] {
	case "ingest":
		return runIngest(args[2:])
	case "serve":
		return runServe(args[2:])
	case "mcp":
		return runMCP(args[2:])
	case "-h", "--help", "help":
		return usage()
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[1], usageText())
	}
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
	DBPath       string
	CacheEntries int
	DocsDir      string
}

func runIngest(args []string) error {
	flags := flag.NewFlagSet("ingest", flag.ContinueOnError)
	releaseDir := flags.String("release", "", "path to local LOINC release directory")
	dbPath := flags.String("db", "./data/loinc-normalized.sqlite", "path to generated SQLite database")
	if err := flags.Parse(args); err != nil {
		return err
	}
	summary, err := loinc.Ingest(context.Background(), loinc.IngestOptions{
		ReleaseDir: *releaseDir,
		DBPath:     *dbPath,
	})
	if err != nil {
		return err
	}
	fmt.Printf("Imported %d LOINC terms into %s\n", summary.TermCount, summary.DBPath)
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
	fmt.Printf("Serving LOINC browser on http://localhost%s\n", cfg.Addr)
	if cfg.EnableMCP {
		fmt.Printf("Serving LOINC MCP over HTTP at http://localhost%s%s\n", cfg.Addr, cfg.MCPPath)
	}
	return http.ListenAndServe(cfg.Addr, handler)
}

func parseServeConfig(args []string) (serveConfig, error) {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	dbPath := flags.String("db", "./data/loinc-normalized.sqlite", "path to generated SQLite database")
	addr := flags.String("addr", defaultServeAddr(), "HTTP listen address")
	cacheEntries := flags.Int("cache-entries", 2048, "maximum in-memory term cache entries")
	enableMCP := flags.Bool("mcp", false, "enable local MCP over HTTP")
	mcpPath := flags.String("mcp-path", "/mcp", "HTTP MCP route path")
	docsDir := flags.String("docs-dir", defaultAgentDocsDir(), "path to editable agent Markdown docs")
	if err := flags.Parse(args); err != nil {
		return serveConfig{}, err
	}
	return serveConfig{
		DBPath:       *dbPath,
		Addr:         *addr,
		CacheEntries: *cacheEntries,
		EnableMCP:    *enableMCP,
		MCPPath:      normalizePathFlag(*mcpPath),
		DocsDir:      *docsDir,
	}, nil
}

func runMCP(args []string) error {
	if err := loadDotEnv(".env"); err != nil {
		return err
	}
	cfg, err := parseMCPConfig(args)
	if err != nil {
		return err
	}
	store, err := loinc.OpenStore(cfg.DBPath, loinc.StoreOptions{CacheEntries: cfg.CacheEntries})
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
	dbPath := flags.String("db", "./data/loinc-normalized.sqlite", "path to generated SQLite database")
	cacheEntries := flags.Int("cache-entries", 2048, "maximum in-memory term cache entries")
	docsDir := flags.String("docs-dir", defaultAgentDocsDir(), "path to editable agent Markdown docs")
	if err := flags.Parse(args); err != nil {
		return mcpConfig{}, err
	}
	return mcpConfig{DBPath: *dbPath, CacheEntries: *cacheEntries, DocsDir: *docsDir}, nil
}

func ensureDatabaseFromLocalZip(ctx context.Context, cwd string, dbPath string) error {
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
	return ":8080"
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

func usage() error {
	fmt.Print(usageText())
	return nil
}

func usageText() string {
	return `Usage:
  loinc-browser ingest --release ./Loinc_2.82 --db ./data/loinc-normalized.sqlite
  loinc-browser serve --db ./data/loinc-normalized.sqlite --addr :8080
  loinc-browser serve --db ./data/loinc-normalized.sqlite --addr :8080 --mcp
  loinc-browser mcp --db ./data/loinc-normalized.sqlite --docs-dir ./docs/agent

Environment:
  LOINC_BROWSER_ADDR=:8080
  PORT=8080
  LOINC_AGENT_DOCS_DIR=./docs/agent
`
}
