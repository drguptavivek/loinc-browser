package fhirhttp

import (
	"net/http"
	"strconv"
	"strings"

	"loinc-browser/pkg/terminology"
)

// init registers ConceptMap's routes onto the shared route table built by routes.go's Register
// (see RouteEntry): search-type, read, and $translate on both /fhir/ConceptMap/$translate and
// /fhir/ConceptMap/{id}/$translate.
func init() {
	AppendRoutes(
		RouteEntry{Method: "GET", Pattern: "/fhir/ConceptMap", Handler: conceptMapSearchHandlerFunc},
		RouteEntry{Method: "GET", Pattern: "/fhir/ConceptMap/{rest...}", Handler: conceptMapHandlerFunc},
		RouteEntry{Method: "POST", Pattern: "/fhir/ConceptMap/{rest...}", Handler: conceptMapHandlerFunc},
	)
}

// currentConceptMapService is the terminology.Service the routes queued by this file's init()
// dispatch to (also used by questionnaire.go). AppendRoutes queues route entries at package
// init(), before any Service exists, so — unlike baseRoutes' handlers, which close over svc
// directly — these resolve it lazily through this package-level pointer, set by Register
// (routes.go) before serving traffic; see valueset.go's currentValueSetService for the same
// pattern.
var currentConceptMapService *terminology.Service

// RegisterConceptMapService supplies the terminology.Service this file's and questionnaire.go's
// routes dispatch to.
func RegisterConceptMapService(svc *terminology.Service) {
	currentConceptMapService = svc
}

func conceptMapSearchHandlerFunc(w http.ResponseWriter, r *http.Request) {
	conceptMapSearchHandler(currentConceptMapService)(w, r)
}

func conceptMapHandlerFunc(w http.ResponseWriter, r *http.Request) {
	conceptMapHandler(currentConceptMapService)(w, r)
}

// conceptMapSearchHandler serves GET /fhir/ConceptMap?url=...&source-system=...&... (§4.9).
func conceptMapSearchHandler(svc *terminology.Service) http.HandlerFunc {
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
		// Mirror SearchConceptMaps' own clampSearchPage defaults (20, capped at 100, offset>=0)
		// so the self/next Bundle.link this handler builds match the page it actually served.
		count := parseIntOr(query.Get("_count"), 20)
		if count <= 0 {
			count = 20
		}
		if count > 100 {
			count = 100
		}
		offset := parseIntOr(query.Get("_offset"), 0)
		if offset < 0 {
			offset = 0
		}
		params := terminology.ConceptMapSearchParams{
			URL:          query.Get("url"),
			SourceSystem: query.Get("source-system"),
			TargetSystem: query.Get("target-system"),
			SourceCode:   query.Get("source-code"),
			TargetCode:   query.Get("target-code"),
			Count:        count,
			Offset:       offset,
		}
		bundle, err := svc.SearchConceptMaps(r.Context(), params)
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

// conceptMapHandler serves GET/POST /fhir/ConceptMap/{rest...}, where rest is either an id, the
// "$translate" operation, or "{id}/$translate" — the same dispatch shape as CodeSystem's
// {rest...} handler in routes.go, needed because ServeMux cannot register colliding "{id}" and
// "$op" patterns.
func conceptMapHandler(svc *terminology.Service) http.HandlerFunc {
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
			resource, err := svc.ReadConceptMap(r.Context(), id)
			if err != nil {
				writeOutcomeError(w, err)
				return
			}
			writeFHIRSummary(w, http.StatusOK, resource, filter)
			return
		}
		if op != "$translate" {
			writeOutcome(w, http.StatusNotFound, "not-found", "Unknown operation "+op)
			return
		}
		params, paramErr := readParams(w, r)
		if paramErr != nil {
			writeOutcomeError(w, paramErr)
			return
		}
		translateParams := parseTranslateParams(params)
		translateParams.ConceptMapID = id
		result, err := svc.Translate(r.Context(), translateParams)
		if err != nil {
			writeOutcomeError(w, err)
			return
		}
		writeFHIR(w, http.StatusOK, result)
	}
}

func parseTranslateParams(p *requestParams) terminology.TranslateParams {
	var codeableConcept []terminology.Coding
	if ccs := p.codeableConceptCodings("codeableConcept"); len(ccs) > 0 {
		codeableConcept = ccs
	}
	return terminology.TranslateParams{
		URL:             p.first("url"),
		Code:            p.first("code"),
		System:          p.first("system"),
		Version:         p.first("version"),
		Coding:          p.coding("coding"),
		CodeableConcept: codeableConcept,
		Target:          p.first("target"),
		TargetSystem:    p.first("targetsystem"),
		Reverse:         strings.EqualFold(p.first("reverse"), "true"),
	}
}

func parseIntOr(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}
