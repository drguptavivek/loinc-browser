package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"loinc-browser/internal/loinc"
)

func newSearchAPITestServer(t *testing.T) *httptest.Server {
	t.Helper()
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
	t.Cleanup(func() { _ = store.Close() })

	indexPath := filepath.Join(t.TempDir(), "loinc-search.bleve")
	server := httptest.NewServer(New(Options{Store: store, SearchIndexPath: indexPath}))
	t.Cleanup(server.Close)

	var status LocalSearchStatus
	postJSONValue(t, server.URL+"/api/v1/local-search/rebuild", nil, &status)
	if status.State != "ready" {
		t.Fatalf("expected rebuilt local search index, got %#v", status)
	}
	return server
}

func getSearchAPI(t *testing.T, url string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("content-type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("get %s: expected JSON content type, got %q", url, ct)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode %s: %v", url, err)
	}
	return resp.StatusCode, body
}

func TestSearchAPIUnknownScope(t *testing.T) {
	server := newSearchAPITestServer(t)
	status, body := getSearchAPI(t, server.URL+"/searchapi/bogus?query=x")
	if status != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown scope, got %d: %#v", status, body)
	}
	if _, ok := body["Message"]; !ok {
		t.Fatalf("expected Message field in 404 body, got %#v", body)
	}
}

func TestSearchAPIMissingIndexReturns503(t *testing.T) {
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
	indexPath := filepath.Join(t.TempDir(), "loinc-search.bleve")
	server := httptest.NewServer(New(Options{Store: store, SearchIndexPath: indexPath}))
	defer server.Close()

	status, body := getSearchAPI(t, server.URL+"/searchapi/loincs?query=Cholesterol")
	if status != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 before index build, got %d: %#v", status, body)
	}
	if msg, _ := body["Message"].(string); msg != searchAPIMissingIndex {
		t.Fatalf("expected missing-index message, got %#v", body)
	}
}

