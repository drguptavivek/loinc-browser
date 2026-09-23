// Package fhirhttp adapts pkg/terminology's Mode A API to HTTP: GET query parameters and POST
// FHIR Parameters bodies in, application/fhir+json out (docs/FHIR_TERMINOLOGY_PLAN.md §4).
package fhirhttp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"loinc-browser/pkg/terminology"
)

// maxRequestBodySize bounds a POSTed Parameters/ValueSet body (§4.0). Every operation this
// package serves takes a handful of short fields plus at most one small inline compose; 4MB is
// generous headroom for that.
const maxRequestBodySize = 4 << 20

// rawParameters mirrors the wire shape of a POSTed FHIR Parameters resource, loosely typed so it
// accepts a valueX of any type without per-operation parsing code.
type rawParameters struct {
	ResourceType string     `json:"resourceType"`
	Parameter    []rawParam `json:"parameter"`
}

type rawParam struct {
	Name string `json:"name"`

	ValueCode            *string    `json:"valueCode"`
	ValueString          *string    `json:"valueString"`
	ValueBoolean         *bool      `json:"valueBoolean"`
	ValueInteger         *int       `json:"valueInteger"`
	ValueUri             *string    `json:"valueUri"`
	ValueCoding          *rawCoding `json:"valueCoding"`
	ValueCodeableConcept *struct {
		Coding []rawCoding `json:"coding"`
	} `json:"valueCodeableConcept"`

	// Resource carries a POSTed inline resource (e.g. `valueSet`'s ValueSet body, §4.7.1), which
	// FHIR encodes as parameter.resource rather than any value[x].
	Resource json.RawMessage `json:"resource"`

	Part []rawParam `json:"part"`
}

type rawCoding struct {
	System  string `json:"system"`
	Code    string `json:"code"`
	Display string `json:"display"`
}

func (p rawParam) stringValue() string {
	switch {
	case p.ValueCode != nil:
		return *p.ValueCode
	case p.ValueString != nil:
		return *p.ValueString
	case p.ValueUri != nil:
		return *p.ValueUri
	}
	return ""
}

func (p rawParam) coding() *terminology.Coding {
	if p.ValueCoding == nil {
		return nil
	}
	return &terminology.Coding{System: p.ValueCoding.System, Code: p.ValueCoding.Code, Display: p.ValueCoding.Display}
}

// requestParams is a name -> values reader that works identically whether the request was a GET
// with query parameters or a POST with a Parameters body, so each operation's param-building
// code (below) is written once.
type requestParams struct {
	query url.Values
	body  []rawParam
}

// readParams reads a GET's query parameters or a POST's Parameters/ValueSet body. w bounds the
// POST body to maxRequestBodySize via http.MaxBytesReader (§4.0, item 9): unlike io.LimitReader,
// which would just silently hand back a truncated (and then invalid-JSON) body, MaxBytesReader
// closes the connection and reports the overrun, which readParams turns into an explicit 413
// "too-costly" OperationOutcome instead of a generic 400.
func readParams(w http.ResponseWriter, r *http.Request) (*requestParams, *terminology.OutcomeError) {
	if r.Method == http.MethodGet {
		return &requestParams{query: r.URL.Query()}, nil
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, &terminology.OutcomeError{Status: 413, Code: "too-costly", Text: "Request body exceeds the maximum allowed size"}
		}
		return nil, &terminology.OutcomeError{Status: 400, Code: "invalid", Text: "Unable to read request body"}
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return &requestParams{}, nil
	}
	var parsed rawParameters
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, &terminology.OutcomeError{Status: 400, Code: "invalid", Text: "Request body is not a valid Parameters resource"}
	}
	return &requestParams{body: parsed.Parameter}, nil
}

// first returns the first value of name from a GET query, or the stringValue() of the first
// matching POST parameter.
func (p *requestParams) first(name string) string {
	if p.query != nil {
		values := p.query[name]
		if len(values) > 0 {
			return values[0]
		}
		return ""
	}
	for _, param := range p.body {
		if param.Name == name {
			return param.stringValue()
		}
	}
	return ""
}

// all returns every value of a repeating parameter (e.g. `property`).
func (p *requestParams) all(name string) []string {
	if p.query != nil {
		return p.query[name]
	}
	var out []string
	for _, param := range p.body {
		if param.Name == name {
			out = append(out, param.stringValue())
		}
	}
	return out
}

// coding returns the terminology.Coding carried by a `coding` parameter, GET or POST. GET never
// carries a structured Coding, so this is POST-only by construction.
func (p *requestParams) coding(name string) *terminology.Coding {
	if p.query != nil {
		return nil
	}
	for _, param := range p.body {
		if param.Name == name {
			return param.coding()
		}
	}
	return nil
}

// inlineValueSet returns the ValueSet.compose carried by a POSTed inline resource parameter (the
// `valueSet` parameter of $expand / ValueSet $validate-code, §4.7.1), or nil when absent, not a
// POST, or the resource has no compose. Only compose is evaluated; every other ValueSet field on
// an inline resource is ignored.
func (p *requestParams) inlineValueSet(name string) *terminology.InlineValueSet {
	if p.query != nil {
		return nil
	}
	for _, param := range p.body {
		if param.Name != name || len(param.Resource) == 0 {
			continue
		}
		var resource struct {
			Compose *terminology.ValueSetCompose `json:"compose"`
		}
		if err := json.Unmarshal(param.Resource, &resource); err != nil || resource.Compose == nil {
			return nil
		}
		return &terminology.InlineValueSet{Compose: resource.Compose}
	}
	return nil
}

// codeableConceptCodings returns every Coding inside a `codeableConcept` parameter.
func (p *requestParams) codeableConceptCodings(name string) []terminology.Coding {
	if p.query != nil {
		return nil
	}
	for _, param := range p.body {
		if param.Name == name && param.ValueCodeableConcept != nil {
			out := make([]terminology.Coding, 0, len(param.ValueCodeableConcept.Coding))
			for _, c := range param.ValueCodeableConcept.Coding {
				out = append(out, terminology.Coding{System: c.System, Code: c.Code, Display: c.Display})
			}
			return out
		}
	}
	return nil
}

func parseLookupParams(p *requestParams) terminology.LookupParams {
	return terminology.LookupParams{
		Code:            p.first("code"),
		System:          p.first("system"),
		Version:         p.first("version"),
		Coding:          p.coding("coding"),
		DisplayLanguage: p.first("displayLanguage"),
		Property:        p.all("property"),
	}
}

func parseValidateCodeParams(p *requestParams) terminology.ValidateCodeParams {
	return terminology.ValidateCodeParams{
		URL:             p.first("url"),
		System:          p.first("system"),
		Code:            p.first("code"),
		Version:         p.first("version"),
		Display:         p.first("display"),
		Coding:          p.coding("coding"),
		CodeableConcept: p.codeableConceptCodings("codeableConcept"),
		DisplayLanguage: p.first("displayLanguage"),
	}
}

func parseSubsumesParams(p *requestParams) terminology.SubsumesParams {
	return terminology.SubsumesParams{
		CodeA:   p.first("codeA"),
		CodeB:   p.first("codeB"),
		System:  p.first("system"),
		Version: p.first("version"),
		CodingA: p.coding("codingA"),
		CodingB: p.coding("codingB"),
	}
}
