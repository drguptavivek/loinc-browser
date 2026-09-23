package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"loinc-browser/internal/loinc"
	"loinc-browser/pkg/terminology"
)

// loincSystemURL mirrors pkg/terminology's unexported loincSystem constant: the fixed
// http://loinc.org code system URL used to build example FHIR route URLs below.
const loincSystemURL = "http://loinc.org"

// termAxisCodes are the seven CodeSystem $lookup axis property codes (pkg/terminology's
// termAxisOrder), used to split loinc_lookup_code's "property" parameters into the compact
// Axes list versus everything else.
var termAxisCodes = map[string]bool{
	"SYSTEM": true, "TIME_ASPCT": true, "PROPERTY": true, "SCALE_TYP": true,
	"METHOD_TYP": true, "CLASS": true, "COMPONENT": true,
}

// termRelationCodes are the "property" codes that name a related code (parent/child/etc.) rather
// than a plain string attribute, split into loinc_lookup_code's Related list.
var termRelationCodes = map[string]bool{
	"parent": true, "child": true, "answers-for": true, "answer-list": true, "MAP_TO": true,
}

const maxRelated = 50
const maxDesignations = 20

var errTerminologyUnavailable = errors.New("FHIR terminology service is not available on this MCP transport")

// classifyCodeKind mirrors pkg/terminology's unexported classifyLoincCode: which of the five
// http://loinc.org code kinds a code belongs to, by prefix.
func classifyCodeKind(code string) string {
	switch {
	case strings.HasPrefix(code, "LP"):
		return "part"
	case strings.HasPrefix(code, "LL"):
		return "answer_list"
	case strings.HasPrefix(code, "LA"):
		return "answer"
	case strings.HasPrefix(code, "LG"):
		return "group"
	default:
		return "term"
	}
}

func browserURL(code string) string {
	return "/?term=" + url.QueryEscape(code)
}

func codeSystemOpURL(op, query string) string {
	return "/fhir/CodeSystem/" + op + "?" + query
}

func asError(oe *terminology.OutcomeError) error {
	if oe == nil {
		return nil
	}
	return oe
}

func paramStr(params []terminology.Parameter, name string) string {
	for _, p := range params {
		if p.Name == name && p.ValueString != nil {
			return *p.ValueString
		}
	}
	return ""
}

func paramBoolVal(params []terminology.Parameter, name string) bool {
	for _, p := range params {
		if p.Name == name && p.ValueBoolean != nil {
			return *p.ValueBoolean
		}
	}
	return false
}

// --- loinc_lookup_code ---

type LookupCodeRequest struct {
	Code         string   `json:"code" jsonschema:"Any LOINC code: a term number (e.g. 718-7), an LP part number (e.g. LP14559-6), an LL answer list ID (e.g. LL1162-8), an LA answer ID (e.g. LA6576-8), or an LG group ID."`
	Properties   []string `json:"properties,omitempty" jsonschema:"Optional property codes to filter the response to, e.g. [\"COMPONENT\",\"SYSTEM\"]."`
	Designations bool     `json:"designations,omitempty" jsonschema:"Include language designations (alternate names/translations) in the response."`
	Language     string   `json:"language,omitempty" jsonschema:"Only used with designations=true: filter designations to this BCP-47 language, e.g. es."`
	RawFHIR      bool     `json:"rawFhir,omitempty" jsonschema:"Return the full FHIR CodeSystem $lookup Parameters resource instead of the compact summary."`
}

type LookupAxis struct {
	Axis    string `json:"axis"`
	Code    string `json:"code"`
	Display string `json:"display,omitempty"`
}

type LookupRelated struct {
	Relation string `json:"relation"`
	Code     string `json:"code"`
	Display  string `json:"display,omitempty"`
}

type LookupCodeResult struct {
	Code             string            `json:"code"`
	Kind             string            `json:"kind"`
	Display          string            `json:"display,omitempty"`
	Status           string            `json:"status,omitempty"`
	Axes             []LookupAxis      `json:"axes,omitempty"`
	Properties       map[string]string `json:"properties,omitempty"`
	Related          []LookupRelated   `json:"related,omitempty"`
	RelatedTruncated bool              `json:"relatedTruncated,omitempty"`
	Designations     []string          `json:"designations,omitempty"`
	BrowserURL       string            `json:"browserUrl"`
	FHIRURL          string            `json:"fhirUrl"`
	// Raw holds the full FHIR Parameters resource when requested. Typed as any (not
	// *terminology.Parameters) because terminology.Parameter is self-referential (Part
	// []Parameter) and the MCP SDK's output-schema generator rejects a recursive struct type.
	Raw any `json:"rawFhir,omitempty"`
}

