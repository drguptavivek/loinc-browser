package loinc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"
)

// skipLazyIndex reports whether s is read-only, logging the "lazy indexes disabled" notice once
// per Store the first time any ensure*Index path is skipped. Callers still answer the request
// correctly without the index, just slower (unindexed scan) until the DB is re-ingested.
func (s *Store) skipLazyIndex() bool {
	if !s.readOnly {
		return false
	}
	s.readOnlyIndexLogOnce.Do(func() {
		log.Printf("loinc: read-only store, skipping lazy CREATE INDEX (queries still served, unindexed)")
	})
	return true
}

// ensureRawIndex creates a covering index on a raw_csv_* table's key column the first time that
// (table, column) pair is used by this Store, then remembers it in s.rawIndexed so every later
// $lookup for a different LOINC number is an indexed seek instead of a full table scan. Raw
// tables are discovered at runtime via RawTable, so this can't be done once at store open like
// ensureFHIRIndexes; it has to run lazily, per table, the first time that table is read. It is
// safe to call from concurrent requests (sync.Map dedupes) and idempotent (CREATE INDEX IF NOT
// EXISTS). table and column are never user input -- callers only ever pass compile-time
// constants -- so building the statement by concatenation is safe here.
func (s *Store) ensureRawIndex(ctx context.Context, table, column string) error {
	key := table + "\x00" + column
	if _, done := s.rawIndexed.Load(key); done {
		return nil
	}
	if s.skipLazyIndex() {
		return nil
	}
	indexName := "idx_" + table + "_" + strings.ToLower(column)
	stmt := `create index if not exists ` + quoteIdentifier(indexName) + ` on ` + quoteIdentifier(table) + `(` + quoteIdentifier(column) + `)`
	if _, err := s.db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("create raw index on %s(%s): %w", table, column, err)
	}
	s.rawIndexed.Store(key, struct{}{})
	return nil
}

// EnsureRawIndex is ensureRawIndex exported for pkg/terminology (valueset.go's named-ValueSet
// sources join loinc_terms to a raw release CSV table on loincColumn; without an index that join
// is a nested-loop full scan of the raw table for every candidate term). table and column must
// still only ever be compile-time constants or values already resolved via RawTable, never raw
// user input, per ensureRawIndex's own contract.
func (s *Store) EnsureRawIndex(ctx context.Context, table, column string) error {
	return s.ensureRawIndex(ctx, table, column)
}

