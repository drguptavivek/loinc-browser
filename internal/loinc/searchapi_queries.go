package loinc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// This file backs the local Search API clone (internal/server/searchapi.go,
// plan §5). It reads release CSVs verbatim through the raw_csv_* tables that
// ingest.go preserves (rawCSVTableName), so row shapes can mirror the
// official searchapi.regenstrief.org JSON without any schema change.

// ReleaseVersion and RawTable are provided by fhir_queries.go (same
// package); this file only adds the row/facet builders below.

func strOrNil(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func atoiOrZero(value string) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return n
}

func sqlQuoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

// tableExists reports whether a raw CSV table (or any table) is present.
// Older or partially-ingested databases can lack a raw table for an
// AccessoryFile that a newer release added; callers treat that as "no data"
// rather than an error.
func (s *Store) tableExists(ctx context.Context, table string) bool {
	var name string
	err := s.db.QueryRowContext(ctx, `select name from sqlite_master where type = 'table' and name = ?`, table).Scan(&name)
	return err == nil
}

func (s *Store) rawColumns(ctx context.Context, table string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `select name from pragma_table_info(`+sqlQuoteLiteral(table)+`) order by cid`)
	if err != nil {
		return nil, fmt.Errorf("inspect raw table %s: %w", table, err)
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	return columns, rows.Err()
}