// LookupCode implements loinc_lookup_code by delegating to pkg/terminology's CodeSystem $lookup
// and reshaping the FHIR Parameters wire shape (axis/relation/string properties, designations)
// into a compact, agent-friendly summary. See docs/FHIR_TERMINOLOGY_PLAN.md §4.3 for the source
// shape this parses.
func (s *Service) LookupCode(ctx context.Context, req LookupCodeRequest) (LookupCodeResult, error) {
	if s.terminology == nil {
		return LookupCodeResult{}, errTerminologyUnavailable
	}
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code == "" {
		return LookupCodeResult{}, errors.New("code is required")
	}
	properties := req.Properties
	if req.Designations && len(properties) > 0 {
		hasDesignation := false
		for _, p := range properties {
			if strings.EqualFold(p, "designation") {
				hasDesignation = true
				break
			}
		}
		if !hasDesignation {
			properties = append(append([]string{}, properties...), "designation")
		}
	}
	result, oe := s.terminology.Lookup(ctx, terminology.LookupParams{Code: code, Property: properties, DisplayLanguage: req.Language})
	if oe != nil {
		return LookupCodeResult{}, asError(oe)
	}

	out := LookupCodeResult{
		Code:       code,
		Kind:       classifyCodeKind(code),
		BrowserURL: browserURL(code),
		FHIRURL:    codeSystemOpURL("$lookup", "system="+url.QueryEscape(loincSystemURL)+"&code="+url.QueryEscape(code)),
	}
	if req.RawFHIR {
		out.Raw = result
	}
	stringProps := map[string]string{}
	for _, p := range result.Parameter {
		switch p.Name {
		case "display":
			if p.ValueString != nil {
				out.Display = *p.ValueString
			}
		case "status":
			if p.ValueCode != nil {
				out.Status = *p.ValueCode
			}
		case "designation":
			if req.Designations && len(out.Designations) < maxDesignations {
				out.Designations = append(out.Designations, formatDesignation(p))
			}
		case "property":
			propCode, value, display, isCoding := parseLookupProperty(p)
			switch {
			case termAxisCodes[propCode]:
				out.Axes = append(out.Axes, LookupAxis{Axis: propCode, Code: value, Display: display})
			case termRelationCodes[propCode]:
				if len(out.Related) < maxRelated {
					out.Related = append(out.Related, LookupRelated{Relation: propCode, Code: value, Display: display})
				} else {
					out.RelatedTruncated = true
				}
			case !isCoding:
				stringProps[propCode] = value
			}
		}
	}
	if len(stringProps) > 0 {
		out.Properties = stringProps
	}
	return out, nil
}

func parseLookupProperty(p terminology.Parameter) (code, value, display string, isCoding bool) {
	for _, part := range p.Part {
		switch part.Name {
		case "code":
			if part.ValueCode != nil {
				code = *part.ValueCode
			}
		case "value":
			if part.ValueCoding != nil {
				value = part.ValueCoding.Code
				display = part.ValueCoding.Display
				isCoding = true
			} else if part.ValueString != nil {
				value = *part.ValueString
			}
		}
	}
	return code, value, display, isCoding
}

func formatDesignation(p terminology.Parameter) string {
	var lang, use, value string
	for _, part := range p.Part {
		switch part.Name {
		case "language":
			if part.ValueCode != nil {
				lang = *part.ValueCode
			}
		case "use":
			if part.ValueCoding != nil {
				use = part.ValueCoding.Code
			}
		case "value":
			if part.ValueString != nil {
				value = *part.ValueString
			}
		}
	}
	return lang + ":" + use + "=" + value
}

// --- loinc_validate_code ---

type ValidateCodeRequest struct {
	Code    string `json:"code" jsonschema:"LOINC code to validate, e.g. 718-7."`
	Display string `json:"display,omitempty" jsonschema:"Optional display text to check against the code's designations."`
}

type ValidateCodeResult struct {
	Valid      bool   `json:"valid"`
	Active     bool   `json:"active"`
	Display    string `json:"display,omitempty"`
	Message    string `json:"message,omitempty"`
	BrowserURL string `json:"browserUrl,omitempty"`
	FHIRURL    string `json:"fhirUrl"`
}