func TestSearchAPILoincsRowShapeAndPaging(t *testing.T) {
	server := newSearchAPITestServer(t)

	status, body := getSearchAPI(t, server.URL+"/searchapi/loincs?query=Cholesterol&rows=1")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %#v", status, body)
	}
	summary, ok := body["ResponseSummary"].(map[string]any)
	if !ok {
		t.Fatalf("expected ResponseSummary object, got %#v", body)
	}
	for _, key := range []string{"RecordsFound", "StartingOffset", "RowsReturned", "LoincVersion", "Copyright", "QueryUrl", "QueryExecutionTime", "QueryDuration"} {
		if _, ok := summary[key]; !ok {
			t.Fatalf("expected ResponseSummary.%s, got %#v", key, summary)
		}
	}
	if version, ok := summary["LoincVersion"].(string); !ok || version == "" {
		t.Fatalf("expected non-empty LoincVersion, got %#v", summary["LoincVersion"])
	}

	results, ok := body["Results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("expected exactly one result row, got %#v", body["Results"])
	}
	row, ok := results[0].(map[string]any)
	if !ok {
		t.Fatalf("expected result row object, got %#v", results[0])
	}
	if row["LOINC_NUM"] != "2000-1" {
		t.Fatalf("expected Cholesterol row 2000-1, got %#v", row["LOINC_NUM"])
	}
	for _, key := range []string{
		"COMPONENT", "PROPERTY", "TIME_ASPCT", "SYSTEM", "SCALE_TYP", "METHOD_TYP", "CLASS",
		"CLASSTYPE", "COMMON_TEST_RANK", "COMMON_ORDER_RANK", "COMMON_SI_TEST_RANK",
		"FormalName", "DisplayName", "TermDescriptions", "CodeSystems", "Tags", "Link",
	} {
		if _, ok := row[key]; !ok {
			t.Fatalf("expected loincs row key %s, got keys %v", key, mapKeys(row))
		}
	}
	if _, isNumber := row["CLASSTYPE"].(float64); !isNumber {
		t.Fatalf("expected CLASSTYPE as a JSON number, got %#v", row["CLASSTYPE"])
	}
	if link := row["Link"]; link != "https://loinc.org/2000-1" {
		t.Fatalf("expected computed Link, got %#v", link)
	}

	// Paging: two loincs total (Cholesterol, Platelets); rows=1 should page.
	status, body = getSearchAPI(t, server.URL+"/searchapi/loincs?rows=1&sortorder=loinc_num")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %#v", status, body)
	}
	summary = body["ResponseSummary"].(map[string]any)
	if summary["RecordsFound"].(float64) != 2 {
		t.Fatalf("expected 2 records found across both terms, got %#v", summary["RecordsFound"])
	}
	next, ok := summary["Next"].(string)
	if !ok || next == "" {
		t.Fatalf("expected Next page URL when more rows remain, got %#v", summary)
	}
	if _, hasPrevious := summary["Previous"]; hasPrevious {
		t.Fatalf("did not expect Previous on the first page, got %#v", summary)
	}
	first := body["Results"].([]any)[0].(map[string]any)
	if first["LOINC_NUM"] != "2000-1" {
		t.Fatalf("expected ascending loinc_num sort to start at 2000-1, got %#v", first["LOINC_NUM"])
	}

	status, body = getSearchAPI(t, next)
	if status != http.StatusOK {
		t.Fatalf("expected 200 following Next, got %d: %#v", status, body)
	}
	summary = body["ResponseSummary"].(map[string]any)
	if _, hasPrevious := summary["Previous"]; !hasPrevious {
		t.Fatalf("expected Previous once offset > 0, got %#v", summary)
	}
	second := body["Results"].([]any)[0].(map[string]any)
	if second["LOINC_NUM"] != "2001-9" {
		t.Fatalf("expected second ascending page to be 2001-9, got %#v", second["LOINC_NUM"])
	}

	// rows above the cap clamp to the max.
	status, body = getSearchAPI(t, server.URL+"/searchapi/loincs?rows=99999")
	if status != http.StatusOK {
		t.Fatalf("expected 200 for oversized rows, got %d: %#v", status, body)
	}

	// no match -> empty Results, no Next/Previous.
	status, body = getSearchAPI(t, server.URL+"/searchapi/loincs?query=zzqqxxnomatch")
	if status != http.StatusOK {
		t.Fatalf("expected 200 for no-match query, got %d: %#v", status, body)
	}
	summary = body["ResponseSummary"].(map[string]any)
	if summary["RecordsFound"].(float64) != 0 || summary["RowsReturned"].(float64) != 0 {
		t.Fatalf("expected zero results for no-match query, got %#v", summary)
	}
	if results, ok := body["Results"].([]any); !ok || len(results) != 0 {
		t.Fatalf("expected empty Results array, got %#v", body["Results"])
	}
}

func TestSearchAPIFilterCounts(t *testing.T) {
	server := newSearchAPITestServer(t)
	status, body := getSearchAPI(t, server.URL+"/searchapi/loincs?rows=1&includefiltercounts=true")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %#v", status, body)
	}
	counts, ok := body["FilterCounts"].(map[string]any)
	if !ok {
		t.Fatalf("expected FilterCounts object, got %#v", body)
	}
	classFacet, ok := counts["Class"].([]any)
	if !ok || len(classFacet) != 2 {
		t.Fatalf("expected two Class facet buckets (CHEM, HEM/BC), got %#v", counts["Class"])
	}
	entry := classFacet[0].(map[string]any)
	for _, key := range []string{"Label", "Search", "Count"} {
		if _, ok := entry[key]; !ok {
			t.Fatalf("expected filter count entry key %s, got %#v", key, entry)
		}
	}

	// Other scopes never carry FilterCounts, matching the exemplars.
	status, body = getSearchAPI(t, server.URL+"/searchapi/parts?query=Cholesterol&includefiltercounts=true")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %#v", status, body)
	}
	if _, ok := body["FilterCounts"]; ok {
		t.Fatalf("did not expect FilterCounts for parts scope, got %#v", body)
	}
}

