package loinc

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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
	db.SetMaxOpenConns(1)

	if err := configureIngestPragmas(ctx, db); err != nil {
		return IngestSummary{}, err
	}
	if err := createSchema(ctx, db); err != nil {
		return IngestSummary{}, err
	}
	if err := importRawCSVTables(ctx, db, options.ReleaseDir); err != nil {
		return IngestSummary{}, err
	}

	count, err := importLoincCSV(ctx, db, csvPath)
	if err != nil {
		return IngestSummary{}, err
	}
	if err := importRequiredReleaseFiles(ctx, db, options.ReleaseDir); err != nil {
		return IngestSummary{}, err
	}
	if err := finalizeIngestDatabase(ctx, db); err != nil {
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

func configureIngestPragmas(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`pragma foreign_keys = off`,
		`pragma journal_mode = off`,
		`pragma synchronous = off`,
		`pragma temp_store = memory`,
		`pragma cache_size = -262144`,
		`pragma locking_mode = exclusive`,
		`pragma mmap_size = 268435456`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("configure ingest sqlite pragma %q: %w", statement, err)
		}
	}
	return nil
}

func createSchema(ctx context.Context, db *sql.DB) error {
	statements := []string{
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
			common_order_rank integer not null default 0
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
		`create table import_meta (
			key text primary key,
			value text not null
		) without rowid`,
		`create table parts (
			part_number text primary key,
			part_type_name text not null default '',
			part_name text not null default '',
			part_display_name text not null default '',
			status text not null default ''
		) without rowid`,
		`create table loinc_part_links (
			loinc_num text not null references loinc_terms(loinc_num),
			part_number text not null references parts(part_number),
			link_set text not null,
			part_name text not null default '',
			part_code_system text not null default '',
			part_type_name text not null default '',
			link_type_name text not null default '',
			property text not null default '',
			primary key(loinc_num, part_number, link_set, link_type_name, property)
		) without rowid`,
		`create table answer_lists (
			answer_list_id text primary key,
			answer_list_name text not null default '',
			answer_list_oid text not null default '',
			ext_defined_yn text not null default '',
			ext_defined_answer_list_code_system text not null default '',
			ext_defined_answer_list_link text not null default ''
		) without rowid`,
		`create table answer_list_answers (
			answer_list_id text not null references answer_lists(answer_list_id),
			answer_string_id text not null default '',
			local_answer_code text not null default '',
			local_answer_code_system text not null default '',
			sequence_number integer not null default 0,
			display_text text not null default '',
			ext_code_id text not null default '',
			ext_code_display_name text not null default '',
			ext_code_system text not null default '',
			ext_code_system_version text not null default '',
			subsequent_text_prompt text not null default '',
			description text not null default '',
			score text not null default '',
			primary key(answer_list_id, sequence_number, answer_string_id, display_text)
		) without rowid`,
		`create table loinc_answer_list_links (
			loinc_num text not null references loinc_terms(loinc_num),
			answer_list_id text not null references answer_lists(answer_list_id),
			answer_list_name text not null default '',
			answer_list_link_type text not null default '',
			applicable_context text not null default '',
			primary key(loinc_num, answer_list_id, answer_list_link_type, applicable_context)
		) without rowid`,
		`create table loinc_map_to (
			loinc_num text not null references loinc_terms(loinc_num),
			target_loinc_num text not null references loinc_terms(loinc_num),
			comment text not null default '',
			primary key(loinc_num, target_loinc_num)
		) without rowid`,
		`create table panel_items (
			parent_loinc_num text not null references loinc_terms(loinc_num),
			child_loinc_num text not null references loinc_terms(loinc_num),
			parent_id text not null default '',
			item_id text not null default '',
			sequence integer not null default 0,
			parent_name text not null default '',
			child_name text not null default '',
			display_name_for_form text not null default '',
			observation_required_in_panel text not null default '',
			entry_type text not null default '',
			data_type_in_form text not null default '',
			answer_list_id_override text references answer_lists(answer_list_id),
			primary key(parent_loinc_num, sequence, child_loinc_num, item_id)
		) without rowid`,
		`create table parent_groups (
			parent_group_id text primary key,
			parent_group text not null default '',
			status text not null default ''
		) without rowid`,
		`create table loinc_groups (
			group_id text primary key,
			parent_group_id text not null references parent_groups(parent_group_id),
			group_name text not null default '',
			archetype text not null default '',
			status text not null default '',
			version_first_released text not null default ''
		) without rowid`,
		`create table group_loinc_terms (
			group_id text not null references loinc_groups(group_id),
			loinc_num text not null references loinc_terms(loinc_num),
			category text not null default '',
			archetype text not null default '',
			long_common_name text not null default '',
			primary key(group_id, loinc_num)
		) without rowid`,
		`create table hierarchy_concepts (
			code text primary key,
			label text not null default '',
			node_kind text not null default 'hierarchy_only',
			loinc_num text references loinc_terms(loinc_num),
			part_number text references parts(part_number)
		) without rowid`,
		`create table hierarchy_occurrences (
			node_id integer primary key autoincrement,
			code text not null references hierarchy_concepts(code),
			parent_node_id integer references hierarchy_occurrences(node_id),
			path_key text not null,
			occurrence_ordinal integer not null default 1,
			path_to_root text not null default '',
			sequence integer not null default 0,
			depth integer not null default 0,
			direct_term_count integer not null default 0,
			subtree_term_count integer not null default 0,
			unique(path_key, occurrence_ordinal)
		)`,
		`create table hierarchy_edges (
			parent_node_id integer not null references hierarchy_occurrences(node_id),
			child_node_id integer not null references hierarchy_occurrences(node_id),
			sequence integer not null default 0,
			primary key(parent_node_id, child_node_id)
		) without rowid`,
		`create table hierarchy_closure (
			ancestor_node_id integer not null references hierarchy_occurrences(node_id),
			descendant_node_id integer not null references hierarchy_occurrences(node_id),
			depth integer not null,
			primary key(ancestor_node_id, descendant_node_id)
		) without rowid`,
		`create index idx_hierarchy_closure_descendant on hierarchy_closure(descendant_node_id, ancestor_node_id)`,
		`create table hierarchy_subtree_terms (
			node_id integer not null references hierarchy_occurrences(node_id),
			loinc_num text not null references loinc_terms(loinc_num),
			descendant_node_id integer not null references hierarchy_occurrences(node_id),
			distance integer not null,
			primary key(node_id, distance, loinc_num, descendant_node_id)
		) without rowid`,
		`create table source_organizations (
			id text primary key,
			copyright_id text not null default '',
			name text not null default '',
			copyright text not null default '',
			terms_of_use text not null default '',
			url text not null default ''
		) without rowid`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
	}
	return nil
}