// rawRowsWhere reads a raw CSV table filtered by an equality match (case
// insensitive) on keyCol, or the whole table when keyCol is empty. Column
// names and values come back verbatim, as the CSV stored them (blank string,
// never SQL NULL). Returns (nil, nil) when the table does not exist, so
// callers can render an empty/omitted field instead of failing the request.
func (s *Store) rawRowsWhere(ctx context.Context, table, keyCol, keyVal, orderBy string, limit int) ([]map[string]string, error) {
	if !s.tableExists(ctx, table) {
		return nil, nil
	}
	columns, err := s.rawColumns(ctx, table)
	if err != nil || len(columns) == 0 {
		return nil, err
	}
	selectCols := make([]string, len(columns))
	for i, column := range columns {
		selectCols[i] = quoteIdentifier(column)
	}
	query := "select " + strings.Join(selectCols, ", ") + " from " + quoteIdentifier(table)
	var args []any
	if keyCol != "" {
		query += " where " + quoteIdentifier(keyCol) + " = ? collate nocase"
		args = append(args, keyVal)
	}
	if orderBy != "" {
		query += " order by " + orderBy
	}
	if limit > 0 {
		query += " limit ?"
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("read raw table %s: %w", table, err)
	}
	defer rows.Close()
	var out []map[string]string
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		row := make(map[string]string, len(columns))
		for i, column := range columns {
			switch v := values[i].(type) {
			case string:
				row[column] = v
			case []byte:
				row[column] = string(v)
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) rawRow(ctx context.Context, table, keyCol, keyVal string) (map[string]string, bool, error) {
	rows, err := s.rawRowsWhere(ctx, table, keyCol, keyVal, "", 1)
	if err != nil || len(rows) == 0 {
		return nil, false, err
	}
	return rows[0], true, nil
}

// linguisticVariantTable resolves a LinguisticVariants.csv numeric ID (the
// Search API's `language` parameter) to the raw table for that language's
// per-term CSV (for example ID 15 -> AccessoryFiles/LinguisticVariants/
// deDE15LinguisticVariant.csv). Returns ok=false when the language or its
// file is not present locally.
func (s *Store) linguisticVariantTable(ctx context.Context, id string) (string, bool, error) {
	metaTable, ok := s.RawTable(ctx, "AccessoryFiles/LinguisticVariants/LinguisticVariants.csv")
	if !ok {
		return "", false, nil
	}
	row, ok, err := s.rawRow(ctx, metaTable, "ID", id)
	if err != nil || !ok {
		return "", false, err
	}
	lang := strings.ToLower(strings.TrimSpace(row["ISO_LANGUAGE"]))
	country := strings.ToUpper(strings.TrimSpace(row["ISO_COUNTRY"]))
	if lang == "" || country == "" {
		return "", false, nil
	}
	relativePath := fmt.Sprintf("AccessoryFiles/LinguisticVariants/%s%s%sLinguisticVariant.csv", lang, country, id)
	table, ok := s.RawTable(ctx, relativePath)
	if !ok {
		return "", false, nil
	}
	return table, true, nil
}

// SearchAPILoincDescription mirrors one entry of the upstream Search API's
// per-term TermDescriptions[]. Only the description carried by Loinc.csv's
// own DefinitionDescription column is reproducible locally; Url/Copyright
// are not release data and stay null.
type SearchAPILoincDescription struct {
	Description     *string `json:"Description"`
	DescriptionHtml *string `json:"DescriptionHtml"`
	Source          *string `json:"Source"`
	Sequence        int     `json:"Sequence"`
	Url             *string `json:"Url"`
	UrlDisplayText  *string `json:"UrlDisplayText"`
	Copyright       *string `json:"Copyright"`
}

// SearchAPILoincRow is one `loincs` scope result row: every LoincTable/
// Loinc.csv column (blank -> null, CLASSTYPE/ranks as numbers), plus the
// derived keys the exemplars carry (FormalName, Link, TermDescriptions).
// CodeSystems and Tags are not in the release data and are always `[]`;
// LHCForms has no local source and is always "false" (see searchapi.go's
// doc comment for the full list of documented gaps).
type SearchAPILoincRow struct {
	LOINCNum                     string                      `json:"LOINC_NUM"`
	Component                    *string                     `json:"COMPONENT"`
	Property                     *string                     `json:"PROPERTY"`
	TimeAspect                   *string                     `json:"TIME_ASPCT"`
	System                       *string                     `json:"SYSTEM"`
	Scale                        *string                     `json:"SCALE_TYP"`
	Method                       *string                     `json:"METHOD_TYP"`
	Class                        *string                     `json:"CLASS"`
	VersionLastChanged           *string                     `json:"VersionLastChanged"`
	ChangeType                   *string                     `json:"CHNG_TYPE"`
	DefinitionDescription        *string                     `json:"DefinitionDescription"`
	Status                       *string                     `json:"STATUS"`
	ClassType                    int                         `json:"CLASSTYPE"`
	Formula                      *string                     `json:"FORMULA"`
	ExampleAnswers               *string                     `json:"ExampleAnswers"`
	SurveyQuestText              *string                     `json:"SURVEY_QUEST_TEXT"`
	SurveyQuestSrc               *string                     `json:"SURVEY_QUEST_SRC"`
	UnitsRequired                *string                     `json:"UNITSREQUIRED"`
	RelatedNames2                *string                     `json:"RELATEDNAMES2"`
	ShortName                    *string                     `json:"SHORTNAME"`
	OrderObs                     *string                     `json:"ORDER_OBS"`
	HL7FieldSubfieldID           *string                     `json:"HL7_FIELD_SUBFIELD_ID"`
	ExternalCopyrightNotice      *string                     `json:"EXTERNAL_COPYRIGHT_NOTICE"`
	ExampleUnits                 *string                     `json:"EXAMPLE_UNITS"`
	LongCommonName               *string                     `json:"LONG_COMMON_NAME"`
	ExampleUcumUnits             *string                     `json:"EXAMPLE_UCUM_UNITS"`
	StatusReason                 *string                     `json:"STATUS_REASON"`
	StatusText                   *string                     `json:"STATUS_TEXT"`
	ChangeReasonPublic           *string                     `json:"CHANGE_REASON_PUBLIC"`
	CommonTestRank               int                         `json:"COMMON_TEST_RANK"`
	CommonOrderRank              int                         `json:"COMMON_ORDER_RANK"`
	CommonSITestRank             int                         `json:"COMMON_SI_TEST_RANK"`
	HL7AttachmentStructure       *string                     `json:"HL7_ATTACHMENT_STRUCTURE"`
	ExternalCopyrightLink        *string                     `json:"EXTERNAL_COPYRIGHT_LINK"`
	PanelType                    *string                     `json:"PanelType"`
	AskAtOrderEntry              *string                     `json:"AskAtOrderEntry"`
	AssociatedObservations       *string                     `json:"AssociatedObservations"`
	VersionFirstReleased         *string                     `json:"VersionFirstReleased"`
	ValidHL7AttachmentRequest    *string                     `json:"ValidHL7AttachmentRequest"`
	DisplayName                  *string                     `json:"DisplayName"`
	LHCForms                     string                      `json:"LHCForms"`
	FormalName                   string                      `json:"FormalName"`
	LinguisticVariantDisplayName *string                     `json:"LinguisticVariantDisplayName"`
	TermDescriptions             []SearchAPILoincDescription `json:"TermDescriptions"`
	CodeSystems                  []string                    `json:"CodeSystems"`
	Tags                         []string                    `json:"Tags"`
	Link                         string                      `json:"Link"`
}

// SearchAPILoincRow builds one `loincs` scope result row for loincNum. When
// languageID is non-blank and a matching LinguisticVariants file is loaded,
// the translatable fields (component/property/.../longName/relatedNames2)
// are swapped for their localized value, falling back to the base English
// value when the localized one is blank (matches loincs-hemoglobin-lang15).
func (s *Store) SearchAPILoincRow(ctx context.Context, loincNum string, languageID string) (SearchAPILoincRow, bool, error) {
	table, _ := s.RawTable(ctx, "LoincTable/Loinc.csv")
	raw, ok, err := s.rawRow(ctx, table, "LOINC_NUM", loincNum)
	if err != nil {
		return SearchAPILoincRow{}, false, err
	}
	if !ok {
		return SearchAPILoincRow{}, false, nil
	}

	component := raw["COMPONENT"]
	property := raw["PROPERTY"]
	timeAspect := raw["TIME_ASPCT"]
	system := raw["SYSTEM"]
	scale := raw["SCALE_TYP"]
	method := raw["METHOD_TYP"]
	class := raw["CLASS"]
	shortName := raw["SHORTNAME"]
	longCommonName := raw["LONG_COMMON_NAME"]
	relatedNames := raw["RELATEDNAMES2"]
	linguisticDisplayName := ""

	languageID = strings.TrimSpace(languageID)
	if languageID != "" {
		if langTable, ok, err := s.linguisticVariantTable(ctx, languageID); err == nil && ok {
			if variant, ok, err := s.rawRow(ctx, langTable, "LOINC_NUM", loincNum); err == nil && ok {
				component = firstNonBlank(variant["COMPONENT"], component)
				property = firstNonBlank(variant["PROPERTY"], property)
				timeAspect = firstNonBlank(variant["TIME_ASPCT"], timeAspect)
				system = firstNonBlank(variant["SYSTEM"], system)
				scale = firstNonBlank(variant["SCALE_TYP"], scale)
				method = firstNonBlank(variant["METHOD_TYP"], method)
				class = firstNonBlank(variant["CLASS"], class)
				shortName = firstNonBlank(variant["SHORTNAME"], shortName)
				longCommonName = firstNonBlank(variant["LONG_COMMON_NAME"], longCommonName)
				relatedNames = firstNonBlank(variant["RELATEDNAMES2"], relatedNames)
				linguisticDisplayName = variant["LinguisticVariantDisplayName"]
			}
		}
	}

	var descriptions []SearchAPILoincDescription
	if definition := strings.TrimSpace(raw["DefinitionDescription"]); definition != "" {
		descriptions = []SearchAPILoincDescription{{
			Description:     strOrNil(definition),
			DescriptionHtml: strOrNil(definition),
			Source:          strOrNil("Regenstrief LOINC"),
			Sequence:        0,
		}}
	} else {
		descriptions = []SearchAPILoincDescription{}
	}

	row := SearchAPILoincRow{
		LOINCNum:                     loincNum,
		Component:                    strOrNil(component),
		Property:                     strOrNil(property),
		TimeAspect:                   strOrNil(timeAspect),
		System:                       strOrNil(system),
		Scale:                        strOrNil(scale),
		Method:                       strOrNil(method),
		Class:                        strOrNil(class),
		VersionLastChanged:           strOrNil(raw["VersionLastChanged"]),
		ChangeType:                   strOrNil(raw["CHNG_TYPE"]),
		DefinitionDescription:        strOrNil(raw["DefinitionDescription"]),
		Status:                       strOrNil(raw["STATUS"]),
		ClassType:                    atoiOrZero(raw["CLASSTYPE"]),
		Formula:                      strOrNil(raw["FORMULA"]),
		ExampleAnswers:               strOrNil(raw["EXMPL_ANSWERS"]),
		SurveyQuestText:              strOrNil(raw["SURVEY_QUEST_TEXT"]),
		SurveyQuestSrc:               strOrNil(raw["SURVEY_QUEST_SRC"]),
		UnitsRequired:                strOrNil(raw["UNITSREQUIRED"]),
		RelatedNames2:                strOrNil(relatedNames),
		ShortName:                    strOrNil(shortName),
		OrderObs:                     strOrNil(raw["ORDER_OBS"]),
		HL7FieldSubfieldID:           strOrNil(raw["HL7_FIELD_SUBFIELD_ID"]),
		ExternalCopyrightNotice:      strOrNil(raw["EXTERNAL_COPYRIGHT_NOTICE"]),
		ExampleUnits:                 strOrNil(raw["EXAMPLE_UNITS"]),
		LongCommonName:               strOrNil(longCommonName),
		ExampleUcumUnits:             strOrNil(raw["EXAMPLE_UCUM_UNITS"]),
		StatusReason:                 strOrNil(raw["STATUS_REASON"]),
		StatusText:                   strOrNil(raw["STATUS_TEXT"]),
		ChangeReasonPublic:           strOrNil(raw["CHANGE_REASON_PUBLIC"]),
		CommonTestRank:               atoiOrZero(raw["COMMON_TEST_RANK"]),
		CommonOrderRank:              atoiOrZero(raw["COMMON_ORDER_RANK"]),
		CommonSITestRank:             atoiOrZero(raw["COMMON_SI_TEST_RANK"]),
		HL7AttachmentStructure:       strOrNil(raw["HL7_ATTACHMENT_STRUCTURE"]),
		ExternalCopyrightLink:        strOrNil(raw["EXTERNAL_COPYRIGHT_LINK"]),
		PanelType:                    strOrNil(raw["PanelType"]),
		AskAtOrderEntry:              strOrNil(raw["AskAtOrderEntry"]),
		AssociatedObservations:       strOrNil(raw["AssociatedObservations"]),
		VersionFirstReleased:         strOrNil(raw["VersionFirstReleased"]),
		ValidHL7AttachmentRequest:    strOrNil(raw["ValidHL7AttachmentRequest"]),
		DisplayName:                  strOrNil(raw["DisplayName"]),
		LHCForms:                     "false",
		FormalName:                   strings.Join([]string{component, property, timeAspect, system, scale, method}, ":"),
		LinguisticVariantDisplayName: strOrNil(linguisticDisplayName),
		TermDescriptions:             descriptions,
		CodeSystems:                  []string{},
		Tags:                         []string{},
		Link:                         "https://loinc.org/" + loincNum,
	}
	return row, true, nil
}

// SearchAPIPartRow is one `parts` scope result row.
type SearchAPIPartRow struct {
	PartNumber      string  `json:"PartNumber"`
	PartTypeName    *string `json:"PartTypeName"`
	PartName        *string `json:"PartName"`
	PartDisplayName *string `json:"PartDisplayName"`
	Status          *string `json:"Status"`
	Classlist       *string `json:"Classlist"`
	Link            string  `json:"Link"`
}

func (s *Store) SearchAPIPartRow(ctx context.Context, partNumber string) (SearchAPIPartRow, bool, error) {
	part, err := s.Part(ctx, partNumber)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SearchAPIPartRow{}, false, nil
		}
		return SearchAPIPartRow{}, false, err
	}
	classes, err := s.partClassNames(ctx, partNumber)
	if err != nil {
		return SearchAPIPartRow{}, false, err
	}
	return SearchAPIPartRow{
		PartNumber:      part.PartNumber,
		PartTypeName:    strOrNil(part.PartTypeName),
		PartName:        strOrNil(part.PartName),
		PartDisplayName: strOrNil(part.PartDisplayName),
		Status:          strOrNil(part.Status),
		Classlist:       strOrNil(strings.Join(classes, ", ")),
		Link:            "https://loinc.org/" + part.PartNumber,
	}, true, nil
}