// ensureFHIRIndexes adds the covering indexes the FHIR read paths need, if they are not already
// present. It is called once at store open and only ever adds indexes (no schema changes).
func ensureFHIRIndexes(db *sql.DB) error {
	statements := []string{
		`create index if not exists idx_answer_list_answers_string on answer_list_answers(answer_string_id)`,
		`create index if not exists idx_loinc_map_to_loinc on loinc_map_to(loinc_num)`,
		// The queries below (v1_queries.go, search.go) compare these columns with
		// "= ? collate nocase", which SQLite can only satisfy from an index whose own
		// collation is also NOCASE -- ordinary or PK indexes on these BINARY-collation columns
		// are silently skipped in favor of a full table scan. $lookup for a term
		// (TermWithAccessories) alone was hitting six such scans (loinc_map_to,
		// loinc_part_links, loinc_answer_list_links, panel_items twice, group_loinc_terms),
		// which is the dominant cost behind multi-second $lookup latency on the full DB.
		`create index if not exists idx_loinc_terms_loinc_num_nc on loinc_terms(loinc_num collate nocase)`,
		`create index if not exists idx_loinc_map_to_loinc_nc on loinc_map_to(loinc_num collate nocase)`,
		`create index if not exists idx_loinc_map_to_target_nc on loinc_map_to(target_loinc_num collate nocase)`,
		`create index if not exists idx_loinc_part_links_loinc_nc on loinc_part_links(loinc_num collate nocase)`,
		`create index if not exists idx_loinc_answer_list_links_loinc_nc on loinc_answer_list_links(loinc_num collate nocase)`,
		`create index if not exists idx_panel_items_child_nc on panel_items(child_loinc_num collate nocase)`,
		`create index if not exists idx_panel_items_parent_nc on panel_items(parent_loinc_num collate nocase)`,
		`create index if not exists idx_group_loinc_terms_loinc_nc on group_loinc_terms(loinc_num collate nocase)`,
		`create index if not exists idx_hierarchy_concepts_loinc_nc on hierarchy_concepts(loinc_num collate nocase)`,
		`create index if not exists idx_parts_part_number_nc on parts(part_number collate nocase)`,
		`create index if not exists idx_answer_lists_id_nc on answer_lists(answer_list_id collate nocase)`,
		`create index if not exists idx_loinc_groups_group_id_nc on loinc_groups(group_id collate nocase)`,
		// FHIRChildGroupIDs filters loinc_groups by parent_group_id with no existing index.
		`create index if not exists idx_loinc_groups_parent on loinc_groups(parent_group_id)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("configure FHIR index %q: %w", statement, err)
		}
	}
	return nil
}

// ReleaseVersion returns the loaded LOINC release version, derived from import_meta.release_dir
// (basename "Loinc_2.82" -> "2.82"). Returns ErrNotFound when import_meta has no release_dir row.
func (s *Store) ReleaseVersion(ctx context.Context) (string, error) {
	var releaseDir string
	err := s.db.QueryRowContext(ctx, `select value from import_meta where key = 'release_dir'`).Scan(&releaseDir)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("load release_dir: %w", err)
	}
	base := filepath.Base(filepath.ToSlash(strings.TrimRight(releaseDir, "/")))
	version := strings.TrimPrefix(base, "Loinc_")
	if version == "" {
		return "", ErrNotFound
	}
	return version, nil
}

// ImportedAt returns when the loaded release was imported (import_meta.imported_at). Returns
// ErrNotFound when the row is absent or unparseable.
func (s *Store) ImportedAt(ctx context.Context) (time.Time, error) {
	var value string
	if err := s.db.QueryRowContext(ctx, `select value from import_meta where key = 'imported_at'`).Scan(&value); err != nil {
		return time.Time{}, ErrNotFound
	}
	importedAt, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, ErrNotFound
	}
	return importedAt, nil
}

// RawTable resolves relPath (a path relative to the release directory, e.g.
// "AccessoryFiles/LinguisticVariants/LinguisticVariants.csv") to the raw_csv_* table that holds
// that release CSV verbatim, confirming the table exists in sqlite_master. It never hard-codes
// the hash suffix rawCSVTableName appends. ok is false when the table is absent, for example on
// an older database ingested before that file existed.
func (s *Store) RawTable(ctx context.Context, relPath string) (string, bool) {
	tableName := rawCSVTableName(relPath)
	var found string
	err := s.db.QueryRowContext(ctx, `select name from sqlite_master where type = 'table' and name = ?`, tableName).Scan(&found)
	if err != nil {
		return "", false
	}
	return found, true
}

// FHIRConceptRef is a code/display pair used for $lookup Coding-valued properties (parent,
// child, answers-for, MAP_TO, category, ...).
type FHIRConceptRef struct {
	Code    string
	Display string
}

// FHIRPartLink is one loinc_part_links row resolved against parts, used to build the primary
// (axis) and supplementary Coding properties of a term $lookup.
type FHIRPartLink struct {
	LinkSet    string // "primary" or "supplementary"
	Property   string // property code: last path segment of loinc_part_links.property
	PartNumber string
	Display    string
}

// FHIRPartLinks returns every loinc_part_links row for a term, primary links first, in
// loinc_part_links.property order, for $lookup steps 1 and 2 (§4.3). loincNum must already be in
// the canonical case the table stores (callers in pkg/terminology normalize before calling), so
// the equality test can seek loinc_part_links' leading primary-key column instead of scanning.
func (s *Store) FHIRPartLinks(ctx context.Context, loincNum string) ([]FHIRPartLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		select l.link_set, l.property, l.part_number,
			coalesce(nullif(p.part_name, ''), nullif(l.part_name, ''), p.part_display_name)
		from loinc_part_links l
		left join parts p on p.part_number = l.part_number
		where l.loinc_num = ?
		order by case l.link_set when 'primary' then 0 else 1 end, l.property, l.part_number`, loincNum)
	if err != nil {
		return nil, fmt.Errorf("load part links for %s: %w", loincNum, err)
	}
	defer rows.Close()
	var items []FHIRPartLink
	for rows.Next() {
		var item FHIRPartLink
		var property string
		if err := rows.Scan(&item.LinkSet, &property, &item.PartNumber, &item.Display); err != nil {
			return nil, fmt.Errorf("scan part link for %s: %w", loincNum, err)
		}
		item.Property = lastPathSegment(property)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate part links for %s: %w", loincNum, err)
	}
	return items, nil
}

