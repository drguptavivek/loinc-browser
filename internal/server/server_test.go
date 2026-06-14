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
	"strings"
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

func TestV1API(t *testing.T) {
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

	server := httptest.NewServer(New(Options{Store: store}))
	defer server.Close()

	var health map[string]any
	getJSON(t, server.URL+"/api/v1/health", &health)
	if health["ok"] != true {
		t.Fatalf("expected v1 healthy response, got %#v", health)
	}

	var search loinc.SearchResponse
	getJSON(t, server.URL+"/api/v1/terms/search?q=cholesterol&usageType=observation&rankMode=observation&sort=relevance", &search)
	if len(search.Results) != 1 || search.Results[0].LOINCNum != "2000-1" {
		t.Fatalf("expected v1 cholesterol result, got %#v", search.Results)
	}
	if search.Results[0].CommonOrderRank != 0 || len(search.Results[0].UsageTypes) == 0 || search.Results[0].Links["self"] == "" {
		t.Fatalf("expected v1 summary metadata and links, got %#v", search.Results[0])
	}
	if !search.HasMore && search.Links["self"] == "" {
		t.Fatalf("expected v1 page links, got %#v", search.Links)
	}

	var top loinc.SearchResponse
	getJSON(t, server.URL+"/api/v1/terms/top?rankMode=observation&limit=1", &top)
	if len(top.Results) != 1 || top.Results[0].LOINCNum != "2001-9" {
		t.Fatalf("expected top observation term, got %#v", top.Results)
	}

	var term loinc.Term
	getJSON(t, server.URL+"/api/v1/terms/2000-1", &term)
	if term.LOINCNum != "2000-1" || term.Links["relationships"] == "" {
		t.Fatalf("unexpected v1 term detail: %#v", term)
	}

	var fit loinc.TermFit
	getJSON(t, server.URL+"/api/v1/terms/2000-1/fit", &fit)
	if fit.LOINCNum != "2000-1" || fit.Status != "ACTIVE" || fit.Deprecated {
		t.Fatalf("unexpected v1 term fit: %#v", fit)
	}

	var legacyInclude loinc.Term
	getJSON(t, server.URL+"/api/terms/2000-1?include=relationships", &legacyInclude)
	if len(legacyInclude.Parts) != 0 || len(legacyInclude.AnswerLists) != 0 || len(legacyInclude.Hierarchy) != 0 {
		t.Fatalf("old include=relationships should not nest relationship payloads, got %#v", legacyInclude)
	}

	var answerLists loinc.Page[loinc.AnswerList]
	getJSON(t, server.URL+"/api/v1/answer-lists/search?q=positive", &answerLists)
	if answerLists.Links.Self == "" {
		t.Fatalf("expected answer list page links, got %#v", answerLists)
	}
}

