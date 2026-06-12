package loinc

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var requiredColumns = []string{
	"LOINC_NUM", "COMPONENT", "PROPERTY", "TIME_ASPCT", "SYSTEM", "SCALE_TYP",
	"METHOD_TYP", "CLASS", "DefinitionDescription", "STATUS", "CONSUMER_NAME",
	"RELATEDNAMES2", "SHORTNAME", "ORDER_OBS", "LONG_COMMON_NAME", "DisplayName",
}

func Ingest(ctx context.Context, options IngestOptions) (IngestSummary, error) {
	if strings.TrimSpace(options.ReleaseDir) == "" {
		return IngestSummary{}, errors.New("release directory is required")
	}
	if strings.TrimSpace(options.DBPath) == "" {
		return IngestSummary{}, errors.New("database path is required")
	}

	csvPath := filepath.Join(options.ReleaseDir, "LoincTable", "Loinc.csv")
	if _, err := os.Stat(csvPath); err != nil {
		return IngestSummary{}, fmt.Errorf("find Loinc.csv: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(options.DBPath), 0o755); err != nil {
		return IngestSummary{}, fmt.Errorf("create database directory: %w", err)
	}
	_ = os.Remove(options.DBPath + "-wal")
	_ = os.Remove(options.DBPath + "-shm")
	if err := os.Remove(options.DBPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return IngestSummary{}, fmt.Errorf("replace existing database: %w", err)
	}

	db, err := sql.Open("sqlite", options.DBPath)
	if err != nil {
		return IngestSummary{}, fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := createSchema(ctx, db); err != nil {
		return IngestSummary{}, err
	}

	count, err := importLoincCSV(ctx, db, csvPath)
	if err != nil {
		return IngestSummary{}, err
	}
	if err := importOptionalReleaseFiles(ctx, db, options.ReleaseDir); err != nil {
		return IngestSummary{}, err
	}

	importedAt := time.Now().UTC()
	if _, err := db.ExecContext(ctx, `insert into import_meta(key, value) values
		('release_dir', ?),
		('term_count', ?),
		('imported_at', ?)`,
		options.ReleaseDir,
		strconv.Itoa(count),
		importedAt.Format(time.RFC3339),
	); err != nil {
		return IngestSummary{}, fmt.Errorf("write import metadata: %w", err)
	}

	return IngestSummary{
		TermCount:  count,
		DBPath:     options.DBPath,
		ReleaseDir: options.ReleaseDir,
		ImportedAt: importedAt,
	}, nil
}

func createSchema(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`pragma journal_mode = delete`,
		`pragma synchronous = normal`,
		`create table loinc_terms (
			loinc_num text primary key,
			component text not null default '',
			property text not null default '',
			time_aspect text not null default '',
			system text not null default '',
			scale text not null default '',
			method text not null default '',
			class text not null default '',
			status text not null default '',
			consumer_name text not null default '',
			related_names text not null default '',
			short_name text not null default '',
			order_obs text not null default '',
			long_common_name text not null default '',
			definition text not null default '',
			display_name text not null default '',
			common_test_rank integer not null default 0,
			common_order_rank integer not null default 0,
			raw_json text not null default '{}'
		)`,
		`create virtual table loinc_terms_fts using fts5(
			loinc_num,
			long_common_name,
			short_name,
			component,
			related_names,
			consumer_name,
			definition,
			display_name,
			system,
			property,
			scale,
			method,
			class,
			tokenize = 'unicode61'
		)`,
		`create index idx_loinc_class on loinc_terms(class)`,
		`create index idx_loinc_status on loinc_terms(status)`,
		`create index idx_loinc_system on loinc_terms(system)`,
		`create index idx_loinc_scale on loinc_terms(scale)`,
		`create index idx_loinc_property on loinc_terms(property)`,
		`create index idx_loinc_order_obs on loinc_terms(order_obs)`,
		`create table import_meta (
			key text primary key,
			value text not null
		)`,
		`create table map_to (
			loinc_num text not null,
			map_to text not null,
			comment text not null default '',
			primary key(loinc_num, map_to)
		)`,
		`create table source_organizations (
			id text primary key,
			copyright_id text not null default '',
			name text not null default '',
			copyright text not null default '',
			terms_of_use text not null default '',
			url text not null default '',
			raw_json text not null default '{}'
		)`,
		`create table term_accessories (
			id integer primary key autoincrement,
			kind text not null,
			loinc_num text not null,
			code text not null default '',
			title text not null default '',
			subtitle text not null default '',
			raw_json text not null default '{}'
		)`,
		`create index idx_term_accessories_loinc_kind on term_accessories(loinc_num, kind)`,
		`create index idx_term_accessories_kind_title on term_accessories(kind, title, code)`,
		`create index idx_term_accessories_kind_code_loinc on term_accessories(kind, code, loinc_num)`,
		`create index idx_term_accessories_kind_id on term_accessories(kind, id)`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
	}
	return nil
}

func importLoincCSV(ctx context.Context, db *sql.DB, csvPath string) (int, error) {
	file, err := os.Open(csvPath)
	if err != nil {
		return 0, fmt.Errorf("open Loinc.csv: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("read Loinc.csv header: %w", err)
	}
	index := columnIndex(header)
	for _, name := range requiredColumns {
		if _, ok := index[name]; !ok {
			return 0, fmt.Errorf("Loinc.csv missing required column %q", name)
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin import: %w", err)
	}
	defer tx.Rollback()

	termStmt, err := tx.PrepareContext(ctx, `insert into loinc_terms (
		loinc_num, component, property, time_aspect, system, scale, method, class,
		status, consumer_name, related_names, short_name, order_obs, long_common_name,
		definition, display_name, common_test_rank, common_order_rank, raw_json
	) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("prepare term insert: %w", err)
	}
	defer termStmt.Close()

	ftsStmt, err := tx.PrepareContext(ctx, `insert into loinc_terms_fts (
		loinc_num, long_common_name, short_name, component, related_names, consumer_name,
		definition, display_name, system, property, scale, method, class
	) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("prepare fts insert: %w", err)
	}
	defer ftsStmt.Close()

	count := 0
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("read Loinc.csv row %d: %w", count+2, err)
		}
		fields := recordMap(header, record)
		loincNum := fields["LOINC_NUM"]
		if loincNum == "" {
			continue
		}
		raw, err := json.Marshal(fields)
		if err != nil {
			return 0, fmt.Errorf("encode row %s: %w", loincNum, err)
		}
		values := normalizedValues(fields)
		if _, err := termStmt.ExecContext(ctx,
			loincNum,
			values.Component,
			values.Property,
			values.TimeAspect,
			values.System,
			values.Scale,
			values.Method,
			values.Class,
			values.Status,
			values.ConsumerName,
			values.RelatedNames,
			values.ShortName,
			values.OrderObs,
			values.LongCommonName,
			values.Definition,
			values.DisplayName,
			parseRank(fields["COMMON_TEST_RANK"]),
			parseRank(fields["COMMON_ORDER_RANK"]),
			string(raw),
		); err != nil {
			return 0, fmt.Errorf("insert term %s: %w", loincNum, err)
		}
		if _, err := ftsStmt.ExecContext(ctx,
			loincNum,
			values.LongCommonName,
			values.ShortName,
			values.Component,
			values.RelatedNames,
			values.ConsumerName,
			values.Definition,
			values.DisplayName,
			values.System,
			values.Property,
			values.Scale,
			values.Method,
			values.Class,
		); err != nil {
			return 0, fmt.Errorf("insert fts %s: %w", loincNum, err)
		}
		count++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit import: %w", err)
	}
	return count, nil
}

func importOptionalReleaseFiles(ctx context.Context, db *sql.DB, releaseDir string) error {
	importers := []struct {
		path string
		fn   func(context.Context, *sql.DB, string) error
	}{
		{filepath.Join(releaseDir, "LoincTable", "MapTo.csv"), importMapToCSV},
		{filepath.Join(releaseDir, "LoincTable", "SourceOrganization.csv"), importSourceOrganizationCSV},
		{filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "LoincPartLink_Primary.csv"), func(ctx context.Context, db *sql.DB, path string) error {
			return importTermAccessoryCSV(ctx, db, path, accessoryImportSpec{
				Kind: "part-primary", LOINCColumn: "LoincNumber", CodeColumn: "PartNumber", TitleColumn: "PartName",
				SubtitleColumns: []string{"PartTypeName", "LinkTypeName"},
			})
		}},
		{filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "LoincPartLink_Supplementary.csv"), func(ctx context.Context, db *sql.DB, path string) error {
			return importTermAccessoryCSV(ctx, db, path, accessoryImportSpec{
				Kind: "part-supplementary", LOINCColumn: "LoincNumber", CodeColumn: "PartNumber", TitleColumn: "PartName",
				SubtitleColumns: []string{"PartTypeName", "LinkTypeName"},
			})
		}},
		{filepath.Join(releaseDir, "AccessoryFiles", "AnswerFile", "LoincAnswerListLink.csv"), func(ctx context.Context, db *sql.DB, path string) error {
			return importTermAccessoryCSV(ctx, db, path, accessoryImportSpec{
				Kind: "answer-list", LOINCColumn: "LoincNumber", CodeColumn: "AnswerListId", TitleColumn: "AnswerListName",
				SubtitleColumns: []string{"AnswerListLinkType", "ApplicableContext"},
			})
		}},
		{filepath.Join(releaseDir, "AccessoryFiles", "PanelsAndForms", "PanelsAndForms.csv"), importPanelsAndFormsCSV},
		{filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "GroupLoincTerms.csv"), func(ctx context.Context, db *sql.DB, path string) error {
			return importTermAccessoryCSV(ctx, db, path, accessoryImportSpec{
				Kind: "group", LOINCColumn: "LoincNumber", CodeColumn: "GroupId", TitleColumn: "LongCommonName",
				SubtitleColumns: []string{"Category", "Archetype"},
			})
		}},
		{filepath.Join(releaseDir, "AccessoryFiles", "ComponentHierarchyBySystem", "ComponentHierarchyBySystem.csv"), importHierarchyCSV},
	}
	for _, importer := range importers {
		if _, err := os.Stat(importer.path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		if err := importer.fn(ctx, db, importer.path); err != nil {
			return err
		}
	}
	return nil
}

