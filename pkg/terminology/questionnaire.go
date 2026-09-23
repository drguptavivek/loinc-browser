package terminology

import (
	"context"
	"strings"

	"loinc-browser/internal/loinc"
)

// QuestionnaireAnswerOption is one Questionnaire.item.answerOption entry.
type QuestionnaireAnswerOption struct {
	ValueCoding Coding `json:"valueCoding"`
}

// QuestionnaireItem is one Questionnaire.item entry, one panel_items row (§4.11). Item nests a
// group's own children when Type is "group".
type QuestionnaireItem struct {
	LinkID       string                      `json:"linkId"`
	Code         []Coding                    `json:"code,omitempty"`
	Text         string                      `json:"text,omitempty"`
	Type         string                      `json:"type"`
	Required     bool                        `json:"required,omitempty"`
	Repeats      bool                        `json:"repeats"`
	AnswerOption []QuestionnaireAnswerOption `json:"answerOption,omitempty"`
	Item         []QuestionnaireItem         `json:"item,omitempty"`
}

// Questionnaire is a served http://loinc.org/q/{LOINC} Questionnaire resource for a LOINC panel
// (§4.11).
type Questionnaire struct {
	ResourceType string              `json:"resourceType"`
	ID           string              `json:"id"`
	URL          string              `json:"url"`
	Version      string              `json:"version"`
	Name         string              `json:"name"`
	Title        string              `json:"title"`
	Status       string              `json:"status"`
	SubjectType  []string            `json:"subjectType"`
	Publisher    string              `json:"publisher"`
	Contact      []namedContact      `json:"contact,omitempty"`
	Description  string              `json:"description"`
	Copyright    string              `json:"copyright"`
	Code         []Coding            `json:"code"`
	Item         []QuestionnaireItem `json:"item,omitempty"`
}

