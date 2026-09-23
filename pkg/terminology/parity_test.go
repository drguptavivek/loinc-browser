// Package terminology_test holds the exemplar shape-parity test. It is an external test package
// (not `package terminology`) specifically so it can import internal/fhirhttp, which itself
// imports pkg/terminology — importing fhirhttp from inside the terminology package would be an
// import cycle.
package terminology_test

import (
	"bufio"
	"encoding/json"
	"fmt"
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

// parityCase is one captured fhir.loinc.org exemplar to replay against the real local DB and
// compare in shape (never in value, since upstream serves 2.83 and the local DB serves 2.82;
// §9.2 of the plan).
type parityCase struct {
	file   string // basename under docs/exemplars/fhir.loinc.org, without extension
	method string
	path   string
}

// parityCases lists every P1 exemplar this test can reconstruct a request for. A few captured
// exemplars (the `property=`-filtered $lookup variants, and the version-mismatch case) carry no
// record of their exact request parameters — nothing in the exemplar or its .headers file states
// them — so they are not replayable and are intentionally left out here rather than guessed at;
// this is called out in the final report as a known gap, not a silent skip.
func parityCases() []parityCase {
	return []parityCase{
		{"codesystem-lookup-718-7", "GET", "/fhir/CodeSystem/$lookup?code=718-7"},
		{"codesystem-lookup-4544-3-props", "GET", "/fhir/CodeSystem/$lookup?code=4544-3&property=METHOD_TYP&property=VersionFirstReleased"},
		{"codesystem-lookup-LP14542-2-parent", "GET", "/fhir/CodeSystem/$lookup?code=LP14542-2&property=parent"},
		{"codesystem-lookup-30064-0-parent", "GET", "/fhir/CodeSystem/$lookup?code=30064-0&property=parent"},
		{"codesystem-lookup-LP31448-1-child", "GET", "/fhir/CodeSystem/$lookup?code=LP31448-1&property=child"},
		{"codesystem-lookup-LP31755-9", "GET", "/fhir/CodeSystem/$lookup?code=LP31755-9"},
		{"codesystem-lookup-LL1162-8", "GET", "/fhir/CodeSystem/$lookup?code=LL1162-8"},
		{"codesystem-lookup-LA6751-7", "GET", "/fhir/CodeSystem/$lookup?code=LA6751-7"},
		{"codesystem-lookup-deprecated-6796-7", "GET", "/fhir/CodeSystem/$lookup?code=6796-7"},
		{"codesystem-lookup-unknown", "GET", "/fhir/CodeSystem/$lookup?code=99999-9"},
		{"codesystem-validate-code-718-7", "GET", "/fhir/CodeSystem/$validate-code?code=718-7"},
		{"codesystem-validate-code-unknown", "GET", "/fhir/CodeSystem/$validate-code?code=99999-9"},
		{"codesystem-validate-code-deprecated-6796-7", "GET", "/fhir/CodeSystem/$validate-code?code=6796-7"},
		{"codesystem-subsumes-718-7_718-7", "GET", "/fhir/CodeSystem/$subsumes?codeA=718-7&codeB=718-7"},
		{"codesystem-subsumes-unknown", "GET", "/fhir/CodeSystem/$subsumes?codeA=99999-9&codeB=718-7"},
		{"codesystem-search-url", "GET", "/fhir/CodeSystem?url=http://loinc.org"},
		{"metadata", "GET", "/fhir/metadata"},
		{"metadata-terminology", "GET", "/fhir/metadata?mode=terminology"},
	}
}

// TestExemplarParity replays every reconstructable P1 exemplar against the real local DB and
// compares HTTP status, resourceType, and the collapsed ordered parameter-name skeleton (§9.2).
// It runs only when LOINC_TEST_DB points at a real database and docs/exemplars/fhir.loinc.org
// exists on disk; otherwise it is skipped with a clear reason.
func TestExemplarParity(t *testing.T) {
	dbPath := os.Getenv("LOINC_TEST_DB")
	if dbPath == "" {
		t.Skip("LOINC_TEST_DB is not set; skipping exemplar parity test")
	}
	exemplarDir := findExemplarDir(t)
	if exemplarDir == "" {
		t.Skip("docs/exemplars/fhir.loinc.org not found on disk; skipping exemplar parity test")
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

	for _, tc := range parityCases() {
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
			wantNames := collapsedParameterNames(wantBody)
			gotNames := collapsedParameterNames(gotBody)
			if !equalStrings(wantNames, gotNames) {
				t.Errorf("parameter name skeleton = %v, want %v", gotNames, wantNames)
			}

			if strings.HasPrefix(tc.file, "codesystem-lookup-") {
				assertNoDuplicateProperties(t, gotBody)
				assertSameCodingDisplays(t, wantBody, gotBody)
			}
			if isPropertyFilteredCase(tc.file) {
				wantSeq := propertyValueSequence(wantBody)
				gotSeq := propertyValueSequence(gotBody)
				if !equalStrings(wantSeq, gotSeq) {
					t.Errorf("ordered (name, property code, value[x]) sequence = %v, want %v", gotSeq, wantSeq)
				}
			}
		})
	}
}