// partClassNames looks up distinct term classes linked to a part. partNumber is always the
// canonical part_number from a prior Store.Part lookup (never raw user input), so this can bind
// it without "collate nocase": that collation would otherwise stop SQLite from using the plain
// BINARY-collated idx_loinc_part_links_part index, forcing a full table scan (~600ms for 2 rows).
func (s *Store) partClassNames(ctx context.Context, partNumber string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `select distinct t.class
		from loinc_part_links l join loinc_terms t on t.loinc_num = l.loinc_num
		where l.part_number = ? and t.class <> ''
		order by t.class`, partNumber)
	if err != nil {
		return nil, fmt.Errorf("load part classlist: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var class string
		if err := rows.Scan(&class); err != nil {
			return nil, err
		}
		out = append(out, class)
	}
	return out, rows.Err()
}

// SearchAPIAnswerRow is one entry of an answerlists row's Answers[].
type SearchAPIAnswerRow struct {
	AnswerStringId               string  `json:"AnswerStringId"`
	LocalAnswerCode              *string `json:"LocalAnswerCode"`
	LocalAnswerCodeSystem        *string `json:"LocalAnswerCodeSystem"`
	SequenceNumber               int     `json:"SequenceNumber"`
	DisplayText                  *string `json:"DisplayText"`
	ExtCodeId                    *string `json:"ExtCodeId"`
	ExtCodeDisplayName           *string `json:"ExtCodeDisplayName"`
	ExtCodeSystem                *string `json:"ExtCodeSystem"`
	ExtCodeSystemVersion         *string `json:"ExtCodeSystemVersion"`
	ExtCodeSystemCopyrightNotice *string `json:"ExtCodeSystemCopyrightNotice"`
	SubsequentTextPrompt         *string `json:"SubsequentTextPrompt"`
	Description                  *string `json:"Description"`
	Score                        *string `json:"Score"`
}

