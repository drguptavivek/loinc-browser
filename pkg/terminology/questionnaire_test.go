package terminology

import (
	"context"
	"testing"
)

func findItem(t *testing.T, items []QuestionnaireItem, linkID string) QuestionnaireItem {
	t.Helper()
	for _, item := range items {
		if item.LinkID == linkID {
			return item
		}
	}
	t.Fatalf("item %s not found in %+v", linkID, items)
	return QuestionnaireItem{}
}

func TestQuestionnaireItemsAndTypes(t *testing.T) {
	svc := newConceptMapTestService(t)
	ctx := context.Background()

	q, err := svc.Questionnaire(ctx, "30000-6")
	if err != nil {
		t.Fatalf("questionnaire: %v", err)
	}
	if q.ResourceType != "Questionnaire" || q.ID != "30000-6" || q.URL != loincSystem+"/q/30000-6" {
		t.Fatalf("resource = %+v", q)
	}
	if q.Status != "draft" || len(q.SubjectType) != 1 || q.SubjectType[0] != "Patient" {
		t.Fatalf("status/subjectType = %+v", q)
	}
	if len(q.Item) != 4 {
		t.Fatalf("item count = %d, want 4", len(q.Item))
	}

	choice := findItem(t, q.Item, "I1")
	if choice.Type != "choice" || len(choice.AnswerOption) != 2 {
		t.Fatalf("choice item = %+v", choice)
	}
	if choice.AnswerOption[0].ValueCoding.Code != "LA1-1" || choice.AnswerOption[0].ValueCoding.System != loincSystem {
		t.Fatalf("answerOption[0] = %+v", choice.AnswerOption[0])
	}

	decimal := findItem(t, q.Item, "I2")
	if decimal.Type != "decimal" {
		t.Fatalf("decimal item type = %q, want decimal", decimal.Type)
	}
	if !decimal.Required {
		t.Fatalf("decimal item required = false, want true (ObservationRequiredInPanel=R)")
	}

	group := findItem(t, q.Item, "I3")
	if group.Type != "group" {
		t.Fatalf("group item type = %q, want group", group.Type)
	}
	if len(group.Item) != 1 || group.Item[0].LinkID != "I5" || group.Item[0].Type != "choice" {
		t.Fatalf("nested group items = %+v", group.Item)
	}

	overridden := findItem(t, q.Item, "I4")
	if overridden.Type != "choice" || len(overridden.AnswerOption) != 2 {
		t.Fatalf("override item = %+v", overridden)
	}
}

func TestQuestionnaireCopyright(t *testing.T) {
	svc := newConceptMapTestService(t)
	q, err := svc.Questionnaire(context.Background(), "30000-6")
	if err != nil {
		t.Fatalf("questionnaire: %v", err)
	}
	want := loincCopyright + "\r\nExtra test copyright"
	if q.Copyright != want {
		t.Fatalf("copyright = %q, want %q", q.Copyright, want)
	}
}

func TestQuestionnaireNameDerivation(t *testing.T) {
	got := questionnaireName("PROMIS cancer item bank - physical function - version 1.1")
	want := "PROMIS_cancer_item_bank_physical_function_version"
	if got != want {
		t.Fatalf("questionnaireName = %q, want %q", got, want)
	}
}

func TestQuestionnaireNonPanel404(t *testing.T) {
	svc := newConceptMapTestService(t)
	_, err := svc.Questionnaire(context.Background(), "10000-1")
	if err == nil || err.Status != 404 {
		t.Fatalf("err = %+v, want 404", err)
	}
}

func TestQuestionnaireUnknownCode404(t *testing.T) {
	svc := newConceptMapTestService(t)
	_, err := svc.Questionnaire(context.Background(), "99999-9")
	if err == nil || err.Status != 404 {
		t.Fatalf("err = %+v, want 404", err)
	}
}

func TestSearchQuestionnaireByURL(t *testing.T) {
	svc := newConceptMapTestService(t)
	ctx := context.Background()

	found, err := svc.SearchQuestionnaire(ctx, loincSystem+"/q/30000-6")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if found.Total != 1 || len(found.Entry) != 1 {
		t.Fatalf("found = %+v", found)
	}

	empty, err := svc.SearchQuestionnaire(ctx, loincSystem+"/q/99999-9")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if empty.Total != 0 {
		t.Fatalf("empty = %+v", empty)
	}
}
