package terminology

import (
	"context"
	"strings"

	"loinc-browser/internal/loinc"
)

// TranslateParams is ConceptMap $translate's input (§4.10). Exactly one of URL/ConceptMapID picks
// a specific map; when both are empty, System drives a search across every map whose source
// system equals it.
type TranslateParams struct {
	URL             string
	ConceptMapID    string // the path {id} of /fhir/ConceptMap/{id}/$translate
	Code            string
	System          string
	Version         string
	Coding          *Coding
	CodeableConcept []Coding
	Target          string
	TargetSystem    string
	Reverse         bool
}

// resolvedCode picks the code to translate from Code, else the first coding's code (an explicit
// `coding` parameter, else the first `codeableConcept` coding), also resolving system from the
// same source when System is not already set.
func (p TranslateParams) resolvedCode() (code, system string) {
	code = strings.TrimSpace(p.Code)
	system = strings.TrimSpace(p.System)
	if code != "" {
		return code, system
	}
	if p.Coding != nil && p.Coding.Code != "" {
		code = p.Coding.Code
		if system == "" {
			system = p.Coding.System
		}
		return code, system
	}
	for _, c := range p.CodeableConcept {
		if c.Code != "" {
			code = c.Code
			if system == "" {
				system = c.System
			}
			return code, system
		}
	}
	return "", system
}

// Translate implements ConceptMap $translate (§4.10): without a url/id, it searches every served
// map whose source system equals the given system, in catalogue order, and collects every match.
func (s *Service) Translate(ctx context.Context, params TranslateParams) (*Parameters, *OutcomeError) {
	store, outcomeErr := s.store()
	if outcomeErr != nil {
		return nil, outcomeErr
	}

	code, system := params.resolvedCode()
	code = normalizeCode(code)
	if code == "" {
		return nil, requiredError("Parameter 'code' or 'coding' is required")
	}

	id := strings.TrimSpace(params.ConceptMapID)
	if id == "" && params.URL != "" {
		id = conceptMapIDFromURL(params.URL)
	}

	var descriptors []conceptMapDescriptor
	if id != "" || params.URL != "" {
		lookupID := id
		if lookupID == "" {
			lookupID = strings.TrimSpace(params.URL)
		}
		d, ok := resolveConceptMapID(lookupID)
		if !ok {
			return nil, notFoundError("Code system not found matching 'system' parameter")
		}
		if params.Reverse {
			d = d.reversed()
			if d.ID() == "" {
				return nil, notFoundError("Code system not found matching 'system' parameter")
			}
		}
		descriptors = []conceptMapDescriptor{d}
	} else {
		if system == "" {
			return nil, requiredError("Parameter 'system' is required when 'url' is not given")
		}
		descriptors = descriptorsForSourceSystem(system)
		if params.Reverse {
			flipped := make([]conceptMapDescriptor, 0, len(descriptors))
			for _, d := range descriptors {
				if rd := d.reversed(); rd.ID() != "" {
					flipped = append(flipped, rd)
				}
			}
			descriptors = flipped
		}
		if len(descriptors) == 0 {
			return nil, notFoundError("Code system not found matching 'system' parameter")
		}
	}

	var matches []Parameter
	for _, d := range descriptors {
		parts, err := s.translateMatches(ctx, store, d, code)
		if err != nil {
			return nil, &OutcomeError{Status: 503, Code: "exception", Text: err.Error()}
		}
		matches = append(matches, parts...)
	}

	if len(matches) == 0 {
		return &Parameters{Parameter: []Parameter{
			paramBool("result", false),
			paramString("message", "No mapping found matching specified criteria"),
		}}, nil
	}
	out := append([]Parameter{paramBool("result", true)}, matches...)
	return &Parameters{Parameter: out}, nil
}

// translateMatches builds the "match" Parameters for one candidate map (§4.10): equivalence,
// concept (the other side's Coding), source (the map's url), plus a comment part for
// loinc-map-to when the row carries one.
func (s *Service) translateMatches(ctx context.Context, store *loinc.Store, d conceptMapDescriptor, code string) ([]Parameter, error) {
	rows, err := s.conceptMapRows(ctx, store, d, code, 0)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	mapURL := loincSystem + "/cm/" + d.ID()
	out := make([]Parameter, 0, len(rows))
	for _, row := range rows {
		targetCode, targetDisplay := row.TargetCode, row.TargetDisplay
		if d.Reversed {
			targetCode, targetDisplay = row.SourceCode, row.SourceDisplay
		}
		parts := []Parameter{
			paramCode("equivalence", rowEquivalence(d, row)),
			paramCoding("concept", Coding{System: d.TargetURI(), Code: targetCode, Display: targetDisplay}),
			paramURI("source", mapURL),
		}
		if row.Comment != "" {
			parts = append(parts, paramString("comment", row.Comment))
		}
		out = append(out, paramPart("match", parts...))
	}
	return out, nil
}