// SearchAPIAnswerListRow is one `answerlists` scope result row. The
// list-level Description is not distinct from the per-answer Description
// column in AnswerList.csv and is always null (see searchapi.go).
type SearchAPIAnswerListRow struct {
	AnswerListId                   string               `json:"AnswerListId"`
	Name                           *string              `json:"Name"`
	Description                    *string              `json:"Description"`
	LoincAnswerListOid             *string              `json:"LoincAnswerListOid"`
	ExtDefinedYN                   *string              `json:"ExtDefinedYN"`
	ExtDefinedAnswerListCodeSystem *string              `json:"ExtDefinedAnswerListCodeSystem"`
	ExtDefinedAnswerListLink       *string              `json:"ExtDefinedAnswerListLink"`
	Answers                        []SearchAPIAnswerRow `json:"Answers"`
	Link                           string               `json:"Link"`
}

func (s *Store) SearchAPIAnswerListRow(ctx context.Context, answerListID string) (SearchAPIAnswerListRow, bool, error) {
	table, _ := s.RawTable(ctx, "AccessoryFiles/AnswerFile/AnswerList.csv")
	rows, err := s.rawRowsWhere(ctx, table, "AnswerListId", answerListID, `cast("SequenceNumber" as integer) asc`, 0)
	if err != nil {
		return SearchAPIAnswerListRow{}, false, err
	}
	if len(rows) == 0 {
		return SearchAPIAnswerListRow{}, false, nil
	}
	head := rows[0]
	answers := make([]SearchAPIAnswerRow, 0, len(rows))
	for _, row := range rows {
		answers = append(answers, SearchAPIAnswerRow{
			AnswerStringId:               row["AnswerStringId"],
			LocalAnswerCode:              strOrNil(row["LocalAnswerCode"]),
			LocalAnswerCodeSystem:        strOrNil(row["LocalAnswerCodeSystem"]),
			SequenceNumber:               atoiOrZero(row["SequenceNumber"]),
			DisplayText:                  strOrNil(row["DisplayText"]),
			ExtCodeId:                    strOrNil(row["ExtCodeId"]),
			ExtCodeDisplayName:           strOrNil(row["ExtCodeDisplayName"]),
			ExtCodeSystem:                strOrNil(row["ExtCodeSystem"]),
			ExtCodeSystemVersion:         strOrNil(row["ExtCodeSystemVersion"]),
			ExtCodeSystemCopyrightNotice: strOrNil(row["ExtCodeSystemCopyrightNotice"]),
			SubsequentTextPrompt:         strOrNil(row["SubsequentTextPrompt"]),
			Description:                  strOrNil(row["Description"]),
			Score:                        strOrNil(row["Score"]),
		})
	}
	return SearchAPIAnswerListRow{
		AnswerListId:                   answerListID,
		Name:                           strOrNil(head["AnswerListName"]),
		Description:                    nil,
		LoincAnswerListOid:             strOrNil(head["AnswerListOID"]),
		ExtDefinedYN:                   strOrNil(head["ExtDefinedYN"]),
		ExtDefinedAnswerListCodeSystem: strOrNil(head["ExtDefinedAnswerListCodeSystem"]),
		ExtDefinedAnswerListLink:       strOrNil(head["ExtDefinedAnswerListLink"]),
		Answers:                        answers,
		Link:                           "https://loinc.org/" + answerListID,
	}, true, nil
}

