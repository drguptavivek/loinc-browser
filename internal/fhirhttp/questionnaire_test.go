package fhirhttp

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestQuestionnaireReadAndSearch(t *testing.T) {
	server := newConceptMapQuestionnaireServer(t)

	resp, err := http.Get(server.URL + "/fhir/Questionnaire/20001-0")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var body struct {
		ResourceType string `json:"resourceType"`
		Item         []struct {
			LinkID string `json:"linkId"`
		} `json:"item"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ResourceType != "Questionnaire" || len(body.Item) != 1 || body.Item[0].LinkID != "I1" {
		t.Fatalf("body = %+v", body)
	}

	searchResp, err := http.Get(server.URL + "/fhir/Questionnaire?url=" + loincSystemForTest + "/q/20001-0")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer searchResp.Body.Close()
	var bundle struct {
		Total int `json:"total"`
		Link  []struct {
			Relation string `json:"relation"`
		} `json:"link"`
	}
	if err := json.NewDecoder(searchResp.Body).Decode(&bundle); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if bundle.Total != 1 {
		t.Fatalf("bundle = %+v", bundle)
	}
	if len(bundle.Link) != 1 || bundle.Link[0].Relation != "self" {
		t.Fatalf("expected a self Bundle.link, got %+v", bundle.Link)
	}
}

// TestQuestionnaireSearchRejectsInvalidCount guards against _count/_offset being silently
// ignored on Questionnaire search-type (it now shares codeSystemSearchHandler's validation).
func TestQuestionnaireSearchRejectsInvalidCount(t *testing.T) {
	server := newConceptMapQuestionnaireServer(t)
	resp, err := http.Get(server.URL + "/fhir/Questionnaire?url=" + loincSystemForTest + "/q/20001-0&_count=-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestQuestionnaireNonPanelNotFound(t *testing.T) {
	server := newConceptMapQuestionnaireServer(t)
	resp, err := http.Get(server.URL + "/fhir/Questionnaire/10000-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}
