package terminology

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"loinc-browser/internal/loinc"
)

// LookupParams is CodeSystem $lookup's input (§4.3), shared by the GET query-parameter and POST
// Parameters-body adapters in internal/fhirhttp.
type LookupParams struct {
	Code            string
	System          string
	Version         string
	Coding          *Coding // an explicit `coding` parameter is used when Code is empty
	DisplayLanguage string
	Property        []string
}

// termAxisOrder is the fixed order term $lookup emits primary-link axis Coding properties in
// (§4.3 step 1), verified against codesystem-lookup-718-7.json.
var termAxisOrder = []string{"SYSTEM", "TIME_ASPCT", "PROPERTY", "SCALE_TYP", "METHOD_TYP", "CLASS", "COMPONENT"}

// Lookup implements CodeSystem $lookup for all five http://loinc.org code kinds: LOINC terms,
// LP parts, LL answer lists, LA answers, and LG groups (§4.3).
func (s *Service) Lookup(ctx context.Context, params LookupParams) (*Parameters, *OutcomeError) {
	store, outcomeErr := s.store()
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	version, outcomeErr := s.releaseVersion(ctx, store)
	if outcomeErr != nil {
		return nil, outcomeErr
	}

	code := params.Code
	if code == "" && params.Coding != nil {
		code = params.Coding.Code
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, requiredError("Parameter 'code' or 'coding' is required")
	}
	code = normalizeCode(code)
	notFoundText := "Unable to find code for system/version = " + code
	if err := checkSystemVersion(params.System, params.Version, version, notFoundText); err != nil {
		return nil, err
	}

	var (
		display      string
		status       string
		props        []Parameter
		designations []Parameter
		err          error
	)
	switch classifyLoincCode(code) {
	case codeKindPart:
		display, status, designations, props, err = lookupPart(ctx, store, code)
	case codeKindAnswerList:
		display, status, designations, props, err = lookupAnswerList(ctx, store, code)
	case codeKindAnswer:
		display, status, designations, props, err = lookupAnswer(ctx, store, code)
	case codeKindGroup:
		display, status, designations, props, err = lookupGroup(ctx, store, code)
	default:
		display, status, designations, props, err = lookupTerm(ctx, store, code)
	}
	if err != nil {
		if errors.Is(err, loinc.ErrNotFound) {
			return nil, notFoundError(notFoundText)
		}
		return nil, &OutcomeError{Status: 503, Code: "exception", Text: err.Error()}
	}

	designations = filterDesignationsByLanguage(designations, params.DisplayLanguage)
	if override := primaryDisplayForLanguage(designations, params.DisplayLanguage); override != "" {
		display = override
	}
	// A property filter narrows the response to just those properties and drops designations
	// entirely, unless the caller explicitly asked for them via property=designation (§4.3;
	// verified against codesystem-lookup-4544-3-props.json, -LP14542-2-parent.json,
	// -30064-0-parent.json, -LP31448-1-child.json, none of which carry a designation).
	if hasPropertyFilter(params.Property) && !requestsDesignations(params.Property) {
		designations = nil
	}
	props = filterPropertiesByCode(props, params.Property)

	out := []Parameter{
		paramCode("code", code),
		paramString("system", loincSystem),
		paramString("name", "LOINC"),
		paramString("version", version),
		paramString("display", display),
		paramCode("status", status),
	}
	out = append(out, designations...)
	out = append(out, props...)
	return &Parameters{Parameter: out}, nil
}

type codeKind int

const (
	codeKindTerm codeKind = iota
	codeKindPart
	codeKindAnswerList
	codeKindAnswer
	codeKindGroup
)

// codeResolves confirms code resolves to one of the five known http://loinc.org code kinds
// (term, LP part incl. hierarchy-only, LL, LA, LG), without building the full $lookup response.
// $lookup, $validate-code (via Lookup), and $subsumes (assertCodeExists) all route through this
// so a hierarchy-only LP node (hierarchy_concepts but never published as a Part.csv row, e.g.
// LP31448-1) resolves consistently everywhere.
func codeResolves(ctx context.Context, store *loinc.Store, code string) error {
	switch classifyLoincCode(code) {
	case codeKindPart:
		return partCodeResolves(ctx, store, code)
	case codeKindAnswerList:
		_, err := store.AnswerList(ctx, code)
		return err
	case codeKindAnswer:
		rows, err := store.FHIRAnswer(ctx, code)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return loinc.ErrNotFound
		}
		return nil
	case codeKindGroup:
		_, err := store.Group(ctx, code)
		return err
	default:
		_, err := store.Term(ctx, code)
		return err
	}
}

