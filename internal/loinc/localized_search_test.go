package loinc

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// openLocalizedTestStore opens LOINC_TEST_DB and waits for loinc_variant_fts to finish its
// background build (startVariantFTSBuild), so these tests don't race the goroutine OpenStore
// kicks off. t.Skip when LOINC_TEST_DB isn't set, same as TestLocalizedNamesGerman.
func openLocalizedTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := os.Getenv("LOINC_TEST_DB")
	if dbPath == "" {
		t.Skip("LOINC_TEST_DB is not set; skipping localized word search against the full release")
	}
	store, err := OpenStore(dbPath, StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open %s: %v", dbPath, err)
	}
	t.Cleanup(func() { store.Close() })

	deadline := time.Now().Add(2 * time.Minute)
	for !store.variantFTS.ready.Load() {
		if time.Now().After(deadline) {
			t.Fatalf("loinc_variant_fts did not become ready within 2m")
		}
		time.Sleep(50 * time.Millisecond)
	}
	return store
}

// TestLocalizedSearchGermanWord checks that a German word ("Natrium") finds the German-named term
// via loinc_variant_fts even though English search alone matches nothing for that word.
func TestLocalizedSearchGermanWord(t *testing.T) {
	store := openLocalizedTestStore(t)
	ctx := context.Background()

	response, err := store.Search(ctx, SearchParams{Query: "Natrium", Lang: "de-DE", Limit: 5})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	found := false
	for _, result := range response.Results {
		if result.LOINCNum == "2951-2" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Search(q=Natrium, lang=de-DE) did not return 2951-2 in top %d: %#v", len(response.Results), response.Results)
	}
}

// TestLocalizedSearchGermanUrea checks a second German word maps to urea terms, so the merge isn't
// tied to one hand-picked code.
func TestLocalizedSearchGermanUrea(t *testing.T) {
	store := openLocalizedTestStore(t)
	ctx := context.Background()

	response, err := store.Search(ctx, SearchParams{Query: "Harnstoff", Lang: "de-DE", Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(response.Results) == 0 {
		t.Fatalf("Search(q=Harnstoff, lang=de-DE) returned no results")
	}
	for _, result := range response.Results {
		if !strings.Contains(strings.ToLower(result.Component), "urea") {
			t.Errorf("Search(q=Harnstoff, lang=de-DE) returned non-urea component %q for %s", result.Component, result.LOINCNum)
		}
	}
}

// TestLocalizedSearchWithoutLangUnaffected checks that lang search only adds matches: with lang
// unset, a German word still finds nothing (unchanged English-only behavior).
func TestLocalizedSearchWithoutLangUnaffected(t *testing.T) {
	store := openLocalizedTestStore(t)
	ctx := context.Background()

	response, err := store.Search(ctx, SearchParams{Query: "Natrium", Limit: 5})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	for _, result := range response.Results {
		if result.LOINCNum == "2951-2" {
			t.Fatalf("Search(q=Natrium) with no lang unexpectedly returned 2951-2")
		}
	}
}

// TestLocalizedSearchEnglishQueryUnaffectedByLang checks that an English query's top result is the
// same with or without lang set: the merge only adds results, it never reorders or replaces an
// English match.
func TestLocalizedSearchEnglishQueryUnaffectedByLang(t *testing.T) {
	store := openLocalizedTestStore(t)
	ctx := context.Background()

	plain, err := store.Search(ctx, SearchParams{Query: "serum sodium", Limit: 5})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(plain.Results) == 0 || plain.Results[0].LOINCNum != "2951-2" {
		t.Fatalf("Search(q=serum sodium) top result = %#v, want 2951-2 first", plain.Results)
	}

	localized, err := store.Search(ctx, SearchParams{Query: "serum sodium", Lang: "de-DE", Limit: 5})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(localized.Results) == 0 || localized.Results[0].LOINCNum != plain.Results[0].LOINCNum {
		t.Fatalf("Search(q=serum sodium, lang=de-DE) top result = %#v, want %s first", localized.Results, plain.Results[0].LOINCNum)
	}
}
