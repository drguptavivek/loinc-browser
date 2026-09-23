package fhirhttp

import (
	"net/http"
	"strings"
	"testing"
)

// TestSummaryTrueOnCodeSystemRead checks §4.13 `_summary=true`: only R4 summary elements
// (docs/vendor/hl7/r4-summary-elements.json) plus resourceType/id/meta survive, and the
// SUBSETTED meta.tag is added since our CodeSystem struct never carries a meta field itself.
func TestSummaryTrueOnCodeSystemRead(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/CodeSystem/loinc?_summary=true")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body := decodeParameters(t, mustReadAll(t, resp))
	if body["resourceType"] != "CodeSystem" {
		t.Fatalf("resourceType = %v", body["resourceType"])
	}
	if _, ok := body["description"]; ok {
		t.Errorf("description is not a summary element, should be dropped: %+v", body)
	}
	if _, ok := body["copyright"]; ok {
		t.Errorf("copyright is not a summary element, should be dropped: %+v", body)
	}
	if body["url"] == nil || body["status"] == nil {
		t.Errorf("expected summary elements url/status to survive: %+v", body)
	}
	assertSubsettedTag(t, body)
}

// TestElementsOnCodeSystemRead checks `_elements=url,name`: only the named elements plus
// resourceType/id/meta/mandatory (status, content for CodeSystem) survive; unknown names are
// silently ignored.
func TestElementsOnCodeSystemRead(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/CodeSystem/loinc?_elements=url,name,bogus")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body := decodeParameters(t, mustReadAll(t, resp))
	if body["url"] == nil || body["name"] == nil {
		t.Fatalf("expected requested elements url/name: %+v", body)
	}
	if body["status"] == nil || body["content"] == nil {
		t.Errorf("expected mandatory elements status/content to survive: %+v", body)
	}
	if _, ok := body["title"]; ok {
		t.Errorf("title was not requested and is not mandatory, should be dropped: %+v", body)
	}
	assertSubsettedTag(t, body)
}

// TestSummaryTextOnCodeSystemRead checks `_summary=text`: only text/id/meta/mandatory survive.
// None of this server's resources ever carry a "text" narrative field, so the only observable
// difference from a bare base filter is that non-mandatory summary elements like url/name are
// also dropped (unlike `_summary=true`).
func TestSummaryTextOnCodeSystemRead(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/CodeSystem/loinc?_summary=text")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body := decodeParameters(t, mustReadAll(t, resp))
	if body["status"] == nil || body["content"] == nil {
		t.Errorf("expected mandatory elements status/content to survive: %+v", body)
	}
	if _, ok := body["url"]; ok {
		t.Errorf("url is not mandatory and _summary=text is not _summary=true, should be dropped: %+v", body)
	}
	if _, ok := body["name"]; ok {
		t.Errorf("name should be dropped under _summary=text: %+v", body)
	}
	assertSubsettedTag(t, body)
}

// TestSummaryDataOnCodeSystemRead checks `_summary=data`: everything except "text" survives.
// Since no resource here carries a "text" field, the observable effect is that nothing is
// dropped except the SUBSETTED tag being added.
func TestSummaryDataOnCodeSystemRead(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/CodeSystem/loinc?_summary=data")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body := decodeParameters(t, mustReadAll(t, resp))
	if body["description"] == nil || body["copyright"] == nil {
		t.Errorf("expected _summary=data to keep non-text elements: %+v", body)
	}
	assertSubsettedTag(t, body)
}