func TestMCPRouteCanBeEnabledWithoutBreakingAPI(t *testing.T) {
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

	docsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(docsDir, "LOINC_CONCEPTS.md"), []byte("# Concepts\n"), 0o644); err != nil {
		t.Fatalf("write concepts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "LOINC_AGENT_GUIDE.md"), []byte("# Guide\n"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "LOINC_LICENSE_NOTE.md"), []byte("# License\n"), 0o644); err != nil {
		t.Fatalf("write license: %v", err)
	}

	testServer := httptest.NewServer(New(Options{Store: store, EnableMCP: true, MCPPath: "/mcp", DocsDir: docsDir}))
	defer testServer.Close()

	var health map[string]any
	getJSON(t, testServer.URL+"/api/v1/health", &health)
	if health["ok"] != true {
		t.Fatalf("expected v1 health with mcp enabled, got %#v", health)
	}

	resp, err := http.Post(testServer.URL+"/mcp", "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`))
	if err != nil {
		t.Fatalf("post mcp initialize: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 500 {
		t.Fatalf("expected MCP route to handle request, got status %d", resp.StatusCode)
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
	for _, path := range v1OpenAPIPaths() {
		if _, ok := paths[path]; !ok {
			t.Fatalf("expected OpenAPI path %s", path)
		}
	}
	if _, ok := paths["/api/accessories"]; ok {
		t.Fatalf("OpenAPI v1 spec should not document old /api/accessories path")
	}
	requireOpenAPIQueryParams(t, paths, "/api/v1/terms/search", "q", "status", "usageType", "rankMode", "sort", "rankedOnly", "hierarchyNodeId", "limit", "offset")
	requireOpenAPIPathParams(t, paths, "/api/v1/terms/{loincNum}/answer-lists", "loincNum")
	requireOpenAPIPathParams(t, paths, "/api/v1/terms/{loincNum}/panel-memberships", "loincNum")
	requireOpenAPIPathParams(t, paths, "/api/v1/hierarchy/nodes/{nodeId}/parents", "nodeId")
	requireOpenAPIPathParams(t, paths, "/api/v1/answer-lists/{answerListId}/terms", "answerListId")
	requireOpenAPIPathParams(t, paths, "/api/v1/parts/{partNumber}/terms", "partNumber")
	requireOpenAPIPathParams(t, paths, "/api/v1/groups/{groupId}/terms", "groupId")

	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
	searchResponse := schemas["SearchResponse"].(map[string]any)["properties"].(map[string]any)
	if _, ok := searchResponse["_links"]; !ok {
		t.Fatalf("expected SearchResponse schema to include _links")
	}
	requireOpenAPISchemaProperties(t, schemas, "AccessoryBrowseResponse", "results", "total", "limit", "offset", "hasMore", "query", "kind", "_links")
	requireOpenAPISchemaProperties(t, schemas, "HierarchyChildrenResponse", "parentNodeId", "parentCode", "query", "results", "_links")
	requireOpenAPISchemaProperties(t, schemas, "PanelItem", "parentLoincNum", "childLoincNum", "sequence", "itemId", "displayNameForForm", "observationRequired", "entryType", "dataTypeInForm", "answerListIdOverride", "childTerm", "_links")
	requireOpenAPISchemaProperties(t, schemas, "TermAccessoryPage", "results", "total", "limit", "offset", "hasMore", "_links")
}

func TestSwaggerUIDocs(t *testing.T) {
	server := httptest.NewServer(New(Options{}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/docs")
	if err != nil {
		t.Fatalf("get swagger docs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if contentType := resp.Header.Get("content-type"); contentType != "text/html; charset=utf-8" {
		t.Fatalf("unexpected content type %q", contentType)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read swagger docs body: %v", err)
	}
	page := string(body)
	for _, phrase := range []string{"SwaggerUIBundle", "/openapi.json", "LOINC Browser API"} {
		if !strings.Contains(page, phrase) {
			t.Fatalf("expected swagger docs page to contain %q", phrase)
		}
	}
}

func TestAPIDocumentationCoversV1Routes(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "API.md"))
	if err != nil {
		t.Fatalf("read API documentation: %v", err)
	}
	documentation := string(body)
	for _, path := range v1OpenAPIPaths() {
		if !strings.Contains(documentation, path) {
			t.Fatalf("expected docs/API.md to mention %s", path)
		}
	}
	for _, phrase := range []string{
		"Term list defaults",
		"HATEOAS",
		"status=*",
		"usageType",
		"rankMode",
		"hierarchyNodeId",
		"EMR form-builder workflows",
	} {
		if !strings.Contains(documentation, phrase) {
			t.Fatalf("expected docs/API.md to mention %q", phrase)
		}
	}
}

func TestFrontendGlobalFooterLinks(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "App.svelte"))
	if err != nil {
		t.Fatalf("read frontend app: %v", err)
	}
	source := string(body)
	for _, phrase := range []string{
		"<footer",
		"href=\"/api/docs\"",
		"Swagger API",
		"LOINC license",
		"CC BY attribution",
		"GitHub",
	} {
		if !strings.Contains(source, phrase) {
			t.Fatalf("expected global footer source to contain %q", phrase)
		}
	}
}

func TestFrontendTabletBrowseDrawer(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "App.svelte"))
	if err != nil {
		t.Fatalf("read frontend app: %v", err)
	}
	cssBody, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "app.css"))
	if err != nil {
		t.Fatalf("read frontend css: %v", err)
	}
	source := string(body)
	for _, phrase := range []string{
		"browseDrawerOpen",
		"ariaLabel=\"Open browse drawer\"",
		"aria-label=\"Close browse drawer\"",
		"data-testid=\"browse-drawer-backdrop\"",
		"-translate-x-full",
		"translate-x-0",
		"lg:static",
		"lg:translate-x-0",
	} {
		if !strings.Contains(source, phrase) {
			t.Fatalf("expected tablet browse drawer source to contain %q", phrase)
		}
	}
	css := string(cssBody)
	for _, phrase := range []string{
		"width: min(75vw, 88vw)",
		"max-width: 88vw",
		"width: min(50vw, calc(100vw - 2rem))",
		"max-width: calc(100vw - 2rem)",
		"[data-loinc-shell-pane-group]",
		"width: 100%",
	} {
		if !strings.Contains(css, phrase) {
			t.Fatalf("expected tablet browse drawer CSS to contain %q", phrase)
		}
	}
}