func importRawCSVTables(ctx context.Context, db *sql.DB, releaseDir string) error {
	paths := []string{}
	err := filepath.WalkDir(releaseDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".csv") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(paths)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, path := range paths {
		if err := importRawCSVTable(ctx, tx, releaseDir, path); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit raw CSV table preservation: %w", err)
	}
	return nil
}

func importRawCSVTable(ctx context.Context, tx *sql.Tx, releaseDir string, path string) error {
	relativePath, err := filepath.Rel(releaseDir, path)
	if err != nil {
		return fmt.Errorf("compute relative CSV path for %s: %w", path, err)
	}
	relativePath = filepath.ToSlash(relativePath)
	header, maxFields, err := inspectCSVShape(path)
	if err != nil {
		return fmt.Errorf("inspect raw CSV %s: %w", relativePath, err)
	}
	tableName := rawCSVTableName(relativePath)
	columns := rawCSVColumnNames(header, maxFields)
	createParts := []string{quoteIdentifier("_row_number") + " integer not null primary key"}
	for _, column := range columns {
		createParts = append(createParts, quoteIdentifier(column)+" text not null default ''")
	}
	if _, err := tx.ExecContext(ctx, `create table `+quoteIdentifier(tableName)+` (`+strings.Join(createParts, ", ")+`)`); err != nil {
		return fmt.Errorf("create raw CSV table %s for %s: %w", tableName, relativePath, err)
	}
	insertColumns := []string{quoteIdentifier("_row_number")}
	placeholders := []string{"?"}
	for _, column := range columns {
		insertColumns = append(insertColumns, quoteIdentifier(column))
		placeholders = append(placeholders, "?")
	}
	stmt, err := tx.PrepareContext(ctx, `insert into `+quoteIdentifier(tableName)+` (`+strings.Join(insertColumns, ", ")+`) values (`+strings.Join(placeholders, ", ")+`)`)
	if err != nil {
		return fmt.Errorf("prepare raw CSV insert %s: %w", tableName, err)
	}
	defer stmt.Close()
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open raw CSV %s: %w", relativePath, err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	if _, err := reader.Read(); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("read raw CSV header %s: %w", relativePath, err)
	}
	rowNumber := 0
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read raw CSV row %s/%d: %w", relativePath, rowNumber+2, err)
		}
		rowNumber++
		args := make([]any, 0, len(columns)+1)
		args = append(args, rowNumber)
		for i := range columns {
			value := ""
			if i < len(record) {
				value = record[i]
			}
			args = append(args, value)
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			return fmt.Errorf("insert raw CSV row %s/%d: %w", relativePath, rowNumber, err)
		}
	}
	return nil
}

