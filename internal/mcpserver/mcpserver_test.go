package mcpserver

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"loinc-browser/internal/loinc"
)

func TestConceptDocsReturnTopicIndexAndSections(t *testing.T) {
	docsDir := t.TempDir()
	writeTestFile(t, docsDir, "LOINC_CONCEPTS.md", `# LOINC Concepts

## Status

Active terms are normally preferred. Deprecated terms require caution.

## Answer Lists

Answer lists define allowed answer choices.
`)

	docs := NewDocs(docsDir)
	index, err := docs.ExplainConcept(context.Background(), ConceptRequest{})
	if err != nil {
		t.Fatalf("explain concept index: %v", err)
	}
	if !strings.Contains(index.Text, "status") || !strings.Contains(index.Text, "answer_lists") {
		t.Fatalf("expected compact topic index, got %q", index.Text)
	}
	if strings.Contains(index.Text, "Active terms are normally preferred") {
		t.Fatalf("topic index should not include full section body, got %q", index.Text)
	}

	section, err := docs.ExplainConcept(context.Background(), ConceptRequest{Topic: "answer_lists"})
	if err != nil {
		t.Fatalf("explain answer lists: %v", err)
	}
	if !strings.Contains(section.Text, "Answer lists define allowed answer choices.") {
		t.Fatalf("expected answer list section, got %q", section.Text)
	}
	if strings.Contains(section.Text, "Deprecated terms require caution") {
		t.Fatalf("expected only matching section, got %q", section.Text)
	}
}

func TestConceptDocsSearchStructuredFiles(t *testing.T) {
	docsDir := t.TempDir()
	writeTestFile(t, docsDir, "LOINC_CONCEPTS.md", `# LOINC Concepts

## Purpose

Core concept index.
`)
	writeTestFile(t, docsDir, "LOINC_SPECIAL_CASES.md", `# LOINC Special Cases

## Microbiology

Culture terms identify the observation; organisms are usually result values.
`)
	writeTestFile(t, docsDir, "LOINC_DATABASE_STRUCTURE.md", `# LOINC Database Structure

## Map To Table

Replacement mappings link deprecated terms to candidate replacements.
`)
	writeTestFile(t, docsDir, "LOINC_PART_LINKAGES.md", `# LOINC Part Linkages

## Primary Linkages

Primary links represent the exact five or six FSN axis fields.
`)

	docs := NewDocs(docsDir)
	index, err := docs.ExplainConcept(context.Background(), ConceptRequest{})
	if err != nil {
		t.Fatalf("explain concept index: %v", err)
	}
	if !strings.Contains(index.Text, "purpose") || !strings.Contains(index.Text, "microbiology") || !strings.Contains(index.Text, "map_to_table") || !strings.Contains(index.Text, "primary_linkages") {
		t.Fatalf("expected index to include topics from structured docs, got %q", index.Text)
	}

	section, err := docs.ExplainConcept(context.Background(), ConceptRequest{Topic: "microbiology"})
	if err != nil {
		t.Fatalf("explain microbiology: %v", err)
	}
	if !strings.Contains(section.Text, "organisms are usually result values") {
		t.Fatalf("expected microbiology section from structured doc, got %q", section.Text)
	}

	replacementSection, err := docs.ExplainConcept(context.Background(), ConceptRequest{Topic: "map_to_table"})
	if err != nil {
		t.Fatalf("explain map to table: %v", err)
	}
	if !strings.Contains(replacementSection.Text, "candidate replacements") {
		t.Fatalf("expected map to section from structured doc, got %q", replacementSection.Text)
	}

	linkageSection, err := docs.ExplainConcept(context.Background(), ConceptRequest{Topic: "primary_linkages"})
	if err != nil {
		t.Fatalf("explain primary linkages: %v", err)
	}
	if !strings.Contains(linkageSection.Text, "exact five or six FSN axis fields") {
		t.Fatalf("expected part linkage section from structured doc, got %q", linkageSection.Text)
	}
}