func v1OpenAPIPaths() []string {
	return []string{
		"/api/v1/health",
		"/api/v1/terms/search",
		"/api/v1/terms/top",
		"/api/v1/terms/{loincNum}",
		"/api/v1/terms/{loincNum}/fit",
		"/api/v1/terms/{loincNum}/relationships",
		"/api/v1/terms/{loincNum}/answer-lists",
		"/api/v1/terms/{loincNum}/panel-memberships",
		"/api/v1/terms/{loincNum}/copyright",
		"/api/v1/hierarchy/roots",
		"/api/v1/hierarchy/nodes/{nodeId}",
		"/api/v1/hierarchy/nodes/{nodeId}/parents",
		"/api/v1/hierarchy/nodes/{nodeId}/children",
		"/api/v1/hierarchy/nodes/{nodeId}/terms",
		"/api/v1/panels/search",
		"/api/v1/panels/{loincNum}",
		"/api/v1/panels/{loincNum}/items",
		"/api/v1/answer-lists/search",
		"/api/v1/answer-lists/{answerListId}",
		"/api/v1/answer-lists/{answerListId}/answers",
		"/api/v1/answer-lists/{answerListId}/terms",
		"/api/v1/parts/search",
		"/api/v1/parts/{partNumber}",
		"/api/v1/parts/{partNumber}/terms",
		"/api/v1/groups/search",
		"/api/v1/groups/{groupId}",
		"/api/v1/groups/{groupId}/terms",
		"/api/v1/source-organizations",
		"/api/v1/source-organizations/{id}",
		"/api/v1/accessories",
	}
}

func requireOpenAPIQueryParams(t *testing.T, paths map[string]any, path string, names ...string) {
	t.Helper()
	params := openAPIParameters(t, paths, path)
	for _, name := range names {
		if !hasOpenAPIParam(params, name, "query") {
			t.Fatalf("expected OpenAPI path %s to document query parameter %s", path, name)
		}
	}
}

func requireOpenAPIPathParams(t *testing.T, paths map[string]any, path string, names ...string) {
	t.Helper()
	params := openAPIParameters(t, paths, path)
	for _, name := range names {
		if !hasOpenAPIParam(params, name, "path") {
			t.Fatalf("expected OpenAPI path %s to document path parameter %s", path, name)
		}
	}
}

func openAPIParameters(t *testing.T, paths map[string]any, path string) []any {
	t.Helper()
	pathSpec, ok := paths[path].(map[string]any)
	if !ok {
		t.Fatalf("expected OpenAPI path %s", path)
	}
	getSpec, ok := pathSpec["get"].(map[string]any)
	if !ok {
		t.Fatalf("expected OpenAPI GET operation for %s", path)
	}
	params, ok := getSpec["parameters"].([]any)
	if !ok || len(params) == 0 {
		t.Fatalf("expected OpenAPI parameters for %s", path)
	}
	return params
}

func hasOpenAPIParam(params []any, name string, in string) bool {
	for _, raw := range params {
		param, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if param["name"] == name && param["in"] == in {
			return true
		}
	}
	return false
}