func lastPathSegment(value string) string {
	value = strings.TrimSuffix(value, "/")
	if idx := strings.LastIndex(value, "/"); idx >= 0 {
		return value[idx+1:]
	}
	return value
}

// FHIRHierarchyImmediateParents returns the distinct immediate hierarchy parents of a term or
// part code, across every occurrence of that code in the Component Hierarchy by System, for
// $lookup "parent" properties. Display is the Part's PartName (fhir.loinc.org shows LP7846-1 as
// "SPEC", not its PartDisplayName "Specimen information"), falling back to the hierarchy label.
func (s *Store) FHIRHierarchyImmediateParents(ctx context.Context, code string) ([]FHIRConceptRef, error) {
	rows, err := s.db.QueryContext(ctx, `
		select distinct pc.code, coalesce(nullif(pp.part_name, ''), pc.label)
		from hierarchy_occurrences o
		join hierarchy_concepts c on c.code = o.code
		join hierarchy_occurrences po on po.node_id = o.parent_node_id
		join hierarchy_concepts pc on pc.code = po.code
		left join parts pp on pp.part_number = pc.code
		where c.code = ?
		order by pc.code`, code)
	if err != nil {
		return nil, fmt.Errorf("load hierarchy parents for %s: %w", code, err)
	}
	defer rows.Close()
	return scanConceptRefs(rows)
}

// FHIRHierarchyImmediateChildren mirrors FHIRHierarchyImmediateParents for "child" properties.
// Only LOINC Parts have hierarchy children; LOINC terms are always leaves.
func (s *Store) FHIRHierarchyImmediateChildren(ctx context.Context, code string) ([]FHIRConceptRef, error) {
	rows, err := s.db.QueryContext(ctx, `
		select distinct cc.code, coalesce(nullif(cp.part_name, ''), cc.label)
		from hierarchy_occurrences o
		join hierarchy_concepts c on c.code = o.code
		join hierarchy_edges e on e.parent_node_id = o.node_id
		join hierarchy_occurrences co on co.node_id = e.child_node_id
		join hierarchy_concepts cc on cc.code = co.code
		left join parts cp on cp.part_number = cc.code
		where c.code = ?
		order by cc.code`, code)
	if err != nil {
		return nil, fmt.Errorf("load hierarchy children for %s: %w", code, err)
	}
	defer rows.Close()
	return scanConceptRefs(rows)
}

// FHIRTermGroups returns the LG groups that directly contain loincNum, for the term $lookup
// "parent" property (in addition to the hierarchy parent).
func (s *Store) FHIRTermGroups(ctx context.Context, loincNum string) ([]FHIRConceptRef, error) {
	rows, err := s.db.QueryContext(ctx, `
		select g.group_id, g.group_name
		from group_loinc_terms gt
		join loinc_groups g on g.group_id = gt.group_id
		where gt.loinc_num = ?
		order by g.group_id`, loincNum)
	if err != nil {
		return nil, fmt.Errorf("load groups for %s: %w", loincNum, err)
	}
	defer rows.Close()
	return scanConceptRefs(rows)
}

// FHIRSubsumes classifies the hierarchy relationship between two known codes (term or LP part)
// for CodeSystem $subsumes (§4.5): "equivalent", "subsumes", "subsumed-by", or "not-subsumed".
// Terms sit only at hierarchy leaves, so subtree membership for a term target uses
// hierarchy_subtree_terms, while a part target uses hierarchy_closure.
func (s *Store) FHIRSubsumes(ctx context.Context, codeA string, codeB string) (string, error) {
	if strings.EqualFold(codeA, codeB) {
		return "equivalent", nil
	}
	aSubsumesB, err := s.hierarchyAncestorOf(ctx, codeA, codeB)
	if err != nil {
		return "", err
	}
	if aSubsumesB {
		return "subsumes", nil
	}
	bSubsumesA, err := s.hierarchyAncestorOf(ctx, codeB, codeA)
	if err != nil {
		return "", err
	}
	if bSubsumesA {
		return "subsumed-by", nil
	}
	return "not-subsumed", nil
}

