package server

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"loinc-browser/internal/loinc"
)

// TestUnknownLangIgnored checks that an unrecognised lang= is ignored, not an error: the test
// fixture has no linguistic variant tables, so a recognised-language response is covered by the
// gated TestLocalizedNamesGerman in internal/loinc against the full release.
func TestUnknownLangIgnored(t *testing.T) {
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

	server := httptest.NewServer(New(Options{Store: store}))
	defer server.Close()

	var search loinc.SearchResponse
	getJSON(t, server.URL+"/api/v1/terms/search?q=cholesterol&lang=xx-XX", &search)
	if len(search.Results) != 1 || search.Results[0].LOINCNum != "2000-1" {
		t.Fatalf("expected cholesterol result, got %#v", search.Results)
	}
	if search.Results[0].LocalizedName != "" {
		t.Fatalf("expected no localizedName for an unrecognised lang, got %q", search.Results[0].LocalizedName)
	}

	var term loinc.Term
	getJSON(t, server.URL+"/api/v1/terms/2000-1?lang=xx-XX", &term)
	if term.LOINCNum != "2000-1" || term.LocalizedName != "" {
		t.Fatalf("expected no localizedName for an unrecognised lang, got %#v", term)
	}
}