func importMapToCSV(ctx context.Context, db *sql.DB, csvPath string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `insert or ignore into map_to(loinc_num, map_to, comment) values (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	return readCSVRows(csvPath, func(header []string, record []string) error {
		fields := recordMap(header, record)
		loincNum := fields["LOINC"]
		mapTo := fields["MAP_TO"]
		if loincNum == "" || mapTo == "" {
			return nil
		}
		_, err := stmt.ExecContext(ctx, loincNum, mapTo, fields["COMMENT"])
		if err != nil {
			return fmt.Errorf("insert MapTo %s -> %s: %w", loincNum, mapTo, err)
		}
		return nil
	}, tx)
}

func importSourceOrganizationCSV(ctx context.Context, db *sql.DB, csvPath string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `insert or replace into source_organizations(
		id, copyright_id, name, copyright, terms_of_use, url, raw_json
	) values (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	return readCSVRows(csvPath, func(header []string, record []string) error {
		fields := recordMap(header, record)
		raw, err := json.Marshal(fields)
		if err != nil {
			return err
		}
		_, err = stmt.ExecContext(ctx,
			fields["ID"], fields["COPYRIGHT_ID"], fields["NAME"], fields["COPYRIGHT"], fields["TERMS_OF_USE"], fields["URL"], string(raw),
		)
		if err != nil {
			return fmt.Errorf("insert source organization %s: %w", fields["ID"], err)
		}
		return nil
	}, tx)
}

type accessoryImportSpec struct {
	Kind            string
	LOINCColumn     string
	CodeColumn      string
	TitleColumn     string
	SubtitleColumns []string
}

func importTermAccessoryCSV(ctx context.Context, db *sql.DB, csvPath string, spec accessoryImportSpec) error {
	tx, stmt, err := prepareTermAccessoryImport(ctx, db)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	defer stmt.Close()
	return readCSVRows(csvPath, func(header []string, record []string) error {
		fields := recordMap(header, record)
		return insertTermAccessory(ctx, stmt, spec.Kind, fields[spec.LOINCColumn], fields[spec.CodeColumn], fields[spec.TitleColumn], joinFields(fields, spec.SubtitleColumns), fields)
	}, tx)
}

func importPanelsAndFormsCSV(ctx context.Context, db *sql.DB, csvPath string) error {
	tx, stmt, err := prepareTermAccessoryImport(ctx, db)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	defer stmt.Close()
	return readCSVRows(csvPath, func(header []string, record []string) error {
		fields := recordMap(header, record)
		parent := fields["ParentLoinc"]
		child := fields["Loinc"]
		if child != "" {
			if err := insertTermAccessory(ctx, stmt, "panel-membership", child, parent, fields["ParentName"], "Parent panel", fields); err != nil {
				return err
			}
		}
		if parent != "" && child != "" && parent != child {
			if err := insertTermAccessory(ctx, stmt, "panel-child", parent, child, fields["LoincName"], joinFields(fields, []string{"SEQUENCE", "ObservationRequiredInPanel", "EntryType"}), fields); err != nil {
				return err
			}
		}
		return nil
	}, tx)
}

func importHierarchyCSV(ctx context.Context, db *sql.DB, csvPath string) error {
	tx, stmt, err := prepareTermAccessoryImport(ctx, db)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	defer stmt.Close()
	return readCSVRows(csvPath, func(header []string, record []string) error {
		fields := recordMap(header, record)
		code := fields["CODE"]
		if !loincNumberRegexp.MatchString(code) {
			return nil
		}
		return insertTermAccessory(ctx, stmt, "hierarchy", code, fields["IMMEDIATE_PARENT"], fields["CODE_TEXT"], fields["PATH_TO_ROOT"], fields)
	}, tx)
}

func prepareTermAccessoryImport(ctx context.Context, db *sql.DB) (*sql.Tx, *sql.Stmt, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	stmt, err := tx.PrepareContext(ctx, `insert into term_accessories(kind, loinc_num, code, title, subtitle, raw_json) values (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return nil, nil, err
	}
	return tx, stmt, nil
}

func insertTermAccessory(ctx context.Context, stmt *sql.Stmt, kind string, loincNum string, code string, title string, subtitle string, fields map[string]string) error {
	if strings.TrimSpace(loincNum) == "" {
		return nil
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	_, err = stmt.ExecContext(ctx, kind, loincNum, code, title, subtitle, string(raw))
	if err != nil {
		return fmt.Errorf("insert %s accessory for %s: %w", kind, loincNum, err)
	}
	return nil
}

func readCSVRows(csvPath string, visit func(header []string, record []string) error, tx *sql.Tx) error {
	file, err := os.Open(csvPath)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("read %s header: %w", filepath.Base(csvPath), err)
	}
	for row := 2; ; row++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read %s row %d: %w", filepath.Base(csvPath), row, err)
		}
		if err := visit(header, record); err != nil {
			return fmt.Errorf("%s row %d: %w", filepath.Base(csvPath), row, err)
		}
	}
	return tx.Commit()
}

func joinFields(fields map[string]string, names []string) string {
	var parts []string
	for _, name := range names {
		value := strings.TrimSpace(fields[name])
		if value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, " / ")
}

type normalizedTermValues struct {
	Component      string
	Property       string
	TimeAspect     string
	System         string
	Scale          string
	Method         string
	Class          string
	Status         string
	ConsumerName   string
	RelatedNames   string
	ShortName      string
	OrderObs       string
	LongCommonName string
	Definition     string
	DisplayName    string
}

func normalizedValues(fields map[string]string) normalizedTermValues {
	return normalizedTermValues{
		Component:      fields["COMPONENT"],
		Property:       fields["PROPERTY"],
		TimeAspect:     fields["TIME_ASPCT"],
		System:         fields["SYSTEM"],
		Scale:          fields["SCALE_TYP"],
		Method:         fields["METHOD_TYP"],
		Class:          fields["CLASS"],
		Status:         fields["STATUS"],
		ConsumerName:   fields["CONSUMER_NAME"],
		RelatedNames:   fields["RELATEDNAMES2"],
		ShortName:      fields["SHORTNAME"],
		OrderObs:       fields["ORDER_OBS"],
		LongCommonName: fields["LONG_COMMON_NAME"],
		Definition:     fields["DefinitionDescription"],
		DisplayName:    fields["DisplayName"],
	}
}

func columnIndex(header []string) map[string]int {
	index := make(map[string]int, len(header))
	for i, name := range header {
		index[strings.TrimSpace(name)] = i
	}
	return index
}

func recordMap(header []string, record []string) map[string]string {
	fields := make(map[string]string, len(header))
	for i, name := range header {
		if i < len(record) {
			fields[name] = record[i]
		} else {
			fields[name] = ""
		}
	}
	return fields
}

func parseRank(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return value
}
