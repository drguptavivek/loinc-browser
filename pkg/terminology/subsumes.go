package terminology

import (
	"context"
	"errors"
	"strings"

	"loinc-browser/internal/loinc"
)

// SubsumesParams is CodeSystem $subsumes's input (§4.5).
type SubsumesParams struct {
	CodeA, CodeB     string
	System, Version  string
	CodingA, CodingB *Coding
}

func (p SubsumesParams) resolvedCodeA() string {
	if strings.TrimSpace(p.CodeA) != "" {
		return p.CodeA
	}
	if p.CodingA != nil {
		return p.CodingA.Code
	}
	return ""
}

func (p SubsumesParams) resolvedCodeB() string {
	if strings.TrimSpace(p.CodeB) != "" {
		return p.CodeB
	}
	if p.CodingB != nil {
		return p.CodingB.Code
	}
	return ""
}

// Subsumes implements CodeSystem $subsumes (§4.5): equivalent/subsumes/subsumed-by/not-subsumed,
// walking the Component Hierarchy by System.
func (s *Service) Subsumes(ctx context.Context, params SubsumesParams) (*Parameters, *OutcomeError) {
	store, outcomeErr := s.store()
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	version, outcomeErr := s.releaseVersion(ctx, store)
	if outcomeErr != nil {
		return nil, outcomeErr
	}

	codeA := strings.TrimSpace(params.resolvedCodeA())
	codeB := strings.TrimSpace(params.resolvedCodeB())
	if codeA == "" || codeB == "" {
		return nil, requiredError("Parameters 'codeA' and 'codeB' are required")
	}
	codeA = normalizeCode(codeA)
	codeB = normalizeCode(codeB)

	notFoundText := "Code system not found = " + loincSystem
	if err := checkSystemVersion(params.System, params.Version, version, notFoundText); err != nil {
		return nil, err
	}

	for _, code := range []string{codeA, codeB} {
		if err := s.assertCodeExists(ctx, store, code); err != nil {
			if errors.Is(err, loinc.ErrNotFound) {
				return nil, invalidError("Code does not exist for code system =" + code + "," + loincSystem)
			}
			return nil, &OutcomeError{Status: 503, Code: "exception", Text: err.Error()}
		}
	}

	outcome, err := store.FHIRSubsumes(ctx, codeA, codeB)
	if err != nil {
		return nil, &OutcomeError{Status: 503, Code: "exception", Text: err.Error()}
	}
	return &Parameters{Parameter: []Parameter{
		paramString("outcome", outcome),
		paramString("system", loincSystem),
		paramString("version", version),
	}}, nil
}

// assertCodeExists confirms a code resolves to one of the five known kinds, without building the
// full $lookup response. It delegates to codeResolves (lookup.go), the same per-kind resolution
// $lookup and $validate-code use, so a hierarchy-only LP node (in hierarchy_concepts but never
// published as a Part.csv row, e.g. LP31448-1) is accepted here too.
func (s *Service) assertCodeExists(ctx context.Context, store *loinc.Store, code string) error {
	return codeResolves(ctx, store, code)
}
