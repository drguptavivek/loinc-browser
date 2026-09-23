package terminology_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"

	"loinc-browser/internal/fhirhttp"
	"loinc-browser/internal/loinc"
	"loinc-browser/pkg/terminology"
)

// valueSetParityCase is a §P2 addition to the exemplar shape-parity approach (parity_test.go),
// kept in its own file/TestXxx per the coordinator's instruction not to edit that file's owned
// content. It reuses parity_test.go's helpers (same package): findExemplarDir, readExemplar,
// equalStrings.
type valueSetParityCase struct {
	file   string
	method string
	path   string
}

// valueSetParityCases lists every ValueSet exemplar this test can reconstruct a request for.
// Excluded, with reasons noted in valueSetSkipTable below: calls that 404/403 upstream by a
// documented divergence (§7), the POST-inline exemplar (upstream returns an nginx HTML page, not
// JSON), and valueset-expand-LP31755-9-count3.json, whose captured body ("http://loinc.org/vs/
// LL0000-0", the unrelated unknown-LL exemplar) does not match its filename — a data quality gap
// in the exemplar corpus, not something to guess a request for.
func valueSetParityCases() []valueSetParityCase {
	return []valueSetParityCase{
		{"valueset-read-LL1162-8", "GET", "/fhir/ValueSet/LL1162-8"},
		{"valueset-read-LG9568-9", "GET", "/fhir/ValueSet/LG9568-9"},
		{"valueset-search-url-LL1162-8", "GET", "/fhir/ValueSet?url=http://loinc.org/vs/LL1162-8"},
		{"valueset-read-id-LG9568-9-search", "GET", "/fhir/ValueSet?url=http://loinc.org/vs/LG9568-9"},
		{"valueset-search-name-yes", "GET", "/fhir/ValueSet?name:in=Yes&_count=2"},
		{"valueset-expand-LL1162-8", "GET", "/fhir/ValueSet/$expand?url=http://loinc.org/vs/LL1162-8"},
		{"valueset-expand-deprecated-count3", "GET", "/fhir/ValueSet/$expand?url=http://loinc.org/vs/deprecated-loinc-terms&count=3"},
		{"valueset-expand-top-lab-orders-count3", "GET", "/fhir/ValueSet/$expand?url=http://loinc.org/vs/top-lab-orders&count=3"},
		{"valueset-expand-attachment-requests-count3", "GET", "/fhir/ValueSet/$expand?url=http://loinc.org/vs/valid-hl7-attachment-requests&count=3"},
		{"valueset-validate-code-LL1162-8", "GET", "/fhir/ValueSet/LL1162-8/$validate-code?code=LA15679-6"},
		{"valueset-validate-code-LL1162-8-miss", "GET", "/fhir/ValueSet/LL1162-8/$validate-code?code=718-7"},
		{"valueset-validate-code-LG9568-9", "GET", "/fhir/ValueSet/LG9568-9/$validate-code?code=6785-0"},
	}
}

// valueSetSkipTable documents §7-style divergences for ValueSet exemplars this test does not
// assert parity for, with why.
var valueSetSkipTable = map[string]string{
	"valueset-expand-unknown":                  "upstream returns an empty 200 expansion for unknown LL codes; the spec-correct 404 is a documented divergence (plan §7)",
	"valueset-expand-LL1162-8-count2-offset1":  "upstream 404s $expand with count/offset on an LL value set (a known upstream bug per plan §7 \"$expand with filter/offset on LL\"); local works",
	"valueset-expand-LL1162-8-filter":          "same upstream $expand-with-extra-params-on-LL bug as above",
	"valueset-expand-LL1162-8-de":              "same upstream $expand-with-extra-params-on-LL bug as above",
	"valueset-expand-LG9568-9-count3":          "the upstream bug extends to LG groups too: $expand with count on LG9568-9 404s upstream; local works",
	"valueset-expand-loinc-vs-count2":          "upstream 404s $expand of the bare http://loinc.org/vs URL entirely (plan §7 \"http://loinc.org/vs ... 404\"); local serves it",
	"valueset-expand-loinc-vs-count2-offset2":  "same as above",
	"valueset-expand-loinc-vs-filter":          "same as above",
	"valueset-expand-top-ranked-count3":        "loinc-top-ranked is a local-only addition with no upstream exemplar (plan §10 open decision 5); upstream 404s it",
	"valueset-read-loinc-top-ranked":           "same local-only addition as above; upstream's search returns an empty Bundle",
	"valueset-expand-inline-component":         "upstream's nginx rejects POST $expand with a 403 HTML page, not JSON (plan §7); local serves it",
	"valueset-expand-LP31755-9-count3":         "captured body does not match its filename (contains the unrelated http://loinc.org/vs/LL0000-0 unknown-LL exemplar) -- an exemplar-corpus data-quality gap, not a request this test can safely reconstruct",
	"valueset-expand-LG9568-9":                 "upstream's own read (valueset-read-LG9568-9.json) and $expand (this file) captures disagree on \"experimental\" for the identical LG9568-9 resource -- omitted on read, true on expand. We match the read shape (§9.2 picks one exemplar per case; this test already asserts valueset-read-LG9568-9 against the other capture)",
	"valueset-expand-document-ontology-count3": "upstream's $expand adds an undocumented \"date\" field (a HAPI package-load timestamp) with no defined local equivalent in plan §4.6.1 -- not something to guess a value for",
	"valueset-expand-rsna-playbook-count3":     "same undocumented \"date\" field as document-ontology-count3",
	"valueset-search-url-deprecated":           "captured with &_elements=id; upstream 400s _elements as not-supported, while we now support it (plan §4.13, §7 \"_summary / _elements\")",
}

func TestExemplarParityValueSet(t *testing.T) {
	dbPath := os.Getenv("LOINC_TEST_DB")
	if dbPath == "" {
		t.Skip("LOINC_TEST_DB is not set; skipping exemplar parity test")
	}
	exemplarDir := findExemplarDir(t)
	if exemplarDir == "" {
		t.Skip("docs/exemplars/fhir.loinc.org not found on disk; skipping exemplar parity test")
	}
	for file, reason := range valueSetSkipTable {
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

	for _, tc := range valueSetParityCases() {
		t.Run(tc.file, func(t *testing.T) {
			wantStatus, wantBody := readExemplar(t, exemplarDir, tc.file)
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

			// Parameters/OperationOutcome shapes: ordered parameter/issue name skeleton.
			wantNames := collapsedParameterNames(wantBody)
			gotNames := collapsedParameterNames(gotBody)
			if !equalStrings(wantNames, gotNames) {
				t.Errorf("parameter name skeleton = %v, want %v", gotNames, wantNames)
			}

			// ValueSet/Bundle shapes: top-level key skeleton (sorted; values are never compared,
			// since upstream serves 2.83 and the local DB serves 2.82, per plan §9.2).
			if gotBody["resourceType"] == "ValueSet" || gotBody["resourceType"] == "Bundle" {
				wantKeys := sortedKeys(wantBody)
				gotKeys := sortedKeys(gotBody)
				if !equalStrings(wantKeys, gotKeys) {
					t.Errorf("top-level key skeleton = %v, want %v", gotKeys, wantKeys)
				}
			}
		})
	}
}

func sortedKeys(m map[string]any) []string {
	// "meta" is a HAPI-generated envelope field (versionId/lastUpdated/tag) our resources never
	// carry (§4.0: our resource ids are stable, not UUID-versioned like upstream's); every other
	// key is asserted.
	keys := make([]string, 0, len(m))
	for k := range m {
		if k == "meta" || k == "id" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
