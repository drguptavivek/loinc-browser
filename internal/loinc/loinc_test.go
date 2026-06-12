package loinc

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
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
	if summary.TermCount != 4 {
		t.Fatalf("expected 4 imported terms, got %d", summary.TermCount)
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
	if len(browse.Results) != 2 {
		t.Fatalf("expected 2 CHEM browse results, got %d", len(browse.Results))
	}

	deprecated, err := store.Search(ctx, SearchParams{Status: "DEPRECATED", Limit: 10})
	if err != nil {
		t.Fatalf("deprecated status search failed: %v", err)
	}
	if len(deprecated.Results) != 1 || deprecated.Results[0].LOINCNum != "1999-9" {
		t.Fatalf("expected explicit deprecated result, got %#v", deprecated.Results)
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
	if facets.Classes["CHEM"] != 3 {
		t.Fatalf("expected CHEM facet count 2, got %#v", facets.Classes)
	}
	if facets.Statuses["ACTIVE"] != 2 {
		t.Fatalf("expected ACTIVE facet count 2, got %#v", facets.Statuses)
	}

	term, err := store.Term(ctx, "1000-1")
	if err != nil {
		t.Fatalf("first term lookup failed: %v", err)
	}
	if len(term.Parts) != 0 {
		t.Fatalf("expected lean term lookup to skip accessories, got %#v", term.Parts)
	}
	termWithAccessories, err := store.TermWithAccessories(ctx, "1000-1")
	if err != nil {
		t.Fatalf("term relationship lookup failed: %v", err)
	}
	if len(termWithAccessories.Parts) == 0 {
		t.Fatalf("expected imported part links on term")
	}
	if len(termWithAccessories.AnswerLists) == 0 {
		t.Fatalf("expected imported answer list links on term")
	}
	if len(termWithAccessories.Panels) == 0 {
		t.Fatalf("expected imported panel links on term")
	}
	if len(termWithAccessories.Groups) == 0 {
		t.Fatalf("expected imported group links on term")
	}
	if len(termWithAccessories.Hierarchy) == 0 {
		t.Fatalf("expected imported hierarchy links on term")
	}
	statsAfterMiss := store.CacheStats()
	if statsAfterMiss.TermHits != 0 || statsAfterMiss.TermMisses != 2 {
		t.Fatalf("expected two term misses, got %#v", statsAfterMiss)
	}
	if _, err := store.Term(ctx, "1000-1"); err != nil {
		t.Fatalf("second term lookup failed: %v", err)
	}
	statsAfterHit := store.CacheStats()
	if statsAfterHit.TermHits != 1 || statsAfterHit.TermMisses != 2 {
		t.Fatalf("expected cached term hit, got %#v", statsAfterHit)
	}

	deprecatedTerm, err := store.TermWithAccessories(ctx, "1999-9")
	if err != nil {
		t.Fatalf("deprecated term lookup failed: %v", err)
	}
	if len(deprecatedTerm.MapTo) != 1 || deprecatedTerm.MapTo[0].MapTo != "1000-1" {
		t.Fatalf("expected MapTo replacement, got %#v", deprecatedTerm.MapTo)
	}

	graph, err := store.TermRelationships(ctx, "1000-1")
	if err != nil {
		t.Fatalf("term relationships failed: %v", err)
	}
	if len(graph.IncomingMapTo) != 1 || graph.IncomingMapTo[0].LOINC != "1999-9" {
		t.Fatalf("expected incoming MapTo from deprecated term, got %#v", graph.IncomingMapTo)
	}
	var foundSharedPart bool
	for _, concept := range graph.SharedConcepts {
		if concept.Kind == "part-primary" && concept.Code == "LP1" && concept.RelatedTotal == 1 && len(concept.RelatedTerms) == 1 && concept.RelatedTerms[0].LOINCNum == "1002-7" {
			foundSharedPart = true
		}
	}
	if !foundSharedPart {
		t.Fatalf("expected shared LP1 concept with related 1002-7, got %#v", graph.SharedConcepts)
	}
	graphStatsAfterMiss := store.CacheStats()
	if graphStatsAfterMiss.RelationshipHits != 0 || graphStatsAfterMiss.RelationshipMisses != 1 || graphStatsAfterMiss.RelationshipEntries != 1 {
		t.Fatalf("expected one relationship cache miss and one entry, got %#v", graphStatsAfterMiss)
	}
	if _, err := store.TermRelationships(ctx, "1000-1"); err != nil {
		t.Fatalf("second term relationships lookup failed: %v", err)
	}
	graphStatsAfterHit := store.CacheStats()
	if graphStatsAfterHit.RelationshipHits != 1 || graphStatsAfterHit.RelationshipMisses != 1 || graphStatsAfterHit.RelationshipEntries != 1 {
		t.Fatalf("expected relationship cache hit, got %#v", graphStatsAfterHit)
	}
	if _, err := store.BrowseAccessories(ctx, AccessoryBrowseParams{Kind: "part-primary", Limit: 2}); err != nil {
		t.Fatalf("browse accessories failed: %v", err)
	}
	accessoryStatsAfterMiss := store.CacheStats()
	if accessoryStatsAfterMiss.AccessoryHits != 0 || accessoryStatsAfterMiss.AccessoryMisses != 1 || accessoryStatsAfterMiss.AccessoryEntries != 1 {
		t.Fatalf("expected accessory cache miss and one entry, got %#v", accessoryStatsAfterMiss)
	}
	if _, err := store.BrowseAccessories(ctx, AccessoryBrowseParams{Kind: "part-primary", Limit: 2}); err != nil {
		t.Fatalf("second browse accessories failed: %v", err)
	}
	accessoryStatsAfterHit := store.CacheStats()
	if accessoryStatsAfterHit.AccessoryHits != 1 || accessoryStatsAfterHit.AccessoryMisses != 1 || accessoryStatsAfterHit.AccessoryEntries != 1 {
		t.Fatalf("expected accessory cache hit, got %#v", accessoryStatsAfterHit)
	}

	sources, err := store.SourceOrganizations(ctx)
	if err != nil {
		t.Fatalf("source organizations failed: %v", err)
	}
	if len(sources) != 1 || sources[0].Name != "Example Source" {
		t.Fatalf("expected source organization, got %#v", sources)
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
	writeCSV("AccessoryFiles/PartFile/LoincPartLink_Primary.csv", []string{"LoincNumber", "LongCommonName", "PartNumber", "PartName", "PartCodeSystem", "PartTypeName", "LinkTypeName", "Property"}, [][]string{
		{"1000-1", "Glucose [Mass/volume] in Plasma", "LP1", "Glucose", "http://loinc.org", "COMPONENT", "Primary", "http://loinc.org/property/COMPONENT"},
		{"1002-7", "Glucose [Presence] in Urine", "LP1", "Glucose", "http://loinc.org", "COMPONENT", "Primary", "http://loinc.org/property/COMPONENT"},
	})
	writeCSV("AccessoryFiles/AnswerFile/LoincAnswerListLink.csv", []string{"LoincNumber", "LongCommonName", "AnswerListId", "AnswerListName", "AnswerListLinkType", "ApplicableContext"}, [][]string{{"1000-1", "Glucose [Mass/volume] in Plasma", "LL1", "Positive/negative", "EXAMPLE", ""}})
	writeCSV("AccessoryFiles/PanelsAndForms/PanelsAndForms.csv", []string{"ParentLoinc", "ParentName", "SEQUENCE", "Loinc", "LoincName", "ObservationRequiredInPanel", "EntryType"}, [][]string{{"2000-1", "Example panel", "1", "1000-1", "Glucose", "R", "Q"}})
	writeCSV("AccessoryFiles/GroupFile/GroupLoincTerms.csv", []string{"Category", "GroupId", "Archetype", "LoincNumber", "LongCommonName"}, [][]string{{"Example", "LG1", "", "1000-1", "Glucose [Mass/volume] in Plasma"}})
	writeCSV("AccessoryFiles/ComponentHierarchyBySystem/ComponentHierarchyBySystem.csv", []string{"PATH_TO_ROOT", "SEQUENCE", "IMMEDIATE_PARENT", "CODE", "CODE_TEXT"}, [][]string{{"LPROOT.LP1", "1", "LP1", "1000-1", "Glucose P"}})
}
