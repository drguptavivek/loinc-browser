package terminology

import (
	"path/filepath"
	"testing"
)

// loincCSVHeader is the exact Loinc.csv column order writeTerminologyTestRelease uses. Kept here,
// rather than exported from fixture_test.go, so this file can rewrite Loinc.csv with extra rows
// without editing that shared fixture.
var loincCSVHeader = []string{
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

// loincCSVRow builds one Loinc.csv row matching loincCSVHeader, filling every column blank except
// the ones named in fields.
func loincCSVRow(fields map[string]string) []string {
	row := make([]string, len(loincCSVHeader))
	for i, column := range loincCSVHeader {
		row[i] = fields[column]
	}
	return row
}

// writeConceptMapQuestionnaireFixture extends writeTerminologyTestRelease's release directory
// with the accessory files ConceptMap/$translate/Questionnaire need: an IEEE row and a term-level
// RadLex playbook row for "10000-1", two PartRelatedCodeMapping rows for part "LP1000-1" (SNOMED
// CT and www.radlex.org, covering two of the eight PartRelatedCodeMapping-driven maps), a comment
// on the existing MapTo row, and a panel "30000-6" covering every Questionnaire item shape: a
// choice item (10000-1, via its own linked answer list LL1000-1), a required decimal item
// (40000-0, SCALE_TYP=Qn, no answer list), a group item (50000-3, itself a one-child panel), and
// a choice item resolved through AnswerListIdOverride rather than its own link (60000-1).
func writeConceptMapQuestionnaireFixture(t *testing.T) string {
	t.Helper()
	releaseDir := writeTerminologyTestRelease(t)

	writeCSV(t, filepath.Join(releaseDir, "LoincTable", "Loinc.csv"), loincCSVHeader, [][]string{
		loincCSVRow(map[string]string{"LOINC_NUM": "10000-1", "COMPONENT": "Cholesterol", "PROPERTY": "MCnc", "TIME_ASPCT": "Pt", "SYSTEM": "Serum", "SCALE_TYP": "Qn", "CLASS": "CHEM", "STATUS": "ACTIVE", "LONG_COMMON_NAME": "Cholesterol [Mass/volume] in Serum"}),
		loincCSVRow(map[string]string{"LOINC_NUM": "20000-8", "COMPONENT": "Cholesterol", "PROPERTY": "MCnc", "TIME_ASPCT": "Pt", "SYSTEM": "Serum", "SCALE_TYP": "Qn", "CLASS": "CHEM", "STATUS": "DEPRECATED", "LONG_COMMON_NAME": "Deprecated Cholesterol in Serum"}),
		loincCSVRow(map[string]string{"LOINC_NUM": "30000-6", "COMPONENT": "Test panel", "SCALE_TYP": "-", "CLASS": "PANEL", "STATUS": "ACTIVE", "LONG_COMMON_NAME": "Test panel - version 1.0", "PanelType": "Convenience group"}),
		loincCSVRow(map[string]string{"LOINC_NUM": "40000-0", "COMPONENT": "Test score", "PROPERTY": "Score", "TIME_ASPCT": "Pt", "SYSTEM": "^Patient", "SCALE_TYP": "Qn", "CLASS": "PANEL", "STATUS": "ACTIVE", "LONG_COMMON_NAME": "Test panel T-score"}),
		loincCSVRow(map[string]string{"LOINC_NUM": "50000-3", "COMPONENT": "Test subgroup", "SCALE_TYP": "-", "CLASS": "PANEL", "STATUS": "ACTIVE", "LONG_COMMON_NAME": "Test subgroup"}),
		loincCSVRow(map[string]string{"LOINC_NUM": "60000-1", "COMPONENT": "Test plain item", "PROPERTY": "Find", "TIME_ASPCT": "Pt", "SYSTEM": "^Patient", "SCALE_TYP": "Ord", "CLASS": "PANEL", "STATUS": "ACTIVE", "LONG_COMMON_NAME": "Test plain item"}),
	})

	// A comment on the existing 20000-8 -> 10000-1 replacement, for the loinc-map-to $translate
	// comment-part test.
	writeCSV(t, filepath.Join(releaseDir, "LoincTable", "MapTo.csv"),
		[]string{"LOINC", "MAP_TO", "COMMENT"},
		[][]string{{"20000-8", "10000-1", "See replacement term 10000-1"}})

	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "LoincIeeeMedicalDeviceCodeMappingTable", "LoincIeeeMedicalDeviceCodeMappingTable.csv"),
		[]string{"LOINC_NUM", "LOINC_LONG_COMMON_NAME", "IEEE_CF_CODE10", "IEEE_REFID", "EQUIVALENCE"},
		[][]string{{"10000-1", "Cholesterol [Mass/volume] in Serum", "999001", "MDC_TEST_CHOL", "equivalent"}})

	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "LoincRsnaRadiologyPlaybook", "LoincRsnaRadiologyPlaybook.csv"),
		[]string{"LoincNumber", "LongCommonName", "PartNumber", "PartTypeName", "PartName", "PartSequenceOrder", "RID", "PreferredName", "RPID", "LongName"},
		[][]string{{"10000-1", "Cholesterol [Mass/volume] in Serum", "", "", "", "", "", "", "RPID9001", "Test RadLex Cholesterol"}})

	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "PartRelatedCodeMapping.csv"),
		[]string{"PartNumber", "PartName", "PartTypeName", "ExtCodeId", "ExtCodeDisplayName", "ExtCodeSystem", "Equivalence", "ContentOrigin", "ExtCodeSystemVersion", "ExtCodeSystemCopyrightNotice"},
		[][]string{
			{"LP1000-1", "Cholesterol", "COMPONENT", "123456", "Cholesterol (substance)", "http://snomed.info/sct", "equivalent", "LN", "", ""},
			{"LP1000-1", "Cholesterol", "COMPONENT", "RID999", "Cholesterol imaging finding", "http://www.radlex.org", "narrower", "LN", "", ""},
		})

	writeCSV(t, filepath.Join(releaseDir, "AccessoryFiles", "PanelsAndForms", "PanelsAndForms.csv"),
		[]string{"ParentId", "ParentLoinc", "ParentName", "ID", "SEQUENCE", "Loinc", "LoincName", "DisplayNameForForm", "ObservationRequiredInPanel", "ObservationIdInForm", "SkipLogicHelpText", "DefaultValue", "EntryType", "DataTypeInForm", "DataTypeSource", "AnswerSequenceOverride", "ConditionForInclusion", "AllowableAlternative", "ObservationCategory", "Context", "ConsistencyChecks", "RelevanceEquation", "CodingInstructions", "QuestionCardinality", "AnswerCardinality", "AnswerListIdOverride", "AnswerListTypeOverride", "EXTERNAL_COPYRIGHT_NOTICE", "AdditionalCopyright"},
		[][]string{
			{"P1", "30000-6", "Test panel", "P1", "0", "30000-6", "Test panel", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "Extra test copyright"},
			{"P1", "30000-6", "Test panel", "I1", "1", "10000-1", "Cholesterol", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "Extra test copyright"},
			{"P1", "30000-6", "Test panel", "I2", "2", "40000-0", "Test score", "T-score", "R", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "Extra test copyright"},
			{"P1", "30000-6", "Test panel", "I3", "3", "50000-3", "Test subgroup", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "Extra test copyright"},
			{"P1", "30000-6", "Test panel", "I4", "4", "60000-1", "Test plain item", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "LL1000-1", "", "", "Extra test copyright"},
			{"P2", "50000-3", "Test subgroup", "P2", "0", "50000-3", "Test subgroup", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
			{"P2", "50000-3", "Test subgroup", "I5", "1", "10000-1", "Cholesterol", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		})

	return releaseDir
}