// hierarchyAncestorOf reports whether any hierarchy occurrence of ancestorCode is a strict
// ancestor of any occurrence of descendantCode.
func (s *Store) hierarchyAncestorOf(ctx context.Context, ancestorCode string, descendantCode string) (bool, error) {
	isTerm, err := s.isHierarchyTerm(ctx, descendantCode)
	if err != nil {
		return false, err
	}
	var exists bool
	if isTerm {
		err = s.db.QueryRowContext(ctx, `
			select exists(
				select 1
				from hierarchy_subtree_terms st
				join hierarchy_occurrences o on o.node_id = st.node_id
				join hierarchy_concepts c on c.code = o.code
				where c.code = ? and st.loinc_num = ?
			)`, ancestorCode, descendantCode).Scan(&exists)
	} else {
		err = s.db.QueryRowContext(ctx, `
			select exists(
				select 1
				from hierarchy_closure cl
				join hierarchy_occurrences ao on ao.node_id = cl.ancestor_node_id
				join hierarchy_concepts ac on ac.code = ao.code
				join hierarchy_occurrences do_ on do_.node_id = cl.descendant_node_id
				join hierarchy_concepts dc on dc.code = do_.code
				where ac.code = ? and dc.code = ? and cl.depth > 0
			)`, ancestorCode, descendantCode).Scan(&exists)
	}
	if err != nil {
		return false, fmt.Errorf("test hierarchy ancestry %s -> %s: %w", ancestorCode, descendantCode, err)
	}
	return exists, nil
}

func (s *Store) isHierarchyTerm(ctx context.Context, code string) (bool, error) {
	var nodeKind string
	err := s.db.QueryRowContext(ctx, `select node_kind from hierarchy_concepts where code = ?`, code).Scan(&nodeKind)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("load hierarchy concept kind for %s: %w", code, err)
	}
	return nodeKind == "term", nil
}

// FHIRHierarchyConceptLabel returns the display label of a hierarchy_concepts row that has no
// matching parts row (node_kind = "hierarchy_only" -- a Component Hierarchy by System node that
// groups other parts but was never itself published as a Part.csv row, e.g. LP31448-1). ok is
// false when there is no hierarchy_concepts row for code either, so the caller can fall through
// to the ordinary not-found path.
func (s *Store) FHIRHierarchyConceptLabel(ctx context.Context, code string) (string, bool, error) {
	var label string
	err := s.db.QueryRowContext(ctx, `select label from hierarchy_concepts where code = ?`, code).Scan(&label)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("load hierarchy concept %s: %w", code, err)
	}
	return label, true, nil
}

