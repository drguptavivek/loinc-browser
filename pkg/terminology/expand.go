package terminology

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"

	"loinc-browser/internal/loinc"
)

// defaultExpandCount and maxExpandCount are $expand's count default/ceiling (§4.7).
const (
	defaultExpandCount = 100
	maxExpandCount     = 1000
)

// InlineValueSet is a POSTed `valueSet` parameter's compose (§4.7.1): the only part of an inline
// ValueSet resource $expand evaluates.
type InlineValueSet struct {
	Compose *ValueSetCompose
}

// ExpandParams is ValueSet $expand's input (§4.7).
type ExpandParams struct {
	URL                 string
	ID                  string // set when hit via /ValueSet/{id}/$expand
	Inline              *InlineValueSet
	Filter              string
	Offset              *int // nil means "not supplied" -> 0
	Count               *int // nil means "not supplied" -> defaultExpandCount
	ActiveOnly          bool
	IncludeDesignations bool
	DisplayLanguage     string
}

// Expand implements ValueSet $expand (§4.7), including inline POSTed compose (§4.7.1).
func (s *Service) Expand(ctx context.Context, params ExpandParams) (*ValueSet, *OutcomeError) {
	store, outcomeErr := s.store()
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	version, outcomeErr := s.releaseVersion(ctx, store)
	if outcomeErr != nil {
		return nil, outcomeErr
	}

	offset := 0
	if params.Offset != nil {
		if *params.Offset < 0 {
			return nil, invalidError("Parameter 'offset' must not be negative")
		}
		offset = *params.Offset
	}
	count := defaultExpandCount
	if params.Count != nil {
		if *params.Count < 0 {
			return nil, invalidError("Parameter 'count' must not be negative")
		}
		count = *params.Count
		if count > maxExpandCount {
			count = maxExpandCount
		}
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

	vs, outcomeErr := s.buildValueSet(ctx, store, version, resolved)
	if outcomeErr != nil {
		return nil, outcomeErr
	}

	var total int
	var items []loinc.FHIRExpandedConcept
	var err error
	if resolved.answerListID != "" {
		total, err = store.FHIRAnswerListExpandCount(ctx, resolved.answerListID, params.Filter)
		if err == nil && count > 0 {
			items, err = store.FHIRAnswerListExpandPage(ctx, resolved.answerListID, params.Filter, offset, count)
		}
	} else if cacheKey := resolved.cacheKey(); cacheKey != "" && params.Filter == "" && !params.ActiveOnly {
		// The common case: no filter/activeOnly means every page of this named/dynamic set comes
		// from the same unfiltered member list every time, so this pages an already-sorted,
		// already-cached slice instead of re-running SQLite's join+dedupe+sort on every request
		// (§11: was the dominant cost behind the named-set $expand budget miss).
		var members []loinc.FHIRExpandedConcept
		members, err = store.CachedExpandMembers(ctx, cacheKey, *resolved.termSource)
		if err == nil {
			total = len(members)
			if count > 0 && offset < total {
				end := offset + count
				if end > total {
					end = total
				}
				items = members[offset:end]
			}
		}
	} else {
		total, items, err = store.FHIRExpandTermSource(ctx, *resolved.termSource, params.Filter, params.ActiveOnly, offset, count)
	}
	if err != nil {
		return nil, &OutcomeError{Status: 503, Code: "exception", Text: err.Error()}
	}

	contains := make([]ExpansionContains, 0, len(items))
	for _, item := range items {
		c := ExpansionContains{System: loincSystem, Code: item.Code, Display: item.Display, Inactive: item.Inactive}
		if params.IncludeDesignations {
			c.Designation = s.designationsForExpand(ctx, store, item.Code, params.DisplayLanguage)
		}
		contains = append(contains, c)
	}

	vs.Expansion = &Expansion{
		ID:         newUUID(),
		Identifier: newUUID(),
		Timestamp:  now().UTC().Format("2006-01-02T15:04:05") + "+00:00",
		Total:      total,
		Offset:     offset,
		Parameter:  []ExpansionParameter{intParam("offset", offset), intParam("count", count)},
		Contains:   contains,
	}
	return vs, nil
}

func intParam(name string, value int) ExpansionParameter {
	v := value
	return ExpansionParameter{Name: name, ValueInteger: &v}
}

// newUUID returns a random RFC 4122 v4 UUID string, using only crypto/rand (no dependency).
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// designationsForExpand builds expansion.contains[].designation for one code (§4.7
// includeDesignations), reusing the same per-kind designation logic as $lookup (§4.3).
func (s *Service) designationsForExpand(ctx context.Context, store *loinc.Store, code, displayLanguage string) []ExpansionDesignation {
	var designations []Parameter
	switch classifyLoincCode(code) {
	case codeKindPart:
		_, _, designations, _, _ = lookupPart(ctx, store, code)
	case codeKindAnswerList:
		_, _, designations, _, _ = lookupAnswerList(ctx, store, code)
	case codeKindAnswer:
		_, _, designations, _, _ = lookupAnswer(ctx, store, code)
	case codeKindGroup:
		_, _, designations, _, _ = lookupGroup(ctx, store, code)
	default:
		_, _, designations, _, _ = lookupTerm(ctx, store, code)
	}
	designations = filterDesignationsByLanguage(designations, displayLanguage)
	out := make([]ExpansionDesignation, 0, len(designations))
	for _, d := range designations {
		var language, value string
		var use *Coding
		for _, part := range d.Part {
			switch part.Name {
			case "language":
				if part.ValueCode != nil {
					language = *part.ValueCode
				}
			case "use":
				use = part.ValueCoding
			case "value":
				if part.ValueString != nil {
					value = *part.ValueString
				}
			}
		}
		out = append(out, ExpansionDesignation{Language: language, Use: use, Value: value})
	}
	return out
}

// resolveInlineValueSet evaluates a POSTed `valueSet` inline compose (§4.7.1) into a
// resolvedValueSet backed by a single SQL term source: include entries are unioned, exclude
// entries subtracted, entirely in SQL (no row is loaded into Go to test membership).
func (s *Service) resolveInlineValueSet(ctx context.Context, store *loinc.Store, inline *InlineValueSet) (*resolvedValueSet, *OutcomeError) {
	if inline.Compose == nil || len(inline.Compose.Include) == 0 {
		return nil, requiredError("valueSet.compose.include is required")
	}
	includeSQL, includeArgs, outcomeErr := s.unionIncludes(ctx, store, inline.Compose.Include)
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	where := "t.loinc_num in (" + includeSQL + ")"
	args := includeArgs
	if len(inline.Compose.Exclude) > 0 {
		excludeSQL, excludeArgs, outcomeErr := s.unionIncludes(ctx, store, inline.Compose.Exclude)
		if outcomeErr != nil {
			return nil, outcomeErr
		}
		where += " and t.loinc_num not in (" + excludeSQL + ")"
		args = append(args, excludeArgs...)
	}
	// loinc_terms t is never joined here; the include/exclude sets are IN-subqueries against
	// t.loinc_num (its own primary key), so it can never repeat -- see NoDuplicates.
	src := loinc.FHIRTermValueSetSource{From: "loinc_terms t", Where: where, Args: args, NoDuplicates: true}
	return &resolvedValueSet{
		id: "inline", url: loincSystem + "/vs", termSource: &src, form: composeFilter,
	}, nil
}

func (s *Service) unionIncludes(ctx context.Context, store *loinc.Store, includes []ValueSetInclude) (string, []any, *OutcomeError) {
	var subqueries []string
	var args []any
	for _, inc := range includes {
		if inc.System != "" && !strings.EqualFold(inc.System, loincSystem) {
			return "", nil, notFoundError("Unsupported compose.include.system: " + inc.System)
		}
		sql, subArgs, err := s.includeSQL(ctx, store, inc)
		if err != nil {
			return "", nil, err
		}
		subqueries = append(subqueries, sql)
		args = append(args, subArgs...)
	}
	return strings.Join(subqueries, " union "), args, nil
}

func (s *Service) includeSQL(ctx context.Context, store *loinc.Store, inc ValueSetInclude) (string, []any, *OutcomeError) {
	switch {
	case len(inc.Concept) > 0:
		placeholders := make([]string, len(inc.Concept))
		args := make([]any, len(inc.Concept))
		for i, c := range inc.Concept {
			placeholders[i] = "(?)"
			args[i] = normalizeCode(c.Code)
		}
		return "select column1 as loinc_num from (values " + strings.Join(placeholders, ",") + ")", args, nil
	case len(inc.Filter) > 0:
		return s.filtersSQL(ctx, store, inc.Filter)
	case len(inc.ValueSet) > 0:
		var subqueries []string
		var args []any
		for _, ref := range inc.ValueSet {
			id := idFromURL(ref)
			if id == "" {
				id = ref
			}
			resolved, err := s.resolveValueSet(ctx, store, id)
			if err != nil || resolved.termSource == nil {
				return "", nil, notFoundError("Failed to find matching value set: " + ref)
			}
			subqueries = append(subqueries, "select t.loinc_num from "+resolved.termSource.From+" where "+resolved.termSource.Where)
			args = append(args, resolved.termSource.Args...)
		}
		return strings.Join(subqueries, " union "), args, nil
	default:
		return "select loinc_num from loinc_terms", nil, nil
	}
}

// filtersSQL AND-combines a compose.include.filter[] list into one SQL predicate over
// loinc_terms.loinc_num, evaluated entirely in SQL via correlated EXISTS subqueries (§4.7.1).
func (s *Service) filtersSQL(ctx context.Context, store *loinc.Store, filters []ValueSetIncludeFilter) (string, []any, *OutcomeError) {
	var conditions []string
	var args []any
	for _, f := range filters {
		cond, condArgs, err := s.filterCondition(ctx, store, f)
		if err != nil {
			return "", nil, err
		}
		conditions = append(conditions, cond)
		args = append(args, condArgs...)
	}
	sql := "select t.loinc_num from loinc_terms t where " + strings.Join(conditions, " and ")
	return sql, args, nil
}

func filterValues(op, value string) []string {
	if op == "in" {
		parts := strings.Split(value, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if v := strings.TrimSpace(p); v != "" {
				out = append(out, v)
			}
		}
		return out
	}
	return []string{strings.TrimSpace(value)}
}

// filterCondition builds one filter's EXISTS predicate on t.loinc_num (§4.7.1 filter table).
func (s *Service) filterCondition(ctx context.Context, store *loinc.Store, f ValueSetIncludeFilter) (string, []any, *OutcomeError) {
	values := filterValues(f.Op, f.Value)
	switch f.Property {
	case "ancestor":
		placeholders := placeholderList(len(values))
		args := make([]any, len(values))
		for i, v := range values {
			args[i] = normalizeCode(v)
		}
		return `exists (select 1 from hierarchy_subtree_terms st join hierarchy_occurrences o on o.node_id = st.node_id ` +
			`join hierarchy_concepts c on c.code = o.code where st.loinc_num = t.loinc_num and c.code in (` + placeholders + `))`, args, nil
	case "parent":
		placeholders := placeholderList(len(values))
		args := make([]any, len(values))
		for i, v := range values {
			args[i] = normalizeCode(v)
		}
		return `exists (select 1 from hierarchy_edges e ` +
			`join hierarchy_occurrences po on po.node_id = e.parent_node_id join hierarchy_concepts pc on pc.code = po.code ` +
			`join hierarchy_occurrences co on co.node_id = e.child_node_id join hierarchy_concepts cc on cc.code = co.code ` +
			`where cc.code = t.loinc_num and pc.code in (` + placeholders + `))`, args, nil
	case "child":
		// ponytail: compose evaluation here only ever selects LOINC terms (always hierarchy
		// leaves), so "does t have an immediate child equal to one of these codes" is always
		// false for a real term. Correct as far as term-only composition goes; extend with a
		// Part-valued concept source if a caller needs Part-level "child" composition.
		return "0=1", nil, nil
	case "concept":
		return s.conceptFilterCondition(f)
	case "copyright":
		return s.copyrightFilterCondition(ctx, store, f)
	default:
		return s.propertyFilterCondition(ctx, store, f, values)
	}
}

func placeholderList(n int) string {
	placeholders := make([]string, n)
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return strings.Join(placeholders, ",")
}

func (s *Service) conceptFilterCondition(f ValueSetIncludeFilter) (string, []any, *OutcomeError) {
	code := normalizeCode(f.Value)
	ancestorExists := `exists (select 1 from hierarchy_subtree_terms st join hierarchy_occurrences o on o.node_id = st.node_id ` +
		`join hierarchy_concepts c on c.code = o.code where st.loinc_num = t.loinc_num and c.code = ?)`
	switch f.Op {
	case "is-a":
		return "(t.loinc_num = ? or " + ancestorExists + ")", []any{code, code}, nil
	case "descendent-of":
		return ancestorExists, []any{code}, nil
	case "is-not-a":
		return "not (t.loinc_num = ? or " + ancestorExists + ")", []any{code, code}, nil
	default:
		return "", nil, invalidError("Unsupported concept filter operator: " + f.Op)
	}
}

func (s *Service) copyrightFilterCondition(ctx context.Context, store *loinc.Store, f ValueSetIncludeFilter) (string, []any, *OutcomeError) {
	table, ok := store.RawTable(ctx, "LoincTable/Loinc.csv")
	if !ok {
		return "0=1", nil, nil
	}
	cmp := `<> ''`
	if strings.EqualFold(strings.TrimSpace(f.Value), "LOINC") {
		cmp = `= ''`
	} else if !strings.EqualFold(strings.TrimSpace(f.Value), "3rdParty") {
		return "", nil, invalidError("Unsupported copyright filter value: " + f.Value)
	}
	return `exists (select 1 from ` + quoteSQLIdent(table) + ` r where r."LOINC_NUM" = t.loinc_num and trim(coalesce(r."EXTERNAL_COPYRIGHT_NOTICE",'')) ` + cmp + `)`, nil, nil
}

// propertyFilterCondition handles the 83 CodeSystem.property filters (§4.7.1): Coding-typed
// properties match via loinc_part_links (LP code or part name/display name); string-typed
// properties match the identically-named raw Loinc.csv column.
func (s *Service) propertyFilterCondition(ctx context.Context, store *loinc.Store, f ValueSetIncludeFilter, values []string) (string, []any, *OutcomeError) {
	prop, ok := propertyByCode(f.Property)
	if !ok {
		return "", nil, &OutcomeError{Status: 400, Code: "not-supported", Text: "Filter property '" + f.Property + "' is not supported"}
	}
	if prop.Type == "Coding" {
		var clauses []string
		var args []any
		for _, v := range values {
			if f.Op == "regex" {
				if err := loinc.ValidateRegexFilter(v); err != nil {
					return "", nil, invalidError("Invalid regex filter pattern: " + err.Error())
				}
				clauses = append(clauses, `(regexp(?, p.part_name) or regexp(?, p.part_display_name))`)
				args = append(args, v, v)
				continue
			}
			clauses = append(clauses, `(pl.part_number = ? collate nocase or p.part_name = ? collate nocase or p.part_display_name = ? collate nocase)`)
			args = append(args, v, v, v)
		}
		cond := `exists (select 1 from loinc_part_links pl left join parts p on p.part_number = pl.part_number ` +
			`where pl.loinc_num = t.loinc_num and pl.property = ? and (` + strings.Join(clauses, " or ") + `))`
		return cond, append([]any{loincSystem + "/property/" + f.Property}, args...), nil
	}

	table, ok := store.RawTable(ctx, "LoincTable/Loinc.csv")
	if !ok {
		return "0=1", nil, nil
	}
	column := quoteSQLIdent(f.Property)
	var cond string
	var args []any
	switch f.Op {
	case "=", "in":
		cond = `r.` + column + ` in (` + placeholderList(len(values)) + `)`
		for _, v := range values {
			args = append(args, v)
		}
	case "regex":
		if err := loinc.ValidateRegexFilter(values[0]); err != nil {
			return "", nil, invalidError("Invalid regex filter pattern: " + err.Error())
		}
		cond = `regexp(?, r.` + column + `)`
		args = append(args, values[0])
	default:
		return "", nil, invalidError("Unsupported filter operator: " + f.Op)
	}
	return `exists (select 1 from ` + quoteSQLIdent(table) + ` r where r."LOINC_NUM" = t.loinc_num and ` + cond + `)`, args, nil
}