// ValidateCode implements loinc_validate_code via pkg/terminology's CodeSystem $validate-code.
func (s *Service) ValidateCode(ctx context.Context, req ValidateCodeRequest) (ValidateCodeResult, error) {
	if s.terminology == nil {
		return ValidateCodeResult{}, errTerminologyUnavailable
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return ValidateCodeResult{}, errors.New("code is required")
	}
	result, oe := s.terminology.ValidateCode(ctx, terminology.ValidateCodeParams{Code: code, Display: req.Display})
	if oe != nil {
		return ValidateCodeResult{}, asError(oe)
	}
	out := ValidateCodeResult{
		Valid:   paramStr(result.Parameter, "result") == "true",
		Active:  paramBoolVal(result.Parameter, "active"),
		Display: paramStr(result.Parameter, "display"),
		Message: paramStr(result.Parameter, "message"),
		FHIRURL: codeSystemOpURL("$validate-code", "system="+url.QueryEscape(loincSystemURL)+"&code="+url.QueryEscape(code)),
	}
	if out.Valid {
		out.BrowserURL = browserURL(code)
	}
	return out, nil
}

// --- loinc_subsumes ---

type SubsumesRequest struct {
	CodeA string `json:"codeA" jsonschema:"First LOINC code to compare, e.g. a term or LP part number."`
	CodeB string `json:"codeB" jsonschema:"Second LOINC code to compare."`
}

type SubsumesResult struct {
	Outcome string `json:"outcome"`
	FHIRURL string `json:"fhirUrl"`
}

// Subsumes implements loinc_subsumes via pkg/terminology's CodeSystem $subsumes.
func (s *Service) Subsumes(ctx context.Context, req SubsumesRequest) (SubsumesResult, error) {
	if s.terminology == nil {
		return SubsumesResult{}, errTerminologyUnavailable
	}
	result, oe := s.terminology.Subsumes(ctx, terminology.SubsumesParams{CodeA: req.CodeA, CodeB: req.CodeB})
	if oe != nil {
		return SubsumesResult{}, asError(oe)
	}
	return SubsumesResult{
		Outcome: paramStr(result.Parameter, "outcome"),
		FHIRURL: codeSystemOpURL("$subsumes", "system="+url.QueryEscape(loincSystemURL)+"&codeA="+url.QueryEscape(req.CodeA)+"&codeB="+url.QueryEscape(req.CodeB)),
	}, nil
}

// --- loinc_expand_value_set ---

const (
	defaultExpandRows = 25
	maxExpandRows     = 200
)

type ExpandValueSetRequest struct {
	URL        string `json:"url,omitempty" jsonschema:"ValueSet canonical url, e.g. http://loinc.org/vs/LL1162-8 (answer list), http://loinc.org/vs/LG100-4 (group), or http://loinc.org/vs/LP14559-6 (implicit part-hierarchy set). Required unless id is given."`
	ID         string `json:"id,omitempty" jsonschema:"ValueSet id in place of url, e.g. loinc-all or an LL/LG/LP code."`
	Filter     string `json:"filter,omitempty" jsonschema:"Case-insensitive substring filter over member display text."`
	Count      int    `json:"count,omitempty" jsonschema:"Maximum members to return, default 25, capped at 200."`
	Offset     int    `json:"offset,omitempty" jsonschema:"Paging offset into the expansion."`
	ActiveOnly bool   `json:"activeOnly,omitempty" jsonschema:"Exclude deprecated/inactive members."`
}

type ValueSetMember struct {
	Code     string `json:"code"`
	Display  string `json:"display,omitempty"`
	Inactive bool   `json:"inactive,omitempty"`
}

type ExpandValueSetResult struct {
	URL     string           `json:"url,omitempty"`
	Name    string           `json:"name,omitempty"`
	Total   int              `json:"total"`
	Offset  int              `json:"offset"`
	Count   int              `json:"count"`
	Members []ValueSetMember `json:"members"`
	FHIRURL string           `json:"fhirUrl"`
}

