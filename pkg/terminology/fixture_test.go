package terminology

import (
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// writeTerminologyTestRelease builds a minimal release directory covering all five $lookup code
// kinds (term, LP part, LL answer list, LA answer, LG group), modeled on writeServerTestRelease
// in internal/server/server_test.go and the loinc package's own fixture in
// internal/loinc/loinc_test.go.
//
// Fixture shape: term "10000-1" (active, COMPONENT part LP1000-1, hierarchy leaf under
// LP2000-1 -> LP1000-1, member of group LG1000-1, answered by list LL1000-1); deprecated term
// "20000-8" (STATUS=DEPRECATED, MAP_TO "10000-1"); answer list LL1000-1 with answers LA1-1
// ("Positive") and LA2-2 ("Negative"); group LG1000-1 under parent group PG1000; a German
// linguistic variant and a ConsumerName row for "10000-1".
func writeTerminologyTestRelease(t *testing.T) string {
	t.Helper()
	releaseDir := filepath.Join(t.TempDir(), "Loinc_2.80")
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		t.Fatalf("mkdir release dir: %v", err)
	}

	writeCSV(t, filepath.Join(releaseDir, "LoincTable", "Loinc.csv"),
		[]string{
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
		},
		[][]string{
			{
				"10000-1", "Cholesterol", "MCnc", "Pt", "Serum", "Qn", "", "CHEM",
				"2.80", "ADD", "Cholesterol mass concentration in serum", "ACTIVE",
				"", "1", "", "", "", "", "N", "lipid; cholesterol serum",
				"Chol Ser", "Observation", "", "", "mg/dL", "Cholesterol [Mass/volume] in Serum",
				"mg/dL", "", "", "", "100", "0", "", "", "", "", "", "1.0", "", "Cholesterol Serum",
			},
			{
				"20000-8", "Cholesterol", "MCnc", "Pt", "Serum", "Qn", "", "CHEM",
				"2.80", "MIN", "Deprecated duplicate", "DEPRECATED",
				"", "1", "", "", "", "", "N", "lipid; cholesterol serum deprecated",
				"Dep Chol Ser", "Observation", "", "", "mg/dL", "Deprecated Cholesterol in Serum",
				"mg/dL", "Duplicate", "", "", "0", "0", "", "", "", "", "", "1.0", "", "Deprecated Cholesterol Serum",
			},
		})

	writeCSV(t, filepath.Join(releaseDir, "LoincTable", "MapTo.csv"),
		[]string{"LOINC", "MAP_TO", "COMMENT"},
		[][]string{{"20000-8", "10000-1", ""}})
	writeCSV(t, filepath.Join(releaseDir, "LoincTable", "SourceOrganization.csv"),
		[]string{"ID", "COPYRIGHT_ID", "NAME", "COPYRIGHT", "TERMS_OF_USE", "URL"}, nil)

	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "Part.csv"),
		[]string{"PartNumber", "PartTypeName", "PartName", "PartDisplayName", "Status"},
		[][]string{
			{"LP1000-1", "COMPONENT", "Cholesterol", "Cholesterol", "ACTIVE"},
			{"LP2000-1", "COMPONENT", "Lipids", "Lipids", "ACTIVE"},
		})
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "LoincPartLink_Primary.csv"),
		[]string{"LoincNumber", "LongCommonName", "PartNumber", "PartName", "PartCodeSystem", "PartTypeName", "LinkTypeName", "Property"},
		[][]string{
			{"10000-1", "Cholesterol [Mass/volume] in Serum", "LP1000-1", "Cholesterol", "http://loinc.org", "COMPONENT", "Primary", "http://loinc.org/property/COMPONENT"},
			{"20000-8", "Deprecated Cholesterol in Serum", "LP1000-1", "Cholesterol", "http://loinc.org", "COMPONENT", "Primary", "http://loinc.org/property/COMPONENT"},
		})
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "LoincPartLink_Supplementary.csv"),
		[]string{"LoincNumber", "LongCommonName", "PartNumber", "PartName", "PartCodeSystem", "PartTypeName", "LinkTypeName", "Property"},
		[][]string{
			{"10000-1", "Cholesterol [Mass/volume] in Serum", "LP1000-1", "Cholesterol", "http://loinc.org", "COMPONENT", "Supplementary", "http://loinc.org/property/category"},
		})

	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "AnswerFile", "AnswerList.csv"),
		[]string{"AnswerListId", "AnswerListName", "AnswerListOID", "ExtDefinedYN", "ExtDefinedAnswerListCodeSystem", "ExtDefinedAnswerListLink", "AnswerStringId", "LocalAnswerCode", "LocalAnswerCodeSystem", "SequenceNumber", "DisplayText", "ExtCodeId", "ExtCodeDisplayName", "ExtCodeSystem", "ExtCodeSystemVersion", "ExtCodeSystemCopyrightNotice", "SubsequentTextPrompt", "Description", "Score"},
		[][]string{
			{"LL1000-1", "Positive negative", "1.2.3.4", "N", "", "", "LA1-1", "", "", "1", "Positive", "", "", "", "", "", "", "", "1"},
			{"LL1000-1", "Positive negative", "1.2.3.4", "N", "", "", "LA2-2", "", "", "2", "Negative", "", "", "", "", "", "", "", "0"},
		})
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "AnswerFile", "LoincAnswerListLink.csv"),
		[]string{"LoincNumber", "LongCommonName", "AnswerListId", "AnswerListName", "AnswerListLinkType", "ApplicableContext"},
		[][]string{{"10000-1", "Cholesterol [Mass/volume] in Serum", "LL1000-1", "Positive negative", "NORMATIVE", ""}})

	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "PanelsAndForms", "PanelsAndForms.csv"),
		[]string{"ParentId", "ParentLoinc", "ParentName", "ID", "SEQUENCE", "Loinc", "LoincName", "DisplayNameForForm", "ObservationRequiredInPanel", "ObservationIdInForm", "SkipLogicHelpText", "DefaultValue", "EntryType", "DataTypeInForm", "DataTypeSource", "AnswerSequenceOverride", "ConditionForInclusion", "AllowableAlternative", "ObservationCategory", "Context", "ConsistencyChecks", "RelevanceEquation", "CodingInstructions", "QuestionCardinality", "AnswerCardinality", "AnswerListIdOverride", "AnswerListTypeOverride", "EXTERNAL_COPYRIGHT_NOTICE", "AdditionalCopyright"},
		nil)

	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "ParentGroup.csv"),
		[]string{"ParentGroupId", "ParentGroup", "Status"},
		[][]string{{"PG1000", "Chemistry", "ACTIVE"}})
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "Group.csv"),
		[]string{"ParentGroupId", "GroupId", "Group", "Archetype", "Status", "VersionFirstReleased"},
		[][]string{{"PG1000", "LG1000-1", "Chemistry tests", "Laboratory", "ACTIVE", "1.0"}})
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "GroupLoincTerms.csv"),
		[]string{"Category", "GroupId", "Archetype", "LoincNumber", "LongCommonName"},
		[][]string{{"Laboratory", "LG1000-1", "Laboratory", "10000-1", "Cholesterol [Mass/volume] in Serum"}})

	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "ComponentHierarchyBySystem", "ComponentHierarchyBySystem.csv"),
		[]string{"PATH_TO_ROOT", "SEQUENCE", "IMMEDIATE_PARENT", "CODE", "CODE_TEXT"},
		[][]string{
			{"", "1", "", "LP2000-1", "Lipids"},
			{"LP2000-1", "1", "LP2000-1", "LP1000-1", "Cholesterol"},
			{"LP2000-1.LP1000-1", "1", "LP1000-1", "10000-1", "Cholesterol Ser-mCnc"},
			// LP3000-1 is a hierarchy-only node (child of LP2000-1) that is never published as a
			// Part.csv row -- see fixture note above and TestSubsumesHierarchyOnlyPart.
			{"LP2000-1", "2", "LP2000-1", "LP3000-1", "Lipid Marker Group"},
		})

	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "ConsumerName", "ConsumerName.csv"),
		[]string{"LoincNumber", "ConsumerName"},
		[][]string{{"10000-1", "Cholesterol, Blood"}})

	// Two languages (not just one) exercise FHIRLinguisticVariants' UNION ALL across per-language
	// tables (internal/loinc/fhir_queries.go), including that its "order by seq" preserves
	// LinguisticVariants.csv's ID order (German ID=1 before French ID=2) rather than whatever
	// order SQLite might otherwise return UNION ALL branches in.
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "LinguisticVariants", "LinguisticVariants.csv"),
		[]string{"ID", "ISO_LANGUAGE", "ISO_COUNTRY", "LANGUAGE_NAME", "PRODUCER"},
		[][]string{
			{"1", "de", "DE", "German (GERMANY)", "Test"},
			{"2", "fr", "FR", "French (FRANCE)", "Test"},
		})
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "LinguisticVariants", "deDE1LinguisticVariant.csv"),
		[]string{"LOINC_NUM", "COMPONENT", "PROPERTY", "TIME_ASPCT", "SYSTEM", "SCALE_TYP", "METHOD_TYP", "CLASS", "SHORTNAME", "LONG_COMMON_NAME", "RELATEDNAMES2", "LinguisticVariantDisplayName"},
		[][]string{{"10000-1", "Cholesterin", "MCnc", "Pt", "Serum", "Qn", "", "Klinische Chemie", "Chol Ser DE", "Cholesterin [Masse/Volumen] in Serum", "", "Cholesterin"}})
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "LinguisticVariants", "frFR2LinguisticVariant.csv"),
		[]string{"LOINC_NUM", "COMPONENT", "PROPERTY", "TIME_ASPCT", "SYSTEM", "SCALE_TYP", "METHOD_TYP", "CLASS", "SHORTNAME", "LONG_COMMON_NAME", "RELATEDNAMES2", "LinguisticVariantDisplayName"},
		[][]string{{"10000-1", "Cholesterol", "MCnc", "Pt", "Serum", "Qn", "", "Chimie clinique", "Chol Ser FR", "Cholesterol [Masse/Volume] dans Serum", "", "Cholesterol"}})

	return releaseDir
}

func writeCSV(t *testing.T, path string, header []string, rows [][]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer file.Close()
	writeRowsCSV(t, file, header, rows)
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
