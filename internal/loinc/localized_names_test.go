package loinc

import (
	"context"
	"os"
	"testing"
)

// TestLocalizedNamesGerman checks Store.LocalizedNames and the Lang search param against a real
// German linguistic variant; it needs the full release (LOINC_TEST_DB=./data/loinc-normalized.sqlite)
// since the test fixture (writeTestRelease) carries no linguistic variant tables.
func TestLocalizedNamesGerman(t *testing.T) {
	dbPath := os.Getenv("LOINC_TEST_DB")
	if dbPath == "" {
		t.Skip("LOINC_TEST_DB is not set; skipping linguistic variant evaluation against the full release")
	}
	store, err := OpenStore(dbPath, StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open %s: %v", dbPath, err)
	}
	defer store.Close()
	ctx := context.Background()

	const code = "2951-2"
	const want = "Natrium [Mol/Volumen] in Serum oder Plasma"

	names, err := store.LocalizedNames(ctx, "de-DE", []string{code})
	if err != nil {
		t.Fatalf("LocalizedNames: %v", err)
	}
	if got := names[code]; got != want {
		t.Fatalf("LocalizedNames(de-DE, %s) = %q, want %q", code, got, want)
	}

	response, err := store.Search(ctx, SearchParams{Query: code, Lang: "de-DE", Limit: 5})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(response.Results) == 0 || response.Results[0].LocalizedName != want {
		t.Fatalf("Search(lang=de-DE) localizedName = %q, want %q", firstLocalizedName(response), want)
	}

	// An unrecognised language is ignored, not an error.
	unknown, err := store.LocalizedNames(ctx, "xx-XX", []string{code})
	if err != nil {
		t.Fatalf("LocalizedNames(unknown lang): %v", err)
	}
	if len(unknown) != 0 {
		t.Fatalf("LocalizedNames(unknown lang) = %v, want empty", unknown)
	}
}

func firstLocalizedName(response SearchResponse) string {
	if len(response.Results) == 0 {
		return ""
	}
	return response.Results[0].LocalizedName
}
