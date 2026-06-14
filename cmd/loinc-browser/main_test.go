package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"testing"

	"loinc-browser/internal/loinc"
)

func TestLoadDotEnvAndDefaultAddr(t *testing.T) {
	t.Setenv("LOINC_BROWSER_ADDR", "")
	t.Setenv("PORT", "")

	envPath := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envPath, []byte(`
# comment
PORT=9090
LOINC_BROWSER_ADDR=:9191
IGNORED_WITHOUT_VALUE
`), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	if err := loadDotEnv(envPath); err != nil {
		t.Fatalf("load env: %v", err)
	}
	if got := defaultServeAddr(); got != ":9191" {
		t.Fatalf("expected :9191, got %q", got)
	}
}

func TestDefaultAddrUsesPortWhenAddrMissing(t *testing.T) {
	t.Setenv("LOINC_BROWSER_ADDR", "")
	t.Setenv("PORT", "7070")
	if got := defaultServeAddr(); got != ":7070" {
		t.Fatalf("expected :7070, got %q", got)
	}
}

func TestEnsureDatabaseFromLocalZipImportsWhenDatabaseMissing(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "Loinc_Test.zip"), testCLIReleaseZip(t, "4000-1", "Bootstrap glucose term"), 0o600); err != nil {
		t.Fatalf("write release zip: %v", err)
	}
	dbPath := filepath.Join(cwd, "data", "loinc.sqlite")

	if err := ensureDatabaseFromLocalZip(context.Background(), cwd, dbPath); err != nil {
		t.Fatalf("ensure database: %v", err)
	}
	hasData, err := databaseHasTerms(dbPath)
	if err != nil {
		t.Fatalf("check database: %v", err)
	}
	if !hasData {
		t.Fatalf("expected auto-ingested database to have terms")
	}
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	results, err := store.Search(context.Background(), loinc.SearchParams{Query: "bootstrap", Limit: 10})
	if err != nil {
		t.Fatalf("search auto-ingested database: %v", err)
	}
	if len(results.Results) != 1 || results.Results[0].LOINCNum != "4000-1" {
		t.Fatalf("expected bootstrap term, got %#v", results.Results)
	}
}

func TestEnsureDatabaseFromLocalZipDoesNotOverwriteExistingData(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "Loinc_Test.zip"), testCLIReleaseZip(t, "5000-1", "New zip term"), 0o600); err != nil {
		t.Fatalf("write release zip: %v", err)
	}
	existingRelease := filepath.Join(cwd, "existing")
	writeCLIReleaseDir(t, existingRelease, "4999-9", "Existing database term")
	dbPath := filepath.Join(cwd, "data", "loinc.sqlite")
	if _, err := loinc.Ingest(context.Background(), loinc.IngestOptions{ReleaseDir: existingRelease, DBPath: dbPath}); err != nil {
		t.Fatalf("ingest existing release: %v", err)
	}

	if err := ensureDatabaseFromLocalZip(context.Background(), cwd, dbPath); err != nil {
		t.Fatalf("ensure database: %v", err)
	}
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	results, err := store.Search(context.Background(), loinc.SearchParams{Query: "existing", Limit: 10})
	if err != nil {
		t.Fatalf("search existing database: %v", err)
	}
	if len(results.Results) != 1 || results.Results[0].LOINCNum != "4999-9" {
		t.Fatalf("expected existing term to remain, got %#v", results.Results)
	}
}