// ExpandValueSet implements loinc_expand_value_set via pkg/terminology's ValueSet $expand: named
// sets (loinc-all, deprecated-loinc-terms, ...), LL answer lists, LG groups, and implicit LP
// part-hierarchy sets all resolve through the same url/id.
func (s *Service) ExpandValueSet(ctx context.Context, req ExpandValueSetRequest) (ExpandValueSetResult, error) {
	if s.terminology == nil {
		return ExpandValueSetResult{}, errTerminologyUnavailable
	}
	if strings.TrimSpace(req.URL) == "" && strings.TrimSpace(req.ID) == "" {
		return ExpandValueSetResult{}, errors.New("url or id is required")
	}
	count := req.Count
	if count <= 0 {
		count = defaultExpandRows
	}
	if count > maxExpandRows {
		count = maxExpandRows
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}
	vs, oe := s.terminology.Expand(ctx, terminology.ExpandParams{
		URL: req.URL, ID: req.ID, Filter: req.Filter, Offset: &offset, Count: &count, ActiveOnly: req.ActiveOnly,
	})
	if oe != nil {
		return ExpandValueSetResult{}, asError(oe)
	}
	out := ExpandValueSetResult{URL: vs.URL, Name: vs.Name, Count: count, Offset: offset}
	if vs.Expansion != nil {
		out.Total = vs.Expansion.Total
		out.Members = make([]ValueSetMember, 0, len(vs.Expansion.Contains))
		for _, c := range vs.Expansion.Contains {
			out.Members = append(out.Members, ValueSetMember{Code: c.Code, Display: c.Display, Inactive: c.Inactive})
		}
	}
	query := "count=" + fmt.Sprint(count) + "&offset=" + fmt.Sprint(offset)
	if req.URL != "" {
		query = "url=" + url.QueryEscape(req.URL) + "&" + query
	} else {
		query = "id=" + url.QueryEscape(req.ID) + "&" + query
	}
	out.FHIRURL = "/fhir/ValueSet/$expand?" + query
	return out, nil
}

// --- loinc_search_value_sets ---

const (
	defaultValueSetSearchCount = 25
	maxValueSetSearchCount     = 100
)

type SearchValueSetsRequest struct {
	Name         string `json:"name,omitempty" jsonschema:"Prefix match on ValueSet name."`
	NameContains string `json:"nameContains,omitempty" jsonschema:"Substring match on ValueSet name."`
	URL          string `json:"url,omitempty" jsonschema:"Exact ValueSet canonical url to look up."`
	Count        int    `json:"count,omitempty" jsonschema:"Maximum rows, default 25, capped at 100."`
	Offset       int    `json:"offset,omitempty" jsonschema:"Paging offset."`
}

type ValueSetSummary struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	FHIRURL     string `json:"fhirUrl"`
}

// SearchValueSets implements loinc_search_value_sets via pkg/terminology's ValueSet search-type:
// name/nameContains covers answer lists, groups, and named catalogue sets (LL/LG implicit sets
// are looked up directly by url instead, via loinc_expand_value_set).
func (s *Service) SearchValueSets(ctx context.Context, req SearchValueSetsRequest) ([]ValueSetSummary, error) {
	if s.terminology == nil {
		return nil, errTerminologyUnavailable
	}
	count := req.Count
	if count <= 0 {
		count = defaultValueSetSearchCount
	}
	if count > maxValueSetSearchCount {
		count = maxValueSetSearchCount
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}
	bundle, oe := s.terminology.SearchValueSets(ctx, terminology.ValueSetSearchParams{
		URL: req.URL, Name: req.Name, NameContains: req.NameContains, Count: count, Offset: offset,
	})
	if oe != nil {
		return nil, asError(oe)
	}
	out := make([]ValueSetSummary, 0, len(bundle.Entry))
	for _, entry := range bundle.Entry {
		vs, ok := entry.Resource.(*terminology.ValueSet)
		if !ok {
			continue
		}
		out = append(out, ValueSetSummary{
			ID: vs.ID, URL: vs.URL, Name: vs.Name, Description: vs.Description,
			FHIRURL: "/fhir/ValueSet/" + url.PathEscape(vs.ID),
		})
	}
	return out, nil
}

// --- loinc_validate_value_set_membership ---

type ValidateValueSetMembershipRequest struct {
	URL        string `json:"url,omitempty" jsonschema:"ValueSet canonical url. Required unless id is given."`
	ID         string `json:"id,omitempty" jsonschema:"ValueSet id in place of url."`
	Code       string `json:"code" jsonschema:"Code to test for membership, e.g. an answer ID for an LL answer-list value set."`
	ActiveOnly bool   `json:"activeOnly,omitempty" jsonschema:"Only count active (non-deprecated) members."`
}

type ValidateValueSetMembershipResult struct {
	Member  bool   `json:"member"`
	Display string `json:"display,omitempty"`
	Message string `json:"message,omitempty"`
	FHIRURL string `json:"fhirUrl"`
}