func inspectCSVShape(path string) ([]string, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if errors.Is(err, io.EOF) {
		return []string{}, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	maxFields := len(header)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, 0, err
		}
		if len(record) > maxFields {
			maxFields = len(record)
		}
	}
	return header, maxFields, nil
}

func rawCSVTableName(relativePath string) string {
	withoutExt := strings.TrimSuffix(filepath.ToSlash(relativePath), filepath.Ext(relativePath))
	var builder strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(withoutExt) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	base := strings.Trim(builder.String(), "_")
	if base == "" {
		base = "file"
	}
	if len(base) > 80 {
		base = base[:80]
		base = strings.TrimRight(base, "_")
	}
	sum := sha256.Sum256([]byte(filepath.ToSlash(relativePath)))
	return "raw_csv_" + base + "_" + hex.EncodeToString(sum[:4])
}

func rawCSVColumnNames(header []string, maxFields int) []string {
	columns := make([]string, 0, maxFields)
	seen := map[string]int{}
	for i := 0; i < maxFields; i++ {
		column := ""
		if i < len(header) {
			column = strings.TrimSpace(header[i])
		}
		if column == "" {
			column = fmt.Sprintf("_column_%d", i+1)
		}
		base := column
		if seen[base] > 0 {
			column = fmt.Sprintf("%s_%d", base, seen[base]+1)
		}
		seen[base]++
		columns = append(columns, column)
	}
	return columns
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func finalizeIngestDatabase(ctx context.Context, db *sql.DB) error {
	if err := createPostImportIndexes(ctx, db); err != nil {
		return err
	}
	if err := validateForeignKeys(ctx, db); err != nil {
		return err
	}
	statements := []string{
		`pragma analysis_limit = 1000`,
		`pragma optimize`,
		`pragma foreign_keys = on`,
		`pragma locking_mode = normal`,
		`pragma journal_mode = wal`,
		`pragma synchronous = normal`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("finalize ingest sqlite database %q: %w", statement, err)
		}
	}
	return nil
}

func createPostImportIndexes(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`create index idx_loinc_class on loinc_terms(class)`,
		`create index idx_loinc_status on loinc_terms(status)`,
		`create index idx_loinc_system on loinc_terms(system)`,
		`create index idx_loinc_scale on loinc_terms(scale)`,
		`create index idx_loinc_property on loinc_terms(property)`,
		`create index idx_loinc_order_obs on loinc_terms(order_obs)`,
		`create index idx_loinc_common_test_rank on loinc_terms(common_test_rank, long_common_name, loinc_num) where common_test_rank > 0`,
		`create index idx_loinc_common_order_rank on loinc_terms(common_order_rank, long_common_name, loinc_num) where common_order_rank > 0`,
		`create index idx_loinc_part_links_part on loinc_part_links(part_number, link_set, loinc_num)`,
		`create index idx_loinc_answer_list_links_list on loinc_answer_list_links(answer_list_id, loinc_num)`,
		`create index idx_loinc_map_to_target on loinc_map_to(target_loinc_num)`,
		`create index idx_panel_items_child on panel_items(child_loinc_num, parent_loinc_num)`,
		`create index idx_group_loinc_terms_loinc on group_loinc_terms(loinc_num, group_id)`,
		`create index idx_hierarchy_concepts_loinc on hierarchy_concepts(loinc_num)`,
		`create index idx_hierarchy_concepts_part on hierarchy_concepts(part_number)`,
		`create index idx_hierarchy_occurrences_parent on hierarchy_occurrences(parent_node_id, sequence)`,
		`create index idx_hierarchy_occurrences_code on hierarchy_occurrences(code)`,
		`create index idx_hierarchy_edges_child on hierarchy_edges(child_node_id, parent_node_id)`,
		`create index idx_hierarchy_closure_ancestor on hierarchy_closure(ancestor_node_id, depth, descendant_node_id)`,
		`create index idx_hierarchy_subtree_terms_loinc on hierarchy_subtree_terms(loinc_num, node_id)`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("create post-import index: %w", err)
		}
	}
	return nil
}