func TestReadAgentResourceReadsMarkdownAtRequestTime(t *testing.T) {
	rootDir := t.TempDir()
	docsDir := filepath.Join(rootDir, "agent")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("mkdir docs dir: %v", err)
	}
	conceptsPath := filepath.Join(docsDir, "LOINC_CONCEPTS.md")
	if err := os.WriteFile(conceptsPath, []byte("# Concepts\n\nVersion one.\n"), 0o644); err != nil {
		t.Fatalf("write concepts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "API.md"), []byte("# API Guide\n\nRoute map.\n"), 0o644); err != nil {
		t.Fatalf("write api guide: %v", err)
	}

	docs := NewDocs(docsDir)
	first, err := docs.ReadResource(context.Background(), "loinc://concepts")
	if err != nil {
		t.Fatalf("read resource first: %v", err)
	}
	if !strings.Contains(first.Text, "Version one.") {
		t.Fatalf("expected first version, got %q", first.Text)
	}

	if err := os.WriteFile(conceptsPath, []byte("# Concepts\n\nVersion two.\n"), 0o644); err != nil {
		t.Fatalf("rewrite concepts: %v", err)
	}
	second, err := docs.ReadResource(context.Background(), "loinc://concepts")
	if err != nil {
		t.Fatalf("read resource second: %v", err)
	}
	if !strings.Contains(second.Text, "Version two.") || strings.Contains(second.Text, "Version one.") {
		t.Fatalf("expected request-time read, got %q", second.Text)
	}

	apiGuide, err := docs.ReadResource(context.Background(), "loinc://api-guide")
	if err != nil {
		t.Fatalf("read api guide: %v", err)
	}
	if !strings.Contains(apiGuide.Text, "Route map.") {
		t.Fatalf("expected api guide content, got %q", apiGuide.Text)
	}
}

func TestReadAgentResourceReportsMissingDocsFile(t *testing.T) {
	docs := NewDocs(t.TempDir())
	_, err := docs.ReadResource(context.Background(), "loinc://concepts")
	if err == nil {
		t.Fatal("expected missing docs error")
	}
	if !strings.Contains(err.Error(), "LOINC_CONCEPTS.md") {
		t.Fatalf("expected missing file name in error, got %v", err)
	}
}

func TestSearchTermsReturnsCompactCappedResults(t *testing.T) {
	service := newTestService(t)

	result, err := service.SearchTerms(context.Background(), SearchTermsRequest{
		Query:     "glucose",
		UsageType: "observation",
		Limit:     500,
	})
	if err != nil {
		t.Fatalf("search terms: %v", err)
	}
	if result.Limit != 50 {
		t.Fatalf("expected capped limit 50, got %d", result.Limit)
	}
	if len(result.Results) == 0 {
		t.Fatal("expected glucose search result")
	}
	first := result.Results[0]
	if first.LOINCNum == "" || first.DisplayName == "" || len(first.UsageTypes) == 0 {
		t.Fatalf("expected compact identifiers and usage metadata, got %#v", first)
	}
	if first.Fields != nil {
		t.Fatalf("summary results should omit full fields, got %#v", first.Fields)
	}
}

func TestTermFitAddsContextWarnings(t *testing.T) {
	service := newTestService(t)

	result, err := service.GetTermFit(context.Background(), LOINCRequest{LOINCNum: "1002-7"})
	if err != nil {
		t.Fatalf("get term fit: %v", err)
	}
	if !strings.Contains(strings.ToLower(strings.Join(result.Notes, " ")), "discouraged") {
		t.Fatalf("expected discouraged warning, got %#v", result.Notes)
	}
}

func TestPanelItemsDefaultToCompactAuthoredItems(t *testing.T) {
	service := newTestService(t)

	result, err := service.GetPanelItems(context.Background(), LOINCRequest{LOINCNum: "1001-9", Limit: 1000})
	if err != nil {
		t.Fatalf("get panel items: %v", err)
	}
	if result.Limit != 50 {
		t.Fatalf("expected capped limit 50, got %d", result.Limit)
	}
	if len(result.Results) != 2 {
		t.Fatalf("expected two panel items, got %#v", result.Results)
	}
	if result.Results[0].Sequence != 1 || result.Results[0].ChildLOINCNum != "1000-1" || result.Results[0].AnswerListIDOverride != "LL1" {
		t.Fatalf("expected compact authored panel item, got %#v", result.Results[0])
	}
}