// partCodeResolves is codeResolves' LP branch: it mirrors lookupPart's two-way resolution (a real
// Part.csv row, or a hierarchy-only node) without building the parent/child property lists a full
// $lookup needs, since a bare existence check has no use for them (they cost 2 extra queries per
// call -- BenchmarkSubsumes regressed ~3x when this first routed through lookupPart itself).
func partCodeResolves(ctx context.Context, store *loinc.Store, code string) error {
	if _, err := store.Part(ctx, code); err == nil {
		return nil
	} else if !errors.Is(err, loinc.ErrNotFound) {
		return err
	}
	_, ok, err := store.FHIRHierarchyConceptLabel(ctx, code)
	if err != nil {
		return err
	}
	if !ok {
		return loinc.ErrNotFound
	}
	return nil
}

func classifyLoincCode(code string) codeKind {
	switch {
	case strings.HasPrefix(code, "LP"):
		return codeKindPart
	case strings.HasPrefix(code, "LL"):
		return codeKindAnswerList
	case strings.HasPrefix(code, "LA"):
		return codeKindAnswer
	case strings.HasPrefix(code, "LG"):
		return codeKindGroup
	default:
		return codeKindTerm
	}
}

// lookupTerm builds the display/status/designations/properties for a LOINC term code.
func lookupTerm(ctx context.Context, store *loinc.Store, code string) (string, string, []Parameter, []Parameter, error) {
	term, err := store.TermWithAccessories(ctx, code)
	if err != nil {
		return "", "", nil, nil, err
	}
	status := "active"
	if strings.EqualFold(term.Status, "DEPRECATED") {
		status = "retired"
	}

	var designations []Parameter
	designations = append(designations, designation("en-US", "LONG_COMMON_NAME", term.LongCommonName))
	if fsn := fullySpecifiedName(term.Component, term.Property, term.TimeAspect, term.System, term.Scale, term.Method, false); fsn != "" {
		designations = append(designations, designation("en-US", "FullySpecifiedName", fsn))
	}
	if consumerName, ok, err := store.FHIRConsumerName(ctx, term.LOINCNum); err == nil && ok {
		designations = append(designations, designation("en-US", "ConsumerName", consumerName))
	}
	if term.ShortName != "" {
		designations = append(designations, designation("en-US", "SHORTNAME", term.ShortName))
	}
	if term.DisplayName != "" {
		designations = append(designations, designation("en-US", "DisplayName", term.DisplayName))
	}

	variants, err := store.FHIRLinguisticVariants(ctx, term.LOINCNum)
	if err != nil {
		return "", "", nil, nil, err
	}
	for _, v := range variants {
		if v.LongCommonName != "" {
			designations = append(designations, designation(v.Language, "LONG_COMMON_NAME", v.LongCommonName))
		}
		if v.ShortName != "" {
			designations = append(designations, designation(v.Language, "SHORTNAME", v.ShortName))
		}
		if fsn := fullySpecifiedName(v.Component, v.Property, v.TimeAspect, v.System, v.Scale, v.Method, true); fsn != "" {
			designations = append(designations, designation(v.Language, "FullySpecifiedName", fsn))
		}
		if v.LinguisticVariantDisplayName != "" {
			designations = append(designations, designation(v.Language, "LinguisticVariantDisplayName", v.LinguisticVariantDisplayName))
		}
	}

	var props []Parameter
	partLinks, err := store.FHIRPartLinks(ctx, term.LOINCNum)
	if err != nil {
		return "", "", nil, nil, err
	}
	// The seven axis properties (termAxisOrder) are identified by their property code, not by
	// loinc_part_links.link_set: CLASS in particular is stored as a "supplementary" row (see
	// codesystem-lookup-718-7.json), and the same axis can appear as both a "primary" row and a
	// "supplementary" DetailedModel row with an identical value, which would otherwise emit it
	// twice. Take the first occurrence of each axis code (primary sorts first, per
	// FHIRPartLinks); route everything else into the non-axis "supplementary" property list.
	axisIsProperty := make(map[string]bool, len(termAxisOrder))
	for _, axis := range termAxisOrder {
		axisIsProperty[axis] = true
	}
	primaryByProperty := map[string]loinc.FHIRPartLink{}
	var supplementary []loinc.FHIRPartLink
	seenSupplementary := map[string]bool{}
	for _, link := range partLinks {
		if link.PartNumber == "" {
			continue
		}
		if axisIsProperty[link.Property] {
			if _, exists := primaryByProperty[link.Property]; !exists {
				primaryByProperty[link.Property] = link
			}
			continue
		}
		key := link.Property + "|" + link.PartNumber
		if seenSupplementary[key] {
			continue
		}
		seenSupplementary[key] = true
		supplementary = append(supplementary, link)
	}
	for _, axis := range termAxisOrder {
		if link, ok := primaryByProperty[axis]; ok {
			props = append(props, propertyCoding(axis, link.PartNumber, link.Display))
		}
	}
	// Divergence: upstream's exact supplementary order (codesystem-lookup-718-7.json:
	// category, system-core, analyte-core, analyte, category, time-core) reflects the original
	// LoincPartLink_Supplementary.csv row order, which loinc_part_links (a WITHOUT ROWID table
	// keyed by (loinc_num, part_number, link_set, link_type_name, property), per the plan's
	// "no schema changes" constraint) does not preserve. The set and codes match; only the
	// relative order among non-axis properties can differ from upstream.
	for _, link := range supplementary {
		props = append(props, propertyCoding(link.Property, link.PartNumber, link.Display))
	}

	row, ok, err := store.FHIRLoincRow(ctx, term.LOINCNum)
	if err != nil {
		return "", "", nil, nil, err
	}
	if !ok {
		row = term.Fields
	}
	for _, column := range termStringPropertyColumns() {
		value := strings.TrimSpace(row[column])
		if value == "" {
			continue
		}
		props = append(props, propertyString(column, value))
	}

	for _, mapTo := range term.MapTo {
		target, err := store.Term(ctx, mapTo.MapTo)
		display := mapTo.MapTo
		if err == nil {
			display = target.LongCommonName
		}
		props = append(props, propertyCoding("MAP_TO", mapTo.MapTo, display))
	}
	for _, al := range term.AnswerLists {
		props = append(props, propertyCoding("answer-list", al.Code, al.Title))
	}

	parents, err := store.FHIRHierarchyImmediateParents(ctx, term.LOINCNum)
	if err != nil {
		return "", "", nil, nil, err
	}
	for _, p := range parents {
		props = append(props, propertyCoding("parent", p.Code, p.Display))
	}
	groups, err := store.FHIRTermGroups(ctx, term.LOINCNum)
	if err != nil {
		return "", "", nil, nil, err
	}
	for _, g := range groups {
		props = append(props, propertyCoding("parent", g.Code, g.Display))
	}

	return term.LongCommonName, status, designations, props, nil
}

