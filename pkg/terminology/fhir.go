// Package terminology is the Mode A public API for local LOINC FHIR terminology lookups
// (docs/FHIR_TERMINOLOGY_PLAN.md). It serves entirely from the local normalized SQLite database;
// it never calls fhir.loinc.org or any other network service.
package terminology

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Coding is a minimal FHIR Coding, used for every valueCoding in this package.
type Coding struct {
	System  string `json:"system,omitempty"`
	Code    string `json:"code,omitempty"`
	Display string `json:"display,omitempty"`
}

// Parameter is one FHIR Parameters.parameter (or .part) entry. Exactly one Value* field is set
// at a time; MarshalJSON emits exactly one "value{X}" key, matching upstream's wire shape.
type Parameter struct {
	Name string

	ValueCode    *string
	ValueString  *string
	ValueBoolean *bool
	ValueInteger *int
	ValueUri     *string
	ValueCoding  *Coding

	Part []Parameter
}

// MarshalJSON emits {"name":..., "value{X}":..., "part":[...]} in that order, with only the one
// value{X} key that is actually set — an explicit, ordered encoding rather than a map, so field
// order matches the exemplars byte-for-byte in shape.
func (p Parameter) MarshalJSON() ([]byte, error) {
	type kv struct {
		key string
		val any
	}
	entries := make([]kv, 0, 3)
	entries = append(entries, kv{"name", p.Name})
	switch {
	case p.ValueCode != nil:
		entries = append(entries, kv{"valueCode", *p.ValueCode})
	case p.ValueString != nil:
		entries = append(entries, kv{"valueString", *p.ValueString})
	case p.ValueBoolean != nil:
		entries = append(entries, kv{"valueBoolean", *p.ValueBoolean})
	case p.ValueInteger != nil:
		entries = append(entries, kv{"valueInteger", *p.ValueInteger})
	case p.ValueUri != nil:
		entries = append(entries, kv{"valueUri", *p.ValueUri})
	case p.ValueCoding != nil:
		entries = append(entries, kv{"valueCoding", p.ValueCoding})
	}
	if len(p.Part) > 0 {
		entries = append(entries, kv{"part", p.Part})
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, e := range entries {
		if i > 0 {
			buf.WriteByte(',')
		}
		keyJSON, err := json.Marshal(e.key)
		if err != nil {
			return nil, err
		}
		buf.Write(keyJSON)
		buf.WriteByte(':')
		valJSON, err := json.Marshal(e.val)
		if err != nil {
			return nil, err
		}
		buf.Write(valJSON)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// Parameters is a FHIR Parameters resource: the output shape of $lookup, $validate-code, and
// $subsumes.
type Parameters struct {
	Parameter []Parameter
}

// MarshalJSON emits {"resourceType":"Parameters","parameter":[...]}.
func (p Parameters) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ResourceType string      `json:"resourceType"`
		Parameter    []Parameter `json:"parameter,omitempty"`
	}{ResourceType: "Parameters", Parameter: p.Parameter})
}

func paramCode(name, value string) Parameter      { return Parameter{Name: name, ValueCode: &value} }
func paramString(name, value string) Parameter    { return Parameter{Name: name, ValueString: &value} }
func paramBool(name string, value bool) Parameter { return Parameter{Name: name, ValueBoolean: &value} }
func paramURI(name, value string) Parameter       { return Parameter{Name: name, ValueUri: &value} }
func paramCoding(name string, c Coding) Parameter { return Parameter{Name: name, ValueCoding: &c} }
func paramPart(name string, parts ...Parameter) Parameter {
	return Parameter{Name: name, Part: parts}
}

// loincCoding builds a Coding on the http://loinc.org system.
func loincCoding(code, display string) Coding {
	return Coding{System: loincSystem, Code: code, Display: display}
}

// OutcomeError is a typed FHIR error: an HTTP status plus an OperationOutcome issue
// (severity "error", the given code, and text used for both details.text and diagnostics), per
// §4.12. internal/fhirhttp renders it as the wire OperationOutcome.
type OutcomeError struct {
	Status int
	Code   string // not-found | invalid | required | not-supported | exception
	Text   string
}

func (e *OutcomeError) Error() string {
	return fmt.Sprintf("%s (%s): %s", e.Code, httpStatusText(e.Status), e.Text)
}

func httpStatusText(status int) string {
	switch status {
	case 400:
		return "400"
	case 404:
		return "404"
	case 406:
		return "406"
	case 503:
		return "503"
	default:
		return fmt.Sprintf("%d", status)
	}
}

func notFoundError(text string) *OutcomeError {
	return &OutcomeError{Status: 404, Code: "not-found", Text: text}
}

func invalidError(text string) *OutcomeError {
	return &OutcomeError{Status: 400, Code: "invalid", Text: text}
}

func requiredError(text string) *OutcomeError {
	return &OutcomeError{Status: 400, Code: "required", Text: text}
}

const loincSystem = "http://loinc.org"
