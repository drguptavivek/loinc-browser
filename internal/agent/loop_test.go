package agent

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"loinc-browser/internal/loinc"
)

// fakeLLM plays a scripted two-round conversation: first a tool call, then a final answer that
// mentions one real code (from the fixture, and returned by the tool) and one fake code that was
// never in any tool result.
type fakeLLM struct{ round int }

func (f *fakeLLM) ChatStream(ctx context.Context, req ChatRequest, onDelta func(kind, text string)) (FinalMessage, string, error) {
	f.round++
	if f.round == 1 {
		if onDelta != nil {
			onDelta("text", "")
		}
		return FinalMessage{ToolCalls: []ToolCall{{ID: "call_1", Name: "loinc_search_terms", Arguments: `{"query":"sodium"}`}}}, "tool_calls", nil
	}
	answer := "1000-1 — Glucose [Mass/volume] in Plasma — best match.\n99999-9 — not real — should be flagged."
	if onDelta != nil {
		onDelta("text", answer)
	}
	return FinalMessage{Content: answer}, "stop", nil
}

// fakeTools returns a canned search result mentioning the fixture's real code (1000-1) without
// running a real search.
type fakeTools struct{ calls []string }

func (f *fakeTools) Tools() []ToolDef {
	return []ToolDef{{Name: "loinc_search_terms", Description: "search"}}
}

func (f *fakeTools) CallTool(ctx context.Context, name string, argsJSON string) (string, error) {
	f.calls = append(f.calls, name)
	return `[{"loincNum":"1000-1","longCommonName":"Glucose [Mass/volume] in Plasma"}]`, nil
}

func TestRunEmitsEventsAndVerifiesCodesAgainstToolResults(t *testing.T) {
	store := newAgentFixtureStore(t)
	termLookup := func(ctx context.Context, loincNum string) (string, string, bool) {
		term, err := store.Term(ctx, loincNum)
		if err != nil {
			return "", "", false
		}
		return term.LongCommonName, term.Status, true
	}

	llm := &fakeLLM{}
	tools := &fakeTools{}
	var events []Event
	Run(context.Background(), Config{Client: llm, Model: "test-model", Tools: tools, TermLookup: termLookup}, []Message{{Role: "user", Content: "serum sodium"}}, false, func(e Event) {
		events = append(events, e)
	})

	var sawToolStart, sawToolEnd bool
	var codes []VerifiedCode
	var unverified []string
	var sawDone bool
	for _, e := range events {
		switch e.Type {
		case "tool-start":
			sawToolStart = true
		case "tool-end":
			sawToolEnd = true
		case "codes":
			codes = e.Data.(map[string]any)["codes"].([]VerifiedCode)
		case "unverified":
			unverified = e.Data.(map[string]any)["codes"].([]string)
		case "done":
			sawDone = true
		case "error":
			t.Fatalf("unexpected error event: %v", e.Data)
		}
	}
	if !sawToolStart || !sawToolEnd {
		t.Fatalf("expected tool-start and tool-end events, got %+v", events)
	}
	if !sawDone {
		t.Fatalf("expected a done event, got %+v", events)
	}
	if len(tools.calls) != 1 || tools.calls[0] != "loinc_search_terms" {
		t.Fatalf("tool calls = %v", tools.calls)
	}
	if len(codes) != 1 || codes[0].LOINCNum != "1000-1" {
		t.Fatalf("verified codes = %+v", codes)
	}
	if len(unverified) != 1 || unverified[0] != "99999-9" {
		t.Fatalf("unverified codes = %v", unverified)
	}
}