// fullySpecifiedName builds "COMPONENT:PROPERTY:TIME_ASPCT:SYSTEM:SCALE_TYP[:METHOD_TYP]".
// English designations omit a trailing separator for a blank method; linguistic-variant
// translations always join all six fields (observed from the exemplars: a translated row with
// no axis content at all yields no designation, but one with any axis content keeps the
// trailing colon for a blank method).
func fullySpecifiedName(component, property, timeAspect, system, scale, method string, alwaysSixFields bool) string {
	parts := []string{component, property, timeAspect, system, scale}
	allBlank := true
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			allBlank = false
			break
		}
	}
	if allBlank {
		return ""
	}
	if alwaysSixFields {
		parts = append(parts, method)
		return strings.Join(parts, ":")
	}
	if strings.TrimSpace(method) != "" {
		parts = append(parts, method)
	}
	return strings.Join(parts, ":")
}

// lookupPart builds the display/status/designations/properties for an LP part code.
func lookupPart(ctx context.Context, store *loinc.Store, code string) (string, string, []Parameter, []Parameter, error) {
	part, err := store.Part(ctx, code)
	if errors.Is(err, loinc.ErrNotFound) {
		// Some Component Hierarchy by System nodes group other parts but were never themselves
		// published as a Part.csv row (node_kind "hierarchy_only", e.g. LP31448-1). They still
		// resolve for $lookup, with only "parent"/"child" hierarchy properties and no part
		// metadata or designations (codesystem-lookup-LP31448-1-child.json).
		return lookupHierarchyOnlyPart(ctx, store, code)
	}
	if err != nil {
		return "", "", nil, nil, err
	}
	display := part.PartDisplayName
	if display == "" {
		display = part.PartName
	}
	status := "active"
	if strings.EqualFold(part.Status, "DEPRECATED") {
		status = "retired"
	}
	var designations []Parameter
	if part.PartName != "" {
		designations = append(designations, designation("en-US", "PartName", part.PartName))
	}
	if part.PartDisplayName != "" {
		designations = append(designations, designation("en-US", "PartDisplayName", part.PartDisplayName))
	}

	var props []Parameter
	if part.PartTypeName != "" {
		props = append(props, propertyString("PartTypeName", part.PartTypeName))
	}
	if part.Status != "" {
		props = append(props, propertyString("STATUS", part.Status))
	}
	parents, err := store.FHIRHierarchyImmediateParents(ctx, part.PartNumber)
	if err != nil {
		return "", "", nil, nil, err
	}
	for _, p := range parents {
		props = append(props, propertyCoding("parent", p.Code, p.Display))
	}
	children, err := store.FHIRHierarchyImmediateChildren(ctx, part.PartNumber)
	if err != nil {
		return "", "", nil, nil, err
	}
	for _, c := range children {
		props = append(props, propertyCoding("child", c.Code, c.Display))
	}
	return display, status, designations, props, nil
}

