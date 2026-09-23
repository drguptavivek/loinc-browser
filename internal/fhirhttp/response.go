package fhirhttp

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"loinc-browser/pkg/terminology"
)

const fhirContentType = "application/fhir+json;charset=UTF-8"

// writeFHIR writes any FHIR resource as application/fhir+json (§4.0).
func writeFHIR(w http.ResponseWriter, status int, resource any) {
	w.Header().Set("Content-Type", fhirContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resource)
}

// operationOutcome is the wire shape of an OperationOutcome (§4.12):
// {"resourceType":"OperationOutcome","issue":[{"severity":"error","code":...,"details":{"text":...},"diagnostics":...}]}.
type operationOutcome struct {
	ResourceType string         `json:"resourceType"`
	Issue        []outcomeIssue `json:"issue"`
}

type outcomeIssue struct {
	Severity    string         `json:"severity"`
	Code        string         `json:"code"`
	Details     outcomeDetails `json:"details"`
	Diagnostics string         `json:"diagnostics"`
}

type outcomeDetails struct {
	Text string `json:"text"`
}

// writeOutcomeError renders a *terminology.OutcomeError as its OperationOutcome (§4.12).
func writeOutcomeError(w http.ResponseWriter, err *terminology.OutcomeError) {
	writeFHIR(w, err.Status, operationOutcome{
		ResourceType: "OperationOutcome",
		Issue: []outcomeIssue{{
			Severity:    "error",
			Code:        err.Code,
			Details:     outcomeDetails{Text: err.Text},
			Diagnostics: err.Text,
		}},
	})
}

// writeOutcome renders any code/status pair fhirhttp needs outside pkg/terminology as an
// OperationOutcome (for example a 406 for an XML-only Accept header, or a 404 for an unknown
// operation segment).
func writeOutcome(w http.ResponseWriter, status int, code string, text string) {
	writeOutcomeError(w, &terminology.OutcomeError{Status: status, Code: code, Text: text})
}

// searchBundleLinks builds Bundle.link[self] (always) and Bundle.link[next] (when more results
// remain), matching upstream's _count/_offset paging (§4.0, valueset-search-name-yes.json).
// Shared by every search-type Bundle this package serves (ValueSet, CodeSystem, ConceptMap,
// Questionnaire) so paging behaves identically across resources.
func searchBundleLinks(r *http.Request, total, offset, count int) []terminology.BundleLink {
	links := []terminology.BundleLink{{Relation: "self", URL: requestURL(r, r.URL.Query())}}
	if offset+count < total {
		next := cloneQuery(r.URL.Query())
		next.Set("_offset", strconv.Itoa(offset+count))
		next.Set("_count", strconv.Itoa(count))
		links = append(links, terminology.BundleLink{Relation: "next", URL: requestURL(r, next)})
	}
	return links
}

func cloneQuery(q url.Values) url.Values {
	out := make(url.Values, len(q))
	for k, v := range q {
		out[k] = append([]string{}, v...)
	}
	return out
}

// requestURL rebuilds an absolute URL for r with the given query, honouring
// X-Forwarded-Proto/-Host (§4.0) ahead of the request's own scheme/host.
func requestURL(r *http.Request, query url.Values) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = forwarded
	}
	host := r.Host
	if forwarded := r.Header.Get("X-Forwarded-Host"); forwarded != "" {
		host = forwarded
	}
	u := url.URL{Scheme: scheme, Host: host, Path: r.URL.Path, RawQuery: query.Encode()}
	return u.String()
}

// wantsXML reports whether the request asked for XML via `_format=xml` or an XML-only Accept
// header (§4.0): both get a 406 OperationOutcome rather than being silently ignored.
func wantsXML(r *http.Request) bool {
	if strings.EqualFold(r.URL.Query().Get("_format"), "xml") {
		return true
	}
	accept := r.Header.Get("Accept")
	if accept == "" || accept == "*/*" {
		return false
	}
	for _, part := range strings.Split(accept, ",") {
		part = strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		switch part {
		case "application/fhir+json", "application/json", "*/*", "":
			return false
		}
	}
	// Every media range named something other than a JSON one; treat as XML-only.
	for _, part := range strings.Split(accept, ",") {
		part = strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		if strings.Contains(part, "xml") {
			return true
		}
	}
	return false
}
