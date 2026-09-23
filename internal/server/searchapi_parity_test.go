package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/blevesearch/bleve/v2"

	"loinc-browser/internal/loinc"
)

// TestSearchAPIExemplarKeySetParity replays every captured
// docs/exemplars/searchapi/*.json request against the real local database
// and search index, and compares the JSON key skeleton (paths + JSON types,
// null tolerated either way) rather than values, since the real DB (2.82)
// and the upstream capture (2.83) hold different data (plan §9.2).
//
// It only runs when LOINC_TEST_DB and LOINC_TEST_SEARCH_INDEX point at real,
// already-built files and docs/exemplars/searchapi exists; otherwise it
// skips. It opens both read-only in spirit: the store is never written to
// beyond the sqlite driver's own housekeeping pragmas, and the search index
// is only ever queried, never rebuilt in place.
func TestSearchAPIExemplarKeySetParity(t *testing.T) {
	dbPath := os.Getenv("LOINC_TEST_DB")
	indexPath := os.Getenv("LOINC_TEST_SEARCH_INDEX")
	exemplarDir := filepath.Join("..", "..", "docs", "exemplars", "searchapi")
	if dbPath == "" || indexPath == "" {
		t.Skip("set LOINC_TEST_DB and LOINC_TEST_SEARCH_INDEX to run the exemplar parity test")
	}
	if _, err := os.Stat(exemplarDir); err != nil {
		t.Skip("docs/exemplars/searchapi not present")
	}
	if _, err := os.Stat(indexPath); err != nil {
		t.Skipf("local search index %s not present", indexPath)
	}

	// A stale/incompatible index should skip rather than fail the suite.
	index, err := bleve.Open(indexPath)
	if err != nil {
		t.Skipf("local search index %s cannot be opened (stale?): %v", indexPath, err)
	}
	count, docCountErr := index.DocCount()
	_ = index.Close()
	if docCountErr != nil || count == 0 {
		t.Skipf("local search index %s is empty or unreadable", indexPath)
	}

	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 16})
	if err != nil {
		t.Fatalf("open real store: %v", err)
	}
	defer store.Close()

	server := httptest.NewServer(New(Options{Store: store, SearchIndexPath: indexPath}))
	defer server.Close()

	entries, err := os.ReadDir(exemplarDir)
	if err != nil {
		t.Fatalf("read exemplar dir: %v", err)
	}
	ran := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		t.Run(name, func(t *testing.T) {
			exemplarBytes, err := os.ReadFile(filepath.Join(exemplarDir, entry.Name()))
			if err != nil {
				t.Fatalf("read exemplar: %v", err)
			}
			var exemplar map[string]any
			if err := json.Unmarshal(exemplarBytes, &exemplar); err != nil {
				t.Fatalf("parse exemplar: %v", err)
			}
			summary, _ := exemplar["ResponseSummary"].(map[string]any)
			queryURL, _ := summary["QueryUrl"].(string)
			parsed, err := url.Parse(queryURL)
			if err != nil || parsed.Path == "" {
				t.Fatalf("parse exemplar QueryUrl %q: %v", queryURL, err)
			}
			scope := strings.TrimPrefix(parsed.Path, "/searchapi/")

			localURL := server.URL + "/searchapi/" + scope
			if parsed.RawQuery != "" {
				localURL += "?" + parsed.RawQuery
			}
			resp, err := http.Get(localURL)
			if err != nil {
				t.Fatalf("get local %s: %v", localURL, err)
			}
			defer resp.Body.Close()
			var local map[string]any
			decodeErr := json.NewDecoder(resp.Body).Decode(&local)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("local %s returned %d: %#v", localURL, resp.StatusCode, local)
			}
			if decodeErr != nil {
				t.Fatalf("decode local response: %v", decodeErr)
			}

			exemplarSkeleton := map[string]string{}
			collectJSONSkeleton(exemplar, "", exemplarSkeleton, 6)
			localSkeleton := map[string]string{}
			collectJSONSkeleton(local, "", localSkeleton, 6)

			paths := make([]string, 0, len(exemplarSkeleton))
			for path := range exemplarSkeleton {
				paths = append(paths, path)
			}
			sort.Strings(paths)

			var missing []string
			var mismatched []string
			for _, path := range paths {
				if searchAPIParityTolerated(path) {
					continue
				}
				wantType := exemplarSkeleton[path]
				gotType, ok := localSkeleton[path]
				if !ok {
					// An array present locally but empty for this hit set
					// (e.g. a term with no local DefinitionDescription) does
					// not recurse into element fields; that is a data
					// artifact, not a shape mismatch. See searchAPIRow's
					// documented gaps for the fields omitted by design.
					if idx := strings.LastIndex(path, "[]"); idx > 0 && localSkeleton[path[:idx]] == "array" {
						continue
					}
					missing = append(missing, path)
					continue
				}
				if wantType == "null" || gotType == "null" {
					continue
				}
				if wantType != gotType {
					mismatched = append(mismatched, fmt.Sprintf("%s: exemplar=%s local=%s", path, wantType, gotType))
				}
			}
			if len(missing) > 0 {
				t.Errorf("keys present upstream but missing locally: %v", missing)
			}
			if len(mismatched) > 0 {
				t.Errorf("type mismatches: %v", mismatched)
			}
			ran++
		})
	}
	if ran == 0 {
		t.Skip("no *.json exemplars found under docs/exemplars/searchapi")
	}
}

