package loinc

import (
	"context"
	"database/sql"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIngestSearchFacetsAndCache(t *testing.T) {
	ctx := context.Background()
	releaseDir := writeTestRelease(t)
	dbPath := filepath.Join(t.TempDir(), "loinc.sqlite")

	summary, err := Ingest(ctx, IngestOptions{
		ReleaseDir: releaseDir,
		DBPath:     dbPath,
	})
	if err != nil {
		t.Fatalf("ingest failed: %v", err)
	}
	if summary.TermCount != 5 {
		t.Fatalf("expected 5 imported terms, got %d", summary.TermCount)
	}

	store, err := OpenStore(dbPath, StoreOptions{CacheEntries: 8})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	exact, err := store.Search(ctx, SearchParams{Query: "1000-1", Limit: 10})
	if err != nil {
		t.Fatalf("exact search failed: %v", err)
	}
	if len(exact.Results) == 0 || exact.Results[0].LOINCNum != "1000-1" {
		t.Fatalf("expected exact LOINC match first, got %#v", exact.Results)
	}

	text, err := store.Search(ctx, SearchParams{Query: "glucose plasma", Status: "ACTIVE", Limit: 10})
	if err != nil {
		t.Fatalf("text search failed: %v", err)
	}
	if len(text.Results) != 1 || text.Results[0].LOINCNum != "1000-1" {
		t.Fatalf("expected glucose plasma result, got %#v", text.Results)
	}

	browse, err := store.Search(ctx, SearchParams{Class: "CHEM", Limit: 10})
	if err != nil {
		t.Fatalf("browse search failed: %v", err)
	}
	if len(browse.Results) != 3 {
		t.Fatalf("expected 3 non-inactive CHEM browse results, got %#v", browse.Results)
	}
	for _, term := range browse.Results {
		if term.Status == "INACTIVE" {
			t.Fatalf("default browse should exclude INACTIVE terms, got %#v", browse.Results)
		}
	}

	deprecated, err := store.Search(ctx, SearchParams{Status: "DEPRECATED", Limit: 10})
	if err != nil {
		t.Fatalf("deprecated status search failed: %v", err)
	}
	if len(deprecated.Results) != 1 || deprecated.Results[0].LOINCNum != "1999-9" {
		t.Fatalf("expected explicit deprecated result, got %#v", deprecated.Results)
	}

	inactive, err := store.Search(ctx, SearchParams{Status: "INACTIVE", Limit: 10})
	if err != nil {
		t.Fatalf("inactive status search failed: %v", err)
	}
	if len(inactive.Results) != 1 || inactive.Results[0].LOINCNum != "1003-5" {
		t.Fatalf("expected explicit inactive result, got %#v", inactive.Results)
	}

	allStatuses, err := store.Search(ctx, SearchParams{Status: "*", Limit: 10})
	if err != nil {
		t.Fatalf("all-status search failed: %v", err)
	}
	if len(allStatuses.Results) != 5 {
		t.Fatalf("expected status=* to include all imported terms, got %#v", allStatuses.Results)
	}

	multi, err := store.Search(ctx, SearchParams{Statuses: []string{"ACTIVE", "DISCOURAGED"}, Scales: []string{"Qn", "Ord"}, Limit: 10})
	if err != nil {
		t.Fatalf("multi-filter search failed: %v", err)
	}
	if len(multi.Results) != 3 {
		t.Fatalf("expected 3 multi-filter results, got %#v", multi.Results)
	}

	facets, err := store.Facets(ctx)
	if err != nil {
		t.Fatalf("facets failed: %v", err)
	}
	if facets.Classes["CHEM"] != 4 {
		t.Fatalf("expected CHEM facet count 4, got %#v", facets.Classes)
	}
	if facets.Statuses["ACTIVE"] != 2 {
		t.Fatalf("expected ACTIVE facet count 2, got %#v", facets.Statuses)
	}
	if facets.Statuses["INACTIVE"] != 1 {
		t.Fatalf("expected INACTIVE facet count 1, got %#v", facets.Statuses)
	}

	term, err := store.Term(ctx, "1000-1")
	if err != nil {
		t.Fatalf("first term lookup failed: %v", err)
	}
	if len(term.Parts) != 0 {
		t.Fatalf("expected lean term lookup to skip accessories, got %#v", term.Parts)
	}
	statsAfterMiss := store.CacheStats()
	if statsAfterMiss.TermHits != 0 || statsAfterMiss.TermMisses != 1 {
		t.Fatalf("expected one term miss, got %#v", statsAfterMiss)
	}
	if _, err := store.Term(ctx, "1000-1"); err != nil {
		t.Fatalf("second term lookup failed: %v", err)
	}
	statsAfterHit := store.CacheStats()
	if statsAfterHit.TermHits != 1 || statsAfterHit.TermMisses != 1 {
		t.Fatalf("expected cached term hit, got %#v", statsAfterHit)
	}

	sources, err := store.SourceOrganizations(ctx)
	if err != nil {
		t.Fatalf("source organizations failed: %v", err)
	}
	if len(sources) != 1 || sources[0].Name != "Example Source" {
		t.Fatalf("expected source organization, got %#v", sources)
	}
}

func TestNormalizedRelationshipQueries(t *testing.T) {
	ctx := context.Background()
	releaseDir := writeTestRelease(t)
	dbPath := filepath.Join(t.TempDir(), "loinc-normalized.sqlite")

	if _, err := Ingest(ctx, IngestOptions{
		ReleaseDir: releaseDir,
		DBPath:     dbPath,
	}); err != nil {
		t.Fatalf("ingest failed: %v", err)
	}

	store, err := OpenStore(dbPath, StoreOptions{CacheEntries: 8})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	term, err := store.TermWithAccessories(ctx, "1000-1")
	if err != nil {
		t.Fatalf("term relationship lookup failed: %v", err)
	}
	if !hasAccessory(term.Parts, "part-primary", "LP1") || !hasAccessory(term.Parts, "part-supplementary", "LP2") {
		t.Fatalf("expected normalized part links, got %#v", term.Parts)
	}
	if !hasAccessory(term.AnswerLists, "answer-list", "LL1") {
		t.Fatalf("expected normalized answer list link, got %#v", term.AnswerLists)
	}
	if !hasAccessory(term.Panels, "panel-membership", "1001-9") {
		t.Fatalf("expected normalized panel membership, got %#v", term.Panels)
	}
	if !hasAccessory(term.Groups, "group", "LG1") {
		t.Fatalf("expected normalized group membership, got %#v", term.Groups)
	}
	if !hasAccessory(term.Hierarchy, "hierarchy", "LPROOT.LP1.1000-1") {
		t.Fatalf("expected normalized hierarchy occurrence, got %#v", term.Hierarchy)
	}

	termWithNullPanelOverride, err := store.TermWithAccessories(ctx, "1002-7")
	if err != nil {
		t.Fatalf("term with nullable panel override lookup failed: %v", err)
	}
	if !hasAccessoryField(termWithNullPanelOverride.Panels, "panel-membership", "1001-9", "answerListIdOverride", "") {
		t.Fatalf("expected blank answer-list override on nullable panel membership, got %#v", termWithNullPanelOverride.Panels)
	}

	deprecatedTerm, err := store.TermWithAccessories(ctx, "1999-9")
	if err != nil {
		t.Fatalf("deprecated term lookup failed: %v", err)
	}
	if len(deprecatedTerm.MapTo) != 1 || deprecatedTerm.MapTo[0].MapTo != "1000-1" {
		t.Fatalf("expected normalized MapTo replacement, got %#v", deprecatedTerm.MapTo)
	}

	graph, err := store.TermRelationships(ctx, "1000-1")
	if err != nil {
		t.Fatalf("term relationships failed: %v", err)
	}
	if len(graph.IncomingMapTo) != 1 || graph.IncomingMapTo[0].LOINC != "1999-9" {
		t.Fatalf("expected incoming MapTo from deprecated term, got %#v", graph.IncomingMapTo)
	}
	if !hasRelatedConcept(graph.SharedConcepts, "part-primary", "LP1", "1002-7") {
		t.Fatalf("expected shared LP1 primary part concept with related 1002-7, got %#v", graph.SharedConcepts)
	}

	groups, err := store.TermRelationshipGroups(ctx, "1000-1")
	if err != nil {
		t.Fatalf("term relationship groups failed: %v", err)
	}
	if !hasRelatedConcept(groups.SharedConcepts, "part-primary", "LP1", "1002-7") {
		t.Fatalf("expected v1 relationship groups to include shared LP1 graph concept with related 1002-7, got %#v", groups.SharedConcepts)
	}

	accessories, err := store.BrowseAccessories(ctx, AccessoryBrowseParams{Kind: "part-primary", Query: "glucose", Limit: 10})
	if err != nil {
		t.Fatalf("browse normalized accessories failed: %v", err)
	}
	if accessories.Total != 2 || !hasAccessoryRecord(accessories.Results, "part-primary", "1000-1", "LP1") || !hasAccessoryRecord(accessories.Results, "part-primary", "1002-7", "LP1") {
		t.Fatalf("expected two normalized LP1 primary part rows, got %#v", accessories)
	}

	roots, err := store.HierarchyChildren(ctx, "", "", true)
	if err != nil {
		t.Fatalf("load hierarchy roots failed: %v", err)
	}
	if len(roots.Results) != 1 || roots.Results[0].Code != "LPROOT" || roots.Results[0].NodeID == "" {
		t.Fatalf("expected normalized hierarchy root with node id, got %#v", roots.Results)
	}
	children, err := store.HierarchyChildren(ctx, roots.Results[0].NodeID, "", true)
	if err != nil {
		t.Fatalf("load hierarchy children failed: %v", err)
	}
	if !hasHierarchyNode(children.Results, "LP1") || !hasHierarchyNode(children.Results, "LPALT") {
		t.Fatalf("expected LP1 and LPALT children, got %#v", children.Results)
	}

	hierarchySearch, err := store.Search(ctx, SearchParams{HierarchyCode: roots.Results[0].NodeID, Status: "*", Limit: 10})
	if err != nil {
		t.Fatalf("hierarchy-scoped search failed: %v", err)
	}
	if len(hierarchySearch.Results) != 2 || !hasSearchResult(hierarchySearch.Results, "1000-1") || !hasSearchResult(hierarchySearch.Results, "1002-7") {
		t.Fatalf("expected hierarchy search under root to return descendant terms, got %#v", hierarchySearch.Results)
	}
}

func TestIngestCreatesNormalizedRelationshipModel(t *testing.T) {
	ctx := context.Background()
	releaseDir := writeTestRelease(t)
	dbPath := filepath.Join(t.TempDir(), "loinc-normalized.sqlite")

	if _, err := Ingest(ctx, IngestOptions{
		ReleaseDir: releaseDir,
		DBPath:     dbPath,
	}); err != nil {
		t.Fatalf("ingest failed: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	assertCount := func(query string, args []any, want int) {
		t.Helper()
		var got int
		if err := db.QueryRow(query, args...).Scan(&got); err != nil {
			t.Fatalf("query %q failed: %v", query, err)
		}
		if got != want {
			t.Fatalf("query %q got %d, want %d", query, got, want)
		}
	}

	assertCount(`select count(*) from parts`, nil, 3)
	assertCount(`select count(*) from loinc_part_links where link_set = 'primary'`, nil, 2)
	assertCount(`select count(*) from loinc_part_links where link_set = 'supplementary'`, nil, 1)
	assertCount(`select count(*) from answer_lists`, nil, 1)
	assertCount(`select count(*) from answer_list_answers`, nil, 2)
	assertCount(`select count(*) from loinc_answer_list_links where loinc_num = '1000-1' and answer_list_id = 'LL1'`, nil, 1)
	assertCount(`select count(*) from panel_items where parent_loinc_num = '1001-9' and child_loinc_num = '1000-1'`, nil, 1)
	assertCount(`select count(*) from parent_groups`, nil, 1)
	assertCount(`select count(*) from loinc_groups`, nil, 1)
	assertCount(`select count(*) from group_loinc_terms where group_id = 'LG1' and loinc_num = '1000-1'`, nil, 1)
	assertCount(`select count(*) from loinc_map_to where loinc_num = '1999-9' and target_loinc_num = '1000-1'`, nil, 1)
	assertCount(`select count(*) from hierarchy_concepts where code = 'LP1'`, nil, 1)
	assertCount(`select count(*) from hierarchy_occurrences where code = 'LP1'`, nil, 2)
	assertCount(`select count(distinct loinc_num) from hierarchy_subtree_terms st join hierarchy_occurrences n on n.node_id = st.node_id where n.path_key = 'LPROOT'`, nil, 2)
	assertCount(`select count(*) from hierarchy_subtree_terms st join hierarchy_occurrences n on n.node_id = st.node_id where n.path_key = 'LPROOT.LP1' and st.loinc_num = '1000-1'`, nil, 1)
	assertCount(`select count(*) from hierarchy_subtree_terms st join hierarchy_occurrences n on n.node_id = st.node_id where n.path_key = 'LPROOT.LPALT.LP1' and st.loinc_num = '1002-7'`, nil, 1)

	assertCount(`select count(*) from sqlite_master where type = 'table' and name in ('map_to', 'term_accessories', 'hierarchy_nodes', 'hierarchy_nav_edges', 'hierarchy_term_members')`, nil, 0)
	assertCount(`select count(*) from sqlite_master where type = 'index' and name in ('idx_hierarchy_edges_child', 'idx_loinc_part_links_part', 'idx_loinc_answer_list_links_list', 'idx_loinc_common_test_rank', 'idx_loinc_common_order_rank')`, nil, 5)
	assertCount(`select count(*) from sqlite_master where type = 'index' and name in ('idx_hierarchy_subtree_terms_node_distance', 'idx_loinc_part_links_term', 'idx_loinc_answer_list_links_term', 'idx_panel_items_parent_seq', 'idx_hierarchy_occurrences_path')`, nil, 0)

	assertNoColumn := func(table string, column string) {
		t.Helper()
		var got int
		if err := db.QueryRow(`select count(*) from pragma_table_info(?) where name = ?`, table, column).Scan(&got); err != nil {
			t.Fatalf("inspect %s.%s: %v", table, column, err)
		}
		if got != 0 {
			t.Fatalf("expected %s to omit %s", table, column)
		}
	}
	for _, table := range []string{
		"loinc_terms",
		"parts",
		"loinc_part_links",
		"answer_list_answers",
		"loinc_answer_list_links",
		"panel_items",
		"parent_groups",
		"loinc_groups",
		"group_loinc_terms",
		"hierarchy_concepts",
		"hierarchy_occurrences",
		"source_organizations",
	} {
		assertNoColumn(table, "raw_json")
	}

	assertWithoutRowID := func(table string) {
		t.Helper()
		var sqlText string
		if err := db.QueryRow(`select sql from sqlite_master where type = 'table' and name = ?`, table).Scan(&sqlText); err != nil {
			t.Fatalf("load table sql for %s: %v", table, err)
		}
		if !strings.Contains(strings.ToLower(sqlText), "without rowid") {
			t.Fatalf("expected %s to use WITHOUT ROWID, got %s", table, sqlText)
		}
	}
	for _, table := range []string{
		"loinc_part_links",
		"answer_list_answers",
		"loinc_answer_list_links",
		"panel_items",
		"group_loinc_terms",
		"hierarchy_edges",
		"hierarchy_closure",
		"hierarchy_subtree_terms",
	} {
		assertWithoutRowID(table)
	}

	var journalMode string
	if err := db.QueryRow(`pragma journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("read journal mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("expected WAL journal mode, got %q", journalMode)
	}

	rows, err := db.Query(`pragma foreign_key_check`)
	if err != nil {
		t.Fatalf("foreign_key_check failed: %v", err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatalf("expected normalized foreign keys to be valid")
	}
}

func hasAccessory(items []TermAccessory, kind string, code string) bool {
	for _, item := range items {
		if item.Kind == kind && item.Code == code {
			return true
		}
	}
	return false
}

func hasAccessoryField(items []TermAccessory, kind string, code string, field string, value string) bool {
	for _, item := range items {
		if item.Kind == kind && item.Code == code && item.Fields[field] == value {
			return true
		}
	}
	return false
}

func hasAccessoryRecord(items []AccessoryRecord, kind string, loincNum string, code string) bool {
	for _, item := range items {
		if item.Kind == kind && item.LOINCNum == loincNum && item.Code == code {
			return true
		}
	}
	return false
}

func hasRelatedConcept(items []RelationshipConcept, kind string, code string, relatedLOINC string) bool {
	for _, item := range items {
		if item.Kind != kind || item.Code != code {
			continue
		}
		for _, term := range item.RelatedTerms {
			if term.LOINCNum == relatedLOINC {
				return true
			}
		}
	}
	return false
}

func hasHierarchyNode(items []HierarchyNode, code string) bool {
	for _, item := range items {
		if item.Code == code && item.NodeID != "" {
			return true
		}
	}
	return false
}

func hasSearchResult(items []SearchResult, loincNum string) bool {
	for _, item := range items {
		if item.LOINCNum == loincNum {
			return true
		}
	}
	return false
}

func TestIngestRequiresRelationshipReleaseFiles(t *testing.T) {
	ctx := context.Background()
	releaseDir := writeTestRelease(t)
	if err := os.Remove(filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "Part.csv")); err != nil {
		t.Fatalf("remove required Part.csv: %v", err)
	}

	_, err := Ingest(ctx, IngestOptions{
		ReleaseDir: releaseDir,
		DBPath:     filepath.Join(t.TempDir(), "loinc-normalized.sqlite"),
	})
	if err == nil {
		t.Fatalf("expected missing required relationship file to fail ingest")
	}
	if !strings.Contains(err.Error(), "required release file") || !strings.Contains(err.Error(), "Part.csv") {
		t.Fatalf("expected required Part.csv error, got %v", err)
	}
}

func writeTestRelease(t *testing.T) string {
	t.Helper()

	releaseDir := t.TempDir()
	tableDir := filepath.Join(releaseDir, "LoincTable")
	if err := os.MkdirAll(tableDir, 0o755); err != nil {
		t.Fatalf("mkdir release: %v", err)
	}
	writeOptionalTestFiles(t, releaseDir)

	file, err := os.Create(filepath.Join(tableDir, "Loinc.csv"))
	if err != nil {
		t.Fatalf("create csv: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	header := []string{
		"LOINC_NUM", "COMPONENT", "PROPERTY", "TIME_ASPCT", "SYSTEM",
		"SCALE_TYP", "METHOD_TYP", "CLASS", "VersionLastChanged", "CHNG_TYPE",
		"DefinitionDescription", "STATUS", "CONSUMER_NAME", "CLASSTYPE",
		"FORMULA", "EXMPL_ANSWERS", "SURVEY_QUEST_TEXT", "SURVEY_QUEST_SRC",
		"UNITSREQUIRED", "RELATEDNAMES2", "SHORTNAME", "ORDER_OBS",
		"HL7_FIELD_SUBFIELD_ID", "EXTERNAL_COPYRIGHT_NOTICE", "EXAMPLE_UNITS",
		"LONG_COMMON_NAME", "EXAMPLE_UCUM_UNITS", "STATUS_REASON", "STATUS_TEXT",
		"CHANGE_REASON_PUBLIC", "COMMON_TEST_RANK", "COMMON_ORDER_RANK",
		"HL7_ATTACHMENT_STRUCTURE", "EXTERNAL_COPYRIGHT_LINK", "PanelType",
		"AskAtOrderEntry", "AssociatedObservations", "VersionFirstReleased",
		"ValidHL7AttachmentRequest", "DisplayName",
	}
	rows := [][]string{
		{
			"1000-1", "Glucose", "MCnc", "Pt", "Plasma", "Qn", "", "CHEM",
			"2.80", "ADD", "Glucose mass concentration in plasma", "ACTIVE",
			"Blood sugar", "1", "", "", "", "", "N", "blood sugar; glucose plasma",
			"Glucose P", "Both", "", "", "mg/dL", "Glucose [Mass/volume] in Plasma",
			"mg/dL", "", "", "", "100", "100", "", "", "", "", "", "2.80", "", "Glucose Plasma",
		},
		{
			"1001-9", "Hemoglobin", "MCnc", "Pt", "Blood", "Qn", "", "HEM/BC",
			"2.80", "ADD", "Hemoglobin mass concentration", "ACTIVE",
			"Hgb", "1", "", "", "", "", "N", "hgb; hb",
			"Hgb Bld", "Observation", "", "", "g/dL", "Hemoglobin [Mass/volume] in Blood",
			"g/dL", "", "", "", "50", "0", "", "", "", "", "", "2.80", "", "Hemoglobin Blood",
		},
		{
			"1002-7", "Glucose", "SCnc", "Pt", "Urine", "Ord", "", "CHEM",
			"2.80", "ADD", "Glucose presence in urine", "DISCOURAGED",
			"Urine sugar", "1", "", "", "", "", "N", "urine sugar",
			"Glucose Ur", "Observation", "", "", "", "Glucose [Presence] in Urine",
			"", "", "", "", "0", "0", "", "", "", "", "", "2.80", "", "Glucose Urine",
		},
		{
			"1999-9", "Deprecated chemistry", "MCnc", "Pt", "Serum", "Qn", "", "CHEM",
			"2.80", "DEL", "Deprecated chemistry term", "DEPRECATED",
			"", "1", "", "", "", "", "N", "deprecated chemistry",
			"Deprecated Chem", "Observation", "", "", "mg/dL", "Deprecated chemistry term",
			"mg/dL", "", "", "", "0", "0", "", "", "", "", "", "2.80", "", "Deprecated chemistry",
		},
		{
			"1003-5", "Inactive chemistry", "MCnc", "Pt", "Serum", "Qn", "", "CHEM",
			"2.80", "DEL", "Inactive chemistry term", "INACTIVE",
			"", "1", "", "", "", "", "N", "inactive chemistry",
			"Inactive Chem", "Observation", "", "", "mg/dL", "Inactive chemistry term",
			"mg/dL", "", "", "", "0", "0", "", "", "", "", "", "2.80", "", "Inactive chemistry",
		},
	}

	if err := writer.Write(header); err != nil {
		t.Fatalf("write header: %v", err)
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			t.Fatalf("write row: %v", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatalf("flush csv: %v", err)
	}
	return releaseDir
}

func writeOptionalTestFiles(t *testing.T, releaseDir string) {
	t.Helper()
	writeCSV := func(rel string, header []string, rows [][]string) {
		t.Helper()
		path := filepath.Join(releaseDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("create %s: %v", rel, err)
		}
		defer file.Close()
		writer := csv.NewWriter(file)
		if err := writer.Write(header); err != nil {
			t.Fatalf("write %s header: %v", rel, err)
		}
		for _, row := range rows {
			if err := writer.Write(row); err != nil {
				t.Fatalf("write %s row: %v", rel, err)
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			t.Fatalf("flush %s: %v", rel, err)
		}
	}
	writeCSV("LoincTable/MapTo.csv", []string{"LOINC", "MAP_TO", "COMMENT"}, [][]string{{"1999-9", "1000-1", "Use active glucose term"}})
	writeCSV("LoincTable/SourceOrganization.csv", []string{"ID", "COPYRIGHT_ID", "NAME", "COPYRIGHT", "TERMS_OF_USE", "URL"}, [][]string{{"1", "EX", "Example Source", "Copyright Example", "Use terms", "https://example.org"}})
	writeCSV("AccessoryFiles/PartFile/Part.csv", []string{"PartNumber", "PartTypeName", "PartName", "PartDisplayName", "Status"}, [][]string{
		{"LP1", "COMPONENT", "Glucose", "Glucose", "ACTIVE"},
		{"LP2", "SYSTEM", "Plasma", "Plasma", "ACTIVE"},
		{"LPALT", "HIERARCHY", "Alternate branch", "Alternate branch", "ACTIVE"},
	})
	writeCSV("AccessoryFiles/PartFile/LoincPartLink_Primary.csv", []string{"LoincNumber", "LongCommonName", "PartNumber", "PartName", "PartCodeSystem", "PartTypeName", "LinkTypeName", "Property"}, [][]string{
		{"1000-1", "Glucose [Mass/volume] in Plasma", "LP1", "Glucose", "http://loinc.org", "COMPONENT", "Primary", "http://loinc.org/property/COMPONENT"},
		{"1002-7", "Glucose [Presence] in Urine", "LP1", "Glucose", "http://loinc.org", "COMPONENT", "Primary", "http://loinc.org/property/COMPONENT"},
	})
	writeCSV("AccessoryFiles/PartFile/LoincPartLink_Supplementary.csv", []string{"LoincNumber", "LongCommonName", "PartNumber", "PartName", "PartCodeSystem", "PartTypeName", "LinkTypeName", "Property"}, [][]string{
		{"1000-1", "Glucose [Mass/volume] in Plasma", "LP2", "Plasma", "http://loinc.org", "SYSTEM", "Supplementary", "http://loinc.org/property/SYSTEM"},
	})
	writeCSV("AccessoryFiles/AnswerFile/AnswerList.csv", []string{"AnswerListId", "AnswerListName", "AnswerListOID", "ExtDefinedYN", "ExtDefinedAnswerListCodeSystem", "ExtDefinedAnswerListLink", "AnswerStringId", "LocalAnswerCode", "LocalAnswerCodeSystem", "SequenceNumber", "DisplayText", "ExtCodeId", "ExtCodeDisplayName", "ExtCodeSystem", "ExtCodeSystemVersion", "ExtCodeSystemCopyrightNotice", "SubsequentTextPrompt", "Description", "Score"}, [][]string{
		{"LL1", "Positive/negative", "", "N", "", "", "LA1", "POS", "LOCAL", "1", "Positive", "", "", "", "", "", "", "", "1"},
		{"LL1", "Positive/negative", "", "N", "", "", "LA2", "NEG", "LOCAL", "2", "Negative", "", "", "", "", "", "", "", "0"},
	})
	writeCSV("AccessoryFiles/AnswerFile/LoincAnswerListLink.csv", []string{"LoincNumber", "LongCommonName", "AnswerListId", "AnswerListName", "AnswerListLinkType", "ApplicableContext"}, [][]string{{"1000-1", "Glucose [Mass/volume] in Plasma", "LL1", "Positive/negative", "EXAMPLE", ""}})
	writeCSV("AccessoryFiles/PanelsAndForms/PanelsAndForms.csv", []string{"ParentId", "ParentLoinc", "ParentName", "ID", "SEQUENCE", "Loinc", "LoincName", "DisplayNameForForm", "ObservationRequiredInPanel", "ObservationIdInForm", "SkipLogicHelpText", "DefaultValue", "EntryType", "DataTypeInForm", "DataTypeSource", "AnswerSequenceOverride", "ConditionForInclusion", "AllowableAlternative", "ObservationCategory", "Context", "ConsistencyChecks", "RelevanceEquation", "CodingInstructions", "QuestionCardinality", "AnswerCardinality", "AnswerListIdOverride", "AnswerListTypeOverride", "EXTERNAL_COPYRIGHT_NOTICE", "AdditionalCopyright"}, [][]string{
		{"P1", "1001-9", "Example panel", "I1", "1", "1000-1", "Glucose", "Glucose", "R", "", "", "", "Q", "", "", "", "", "", "", "", "", "", "", "", "", "LL1", "", "", ""},
		{"P1", "1001-9", "Example panel", "I2", "2", "1002-7", "Glucose urine", "Glucose urine", "O", "", "", "", "Q", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
	})
	writeCSV("AccessoryFiles/GroupFile/ParentGroup.csv", []string{"ParentGroupId", "ParentGroup", "Status"}, [][]string{{"PG1", "Example parent group", "ACTIVE"}})
	writeCSV("AccessoryFiles/GroupFile/Group.csv", []string{"ParentGroupId", "GroupId", "Group", "Archetype", "Status", "VersionFirstReleased"}, [][]string{{"PG1", "LG1", "Example group", "Example archetype", "ACTIVE", "2.80"}})
	writeCSV("AccessoryFiles/GroupFile/GroupLoincTerms.csv", []string{"Category", "GroupId", "Archetype", "LoincNumber", "LongCommonName"}, [][]string{{"Example", "LG1", "", "1000-1", "Glucose [Mass/volume] in Plasma"}})
	writeCSV("AccessoryFiles/ComponentHierarchyBySystem/ComponentHierarchyBySystem.csv", []string{"PATH_TO_ROOT", "SEQUENCE", "IMMEDIATE_PARENT", "CODE", "CODE_TEXT"}, [][]string{
		{"", "1", "", "LPROOT", "{component}"},
		{"LPROOT", "1", "LPROOT", "LP1", "Glucose"},
		{"LPROOT.LP1", "1", "LP1", "1000-1", "Glucose P"},
		{"LPROOT", "2", "LPROOT", "LPALT", "Alternate branch"},
		{"LPROOT.LPALT", "1", "LPALT", "LP1", "Glucose"},
		{"LPROOT.LPALT.LP1", "1", "LP1", "1002-7", "Glucose Ur"},
	})
}
