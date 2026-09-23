package udp

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// writeUDPTestRelease builds a minimal release directory covering every op this package's tests
// exercise: term "10000-1" (active, COMPONENT part LP1000-1, hierarchy leaf under
// LP2000-1 -> LP1000-1), deprecated term "20000-8" mapped to "10000-1" (drives the "translate"
// op via the local loinc-map-to ConceptMap), and answer list LL1000-1 with two answers (drives
// "expand"). Modeled on pkg/terminology's writeTerminologyTestRelease.
func writeUDPTestRelease(t *testing.T) string {
	t.Helper()
	releaseDir := filepath.Join(t.TempDir(), "Loinc_2.80")
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		t.Fatalf("mkdir release dir: %v", err)
	}

	loincRows := [][]string{
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
	}
	// Pad "loinc-all" ($expand http://loinc.org/vs) past the 1400-byte UDP response budget, to
	// exercise the truncated/use-http path (§6) without needing the real DB.
	for i := 0; i < 30; i++ {
		code := fmt.Sprintf("3%04d-0", i)
		loincRows = append(loincRows, []string{
			code, "Filler", "MCnc", "Pt", "Serum", "Qn", "", "CHEM",
			"2.80", "ADD", "Filler term for UDP truncation test", "ACTIVE",
			"", "1", "", "", "", "", "N", "",
			"Filler", "Observation", "", "", "", "Filler analyte [Mass/volume] in Serum for truncation padding",
			"", "", "", "", "0", "0", "", "", "", "", "", "1.0", "", "",
		})
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
		loincRows)

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
			{"LP3000-1", "CLASS", "CHEM", "CHEM", "ACTIVE"},
		})
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "LoincPartLink_Primary.csv"),
		[]string{"LoincNumber", "LongCommonName", "PartNumber", "PartName", "PartCodeSystem", "PartTypeName", "LinkTypeName", "Property"},
		[][]string{
			{"10000-1", "Cholesterol [Mass/volume] in Serum", "LP1000-1", "Cholesterol", "http://loinc.org", "COMPONENT", "Primary", "http://loinc.org/property/COMPONENT"},
			{"10000-1", "Cholesterol [Mass/volume] in Serum", "LP3000-1", "CHEM", "http://loinc.org", "CLASS", "Primary", "http://loinc.org/property/CLASS"},
			{"20000-8", "Deprecated Cholesterol in Serum", "LP1000-1", "Cholesterol", "http://loinc.org", "COMPONENT", "Primary", "http://loinc.org/property/COMPONENT"},
		})
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "LoincPartLink_Supplementary.csv"),
		[]string{"LoincNumber", "LongCommonName", "PartNumber", "PartName", "PartCodeSystem", "PartTypeName", "LinkTypeName", "Property"}, nil)

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
		[]string{"ParentGroupId", "ParentGroup", "Status"}, nil)
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "Group.csv"),
		[]string{"ParentGroupId", "GroupId", "Group", "Archetype", "Status", "VersionFirstReleased"}, nil)
	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "GroupLoincTerms.csv"),
		[]string{"Category", "GroupId", "Archetype", "LoincNumber", "LongCommonName"}, nil)

	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "ComponentHierarchyBySystem", "ComponentHierarchyBySystem.csv"),
		[]string{"PATH_TO_ROOT", "SEQUENCE", "IMMEDIATE_PARENT", "CODE", "CODE_TEXT"},
		[][]string{
			{"", "1", "", "LP2000-1", "Lipids"},
			{"LP2000-1", "1", "LP2000-1", "LP1000-1", "Cholesterol"},
			{"LP2000-1.LP1000-1", "1", "LP1000-1", "10000-1", "Cholesterol Ser-mCnc"},
		})

	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "ConsumerName", "ConsumerName.csv"),
		[]string{"LoincNumber", "ConsumerName"}, nil)

	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "LinguisticVariants", "LinguisticVariants.csv"),
		[]string{"ID", "ISO_LANGUAGE", "ISO_COUNTRY", "LANGUAGE_NAME", "PRODUCER"}, nil)

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