func validateForeignKeys(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `pragma foreign_key_check`)
	if err != nil {
		return fmt.Errorf("run foreign key check: %w", err)
	}
	defer rows.Close()
	var problems []string
	for rows.Next() {
		var table string
		var rowID any
		var parent string
		var fkID int
		if err := rows.Scan(&table, &rowID, &parent, &fkID); err != nil {
			return fmt.Errorf("scan foreign key check row: %w", err)
		}
		if len(problems) < 10 {
			problems = append(problems, fmt.Sprintf("%s rowid=%v parent=%s fk=%d", table, rowID, parent, fkID))
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate foreign key check rows: %w", err)
	}
	if len(problems) > 0 {
		return fmt.Errorf("foreign key check failed: %s", strings.Join(problems, "; "))
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
		definition, display_name, common_test_rank, common_order_rank
	) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
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

func importRequiredReleaseFiles(ctx context.Context, db *sql.DB, releaseDir string) error {
	importers := []struct {
		path string
		fn   func(context.Context, *sql.DB, string) error
	}{
		{filepath.Join(releaseDir, "LoincTable", "MapTo.csv"), importMapToCSV},
		{filepath.Join(releaseDir, "LoincTable", "SourceOrganization.csv"), importSourceOrganizationCSV},
		{filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "Part.csv"), importPartCSV},
		{filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "LoincPartLink_Primary.csv"), func(ctx context.Context, db *sql.DB, path string) error {
			return importPartLinkCSV(ctx, db, path, "primary")
		}},
		{filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "LoincPartLink_Supplementary.csv"), func(ctx context.Context, db *sql.DB, path string) error {
			return importPartLinkCSV(ctx, db, path, "supplementary")
		}},
		{filepath.Join(releaseDir, "AccessoryFiles", "AnswerFile", "AnswerList.csv"), importAnswerListCSV},
		{filepath.Join(releaseDir, "AccessoryFiles", "AnswerFile", "LoincAnswerListLink.csv"), func(ctx context.Context, db *sql.DB, path string) error {
			return importAnswerListLinkCSV(ctx, db, path)
		}},
		{filepath.Join(releaseDir, "AccessoryFiles", "PanelsAndForms", "PanelsAndForms.csv"), importPanelsAndFormsCSV},
		{filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "ParentGroup.csv"), importParentGroupCSV},
		{filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "Group.csv"), importGroupCSV},
		{filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "GroupLoincTerms.csv"), func(ctx context.Context, db *sql.DB, path string) error {
			return importGroupLoincTermsCSV(ctx, db, path)
		}},
		{filepath.Join(releaseDir, "AccessoryFiles", "ComponentHierarchyBySystem", "ComponentHierarchyBySystem.csv"), importHierarchyCSV},
	}
	for _, importer := range importers {
		if _, err := os.Stat(importer.path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("required release file %s is missing", importer.path)
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
	stmt, err := tx.PrepareContext(ctx, `insert or ignore into loinc_map_to(loinc_num, target_loinc_num, comment) values (?, ?, ?)`)
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
		if _, err := stmt.ExecContext(ctx, loincNum, mapTo, fields["COMMENT"]); err != nil {
			return fmt.Errorf("insert MapTo %s -> %s: %w", loincNum, mapTo, err)
		}
		return nil
	}, tx)
}