// questionnaireName derives Questionnaire.name from the title (§4.11): every run of
// non-alphanumeric characters becomes one "_", the result is capped at 50 characters, and any
// trailing "_" left by the cap is trimmed. Verified against questionnaire-89689-4.json, whose
// title "PROMIS cancer item bank - physical function - version 1.1" (53 chars sanitized) yields
// name "PROMIS_cancer_item_bank_physical_function_version" (49 chars, the "_1_1" tail cut off by
// the cap).
func questionnaireName(title string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, r := range title {
		alnum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if alnum {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore && b.Len() > 0 {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	name := strings.TrimSuffix(b.String(), "_")
	if len(name) > 50 {
		name = strings.TrimSuffix(name[:50], "_")
	}
	return name
}

// questionnaireItemType classifies a panel child's item type (§4.11): choice wins over quantity
// when the child has an answer list even if it is also Qn-scaled; group wins when the child is
// itself a panel; Qn without an answer list is "decimal" (the exemplar's actual mapping — see
// docs/FHIR_TERMINOLOGY_PLAN.md §4.11 note); everything else is "string".
func questionnaireItemType(hasAnswerList bool, isGroup bool, scale string) string {
	switch {
	case hasAnswerList:
		return "choice"
	case isGroup:
		return "group"
	case strings.EqualFold(scale, "Qn"):
		return "decimal"
	default:
		return "string"
	}
}

// buildQuestionnaireItems turns a panel's flat panel_items rows into Questionnaire.item entries,
// resolving each choice item's answerOption list from either its AnswerListIdOverride or its
// child term's own linked answer list. Nested "group" panels (a child that is itself the parent
// of another panel_items block) recurse one level via FHIRPanelItems; deeper same-LOINC repeat
// groups (panel_items rows whose own container row was dropped at ingest because parent==child)
// are not reconstructed — ponytail: flattened into the parent's item list instead, add real
// recursive-group support if a served panel needs it.
func (s *Service) buildQuestionnaireItems(ctx context.Context, store *loinc.Store, parentLOINCNum string) ([]QuestionnaireItem, *OutcomeError) {
	rows, err := store.FHIRPanelItems(ctx, parentLOINCNum)
	if err != nil {
		return nil, &OutcomeError{Status: 503, Code: "exception", Text: err.Error()}
	}
	if len(rows) == 0 {
		return nil, nil
	}

	var toResolve []string
	for _, row := range rows {
		if row.AnswerListIDOverride == "" && !row.IsGroup {
			toResolve = append(toResolve, row.ChildLOINCNum)
		}
	}
	primaryLists, err := store.FHIRTermPrimaryAnswerLists(ctx, toResolve)
	if err != nil {
		return nil, &OutcomeError{Status: 503, Code: "exception", Text: err.Error()}
	}

	listIDSet := map[string]bool{}
	for _, row := range rows {
		if row.AnswerListIDOverride != "" {
			listIDSet[row.AnswerListIDOverride] = true
		} else if listID := primaryLists[row.ChildLOINCNum]; listID != "" {
			listIDSet[listID] = true
		}
	}
	listIDs := make([]string, 0, len(listIDSet))
	for id := range listIDSet {
		listIDs = append(listIDs, id)
	}
	answersByList, err := store.FHIRAnswerListAnswersBatch(ctx, listIDs)
	if err != nil {
		return nil, &OutcomeError{Status: 503, Code: "exception", Text: err.Error()}
	}

	items := make([]QuestionnaireItem, 0, len(rows))
	for _, row := range rows {
		listID := row.AnswerListIDOverride
		if listID == "" {
			listID = primaryLists[row.ChildLOINCNum]
		}
		answers := answersByList[listID]

		text := row.DisplayNameForForm
		if text == "" {
			text = row.ChildDisplay
		}
		item := QuestionnaireItem{
			LinkID:   row.ItemID,
			Code:     []Coding{loincCoding(row.ChildLOINCNum, row.ChildDisplay)},
			Text:     text,
			Type:     questionnaireItemType(len(answers) > 0, row.IsGroup, row.ChildScale),
			Required: row.Required,
			Repeats:  false,
		}
		for _, a := range answers {
			item.AnswerOption = append(item.AnswerOption, QuestionnaireAnswerOption{
				ValueCoding: loincCoding(a.AnswerStringID, a.DisplayText),
			})
		}
		if row.IsGroup {
			children, outcomeErr := s.buildQuestionnaireItems(ctx, store, row.ChildLOINCNum)
			if outcomeErr != nil {
				return nil, outcomeErr
			}
			item.Item = children
		}
		items = append(items, item)
	}
	return items, nil
}

// questionnaireIDFromURL extracts the LOINC number from a canonical "http://loinc.org/q/{LOINC}"
// url, or "" if url does not have that shape.
func questionnaireIDFromURL(url string) string {
	const prefix = loincSystem + "/q/"
	url = strings.TrimSpace(url)
	if strings.HasPrefix(url, prefix) {
		return strings.TrimPrefix(url, prefix)
	}
	return ""
}

// Questionnaire implements the Questionnaire read interaction for a LOINC panel (§4.11). A
// non-panel LOINC (no panel_items rows) returns 404.
func (s *Service) Questionnaire(ctx context.Context, loincNum string) (*Questionnaire, *OutcomeError) {
	store, outcomeErr := s.store()
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	version, outcomeErr := s.releaseVersion(ctx, store)
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	code := normalizeCode(loincNum)
	notFoundText := "Unable to find questionnaire for = " + code

	term, err := store.Term(ctx, code)
	if err != nil {
		return nil, notFoundError(notFoundText)
	}
	items, outcomeErr := s.buildQuestionnaireItems(ctx, store, code)
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	if len(items) == 0 {
		return nil, notFoundError(notFoundText)
	}

	copyright := loincCopyright
	if extra, ok, err := store.FHIRPanelAdditionalCopyright(ctx, code); err != nil {
		return nil, &OutcomeError{Status: 503, Code: "exception", Text: err.Error()}
	} else if ok {
		copyright = loincCopyright + "\r\n" + extra
	}

	title := term.LongCommonName
	return &Questionnaire{
		ResourceType: "Questionnaire",
		ID:           code,
		URL:          loincSystem + "/q/" + code,
		Version:      version,
		Name:         questionnaireName(title),
		Title:        title,
		Status:       "draft",
		SubjectType:  []string{"Patient"},
		Publisher:    loincPublisher,
		Contact:      loincContact(),
		Description:  title,
		Copyright:    copyright,
		Code:         []Coding{loincCoding(code, title)},
		Item:         items,
	}, nil
}

// SearchQuestionnaire implements Questionnaire search-type (§4.11):
// `?url=http://loinc.org/q/{LOINC}`. An unrecognized url or a non-panel LOINC returns an empty
// Bundle rather than an error, matching the CodeSystem/ValueSet search convention.
func (s *Service) SearchQuestionnaire(ctx context.Context, url string) (*Bundle, *OutcomeError) {
	bundle := &Bundle{ResourceType: "Bundle", Type: "searchset"}
	code := questionnaireIDFromURL(url)
	if code == "" {
		return bundle, nil
	}
	resource, outcomeErr := s.Questionnaire(ctx, code)
	if outcomeErr != nil {
		return bundle, nil
	}
	bundle.Total = 1
	bundle.Entry = []BundleEntry{{Resource: resource}}
	return bundle, nil
}
