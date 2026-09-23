package fhirhttp

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"loinc-browser/internal/loinc"
	"loinc-browser/pkg/terminology"
)

// writeFHIRHTTPTestRelease is a minimal release for exercising the HTTP adapter layer itself
// (param parsing, content type, error shapes); pkg/terminology's own tests cover the five
// $lookup code kinds in depth.
func writeFHIRHTTPTestRelease(t *testing.T) string {
	t.Helper()
	releaseDir := filepath.Join(t.TempDir(), "Loinc_1.0")
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		t.Fatalf("mkdir release dir: %v", err)
	}
	writeCSV(t, filepath.Join(releaseDir, "LoincTable", "Loinc.csv"),
		[]string{
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
		},
		[][]string{{
			"10000-1", "Cholesterol", "MCnc", "Pt", "Serum", "Qn", "", "CHEM",
			"1.0", "ADD", "Cholesterol mass concentration in serum", "ACTIVE",
			"", "1", "", "", "", "", "N", "cholesterol",
			"Chol Ser", "Observation", "", "", "mg/dL", "Cholesterol [Mass/volume] in Serum",
			"mg/dL", "", "", "", "0", "0", "", "", "", "", "", "1.0", "", "Cholesterol Serum",
		}})
	for _, spec := range [][2]any{
		{filepath.Join(releaseDir, "LoincTable", "MapTo.csv"), []string{"LOINC", "MAP_TO", "COMMENT"}},
		{filepath.Join(releaseDir, "LoincTable", "SourceOrganization.csv"), []string{"ID", "COPYRIGHT_ID", "NAME", "COPYRIGHT", "TERMS_OF_USE", "URL"}},
		{filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "Part.csv"), []string{"PartNumber", "PartTypeName", "PartName", "PartDisplayName", "Status"}},
		{filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "LoincPartLink_Primary.csv"), []string{"LoincNumber", "LongCommonName", "PartNumber", "PartName", "PartCodeSystem", "PartTypeName", "LinkTypeName", "Property"}},
		{filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "LoincPartLink_Supplementary.csv"), []string{"LoincNumber", "LongCommonName", "PartNumber", "PartName", "PartCodeSystem", "PartTypeName", "LinkTypeName", "Property"}},
		{filepath.Join(releaseDir, "AccessoryFiles", "AnswerFile", "AnswerList.csv"), []string{"AnswerListId", "AnswerListName", "AnswerListOID", "ExtDefinedYN", "ExtDefinedAnswerListCodeSystem", "ExtDefinedAnswerListLink", "AnswerStringId", "LocalAnswerCode", "LocalAnswerCodeSystem", "SequenceNumber", "DisplayText", "ExtCodeId", "ExtCodeDisplayName", "ExtCodeSystem", "ExtCodeSystemVersion", "ExtCodeSystemCopyrightNotice", "SubsequentTextPrompt", "Description", "Score"}},
		{filepath.Join(releaseDir, "AccessoryFiles", "AnswerFile", "LoincAnswerListLink.csv"), []string{"LoincNumber", "LongCommonName", "AnswerListId", "AnswerListName", "AnswerListLinkType", "ApplicableContext"}},
		{filepath.Join(releaseDir, "AccessoryFiles", "PanelsAndForms", "PanelsAndForms.csv"), []string{"ParentId", "ParentLoinc", "ParentName", "ID", "SEQUENCE", "Loinc", "LoincName", "DisplayNameForForm", "ObservationRequiredInPanel", "ObservationIdInForm", "SkipLogicHelpText", "DefaultValue", "EntryType", "DataTypeInForm", "DataTypeSource", "AnswerSequenceOverride", "ConditionForInclusion", "AllowableAlternative", "ObservationCategory", "Context", "ConsistencyChecks", "RelevanceEquation", "CodingInstructions", "QuestionCardinality", "AnswerCardinality", "AnswerListIdOverride", "AnswerListTypeOverride", "EXTERNAL_COPYRIGHT_NOTICE", "AdditionalCopyright"}},
		{filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "ParentGroup.csv"), []string{"ParentGroupId", "ParentGroup", "Status"}},
		{filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "Group.csv"), []string{"ParentGroupId", "GroupId", "Group", "Archetype", "Status", "VersionFirstReleased"}},
		{filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "GroupLoincTerms.csv"), []string{"Category", "GroupId", "Archetype", "LoincNumber", "LongCommonName"}},
		{filepath.Join(releaseDir, "AccessoryFiles", "ComponentHierarchyBySystem", "ComponentHierarchyBySystem.csv"), []string{"PATH_TO_ROOT", "SEQUENCE", "IMMEDIATE_PARENT", "CODE", "CODE_TEXT"}},
	} {
		writeCSV(t, spec[0].(string), spec[1].([]string), nil)
	}
	return releaseDir
}

