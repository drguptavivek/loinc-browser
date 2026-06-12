package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"loinc-browser/internal/loinc"
)

func TestAPI(t *testing.T) {
	ctx := context.Background()
	releaseDir := writeServerTestRelease(t)
	dbPath := filepath.Join(t.TempDir(), "loinc.sqlite")
	if _, err := loinc.Ingest(ctx, loinc.IngestOptions{ReleaseDir: releaseDir, DBPath: dbPath}); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	handler := New(Options{Store: store})
	server := httptest.NewServer(handler)
	defer server.Close()

	var health map[string]any
	getJSON(t, server.URL+"/api/health", &health)
	if health["ok"] != true {
		t.Fatalf("expected healthy response, got %#v", health)
	}

	var search loinc.SearchResponse
	getJSON(t, server.URL+"/api/search?q=cholesterol&status=ACTIVE", &search)
	if len(search.Results) != 1 || search.Results[0].LOINCNum != "2000-1" {
		t.Fatalf("expected cholesterol result, got %#v", search.Results)
	}

	var term loinc.Term
	getJSON(t, server.URL+"/api/terms/2000-1", &term)
	if term.LongCommonName != "Cholesterol [Mass/volume] in Serum" {
		t.Fatalf("unexpected term detail: %#v", term)
	}

	var facets loinc.Facets
	getJSON(t, server.URL+"/api/facets", &facets)
	if facets.Classes["CHEM"] != 1 || facets.Classes["HEM/BC"] != 1 {
		t.Fatalf("unexpected facets: %#v", facets.Classes)
	}

	var stats loinc.CacheStats
	getJSON(t, server.URL+"/api/cache", &stats)
	if stats.TermEntries == 0 || stats.FacetEntries == 0 {
		t.Fatalf("expected cache entries after term and facet calls, got %#v", stats)
	}

	notFound, err := http.Get(server.URL + "/api/terms/9999-9")
	if err != nil {
		t.Fatalf("get missing term: %v", err)
	}
	defer notFound.Body.Close()
	if notFound.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", notFound.StatusCode)
	}
}

func TestOpenAPISpec(t *testing.T) {
	handler := New(Options{})
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/openapi.json")
	if err != nil {
		t.Fatalf("get openapi spec: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if contentType := resp.Header.Get("content-type"); contentType != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content type %q", contentType)
	}

	var spec map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		t.Fatalf("decode openapi spec: %v", err)
	}
	if spec["openapi"] != "3.1.0" {
		t.Fatalf("expected OpenAPI 3.1.0, got %#v", spec["openapi"])
	}
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatalf("expected paths object, got %#v", spec["paths"])
	}
	for _, path := range []string{"/api/health", "/api/search", "/api/terms/{loincNum}", "/api/facets", "/api/cache", "/api/import/upload"} {
		if _, ok := paths[path]; !ok {
			t.Fatalf("expected OpenAPI path %s", path)
		}
	}
}