// ValidateValueSetMembership implements loinc_validate_value_set_membership via pkg/terminology's
// ValueSet $validate-code.
func (s *Service) ValidateValueSetMembership(ctx context.Context, req ValidateValueSetMembershipRequest) (ValidateValueSetMembershipResult, error) {
	if s.terminology == nil {
		return ValidateValueSetMembershipResult{}, errTerminologyUnavailable
	}
	if strings.TrimSpace(req.URL) == "" && strings.TrimSpace(req.ID) == "" {
		return ValidateValueSetMembershipResult{}, errors.New("url or id is required")
	}
	if strings.TrimSpace(req.Code) == "" {
		return ValidateValueSetMembershipResult{}, errors.New("code is required")
	}
	result, oe := s.terminology.ValidateValueSetCode(ctx, terminology.ValidateValueSetCodeParams{
		URL: req.URL, ID: req.ID, Code: req.Code, ActiveOnly: req.ActiveOnly,
	})
	if oe != nil {
		return ValidateValueSetMembershipResult{}, asError(oe)
	}
	query := "code=" + url.QueryEscape(req.Code)
	if req.URL != "" {
		query = "url=" + url.QueryEscape(req.URL) + "&" + query
	} else {
		query = "id=" + url.QueryEscape(req.ID) + "&" + query
	}
	return ValidateValueSetMembershipResult{
		Member:  paramBoolVal(result.Parameter, "result"),
		Display: paramStr(result.Parameter, "display"),
		Message: paramStr(result.Parameter, "message"),
		FHIRURL: "/fhir/ValueSet/$validate-code?" + query,
	}, nil
}

// --- loinc_translate ---

type TranslateRequest struct {
	Code    string `json:"code" jsonschema:"Code to translate, e.g. a LOINC number or a source-system code being mapped to LOINC."`
	System  string `json:"system,omitempty" jsonschema:"Source code system of code. Defaults to http://loinc.org (search every served map from LOINC) when url/conceptMapId is also not given."`
	URL     string `json:"url,omitempty" jsonschema:"Specific ConceptMap canonical url to translate through."`
	ID      string `json:"conceptMapId,omitempty" jsonschema:"Specific ConceptMap id in place of url."`
	Reverse bool   `json:"reverse,omitempty" jsonschema:"Translate in the reverse (target-to-source) direction, e.g. to find deprecated LOINC replacements via the loinc-to-loinc map."`
}

type TranslateMatch struct {
	System      string `json:"system"`
	Code        string `json:"code"`
	Display     string `json:"display,omitempty"`
	Equivalence string `json:"equivalence"`
	Map         string `json:"map,omitempty"`
	Comment     string `json:"comment,omitempty"`
}

type TranslateResult struct {
	Result  bool             `json:"result"`
	Message string           `json:"message,omitempty"`
	Matches []TranslateMatch `json:"matches,omitempty"`
	FHIRURL string           `json:"fhirUrl"`
}

// Translate implements loinc_translate via pkg/terminology's ConceptMap $translate. Use it for
// legacy mapping workflows: translate a DEPRECATED LOINC term through the loinc-to-loinc map (or
// MAP_TO-derived map) to find its replacement, or map a third-party code onto LOINC.
func (s *Service) Translate(ctx context.Context, req TranslateRequest) (TranslateResult, error) {
	if s.terminology == nil {
		return TranslateResult{}, errTerminologyUnavailable
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return TranslateResult{}, errors.New("code is required")
	}
	system := req.System
	if strings.TrimSpace(system) == "" && strings.TrimSpace(req.URL) == "" && strings.TrimSpace(req.ID) == "" {
		// The MCP tool is agent-facing, unlike FHIR's own $translate route: default to LOINC as the
		// source system rather than requiring the agent to know FHIR's "system is required" contract.
		system = loincSystemURL
	}
	result, oe := s.terminology.Translate(ctx, terminology.TranslateParams{
		Code: code, System: system, URL: req.URL, ConceptMapID: req.ID, Reverse: req.Reverse,
	})
	if oe != nil {
		return TranslateResult{}, asError(oe)
	}
	out := TranslateResult{
		Result:  paramBoolVal(result.Parameter, "result"),
		Message: paramStr(result.Parameter, "message"),
	}
	for _, p := range result.Parameter {
		if p.Name != "match" {
			continue
		}
		var m TranslateMatch
		for _, part := range p.Part {
			switch part.Name {
			case "equivalence":
				if part.ValueCode != nil {
					m.Equivalence = *part.ValueCode
				}
			case "concept":
				if part.ValueCoding != nil {
					m.System = part.ValueCoding.System
					m.Code = part.ValueCoding.Code
					m.Display = part.ValueCoding.Display
				}
			case "source":
				if part.ValueUri != nil {
					m.Map = *part.ValueUri
				}
			case "comment":
				if part.ValueString != nil {
					m.Comment = *part.ValueString
				}
			}
		}
		out.Matches = append(out.Matches, m)
	}
	query := "code=" + url.QueryEscape(code)
	if req.URL != "" {
		out.FHIRURL = "/fhir/ConceptMap/$translate?url=" + url.QueryEscape(req.URL) + "&" + query
	} else if req.ID != "" {
		out.FHIRURL = "/fhir/ConceptMap/" + url.PathEscape(req.ID) + "/$translate?" + query
	} else {
		out.FHIRURL = "/fhir/ConceptMap/$translate?system=" + url.QueryEscape(system) + "&" + query
	}
	return out, nil
}

