package fhirhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"loinc-browser/internal/loinc"
	"loinc-browser/pkg/terminology"
)

// newConceptMapQuestionnaireServer builds its own release, including a term with an IEEE mapping
// and a one-item panel, so this file's tests do not depend on newTestServer's minimal fixture in
// fhirhttp_test.go (which has neither).
func newConceptMapQuestionnaireServer(t *testing.T) *httptest.Server {
	t.Helper()
	ctx := context.Background()
	releaseDir := writeFHIRHTTPTestRelease(t)

	loincHeader := []string{
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
	writeCSV(t, filepath.Join(releaseDir, "LoincTable", "Loinc.csv"), loincHeader, [][]string{
		{
			"10000-1", "Cholesterol", "MCnc", "Pt", "Serum", "Qn", "", "CHEM",
			"1.0", "ADD", "Cholesterol mass concentration in serum", "ACTIVE",
			"", "1", "", "", "", "", "N", "cholesterol",
			"Chol Ser", "Observation", "", "", "mg/dL", "Cholesterol [Mass/volume] in Serum",
			"mg/dL", "", "", "", "0", "0", "", "", "", "", "", "1.0", "", "Cholesterol Serum",
		},
		{
			"20001-0", "Test panel", "", "", "", "-", "", "PANEL",
			"1.0", "ADD", "Test panel", "ACTIVE",
			"", "1", "", "", "", "", "N", "",
			"", "", "", "", "", "Test panel",
			"", "", "", "", "0", "0", "", "", "", "", "", "1.0", "", "Test panel",
		},
	})

	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "LoincIeeeMedicalDeviceCodeMappingTable", "LoincIeeeMedicalDeviceCodeMappingTable.csv"),
		[]string{"LOINC_NUM", "LOINC_LONG_COMMON_NAME", "IEEE_CF_CODE10", "IEEE_REFID", "EQUIVALENCE"},
		[][]string{{"10000-1", "Cholesterol [Mass/volume] in Serum", "999001", "MDC_TEST_CHOL", "equivalent"}})

	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "PanelsAndForms", "PanelsAndForms.csv"),
		[]string{"ParentId", "ParentLoinc", "ParentName", "ID", "SEQUENCE", "Loinc", "LoincName", "DisplayNameForForm", "ObservationRequiredInPanel", "ObservationIdInForm", "SkipLogicHelpText", "DefaultValue", "EntryType", "DataTypeInForm", "DataTypeSource", "AnswerSequenceOverride", "ConditionForInclusion", "AllowableAlternative", "ObservationCategory", "Context", "ConsistencyChecks", "RelevanceEquation", "CodingInstructions", "QuestionCardinality", "AnswerCardinality", "AnswerListIdOverride", "AnswerListTypeOverride", "EXTERNAL_COPYRIGHT_NOTICE", "AdditionalCopyright"},
		[][]string{
			{"P1", "20001-0", "Test panel", "P1", "0", "20001-0", "Test panel", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
			{"P1", "20001-0", "Test panel", "I1", "1", "10000-1", "Cholesterol", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		})

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

func TestConceptMapSearchGET(t *testing.T) {
	server := newConceptMapQuestionnaireServer(t)
	resp, err := http.Get(server.URL + "/fhir/ConceptMap?url=" + loincSystemForTest + "/cm/loinc-to-ieee-11073-10101")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		ResourceType string `json:"resourceType"`
		Total        int    `json:"total"`
		Link         []struct {
			Relation string `json:"relation"`
			URL      string `json:"url"`
		} `json:"link"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ResourceType != "Bundle" || body.Total != 1 {
		t.Fatalf("body = %+v", body)
	}
	if len(body.Link) != 1 || body.Link[0].Relation != "self" {
		t.Fatalf("expected a self Bundle.link, got %+v", body.Link)
	}
}

// TestConceptMapSearchPagingLinks guards against ConceptMap search-type never setting
// Bundle.link[self]/[next] (they were previously only built for ValueSet search); with _count=1
// and more than one ConceptMap catalogued, both links must be present and next must carry the
// next _offset.
func TestConceptMapSearchPagingLinks(t *testing.T) {
	server := newConceptMapQuestionnaireServer(t)
	resp, err := http.Get(server.URL + "/fhir/ConceptMap?_count=1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Total int `json:"total"`
		Link  []struct {
			Relation string `json:"relation"`
			URL      string `json:"url"`
		} `json:"link"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Total <= 1 {
		t.Fatalf("expected more than one catalogued ConceptMap to exercise paging, got total=%d", body.Total)
	}
	var hasSelf, hasNext bool
	for _, l := range body.Link {
		switch l.Relation {
		case "self":
			hasSelf = true
		case "next":
			hasNext = true
			if !strings.Contains(l.URL, "_offset=1") {
				t.Fatalf("expected next link to carry _offset=1, got %s", l.URL)
			}
		}
	}
	if !hasSelf || !hasNext {
		t.Fatalf("expected both self and next Bundle.link, got %+v", body.Link)
	}
}

func TestConceptMapReadAndTranslate(t *testing.T) {
	server := newConceptMapQuestionnaireServer(t)

	resp, err := http.Get(server.URL + "/fhir/ConceptMap/loinc-to-ieee-11073-10101")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("read status = %d", resp.StatusCode)
	}

	tResp, err := http.Get(server.URL + "/fhir/ConceptMap/$translate?system=" + loincSystemForTest + "&code=10000-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer tResp.Body.Close()
	if tResp.StatusCode != http.StatusOK {
		t.Fatalf("translate status = %d", tResp.StatusCode)
	}
	var parsed struct {
		Parameter []struct {
			Name         string `json:"name"`
			ValueBoolean *bool  `json:"valueBoolean"`
		} `json:"parameter"`
	}
	if err := json.NewDecoder(tResp.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(parsed.Parameter) == 0 || parsed.Parameter[0].Name != "result" || parsed.Parameter[0].ValueBoolean == nil || !*parsed.Parameter[0].ValueBoolean {
		t.Fatalf("parameter[0] = %+v", parsed.Parameter)
	}
}

func TestConceptMapUnknownIDNotFound(t *testing.T) {
	server := newConceptMapQuestionnaireServer(t)
	resp, err := http.Get(server.URL + "/fhir/ConceptMap/not-a-map")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

const loincSystemForTest = "http://loinc.org"
