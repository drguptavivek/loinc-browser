// Package terminology_test holds the ConceptMap/Questionnaire exemplar shape-parity test, kept
// separate from parity_test.go (owned by P1) to avoid editing a file another phase might also be
// extending concurrently. It duplicates parity_test.go's small exemplar-reading helpers rather
// than importing them (they are unexported).
package terminology_test

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"loinc-browser/internal/fhirhttp"
	"loinc-browser/internal/loinc"
	"loinc-browser/pkg/terminology"
)

// cmqCase is one captured fhir.loinc.org ConceptMap/Questionnaire exemplar to replay against the
// real local DB (§9.2). Values are ignored except system/name, since upstream serves 2.83 and the
// local DB serves 2.82; only shape (status, resourceType, parameter-name skeleton) is compared.
type cmqCase struct {
	file   string
	method string
	path   string
}

// cmqCases lists every P3 exemplar this test can reconstruct a request for.
//
// Skipped, per §7/§0.1 (documented divergences, not gaps):
//   - conceptmap-translate-mapto-deprecated.json: captured content is a loinc-to-phenx match, a
//     map we do not serve (no release data, §4.9); not reconstructable as a same-shape local call.
//   - conceptmap-translate-reverse-flag / -reverse-ieee / -snomed-reverse: upstream 404s a
//     reverse $translate; ours succeeds by design (§7 "maps are published as bidirectional").
//     Covered instead by TestTranslateReverseFlagEquivalentToReverseMap in conceptmap_test.go.
//   - see cmqSkipTable for conceptmap-search-loinc-to-ieee.json (§4.13).
func cmqCases() []cmqCase {
	return []cmqCase{
		{"conceptmap-search-url-loinc-to-ieee", "GET", "/fhir/ConceptMap?url=http://loinc.org/cm/loinc-to-ieee-11073-10101"},
		{"conceptmap-translate-11556-8", "GET", "/fhir/ConceptMap/$translate?system=http://loinc.org&code=11556-8"},
		{"conceptmap-translate-30657-1", "GET", "/fhir/ConceptMap/$translate?system=http://loinc.org&code=30657-1"},
		{"conceptmap-translate-parts-snomed-LP100006-8", "GET", "/fhir/ConceptMap/$translate?url=http://loinc.org/cm/loinc-parts-to-snomed-ct&code=LP100006-8"},
		{"questionnaire-89689-4", "GET", "/fhir/Questionnaire/89689-4"},
		{"questionnaire-search-url-89689-4", "GET", "/fhir/Questionnaire?url=http://loinc.org/q/89689-4"},
	}
}

// cmqSkipTable documents §7-style divergences for ConceptMap/Questionnaire exemplars this test
// does not assert parity for, with why (mirrors valueSetSkipTable in valueset_parity_test.go).
var cmqSkipTable = map[string]string{
	"conceptmap-search-loinc-to-ieee": "captured with &_summary=true; upstream 400s _summary as not-supported, while we now support it (plan §4.13, §7 \"_summary / _elements\")",
}

func TestConceptMapQuestionnaireExemplarParity(t *testing.T) {
	dbPath := os.Getenv("LOINC_TEST_DB")
	if dbPath == "" {
		t.Skip("LOINC_TEST_DB is not set; skipping exemplar parity test")
	}
	exemplarDir := findCMQExemplarDir(t)
	if exemplarDir == "" {
		t.Skip("docs/exemplars/fhir.loinc.org not found on disk; skipping exemplar parity test")
	}
	for file, reason := range cmqSkipTable {
		t.Run(file+"_(skipped)", func(t *testing.T) {
			t.Skip(reason)
		})
	}

	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open %s: %v", dbPath, err)
	}
	defer store.Close()
	svc := terminology.NewService(func() (*loinc.Store, error) { return store, nil })
	mux := http.NewServeMux()
	fhirhttp.Register(mux, svc)
	server := httptest.NewServer(mux)
	defer server.Close()

	for _, tc := range cmqCases() {
		t.Run(tc.file, func(t *testing.T) {
			wantStatus, wantBody := readCMQExemplar(t, exemplarDir, tc.file)
			req, err := http.NewRequest(tc.method, server.URL+tc.path, nil)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("do request: %v", err)
			}
			defer resp.Body.Close()
			var gotBody map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if resp.StatusCode != wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, wantStatus)
			}
			if gotBody["resourceType"] != wantBody["resourceType"] {
				t.Errorf("resourceType = %v, want %v", gotBody["resourceType"], wantBody["resourceType"])
			}
			wantNames := cmqCollapsedParameterNames(wantBody)
			gotNames := cmqCollapsedParameterNames(gotBody)
			if !cmqEqualStrings(wantNames, gotNames) {
				t.Errorf("parameter name skeleton = %v, want %v", gotNames, wantNames)
			}
		})
	}
}

func findCMQExemplarDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(dir, "docs", "exemplars", "fhir.loinc.org")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

func readCMQExemplar(t *testing.T, dir, file string) (int, map[string]any) {
	t.Helper()
	status := readCMQExemplarStatus(t, filepath.Join(dir, file+".headers"))
	data, err := os.ReadFile(filepath.Join(dir, file+".json"))
	if err != nil {
		t.Fatalf("read exemplar %s.json: %v", file, err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("parse exemplar %s.json: %v", file, err)
	}
	return status, body
}

func readCMQExemplarStatus(t *testing.T, path string) int {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatalf("empty headers file %s", path)
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 2 {
		t.Fatalf("unexpected status line %q in %s", scanner.Text(), path)
	}
	status, err := strconv.Atoi(fields[1])
	if err != nil {
		t.Fatalf("parse status from %q: %v", scanner.Text(), err)
	}
	return status
}

func cmqCollapsedParameterNames(body map[string]any) []string {
	var raw []string
	switch {
	case body["parameter"] != nil:
		params, _ := body["parameter"].([]any)
		for _, p := range params {
			entry, _ := p.(map[string]any)
			name, _ := entry["name"].(string)
			raw = append(raw, name)
		}
	case body["issue"] != nil:
		issues, _ := body["issue"].([]any)
		for _, i := range issues {
			entry, _ := i.(map[string]any)
			code, _ := entry["code"].(string)
			raw = append(raw, code)
		}
	}
	var collapsed []string
	for _, name := range raw {
		if len(collapsed) > 0 && collapsed[len(collapsed)-1] == name {
			continue
		}
		collapsed = append(collapsed, name)
	}
	return collapsed
}

func cmqEqualStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