// isPropertyFilteredCase reports whether tc.file is one of the property=-filtered $lookup
// exemplars, which must return no designations and only the requested properties, in the
// upstream order (bug #2, #5).
func isPropertyFilteredCase(file string) bool {
	switch file {
	case "codesystem-lookup-4544-3-props",
		"codesystem-lookup-LP14542-2-parent",
		"codesystem-lookup-30064-0-parent",
		"codesystem-lookup-LP31448-1-child":
		return true
	default:
		return false
	}
}

// propertyValueSequence returns the full ordered sequence of "name/property-code/value[x]" for
// every top-level parameter, uncollapsed, for the exact-order comparison a property=-filtered
// $lookup exemplar needs (bug #5).
func propertyValueSequence(body map[string]any) []string {
	params, _ := body["parameter"].([]any)
	seq := make([]string, 0, len(params))
	for _, p := range params {
		entry, _ := p.(map[string]any)
		name, _ := entry["name"].(string)
		parts, hasParts := entry["part"].([]any)
		if !hasParts {
			seq = append(seq, name+"/"+valueKind(entry))
			continue
		}
		var code string
		var valueKindStr string
		for _, part := range parts {
			partEntry, _ := part.(map[string]any)
			partName, _ := partEntry["name"].(string)
			switch partName {
			case "code":
				if c, ok := partEntry["valueCode"].(string); ok {
					code = c
				}
			case "value":
				valueKindStr = valueKind(partEntry)
			}
		}
		seq = append(seq, name+"/"+code+"/"+valueKindStr)
	}
	return seq
}

// valueKind returns the value[x] field name present on a Parameter/part map (e.g. "valueCoding",
// "valueString"), or "" when none is set.
func valueKind(entry map[string]any) string {
	for key := range entry {
		if strings.HasPrefix(key, "value") {
			return key
		}
	}
	return ""
}

// assertSameCodingDisplays compares valueCoding displays for every (property code, coding code)
// present in both responses. Shape parity alone missed that axis Codings used PartDisplayName
// ("Blood") where fhir.loinc.org uses PartName ("Bld"). Codes absent from either side (release
// drift 2.82 vs 2.83) are ignored.
func assertSameCodingDisplays(t *testing.T, want, got map[string]any) {
	t.Helper()
	displays := func(body map[string]any) map[string]string {
		out := map[string]string{}
		params, _ := body["parameter"].([]any)
		for _, p := range params {
			entry, _ := p.(map[string]any)
			if entry["name"] != "property" {
				continue
			}
			var prop, code, display string
			parts, _ := entry["part"].([]any)
			for _, part := range parts {
				partEntry, _ := part.(map[string]any)
				switch partEntry["name"] {
				case "code":
					prop, _ = partEntry["valueCode"].(string)
				case "value":
					if coding, ok := partEntry["valueCoding"].(map[string]any); ok {
						code, _ = coding["code"].(string)
						display, _ = coding["display"].(string)
					}
				}
			}
			// Part codings only: term long names legitimately drift between releases.
			if strings.HasPrefix(code, "LP") {
				out[prop+"|"+code] = display
			}
		}
		return out
	}
	gotDisplays := displays(got)
	for key, wantDisplay := range displays(want) {
		if gotDisplay, ok := gotDisplays[key]; ok && gotDisplay != wantDisplay {
			t.Errorf("coding %s display = %q, want %q", key, gotDisplay, wantDisplay)
		}
	}
}

// assertNoDuplicateProperties fails if a $lookup response emits the same (property code, value)
// pair twice (bug #3: DetailedModel supplementary links repeating a primary axis).
func assertNoDuplicateProperties(t *testing.T, body map[string]any) {
	t.Helper()
	params, _ := body["parameter"].([]any)
	seen := map[string]bool{}
	for _, p := range params {
		entry, _ := p.(map[string]any)
		if entry["name"] != "property" {
			continue
		}
		parts, _ := entry["part"].([]any)
		var code, value string
		for _, part := range parts {
			partEntry, _ := part.(map[string]any)
			switch partEntry["name"] {
			case "code":
				code, _ = partEntry["valueCode"].(string)
			case "value":
				value = fmt.Sprint(partEntry[valueKind(partEntry)])
			}
		}
		key := code + "|" + value
		if seen[key] {
			t.Errorf("duplicate property (code=%q, value=%q)", code, value)
		}
		seen[key] = true
	}
}

// findExemplarDir locates docs/exemplars/fhir.loinc.org relative to this package, walking up
// from the working directory (go test runs with cwd = the package directory).
func findExemplarDir(t *testing.T) string {
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

func readExemplar(t *testing.T, dir, file string) (int, map[string]any) {
	t.Helper()
	status := readExemplarStatus(t, filepath.Join(dir, file+".headers"))
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

func readExemplarStatus(t *testing.T, path string) int {
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

// collapsedParameterNames returns the ordered "parameter"/"issue" name skeleton of a decoded
// FHIR resource, collapsing consecutive repeats of the same name to one entry (§9.2: "collapse
// repeated designation/property runs to one label for comparison").
func collapsedParameterNames(body map[string]any) []string {
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

func equalStrings(a, b []string) bool {
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