// FHIRAnswer returns every answer_list_answers row for an LA answer-string code, across every
// answer list that uses it, in answer-list-id order. Used for $lookup LA (display/Score/
// SequenceNumber from the first row, "parent" Coding per row) and for $validate-code display
// matching.
func (s *Store) FHIRAnswer(ctx context.Context, answerStringID string) ([]AnswerListAnswer, error) {
	rows, err := s.db.QueryContext(ctx, `
		select answer_list_id, answer_string_id, local_answer_code, local_answer_code_system,
			sequence_number, display_text, ext_code_id, ext_code_display_name, ext_code_system, score
		from answer_list_answers
		where answer_string_id = ?
		order by answer_list_id, sequence_number`, answerStringID)
	if err != nil {
		return nil, fmt.Errorf("load answer rows for %s: %w", answerStringID, err)
	}
	defer rows.Close()
	var items []AnswerListAnswer
	for rows.Next() {
		var item AnswerListAnswer
		if err := rows.Scan(&item.AnswerListID, &item.AnswerStringID, &item.LocalAnswerCode, &item.LocalAnswerCodeSystem, &item.SequenceNumber, &item.DisplayText, &item.ExtCodeID, &item.ExtCodeDisplayName, &item.ExtCodeSystem, &item.Score); err != nil {
			return nil, fmt.Errorf("scan answer row for %s: %w", answerStringID, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate answer rows for %s: %w", answerStringID, err)
	}
	return items, nil
}

// FHIRAnswerListAnswersFor returns the terms an LL answer list answers for
// (loinc_answer_list_links), for the LL $lookup "answers-for" property.
func (s *Store) FHIRAnswerListAnswersFor(ctx context.Context, answerListID string) ([]FHIRConceptRef, error) {
	rows, err := s.db.QueryContext(ctx, `
		select l.loinc_num, coalesce(nullif(t.long_common_name, ''), l.answer_list_name)
		from loinc_answer_list_links l
		join loinc_terms t on t.loinc_num = l.loinc_num
		where l.answer_list_id = ?
		order by l.loinc_num`, answerListID)
	if err != nil {
		return nil, fmt.Errorf("load answers-for terms for %s: %w", answerListID, err)
	}
	defer rows.Close()
	return scanConceptRefs(rows)
}

// FHIRAnswerListAnswers returns the LA answers of an answer list in sequence order, for the LL
// $lookup "child" property.
func (s *Store) FHIRAnswerListAnswers(ctx context.Context, answerListID string) ([]AnswerListAnswer, error) {
	rows, err := s.db.QueryContext(ctx, `
		select answer_list_id, answer_string_id, local_answer_code, local_answer_code_system,
			sequence_number, display_text, ext_code_id, ext_code_display_name, ext_code_system, score
		from answer_list_answers
		where answer_list_id = ?
		order by sequence_number, answer_string_id`, answerListID)
	if err != nil {
		return nil, fmt.Errorf("load answers for list %s: %w", answerListID, err)
	}
	defer rows.Close()
	var items []AnswerListAnswer
	for rows.Next() {
		var item AnswerListAnswer
		if err := rows.Scan(&item.AnswerListID, &item.AnswerStringID, &item.LocalAnswerCode, &item.LocalAnswerCodeSystem, &item.SequenceNumber, &item.DisplayText, &item.ExtCodeID, &item.ExtCodeDisplayName, &item.ExtCodeSystem, &item.Score); err != nil {
			return nil, fmt.Errorf("scan answer for list %s: %w", answerListID, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate answers for list %s: %w", answerListID, err)
	}
	return items, nil
}

// FHIRGroupMembers returns the terms that are direct members of an LG group, for the LG
// $lookup "child" property.
func (s *Store) FHIRGroupMembers(ctx context.Context, groupID string) ([]FHIRConceptRef, error) {
	rows, err := s.db.QueryContext(ctx, `
		select gt.loinc_num, coalesce(nullif(t.long_common_name, ''), gt.long_common_name)
		from group_loinc_terms gt
		join loinc_terms t on t.loinc_num = gt.loinc_num
		where gt.group_id = ?
		order by gt.loinc_num`, groupID)
	if err != nil {
		return nil, fmt.Errorf("load group members for %s: %w", groupID, err)
	}
	defer rows.Close()
	return scanConceptRefs(rows)
}

// FHIRGroupParent returns the parent group of an LG group (parent_groups), for the LG $lookup
// "parent" property. ok is false when the group has no parent group id set.
func (s *Store) FHIRGroupParent(ctx context.Context, groupID string) (FHIRConceptRef, bool, error) {
	var ref FHIRConceptRef
	err := s.db.QueryRowContext(ctx, `
		select pg.parent_group_id, pg.parent_group
		from loinc_groups g
		join parent_groups pg on pg.parent_group_id = g.parent_group_id
		where g.group_id = ? and pg.parent_group_id <> ''`, groupID).Scan(&ref.Code, &ref.Display)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return FHIRConceptRef{}, false, nil
		}
		return FHIRConceptRef{}, false, fmt.Errorf("load group parent for %s: %w", groupID, err)
	}
	return ref, true, nil
}

// FHIRLinguisticVariant is one row of a per-language LinguisticVariants CSV, joined against
// LinguisticVariants.csv's ID -> language mapping, for a single LOINC term.
type FHIRLinguisticVariant struct {
	Language                     string // e.g. "de-DE"
	Component, Property, System  string
	TimeAspect, Scale, Method    string
	ShortName, LongCommonName    string
	RelatedNames                 string
	LinguisticVariantDisplayName string
}

// FHIRLinguisticVariants returns every linguistic-variant translation of loincNum, one row per
// language that has a row for this term, for the term $lookup non-English designations (§4.3).
// It reads AccessoryFiles/LinguisticVariants/LinguisticVariants.csv for the ID -> language
// mapping, then the per-language file named "{lang}{COUNTRY}{ID}LinguisticVariant.csv" resolved
// through RawTable (never a hard-coded hash suffix).
func (s *Store) FHIRLinguisticVariants(ctx context.Context, loincNum string) ([]FHIRLinguisticVariant, error) {
	union, err := s.buildLinguisticVariantUnionOnce(ctx)
	if err != nil {
		return nil, err
	}
	if union.sql == "" {
		return nil, nil
	}
	args := make([]any, len(union.languages))
	for i := range union.languages {
		args[i] = loincNum
	}
	rows, err := s.db.QueryContext(ctx, union.sql, args...)
	if err != nil {
		return nil, fmt.Errorf("load linguistic variants for %s: %w", loincNum, err)
	}
	defer rows.Close()
	var items []FHIRLinguisticVariant
	for rows.Next() {
		var seq int
		var item FHIRLinguisticVariant
		if err := rows.Scan(&seq, &item.Language, &item.Component, &item.Property, &item.TimeAspect,
			&item.System, &item.Scale, &item.Method, &item.ShortName, &item.LongCommonName,
			&item.RelatedNames, &item.LinguisticVariantDisplayName); err != nil {
			return nil, fmt.Errorf("scan linguistic variant row for %s: %w", loincNum, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate linguistic variants for %s: %w", loincNum, err)
	}
	return items, nil
}

// linguisticVariantUnionQuery is the built-once "select ... from table1 where LOINC_NUM=? union
// all select ... from table2 where LOINC_NUM=? ... order by seq" query FHIRLinguisticVariants
// runs, plus the language list that fixes its parameter count and branch order.
type linguisticVariantUnionQuery struct {
	sql       string
	languages []string // "iso-COUNTRY", in the same order as LinguisticVariants.csv's ID column
}

// buildLinguisticVariantUnionOnce builds linguisticVariantUnion the first time any term is
// looked up, then reuses it for the life of this Store: term $lookup previously ran one query per
// language table (~20 round trips) for every single code, which dominated $lookup's warm p99
// (§11; item 7 of the FHIR terminology review). Table names come only from RawTable resolution
// (never user input), same as ensureRawIndex's own contract.
func (s *Store) buildLinguisticVariantUnionOnce(ctx context.Context) (linguisticVariantUnionQuery, error) {
	var buildErr error
	s.linguisticVariantUnionOnce.Do(func() {
		s.linguisticVariantUnion, buildErr = s.buildLinguisticVariantUnion(ctx)
	})
	return s.linguisticVariantUnion, buildErr
}

func (s *Store) buildLinguisticVariantUnion(ctx context.Context) (linguisticVariantUnionQuery, error) {
	indexTable, ok := s.RawTable(ctx, "AccessoryFiles/LinguisticVariants/LinguisticVariants.csv")
	if !ok {
		return linguisticVariantUnionQuery{}, nil
	}
	rows, err := s.db.QueryContext(ctx, `select "ID", "ISO_LANGUAGE", "ISO_COUNTRY" from `+quoteIdentifier(indexTable)+` order by "ID"`)
	if err != nil {
		return linguisticVariantUnionQuery{}, fmt.Errorf("load linguistic variant languages: %w", err)
	}
	defer rows.Close()
	type language struct{ id, iso, country string }
	var languages []language
	for rows.Next() {
		var l language
		if err := rows.Scan(&l.id, &l.iso, &l.country); err != nil {
			return linguisticVariantUnionQuery{}, fmt.Errorf("scan linguistic variant language: %w", err)
		}
		languages = append(languages, l)
	}
	if err := rows.Err(); err != nil {
		return linguisticVariantUnionQuery{}, fmt.Errorf("iterate linguistic variant languages: %w", err)
	}

	const columns = `coalesce("COMPONENT",''), coalesce("PROPERTY",''), coalesce("TIME_ASPCT",''),
		coalesce("SYSTEM",''), coalesce("SCALE_TYP",''), coalesce("METHOD_TYP",''),
		coalesce("SHORTNAME",''), coalesce("LONG_COMMON_NAME",''),
		coalesce("RELATEDNAMES2",''), coalesce("LinguisticVariantDisplayName",'')`

	var branches []string
	var codes []string
	for _, l := range languages {
		iso := strings.ToLower(strings.TrimSpace(l.iso))
		country := strings.ToUpper(strings.TrimSpace(l.country))
		if iso == "" || country == "" {
			continue
		}
		relPath := fmt.Sprintf("AccessoryFiles/LinguisticVariants/%s%s%sLinguisticVariant.csv", iso, country, l.id)
		table, ok := s.RawTable(ctx, relPath)
		if !ok {
			continue
		}
		// Every branch is bound by LOINC_NUM; ensureRawIndex makes each an indexed point lookup
		// instead of a full scan of that language's file.
		if err := s.ensureRawIndex(ctx, table, "LOINC_NUM"); err != nil {
			return linguisticVariantUnionQuery{}, err
		}
		seq := len(codes)
		code := iso + "-" + country
		codes = append(codes, code)
		branches = append(branches, fmt.Sprintf(
			`select %d as seq, %s as language, %s from %s where "LOINC_NUM" = ?`,
			seq, quoteSQLLiteral(code), columns, quoteIdentifier(table)))
	}
	if len(branches) == 0 {
		return linguisticVariantUnionQuery{}, nil
	}
	sqlText := strings.Join(branches, " union all ") + " order by seq"
	return linguisticVariantUnionQuery{sql: sqlText, languages: codes}, nil
}

// quoteSQLLiteral quotes value as a single-quoted SQL text literal. value is always a
// "iso-COUNTRY" language code assembled from LinguisticVariants.csv (never user input).
func quoteSQLLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

// FHIRConsumerName returns the ConsumerName accessory file's designation for loincNum. ok is
// false when the raw ConsumerName table or a row for this term is absent. This is distinct from
// loinc_terms.consumer_name (imported from Loinc.csv CONSUMER_NAME), which is usually blank; the
// ConsumerName accessory file is the one upstream actually publishes as the "ConsumerName"
// designation.
func (s *Store) FHIRConsumerName(ctx context.Context, loincNum string) (string, bool, error) {
	table, ok := s.RawTable(ctx, "AccessoryFiles/ConsumerName/ConsumerName.csv")
	if !ok {
		return "", false, nil
	}
	if err := s.ensureRawIndex(ctx, table, "LoincNumber"); err != nil {
		return "", false, err
	}
	var name string
	err := s.db.QueryRowContext(ctx, `select "ConsumerName" from `+quoteIdentifier(table)+` where "LoincNumber" = ? limit 1`, loincNum).Scan(&name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("load consumer name for %s: %w", loincNum, err)
	}
	if strings.TrimSpace(name) == "" {
		return "", false, nil
	}
	return name, true, nil
}

// FHIRLoincRow returns the full, verbatim Loinc.csv row for loincNum (every release column,
// keyed by its exact CSV header), for the string-valued $lookup properties of step 3 (§4.3).
// ok is false when the raw Loinc.csv table or the row is absent.
func (s *Store) FHIRLoincRow(ctx context.Context, loincNum string) (map[string]string, bool, error) {
	table, ok := s.RawTable(ctx, "LoincTable/Loinc.csv")
	if !ok {
		return nil, false, nil
	}
	columnRows, err := s.db.QueryContext(ctx, `select name from pragma_table_info(?) order by cid`, table)
	if err != nil {
		return nil, false, fmt.Errorf("load Loinc.csv columns: %w", err)
	}
	var columns []string
	for columnRows.Next() {
		var name string
		if err := columnRows.Scan(&name); err != nil {
			columnRows.Close()
			return nil, false, fmt.Errorf("scan Loinc.csv column: %w", err)
		}
		if name == "_row_number" {
			continue
		}
		columns = append(columns, name)
	}
	if err := columnRows.Err(); err != nil {
		columnRows.Close()
		return nil, false, fmt.Errorf("iterate Loinc.csv columns: %w", err)
	}
	columnRows.Close()
	if len(columns) == 0 {
		return nil, false, nil
	}
	quoted := make([]string, len(columns))
	for i, column := range columns {
		quoted[i] = quoteIdentifier(column)
	}
	values := make([]any, len(columns))
	pointers := make([]any, len(columns))
	for i := range values {
		pointers[i] = &values[i]
	}
	if err := s.ensureRawIndex(ctx, table, "LOINC_NUM"); err != nil {
		return nil, false, err
	}
	query := `select ` + strings.Join(quoted, ", ") + ` from ` + quoteIdentifier(table) + ` where "LOINC_NUM" = ? limit 1`
	if err := s.db.QueryRowContext(ctx, query, loincNum).Scan(pointers...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("load Loinc.csv row for %s: %w", loincNum, err)
	}
	row := make(map[string]string, len(columns))
	for i, column := range columns {
		if s, ok := values[i].(string); ok {
			row[column] = s
		}
	}
	return row, true, nil
}

func scanConceptRefs(rows *sql.Rows) ([]FHIRConceptRef, error) {
	var items []FHIRConceptRef
	for rows.Next() {
		var item FHIRConceptRef
		if err := rows.Scan(&item.Code, &item.Display); err != nil {
			return nil, fmt.Errorf("scan concept ref: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate concept refs: %w", err)
	}
	return items, nil
}