func TestBrowseHierarchyUsesNodeIDs(t *testing.T) {
	service := newTestService(t)

	roots, err := service.BrowseHierarchy(context.Background(), HierarchyRequest{})
	if err != nil {
		t.Fatalf("browse roots: %v", err)
	}
	if len(roots.Results) == 0 || roots.Results[0].NodeID == "" {
		t.Fatalf("expected root node IDs, got %#v", roots.Results)
	}

	children, err := service.BrowseHierarchy(context.Background(), HierarchyRequest{NodeID: roots.Results[0].NodeID})
	if err != nil {
		t.Fatalf("browse children: %v", err)
	}
	if len(children.Results) == 0 || children.Results[0].NodeID == "" {
		t.Fatalf("expected child node IDs, got %#v", children.Results)
	}

	terms, err := service.GetHierarchyTerms(context.Background(), HierarchyTermsRequest{NodeID: roots.Results[0].NodeID, Limit: 10})
	if err != nil {
		t.Fatalf("get hierarchy terms: %v", err)
	}
	if len(terms.Results) == 0 || terms.Results[0].LOINCNum == "" {
		t.Fatalf("expected compact hierarchy terms, got %#v", terms.Results)
	}
}

func writeTestFile(t *testing.T, dir string, name string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	releaseDir := writeMCPTestRelease(t)
	dbPath := filepath.Join(t.TempDir(), "loinc.sqlite")
	if _, err := loinc.Ingest(context.Background(), loinc.IngestOptions{ReleaseDir: releaseDir, DBPath: dbPath}); err != nil {
		t.Fatalf("ingest test release: %v", err)
	}
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 32})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	docsDir := t.TempDir()
	writeTestFile(t, docsDir, "LOINC_CONCEPTS.md", "# Concepts\n\n## Status\n\nDiscouraged terms require caution.\n")
	writeTestFile(t, docsDir, "LOINC_AGENT_GUIDE.md", "# Guide\n")
	writeTestFile(t, docsDir, "LOINC_LICENSE_NOTE.md", "# License\n")
	return NewService(store, NewDocs(docsDir))
}

func writeMCPTestRelease(t *testing.T) string {
	t.Helper()
	releaseDir := t.TempDir()
	writeMCPOptionalFiles(t, releaseDir)
	tableDir := filepath.Join(releaseDir, "LoincTable")
	if err := os.MkdirAll(tableDir, 0o755); err != nil {
		t.Fatalf("mkdir LoincTable: %v", err)
	}
	writeMCPRows(t, filepath.Join(tableDir, "Loinc.csv"), []string{
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
	}, [][]string{
		{"1000-1", "Glucose", "MCnc", "Pt", "Plasma", "Qn", "", "CHEM", "2.80", "ADD", "Glucose mass concentration in plasma", "ACTIVE", "Blood sugar", "1", "", "", "", "", "N", "blood sugar; glucose plasma", "Glucose P", "Both", "", "", "mg/dL", "Glucose [Mass/volume] in Plasma", "mg/dL", "", "", "", "100", "100", "", "", "", "", "", "2.80", "", "Glucose Plasma"},
		{"1001-9", "Example panel", "-", "Pt", "^Patient", "-", "", "PANEL", "2.80", "ADD", "Example panel", "ACTIVE", "", "1", "", "", "", "", "N", "panel", "Example panel", "Both", "", "", "", "Example panel", "", "", "", "", "10", "10", "", "", "Panel", "", "", "2.80", "", "Example panel"},
		{"1002-7", "Glucose", "SCnc", "Pt", "Urine", "Ord", "", "CHEM", "2.80", "ADD", "Glucose presence in urine", "DISCOURAGED", "Urine sugar", "1", "", "", "", "", "N", "urine sugar", "Glucose Ur", "Observation", "", "", "", "Glucose [Presence] in Urine", "", "", "", "", "0", "0", "", "", "", "", "", "2.80", "", "Glucose Urine"},
	})
	return releaseDir
}