func requireOpenAPISchemaProperties(t *testing.T, schemas map[string]any, schemaName string, names ...string) {
	t.Helper()
	schema, ok := schemas[schemaName].(map[string]any)
	if !ok {
		t.Fatalf("expected OpenAPI schema %s", schemaName)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected OpenAPI schema %s properties", schemaName)
	}
	for _, name := range names {
		if _, ok := properties[name]; !ok {
			t.Fatalf("expected OpenAPI schema %s property %s", schemaName, name)
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
	writeServerRequiredZipFiles(t, zipWriter)
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
	writeServerRequiredReleaseFiles(t, releaseDir)
	return releaseDir
}

func writeServerRequiredReleaseFiles(t *testing.T, releaseDir string) {
	t.Helper()
	writeFileCSV := func(rel string, header []string, rows [][]string) {
		t.Helper()
		path := filepath.Join(releaseDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("create %s: %v", rel, err)
		}
		defer file.Close()
		writeRowsCSV(t, file, header, rows)
	}
	for _, spec := range serverRequiredCSVSpecs() {
		writeFileCSV(spec.path, spec.header, spec.rows)
	}
}

func writeServerRequiredZipFiles(t *testing.T, zipWriter *zip.Writer) {
	t.Helper()
	for _, spec := range serverRequiredCSVSpecs() {
		file, err := zipWriter.Create(filepath.ToSlash(filepath.Join("Loinc_Test", spec.path)))
		if err != nil {
			t.Fatalf("create zip entry %s: %v", spec.path, err)
		}
		writeRowsCSV(t, file, spec.header, spec.rows)
	}
}

type serverCSVSpec struct {
	path   string
	header []string
	rows   [][]string
}

func serverRequiredCSVSpecs() []serverCSVSpec {
	return []serverCSVSpec{
		{path: "LoincTable/MapTo.csv", header: []string{"LOINC", "MAP_TO", "COMMENT"}},
		{path: "LoincTable/SourceOrganization.csv", header: []string{"ID", "COPYRIGHT_ID", "NAME", "COPYRIGHT", "TERMS_OF_USE", "URL"}},
		{path: "AccessoryFiles/PartFile/Part.csv", header: []string{"PartNumber", "PartTypeName", "PartName", "PartDisplayName", "Status"}},
		{path: "AccessoryFiles/PartFile/LoincPartLink_Primary.csv", header: []string{"LoincNumber", "LongCommonName", "PartNumber", "PartName", "PartCodeSystem", "PartTypeName", "LinkTypeName", "Property"}},
		{path: "AccessoryFiles/PartFile/LoincPartLink_Supplementary.csv", header: []string{"LoincNumber", "LongCommonName", "PartNumber", "PartName", "PartCodeSystem", "PartTypeName", "LinkTypeName", "Property"}},
		{path: "AccessoryFiles/AnswerFile/AnswerList.csv", header: []string{"AnswerListId", "AnswerListName", "AnswerListOID", "ExtDefinedYN", "ExtDefinedAnswerListCodeSystem", "ExtDefinedAnswerListLink", "AnswerStringId", "LocalAnswerCode", "LocalAnswerCodeSystem", "SequenceNumber", "DisplayText", "ExtCodeId", "ExtCodeDisplayName", "ExtCodeSystem", "ExtCodeSystemVersion", "ExtCodeSystemCopyrightNotice", "SubsequentTextPrompt", "Description", "Score"}},
		{path: "AccessoryFiles/AnswerFile/LoincAnswerListLink.csv", header: []string{"LoincNumber", "LongCommonName", "AnswerListId", "AnswerListName", "AnswerListLinkType", "ApplicableContext"}},
		{path: "AccessoryFiles/PanelsAndForms/PanelsAndForms.csv", header: []string{"ParentId", "ParentLoinc", "ParentName", "ID", "SEQUENCE", "Loinc", "LoincName", "DisplayNameForForm", "ObservationRequiredInPanel", "ObservationIdInForm", "SkipLogicHelpText", "DefaultValue", "EntryType", "DataTypeInForm", "DataTypeSource", "AnswerSequenceOverride", "ConditionForInclusion", "AllowableAlternative", "ObservationCategory", "Context", "ConsistencyChecks", "RelevanceEquation", "CodingInstructions", "QuestionCardinality", "AnswerCardinality", "AnswerListIdOverride", "AnswerListTypeOverride", "EXTERNAL_COPYRIGHT_NOTICE", "AdditionalCopyright"}},
		{path: "AccessoryFiles/GroupFile/ParentGroup.csv", header: []string{"ParentGroupId", "ParentGroup", "Status"}},
		{path: "AccessoryFiles/GroupFile/Group.csv", header: []string{"ParentGroupId", "GroupId", "Group", "Archetype", "Status", "VersionFirstReleased"}},
		{path: "AccessoryFiles/GroupFile/GroupLoincTerms.csv", header: []string{"Category", "GroupId", "Archetype", "LoincNumber", "LongCommonName"}},
		{path: "AccessoryFiles/ComponentHierarchyBySystem/ComponentHierarchyBySystem.csv", header: []string{"PATH_TO_ROOT", "SEQUENCE", "IMMEDIATE_PARENT", "CODE", "CODE_TEXT"}},
	}
}

func writeRowsCSV(t *testing.T, file io.Writer, header []string, rows [][]string) {
	t.Helper()
	writer := csv.NewWriter(file)
	if err := writer.Write(header); err != nil {
		t.Fatalf("write csv header: %v", err)
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			t.Fatalf("write csv row: %v", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatalf("flush csv: %v", err)
	}
}
