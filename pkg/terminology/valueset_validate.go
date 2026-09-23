package terminology

import (
	"context"
	"strings"

	"loinc-browser/internal/loinc"
)

// ValidateValueSetCodeParams is ValueSet $validate-code's input (§4.8).
type ValidateValueSetCodeParams struct {
	URL             string
	ID              string
	Inline          *InlineValueSet
	Code            string
	System          string
	Display         string
	Coding          *Coding
	CodeableConcept []Coding
	ActiveOnly      bool
}

func (p ValidateValueSetCodeParams) resolvedCode() string {
	if strings.TrimSpace(p.Code) != "" {
		return p.Code
	}
	if p.Coding != nil {
		return p.Coding.Code
	}
	for _, c := range p.CodeableConcept {
		if c.System == "" || strings.EqualFold(strings.TrimSpace(c.System), loincSystem) {
			return c.Code
		}
	}
	return ""
}

// ValidateValueSetCode implements ValueSet $validate-code (§4.8). Note valueBoolean here, unlike
// CodeSystem $validate-code's valueString. An unknown value set is a 404 OperationOutcome;
// everything else is reported through result/message at HTTP 200.
func (s *Service) ValidateValueSetCode(ctx context.Context, params ValidateValueSetCodeParams) (*Parameters, *OutcomeError) {
	store, outcomeErr := s.store()
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	version, outcomeErr := s.releaseVersion(ctx, store)
	if outcomeErr != nil {
		return nil, outcomeErr
	}

	var resolved *resolvedValueSet
	switch {
	case params.Inline != nil:
		resolved, outcomeErr = s.resolveInlineValueSet(ctx, store, params.Inline)
	case params.ID != "":
		resolved, outcomeErr = s.resolveValueSet(ctx, store, strings.TrimSuffix(params.ID, "-"+version))
	case params.URL != "":
		id := idFromURL(params.URL)
		if id == "" {
			outcomeErr = notFoundError("Failed to find matching value set")
		} else {
			resolved, outcomeErr = s.resolveValueSet(ctx, store, id)
		}
	default:
		outcomeErr = requiredError("Parameter 'url' or 'valueSet' is required")
	}
	if outcomeErr != nil {
		return nil, outcomeErr
	}

	code := strings.TrimSpace(params.resolvedCode())
	if code == "" {
		return nil, requiredError("Parameter 'code' or 'coding' is required")
	}
	code = normalizeCode(code)

	found, display, err := s.valueSetContainsCode(ctx, store, resolved, code, params.ActiveOnly)
	if err != nil {
		return nil, &OutcomeError{Status: 503, Code: "exception", Text: err.Error()}
	}
	if !found {
		return &Parameters{Parameter: []Parameter{
			paramBool("result", false),
			paramString("message", "The code '"+code+"' was not found in this value set"),
		}}, nil
	}
	if display != "" && strings.TrimSpace(params.Display) != "" && !strings.EqualFold(strings.TrimSpace(params.Display), display) {
		return &Parameters{Parameter: []Parameter{
			paramBool("result", false),
			paramString("message", "The code exists but the display is not valid"),
			paramString("display", display),
		}}, nil
	}
	return &Parameters{Parameter: []Parameter{
		paramBool("result", true),
		paramString("display", display),
	}}, nil
}

// valueSetContainsCode tests membership of one code (§4.8), pushed to SQL rather than paging the
// whole set: the resolved source's WHERE clause gains "and t.loinc_num = ?" (or the LL/LA
// equivalent) and is run as an existence + display lookup.
func (s *Service) valueSetContainsCode(ctx context.Context, store *loinc.Store, r *resolvedValueSet, code string, activeOnly bool) (bool, string, error) {
	if r.answerListID != "" {
		items, err := store.FHIRAnswerListAnswers(ctx, r.answerListID)
		if err != nil {
			return false, "", err
		}
		for _, a := range items {
			if strings.EqualFold(a.AnswerStringID, code) {
				return true, a.DisplayText, nil
			}
		}
		return false, "", nil
	}
	src := loinc.FHIRTermValueSetSource{
		From:  r.termSource.From,
		Where: r.termSource.Where + " and t.loinc_num = ?",
		Args:  append(append([]any{}, r.termSource.Args...), code),
	}
	total, items, err := store.FHIRExpandTermSource(ctx, src, "", activeOnly, 0, 1)
	if err != nil {
		return false, "", err
	}
	if total == 0 || len(items) == 0 {
		return false, "", nil
	}
	return true, items[0].Display, nil
}