func writeMCPOptionalFiles(t *testing.T, releaseDir string) {
	t.Helper()
	writeMCPRows(t, filepath.Join(releaseDir, "LoincTable", "MapTo.csv"), []string{"LOINC", "MAP_TO", "COMMENT"}, nil)
	writeMCPRows(t, filepath.Join(releaseDir, "LoincTable", "SourceOrganization.csv"), []string{"ID", "COPYRIGHT_ID", "NAME", "COPYRIGHT", "TERMS_OF_USE", "URL"}, nil)
	writeMCPRows(t, filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "Part.csv"), []string{"PartNumber", "PartTypeName", "PartName", "PartDisplayName", "Status"}, [][]string{{"LP1", "COMPONENT", "Glucose", "Glucose", "ACTIVE"}, {"LPROOT", "HIERARCHY", "Root", "Root", "ACTIVE"}})
	writeMCPRows(t, filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "LoincPartLink_Primary.csv"), []string{"LoincNumber", "LongCommonName", "PartNumber", "PartName", "PartCodeSystem", "PartTypeName", "LinkTypeName", "Property"}, [][]string{{"1000-1", "Glucose [Mass/volume] in Plasma", "LP1", "Glucose", "http://loinc.org", "COMPONENT", "Primary", "http://loinc.org/property/COMPONENT"}})
	writeMCPRows(t, filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "LoincPartLink_Supplementary.csv"), []string{"LoincNumber", "LongCommonName", "PartNumber", "PartName", "PartCodeSystem", "PartTypeName", "LinkTypeName", "Property"}, nil)
	writeMCPRows(t, filepath.Join(releaseDir, "AccessoryFiles", "AnswerFile", "AnswerList.csv"), []string{"AnswerListId", "AnswerListName", "AnswerListOID", "ExtDefinedYN", "ExtDefinedAnswerListCodeSystem", "ExtDefinedAnswerListLink", "AnswerStringId", "LocalAnswerCode", "LocalAnswerCodeSystem", "SequenceNumber", "DisplayText", "ExtCodeId", "ExtCodeDisplayName", "ExtCodeSystem", "ExtCodeSystemVersion", "ExtCodeSystemCopyrightNotice", "SubsequentTextPrompt", "Description", "Score"}, [][]string{{"LL1", "Positive/negative", "", "N", "", "", "LA1", "POS", "LOCAL", "1", "Positive", "", "", "", "", "", "", "", "1"}})
	writeMCPRows(t, filepath.Join(releaseDir, "AccessoryFiles", "AnswerFile", "LoincAnswerListLink.csv"), []string{"LoincNumber", "LongCommonName", "AnswerListId", "AnswerListName", "AnswerListLinkType", "ApplicableContext"}, [][]string{{"1000-1", "Glucose [Mass/volume] in Plasma", "LL1", "Positive/negative", "EXAMPLE", ""}})
	writeMCPRows(t, filepath.Join(releaseDir, "AccessoryFiles", "PanelsAndForms", "PanelsAndForms.csv"), []string{"ParentId", "ParentLoinc", "ParentName", "ID", "SEQUENCE", "Loinc", "LoincName", "DisplayNameForForm", "ObservationRequiredInPanel", "ObservationIdInForm", "SkipLogicHelpText", "DefaultValue", "EntryType", "DataTypeInForm", "DataTypeSource", "AnswerSequenceOverride", "ConditionForInclusion", "AllowableAlternative", "ObservationCategory", "Context", "ConsistencyChecks", "RelevanceEquation", "CodingInstructions", "QuestionCardinality", "AnswerCardinality", "AnswerListIdOverride", "AnswerListTypeOverride", "EXTERNAL_COPYRIGHT_NOTICE", "AdditionalCopyright"}, [][]string{{"P1", "1001-9", "Example panel", "I1", "1", "1000-1", "Glucose", "Glucose", "R", "", "", "", "Q", "", "", "", "", "", "", "", "", "", "", "", "", "LL1", "", "", ""}, {"P1", "1001-9", "Example panel", "I2", "2", "1002-7", "Glucose urine", "Glucose urine", "O", "", "", "", "Q", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""}})
	writeMCPRows(t, filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "ParentGroup.csv"), []string{"ParentGroupId", "ParentGroup", "Status"}, nil)
	writeMCPRows(t, filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "Group.csv"), []string{"ParentGroupId", "GroupId", "Group", "Archetype", "Status", "VersionFirstReleased"}, nil)
	writeMCPRows(t, filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "GroupLoincTerms.csv"), []string{"Category", "GroupId", "Archetype", "LoincNumber", "LongCommonName"}, nil)
	writeMCPRows(t, filepath.Join(releaseDir, "AccessoryFiles", "ComponentHierarchyBySystem", "ComponentHierarchyBySystem.csv"), []string{"PATH_TO_ROOT", "SEQUENCE", "IMMEDIATE_PARENT", "CODE", "CODE_TEXT"}, [][]string{{"", "1", "", "LPROOT", "{component}"}, {"LPROOT", "1", "LPROOT", "LP1", "Glucose"}, {"LPROOT.LP1", "1", "LP1", "1000-1", "Glucose P"}})
}

func writeMCPRows(t *testing.T, path string, header []string, rows [][]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	if err := writer.Write(header); err != nil {
		t.Fatalf("write header %s: %v", path, err)
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			t.Fatalf("write row %s: %v", path, err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatalf("flush %s: %v", path, err)
	}
}