// SearchAPIGroupLoinc is one entry of a groups row's Loincs[].
type SearchAPIGroupLoinc struct {
	LoincNumber    string  `json:"LoincNumber"`
	LongCommonName *string `json:"LongCommonName"`
}

// SearchAPIGroupRow is one `groups` scope result row.
type SearchAPIGroupRow struct {
	ParentGroupId            string                `json:"ParentGroupId"`
	ParentGroup              *string               `json:"ParentGroup"`
	GroupId                  string                `json:"GroupId"`
	Group                    *string               `json:"Group"`
	Archetype                *string               `json:"Archetype"`
	Status                   *string               `json:"STATUS"`
	VersionFirstReleased     *string               `json:"VersionFirstReleased"`
	UsageNotes               *string               `json:"UsageNotes"`
	MolecularWeightOfAnalyte *string               `json:"MolecularWeightOfAnalyte"`
	Category                 *string               `json:"Category"`
	Loincs                   []SearchAPIGroupLoinc `json:"Loincs"`
	Link                     string                `json:"Link"`
}

func (s *Store) SearchAPIGroupRow(ctx context.Context, groupID string) (SearchAPIGroupRow, bool, error) {
	group, err := s.Group(ctx, groupID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SearchAPIGroupRow{}, false, nil
		}
		return SearchAPIGroupRow{}, false, err
	}

	// group.ParentGroupID is the canonical value from the Group() row, not raw input, so this
	// binds it without "collate nocase" and uses parent_groups' own primary key.
	var parentGroupName string
	if err := s.db.QueryRowContext(ctx, `select parent_group from parent_groups where parent_group_id = ?`, group.ParentGroupID).Scan(&parentGroupName); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return SearchAPIGroupRow{}, false, fmt.Errorf("load parent group name: %w", err)
	}

	var usageNotes, molecularWeight string
	attrTable, _ := s.RawTable(ctx, "AccessoryFiles/GroupFile/GroupAttributes.csv")
	attrRows, err := s.rawRowsWhere(ctx, attrTable, "GroupId", groupID, "", 0)
	if err != nil {
		return SearchAPIGroupRow{}, false, err
	}
	for _, row := range attrRows {
		switch row["Type"] {
		case "UsageNotes":
			usageNotes = row["Value"]
		case "MolecularWeightOfAnalyte":
			molecularWeight = row["Value"]
		}
	}

	termsTable, _ := s.RawTable(ctx, "AccessoryFiles/GroupFile/GroupLoincTerms.csv")
	termRows, err := s.rawRowsWhere(ctx, termsTable, "GroupId", groupID, `cast("_row_number" as integer) asc`, 0)
	if err != nil {
		return SearchAPIGroupRow{}, false, err
	}
	var category string
	loincs := make([]SearchAPIGroupLoinc, 0, len(termRows))
	for _, row := range termRows {
		if category == "" {
			category = row["Category"]
		}
		loincs = append(loincs, SearchAPIGroupLoinc{
			LoincNumber:    row["LoincNumber"],
			LongCommonName: strOrNil(row["LongCommonName"]),
		})
	}

	return SearchAPIGroupRow{
		ParentGroupId:            group.ParentGroupID,
		ParentGroup:              strOrNil(parentGroupName),
		GroupId:                  group.GroupID,
		Group:                    strOrNil(group.GroupName),
		Archetype:                strOrNil(group.Archetype),
		Status:                   strOrNil(group.Status),
		VersionFirstReleased:     strOrNil(group.VersionFirstReleased),
		UsageNotes:               strOrNil(usageNotes),
		MolecularWeightOfAnalyte: strOrNil(molecularWeight),
		Category:                 strOrNil(category),
		Loincs:                   loincs,
		Link:                     "https://loinc.org/" + group.GroupID,
	}, true, nil
}

