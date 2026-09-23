package fhirhttp

import (
	"net/http"

	"loinc-browser/pkg/terminology"
)

// init registers Questionnaire's routes: search-type and read. Unlike CodeSystem/ConceptMap,
// Questionnaire has no operations, so its read id never collides with a "$op" segment and a
// plain "{id}" pattern is enough (§4.11).
func init() {
	AppendRoutes(
		RouteEntry{Method: "GET", Pattern: "/fhir/Questionnaire", Handler: questionnaireSearchHandlerFunc},
		RouteEntry{Method: "GET", Pattern: "/fhir/Questionnaire/{id}", Handler: questionnaireReadHandlerFunc},
	)
}

func questionnaireSearchHandlerFunc(w http.ResponseWriter, r *http.Request) {
	questionnaireSearchHandler(currentConceptMapService)(w, r)
}

func questionnaireReadHandlerFunc(w http.ResponseWriter, r *http.Request) {
	questionnaireReadHandler(currentConceptMapService)(w, r)
}

// questionnaireSearchHandler serves GET /fhir/Questionnaire?url=http://loinc.org/q/{LOINC}. A
// url search only ever matches at most one panel, so _count/_offset never change the result set
// (as with codeSystemSearchHandler); they are still validated and drive the self Bundle.link.
func questionnaireSearchHandler(svc *terminology.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if wantsXML(r) {
			writeOutcome(w, http.StatusNotAcceptable, "not-supported", "XML is not supported")
			return
		}
		query := r.URL.Query()
		filter, filterErr := parseSummaryFilter(query, true)
		if filterErr != nil {
			writeOutcomeError(w, filterErr)
			return
		}
		count, countErr := parsePositiveIntParam(query.Get("_count"), 20)
		if countErr != nil {
			writeOutcomeError(w, countErr)
			return
		}
		offset, offsetErr := parsePositiveIntParam(query.Get("_offset"), 0)
		if offsetErr != nil {
			writeOutcomeError(w, offsetErr)
			return
		}
		bundle, err := svc.SearchQuestionnaire(r.Context(), query.Get("url"))
		if err != nil {
			writeOutcomeError(w, err)
			return
		}
		if bundle.Entry != nil || bundle.Total > 0 {
			bundle.Link = searchBundleLinks(r, bundle.Total, offset, count)
		}
		if filterErr := applySummaryBundle(r, bundle, filter); filterErr != nil {
			writeOutcomeError(w, filterErr)
			return
		}
		writeFHIR(w, http.StatusOK, bundle)
	}
}

// questionnaireReadHandler serves GET /fhir/Questionnaire/{id}. A non-panel LOINC 404s.
func questionnaireReadHandler(svc *terminology.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if wantsXML(r) {
			writeOutcome(w, http.StatusNotAcceptable, "not-supported", "XML is not supported")
			return
		}
		filter, filterErr := parseSummaryFilter(r.URL.Query(), false)
		if filterErr != nil {
			writeOutcomeError(w, filterErr)
			return
		}
		resource, err := svc.Questionnaire(r.Context(), r.PathValue("id"))
		if err != nil {
			writeOutcomeError(w, err)
			return
		}
		writeFHIRSummary(w, http.StatusOK, resource, filter)
	}
}
