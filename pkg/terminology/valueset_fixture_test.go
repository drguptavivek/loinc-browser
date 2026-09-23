package terminology

import (
	"context"
	"path/filepath"
	"testing"

	"loinc-browser/internal/loinc"
)

// writeValueSetFixture extends writeTerminologyTestRelease's release directory with the rows
// every ValueSet catalogue entry needs (§4.6.1): more terms (ranks, HL7-attachment fields, a
// third-party copyright notice, a second deprecated term), a second answer list and group (for
// search), and the four raw-CSV-backed named sets (document ontology, RSNA playbook, universal
// lab orders, imaging document codes).
func writeValueSetFixture(t *testing.T) string {
	t.Helper()
	releaseDir := writeTerminologyTestRelease(t)

	writeCSV(t, filepath.Join(releaseDir, "LoincTable", "Loinc.csv"), loincCSVHeader, [][]string{
		loincCSVRow(map[string]string{"LOINC_NUM": "10000-1", "COMPONENT": "Cholesterol", "PROPERTY": "MCnc", "TIME_ASPCT": "Pt", "SYSTEM": "Serum", "SCALE_TYP": "Qn", "CLASS": "CHEM", "STATUS": "ACTIVE", "LONG_COMMON_NAME": "Cholesterol [Mass/volume] in Serum"}),
		loincCSVRow(map[string]string{"LOINC_NUM": "20000-8", "COMPONENT": "Cholesterol", "PROPERTY": "MCnc", "TIME_ASPCT": "Pt", "SYSTEM": "Serum", "SCALE_TYP": "Qn", "CLASS": "CHEM", "STATUS": "DEPRECATED", "LONG_COMMON_NAME": "Deprecated Cholesterol in Serum"}),
		loincCSVRow(map[string]string{"LOINC_NUM": "11000-0", "COMPONENT": "Sodium", "PROPERTY": "SCnc", "TIME_ASPCT": "Pt", "SYSTEM": "Serum", "SCALE_TYP": "Qn", "CLASS": "CHEM", "STATUS": "ACTIVE",
			"COMMON_TEST_RANK": "5", "COMMON_ORDER_RANK": "2", "ValidHL7AttachmentRequest": "Y", "HL7_ATTACHMENT_STRUCTURE": "IG exists",
			"EXTERNAL_COPYRIGHT_NOTICE": "Third party notice", "LONG_COMMON_NAME": "Sodium [Moles/volume] in Serum"}),
		loincCSVRow(map[string]string{"LOINC_NUM": "12000-8", "COMPONENT": "Potassium", "PROPERTY": "SCnc", "TIME_ASPCT": "Pt", "SYSTEM": "Serum", "SCALE_TYP": "Qn", "CLASS": "CHEM", "STATUS": "ACTIVE",
			"COMMON_TEST_RANK": "1", "COMMON_ORDER_RANK": "1", "HL7_ATTACHMENT_STRUCTURE": "No IG exists", "LONG_COMMON_NAME": "Potassium [Moles/volume] in Serum"}),
		loincCSVRow(map[string]string{"LOINC_NUM": "13000-6", "COMPONENT": "Chloride", "PROPERTY": "SCnc", "TIME_ASPCT": "Pt", "SYSTEM": "Serum", "SCALE_TYP": "Qn", "CLASS": "CHEM", "STATUS": "DEPRECATED", "LONG_COMMON_NAME": "Deprecated Chloride in Serum"}),
	})

	// A second answer list and group, distinct names, for ValueSet search (§4.6.3).
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "AnswerFile", "AnswerList.csv"),
		[]string{"AnswerListId", "AnswerListName", "AnswerListOID", "ExtDefinedYN", "ExtDefinedAnswerListCodeSystem", "ExtDefinedAnswerListLink", "AnswerStringId", "LocalAnswerCode", "LocalAnswerCodeSystem", "SequenceNumber", "DisplayText", "ExtCodeId", "ExtCodeDisplayName", "ExtCodeSystem", "ExtCodeSystemVersion", "ExtCodeSystemCopyrightNotice", "SubsequentTextPrompt", "Description", "Score"},
		[][]string{
			{"LL1000-1", "Positive negative", "1.2.3.4", "N", "", "", "LA1-1", "", "", "1", "Positive", "", "", "", "", "", "", "", "1"},
			{"LL1000-1", "Positive negative", "1.2.3.4", "N", "", "", "LA2-2", "", "", "2", "Negative", "", "", "", "", "", "", "", "0"},
			{"LL2000-2", "Severity scale", "1.2.3.5", "N", "", "", "LA9-9", "", "", "1", "Mild", "", "", "", "", "", "", "", "0"},
			{"LL2000-2", "Severity scale", "1.2.3.5", "N", "", "", "LA10-7", "", "", "2", "Severe", "", "", "", "", "", "", "", "0"},
		})
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "Group.csv"),
		[]string{"ParentGroupId", "GroupId", "Group", "Archetype", "Status", "VersionFirstReleased"},
		[][]string{
			{"PG1000", "LG1000-1", "Chemistry tests", "Laboratory", "ACTIVE", "1.0"},
			{"PG1000", "LG2000-2", "Second chemistry group", "Laboratory", "ACTIVE", "1.0"},
		})
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "GroupLoincTerms.csv"),
		[]string{"Category", "GroupId", "Archetype", "LoincNumber", "LongCommonName"},
		[][]string{
			{"Laboratory", "LG1000-1", "Laboratory", "10000-1", "Cholesterol [Mass/volume] in Serum"},
			{"Laboratory", "LG2000-2", "Laboratory", "11000-0", "Sodium [Moles/volume] in Serum"},
		})

	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "DocumentOntology", "DocumentOntology.csv"),
		[]string{"LoincNumber", "PartNumber", "PartTypeName", "PartSequenceOrder", "PartName"},
		[][]string{
			{"10000-1", "LP173418-7", "Document.Kind", "1", "Note"},
			{"11000-0", "LP173418-7", "Document.Kind", "1", "Note"},
		})
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "LoincRsnaRadiologyPlaybook", "LoincRsnaRadiologyPlaybook.csv"),
		[]string{"LoincNumber", "LongCommonName", "PartNumber", "PartTypeName", "PartName", "PartSequenceOrder", "RID", "PreferredName", "RPID", "LongName"},
		[][]string{
			{"10000-1", "Cholesterol [Mass/volume] in Serum", "", "", "", "", "", "", "RPID9001", "Test RadLex Cholesterol"},
			{"12000-8", "Potassium [Moles/volume] in Serum", "", "", "", "", "", "", "RPID9002", "Test RadLex Potassium"},
		})
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "LoincUniversalLabOrdersValueSet", "LoincUniversalLabOrdersValueSet.csv"),
		[]string{"LOINC_NUM", "LONG_COMMON_NAME", "ORDER_OBS"},
		[][]string{
			{"10000-1", "Cholesterol [Mass/volume] in Serum", "Order"},
			{"11000-0", "Sodium [Moles/volume] in Serum", "Order"},
		})
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "ImagingDocuments", "ImagingDocumentCodes.csv"),
		[]string{"LOINC_NUM", "LONG_COMMON_NAME"},
		[][]string{{"12000-8", "Potassium [Moles/volume] in Serum"}})

	return releaseDir
}

// newValueSetTestService ingests writeValueSetFixture into a fresh DB and returns a Service over
// it, mirroring newTestService in terminology_test.go.
func newValueSetTestService(t *testing.T) *Service {
	t.Helper()
	ctx := context.Background()
	releaseDir := writeValueSetFixture(t)
	dbPath := filepath.Join(t.TempDir(), "loinc.sqlite")
	if _, err := loinc.Ingest(ctx, loinc.IngestOptions{ReleaseDir: releaseDir, DBPath: dbPath}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return NewService(func() (*loinc.Store, error) { return store, nil })
}