// SearchAPIFilterCount is one entry of a FilterCounts[facet] array.
type SearchAPIFilterCount struct {
	Label       string `json:"Label"`
	Search      string `json:"Search"`
	Count       int    `json:"Count"`
	Description string `json:"Description,omitempty"`
}

var searchAPIFilterEscape = strings.NewReplacer(
	"\\", "\\\\", "-", "\\-", "/", "\\/", "+", "\\+", "^", "\\^",
	"(", "\\(", ")", "\\)", ":", "\\:", "~", "\\~", "*", "\\*", "?", "\\?",
	"[", "\\[", "]", "\\]", "{", "\\{", "}", "\\}",
)

func searchAPIFilterSearch(field, value string) string {
	lower := strings.ToLower(field)
	if strings.ContainsAny(value, " \t") {
		return "=" + lower + ":\"" + value + "\""
	}
	return "=" + lower + ":" + searchAPIFilterEscape.Replace(value)
}

var searchAPILoincFacetFields = []struct {
	label string
	col   string
}{
	{"System", "SYSTEM"},
	{"Method", "METHOD_TYP"},
	{"Property", "PROPERTY"},
	{"Timing", "TIME_ASPCT"},
	{"Scale", "SCALE_TYP"},
	{"Class", "CLASS"},
	{"VersionFirstReleased", "VersionFirstReleased"},
	{"VersionLastChanged", "VersionLastChanged"},
	{"ClassType", "CLASSTYPE"},
	{"OrderObs", "ORDER_OBS"},
	{"PanelType", "PanelType"},
	{"HL7AttachmentStructure", "HL7_ATTACHMENT_STRUCTURE"},
	{"Status", "STATUS"},
}

