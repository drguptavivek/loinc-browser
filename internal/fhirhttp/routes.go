package fhirhttp

import (
	"net/http"
	"strings"

	"loinc-browser/pkg/terminology"
)

// RouteEntry is fhirhttp's route-table shape. Register builds the base CodeSystem/metadata
// routes this file owns, then appends any entries queued by AppendRoutes, so a later package
// (P2's valueset.go, P3's conceptmap.go/questionnaire.go) can add its own routes by calling
// AppendRoutes from an init() in this package, without editing this file's Register function.
type RouteEntry struct {
	Method  string // "GET" or "POST"
	Pattern string // the path pattern passed to http.ServeMux, e.g. "/fhir/ValueSet/{id}"
	Handler http.HandlerFunc
}

var extraRoutes []RouteEntry

// AppendRoutes queues additional route entries for the next Register call. Call it from an
// init() in a sibling file within this package (see RouteEntry).
func AppendRoutes(entries ...RouteEntry) {
	extraRoutes = append(extraRoutes, entries...)
}

// Register wires every fhirhttp route onto mux: GET /fhir/metadata (+ ?mode=terminology),
// CodeSystem search/read, and $lookup/$validate-code/$subsumes on both
// /fhir/CodeSystem/$op and /fhir/CodeSystem/{id}/$op, GET and POST (§1).
//
// Go 1.22's http.ServeMux treats "{id}" as a wildcard and "$lookup" as a literal segment, but
// they occupy the same path position and would collide if registered as separate patterns. So
// CodeSystem's operation routes are one handler on "/fhir/CodeSystem/{rest...}" that inspects
// the trailing segment(s) itself and dispatches, rather than trying to register overlapping
// patterns for "{id}" and "{id}/$op" and "$op".
func Register(mux *http.ServeMux, svc *terminology.Service) {
	for _, entry := range baseRoutes(svc) {
		mux.HandleFunc(entry.Method+" "+entry.Pattern, entry.Handler)
	}
	// extraRoutes (queued by other files' init(), e.g. valueset.go) are built before any Service
	// exists, so they resolve svc per request through a package-level pointer rather than a
	// closure; set it here so it's ready before the routes below can be hit.
	RegisterValueSetService(svc)
	RegisterConceptMapService(svc)
	for _, entry := range extraRoutes {
		mux.HandleFunc(entry.Method+" "+entry.Pattern, entry.Handler)
	}
}

func baseRoutes(svc *terminology.Service) []RouteEntry {
	codeSystemDispatch := codeSystemHandler(svc)
	return []RouteEntry{
		{Method: "GET", Pattern: "/fhir/metadata", Handler: metadataHandler(svc)},
		{Method: "GET", Pattern: "/fhir/CodeSystem", Handler: codeSystemSearchHandler(svc)},
		{Method: "GET", Pattern: "/fhir/CodeSystem/{rest...}", Handler: codeSystemDispatch},
		{Method: "POST", Pattern: "/fhir/CodeSystem/{rest...}", Handler: codeSystemDispatch},
	}
}

// metadataHandler serves GET /fhir/metadata and, with ?mode=terminology, the
// TerminologyCapabilities variant (§4.1).
func metadataHandler(svc *terminology.Service) http.HandlerFunc {
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
		if strings.EqualFold(r.URL.Query().Get("mode"), "terminology") {
			resource, err := svc.TerminologyCapabilities(r.Context())
			if err != nil {
				writeOutcomeError(w, err)
				return
			}
			writeFHIRSummary(w, http.StatusOK, resource, filter)
			return
		}
		resource, err := svc.Capabilities(r.Context())
		if err != nil {
			writeOutcomeError(w, err)
			return
		}
		writeFHIRSummary(w, http.StatusOK, resource, filter)
	}
}

// codeSystemSearchHandler serves GET /fhir/CodeSystem?url=...&version=... (§4.2).
func codeSystemSearchHandler(svc *terminology.Service) http.HandlerFunc {
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
		bundle, err := svc.SearchCodeSystem(r.Context(), query.Get("url"), query.Get("version"))
		if err != nil {
			writeOutcomeError(w, err)
			return
		}
		// SearchCodeSystem never returns more than one entry (only "loinc" is served), so _count/
		// _offset never change the result set; they still drive the self/next Bundle.link (§4.0).
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

// codeSystemHandler serves GET/POST /fhir/CodeSystem/{rest...}, where rest is either an id
// ("loinc", "loinc-2.82"), an operation ("$lookup", "$validate-code", "$subsumes"), or
// "{id}/{operation}".
func codeSystemHandler(svc *terminology.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
			resource, err := svc.CodeSystemResource(r.Context(), id)
			if err != nil {
				writeOutcomeError(w, err)
				return
			}
			writeFHIRSummary(w, http.StatusOK, resource, filter)
			return
		}
		dispatchCodeSystemOperation(w, r, svc, op)
	}
}

func dispatchCodeSystemOperation(w http.ResponseWriter, r *http.Request, svc *terminology.Service, op string) {
	params, paramErr := readParams(w, r)
	if paramErr != nil {
		writeOutcomeError(w, paramErr)
		return
	}
	ctx := r.Context()
	switch op {
	case "$lookup":
		result, err := svc.Lookup(ctx, parseLookupParams(params))
		writeOperationResult(w, result, err)
	case "$validate-code":
		result, err := svc.ValidateCode(ctx, parseValidateCodeParams(params))
		writeOperationResult(w, result, err)
	case "$subsumes":
		result, err := svc.Subsumes(ctx, parseSubsumesParams(params))
		writeOperationResult(w, result, err)
	default:
		writeOutcome(w, http.StatusNotFound, "not-found", "Unknown operation "+op)
	}
}

func writeOperationResult(w http.ResponseWriter, result *terminology.Parameters, err *terminology.OutcomeError) {
	if err != nil {
		writeOutcomeError(w, err)
		return
	}
	writeFHIR(w, http.StatusOK, result)
}