func TestUploadImportSwapsLiveStore(t *testing.T) {
	ctx := context.Background()
	initialRelease := writeServerTestRelease(t)
	dbPath := filepath.Join(t.TempDir(), "loinc.sqlite")
	if _, err := loinc.Ingest(ctx, loinc.IngestOptions{ReleaseDir: initialRelease, DBPath: dbPath}); err != nil {
		t.Fatalf("initial ingest: %v", err)
	}

	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open initial store: %v", err)
	}
	defer store.Close()

	handler := New(Options{
		Store:        store,
		DBPath:       dbPath,
		UploadDir:    filepath.Join(t.TempDir(), "uploads"),
		CacheEntries: 4,
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	body, contentType := multipartZipBody(t, "release.zip", testReleaseZip(t, "3000-1", "Uploaded glucose term"))
	resp, err := http.Post(server.URL+"/api/import/upload", contentType, body)
	if err != nil {
		t.Fatalf("post upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, responseBody)
	}

	var upload uploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&upload); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if upload.TermCount != 1 {
		t.Fatalf("expected uploaded term count 1, got %#v", upload)
	}

	var search loinc.SearchResponse
	getJSON(t, server.URL+"/api/search?q=uploaded", &search)
	if len(search.Results) != 1 || search.Results[0].LOINCNum != "3000-1" {
		t.Fatalf("expected uploaded release search result, got %#v", search.Results)
	}
}

func getJSON(t *testing.T, url string, target any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get %s returned %d", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
}

func multipartZipBody(t *testing.T, filename string, zipBytes []byte) (io.Reader, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("releaseZip", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(zipBytes); err != nil {
		t.Fatalf("write zip form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return &body, writer.FormDataContentType()
}

func testReleaseZip(t *testing.T, loincNum string, longName string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	file, err := zipWriter.Create("Loinc_Test/LoincTable/Loinc.csv")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	csvWriter := csv.NewWriter(file)
	header := []string{
		"LOINC_NUM", "COMPONENT", "PROPERTY", "TIME_ASPCT", "SYSTEM",
		"SCALE_TYP", "METHOD_TYP", "CLASS", "VersionLastChanged", "CHNG_TYPE",
		"DefinitionDescription", "STATUS", "CONSUMER_NAME", "CLASSTYPE",
		"FORMULA", "EXMPL_ANSWERS", "SURVEY_QUEST_TEXT", "SURVEY_QUEST_SRC",
		"UNITSREQUIRED", "RELATEDNAMES2", "SHORTNAME", "ORDER_OBS",
		"HL7_FIELD_SUBFIELD_ID", "EXTERNAL_COPYRIGHT_NOTICE", "EXAMPLE_UNITS",
		"LONG_COMMON_NAME", "EXAMPLE_UCUM_UNITS", "STATUS_REASON", "STATUS_TEXT",
		"CHANGE_REASON_PUBLIC", "COMMON_TEST_RANK", "COMMON_ORDER_RANK",
		"HL7_ATTACHMENT_STRUCTURE", "EXTERNAL_COPYRIGHT_LINK", "PanelType",
		"AskAtOrderEntry", "AssociatedObservations", "VersionFirstReleased",
		"ValidHL7AttachmentRequest", "DisplayName",
	}
	row := []string{
		loincNum, "Uploaded glucose", "MCnc", "Pt", "Serum", "Qn", "", "CHEM",
		"2.82", "ADD", "Uploaded test term", "ACTIVE",
		"Uploaded term", "1", "", "", "", "", "N", "uploaded glucose",
		"Upload Glucose", "Observation", "", "", "mg/dL", longName,
		"mg/dL", "", "", "", "1", "1", "", "", "", "", "", "2.82", "", "Uploaded glucose",
	}
	if err := csvWriter.Write(header); err != nil {
		t.Fatalf("write zip csv header: %v", err)
	}
	if err := csvWriter.Write(row); err != nil {
		t.Fatalf("write zip csv row: %v", err)
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		t.Fatalf("flush zip csv: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func writeServerTestRelease(t *testing.T) string {
	t.Helper()
	releaseDir := t.TempDir()
	tableDir := filepath.Join(releaseDir, "LoincTable")
	if err := os.MkdirAll(tableDir, 0o755); err != nil {
		t.Fatalf("mkdir release: %v", err)
	}
	file, err := os.Create(filepath.Join(tableDir, "Loinc.csv"))
	if err != nil {
		t.Fatalf("create csv: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	header := []string{
		"LOINC_NUM", "COMPONENT", "PROPERTY", "TIME_ASPCT", "SYSTEM",
		"SCALE_TYP", "METHOD_TYP", "CLASS", "VersionLastChanged", "CHNG_TYPE",
		"DefinitionDescription", "STATUS", "CONSUMER_NAME", "CLASSTYPE",
		"FORMULA", "EXMPL_ANSWERS", "SURVEY_QUEST_TEXT", "SURVEY_QUEST_SRC",
		"UNITSREQUIRED", "RELATEDNAMES2", "SHORTNAME", "ORDER_OBS",
		"HL7_FIELD_SUBFIELD_ID", "EXTERNAL_COPYRIGHT_NOTICE", "EXAMPLE_UNITS",
		"LONG_COMMON_NAME", "EXAMPLE_UCUM_UNITS", "STATUS_REASON", "STATUS_TEXT",
		"CHANGE_REASON_PUBLIC", "COMMON_TEST_RANK", "COMMON_ORDER_RANK",
		"HL7_ATTACHMENT_STRUCTURE", "EXTERNAL_COPYRIGHT_LINK", "PanelType",
		"AskAtOrderEntry", "AssociatedObservations", "VersionFirstReleased",
		"ValidHL7AttachmentRequest", "DisplayName",
	}
	rows := [][]string{
		{
			"2000-1", "Cholesterol", "MCnc", "Pt", "Serum", "Qn", "", "CHEM",
			"2.80", "ADD", "Cholesterol mass concentration in serum", "ACTIVE",
			"Cholesterol", "1", "", "", "", "", "N", "lipid; cholesterol serum",
			"Chol Ser", "Observation", "", "", "mg/dL", "Cholesterol [Mass/volume] in Serum",
			"mg/dL", "", "", "", "100", "0", "", "", "", "", "", "2.80", "", "Cholesterol Serum",
		},
		{
			"2001-9", "Platelets", "NCnc", "Pt", "Blood", "Qn", "", "HEM/BC",
			"2.80", "ADD", "Platelet count in blood", "ACTIVE",
			"Platelets", "1", "", "", "", "", "N", "plt",
			"Platelets Bld", "Observation", "", "", "10*3/uL", "Platelets [#/volume] in Blood",
			"10*3/uL", "", "", "", "80", "0", "", "", "", "", "", "2.80", "", "Platelets Blood",
		},
	}
	if err := writer.Write(header); err != nil {
		t.Fatalf("write header: %v", err)
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			t.Fatalf("write row: %v", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatalf("flush csv: %v", err)
	}
	return releaseDir
}