// searchAPIParityTolerated lists exemplar paths this clone intentionally
// never produces, because the underlying data is not in the LOINC release
// (see searchAPI's doc comment in searchapi.go). Kept as an explicit,
// documented skip list rather than a blanket tolerance, so a genuine
// regression elsewhere still fails the test.
func searchAPIParityTolerated(path string) bool {
	tolerated := []string{
		// Not in the release data at all (always an empty local array).
		"Results[].CodeSystems", "Results[].CodeSystems[]",
		"Results[].Tags", "Results[].Tags[]",
		"FilterCounts.CodeSystems", "FilterCounts.CodeSystems[]",
		"FilterCounts.CodeSystems[].Label", "FilterCounts.CodeSystems[].Search", "FilterCounts.CodeSystems[].Count",
		"FilterCounts.Tags", "FilterCounts.Tags[]",
		"FilterCounts.Tags[].Label", "FilterCounts.Tags[].Search", "FilterCounts.Tags[].Count",
		// TermDescriptions only reproduces DefinitionDescription; the other
		// upstream fields (Url/Copyright/...) have no local source, and the
		// array itself is legitimately empty when the local hit's
		// DefinitionDescription is blank (a different LOINC number than the
		// 2.83 capture matched the same query against the 2.82 data).
		"Results[].TermDescriptions[]",
		"Results[].TermDescriptions[].Description", "Results[].TermDescriptions[].DescriptionHtml",
		"Results[].TermDescriptions[].Source", "Results[].TermDescriptions[].Sequence",
		"Results[].TermDescriptions[].Url", "Results[].TermDescriptions[].UrlDisplayText",
		"Results[].TermDescriptions[].Copyright",
		// Facet Description is optional even upstream (only set when a
		// friendlier PartDisplayName differs from the code); which facet
		// value happens to be first depends on the (different) matched data.
		"FilterCounts.System[].Description", "FilterCounts.Method[].Description",
		"FilterCounts.Property[].Description", "FilterCounts.Timing[].Description",
		"FilterCounts.Scale[].Description", "FilterCounts.Class[].Description",
	}
	for _, candidate := range tolerated {
		if candidate == path {
			return true
		}
	}
	return false
}

// collectJSONSkeleton walks a decoded JSON value and records, for every
// path, its JSON type ("object", "array", "string", "number", "bool",
// "null"). Arrays are recorded once at "path" and only their first element
// is recursed into at "path[]", since result-row array lengths are
// data-dependent and not part of the wire shape being compared.
func collectJSONSkeleton(value any, path string, out map[string]string, depth int) {
	switch v := value.(type) {
	case map[string]any:
		out[path] = "object"
		if depth <= 0 {
			return
		}
		for key, child := range v {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			collectJSONSkeleton(child, childPath, out, depth-1)
		}
	case []any:
		out[path] = "array"
		if depth <= 0 || len(v) == 0 {
			return
		}
		collectJSONSkeleton(v[0], path+"[]", out, depth-1)
	case string:
		out[path] = "string"
	case json.Number:
		out[path] = "number"
	case float64:
		out[path] = "number"
	case bool:
		out[path] = "bool"
	case nil:
		out[path] = "null"
	}
}
