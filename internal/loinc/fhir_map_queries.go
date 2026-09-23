package loinc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// conceptMapIndexedDBs guards the lazy raw-table index creation below so it runs at most once per
// open database handle (not once per request), per §3's "raw-table indexes are created lazily on
// first use" — a package-level set keyed by *sql.DB rather than a Store field, since Store is
// owned by search.go and not one of this file's files.
var conceptMapIndexedDBs sync.Map

// ensureConceptMapIndexes adds the covering indexes the ConceptMap/$translate raw-table lookups
// need (IEEE LOINC_NUM/IEEE_CF_CODE10, playbook LoincNumber/RPID, PartRelatedCodeMapping
// PartNumber/ExtCodeId), the first time any of them is queried against this store's db handle.
// Errors are ignored: a missing raw table (older DB) already short-circuits via RawTable, and a
// failed CREATE INDEX only costs the lazy performance win, not correctness.
func ensureConceptMapIndexes(ctx context.Context, s *Store) {
	if _, loaded := conceptMapIndexedDBs.LoadOrStore(s.db, struct{}{}); loaded {
		return
	}
	if s.skipLazyIndex() {
		return
	}
	if table, ok := s.RawTable(ctx, "AccessoryFiles/LoincIeeeMedicalDeviceCodeMappingTable/LoincIeeeMedicalDeviceCodeMappingTable.csv"); ok {
		_, _ = s.db.ExecContext(ctx, `create index if not exists idx_raw_ieee_loincnum on `+quoteIdentifier(table)+`("LOINC_NUM")`)
		_, _ = s.db.ExecContext(ctx, `create index if not exists idx_raw_ieee_cfcode on `+quoteIdentifier(table)+`("IEEE_CF_CODE10")`)
	}
	if table, ok := s.RawTable(ctx, "AccessoryFiles/LoincRsnaRadiologyPlaybook/LoincRsnaRadiologyPlaybook.csv"); ok {
		_, _ = s.db.ExecContext(ctx, `create index if not exists idx_raw_playbook_loincnum on `+quoteIdentifier(table)+`("LoincNumber")`)
		_, _ = s.db.ExecContext(ctx, `create index if not exists idx_raw_playbook_rpid on `+quoteIdentifier(table)+`("RPID")`)
	}
	if table, ok := s.RawTable(ctx, "AccessoryFiles/PartFile/PartRelatedCodeMapping.csv"); ok {
		_, _ = s.db.ExecContext(ctx, `create index if not exists idx_raw_partmap_partnumber on `+quoteIdentifier(table)+`("ExtCodeSystem", "PartNumber")`)
		_, _ = s.db.ExecContext(ctx, `create index if not exists idx_raw_partmap_extcodeid on `+quoteIdentifier(table)+`("ExtCodeSystem", "ExtCodeId")`)
	}
}

// FHIRConceptMapRow is one source/target pair for a served ConceptMap group element or a
// $translate match (§4.9, §4.10). Equivalence is "" for playbook rows (no CSV column); callers
// default that to "relatedto". Comment is only populated for loinc-map-to rows.
type FHIRConceptMapRow struct {
	SourceCode, SourceDisplay string
	TargetCode, TargetDisplay string
	Equivalence               string
	Comment                   string
}