func importPartCSV(ctx context.Context, db *sql.DB, csvPath string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `insert or replace into parts(
		part_number, part_type_name, part_name, part_display_name, status
	) values (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	return readCSVRows(csvPath, func(header []string, record []string) error {
		fields := recordMap(header, record)
		partNumber := strings.TrimSpace(fields["PartNumber"])
		if partNumber == "" {
			return nil
		}
		if _, err := stmt.ExecContext(ctx,
			partNumber,
			fields["PartTypeName"],
			fields["PartName"],
			fields["PartDisplayName"],
			fields["Status"],
		); err != nil {
			return fmt.Errorf("insert part %s: %w", partNumber, err)
		}
		return nil
	}, tx)
}

func importPartLinkCSV(ctx context.Context, db *sql.DB, csvPath string, linkSet string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `insert or ignore into loinc_part_links(
		loinc_num, part_number, link_set, part_name, part_code_system, part_type_name, link_type_name, property
	) values (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	return readCSVRows(csvPath, func(header []string, record []string) error {
		fields := recordMap(header, record)
		loincNum := strings.TrimSpace(fields["LoincNumber"])
		partNumber := strings.TrimSpace(fields["PartNumber"])
		if loincNum == "" || partNumber == "" {
			return nil
		}
		if _, err := stmt.ExecContext(ctx,
			loincNum,
			partNumber,
			linkSet,
			fields["PartName"],
			fields["PartCodeSystem"],
			fields["PartTypeName"],
			fields["LinkTypeName"],
			fields["Property"],
		); err != nil {
			return fmt.Errorf("insert normalized %s part link %s -> %s: %w", linkSet, loincNum, partNumber, err)
		}
		return nil
	}, tx)
}

func importAnswerListCSV(ctx context.Context, db *sql.DB, csvPath string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	listStmt, err := tx.PrepareContext(ctx, `insert into answer_lists(
		answer_list_id, answer_list_name, answer_list_oid, ext_defined_yn,
		ext_defined_answer_list_code_system, ext_defined_answer_list_link
	) values (?, ?, ?, ?, ?, ?)
	on conflict(answer_list_id) do update set
		answer_list_name = excluded.answer_list_name,
		answer_list_oid = excluded.answer_list_oid,
		ext_defined_yn = excluded.ext_defined_yn,
		ext_defined_answer_list_code_system = excluded.ext_defined_answer_list_code_system,
		ext_defined_answer_list_link = excluded.ext_defined_answer_list_link`)
	if err != nil {
		return err
	}
	defer listStmt.Close()
	answerStmt, err := tx.PrepareContext(ctx, `insert or ignore into answer_list_answers(
		answer_list_id, answer_string_id, local_answer_code, local_answer_code_system,
		sequence_number, display_text, ext_code_id, ext_code_display_name, ext_code_system,
		ext_code_system_version, subsequent_text_prompt, description, score
	) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer answerStmt.Close()
	return readCSVRows(csvPath, func(header []string, record []string) error {
		fields := recordMap(header, record)
		answerListID := strings.TrimSpace(fields["AnswerListId"])
		if answerListID == "" {
			return nil
		}
		if _, err := listStmt.ExecContext(ctx,
			answerListID,
			fields["AnswerListName"],
			fields["AnswerListOID"],
			fields["ExtDefinedYN"],
			fields["ExtDefinedAnswerListCodeSystem"],
			fields["ExtDefinedAnswerListLink"],
		); err != nil {
			return fmt.Errorf("insert answer list %s: %w", answerListID, err)
		}
		if _, err := answerStmt.ExecContext(ctx,
			answerListID,
			fields["AnswerStringId"],
			fields["LocalAnswerCode"],
			fields["LocalAnswerCodeSystem"],
			parseRank(fields["SequenceNumber"]),
			fields["DisplayText"],
			fields["ExtCodeId"],
			fields["ExtCodeDisplayName"],
			fields["ExtCodeSystem"],
			fields["ExtCodeSystemVersion"],
			fields["SubsequentTextPrompt"],
			fields["Description"],
			fields["Score"],
		); err != nil {
			return fmt.Errorf("insert answer list answer %s/%s: %w", answerListID, fields["AnswerStringId"], err)
		}
		return nil
	}, tx)
}

func importAnswerListLinkCSV(ctx context.Context, db *sql.DB, csvPath string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `insert or ignore into loinc_answer_list_links(
		loinc_num, answer_list_id, answer_list_name, answer_list_link_type, applicable_context
	) values (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	return readCSVRows(csvPath, func(header []string, record []string) error {
		fields := recordMap(header, record)
		loincNum := strings.TrimSpace(fields["LoincNumber"])
		answerListID := strings.TrimSpace(fields["AnswerListId"])
		if loincNum == "" || answerListID == "" {
			return nil
		}
		if _, err := stmt.ExecContext(ctx,
			loincNum,
			answerListID,
			fields["AnswerListName"],
			fields["AnswerListLinkType"],
			fields["ApplicableContext"],
		); err != nil {
			return fmt.Errorf("insert normalized answer list link %s -> %s: %w", loincNum, answerListID, err)
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
		id, copyright_id, name, copyright, terms_of_use, url
	) values (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	return readCSVRows(csvPath, func(header []string, record []string) error {
		fields := recordMap(header, record)
		_, err = stmt.ExecContext(ctx,
			fields["ID"], fields["COPYRIGHT_ID"], fields["NAME"], fields["COPYRIGHT"], fields["TERMS_OF_USE"], fields["URL"],
		)
		if err != nil {
			return fmt.Errorf("insert source organization %s: %w", fields["ID"], err)
		}
		return nil
	}, tx)
}

func importPanelsAndFormsCSV(ctx context.Context, db *sql.DB, csvPath string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `insert or ignore into panel_items(
		parent_loinc_num, child_loinc_num, parent_id, item_id, sequence, parent_name, child_name,
		display_name_for_form, observation_required_in_panel, entry_type, data_type_in_form,
		answer_list_id_override
	) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	return readCSVRows(csvPath, func(header []string, record []string) error {
		fields := recordMap(header, record)
		parent := strings.TrimSpace(fields["ParentLoinc"])
		child := strings.TrimSpace(fields["Loinc"])
		if parent != "" && child != "" && parent != child {
			if _, err := stmt.ExecContext(ctx,
				parent,
				child,
				fields["ParentId"],
				fields["ID"],
				parseRank(fields["SEQUENCE"]),
				fields["ParentName"],
				fields["LoincName"],
				fields["DisplayNameForForm"],
				fields["ObservationRequiredInPanel"],
				fields["EntryType"],
				fields["DataTypeInForm"],
				nullableString(fields["AnswerListIdOverride"]),
			); err != nil {
				return fmt.Errorf("insert normalized panel item %s -> %s: %w", parent, child, err)
			}
		}
		return nil
	}, tx)
}

func importParentGroupCSV(ctx context.Context, db *sql.DB, csvPath string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `insert or replace into parent_groups(parent_group_id, parent_group, status) values (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	return readCSVRows(csvPath, func(header []string, record []string) error {
		fields := recordMap(header, record)
		parentGroupID := strings.TrimSpace(fields["ParentGroupId"])
		if parentGroupID == "" {
			return nil
		}
		if _, err := stmt.ExecContext(ctx, parentGroupID, fields["ParentGroup"], fields["Status"]); err != nil {
			return fmt.Errorf("insert parent group %s: %w", parentGroupID, err)
		}
		return nil
	}, tx)
}

func importGroupCSV(ctx context.Context, db *sql.DB, csvPath string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `insert or replace into loinc_groups(
		group_id, parent_group_id, group_name, archetype, status, version_first_released
	) values (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	return readCSVRows(csvPath, func(header []string, record []string) error {
		fields := recordMap(header, record)
		groupID := strings.TrimSpace(fields["GroupId"])
		if groupID == "" {
			return nil
		}
		if _, err := stmt.ExecContext(ctx,
			groupID,
			fields["ParentGroupId"],
			fields["Group"],
			fields["Archetype"],
			fields["Status"],
			fields["VersionFirstReleased"],
		); err != nil {
			return fmt.Errorf("insert group %s: %w", groupID, err)
		}
		return nil
	}, tx)
}

func importGroupLoincTermsCSV(ctx context.Context, db *sql.DB, csvPath string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `insert or ignore into group_loinc_terms(
		group_id, loinc_num, category, archetype, long_common_name
	) values (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	return readCSVRows(csvPath, func(header []string, record []string) error {
		fields := recordMap(header, record)
		loincNum := strings.TrimSpace(fields["LoincNumber"])
		groupID := strings.TrimSpace(fields["GroupId"])
		if loincNum == "" || groupID == "" {
			return nil
		}
		if _, err := stmt.ExecContext(ctx,
			groupID,
			loincNum,
			fields["Category"],
			fields["Archetype"],
			fields["LongCommonName"],
		); err != nil {
			return fmt.Errorf("insert normalized group term %s -> %s: %w", groupID, loincNum, err)
		}
		return nil
	}, tx)
}

func importHierarchyCSV(ctx context.Context, db *sql.DB, csvPath string) error {
	file, err := os.Open(csvPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", filepath.Base(csvPath), err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("read %s header: %w", filepath.Base(csvPath), err)
	}
	var hierarchyRows []hierarchyCSVRow
	for row := 2; ; row++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read %s row %d: %w", filepath.Base(csvPath), row, err)
		}
		fields := recordMap(header, record)
		code := strings.TrimSpace(fields["CODE"])
		if code == "" {
			continue
		}
		pathToRoot := strings.TrimSpace(fields["PATH_TO_ROOT"])
		pathKey := code
		if pathToRoot != "" {
			pathKey = pathToRoot + "." + code
		}
		hierarchyRows = append(hierarchyRows, hierarchyCSVRow{
			code:          code,
			label:         fields["CODE_TEXT"],
			pathToRoot:    pathToRoot,
			pathKey:       pathKey,
			parentPathKey: pathToRoot,
			sequence:      parseRank(fields["SEQUENCE"]),
			depth:         hierarchyPathDepth(pathKey),
		})
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := importNormalizedHierarchyRows(ctx, tx, hierarchyRows); err != nil {
		return err
	}
	return tx.Commit()
}

type hierarchyCSVRow struct {
	code          string
	label         string
	pathToRoot    string
	pathKey       string
	parentPathKey string
	ordinal       int
	sequence      int
	depth         int
}

func importNormalizedHierarchyRows(ctx context.Context, tx *sql.Tx, rows []hierarchyCSVRow) error {
	if len(rows) == 0 {
		return nil
	}
	termCodes, err := loadKeySet(ctx, tx, `select loinc_num from loinc_terms`)
	if err != nil {
		return fmt.Errorf("load LOINC term keys for hierarchy: %w", err)
	}
	partCodes, err := loadKeySet(ctx, tx, `select part_number from parts`)
	if err != nil {
		return fmt.Errorf("load part keys for hierarchy: %w", err)
	}
	pathCounts := map[string]int{}
	for i := range rows {
		pathCounts[rows[i].pathKey]++
		rows[i].ordinal = pathCounts[rows[i].pathKey]
	}

	conceptRows := make(map[string]hierarchyCSVRow)
	for _, row := range rows {
		if existing, ok := conceptRows[row.code]; ok && existing.label != "" {
			continue
		}
		conceptRows[row.code] = row
	}
	conceptCodes := make([]string, 0, len(conceptRows))
	for code := range conceptRows {
		conceptCodes = append(conceptCodes, code)
	}
	sort.Strings(conceptCodes)

	conceptStmt, err := tx.PrepareContext(ctx, `insert or replace into hierarchy_concepts(
		code, label, node_kind, loinc_num, part_number
	) values (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer conceptStmt.Close()
	for _, code := range conceptCodes {
		row := conceptRows[code]
		nodeKind := "hierarchy_only"
		var loincNum any
		var partNumber any
		if termCodes[code] {
			nodeKind = "term"
			loincNum = code
		} else if partCodes[code] {
			nodeKind = "part"
			partNumber = code
		}
		if _, err := conceptStmt.ExecContext(ctx, code, row.label, nodeKind, loincNum, partNumber); err != nil {
			return fmt.Errorf("insert hierarchy concept %s: %w", code, err)
		}
	}

	occurrenceStmt, err := tx.PrepareContext(ctx, `insert into hierarchy_occurrences(
		code, parent_node_id, path_key, occurrence_ordinal, path_to_root, sequence, depth
	) values (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer occurrenceStmt.Close()

	pathToFirstNodeID := map[string]int64{}
	nodeIDByRow := make([]int64, len(rows))
	for i, row := range rows {
		var parentNodeID any
		if row.parentPathKey != "" {
			if id, ok := pathToFirstNodeID[row.parentPathKey]; ok {
				parentNodeID = id
			}
		}
		result, err := occurrenceStmt.ExecContext(ctx,
			row.code,
			parentNodeID,
			row.pathKey,
			row.ordinal,
			row.pathToRoot,
			row.sequence,
			row.depth,
		)
		if err != nil {
			return fmt.Errorf("insert hierarchy occurrence %s: %w", row.pathKey, err)
		}
		nodeID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("read hierarchy occurrence id %s: %w", row.pathKey, err)
		}
		nodeIDByRow[i] = nodeID
		if _, ok := pathToFirstNodeID[row.pathKey]; !ok {
			pathToFirstNodeID[row.pathKey] = nodeID
		}
	}

	edgeStmt, err := tx.PrepareContext(ctx, `insert or ignore into hierarchy_edges(parent_node_id, child_node_id, sequence) values (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer edgeStmt.Close()
	for i, row := range rows {
		if row.parentPathKey == "" {
			continue
		}
		parentNodeID, ok := pathToFirstNodeID[row.parentPathKey]
		if !ok {
			continue
		}
		if _, err := edgeStmt.ExecContext(ctx, parentNodeID, nodeIDByRow[i], row.sequence); err != nil {
			return fmt.Errorf("insert hierarchy edge %s -> %s: %w", row.parentPathKey, row.pathKey, err)
		}
	}

	if err := buildHierarchyClosure(ctx, tx); err != nil {
		return err
	}
	if err := buildHierarchySubtreeTerms(ctx, tx); err != nil {
		return err
	}
	return updateHierarchyCounts(ctx, tx)
}

func buildHierarchyClosure(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `insert or ignore into hierarchy_closure(ancestor_node_id, descendant_node_id, depth)
		select node_id, node_id, 0 from hierarchy_occurrences`); err != nil {
		return fmt.Errorf("insert hierarchy self closure: %w", err)
	}
	rows, err := tx.QueryContext(ctx, `select node_id, parent_node_id from hierarchy_occurrences where parent_node_id is not null order by depth, sequence, node_id`)
	if err != nil {
		return fmt.Errorf("load hierarchy parent links: %w", err)
	}
	defer rows.Close()

	type edge struct {
		child  int64
		parent int64
	}
	var edges []edge
	for rows.Next() {
		var item edge
		if err := rows.Scan(&item.child, &item.parent); err != nil {
			return fmt.Errorf("scan hierarchy parent link: %w", err)
		}
		edges = append(edges, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate hierarchy parent links: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, `insert or ignore into hierarchy_closure(ancestor_node_id, descendant_node_id, depth)
		select ancestor_node_id, ?, depth + 1 from hierarchy_closure where descendant_node_id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, item := range edges {
		if _, err := stmt.ExecContext(ctx, item.child, item.parent); err != nil {
			return fmt.Errorf("insert hierarchy closure for node %d: %w", item.child, err)
		}
	}
	return nil
}

func buildHierarchySubtreeTerms(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `insert or ignore into hierarchy_subtree_terms(node_id, loinc_num, descendant_node_id, distance)
		select c.ancestor_node_id, hc.loinc_num, c.descendant_node_id, c.depth
		from hierarchy_closure c
		join hierarchy_occurrences ho on ho.node_id = c.descendant_node_id
		join hierarchy_concepts hc on hc.code = ho.code
		where hc.node_kind = 'term' and hc.loinc_num is not null`); err != nil {
		return fmt.Errorf("insert hierarchy subtree terms: %w", err)
	}
	return nil
}

func updateHierarchyCounts(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `update hierarchy_occurrences
		set direct_term_count = (
			select count(*)
			from hierarchy_edges e
			join hierarchy_occurrences child on child.node_id = e.child_node_id
			join hierarchy_concepts concept on concept.code = child.code
			where e.parent_node_id = hierarchy_occurrences.node_id
				and concept.node_kind = 'term'
		),
		subtree_term_count = (
			select count(distinct loinc_num)
			from hierarchy_subtree_terms
			where node_id = hierarchy_occurrences.node_id
		)`); err != nil {
		return fmt.Errorf("update hierarchy counts: %w", err)
	}
	return nil
}

func loadKeySet(ctx context.Context, tx *sql.Tx, query string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		out[value] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func hierarchyPathDepth(pathKey string) int {
	if strings.TrimSpace(pathKey) == "" {
		return 0
	}
	return strings.Count(pathKey, ".")
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
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
