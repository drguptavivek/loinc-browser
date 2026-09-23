package terminology

import (
	"context"
	"strings"
)

// ValidateCodeParams is CodeSystem $validate-code's input (§4.4).
type ValidateCodeParams struct {
	URL             string // `url`, falling back to `system`
	System          string
	Code            string
	Version         string
	Display         string
	Coding          *Coding
	CodeableConcept []Coding // the first LOINC coding wins
	DisplayLanguage string
}

// resolvedCode picks the code to validate: an explicit `code`, else the first LOINC coding from
// `coding`/`codeableConcept`.
func (p ValidateCodeParams) resolvedCode() string {
	if strings.TrimSpace(p.Code) != "" {
		return p.Code
	}
	if p.Coding != nil {
		return p.Coding.Code
	}
	for _, c := range p.CodeableConcept {
		if strings.EqualFold(strings.TrimSpace(c.System), loincSystem) || c.System == "" {
			return c.Code
		}
	}
	return ""
}

func (p ValidateCodeParams) resolvedSystem() string {
	if strings.TrimSpace(p.URL) != "" {
		return p.URL
	}
	return p.System
}

// ValidateCode implements CodeSystem $validate-code (§4.4). It always answers HTTP 200: failures
// are reported through the `result`/`message` parameters, never an OperationOutcome, except for
// request-shape errors (missing code) and a store that is unavailable.
func (s *Service) ValidateCode(ctx context.Context, params ValidateCodeParams) (*Parameters, *OutcomeError) {
	store, outcomeErr := s.store()
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	version, outcomeErr := s.releaseVersion(ctx, store)
	if outcomeErr != nil {
		return nil, outcomeErr
	}

	code := strings.TrimSpace(params.resolvedCode())
	if code == "" {
		return nil, requiredError("Parameter 'code' or 'coding' is required")
	}
	code = normalizeCode(code)
	system := params.resolvedSystem()
	if system != "" && !strings.EqualFold(strings.TrimSpace(system), loincSystem) {
		return falseResult("The code does not exist for the supplied code system and/or version", "", version), nil
	}
	if params.Version != "" && !strings.HasPrefix(version, strings.TrimSpace(params.Version)) {
		return falseResult("The code does not exist for the supplied code system and/or version", "", version), nil
	}

	lookup, err := s.Lookup(ctx, LookupParams{Code: code, DisplayLanguage: params.DisplayLanguage})
	if err != nil {
		if err.Code == "not-found" {
			return falseResult("The code does not exist for the supplied code system and/or version", "", version), nil
		}
		return nil, err
	}

	display := paramValueString(lookup.Parameter, "display")
	status := paramValueCode(lookup.Parameter, "status")
	active := status != "retired"

	if display != "" && strings.TrimSpace(params.Display) != "" && !designationMatchesDisplay(lookup.Parameter, params.Display) {
		return &Parameters{Parameter: []Parameter{
			paramString("result", "false"),
			paramString("message", "The code exists but the display is not valid"),
			paramString("display", display),
			paramBool("active", active),
			paramString("system", loincSystem),
			paramString("version", version),
		}}, nil
	}

	return &Parameters{Parameter: []Parameter{
		paramString("result", "true"),
		paramString("code", code),
		paramString("display", display),
		paramBool("active", active),
		paramString("system", loincSystem),
		paramString("version", version),
	}}, nil
}

func falseResult(message, display, version string) *Parameters {
	params := []Parameter{
		paramString("result", "false"),
		paramString("message", message),
	}
	if display != "" {
		params = append(params, paramString("display", display))
	}
	params = append(params, paramString("system", loincSystem), paramString("version", version))
	return &Parameters{Parameter: params}
}

// designationMatchesDisplay reports whether display case-insensitively equals any designation
// value in the $lookup result (§4.4: "not case-insensitively equal to any designation value").
func designationMatchesDisplay(params []Parameter, display string) bool {
	display = strings.TrimSpace(display)
	for _, p := range params {
		if p.Name != "designation" {
			continue
		}
		for _, part := range p.Part {
			if part.Name == "value" && part.ValueString != nil && strings.EqualFold(strings.TrimSpace(*part.ValueString), display) {
				return true
			}
		}
	}
	return false
}

func paramValueString(params []Parameter, name string) string {
	for _, p := range params {
		if p.Name == name && p.ValueString != nil {
			return *p.ValueString
		}
	}
	return ""
}

func paramValueCode(params []Parameter, name string) string {
	for _, p := range params {
		if p.Name == name && p.ValueCode != nil {
			return *p.ValueCode
		}
	}
	return ""
}