// newAgentFixtureStore ingests a minimal LOINC release (same shape as internal/mcpserver's test
// fixture) so the hallucination guard can verify a real code (1000-1) via Store.Term.
func newAgentFixtureStore(t *testing.T) *loinc.Store {
	t.Helper()
	releaseDir := t.TempDir()
	writeAgentTestRows(t, filepath.Join(releaseDir, "LoincTable", "Loinc.csv"), []string{
		"LOINC_NUM", "COMPONENT", "PROPERTY", "TIME_ASPCT", "SYSTEM", "SCALE_TYP", "METHOD_TYP", "CLASS",
		"VersionLastChanged", "CHNG_TYPE", "DefinitionDescription", "STATUS", "CONSUMER_NAME", "CLASSTYPE",
		"FORMULA", "EXMPL_ANSWERS", "SURVEY_QUEST_TEXT", "SURVEY_QUEST_SRC", "UNITSREQUIRED", "RELATEDNAMES2",
		"SHORTNAME", "ORDER_OBS", "HL7_FIELD_SUBFIELD_ID", "EXTERNAL_COPYRIGHT_NOTICE", "EXAMPLE_UNITS",
		"LONG_COMMON_NAME", "EXAMPLE_UCUM_UNITS", "STATUS_REASON", "STATUS_TEXT", "CHANGE_REASON_PUBLIC",
		"COMMON_TEST_RANK", "COMMON_ORDER_RANK", "HL7_ATTACHMENT_STRUCTURE", "EXTERNAL_COPYRIGHT_LINK",
		"PanelType", "AskAtOrderEntry", "AssociatedObservations", "VersionFirstReleased",
		"ValidHL7AttachmentRequest", "DisplayName",
	}, [][]string{
		{"1000-1", "Glucose", "MCnc", "Pt", "Plasma", "Qn", "", "CHEM", "2.80", "ADD", "Glucose mass concentration in plasma", "ACTIVE", "Blood sugar", "1", "", "", "", "", "N", "blood sugar; glucose plasma", "Glucose P", "Both", "", "", "mg/dL", "Glucose [Mass/volume] in Plasma", "mg/dL", "", "", "", "100", "100", "", "", "", "", "", "2.80", "", "Glucose Plasma"},
	})
	writeAgentTestRows(t, filepath.Join(releaseDir, "LoincTable", "MapTo.csv"), []string{"LOINC", "MAP_TO", "COMMENT"}, nil)
	writeAgentTestRows(t, filepath.Join(releaseDir, "LoincTable", "SourceOrganization.csv"), []string{"ID", "COPYRIGHT_ID", "NAME", "COPYRIGHT", "TERMS_OF_USE", "URL"}, nil)
	writeAgentTestRows(t, filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "Part.csv"), []string{"PartNumber", "PartTypeName", "PartName", "PartDisplayName", "Status"}, nil)
	writeAgentTestRows(t, filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "LoincPartLink_Primary.csv"), []string{"LoincNumber", "LongCommonName", "PartNumber", "PartName", "PartCodeSystem", "PartTypeName", "LinkTypeName", "Property"}, nil)
	writeAgentTestRows(t, filepath.Join(releaseDir, "AccessoryFiles", "PartFile", "LoincPartLink_Supplementary.csv"), []string{"LoincNumber", "LongCommonName", "PartNumber", "PartName", "PartCodeSystem", "PartTypeName", "LinkTypeName", "Property"}, nil)
	writeAgentTestRows(t, filepath.Join(releaseDir, "AccessoryFiles", "AnswerFile", "AnswerList.csv"), []string{"AnswerListId", "AnswerListName", "AnswerListOID", "ExtDefinedYN", "ExtDefinedAnswerListCodeSystem", "ExtDefinedAnswerListLink", "AnswerStringId", "LocalAnswerCode", "LocalAnswerCodeSystem", "SequenceNumber", "DisplayText", "ExtCodeId", "ExtCodeDisplayName", "ExtCodeSystem", "ExtCodeSystemVersion", "ExtCodeSystemCopyrightNotice", "SubsequentTextPrompt", "Description", "Score"}, nil)
	writeAgentTestRows(t, filepath.Join(releaseDir, "AccessoryFiles", "AnswerFile", "LoincAnswerListLink.csv"), []string{"LoincNumber", "LongCommonName", "AnswerListId", "AnswerListName", "AnswerListLinkType", "ApplicableContext"}, nil)
	writeAgentTestRows(t, filepath.Join(releaseDir, "AccessoryFiles", "PanelsAndForms", "PanelsAndForms.csv"), []string{"ParentId", "ParentLoinc", "ParentName", "ID", "SEQUENCE", "Loinc", "LoincName", "DisplayNameForForm", "ObservationRequiredInPanel", "ObservationIdInForm", "SkipLogicHelpText", "DefaultValue", "EntryType", "DataTypeInForm", "DataTypeSource", "AnswerSequenceOverride", "ConditionForInclusion", "AllowableAlternative", "ObservationCategory", "Context", "ConsistencyChecks", "RelevanceEquation", "CodingInstructions", "QuestionCardinality", "AnswerCardinality", "AnswerListIdOverride", "AnswerListTypeOverride", "EXTERNAL_COPYRIGHT_NOTICE", "AdditionalCopyright"}, nil)
	writeAgentTestRows(t, filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "ParentGroup.csv"), []string{"ParentGroupId", "ParentGroup", "Status"}, nil)
	writeAgentTestRows(t, filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "Group.csv"), []string{"ParentGroupId", "GroupId", "Group", "Archetype", "Status", "VersionFirstReleased"}, nil)
	writeAgentTestRows(t, filepath.Join(releaseDir, "AccessoryFiles", "GroupFile", "GroupLoincTerms.csv"), []string{"Category", "GroupId", "Archetype", "LoincNumber", "LongCommonName"}, nil)
	writeAgentTestRows(t, filepath.Join(releaseDir, "AccessoryFiles", "ComponentHierarchyBySystem", "ComponentHierarchyBySystem.csv"), []string{"PATH_TO_ROOT", "SEQUENCE", "IMMEDIATE_PARENT", "CODE", "CODE_TEXT"}, nil)

	dbPath := filepath.Join(t.TempDir(), "loinc.sqlite")
	if _, err := loinc.Ingest(context.Background(), loinc.IngestOptions{ReleaseDir: releaseDir, DBPath: dbPath}); err != nil {
		t.Fatalf("ingest test release: %v", err)
	}
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 8})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func writeAgentTestRows(t *testing.T, path string, header []string, rows [][]string) {
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
		t.Fatalf("write %s header: %v", path, err)
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			t.Fatalf("write %s row: %v", path, err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatalf("flush %s: %v", path, err)
	}
}