// searchAPIPartDescriptionAxes maps a facet label to the loinc_part_links
// axis (parts.part_type_name) that carries a human-readable PartDisplayName
// for its codes, so counts can carry an optional Description like upstream
// (Property "ACnc" -> "Arbitrary Concentration").
var searchAPIPartDescriptionAxes = map[string]string{
	"System": "SYSTEM", "Method": "METHOD", "Property": "PROPERTY",
	"Timing": "TIME", "Scale": "SCALE", "Class": "CLASS",
}

// SearchAPILoincFilterCounts computes the loincs-scope FilterCounts facets
// (plan §5) over the full matching LOINC-number set of a query. Tags and
// CodeSystems are not reproducible from release data and are omitted; see
// internal/server/searchapi.go for the documented list.
func (s *Store) SearchAPILoincFilterCounts(ctx context.Context, loincNums []string) (map[string][]SearchAPIFilterCount, error) {
	if len(loincNums) == 0 {
		return map[string][]SearchAPIFilterCount{}, nil
	}
	table, ok := s.RawTable(ctx, "LoincTable/Loinc.csv")
	if !ok {
		return map[string][]SearchAPIFilterCount{}, nil
	}
	existing, err := s.rawColumns(ctx, table)
	if err != nil {
		return nil, err
	}
	existingSet := make(map[string]bool, len(existing))
	for _, column := range existing {
		existingSet[column] = true
	}
	var selectCols []string
	var useFields []struct{ label, col string }
	selectCols = append(selectCols, quoteIdentifier("LOINC_NUM"))
	for _, facet := range searchAPILoincFacetFields {
		if existingSet[facet.col] {
			selectCols = append(selectCols, quoteIdentifier(facet.col))
			useFields = append(useFields, facet)
		}
	}
	// LOINC numbers have no letters, so "collate nocase" was a no-op on values but still stopped
	// SQLite from using an index on LOINC_NUM, forcing a full scan of the raw Loinc.csv table for
	// every filter-counts request (~105ms measured for includefiltercounts=true). EnsureRawIndex
	// makes the IN-list lookup indexed instead.
	if err := s.ensureRawIndex(ctx, table, "LOINC_NUM"); err != nil {
		return nil, fmt.Errorf("index filter-count table: %w", err)
	}
	placeholders := make([]string, len(loincNums))
	args := make([]any, len(loincNums))
	for i, num := range loincNums {
		placeholders[i] = "?"
		args[i] = num
	}
	query := "select " + strings.Join(selectCols, ", ") + " from " + quoteIdentifier(table) +
		" where " + quoteIdentifier("LOINC_NUM") + " in (" + strings.Join(placeholders, ",") + ")"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load filter-count rows: %w", err)
	}
	defer rows.Close()
	values := make([]any, len(selectCols))
	pointers := make([]any, len(selectCols))
	for i := range values {
		pointers[i] = &values[i]
	}
	counts := make(map[string]map[string]int, len(useFields))
	for _, facet := range useFields {
		counts[facet.label] = map[string]int{}
	}
	for rows.Next() {
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		for i, facet := range useFields {
			raw := values[i+1]
			var text string
			switch v := raw.(type) {
			case string:
				text = v
			case []byte:
				text = string(v)
			}
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			counts[facet.label][text]++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	descriptions, err := s.searchAPIPartDisplayNamesByAxis(ctx)
	if err != nil {
		return nil, err
	}

	out := make(map[string][]SearchAPIFilterCount, len(counts))
	for label, byValue := range counts {
		if len(byValue) == 0 {
			continue
		}
		labels := make([]string, 0, len(byValue))
		for value := range byValue {
			labels = append(labels, value)
		}
		sort.Strings(labels)
		entries := make([]SearchAPIFilterCount, 0, len(labels))
		for _, value := range labels {
			entry := SearchAPIFilterCount{Label: value, Search: searchAPIFilterSearch(label, value), Count: byValue[value]}
			if desc := descriptions[label][value]; desc != "" && desc != value {
				entry.Description = desc
			}
			entries = append(entries, entry)
		}
		out[label] = entries
	}
	return out, nil
}

func (s *Store) searchAPIPartDisplayNamesByAxis(ctx context.Context) (map[string]map[string]string, error) {
	out := make(map[string]map[string]string, len(searchAPIPartDescriptionAxes))
	for label, partType := range searchAPIPartDescriptionAxes {
		rows, err := s.db.QueryContext(ctx, `select part_name, part_display_name from parts where part_type_name = ?`, partType)
		if err != nil {
			return nil, fmt.Errorf("load %s display names: %w", label, err)
		}
		m := map[string]string{}
		for rows.Next() {
			var name, display string
			if err := rows.Scan(&name, &display); err != nil {
				rows.Close()
				return nil, err
			}
			m[name] = display
		}
		closeErr := rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		out[label] = m
	}
	return out, nil
}
