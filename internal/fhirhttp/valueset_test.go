package fhirhttp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"loinc-browser/internal/loinc"
	"loinc-browser/pkg/terminology"
)

// writeValueSetHTTPFixture extends writeFHIRHTTPTestRelease with one answer list, so ValueSet
// routes have an LL id to exercise end-to-end over HTTP.
func writeValueSetHTTPFixture(t *testing.T) string {
	t.Helper()
	releaseDir := writeFHIRHTTPTestRelease(t)
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "AnswerFile", "AnswerList.csv"),
		[]string{"AnswerListId", "AnswerListName", "AnswerListOID", "ExtDefinedYN", "ExtDefinedAnswerListCodeSystem", "ExtDefinedAnswerListLink", "AnswerStringId", "LocalAnswerCode", "LocalAnswerCodeSystem", "SequenceNumber", "DisplayText", "ExtCodeId", "ExtCodeDisplayName", "ExtCodeSystem", "ExtCodeSystemVersion", "ExtCodeSystemCopyrightNotice", "SubsequentTextPrompt", "Description", "Score"},
		[][]string{
			{"LL1000-1", "Positive negative", "1.2.3.4", "N", "", "", "LA1-1", "", "", "1", "Positive", "", "", "", "", "", "", "", "1"},
			{"LL1000-1", "Positive negative", "1.2.3.4", "N", "", "", "LA2-2", "", "", "2", "Negative", "", "", "", "", "", "", "", "0"},
		})
	return releaseDir
}

func newValueSetTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ctx := context.Background()
	releaseDir := writeValueSetHTTPFixture(t)
	dbPath := filepath.Join(t.TempDir(), "loinc.sqlite")
	if _, err := loinc.Ingest(ctx, loinc.IngestOptions{ReleaseDir: releaseDir, DBPath: dbPath}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	svc := terminology.NewService(func() (*loinc.Store, error) { return store, nil })
	mux := http.NewServeMux()
	Register(mux, svc)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func getJSON(t *testing.T, url string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, decodeParameters(t, data)
}

func TestValueSetReadByID(t *testing.T) {
	server := newValueSetTestServer(t)
	status, body := getJSON(t, server.URL+"/fhir/ValueSet/LL1000-1")
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %v", status, body)
	}
	if body["resourceType"] != "ValueSet" || body["id"] != "LL1000-1" {
		t.Errorf("body = %v", body)
	}
}

func TestValueSetReadUnknown404(t *testing.T) {
	server := newValueSetTestServer(t)
	status, body := getJSON(t, server.URL+"/fhir/ValueSet/LL9999-9")
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, body = %v", status, body)
	}
	if body["resourceType"] != "OperationOutcome" {
		t.Errorf("resourceType = %v", body["resourceType"])
	}
}

func TestValueSetSearchByURL(t *testing.T) {
	server := newValueSetTestServer(t)
	status, body := getJSON(t, server.URL+"/fhir/ValueSet?url=http://loinc.org/vs/LL1000-1")
	if status != http.StatusOK || body["resourceType"] != "Bundle" {
		t.Fatalf("status=%d body=%v", status, body)
	}
	if body["total"].(float64) != 1 {
		t.Errorf("total = %v", body["total"])
	}
	links, _ := body["link"].([]any)
	if len(links) == 0 {
		t.Errorf("expected link[self]")
	}
}

func TestValueSetExpandGET(t *testing.T) {
	server := newValueSetTestServer(t)
	status, body := getJSON(t, server.URL+"/fhir/ValueSet/LL1000-1/$expand")
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %v", status, body)
	}
	expansion := body["expansion"].(map[string]any)
	if expansion["total"].(float64) != 2 {
		t.Errorf("total = %v", expansion["total"])
	}
	contains := expansion["contains"].([]any)
	if len(contains) != 2 {
		t.Fatalf("contains = %v", contains)
	}
	first := contains[0].(map[string]any)
	if first["code"] != "LA1-1" || first["display"] != "Positive" {
		t.Errorf("first = %v", first)
	}
}

func TestValueSetExpandUnknownReturns404(t *testing.T) {
	server := newValueSetTestServer(t)
	status, body := getJSON(t, server.URL+"/fhir/ValueSet/$expand?url=http://loinc.org/vs/LL9999-9")
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, body = %v", status, body)
	}
	if body["resourceType"] != "OperationOutcome" {
		t.Errorf("resourceType = %v", body)
	}
}

func TestValueSetExpandNegativeCountInvalid(t *testing.T) {
	server := newValueSetTestServer(t)
	status, body := getJSON(t, server.URL+"/fhir/ValueSet/$expand?url=http://loinc.org/vs&count=-5")
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %v", status, body)
	}
	issue := body["issue"].([]any)[0].(map[string]any)
	if issue["code"] != "invalid" {
		t.Errorf("code = %v", issue["code"])
	}
}

func TestValueSetValidateCodeGET(t *testing.T) {
	server := newValueSetTestServer(t)
	status, body := getJSON(t, server.URL+"/fhir/ValueSet/LL1000-1/$validate-code?code=LA1-1")
	if status != http.StatusOK {
		t.Fatalf("status = %d, body = %v", status, body)
	}
	params := body["parameter"].([]any)
	first := params[0].(map[string]any)
	if first["name"] != "result" || first["valueBoolean"] != true {
		t.Errorf("first param = %v", first)
	}
}

func TestValueSetExpandPOSTInlineCompose(t *testing.T) {
	server := newValueSetTestServer(t)
	payload := `{
		"resourceType": "Parameters",
		"parameter": [ {
			"name": "valueSet",
			"resource": {
				"resourceType": "ValueSet",
				"compose": {
					"include": [ { "system": "http://loinc.org", "concept": [ {"code": "10000-1"} ] } ]
				}
			}
		} ]
	}`
	resp, err := http.Post(server.URL+"/fhir/ValueSet/$expand", "application/fhir+json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %v", resp.StatusCode, body)
	}
	expansion := body["expansion"].(map[string]any)
	if expansion["total"].(float64) != 1 {
		t.Errorf("total = %v", expansion["total"])
	}
	contains := expansion["contains"].([]any)[0].(map[string]any)
	if contains["code"] != "10000-1" {
		t.Errorf("contains = %v", contains)
	}
}
