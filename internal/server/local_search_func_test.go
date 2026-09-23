package server

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"loinc-browser/internal/loinc"
)

// TestNewLuceneSearchFuncQueriesBuiltIndex covers the exported constructor cmd/loinc-browser's
// stdio `mcp` subcommand uses to wire loinc_lucene_search: it must query the same Bleve index the
// HTTP server's rebuild endpoint builds, sharing local_search.go's query logic.
func TestNewLuceneSearchFuncQueriesBuiltIndex(t *testing.T) {
	ctx := context.Background()
	releaseDir := writeServerTestRelease(t)
	dbPath := filepath.Join(t.TempDir(), "loinc.sqlite")
	if _, err := loinc.Ingest(ctx, loinc.IngestOptions{ReleaseDir: releaseDir, DBPath: dbPath}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	indexPath := filepath.Join(t.TempDir(), "loinc-search.bleve")
	localSearch := newLocalSearchService(indexPath)
	if status, err := localSearch.rebuild(ctx, store); err != nil || status.State != "ready" {
		t.Fatalf("rebuild local search index: status=%#v err=%v", status, err)
	}

	getStore := func() (*loinc.Store, error) { return store, nil }
	search := NewLuceneSearchFunc(indexPath, getStore)

	hits, total, err := search(ctx, "loincs", "Component:Cholesterol", 10, 0)
	if err != nil {
		t.Fatalf("lucene search: %v", err)
	}
	if total == 0 || len(hits) == 0 {
		t.Fatalf("expected at least one hit for Component:Cholesterol, got total=%d hits=%#v", total, hits)
	}
}

// TestNewLuceneSearchFuncReportsMissingIndex mirrors the HTTP server's "not ready" error for the
// stdio transport, so loinc_lucene_search fails clearly instead of guessing an index location.
func TestNewLuceneSearchFuncReportsMissingIndex(t *testing.T) {
	ctx := context.Background()
	releaseDir := writeServerTestRelease(t)
	dbPath := filepath.Join(t.TempDir(), "loinc.sqlite")
	if _, err := loinc.Ingest(ctx, loinc.IngestOptions{ReleaseDir: releaseDir, DBPath: dbPath}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	indexPath := filepath.Join(t.TempDir(), "never-built.bleve")
	getStore := func() (*loinc.Store, error) { return store, nil }
	search := NewLuceneSearchFunc(indexPath, getStore)

	if _, _, err := search(ctx, "loincs", "Component:Cholesterol", 10, 0); err == nil || !strings.Contains(err.Error(), searchAPIMissingIndex) {
		t.Fatalf("expected missing-index error, got %v", err)
	}
}
