// valueset.go adapts pkg/terminology's ValueSet API to HTTP (§4.6-§4.8): search/read on
// /fhir/ValueSet, and $expand / $validate-code on both /fhir/ValueSet/$op and
// /fhir/ValueSet/{id}/$op, mirroring codeSystemHandler's dispatch-by-trailing-segment approach in
// routes.go (Go 1.22 ServeMux can't register "{id}" and "$op" as separate overlapping patterns).
package fhirhttp

import (
	"net/http"
	"strconv"
	"strings"

	"loinc-browser/pkg/terminology"
)

func init() {
	AppendRoutes(
		RouteEntry{Method: "GET", Pattern: "/fhir/ValueSet", Handler: valueSetSearchHandler},
		RouteEntry{Method: "GET", Pattern: "/fhir/ValueSet/{rest...}", Handler: valueSetHandler},
		RouteEntry{Method: "POST", Pattern: "/fhir/ValueSet/{rest...}", Handler: valueSetHandler},
	)
}

// currentValueSetService is the terminology.Service the routes queued by this file's init()
// dispatch to. AppendRoutes queues these route entries at package init, before any Service
// exists, so — unlike baseRoutes' handlers, which close over svc directly — these resolve it
// lazily through this package-level pointer, set by Register (routes.go) before serving traffic.
var currentValueSetService *terminology.Service

// RegisterValueSetService supplies the terminology.Service this file's routes dispatch to.
func RegisterValueSetService(svc *terminology.Service) {
	currentValueSetService = svc
}

func valueSetSearchHandler(w http.ResponseWriter, r *http.Request) {
	svc := currentValueSetService
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
	if count > 100 {
		count = 100
	}
	offset, offsetErr := parsePositiveIntParam(query.Get("_offset"), 0)
	if offsetErr != nil {
		writeOutcomeError(w, offsetErr)
		return
	}
	name := query.Get("name")
	nameContains := firstNonEmpty(query.Get("name:contains"), query.Get("name:in"))
	bundle, err := svc.SearchValueSets(r.Context(), terminology.ValueSetSearchParams{
		URL: query.Get("url"), ID: query.Get("_id"), Name: name, NameContains: nameContains,
		Count: count, Offset: offset,
	})
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// valueSetHandler serves GET/POST /fhir/ValueSet/{rest...}: rest is an id, an operation
// ("$expand", "$validate-code"), or "{id}/{operation}" (mirrors codeSystemHandler).
func valueSetHandler(w http.ResponseWriter, r *http.Request) {
	svc := currentValueSetService
	if wantsXML(r) {
		writeOutcome(w, http.StatusNotAcceptable, "not-supported", "XML is not supported")
		return
	}
	rest := strings.Trim(r.PathValue("rest"), "/")
	if rest == "" {
		writeOutcome(w, http.StatusNotFound, "not-found", "Not found")
		return
	}
	segments := strings.Split(rest, "/")
	var id, op string
	switch len(segments) {
	case 1:
		if strings.HasPrefix(segments[0], "$") {
			op = segments[0]
		} else {
			id = segments[0]
		}
	case 2:
		id, op = segments[0], segments[1]
	default:
		writeOutcome(w, http.StatusNotFound, "not-found", "Not found")
		return
	}

	if op == "" {
		if r.Method != http.MethodGet {
			writeOutcome(w, http.StatusNotFound, "not-found", "Not found")
			return
		}
		filter, filterErr := parseSummaryFilter(r.URL.Query(), false)
		if filterErr != nil {
			writeOutcomeError(w, filterErr)
			return
		}
		resource, err := svc.ReadValueSet(r.Context(), id)
		if err != nil {
			writeOutcomeError(w, err)
			return
		}
		writeFHIRSummary(w, http.StatusOK, resource, filter)
		return
	}
	dispatchValueSetOperation(w, r, svc, id, op)
}

func dispatchValueSetOperation(w http.ResponseWriter, r *http.Request, svc *terminology.Service, id, op string) {
	params, paramErr := readParams(w, r)
	if paramErr != nil {
		writeOutcomeError(w, paramErr)
		return
	}
	ctx := r.Context()
	switch op {
	case "$expand":
		filter, filterErr := parseSummaryFilter(r.URL.Query(), false)
		if filterErr != nil {
			writeOutcomeError(w, filterErr)
			return
		}
		expandParams, expandErr := parseExpandParams(params, id)
		if expandErr != nil {
			writeOutcomeError(w, expandErr)
			return
		}
		result, err := svc.Expand(ctx, expandParams)
		if err != nil {
			writeOutcomeError(w, err)
			return
		}
		writeFHIRSummary(w, http.StatusOK, result, filter)
	case "$validate-code":
		result, err := svc.ValidateValueSetCode(ctx, parseValidateValueSetCodeParams(params, id))
		if err != nil {
			writeOutcomeError(w, err)
			return
		}
		writeFHIR(w, http.StatusOK, result)
	default:
		writeOutcome(w, http.StatusNotFound, "not-found", "Unknown operation "+op)
	}
}

// parsePositiveIntParam parses an optional non-negative integer query parameter, returning
// def when raw is empty and a 400 "invalid" OutcomeError for a negative or non-integer value
// (§4.7 count/offset, §4.6.3 _count/_offset).
func parsePositiveIntParam(raw string, def int) (int, *terminology.OutcomeError) {
	if raw == "" {
		return def, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, &terminology.OutcomeError{Status: 400, Code: "invalid", Text: "Parameter must be an integer: " + raw}
	}
	if value < 0 {
		return 0, &terminology.OutcomeError{Status: 400, Code: "invalid", Text: "Parameter must not be negative: " + raw}
	}
	return value, nil
}

func parseExpandParams(p *requestParams, id string) (terminology.ExpandParams, *terminology.OutcomeError) {
	out := terminology.ExpandParams{
		URL: p.first("url"), ID: id, Filter: p.first("filter"),
		ActiveOnly:          strings.EqualFold(p.first("activeOnly"), "true"),
		IncludeDesignations: strings.EqualFold(p.first("includeDesignations"), "true"),
		DisplayLanguage:     p.first("displayLanguage"),
	}
	if inline := p.inlineValueSet("valueSet"); inline != nil {
		out.Inline = inline
	}
	if raw := p.first("offset"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return out, &terminology.OutcomeError{Status: 400, Code: "invalid", Text: "Parameter 'offset' must be an integer"}
		}
		out.Offset = &v
	}
	if raw := p.first("count"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return out, &terminology.OutcomeError{Status: 400, Code: "invalid", Text: "Parameter 'count' must be an integer"}
		}
		out.Count = &v
	}
	return out, nil
}

func parseValidateValueSetCodeParams(p *requestParams, id string) terminology.ValidateValueSetCodeParams {
	return terminology.ValidateValueSetCodeParams{
		URL: p.first("url"), ID: id, Inline: p.inlineValueSet("valueSet"),
		Code: p.first("code"), System: p.first("system"), Display: p.first("display"),
		Coding: p.coding("coding"), CodeableConcept: p.codeableConceptCodings("codeableConcept"),
		ActiveOnly: strings.EqualFold(p.first("activeOnly"), "true"),
	}
}
