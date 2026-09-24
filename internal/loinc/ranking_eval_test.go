package loinc

import (
	"context"
	"os"
	"path/filepath"
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
	// Rank as the server does: with the CLCI prior when that file sits in the data directory.
	if count, err := LoadCLCI(FindCLCIFile(filepath.Dir(dbPath))); err != nil {
		t.Fatal(err)
	} else if count > 0 {
		t.Logf("ranking with the CLCI prior (%d terms)", count)
		defer clci.Store(nil)
	}

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
		// Not "vitamin d": 1989-3 is named "25-hydroxyvitamin D3" (one word), so the word "vitamin"
		// only reaches it through related names and ~10 points of text score behind "Vit D+metab";
		// no sane popularity weight bridges that. Meaning search (mode=hybrid) is the fix.
		{"urine", []string{"5778-6", "5767-9", "2514-8", "630-4"}}, // common urine tests, not rank-161 sediment
		{"homocysteine", []string{"13965-9", "2428-1"}},
		{"lipase", []string{"3040-3"}},
		{"platelet count", []string{"777-3"}},
		{"urine culture", []string{"630-4"}},
		{"ldl cholesterol calculated", []string{"13457-7"}},
		{"ferritin", []string{"2276-4", "20567-4"}},
		{"dengue igm", []string{"23992-1", "25338-5"}},
		{"hba1c fasting", []string{"4548-4"}},        // relaxed: must drop "fasting", not the analyte
		{"tc dc esr", []string{"4537-7", "30341-2"}}, // combined request: the ESR must stay findable
		{"phosphorus", []string{"2777-1"}},           // LOINC says "Phosphate", not "phosphorus"
		{"ncct neck", []string{"36514-8"}},           // non-contrast CT: "CT Neck WO contrast", not 36051-1
		// Known word-search misses from a lab compendium (hybrid finds the first two):
		//   "ham test" -> 13533-5 (acid hemolysis); "vitamin d" -> 1989-3/62292-8 (see above);
		//   "pt inr" -> 34528-0 PT panel: a two-test request, and no term names both PT and INR
		//   except the INR goal 92891-1, which wins; "urea calculated urease" -> 3094-0.
	}
	// What a lab mapper should pass.
	classType := map[string]string{"urine": "lab"}
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

	// "Widal" is not in LOINC; the widal->typhi synonym finds the S. Typhi antibody terms.
	widal, err := store.Search(context.Background(), SearchParams{Query: "widal test", Limit: 3})
	if err != nil {
		t.Fatalf("search widal test: %v", err)
	}
	if len(widal.Results) == 0 || !strings.Contains(widal.Results[0].ShortName, "Typhi") {
		t.Errorf("widal test: want S. Typhi antibody terms first, got %+v", widal.Results)
	}

	// A word LOINC never uses, with only generic words left: nothing, not thousands of terms.
	unknown, err := store.Search(context.Background(), SearchParams{Query: "typhidot test", Limit: 3})
	if err != nil {
		t.Fatalf("search typhidot test: %v", err)
	}
	if unknown.Total != 0 {
		t.Errorf("typhidot test: want no results, got total=%d", unknown.Total)
	}

	// Context filters for mapping: the panel holding PT and INR, radiology parts, lab orders.
	for name, check := range map[string]struct {
		params SearchParams
		want   string
	}{
		"panel containing PT+INR":   {SearchParams{PanelContains: []string{"5902-2", "6301-6"}, PanelOnly: true, Limit: 1}, "34528-0"},
		"CT head without contrast":  {SearchParams{RadParts: map[string]string{"Rad.Modality.Modality Type": "ct", "Rad.Anatomic Location.Region Imaged": "head", "Rad.Timing": "WO"}, Limit: 1}, "30799-1"},
		"lab order glucose":         {SearchParams{Query: "glucose", UniversalLabOrders: true, Limit: 1}, "2345-7"},
		"TSH variants by component": {SearchParams{Component: "thyrotropin", System: "Ser/Plas", Limit: 1}, "3016-3"},
	} {
		response, err := store.Search(context.Background(), check.params)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(response.Results) == 0 || response.Results[0].LOINCNum != check.want {
			t.Errorf("%s: want %s first, got %+v", name, check.want, response.Results)
		}
	}
}