// FHIRIEEERows returns LOINC<->IEEE 11073-10101 mapping rows. Exactly one of loincNum/ieeeCode is
// normally set (a $translate lookup by one side); both blank returns every row, for the
// ConceptMap read's embedded group, capped by limit (0 = unlimited).
func (s *Store) FHIRIEEERows(ctx context.Context, loincNum, ieeeCode string, limit int) ([]FHIRConceptMapRow, error) {
	table, ok := s.RawTable(ctx, "AccessoryFiles/LoincIeeeMedicalDeviceCodeMappingTable/LoincIeeeMedicalDeviceCodeMappingTable.csv")
	if !ok {
		return nil, nil
	}
	ensureConceptMapIndexes(ctx, s)
	query := `select "LOINC_NUM", "LOINC_LONG_COMMON_NAME", "IEEE_CF_CODE10", "IEEE_REFID", "EQUIVALENCE" from ` + quoteIdentifier(table)
	var conds []string
	var args []any
	// Bind uppercased values (LOINC nums have no lowercase letters; IEEE_CF_CODE10 is numeric)
	// against the plain BINARY-collated idx_raw_ieee_loincnum/idx_raw_ieee_cfcode indexes:
	// "collate nocase" here would silently fall back to a full scan (~135ms measured), since
	// SQLite can only use an index whose own declared collation matches the predicate's.
	if loincNum != "" {
		conds = append(conds, `"LOINC_NUM" = ?`)
		args = append(args, strings.ToUpper(loincNum))
	}
	if ieeeCode != "" {
		conds = append(conds, `"IEEE_CF_CODE10" = ?`)
		args = append(args, strings.ToUpper(ieeeCode))
	}
	if len(conds) > 0 {
		query += " where " + strings.Join(conds, " and ")
	}
	query += ` order by "LOINC_NUM"`
	if limit > 0 {
		query += " limit ?"
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load IEEE mapping rows: %w", err)
	}
	defer rows.Close()
	var items []FHIRConceptMapRow
	for rows.Next() {
		var item FHIRConceptMapRow
		if err := rows.Scan(&item.SourceCode, &item.SourceDisplay, &item.TargetCode, &item.TargetDisplay, &item.Equivalence); err != nil {
			return nil, fmt.Errorf("scan IEEE mapping row: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate IEEE mapping rows: %w", err)
	}
	return items, nil
}

// FHIRPlaybookRows returns LOINC<->RadLex RPID mapping rows (RSNA Radiology Playbook), restricted
// to rows that carry an RPID (§4.9). Exactly one of loincNum/rpid is normally set; both blank
// returns every row, capped by limit (0 = unlimited).
func (s *Store) FHIRPlaybookRows(ctx context.Context, loincNum, rpid string, limit int) ([]FHIRConceptMapRow, error) {
	table, ok := s.RawTable(ctx, "AccessoryFiles/LoincRsnaRadiologyPlaybook/LoincRsnaRadiologyPlaybook.csv")
	if !ok {
		return nil, nil
	}
	ensureConceptMapIndexes(ctx, s)
	query := `select "LoincNumber", "LongCommonName", "RPID", "LongName" from ` + quoteIdentifier(table) + ` where "RPID" is not null and "RPID" <> ''`
	// See FHIRIEEERows: bind uppercased against the plain idx_raw_playbook_loincnum/
	// idx_raw_playbook_rpid indexes rather than "collate nocase", which defeats them.
	var args []any
	if loincNum != "" {
		query += ` and "LoincNumber" = ?`
		args = append(args, strings.ToUpper(loincNum))
	}
	if rpid != "" {
		query += ` and "RPID" = ?`
		args = append(args, strings.ToUpper(rpid))
	}
	query += ` order by "LoincNumber", "RPID"`
	if limit > 0 {
		query += " limit ?"
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load playbook mapping rows: %w", err)
	}
	defer rows.Close()
	var items []FHIRConceptMapRow
	for rows.Next() {
		var item FHIRConceptMapRow
		if err := rows.Scan(&item.SourceCode, &item.SourceDisplay, &item.TargetCode, &item.TargetDisplay); err != nil {
			return nil, fmt.Errorf("scan playbook mapping row: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate playbook mapping rows: %w", err)
	}
	return items, nil
}

// FHIRPartRelatedRows returns LOINC Part<->external-code mapping rows for one ExtCodeSystem
// (§4.9: radlex/rxnorm/pubchem/snomed-ct/chebi/ncbi-clinvar/ncbi-gene/ncbi-taxonomy). Exactly one
// of partNumber/extCodeID is normally set; both blank returns every row for that system, capped
// by limit (0 = unlimited).
func (s *Store) FHIRPartRelatedRows(ctx context.Context, extCodeSystem, partNumber, extCodeID string, limit int) ([]FHIRConceptMapRow, error) {
	table, ok := s.RawTable(ctx, "AccessoryFiles/PartFile/PartRelatedCodeMapping.csv")
	if !ok {
		return nil, nil
	}
	ensureConceptMapIndexes(ctx, s)
	query := `select "PartNumber", "PartName", "ExtCodeId", "ExtCodeDisplayName", "Equivalence" from ` + quoteIdentifier(table) + ` where "ExtCodeSystem" = ?`
	// Every ExtCodeId/PartNumber value in this raw table is already uppercase (verified: no row
	// has ExtCodeId != upper(ExtCodeId)); bind uppercased against the plain
	// idx_raw_partmap_partnumber/idx_raw_partmap_extcodeid composite indexes instead of using
	// "collate nocase", which defeats them.
	args := []any{extCodeSystem}
	if partNumber != "" {
		query += ` and "PartNumber" = ?`
		args = append(args, strings.ToUpper(partNumber))
	}
	if extCodeID != "" {
		query += ` and "ExtCodeId" = ?`
		args = append(args, strings.ToUpper(extCodeID))
	}
	query += ` order by "PartNumber"`
	if limit > 0 {
		query += " limit ?"
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load part-related mapping rows for %s: %w", extCodeSystem, err)
	}
	defer rows.Close()
	var items []FHIRConceptMapRow
	for rows.Next() {
		var item FHIRConceptMapRow
		if err := rows.Scan(&item.SourceCode, &item.SourceDisplay, &item.TargetCode, &item.TargetDisplay, &item.Equivalence); err != nil {
			return nil, fmt.Errorf("scan part-related mapping row for %s: %w", extCodeSystem, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate part-related mapping rows for %s: %w", extCodeSystem, err)
	}
	return items, nil
}

// FHIRMapToRows returns LOINC->LOINC replacement rows (loinc_map_to; the local "loinc-map-to"
// ConceptMap, §4.9), with each side's display resolved against loinc_terms. Exactly one of
// loincNum/targetLoincNum is normally set; both blank returns every row, capped by limit
// (0 = unlimited).
func (s *Store) FHIRMapToRows(ctx context.Context, loincNum, targetLoincNum string, limit int) ([]FHIRConceptMapRow, error) {
	query := `
		select m.loinc_num, coalesce(nullif(a.long_common_name, ''), m.loinc_num),
			m.target_loinc_num, coalesce(nullif(b.long_common_name, ''), m.target_loinc_num),
			m.comment
		from loinc_map_to m
		left join loinc_terms a on a.loinc_num = m.loinc_num
		left join loinc_terms b on b.loinc_num = m.target_loinc_num`
	var conds []string
	var args []any
	if loincNum != "" {
		conds = append(conds, `m.loinc_num = ? collate nocase`)
		args = append(args, loincNum)
	}
	if targetLoincNum != "" {
		conds = append(conds, `m.target_loinc_num = ? collate nocase`)
		args = append(args, targetLoincNum)
	}
	if len(conds) > 0 {
		query += " where " + strings.Join(conds, " and ")
	}
	query += ` order by m.loinc_num, m.target_loinc_num`
	if limit > 0 {
		query += " limit ?"
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load MapTo rows: %w", err)
	}
	defer rows.Close()
	var items []FHIRConceptMapRow
	for rows.Next() {
		var item FHIRConceptMapRow
		if err := rows.Scan(&item.SourceCode, &item.SourceDisplay, &item.TargetCode, &item.TargetDisplay, &item.Comment); err != nil {
			return nil, fmt.Errorf("scan MapTo row: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate MapTo rows: %w", err)
	}
	return items, nil
}

// FHIRPanelItem is one panel_items row resolved for Questionnaire item building (§4.11). IsGroup
// is true when the child code is itself the parent of another panel_items block (nested panel).
type FHIRPanelItem struct {
	ItemID               string
	ChildLOINCNum        string
	ChildDisplay         string
	ChildScale           string
	DisplayNameForForm   string
	Required             bool
	AnswerListIDOverride string
	IsGroup              bool
}

// FHIRPanelItems returns the direct panel_items rows of a panel LOINC, in form sequence order,
// for Questionnaire item building (§4.11). An empty result means the code is not a panel.
func (s *Store) FHIRPanelItems(ctx context.Context, parentLOINCNum string) ([]FHIRPanelItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		select p.item_id, p.child_loinc_num,
			coalesce(nullif(c.long_common_name, ''), p.child_name),
			coalesce(c.scale, ''),
			p.display_name_for_form,
			p.observation_required_in_panel,
			coalesce(p.answer_list_id_override, ''),
			exists(select 1 from panel_items g where g.parent_loinc_num = p.child_loinc_num)
		from panel_items p
		left join loinc_terms c on c.loinc_num = p.child_loinc_num
		where p.parent_loinc_num = ? collate nocase
		order by p.sequence, p.item_id`, parentLOINCNum)
	if err != nil {
		return nil, fmt.Errorf("load panel items for %s: %w", parentLOINCNum, err)
	}
	defer rows.Close()
	var items []FHIRPanelItem
	for rows.Next() {
		var item FHIRPanelItem
		var required string
		if err := rows.Scan(&item.ItemID, &item.ChildLOINCNum, &item.ChildDisplay, &item.ChildScale, &item.DisplayNameForForm, &required, &item.AnswerListIDOverride, &item.IsGroup); err != nil {
			return nil, fmt.Errorf("scan panel item for %s: %w", parentLOINCNum, err)
		}
		item.Required = strings.EqualFold(strings.TrimSpace(required), "R")
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate panel items for %s: %w", parentLOINCNum, err)
	}
	return items, nil
}

// FHIRTermPrimaryAnswerLists returns, for each of the given LOINC codes that has at least one
// linked answer list, the lexicographically first answer_list_id — used to resolve a
// Questionnaire choice item's answer list when it carries no AnswerListIdOverride (§4.11). A
// single batched query, so building a panel's items never issues one query per item.
func (s *Store) FHIRTermPrimaryAnswerLists(ctx context.Context, loincNums []string) (map[string]string, error) {
	if len(loincNums) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(loincNums))
	args := make([]any, len(loincNums))
	for i, code := range loincNums {
		placeholders[i] = "?"
		args[i] = code
	}
	rows, err := s.db.QueryContext(ctx, `
		select loinc_num, min(answer_list_id)
		from loinc_answer_list_links
		where loinc_num collate nocase in (`+strings.Join(placeholders, ",")+`)
		group by loinc_num`, args...)
	if err != nil {
		return nil, fmt.Errorf("load primary answer lists: %w", err)
	}
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var code, listID string
		if err := rows.Scan(&code, &listID); err != nil {
			return nil, fmt.Errorf("scan primary answer list: %w", err)
		}
		result[code] = listID
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate primary answer lists: %w", err)
	}
	return result, nil
}

// FHIRAnswerListAnswersBatch returns the answers of several answer lists at once, in sequence
// order and keyed by answer_list_id, so a panel's answerOption lists come from one query rather
// than one per choice item (§4.11).
func (s *Store) FHIRAnswerListAnswersBatch(ctx context.Context, answerListIDs []string) (map[string][]AnswerListAnswer, error) {
	if len(answerListIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(answerListIDs))
	args := make([]any, len(answerListIDs))
	for i, id := range answerListIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	// answerListIDs are always canonical values read back from loinc_answer_list_links /
	// panel_items (never raw user input), so this binds them without "collate nocase": that
	// collation would stop SQLite using answer_list_answers' own (BINARY-collated) primary key,
	// forcing a full scan of every list's answers for an IN-list of just a few ids.
	rows, err := s.db.QueryContext(ctx, `
		select answer_list_id, answer_string_id, local_answer_code, local_answer_code_system,
			sequence_number, display_text, ext_code_id, ext_code_display_name, ext_code_system, score
		from answer_list_answers
		where answer_list_id in (`+strings.Join(placeholders, ",")+`)
		order by answer_list_id, sequence_number`, args...)
	if err != nil {
		return nil, fmt.Errorf("load batched answer list answers: %w", err)
	}
	defer rows.Close()
	result := map[string][]AnswerListAnswer{}
	for rows.Next() {
		var item AnswerListAnswer
		if err := rows.Scan(&item.AnswerListID, &item.AnswerStringID, &item.LocalAnswerCode, &item.LocalAnswerCodeSystem, &item.SequenceNumber, &item.DisplayText, &item.ExtCodeID, &item.ExtCodeDisplayName, &item.ExtCodeSystem, &item.Score); err != nil {
			return nil, fmt.Errorf("scan batched answer list answer: %w", err)
		}
		result[item.AnswerListID] = append(result[item.AnswerListID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate batched answer list answers: %w", err)
	}
	return result, nil
}

// FHIRPanelAdditionalCopyright returns a panel's own attribution text from PanelsAndForms.csv
// (EXTERNAL_COPYRIGHT_NOTICE, falling back to AdditionalCopyright), appended to the base LOINC
// copyright in the Questionnaire resource (§4.11). ok is false when there is none.
func (s *Store) FHIRPanelAdditionalCopyright(ctx context.Context, parentLOINCNum string) (string, bool, error) {
	table, ok := s.RawTable(ctx, "AccessoryFiles/PanelsAndForms/PanelsAndForms.csv")
	if !ok {
		return "", false, nil
	}
	var notice, additional string
	err := s.db.QueryRowContext(ctx, `
		select coalesce("EXTERNAL_COPYRIGHT_NOTICE", ''), coalesce("AdditionalCopyright", '')
		from `+quoteIdentifier(table)+`
		where "ParentLoinc" = ? collate nocase
			and (coalesce("EXTERNAL_COPYRIGHT_NOTICE", '') <> '' or coalesce("AdditionalCopyright", '') <> '')
		limit 1`, parentLOINCNum).Scan(&notice, &additional)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("load panel copyright for %s: %w", parentLOINCNum, err)
	}
	if strings.TrimSpace(notice) != "" {
		return notice, true, nil
	}
	return additional, true, nil
}