// --- loinc_list_concept_maps ---

type ListConceptMapsRequest struct {
	SourceSystem string `json:"sourceSystem,omitempty" jsonschema:"Filter to maps whose source system equals this url."`
	TargetSystem string `json:"targetSystem,omitempty" jsonschema:"Filter to maps whose target system equals this url."`
	Count        int    `json:"count,omitempty" jsonschema:"Maximum rows, default 20, capped at 100."`
	Offset       int    `json:"offset,omitempty" jsonschema:"Paging offset."`
}

type ConceptMapSummary struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	Source  string `json:"source"`
	Target  string `json:"target"`
	FHIRURL string `json:"fhirUrl"`
}

// ListConceptMaps implements loinc_list_concept_maps via pkg/terminology's ConceptMap
// search-type, listing every map this server serves (loinc-to-loinc replacement mappings and
// MAP_TO-derived maps to other code systems).
func (s *Service) ListConceptMaps(ctx context.Context, req ListConceptMapsRequest) ([]ConceptMapSummary, error) {
	if s.terminology == nil {
		return nil, errTerminologyUnavailable
	}
	bundle, oe := s.terminology.SearchConceptMaps(ctx, terminology.ConceptMapSearchParams{
		SourceSystem: req.SourceSystem, TargetSystem: req.TargetSystem, Count: req.Count, Offset: req.Offset,
	})
	if oe != nil {
		return nil, asError(oe)
	}
	out := make([]ConceptMapSummary, 0, len(bundle.Entry))
	for _, entry := range bundle.Entry {
		cm, ok := entry.Resource.(*terminology.ConceptMap)
		if !ok {
			continue
		}
		out = append(out, ConceptMapSummary{
			ID: cm.ID, URL: cm.URL, Source: cm.SourceUri, Target: cm.TargetUri,
			FHIRURL: "/fhir/ConceptMap/" + url.PathEscape(cm.ID),
		})
	}
	return out, nil
}

// --- loinc_get_questionnaire ---

const maxQuestionnaireItems = 200
const maxQuestionnaireAnswerOptions = 30

type GetQuestionnaireRequest struct {
	LOINCNum string `json:"loincNum" jsonschema:"LOINC number of a panel/form, e.g. 24357-6."`
	RawFHIR  bool   `json:"rawFhir,omitempty" jsonschema:"Return the full FHIR Questionnaire resource instead of the compact item tree."`
}

type QuestionnaireAnswerOptionCompact struct {
	Code    string `json:"code"`
	Display string `json:"display,omitempty"`
}

type QuestionnaireItemCompact struct {
	LinkID                 string                             `json:"linkId"`
	Code                   string                             `json:"code,omitempty"`
	Text                   string                             `json:"text,omitempty"`
	Type                   string                             `json:"type"`
	Required               bool                               `json:"required,omitempty"`
	AnswerOptions          []QuestionnaireAnswerOptionCompact `json:"answerOptions,omitempty"`
	AnswerOptionsTruncated bool                               `json:"answerOptionsTruncated,omitempty"`
	Items                  []QuestionnaireItemCompact         `json:"items,omitempty"`
}

type GetQuestionnaireResult struct {
	LOINCNum       string                     `json:"loincNum"`
	Title          string                     `json:"title,omitempty"`
	Items          []QuestionnaireItemCompact `json:"items"`
	ItemsTruncated bool                       `json:"itemsTruncated,omitempty"`
	BrowserURL     string                     `json:"browserUrl"`
	FHIRURL        string                     `json:"fhirUrl"`
	// Raw is typed any for the same reason as LookupCodeResult.Raw: terminology.QuestionnaireItem
	// is self-referential (Item []QuestionnaireItem).
	Raw any `json:"rawFhir,omitempty"`
}