// lookupHierarchyOnlyPart builds the $lookup result for an LP code that exists only in the
// Component Hierarchy by System (hierarchy_concepts), not in Part.csv.
func lookupHierarchyOnlyPart(ctx context.Context, store *loinc.Store, code string) (string, string, []Parameter, []Parameter, error) {
	label, ok, err := store.FHIRHierarchyConceptLabel(ctx, code)
	if err != nil {
		return "", "", nil, nil, err
	}
	if !ok {
		return "", "", nil, nil, loinc.ErrNotFound
	}
	var props []Parameter
	parents, err := store.FHIRHierarchyImmediateParents(ctx, code)
	if err != nil {
		return "", "", nil, nil, err
	}
	for _, p := range parents {
		props = append(props, propertyCoding("parent", p.Code, p.Display))
	}
	children, err := store.FHIRHierarchyImmediateChildren(ctx, code)
	if err != nil {
		return "", "", nil, nil, err
	}
	for _, c := range children {
		props = append(props, propertyCoding("child", c.Code, c.Display))
	}
	return label, "active", nil, props, nil
}

// lookupAnswerList builds the display/status/designations/properties for an LL answer-list code.
func lookupAnswerList(ctx context.Context, store *loinc.Store, code string) (string, string, []Parameter, []Parameter, error) {
	list, err := store.AnswerList(ctx, code)
	if err != nil {
		return "", "", nil, nil, err
	}
	var designations []Parameter
	if list.AnswerListName != "" {
		designations = append(designations, designation("en-US", "AnswerListName", list.AnswerListName))
	}

	var props []Parameter
	answersFor, err := store.FHIRAnswerListAnswersFor(ctx, list.AnswerListID)
	if err != nil {
		return "", "", nil, nil, err
	}
	for _, term := range answersFor {
		props = append(props, propertyCoding("answers-for", term.Code, term.Display))
	}
	if list.ExtDefinedYN != "" {
		props = append(props, propertyString("AnswerExtDefinedYNListOID", list.ExtDefinedYN))
	}
	if list.AnswerListOID != "" {
		props = append(props, propertyString("AnswerListOID", list.AnswerListOID))
	}
	if list.AnswerListName != "" {
		props = append(props, propertyString("AnswerListName", list.AnswerListName))
	}
	answers, err := store.FHIRAnswerListAnswers(ctx, list.AnswerListID)
	if err != nil {
		return "", "", nil, nil, err
	}
	for _, a := range answers {
		props = append(props, propertyCoding("child", a.AnswerStringID, a.DisplayText))
	}
	return list.AnswerListName, "active", designations, props, nil
}