func TestSearchAPIPartsRowShape(t *testing.T) {
	server := newSearchAPITestServer(t)
	status, body := getSearchAPI(t, server.URL+"/searchapi/parts?query=Cholesterol")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %#v", status, body)
	}
	results := body["Results"].([]any)
	if len(results) != 1 {
		t.Fatalf("expected one part result, got %#v", results)
	}
	row := results[0].(map[string]any)
	if row["PartNumber"] != "LP1000-1" {
		t.Fatalf("expected LP1000-1, got %#v", row["PartNumber"])
	}
	for _, key := range []string{"PartTypeName", "PartName", "PartDisplayName", "Status", "Classlist", "Link"} {
		if _, ok := row[key]; !ok {
			t.Fatalf("expected parts row key %s, got keys %v", key, mapKeys(row))
		}
	}
	if row["Classlist"] != "CHEM" {
		t.Fatalf("expected Classlist CHEM from the linked Cholesterol term, got %#v", row["Classlist"])
	}
}

func TestSearchAPIAnswerListsRowShape(t *testing.T) {
	server := newSearchAPITestServer(t)
	status, body := getSearchAPI(t, server.URL+"/searchapi/answerlists?query=Positive")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %#v", status, body)
	}
	results := body["Results"].([]any)
	if len(results) != 1 {
		t.Fatalf("expected one answer list result, got %#v", results)
	}
	row := results[0].(map[string]any)
	if row["AnswerListId"] != "LL1000-1" {
		t.Fatalf("expected LL1000-1, got %#v", row["AnswerListId"])
	}
	for _, key := range []string{"Name", "Description", "LoincAnswerListOid", "ExtDefinedYN", "Answers", "Link"} {
		if _, ok := row[key]; !ok {
			t.Fatalf("expected answerlists row key %s, got keys %v", key, mapKeys(row))
		}
	}
	answers, ok := row["Answers"].([]any)
	if !ok || len(answers) != 1 {
		t.Fatalf("expected one answer, got %#v", row["Answers"])
	}
	answer := answers[0].(map[string]any)
	if answer["AnswerStringId"] != "LA1-1" {
		t.Fatalf("expected LA1-1, got %#v", answer["AnswerStringId"])
	}
	for _, key := range []string{"SequenceNumber", "DisplayText", "Score", "ExtCodeId"} {
		if _, ok := answer[key]; !ok {
			t.Fatalf("expected answer key %s, got keys %v", key, mapKeys(answer))
		}
	}
}

func TestSearchAPIGroupsRowShape(t *testing.T) {
	server := newSearchAPITestServer(t)
	status, body := getSearchAPI(t, server.URL+"/searchapi/groups?query=Chemistry")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %#v", status, body)
	}
	results := body["Results"].([]any)
	if len(results) != 1 {
		t.Fatalf("expected one group result, got %#v", results)
	}
	row := results[0].(map[string]any)
	if row["GroupId"] != "LG1000-1" {
		t.Fatalf("expected LG1000-1, got %#v", row["GroupId"])
	}
	if row["ParentGroupId"] != "PG1000" || row["ParentGroup"] != "Chemistry" {
		t.Fatalf("expected parent group linkage, got %#v %#v", row["ParentGroupId"], row["ParentGroup"])
	}
	for _, key := range []string{
		"Group", "Archetype", "STATUS", "VersionFirstReleased", "UsageNotes",
		"MolecularWeightOfAnalyte", "Category", "Loincs", "Link",
	} {
		if _, ok := row[key]; !ok {
			t.Fatalf("expected groups row key %s, got keys %v", key, mapKeys(row))
		}
	}
	loincs, ok := row["Loincs"].([]any)
	if !ok || len(loincs) != 1 {
		t.Fatalf("expected one member LOINC, got %#v", row["Loincs"])
	}
	member := loincs[0].(map[string]any)
	if member["LoincNumber"] != "2000-1" {
		t.Fatalf("expected member 2000-1, got %#v", member["LoincNumber"])
	}
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