// GetQuestionnaire implements loinc_get_questionnaire via pkg/terminology's Questionnaire read,
// flattening the FHIR item tree into a compact, capped structure for form-building agents.
func (s *Service) GetQuestionnaire(ctx context.Context, req GetQuestionnaireRequest) (GetQuestionnaireResult, error) {
	if s.terminology == nil {
		return GetQuestionnaireResult{}, errTerminologyUnavailable
	}
	code := strings.TrimSpace(req.LOINCNum)
	if code == "" {
		return GetQuestionnaireResult{}, errors.New("loincNum is required")
	}
	q, oe := s.terminology.Questionnaire(ctx, code)
	if oe != nil {
		return GetQuestionnaireResult{}, asError(oe)
	}
	out := GetQuestionnaireResult{
		LOINCNum:   code,
		Title:      q.Title,
		BrowserURL: browserURL(code),
		FHIRURL:    "/fhir/Questionnaire/" + url.PathEscape(code),
	}
	if req.RawFHIR {
		out.Raw = q
	}
	remaining := maxQuestionnaireItems
	out.Items, out.ItemsTruncated = compactQuestionnaireItems(q.Item, &remaining)
	return out, nil
}

func compactQuestionnaireItems(items []terminology.QuestionnaireItem, remaining *int) ([]QuestionnaireItemCompact, bool) {
	out := make([]QuestionnaireItemCompact, 0, len(items))
	truncated := false
	for _, item := range items {
		if *remaining <= 0 {
			truncated = true
			break
		}
		*remaining--
		compact := QuestionnaireItemCompact{
			LinkID:   item.LinkID,
			Text:     item.Text,
			Type:     item.Type,
			Required: item.Required,
		}
		if len(item.Code) > 0 {
			compact.Code = item.Code[0].Code
		}
		for i, opt := range item.AnswerOption {
			if i >= maxQuestionnaireAnswerOptions {
				compact.AnswerOptionsTruncated = true
				break
			}
			compact.AnswerOptions = append(compact.AnswerOptions, QuestionnaireAnswerOptionCompact{
				Code: opt.ValueCoding.Code, Display: opt.ValueCoding.Display,
			})
		}
		if len(item.Item) > 0 {
			var childTruncated bool
			compact.Items, childTruncated = compactQuestionnaireItems(item.Item, remaining)
			truncated = truncated || childTruncated
		}
		out = append(out, compact)
	}
	return out, truncated
}

// --- loinc_lucene_search ---

const (
	defaultLuceneRows = 10
	maxLuceneRows     = 50
)

type LuceneSearchRequest struct {
	Scope  string `json:"scope" jsonschema:"Which local search index to query: loincs, parts, answerlists, or groups."`
	Query  string `json:"query" jsonschema:"Lucene-style query, e.g. \"Component:glucose System:bld\" or \"LOINC:718-7\"."`
	Rows   int    `json:"rows,omitempty" jsonschema:"Maximum rows, default 10, capped at 50."`
	Offset int    `json:"offset,omitempty" jsonschema:"Paging offset."`
}

type LuceneSearchRow struct {
	Key        string  `json:"key"`
	Display    string  `json:"display,omitempty"`
	Status     string  `json:"status,omitempty"`
	Score      float64 `json:"score"`
	BrowserURL string  `json:"browserUrl,omitempty"`
}

type LuceneSearchResult struct {
	Scope string            `json:"scope"`
	Total uint64            `json:"total"`
	Rows  []LuceneSearchRow `json:"rows"`
}

var luceneSearchScopes = map[string]bool{"loincs": true, "parts": true, "answerlists": true, "groups": true}