// lookupAnswer builds the display/status/designations/properties for an LA answer code.
func lookupAnswer(ctx context.Context, store *loinc.Store, code string) (string, string, []Parameter, []Parameter, error) {
	rows, err := store.FHIRAnswer(ctx, code)
	if err != nil {
		return "", "", nil, nil, err
	}
	if len(rows) == 0 {
		return "", "", nil, nil, loinc.ErrNotFound
	}
	first := rows[0]
	var designations []Parameter
	if first.DisplayText != "" {
		designations = append(designations, designation("en-US", "DisplayText", first.DisplayText))
	}
	var props []Parameter
	if first.Score != "" {
		props = append(props, propertyString("Score", first.Score))
	}
	if first.SequenceNumber != 0 {
		props = append(props, propertyString("SequenceNumber", strconv.Itoa(first.SequenceNumber)))
	}
	for _, row := range rows {
		list, err := store.AnswerList(ctx, row.AnswerListID)
		display := row.AnswerListID
		if err == nil {
			display = list.AnswerListName
		}
		props = append(props, propertyCoding("parent", row.AnswerListID, display))
	}
	return first.DisplayText, "active", designations, props, nil
}

// lookupGroup builds the display/status/designations/properties for an LG group code.
func lookupGroup(ctx context.Context, store *loinc.Store, code string) (string, string, []Parameter, []Parameter, error) {
	group, err := store.Group(ctx, code)
	if err != nil {
		return "", "", nil, nil, err
	}
	var designations []Parameter
	if group.GroupName != "" {
		designations = append(designations, designation("en-US", group.GroupName, group.GroupName))
	}
	var props []Parameter
	if parent, ok, err := store.FHIRGroupParent(ctx, group.GroupID); err != nil {
		return "", "", nil, nil, err
	} else if ok {
		props = append(props, propertyCoding("parent", parent.Code, parent.Display))
	}
	members, err := store.FHIRGroupMembers(ctx, group.GroupID)
	if err != nil {
		return "", "", nil, nil, err
	}
	for _, m := range members {
		props = append(props, propertyCoding("child", m.Code, m.Display))
	}
	if group.Status != "" {
		props = append(props, propertyString("STATUS", group.Status))
	}
	return group.GroupName, "active", designations, props, nil
}

func designation(language, useCode, value string) Parameter {
	return paramPart("designation",
		paramCode("language", language),
		paramCoding("use", loincCoding(useCode, useCode)),
		paramString("value", value),
	)
}

func propertyCoding(code, valueCode, display string) Parameter {
	return paramPart("property",
		paramCode("code", code),
		paramCoding("value", loincCoding(valueCode, display)),
	)
}

func propertyString(code, value string) Parameter {
	return paramPart("property",
		paramCode("code", code),
		paramString("value", value),
	)
}

func hasPropertyFilter(requested []string) bool {
	return len(requested) > 0
}

func requestsDesignations(requested []string) bool {
	for _, r := range requested {
		if strings.EqualFold(r, "designation") {
			return true
		}
	}
	return false
}

func filterPropertiesByCode(props []Parameter, requested []string) []Parameter {
	if len(requested) == 0 {
		return props
	}
	allow := map[string]bool{}
	for _, r := range requested {
		allow[r] = true
	}
	var out []Parameter
	for _, p := range props {
		if len(p.Part) > 0 && p.Part[0].ValueCode != nil && allow[*p.Part[0].ValueCode] {
			out = append(out, p)
		}
	}
	return out
}

func filterDesignationsByLanguage(designations []Parameter, displayLanguage string) []Parameter {
	if displayLanguage == "" {
		return designations
	}
	var out []Parameter
	for _, d := range designations {
		lang := designationLanguage(d)
		if strings.EqualFold(lang, "en-US") || strings.EqualFold(lang, displayLanguage) {
			out = append(out, d)
		}
	}
	return out
}

func designationLanguage(d Parameter) string {
	for _, part := range d.Part {
		if part.Name == "language" && part.ValueCode != nil {
			return *part.ValueCode
		}
	}
	return ""
}

func primaryDisplayForLanguage(designations []Parameter, displayLanguage string) string {
	if displayLanguage == "" {
		return ""
	}
	for _, d := range designations {
		if !strings.EqualFold(designationLanguage(d), displayLanguage) {
			continue
		}
		for _, part := range d.Part {
			if part.Name == "use" && part.ValueCoding != nil && part.ValueCoding.Code == "LONG_COMMON_NAME" {
				for _, valuePart := range d.Part {
					if valuePart.Name == "value" && valuePart.ValueString != nil {
						return *valuePart.ValueString
					}
				}
			}
		}
	}
	return ""
}
