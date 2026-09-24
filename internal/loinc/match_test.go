package loinc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMatchBucket(t *testing.T) {
	cases := []struct {
		name string
		resp SearchResponse
		want string
	}{
		{
			name: "no results",
			resp: SearchResponse{},
			want: "none",
		},
		{
			name: "typed LOINC number matches the top result",
			resp: SearchResponse{Results: []SearchResult{{LOINCNum: "2160-0", Rank: 1}}},
			want: "confident",
		},
		{
			name: "top result pinned by CLCI name match",
			resp: SearchResponse{
				Results:     []SearchResult{{LOINCNum: "2160-0", Rank: 1}, {LOINCNum: "9999-9", Rank: 1.1}},
				CLCIMatches: []string{"2160-0"},
			},
			want: "confident",
		},
		{
			name: "single unambiguous result",
			resp: SearchResponse{Results: []SearchResult{{LOINCNum: "2160-0", Rank: -3}}},
			want: "confident",
		},
		{
			name: "clear margin between top and runner-up",
			resp: SearchResponse{Results: []SearchResult{
				{LOINCNum: "2160-0", Rank: -10},
				{LOINCNum: "9999-9", Rank: -2},
			}},
			want: "confident",
		},
		{
			name: "close tie between top and runner-up",
			resp: SearchResponse{Results: []SearchResult{
				{LOINCNum: "2160-0", Rank: -3.0},
				{LOINCNum: "9999-9", Rank: -2.5},
			}},
			want: "review",
		},
		{
			name: "relaxed retry never counts as confident on rank alone",
			resp: SearchResponse{
				Relaxed: true,
				Results: []SearchResult{{LOINCNum: "2160-0", Rank: -10}},
			},
			want: "review",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchBucket(tc.resp, "S. Creatinine"); got != tc.want {
				t.Fatalf("matchBucket() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMatchNamesOrderAndEmptyNames(t *testing.T) {
	ctx := context.Background()
	releaseDir := writeTestRelease(t)
	dbPath := filepath.Join(t.TempDir(), "loinc.sqlite")
	if _, err := Ingest(ctx, IngestOptions{ReleaseDir: releaseDir, DBPath: dbPath}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	store, err := OpenStore(dbPath, StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	matches, err := store.MatchNames(ctx, []string{"glucose plasma", "  ", "1000-1", "nonexistentxyz"}, SearchParams{})
	if err != nil {
		t.Fatalf("MatchNames: %v", err)
	}
	if len(matches) != 4 {
		t.Fatalf("expected 4 matches in input order, got %d", len(matches))
	}
	if matches[0].Name != "glucose plasma" || matches[0].Bucket != "confident" || len(matches[0].Candidates) == 0 || matches[0].Candidates[0].LOINCNum != "1000-1" {
		t.Fatalf("unexpected match[0]: %#v", matches[0])
	}
	if matches[1].Bucket != "none" || len(matches[1].Candidates) != 0 {
		t.Fatalf("expected blank name to bucket none with no candidates, got %#v", matches[1])
	}
	if matches[2].Bucket != "confident" || matches[2].Candidates[0].LOINCNum != "1000-1" {
		t.Fatalf("expected typed LOINC number to be confident, got %#v", matches[2])
	}
	if matches[3].Bucket != "none" {
		t.Fatalf("expected no candidates for an unmatched name, got %#v", matches[3])
	}
}

// TestMatchNamesAgainstCLCI evaluates the confident/wrong-confident split of matchConfidentMargin
// against Common Lab Codes for India General Names (real Indian-lab test names, each with a known
// LOINC code). Needs LOINC_TEST_DB=./data/loinc-normalized.sqlite and the CLCI CSV loaded from the
// data directory next to it; skips otherwise.
func TestMatchNamesAgainstCLCI(t *testing.T) {
	dbPath := os.Getenv("LOINC_TEST_DB")
	if dbPath == "" {
		t.Skip("LOINC_TEST_DB is not set; skipping CLCI name-match evaluation")
	}
	dataDir := filepath.Dir(dbPath)
	clciCount, err := LoadCLCI(FindCLCIFile(dataDir))
	if err != nil {
		t.Fatal(err)
	}
	if clciCount == 0 {
		t.Skip("no CLCI CSV found next to LOINC_TEST_DB; skipping")
	}
	defer clci.Store(nil)

	store, err := OpenStore(dbPath, StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open %s: %v", dbPath, err)
	}
	defer store.Close()

	nameToCodes := map[string][]string{}
	for code, name := range clci.Load().names {
		nameToCodes[name] = append(nameToCodes[name], code)
	}
	names := make([]string, 0, len(nameToCodes))
	for name := range nameToCodes {
		names = append(names, name)
	}

	matches, err := store.MatchNames(context.Background(), names, SearchParams{})
	if err != nil {
		t.Fatalf("MatchNames: %v", err)
	}

	var confident, correct, wrong int
	for _, m := range matches {
		if m.Bucket != "confident" {
			continue
		}
		confident++
		want := nameToCodes[m.Name]
		got := ""
		if len(m.Candidates) > 0 {
			got = m.Candidates[0].LOINCNum
		}
		if containsValue(want, got) {
			correct++
		} else {
			wrong++
		}
	}
	wrongShare := 0.0
	if confident > 0 {
		wrongShare = float64(wrong) / float64(confident)
	}
	t.Logf("CLCI General Names: %d total, %d confident (%.1f%%), %d correct, %d wrong (%.1f%% of confident)",
		len(names), confident, 100*float64(confident)/float64(len(names)), correct, wrong, 100*wrongShare)
	if wrongShare > 0.03 {
		t.Fatalf("wrong-confident share %.1f%% exceeds 3%% target; retune matchConfidentMargin", 100*wrongShare)
	}
}