// LuceneSearch implements loinc_lucene_search over the same local Bleve index /searchapi and
// /api/v1/local-search/query serve (internal/server/local_search.go), for agents that want
// Lucene-style fielded/boolean/wildcard queries the compact loinc_search_* tools don't expose.
func (s *Service) LuceneSearch(ctx context.Context, req LuceneSearchRequest) (LuceneSearchResult, error) {
	if s.luceneSearch == nil {
		return LuceneSearchResult{}, errors.New("local Lucene search is not available on this MCP transport; use loinc_search_terms/parts/answer_lists/groups instead")
	}
	scope := strings.ToLower(strings.TrimSpace(req.Scope))
	if !luceneSearchScopes[scope] {
		return LuceneSearchResult{}, fmt.Errorf("unsupported scope %q; use loincs, parts, answerlists, or groups", req.Scope)
	}
	if strings.TrimSpace(req.Query) == "" {
		return LuceneSearchResult{}, errors.New("query is required")
	}
	rows := req.Rows
	if rows <= 0 {
		rows = defaultLuceneRows
	}
	if rows > maxLuceneRows {
		rows = maxLuceneRows
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}
	hits, total, err := s.luceneSearch(ctx, scope, req.Query, rows, offset)
	if err != nil {
		return LuceneSearchResult{}, err
	}
	out := LuceneSearchResult{Scope: scope, Total: total, Rows: make([]LuceneSearchRow, 0, len(hits))}
	for _, hit := range hits {
		out.Rows = append(out.Rows, buildLuceneRow(hit))
	}
	return out, nil
}

func buildLuceneRow(hit loinc.LocalSearchResult) LuceneSearchRow {
	row := LuceneSearchRow{Key: hit.Key, Score: hit.Score}
	switch r := hit.Result.(type) {
	case loinc.SearchResult:
		row.Display = firstNonEmpty(r.ShortName, r.LongCommonName)
		row.Status = r.Status
		row.BrowserURL = browserURL(r.LOINCNum)
	case loinc.Part:
		row.Display = firstNonEmpty(r.PartDisplayName, r.PartName)
		row.Status = r.Status
	case loinc.AnswerList:
		row.Display = r.AnswerListName
	case loinc.LOINCGroup:
		row.Display = r.GroupName
		row.Status = r.Status
	}
	return row
}

// --- tool registration wrappers ---

func (s *Service) lookupCodeTool(ctx context.Context, _ *mcp.CallToolRequest, req LookupCodeRequest) (*mcp.CallToolResult, LookupCodeResult, error) {
	out, err := s.LookupCode(ctx, req)
	return nil, out, err
}

func (s *Service) validateCodeTool(ctx context.Context, _ *mcp.CallToolRequest, req ValidateCodeRequest) (*mcp.CallToolResult, ValidateCodeResult, error) {
	out, err := s.ValidateCode(ctx, req)
	return nil, out, err
}

func (s *Service) subsumesTool(ctx context.Context, _ *mcp.CallToolRequest, req SubsumesRequest) (*mcp.CallToolResult, SubsumesResult, error) {
	out, err := s.Subsumes(ctx, req)
	return nil, out, err
}

func (s *Service) expandValueSetTool(ctx context.Context, _ *mcp.CallToolRequest, req ExpandValueSetRequest) (*mcp.CallToolResult, ExpandValueSetResult, error) {
	out, err := s.ExpandValueSet(ctx, req)
	return nil, out, err
}

func (s *Service) searchValueSetsTool(ctx context.Context, _ *mcp.CallToolRequest, req SearchValueSetsRequest) (*mcp.CallToolResult, []ValueSetSummary, error) {
	out, err := s.SearchValueSets(ctx, req)
	return nil, out, err
}

func (s *Service) validateValueSetMembershipTool(ctx context.Context, _ *mcp.CallToolRequest, req ValidateValueSetMembershipRequest) (*mcp.CallToolResult, ValidateValueSetMembershipResult, error) {
	out, err := s.ValidateValueSetMembership(ctx, req)
	return nil, out, err
}

func (s *Service) translateTool(ctx context.Context, _ *mcp.CallToolRequest, req TranslateRequest) (*mcp.CallToolResult, TranslateResult, error) {
	out, err := s.Translate(ctx, req)
	return nil, out, err
}

func (s *Service) listConceptMapsTool(ctx context.Context, _ *mcp.CallToolRequest, req ListConceptMapsRequest) (*mcp.CallToolResult, []ConceptMapSummary, error) {
	out, err := s.ListConceptMaps(ctx, req)
	return nil, out, err
}

func (s *Service) getQuestionnaireTool(ctx context.Context, _ *mcp.CallToolRequest, req GetQuestionnaireRequest) (*mcp.CallToolResult, GetQuestionnaireResult, error) {
	out, err := s.GetQuestionnaire(ctx, req)
	return nil, out, err
}

func (s *Service) luceneSearchTool(ctx context.Context, _ *mcp.CallToolRequest, req LuceneSearchRequest) (*mcp.CallToolResult, LuceneSearchResult, error) {
	out, err := s.LuceneSearch(ctx, req)
	return nil, out, err
}
