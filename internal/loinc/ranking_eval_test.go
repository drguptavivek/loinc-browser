package loinc

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestRankingAgainstMappingProbe checks that plain clinical queries put the commonly used LOINC
// term in the top 3. The queries and expected codes come from a real lab-compendium mapping
// probe; it needs the full release (LOINC_TEST_DB=./data/loinc-normalized.sqlite).
func TestRankingAgainstMappingProbe(t *testing.T) {
	dbPath := os.Getenv("LOINC_TEST_DB")
	if dbPath == "" {
		t.Skip("LOINC_TEST_DB is not set; skipping ranking evaluation against the full release")
	}
	store, err := OpenStore(dbPath, StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open %s: %v", dbPath, err)
	}
	defer store.Close()

	probes := []struct {
		query string
		want  []string // any of these in the top 3 passes
	}{
		{"glucose fasting", []string{"1558-6", "76629-5"}},
		{"CRP", []string{"1988-5"}},
		{"HBsAg", []string{"5195-3"}},
		{"blood group", []string{"883-9", "882-1"}}, // ABO, or ABO + Rh
		{"TSH", []string{"3016-3"}},
		// Not "ESR": LOINC's related names for 4537-7/30341-2 say "Sed Rat", never "ESR", so only
		// a client-side synonym (ESR -> sed rate) can find them.
		{"HbA1c", []string{"4548-4"}},
		{"bilirubin total", []string{"1975-2"}},
		{"sgpt alt", []string{"1742-6"}},
		{"potassium serum", []string{"2823-3"}},
		{"creatinine serum", []string{"2160-0"}},
		{"vitamin d", []string{"1989-3", "62292-8"}},
		{"homocysteine", []string{"13965-9", "2428-1"}},
		{"lipase", []string{"3040-3"}},
		{"platelet count", []string{"777-3"}},
		{"urine culture", []string{"630-4"}},
		{"ldl cholesterol calculated", []string{"13457-7"}},
		{"ferritin", []string{"2276-4", "20567-4"}},
		{"dengue igm", []string{"23992-1", "25338-5"}},
		{"hba1c fasting", []string{"4548-4"}}, // relaxed: must drop "fasting", not the analyte
	}
	// What a lab mapper should pass: without classType=lab a PhenX survey protocol ranks #2 for
	// "vitamin d".
	classType := map[string]string{"vitamin d": "lab"}
	for _, probe := range probes {
		response, err := store.Search(context.Background(), SearchParams{Query: probe.query, ClassType: classType[probe.query], Limit: 3})
		if err != nil {
			t.Fatalf("search %q: %v", probe.query, err)
		}
		got := make([]string, 0, len(response.Results))
		found := false
		for _, result := range response.Results {
			got = append(got, result.LOINCNum+" "+result.ShortName)
			for _, want := range probe.want {
				found = found || result.LOINCNum == want
			}
		}
		if !found {
			t.Errorf("%q: want one of %v in the top 3, got %s", probe.query, probe.want, strings.Join(got, "; "))
		}
	}

	// Generic request words go before the specimen: "urine routine examination" must keep urine.
	urinalysis, err := store.Search(context.Background(), SearchParams{Query: "urine routine examination", ClassType: "lab", Limit: 3})
	if err != nil {
		t.Fatalf("search urine routine examination: %v", err)
	}
	for _, word := range urinalysis.DroppedWords {
		if word == "urine" {
			t.Errorf("urine routine examination: dropped the specimen, got %v", urinalysis.DroppedWords)
		}
	}
	if urinalysis.Total == 0 {
		t.Errorf("urine routine examination: want urine results, got none (dropped %v)", urinalysis.DroppedWords)
	}

	// Specimen words are never dropped, even when that means no results.
	for _, query := range []string{"vitreous tap fungal culture", "glucose urine zzzz"} {
		response, err := store.Search(context.Background(), SearchParams{Query: query, ClassType: "lab", Limit: 3})
		if err != nil {
			t.Fatalf("search %q: %v", query, err)
		}
		for _, word := range response.DroppedWords {
			if specimenWords[word] {
				t.Errorf("%q: dropped specimen word %q (dropped %v)", query, word, response.DroppedWords)
			}
		}
	}

	// "Widal" is not in LOINC; dropping it would leave only "test", matching thousands of terms.
	widal, err := store.Search(context.Background(), SearchParams{Query: "widal test", Limit: 3})
	if err != nil {
		t.Fatalf("search widal test: %v", err)
	}
	if widal.Total != 0 || widal.Relaxed {
		t.Errorf("widal test: want no results rather than a generic relaxed set, got total=%d relaxed=%v", widal.Total, widal.Relaxed)
	}
}