// TestSummaryTrueOnMetadata checks `_summary=true` on the TerminologyCapabilities shape, the one
// metadata case that actually discriminates: every top-level CapabilityStatement field happens
// to be an R4 summary element, but TerminologyCapabilities.codeSystem/translation are not.
func TestSummaryTrueOnMetadataTerminology(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/metadata?mode=terminology&_summary=true")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body := decodeParameters(t, mustReadAll(t, resp))
	if body["resourceType"] != "TerminologyCapabilities" {
		t.Fatalf("resourceType = %v", body["resourceType"])
	}
	if _, ok := body["codeSystem"]; ok {
		t.Errorf("codeSystem is not a TerminologyCapabilities summary element, should be dropped: %+v", body)
	}
	if _, ok := body["translation"]; ok {
		t.Errorf("translation is not a TerminologyCapabilities summary element, should be dropped: %+v", body)
	}
	if body["software"] == nil || body["status"] == nil || body["kind"] == nil {
		t.Errorf("expected summary elements software/status/kind to survive: %+v", body)
	}
	assertSubsettedTag(t, body)
}

// TestMetadataSummaryCountIs400 checks `/fhir/metadata` is a single resource, not a searchset:
// `_summary=count` there is 400 invalid.
func TestMetadataSummaryCountIs400(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/metadata?_summary=count")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestSummaryElementsCombined400 checks §4.13: combining _summary and _elements is 400 invalid.
func TestSummaryElementsCombined400(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/CodeSystem/loinc?_summary=true&_elements=url")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	body := decodeParameters(t, mustReadAll(t, resp))
	if body["resourceType"] != "OperationOutcome" {
		t.Fatalf("resourceType = %v", body["resourceType"])
	}
}

// TestInvalidSummaryValue400 checks an unrecognized `_summary` value is 400 invalid.
func TestInvalidSummaryValue400(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/CodeSystem/loinc?_summary=bogus")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestSummaryCountOnReadIs400 checks §4.13: `_summary=count` only applies to searches; on a read
// it is 400 invalid.
func TestSummaryCountOnReadIs400(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/CodeSystem/loinc?_summary=count")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestSummaryFalseIsFullResource checks `_summary=false` (an explicit no-op) returns the
// unfiltered resource.
func TestSummaryFalseIsFullResource(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/CodeSystem/loinc?_summary=false")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body := decodeParameters(t, mustReadAll(t, resp))
	if body["description"] == nil || body["copyright"] == nil {
		t.Errorf("expected full resource, description/copyright dropped: %+v", body)
	}
	if _, ok := body["meta"]; ok {
		t.Errorf("unfiltered resource should carry no meta.tag: %+v", body)
	}
}

// TestSummaryCountOnSearchDropsEntries checks §4.13: on a search, `_summary=count` returns a
// Bundle with type/total/link[self] and no entry.
func TestSummaryCountOnSearchDropsEntries(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/CodeSystem?url=http://loinc.org&_summary=count")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body := decodeParameters(t, mustReadAll(t, resp))
	if body["resourceType"] != "Bundle" || body["total"].(float64) != 1 {
		t.Fatalf("bundle = %+v", body)
	}
	if _, ok := body["entry"]; ok {
		t.Errorf("_summary=count must drop entry: %+v", body)
	}
	links, _ := body["link"].([]any)
	if len(links) != 1 {
		t.Fatalf("expected exactly a self link, got %+v", body["link"])
	}
	self, _ := links[0].(map[string]any)
	if self["relation"] != "self" {
		t.Fatalf("expected relation=self, got %+v", self)
	}
}

// TestElementsFiltersSearchBundleEntries checks `_elements` on a search: each entry.resource is
// filtered to the requested elements plus resourceType/id/meta/mandatory.
func TestElementsFiltersSearchBundleEntries(t *testing.T) {
	server := newValueSetTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/ValueSet?url=http://loinc.org/vs/LL1000-1&_elements=name")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body := decodeParameters(t, mustReadAll(t, resp))
	entries, _ := body["entry"].([]any)
	if len(entries) != 1 {
		t.Fatalf("entry = %+v", entries)
	}
	entry, _ := entries[0].(map[string]any)
	resource, _ := entry["resource"].(map[string]any)
	if resource["name"] == nil {
		t.Errorf("expected requested element name: %+v", resource)
	}
	if resource["status"] == nil {
		t.Errorf("expected mandatory element status to survive: %+v", resource)
	}
	if _, ok := resource["publisher"]; ok {
		t.Errorf("publisher was not requested and is not mandatory, should be dropped: %+v", resource)
	}
	assertSubsettedTag(t, resource)
}

// TestSummaryFiltersSearchBundleEntries checks that `_summary=true` on a search filters each
// entry.resource in place (the Bundle envelope itself is untouched).
func TestSummaryFiltersSearchBundleEntries(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/CodeSystem?url=http://loinc.org&_summary=true")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body := decodeParameters(t, mustReadAll(t, resp))
	if body["resourceType"] != "Bundle" {
		t.Fatalf("resourceType = %v", body["resourceType"])
	}
	entries, _ := body["entry"].([]any)
	if len(entries) != 1 {
		t.Fatalf("entry = %+v", entries)
	}
	entry, _ := entries[0].(map[string]any)
	resource, _ := entry["resource"].(map[string]any)
	if _, ok := resource["description"]; ok {
		t.Errorf("entry.resource should be summary-filtered: %+v", resource)
	}
	assertSubsettedTag(t, resource)
}

// TestSummaryKeyOrderPreserved checks §4.13's ordering requirement: the keys that remain after
// filtering keep their original order (a map[string]any re-marshal would sort them instead).
func TestSummaryKeyOrderPreserved(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/CodeSystem/loinc?_elements=version,url")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	raw := string(mustReadAll(t, resp))
	// CodeSystem's Go struct field order is ResourceType, ID, URL, ..., Version, ...: url must
	// still precede version in the filtered output.
	urlIdx := strings.Index(raw, `"url"`)
	versionIdx := strings.Index(raw, `"version"`)
	if urlIdx == -1 || versionIdx == -1 || urlIdx > versionIdx {
		t.Fatalf("expected url before version in %s", raw)
	}
}

// TestExpandSummaryFiltersValueSetResource checks §4.13 applies to the ValueSet resource
// returned by $expand.
func TestExpandSummaryFiltersValueSetResource(t *testing.T) {
	server := newValueSetTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/ValueSet/LL1000-1/$expand?_summary=true")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body := decodeParameters(t, mustReadAll(t, resp))
	if body["resourceType"] != "ValueSet" {
		t.Fatalf("resourceType = %v", body["resourceType"])
	}
	// ValueSet.expansion/compose are not R4 summary elements (plan §4.13).
	if _, ok := body["expansion"]; ok {
		t.Errorf("expansion is not a ValueSet summary element, should be dropped: %+v", body)
	}
	assertSubsettedTag(t, body)
}

// TestExpandSummaryCountIs400 checks $expand is a single-resource operation, not a search: an
// invalid _summary=count there is 400, not a special zero-entry response.
func TestExpandSummaryCountIs400(t *testing.T) {
	server := newValueSetTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/ValueSet/LL1000-1/$expand?_summary=count")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestLookupIgnoresSummaryParam checks §4.13: Parameters-returning operations ($lookup et al.)
// are out of scope and a stray _summary param is simply ignored, not rejected or applied.
func TestLookupIgnoresSummaryParam(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/CodeSystem/$lookup?code=10000-1&_summary=true")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body := decodeParameters(t, mustReadAll(t, resp))
	if body["resourceType"] != "Parameters" {
		t.Fatalf("resourceType = %v", body["resourceType"])
	}
	if _, ok := body["meta"]; ok {
		t.Errorf("Parameters output must not gain a meta.tag: %+v", body)
	}
}

func assertSubsettedTag(t *testing.T, body map[string]any) {
	t.Helper()
	meta, _ := body["meta"].(map[string]any)
	if meta == nil {
		t.Fatalf("expected meta.tag SUBSETTED, got no meta: %+v", body)
	}
	tags, _ := meta["tag"].([]any)
	for _, tag := range tags {
		entry, _ := tag.(map[string]any)
		if entry["code"] == "SUBSETTED" && entry["system"] == "http://terminology.hl7.org/CodeSystem/v3-ObservationValue" {
			return
		}
	}
	t.Fatalf("expected SUBSETTED tag in meta.tag, got %+v", meta)
}