func writeCSV(t *testing.T, path string, header []string, rows [][]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer file.Close()
	writer := csv.NewWriter(file)
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
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ctx := context.Background()
	releaseDir := writeFHIRHTTPTestRelease(t)
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

func decodeParameters(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode response: %v\nbody: %s", err, body)
	}
	return decoded
}

func TestLookupGET(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/CodeSystem/$lookup?code=10000-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/fhir+json;charset=UTF-8" {
		t.Fatalf("content-type = %q", ct)
	}
	var parsed struct {
		ResourceType string `json:"resourceType"`
		Parameter    []struct {
			Name        string `json:"name"`
			ValueCode   string `json:"valueCode"`
			ValueString string `json:"valueString"`
		} `json:"parameter"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if parsed.ResourceType != "Parameters" {
		t.Fatalf("resourceType = %q", parsed.ResourceType)
	}
	if len(parsed.Parameter) == 0 || parsed.Parameter[0].Name != "code" || parsed.Parameter[0].ValueCode != "10000-1" {
		t.Fatalf("parameter[0] = %+v", parsed.Parameter)
	}
}

func TestLookupPOST(t *testing.T) {
	server := newTestServer(t)
	body := `{"resourceType":"Parameters","parameter":[{"name":"code","valueCode":"10000-1"}]}`
	resp, err := http.Post(server.URL+"/fhir/CodeSystem/$lookup", "application/fhir+json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

// TestLookupPOSTOversizeBodyIs413 guards item 9: a POST body over maxRequestBodySize must be
// rejected with 413 "too-costly", not read in full or truncated into an invalid-JSON 400.
func TestLookupPOSTOversizeBodyIs413(t *testing.T) {
	server := newTestServer(t)
	oversize := strings.Repeat("a", maxRequestBodySize+1)
	body := `{"resourceType":"Parameters","parameter":[{"name":"code","valueCode":"10000-1"},{"name":"padding","valueString":"` + oversize + `"}]}`
	resp, err := http.Post(server.URL+"/fhir/CodeSystem/$lookup", "application/fhir+json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusRequestEntityTooLarge)
	}
	var outcome operationOutcome
	if err := json.NewDecoder(resp.Body).Decode(&outcome); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if outcome.ResourceType != "OperationOutcome" || len(outcome.Issue) != 1 || outcome.Issue[0].Code != "too-costly" {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestLookupWithIDInPath(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/CodeSystem/loinc/$lookup?code=10000-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestLookupUnknownIsOperationOutcome(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/CodeSystem/$lookup?code=99999-9")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var outcome operationOutcome
	if err := json.NewDecoder(resp.Body).Decode(&outcome); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if outcome.ResourceType != "OperationOutcome" || len(outcome.Issue) != 1 || outcome.Issue[0].Code != "not-found" {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestXMLFormatIsNotAcceptable(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/CodeSystem/$lookup?code=10000-1&_format=xml")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotAcceptable {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestMetadata(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/metadata")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body := decodeParameters(t, mustReadAll(t, resp))
	if body["resourceType"] != "CapabilityStatement" {
		t.Fatalf("resourceType = %v", body["resourceType"])
	}
}

func TestMetadataTerminology(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/metadata?mode=terminology")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body := decodeParameters(t, mustReadAll(t, resp))
	if body["resourceType"] != "TerminologyCapabilities" {
		t.Fatalf("resourceType = %v", body["resourceType"])
	}
}

func TestCodeSystemReadAndSearch(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/CodeSystem/loinc")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body := decodeParameters(t, mustReadAll(t, resp))
	if body["resourceType"] != "CodeSystem" {
		t.Fatalf("resourceType = %v", body["resourceType"])
	}

	resp2, err := http.Get(server.URL + "/fhir/CodeSystem?url=http://loinc.org")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp2.Body.Close()
	bundle := decodeParameters(t, mustReadAll(t, resp2))
	if bundle["resourceType"] != "Bundle" || bundle["total"].(float64) != 1 {
		t.Fatalf("bundle = %+v", bundle)
	}
	links, _ := bundle["link"].([]any)
	if len(links) != 1 {
		t.Fatalf("expected a self Bundle.link, got %+v", bundle["link"])
	}
	self, _ := links[0].(map[string]any)
	if self["relation"] != "self" {
		t.Fatalf("expected relation=self, got %+v", self)
	}
}

func TestSubsumesGET(t *testing.T) {
	server := newTestServer(t)
	resp, err := http.Get(server.URL + "/fhir/CodeSystem/$subsumes?codeA=10000-1&codeB=10000-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body := decodeParameters(t, mustReadAll(t, resp))
	params, _ := body["parameter"].([]any)
	if len(params) == 0 {
		t.Fatalf("no parameters: %+v", body)
	}
	first := params[0].(map[string]any)
	if first["name"] != "outcome" || first["valueString"] != "equivalent" {
		t.Fatalf("first parameter = %+v", first)
	}
}

func mustReadAll(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return data
}
