package server

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"

	"loinc-browser/internal/loinc"
)

func TestLocalSearchIndexReportsIncompleteAndStaleBuilds(t *testing.T) {
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
	defer store.Close()

	indexPath := filepath.Join(t.TempDir(), "loinc-search.bleve")
	svc := newLocalSearchService(indexPath)
	if status, err := svc.rebuild(ctx, store); err != nil || status.State != "ready" {
		t.Fatalf("rebuild: %v %#v", err, status)
	}
	if _, err := os.Stat(indexPath + ".building"); !os.IsNotExist(err) {
		t.Fatalf("expected no leftover build directory, stat err %v", err)
	}

	// Stop words don't turn into required terms, in bare or fielded free text.
	for _, pair := range [][2]string{{"Cholesterol", "Cholesterol for the"}, {"Component:Cholesterol", "Component:(Cholesterol of)"}} {
		plain, _, err := svc.query(ctx, store, LocalSearchRequest{Scope: "loincs", Query: pair[0]})
		if err != nil {
			t.Fatalf("query %q: %v", pair[0], err)
		}
		withStops, _, err := svc.query(ctx, store, LocalSearchRequest{Scope: "loincs", Query: pair[1]})
		if err != nil {
			t.Fatalf("query %q: %v", pair[1], err)
		}
		if plain.Total == 0 || withStops.Total != plain.Total {
			t.Fatalf("expected %q (%d) to match %q (%d)", pair[1], withStops.Total, pair[0], plain.Total)
		}
	}

	// A newer import makes the index stale; it still answers queries.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	if _, err := db.Exec(`update import_meta set value = ? where key = 'imported_at'`, future); err != nil {
		t.Fatalf("update imported_at: %v", err)
	}
	db.Close()
	if status := svc.status(ctx, store); status.State != "stale" {
		t.Fatalf("expected stale status, got %#v", status)
	}
	if _, code, err := svc.query(ctx, store, LocalSearchRequest{Scope: "loincs", Query: "Cholesterol"}); err != nil {
		t.Fatalf("stale index should still answer, got %d %v", code, err)
	}

	// An index without the built-at marker was interrupted mid-build: report it and refuse queries.
	index, err := bleve.Open(indexPath)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	if err := index.DeleteInternal([]byte(localSearchBuiltAtKey)); err != nil {
		t.Fatalf("delete marker: %v", err)
	}
	index.Close()
	if status := svc.status(ctx, store); status.State != "incomplete" {
		t.Fatalf("expected incomplete status, got %#v", status)
	}
	if _, code, err := svc.query(ctx, store, LocalSearchRequest{Scope: "loincs", Query: "Cholesterol"}); err == nil || code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for incomplete index, got %d %v", code, err)
	}
}