func testCLIReleaseZip(t *testing.T, loincNum string, longName string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	file, err := zipWriter.Create("Loinc_Test/LoincTable/Loinc.csv")
	if err != nil {
		t.Fatalf("create zip csv: %v", err)
	}
	writeCLICSV(t, file, loincNum, longName)
	writeCLIRequiredZipFiles(t, zipWriter)
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func writeCLIReleaseDir(t *testing.T, releaseDir string, loincNum string, longName string) {
	t.Helper()
	tableDir := filepath.Join(releaseDir, "LoincTable")
	if err := os.MkdirAll(tableDir, 0o755); err != nil {
		t.Fatalf("mkdir release: %v", err)
	}
	file, err := os.Create(filepath.Join(tableDir, "Loinc.csv"))
	if err != nil {
		t.Fatalf("create csv: %v", err)
	}
	defer file.Close()
	writeCLICSV(t, file, loincNum, longName)
	writeCLIRequiredReleaseFiles(t, releaseDir)
}

func writeCLICSV(t *testing.T, file io.Writer, loincNum string, longName string) {
	t.Helper()
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
	row := []string{
		loincNum, "Bootstrap glucose", "MCnc", "Pt", "Serum", "Qn", "", "CHEM",
		"2.82", "ADD", "Bootstrap test term", "ACTIVE",
		"Bootstrap term", "1", "", "", "", "", "N", "bootstrap existing zip",
		"Bootstrap Glucose", "Observation", "", "", "mg/dL", longName,
		"mg/dL", "", "", "", "1", "1", "", "", "", "", "", "2.82", "", "Bootstrap glucose",
	}
	if err := writer.Write(header); err != nil {
		t.Fatalf("write csv header: %v", err)
	}
	if err := writer.Write(row); err != nil {
		t.Fatalf("write csv row: %v", err)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatalf("flush csv: %v", err)
	}
}

func writeCLIRequiredReleaseFiles(t *testing.T, releaseDir string) {
	t.Helper()
	writeFileCSV := func(rel string, header []string, rows [][]string) {
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
		writeRowsCSV(t, file, header, rows)
	}
	for _, spec := range cliRequiredCSVSpecs() {
		writeFileCSV(spec.path, spec.header, spec.rows)
	}
}

func writeCLIRequiredZipFiles(t *testing.T, zipWriter *zip.Writer) {
	t.Helper()
	for _, spec := range cliRequiredCSVSpecs() {
		file, err := zipWriter.Create(filepath.ToSlash(filepath.Join("Loinc_Test", spec.path)))
		if err != nil {
			t.Fatalf("create zip entry %s: %v", spec.path, err)
		}
		writeRowsCSV(t, file, spec.header, spec.rows)
	}
}

type cliCSVSpec struct {
	path   string
	header []string
	rows   [][]string
}

func cliRequiredCSVSpecs() []cliCSVSpec {
	return []cliCSVSpec{
		{path: "LoincTable/MapTo.csv", header: []string{"LOINC", "MAP_TO", "COMMENT"}},
		{path: "LoincTable/SourceOrganization.csv", header: []string{"ID", "COPYRIGHT_ID", "NAME", "COPYRIGHT", "TERMS_OF_USE", "URL"}},
		{path: "AccessoryFiles/PartFile/Part.csv", header: []string{"PartNumber", "PartTypeName", "PartName", "PartDisplayName", "Status"}},
		{path: "AccessoryFiles/PartFile/LoincPartLink_Primary.csv", header: []string{"LoincNumber", "LongCommonName", "PartNumber", "PartName", "PartCodeSystem", "PartTypeName", "LinkTypeName", "Property"}},
		{path: "AccessoryFiles/PartFile/LoincPartLink_Supplementary.csv", header: []string{"LoincNumber", "LongCommonName", "PartNumber", "PartName", "PartCodeSystem", "PartTypeName", "LinkTypeName", "Property"}},
		{path: "AccessoryFiles/AnswerFile/AnswerList.csv", header: []string{"AnswerListId", "AnswerListName", "AnswerListOID", "ExtDefinedYN", "ExtDefinedAnswerListCodeSystem", "ExtDefinedAnswerListLink", "AnswerStringId", "LocalAnswerCode", "LocalAnswerCodeSystem", "SequenceNumber", "DisplayText", "ExtCodeId", "ExtCodeDisplayName", "ExtCodeSystem", "ExtCodeSystemVersion", "ExtCodeSystemCopyrightNotice", "SubsequentTextPrompt", "Description", "Score"}},
		{path: "AccessoryFiles/AnswerFile/LoincAnswerListLink.csv", header: []string{"LoincNumber", "LongCommonName", "AnswerListId", "AnswerListName", "AnswerListLinkType", "ApplicableContext"}},
		{path: "AccessoryFiles/PanelsAndForms/PanelsAndForms.csv", header: []string{"ParentId", "ParentLoinc", "ParentName", "ID", "SEQUENCE", "Loinc", "LoincName", "DisplayNameForForm", "ObservationRequiredInPanel", "ObservationIdInForm", "SkipLogicHelpText", "DefaultValue", "EntryType", "DataTypeInForm", "DataTypeSource", "AnswerSequenceOverride", "ConditionForInclusion", "AllowableAlternative", "ObservationCategory", "Context", "ConsistencyChecks", "RelevanceEquation", "CodingInstructions", "QuestionCardinality", "AnswerCardinality", "AnswerListIdOverride", "AnswerListTypeOverride", "EXTERNAL_COPYRIGHT_NOTICE", "AdditionalCopyright"}},
		{path: "AccessoryFiles/GroupFile/ParentGroup.csv", header: []string{"ParentGroupId", "ParentGroup", "Status"}},
		{path: "AccessoryFiles/GroupFile/Group.csv", header: []string{"ParentGroupId", "GroupId", "Group", "Archetype", "Status", "VersionFirstReleased"}},
		{path: "AccessoryFiles/GroupFile/GroupLoincTerms.csv", header: []string{"Category", "GroupId", "Archetype", "LoincNumber", "LongCommonName"}},
		{path: "AccessoryFiles/ComponentHierarchyBySystem/ComponentHierarchyBySystem.csv", header: []string{"PATH_TO_ROOT", "SEQUENCE", "IMMEDIATE_PARENT", "CODE", "CODE_TEXT"}},
	}
}

func writeRowsCSV(t *testing.T, file io.Writer, header []string, rows [][]string) {
	t.Helper()
	writer := csv.NewWriter(file)
	if err := writer.Write(header); err != nil {
		t.Fatalf("write csv header: %v", err)
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			t.Fatalf("write csv row: %v", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatalf("flush csv: %v", err)
	}
}
