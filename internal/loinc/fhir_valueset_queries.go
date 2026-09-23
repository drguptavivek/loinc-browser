package loinc

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ensureFHIRValueSetIndexes adds the covering index ValueSet $expand needs for numeric-LOINC
// ordering (§4.7: "2-1 < 10-1"), so paging http://loinc.org/vs stays within the ≤25ms budget. It
// is called once at store open and only ever adds an index (no schema changes).
func ensureFHIRValueSetIndexes(db *sql.DB) error {
	statements := []string{
		`create index if not exists idx_loinc_terms_numeric_order on loinc_terms(
			cast(substr(loinc_num, 1, instr(loinc_num, '-') - 1) as integer), loinc_num)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("configure FHIR ValueSet index %q: %w", statement, err)
		}
	}
	return nil
}

// numericLoincOrder is the ORDER BY expression that sorts LOINC term codes numerically
// ("2-1" before "10-1"), backed by idx_loinc_terms_numeric_order.
const numericLoincOrder = `cast(substr(t.loinc_num, 1, instr(t.loinc_num, '-') - 1) as integer), t.loinc_num`

// FHIRTermValueSetSource is a SQL FROM/WHERE fragment selecting from loinc_terms (optionally
// joined), describing one ValueSet's membership (§4.6.1, §4.7.1). Every served ValueSet other than
// an LL answer list is expressed this way, including inline POSTed compose. EmbedOrder governs
// compose.include.concept order (each named set's natural/rank/CSV order); ExpandOrder governs
// $expand's "contains" order (numeric-LOINC ascending for most sets, but plain text loinc_num
// order for LG groups — verified against valueset-expand-LG9568-9.json, which contradicts the
// plan's blanket "ascending by numeric LOINC" claim for groups; see FHIR_TERMINOLOGY_PLAN.md §4.7).
type FHIRTermValueSetSource struct {
	From        string
	Where       string
	Args        []any
	EmbedOrder  string // defaults to numericLoincOrder when blank
	ExpandOrder string // defaults to numericLoincOrder when blank
	// NoDuplicates is true when From selects loinc_terms alone (no join), so t.loinc_num -- its
	// primary key -- can never repeat and "select distinct"/"count(distinct ...)" is pure
	// overhead: it forces SQLite to fully materialize and dedupe the matched rows before applying
	// ORDER BY/LIMIT, which measured ~25x slower for a filtered, sorted few-thousand-row source
	// (§11: loinc-top-ranked $expand). Left false (dedup on, the safe default) for any source that
	// joins loinc_terms to another table, where the same term legitimately can repeat.
	NoDuplicates bool
}

func (src FHIRTermValueSetSource) embedOrder() string {
	if src.EmbedOrder != "" {
		return src.EmbedOrder
	}
	return numericLoincOrder
}

func (src FHIRTermValueSetSource) expandOrder() string {
	if src.ExpandOrder != "" {
		return src.ExpandOrder
	}
	return numericLoincOrder
}

// FHIRExpandedConcept is one code/display/inactive triple for $expand "contains" or compose
// include.concept (§4.7).
type FHIRExpandedConcept struct {
	Code     string
	Display  string
	Inactive bool
}

// FHIRCountTermSource returns the distinct member count of a term source, for compose's ≤10,000
// embedding threshold (§4.6.3) and for $expand "total" with no filter/activeOnly applied.
func (s *Store) FHIRCountTermSource(ctx context.Context, src FHIRTermValueSetSource) (int, error) {
	query := `select count(` + distinctLoincNum(src) + `) from ` + src.From + ` where ` + src.Where
	var total int
	if err := s.db.QueryRowContext(ctx, query, src.Args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count value set term source: %w", err)
	}
	return total, nil
}

// FHIREmbedTermSource returns every member in the source's natural embed order, for
// compose.include.concept. Only called when FHIRCountTermSource is within the embedding
// threshold (§4.6.3).
func (s *Store) FHIREmbedTermSource(ctx context.Context, src FHIRTermValueSetSource) ([]FHIRConceptRef, error) {
	var query string
	if src.NoDuplicates {
		query = `select t.loinc_num, t.long_common_name from ` + src.From + ` where ` + src.Where +
			` order by ` + src.embedOrder()
	} else {
		query = `select t.loinc_num, t.long_common_name from ` + src.From + ` where ` + src.Where +
			` group by t.loinc_num, t.long_common_name order by min(` + src.embedOrder() + `)`
	}
	rows, err := s.db.QueryContext(ctx, query, src.Args...)
	if err != nil {
		return nil, fmt.Errorf("embed value set term source: %w", err)
	}
	defer rows.Close()
	return scanConceptRefs(rows)
}

// CachedCountTermSource is FHIRCountTermSource, memoized per cacheKey for the lifetime of this
// Store (§11 performance budget: a named ValueSet's unfiltered member count never changes for a
// given release, but was previously recomputed by both ReadValueSet/buildValueSet and every
// $expand call -- two-plus full scans of the same join per request). cacheKey is empty for
// sources whose membership isn't stable across calls (an inline POSTed compose), which callers
// must never cache.
func (s *Store) CachedCountTermSource(ctx context.Context, cacheKey string, src FHIRTermValueSetSource) (int, error) {
	if cacheKey != "" {
		if v, ok := s.termSourceCounts.get(cacheKey); ok {
			return v, nil
		}
	}
	total, err := s.FHIRCountTermSource(ctx, src)
	if err != nil {
		return 0, err
	}
	if cacheKey != "" {
		s.termSourceCounts.set(cacheKey, total)
	}
	return total, nil
}

// CachedEmbedTermSource is FHIREmbedTermSource, memoized per cacheKey; see CachedCountTermSource.
// The returned slice is shared across callers -- read-only, never mutated by any caller today.
func (s *Store) CachedEmbedTermSource(ctx context.Context, cacheKey string, src FHIRTermValueSetSource) ([]FHIRConceptRef, error) {
	if cacheKey != "" {
		if v, ok := s.termSourceEmbeds.get(cacheKey); ok {
			return v, nil
		}
	}
	refs, err := s.FHIREmbedTermSource(ctx, src)
	if err != nil {
		return nil, err
	}
	if cacheKey != "" {
		s.termSourceEmbeds.set(cacheKey, refs)
	}
	return refs, nil
}

// CachedExpandMembers returns a term source's full, unfiltered/active-any member list in
// $expand's natural order, memoized per cacheKey (see CachedCountTermSource). It backs the
// common $expand request (no filter, no activeOnly) for a large named/dynamic set: paging that
// case by slicing an already-sorted, already-deduped cached slice is far cheaper than SQLite
// re-scanning and re-sorting the whole join on every request just to serve the next few rows
// (§11: measured ~93ms/~222ms for the RSNA playbook and valid-hl7-attachment-requests sets
// before caching; see FHIR_TERMINOLOGY_PLAN.md §10.1).
func (s *Store) CachedExpandMembers(ctx context.Context, cacheKey string, src FHIRTermValueSetSource) ([]FHIRExpandedConcept, error) {
	if cacheKey != "" {
		if v, ok := s.termSourceExpandList.get(cacheKey); ok {
			return v, nil
		}
	}
	where, args := expandTermSourceWhere(src, "", false)
	selectKeyword := "select distinct"
	if src.NoDuplicates {
		selectKeyword = "select"
	}
	query := selectKeyword + ` t.loinc_num, t.long_common_name, t.status from ` + src.From + ` where ` + where +
		` order by ` + src.expandOrder()
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load expand member list: %w", err)
	}
	defer rows.Close()
	var items []FHIRExpandedConcept
	for rows.Next() {
		var code, display, status string
		if err := rows.Scan(&code, &display, &status); err != nil {
			return nil, fmt.Errorf("scan expand member list row: %w", err)
		}
		items = append(items, FHIRExpandedConcept{Code: code, Display: display, Inactive: strings.EqualFold(status, "DEPRECATED")})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expand member list: %w", err)
	}
	if cacheKey != "" {
		s.termSourceExpandList.set(cacheKey, items)
	}
	return items, nil
}

// FHIRExpandTermSource pages a term source for $expand (§4.7): case-insensitive word-prefix
// `filter` over long_common_name (pushed to loinc_terms_fts), optional activeOnly (excludes
// STATUS=DEPRECATED only), and offset/count. total is the count after filter/activeOnly are
// applied. count<=0 returns total with no items (the "count=0" case).
func (s *Store) FHIRExpandTermSource(ctx context.Context, src FHIRTermValueSetSource, filter string, activeOnly bool, offset, count int) (int, []FHIRExpandedConcept, error) {
	where, args := expandTermSourceWhere(src, filter, activeOnly)
	base := `from ` + src.From + ` where ` + where

	var total int
	if err := s.db.QueryRowContext(ctx, `select count(`+distinctLoincNum(src)+`) `+base, args...).Scan(&total); err != nil {
		return 0, nil, fmt.Errorf("count expand source: %w", err)
	}
	if count <= 0 {
		return total, nil, nil
	}
	items, err := s.pageExpandTermSource(ctx, src, where, args, offset, count)
	if err != nil {
		return 0, nil, err
	}
	return total, items, nil
}

func expandTermSourceWhere(src FHIRTermValueSetSource, filter string, activeOnly bool) (string, []any) {
	where := src.Where
	args := append([]any{}, src.Args...)
	if q := ftsDisplayQuery(filter); q != "" {
		where += ` and t.loinc_num in (select loinc_num from loinc_terms_fts where loinc_terms_fts match ?)`
		args = append(args, q)
	}
	if activeOnly {
		where += ` and t.status <> 'DEPRECATED'`
	}
	return where, args
}

// distinctLoincNum is the count(...) argument for a term source: "*" when NoDuplicates lets a
// plain row count stand in for a distinct one (see FHIRTermValueSetSource.NoDuplicates), else the
// correct but costlier "distinct t.loinc_num".
func distinctLoincNum(src FHIRTermValueSetSource) string {
	if src.NoDuplicates {
		return "*"
	}
	return "distinct t.loinc_num"
}

func (s *Store) pageExpandTermSource(ctx context.Context, src FHIRTermValueSetSource, where string, args []any, offset, count int) ([]FHIRExpandedConcept, error) {
	selectKeyword := "select distinct"
	if src.NoDuplicates {
		selectKeyword = "select"
	}
	query := selectKeyword + ` t.loinc_num, t.long_common_name, t.status from ` + src.From + ` where ` + where +
		` order by ` + src.expandOrder() + ` limit ? offset ?`
	pageArgs := append(append([]any{}, args...), count, offset)
	rows, err := s.db.QueryContext(ctx, query, pageArgs...)
	if err != nil {
		return nil, fmt.Errorf("page expand source: %w", err)
	}
	defer rows.Close()
	var items []FHIRExpandedConcept
	for rows.Next() {
		var code, display, status string
		if err := rows.Scan(&code, &display, &status); err != nil {
			return nil, fmt.Errorf("scan expand source row: %w", err)
		}
		items = append(items, FHIRExpandedConcept{Code: code, Display: display, Inactive: strings.EqualFold(status, "DEPRECATED")})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expand source: %w", err)
	}
	return items, nil
}

// ftsDisplayQuery builds a loinc_terms_fts column-scoped, case-insensitive, all-tokens-prefix
// query over long_common_name (the "display" §4.7 filter targets), or "" when filter is blank.
func ftsDisplayQuery(filter string) string {
	q := makeFTSQuery(filter)
	if q == "" {
		return ""
	}
	return "long_common_name : " + q
}

// FHIRAnswerListExpandCount returns the total answers of an LL list (§4.7 answer-list source).
// answerListID is always normalizeCode'd (uppercased) by the caller before this is reached, so
// the equality can bind it directly: "collate nocase" here would stop SQLite from using the
// answer_list_answers WITHOUT ROWID table's own (BINARY-collated) primary key, forcing a full
// table scan of every list's answers instead of a keyed range on this one.
func (s *Store) FHIRAnswerListExpandCount(ctx context.Context, answerListID, filter string) (int, error) {
	where := `answer_list_id = ?`
	args := []any{answerListID}
	if filter != "" {
		where += ` and ` + wordPrefixLikeClause("display_text", filter, &args)
	}
	var total int
	err := s.db.QueryRowContext(ctx, `select count(*) from answer_list_answers where `+where, args...).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("count answer list %s: %w", answerListID, err)
	}
	return total, nil
}

// FHIRAnswerListExpandPage pages an LL list's answers in sequence order (§4.7: "answer lists by
// sequence"). LA answers have no DEPRECATED concept, so activeOnly never excludes any.
// answerListID is normalizeCode'd by the caller; see FHIRAnswerListExpandCount for why this must
// not use "collate nocase" (it would defeat the table's own primary key).
func (s *Store) FHIRAnswerListExpandPage(ctx context.Context, answerListID, filter string, offset, count int) ([]FHIRExpandedConcept, error) {
	where := `answer_list_id = ?`
	args := []any{answerListID}
	if filter != "" {
		where += ` and ` + wordPrefixLikeClause("display_text", filter, &args)
	}
	args = append(args, count, offset)
	rows, err := s.db.QueryContext(ctx, `
		select answer_string_id, display_text from answer_list_answers
		where `+where+` order by sequence_number, answer_string_id limit ? offset ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("page answer list %s: %w", answerListID, err)
	}
	defer rows.Close()
	var items []FHIRExpandedConcept
	for rows.Next() {
		var item FHIRExpandedConcept
		if err := rows.Scan(&item.Code, &item.Display); err != nil {
			return nil, fmt.Errorf("scan answer list row for %s: %w", answerListID, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate answer list %s: %w", answerListID, err)
	}
	return items, nil
}

// wordPrefixLikeClause appends the LIKE argument(s) for a case-insensitive, all-tokens
// word-prefix match on column (§4.7 filter, small-source variant: answer lists and groups).
// Every token of filter must prefix-match some word of column's value.
func wordPrefixLikeClause(column, filter string, args *[]any) string {
	tokens := strings.Fields(strings.ToLower(filter))
	if len(tokens) == 0 {
		return "1=1"
	}
	var clauses []string
	for _, token := range tokens {
		clauses = append(clauses, `(' '||lower(`+column+`)||' ') like ?`)
		*args = append(*args, "% "+escapeLike(token)+"%")
	}
	return "(" + strings.Join(clauses, " and ") + ")"
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// FHIRGroupCatalogEntry and FHIRAnswerListCatalogEntry are name/id rows used by ValueSet search
// (§4.6.3: "name search covers answer lists and groups").
type FHIRCatalogEntry struct {
	ID   string
	Name string
}

// FHIRSearchAnswerLists returns answer lists whose name matches, for ValueSet search.
func (s *Store) FHIRSearchAnswerLists(ctx context.Context, namePrefix, nameContains string) ([]FHIRCatalogEntry, error) {
	where, args := nameFilterClause("answer_list_name", namePrefix, nameContains)
	rows, err := s.db.QueryContext(ctx, `select answer_list_id, answer_list_name from answer_lists where `+where+` order by answer_list_name, answer_list_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("search answer lists: %w", err)
	}
	defer rows.Close()
	return scanCatalogEntries(rows)
}

// FHIRSearchGroups returns LG groups whose name matches, for ValueSet search.
func (s *Store) FHIRSearchGroups(ctx context.Context, namePrefix, nameContains string) ([]FHIRCatalogEntry, error) {
	where, args := nameFilterClause("group_name", namePrefix, nameContains)
	rows, err := s.db.QueryContext(ctx, `select group_id, group_name from loinc_groups where `+where+` order by group_name, group_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("search groups: %w", err)
	}
	defer rows.Close()
	return scanCatalogEntries(rows)
}

func nameFilterClause(column, namePrefix, nameContains string) (string, []any) {
	switch {
	case namePrefix != "":
		return "lower(" + column + ") like ? escape '\\'", []any{escapeLike(strings.ToLower(namePrefix)) + "%"}
	case nameContains != "":
		return "lower(" + column + ") like ? escape '\\'", []any{"%" + escapeLike(strings.ToLower(nameContains)) + "%"}
	default:
		return "1=1", nil
	}
}

func scanCatalogEntries(rows *sql.Rows) ([]FHIRCatalogEntry, error) {
	var items []FHIRCatalogEntry
	for rows.Next() {
		var item FHIRCatalogEntry
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, fmt.Errorf("scan catalog entry: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate catalog entries: %w", err)
	}
	return items, nil
}

// FHIRChildGroupIDs returns the LG group ids whose parent_group_id is parentGroupID, for the
// parent-group ValueSet's compose.include.valueSet[] (§4.6.2). parentGroupID is normalizeCode'd
// by the caller, so this binds it without "collate nocase" and can use idx_loinc_groups_parent.
func (s *Store) FHIRChildGroupIDs(ctx context.Context, parentGroupID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `select group_id from loinc_groups where parent_group_id = ? order by group_id`, parentGroupID)
	if err != nil {
		return nil, fmt.Errorf("load child groups for %s: %w", parentGroupID, err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan child group id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ParentGroupExists reports whether code is a parent_groups.parent_group_id (§4.6.2). code is
// normalizeCode'd by the caller, so this binds it without "collate nocase" and can use the
// table's own primary key instead of a full scan.
func (s *Store) ParentGroupExists(ctx context.Context, code string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `select exists(select 1 from parent_groups where parent_group_id = ?)`, code).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check parent group %s: %w", code, err)
	}
	return exists, nil
}

// FHIRPartByNameOrCode resolves a filter value that names a Part either by its LP code or by its
// PartName/PartDisplayName, for the Coding-typed property filters (§4.7.1). ok is false when
// nothing matches.
func (s *Store) FHIRPartByNameOrCode(ctx context.Context, value string) (partNumber string, ok bool, err error) {
	err = s.db.QueryRowContext(ctx, `
		select part_number from parts
		where part_number = ? collate nocase or part_name = ? collate nocase or part_display_name = ? collate nocase
		limit 1`, value, value, value).Scan(&partNumber)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, fmt.Errorf("resolve part %s: %w", value, err)
	}
	return partNumber, true, nil
}
