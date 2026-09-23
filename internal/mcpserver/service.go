package mcpserver

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"

	"loinc-browser/internal/loinc"
	"loinc-browser/pkg/terminology"
)

const (
	defaultLimit = 10
	maxLimit     = 50
)

// LuceneSearchFunc runs one page of a Bleve local-search query (the same index /searchapi and
// /api/v1/local-search/query use) for loinc_lucene_search. It is nil when the caller (the stdio
// mcp command, when no index path is available) has nothing to wire it to; the tool then reports
// "not available" rather than a transport error.
type LuceneSearchFunc func(ctx context.Context, scope, query string, rows, offset int) ([]loinc.LocalSearchResult, uint64, error)

// Service backs every MCP tool. getStore is resolved fresh on every call (never a captured
// *loinc.Store) so the HTTP server's store hot-swap after an upload import is picked up per
// request, matching pkg/terminology.Service's own convention.
type Service struct {
	getStore     func() (*loinc.Store, error)
	docs         *Docs
	terminology  *terminology.Service
	luceneSearch LuceneSearchFunc
}

// store resolves the current store, or a plain error suitable for an MCP tool error result.
func (s *Service) store() (*loinc.Store, error) {
	store, err := s.getStore()
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, errors.New("LOINC database is not loaded")
	}
	return store, nil
}

type SearchTermsRequest struct {
	Query           string   `json:"q,omitempty" jsonschema:"Search query or exact LOINC number"`
	Status          string   `json:"status,omitempty" jsonschema:"Status filter. Defaults to every status except DEPRECATED. Use DEPRECATED or * only when needed."`
	Statuses        []string `json:"statuses,omitempty" jsonschema:"Repeatable status filters"`
	UsageType       string   `json:"usageType,omitempty" jsonschema:"any, observation, or order"`
	RankMode        string   `json:"rankMode,omitempty" jsonschema:"observation or order"`
	Sort            string   `json:"sort,omitempty" jsonschema:"relevance, usage, or alpha"`
	RankedOnly      bool     `json:"rankedOnly,omitempty" jsonschema:"Require positive common rank"`
	Class           string   `json:"class,omitempty" jsonschema:"LOINC class filter"`
	System          string   `json:"system,omitempty" jsonschema:"System axis filter"`
	TimeAspect      string   `json:"timeAspect,omitempty" jsonschema:"Time aspect filter"`
	Scale           string   `json:"scale,omitempty" jsonschema:"Scale filter"`
	Method          string   `json:"method,omitempty" jsonschema:"Method filter"`
	Property        string   `json:"property,omitempty" jsonschema:"Property filter"`
	OrderObs        string   `json:"orderObs,omitempty" jsonschema:"Raw ORDER_OBS filter"`
	HierarchyNodeID string   `json:"hierarchyNodeId,omitempty" jsonschema:"Hierarchy occurrence node ID"`
	Limit           int      `json:"limit,omitempty" jsonschema:"Maximum rows, capped for context control"`
	Offset          int      `json:"offset,omitempty" jsonschema:"Result offset"`
	Detail          string   `json:"detail,omitempty" jsonschema:"summary, standard, or full"`
}

type LOINCRequest struct {
	LOINCNum string   `json:"loincNum" jsonschema:"LOINC number"`
	Limit    int      `json:"limit,omitempty" jsonschema:"Maximum rows, capped for context control"`
	Offset   int      `json:"offset,omitempty" jsonschema:"Result offset"`
	Detail   string   `json:"detail,omitempty" jsonschema:"summary, standard, or full"`
	Include  []string `json:"include,omitempty" jsonschema:"Optional related sections to include"`
}

type QueryPageRequest struct {
	Query  string `json:"q,omitempty" jsonschema:"Search query"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Maximum rows, capped for context control"`
	Offset int    `json:"offset,omitempty" jsonschema:"Result offset"`
	Detail string `json:"detail,omitempty" jsonschema:"summary, standard, or full"`
}

type AnswerListRequest struct {
	AnswerListID string `json:"answerListId" jsonschema:"Answer list ID"`
	Limit        int    `json:"limit,omitempty" jsonschema:"Maximum rows, capped for context control"`
	Offset       int    `json:"offset,omitempty" jsonschema:"Result offset"`
	Detail       string `json:"detail,omitempty" jsonschema:"summary, standard, or full"`
}

type HierarchyRequest struct {
	NodeID string `json:"nodeId,omitempty" jsonschema:"Hierarchy occurrence node ID. Omit for roots."`
	Query  string `json:"q,omitempty" jsonschema:"Hierarchy text query"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Maximum rows, capped for context control"`
	Offset int    `json:"offset,omitempty" jsonschema:"Result offset"`
	Detail string `json:"detail,omitempty" jsonschema:"summary, standard, or full"`
}

type HierarchyTermsRequest struct {
	NodeID     string `json:"nodeId" jsonschema:"Hierarchy occurrence node ID"`
	Query      string `json:"q,omitempty" jsonschema:"Optional term search query under the hierarchy node"`
	Status     string `json:"status,omitempty" jsonschema:"Status filter. Use DEPRECATED or * only when needed."`
	UsageType  string `json:"usageType,omitempty" jsonschema:"any, observation, or order"`
	RankMode   string `json:"rankMode,omitempty" jsonschema:"observation or order"`
	Sort       string `json:"sort,omitempty" jsonschema:"relevance, usage, or alpha"`
	RankedOnly bool   `json:"rankedOnly,omitempty" jsonschema:"Require positive common rank"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Maximum rows, capped for context control"`
	Offset     int    `json:"offset,omitempty" jsonschema:"Result offset"`
	Detail     string `json:"detail,omitempty" jsonschema:"summary, standard, or full"`
}

type PageResponse[T any] struct {
	Results       []T    `json:"results"`
	Total         int    `json:"total"`
	Limit         int    `json:"limit"`
	Offset        int    `json:"offset"`
	HasMore       bool   `json:"hasMore"`
	NextCallHint  string `json:"nextCallHint,omitempty"`
	ContextHint   string `json:"contextHint,omitempty"`
	RequestedFull bool   `json:"requestedFull,omitempty"`
	// Relaxed is true when no result matched every query word; DroppedWords were left out.
	Relaxed      bool     `json:"relaxed,omitempty"`
	DroppedWords []string `json:"droppedWords,omitempty"`
}

type TermCandidate struct {
	LOINCNum        string   `json:"loincNum"`
	DisplayName     string   `json:"displayName"`
	LongCommonName  string   `json:"longCommonName,omitempty"`
	Status          string   `json:"status"`
	UsageTypes      []string `json:"usageTypes,omitempty"`
	CommonTestRank  int      `json:"commonTestRank,omitempty"`
	CommonOrderRank int      `json:"commonOrderRank,omitempty"`
	// Relevance is the text-match strength for the query (higher is better; 0 without query
	// text). Compare it within one result set only; sort=relevance orders by it.
	Relevance float64           `json:"relevance,omitempty"`
	System    string            `json:"system,omitempty"`
	Class     string            `json:"class,omitempty"`
	Scale     string            `json:"scale,omitempty"`
	Property  string            `json:"property,omitempty"`
	Notes     []string          `json:"notes,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
}

type TermFitResponse struct {
	loinc.TermFit
	Notes []string `json:"notes,omitempty"`
}

// NewService builds a Service around a store getter (e.g. the server's currentStore, or a stdio
// command's fixed store), the editable-docs reader, and optionally a terminology.Service and a
// LuceneSearchFunc for the FHIR/Search API-backed tools (both may be nil; those tools then report
// a clear "not available" error instead of panicking).
func NewService(getStore func() (*loinc.Store, error), docs *Docs, terminologySvc *terminology.Service, luceneSearch LuceneSearchFunc) *Service {
	return &Service{getStore: getStore, docs: docs, terminology: terminologySvc, luceneSearch: luceneSearch}
}

func (s *Service) SearchTerms(ctx context.Context, req SearchTermsRequest) (PageResponse[TermCandidate], error) {
	store, err := s.store()
	if err != nil {
		return PageResponse[TermCandidate]{}, err
	}
	limit := normalizeMCPLimit(req.Limit)
	offset := normalizeOffset(req.Offset)
	params := req.searchParams(limit, offset)
	response, err := store.Search(ctx, params)
	if err != nil {
		return PageResponse[TermCandidate]{}, err
	}
	return PageResponse[TermCandidate]{
		Results:       compactTerms(response.Results, req.Detail),
		Total:         response.Total,
		Limit:         limit,
		Offset:        offset,
		HasMore:       response.HasMore,
		Relaxed:       response.Relaxed,
		DroppedWords:  response.DroppedWords,
		NextCallHint:  nextCallHint("loinc_search_terms", response.HasMore, limit, offset),
		ContextHint:   firstNonEmpty(response.Notice, "Compact term candidates. Call loinc_get_term_fit before recommending a term."),
		RequestedFull: req.Detail == "full",
	}, nil
}

func (s *Service) GetTerm(ctx context.Context, req LOINCRequest) (loinc.Term, error) {
	store, err := s.store()
	if err != nil {
		return loinc.Term{}, err
	}
	return store.Term(ctx, req.LOINCNum)
}

func (s *Service) GetTermFit(ctx context.Context, req LOINCRequest) (TermFitResponse, error) {
	store, err := s.store()
	if err != nil {
		return TermFitResponse{}, err
	}
	fit, err := store.TermFit(ctx, req.LOINCNum)
	if err != nil {
		return TermFitResponse{}, err
	}
	return TermFitResponse{TermFit: fit, Notes: fitNotes(fit)}, nil
}

func (s *Service) GetTermRelationships(ctx context.Context, req LOINCRequest) (loinc.TermRelationshipGroups, error) {
	store, err := s.store()
	if err != nil {
		return loinc.TermRelationshipGroups{}, err
	}
	return store.TermRelationshipGroups(ctx, req.LOINCNum)
}

func (s *Service) SearchPanels(ctx context.Context, req SearchTermsRequest) (PageResponse[TermCandidate], error) {
	store, err := s.store()
	if err != nil {
		return PageResponse[TermCandidate]{}, err
	}
	limit := normalizeMCPLimit(req.Limit)
	offset := normalizeOffset(req.Offset)
	params := req.searchParams(limit, offset)
	response, err := store.SearchPanels(ctx, params)
	if err != nil {
		return PageResponse[TermCandidate]{}, err
	}
	return PageResponse[TermCandidate]{
		Results:      compactTerms(response.Results, req.Detail),
		Total:        response.Total,
		Limit:        limit,
		Offset:       offset,
		HasMore:      response.HasMore,
		NextCallHint: nextCallHint("loinc_search_panels", response.HasMore, limit, offset),
		ContextHint:  "Panel candidates. Call loinc_get_panel_items for authored form structure.",
	}, nil
}

func (s *Service) GetPanelItems(ctx context.Context, req LOINCRequest) (PageResponse[loinc.PanelItem], error) {
	store, err := s.store()
	if err != nil {
		return PageResponse[loinc.PanelItem]{}, err
	}
	limit := normalizeMCPLimit(req.Limit)
	offset := normalizeOffset(req.Offset)
	page, err := store.PanelItems(ctx, req.LOINCNum, limit, offset)
	if err != nil {
		return PageResponse[loinc.PanelItem]{}, err
	}
	return PageResponse[loinc.PanelItem]{
		Results:      page.Results,
		Total:        page.Total,
		Limit:        limit,
		Offset:       offset,
		HasMore:      page.HasMore,
		NextCallHint: nextCallHint("loinc_get_panel_items", page.HasMore, limit, offset),
		ContextHint:  "Authored panel item order is significant. Preserve sequence values.",
	}, nil
}

func (s *Service) SearchAnswerLists(ctx context.Context, req QueryPageRequest) (PageResponse[loinc.AnswerList], error) {
	store, err := s.store()
	if err != nil {
		return PageResponse[loinc.AnswerList]{}, err
	}
	limit := normalizeMCPLimit(req.Limit)
	offset := normalizeOffset(req.Offset)
	page, err := store.SearchAnswerLists(ctx, req.Query, limit, offset)
	if err != nil {
		return PageResponse[loinc.AnswerList]{}, err
	}
	return fromLOINCPage(page, "loinc_search_answer_lists", "Answer list IDs can be followed with loinc_get_answer_list_answers."), nil
}

func (s *Service) GetAnswerListAnswers(ctx context.Context, req AnswerListRequest) (PageResponse[loinc.AnswerListAnswer], error) {
	store, err := s.store()
	if err != nil {
		return PageResponse[loinc.AnswerListAnswer]{}, err
	}
	limit := normalizeMCPLimit(req.Limit)
	offset := normalizeOffset(req.Offset)
	page, err := store.AnswerListAnswers(ctx, req.AnswerListID, limit, offset)
	if err != nil {
		return PageResponse[loinc.AnswerListAnswer]{}, err
	}
	return fromLOINCPage(page, "loinc_get_answer_list_answers", "Answer sequence is significant for forms."), nil
}

func (s *Service) BrowseHierarchy(ctx context.Context, req HierarchyRequest) (PageResponse[loinc.HierarchyNode], error) {
	store, err := s.store()
	if err != nil {
		return PageResponse[loinc.HierarchyNode]{}, err
	}
	limit := normalizeMCPLimit(req.Limit)
	offset := normalizeOffset(req.Offset)
	response, err := store.HierarchyChildren(ctx, req.NodeID, req.Query, false)
	if err != nil {
		return PageResponse[loinc.HierarchyNode]{}, err
	}
	total := len(response.Results)
	end := offset + limit
	if end > total {
		end = total
	}
	results := []loinc.HierarchyNode{}
	if offset < total {
		results = response.Results[offset:end]
	}
	return PageResponse[loinc.HierarchyNode]{
		Results:      results,
		Total:        total,
		Limit:        limit,
		Offset:       offset,
		HasMore:      end < total,
		NextCallHint: nextCallHint("loinc_browse_hierarchy", end < total, limit, offset),
		ContextHint:  "Use hierarchy nodeId values for follow-up calls, not concept codes.",
	}, nil
}

func (s *Service) GetHierarchyTerms(ctx context.Context, req HierarchyTermsRequest) (PageResponse[TermCandidate], error) {
	return s.SearchTerms(ctx, SearchTermsRequest{
		Query:           req.Query,
		Status:          req.Status,
		UsageType:       req.UsageType,
		RankMode:        req.RankMode,
		Sort:            req.Sort,
		RankedOnly:      req.RankedOnly,
		HierarchyNodeID: req.NodeID,
		Limit:           req.Limit,
		Offset:          req.Offset,
		Detail:          req.Detail,
	})
}

func (s *Service) SearchParts(ctx context.Context, req QueryPageRequest) (PageResponse[loinc.Part], error) {
	store, err := s.store()
	if err != nil {
		return PageResponse[loinc.Part]{}, err
	}
	limit := normalizeMCPLimit(req.Limit)
	offset := normalizeOffset(req.Offset)
	page, err := store.SearchParts(ctx, req.Query, limit, offset)
	if err != nil {
		return PageResponse[loinc.Part]{}, err
	}
	return fromLOINCPage(page, "loinc_search_parts", "Parts help broaden or constrain term searches."), nil
}

func (s *Service) SearchGroups(ctx context.Context, req QueryPageRequest) (PageResponse[loinc.LOINCGroup], error) {
	store, err := s.store()
	if err != nil {
		return PageResponse[loinc.LOINCGroup]{}, err
	}
	limit := normalizeMCPLimit(req.Limit)
	offset := normalizeOffset(req.Offset)
	page, err := store.SearchGroups(ctx, req.Query, limit, offset)
	if err != nil {
		return PageResponse[loinc.LOINCGroup]{}, err
	}
	return fromLOINCPage(page, "loinc_search_groups", "Groups are useful for comparison, but validate final terms with fit metadata."), nil
}

func (s *Service) ExplainConcept(ctx context.Context, req ConceptRequest) (TextResponse, error) {
	return s.docs.ExplainConcept(ctx, req)
}

func (r SearchTermsRequest) searchParams(limit int, offset int) loinc.SearchParams {
	statuses := r.Statuses
	if r.Status != "" {
		statuses = append(statuses, r.Status)
	}
	return loinc.SearchParams{
		Query:           r.Query,
		Statuses:        statuses,
		UsageType:       r.UsageType,
		RankMode:        r.RankMode,
		Sort:            r.Sort,
		RankedOnly:      r.RankedOnly,
		Class:           r.Class,
		System:          r.System,
		TimeAspect:      r.TimeAspect,
		Scale:           r.Scale,
		Method:          r.Method,
		Property:        r.Property,
		OrderObs:        r.OrderObs,
		HierarchyNodeID: r.HierarchyNodeID,
		Limit:           limit,
		Offset:          offset,
	}
}

func compactTerms(results []loinc.SearchResult, detail string) []TermCandidate {
	items := make([]TermCandidate, 0, len(results))
	for _, result := range results {
		item := TermCandidate{
			LOINCNum:        result.LOINCNum,
			DisplayName:     firstNonEmpty(result.ShortName, result.LongCommonName),
			Status:          result.Status,
			UsageTypes:      result.UsageTypes,
			CommonTestRank:  result.CommonTestRank,
			CommonOrderRank: result.CommonOrderRank,
			Relevance:       math.Round(-result.Rank*1000) / 1000,
			System:          result.System,
			Class:           result.Class,
			Scale:           result.Scale,
			Property:        result.Property,
			Notes:           statusNotes(result.Status),
		}
		if detail == "standard" || detail == "full" {
			item.LongCommonName = result.LongCommonName
		}
		items = append(items, item)
	}
	return items
}

func fromLOINCPage[T any](page loinc.Page[T], toolName string, hint string) PageResponse[T] {
	return PageResponse[T]{
		Results:      page.Results,
		Total:        page.Total,
		Limit:        page.Limit,
		Offset:       page.Offset,
		HasMore:      page.HasMore,
		NextCallHint: nextCallHint(toolName, page.HasMore, page.Limit, page.Offset),
		ContextHint:  hint,
	}
}

func normalizeMCPLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func nextCallHint(toolName string, hasMore bool, limit int, offset int) string {
	if !hasMore {
		return ""
	}
	return toolName + " with offset " + strconv.Itoa(offset+limit)
}

func fitNotes(fit loinc.TermFit) []string {
	var notes []string
	notes = append(notes, statusNotes(fit.Status)...)
	if fit.HasAnswerLists {
		notes = append(notes, "Term has linked answer lists; inspect answer choices for structured forms.")
	}
	if fit.HasPanelItems {
		notes = append(notes, "Term is a panel/form parent; inspect panel items before using it as a single field.")
	}
	if fit.HasPanelMemberships {
		notes = append(notes, "Term appears inside one or more panels.")
	}
	return notes
}

func statusNotes(status string) []string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "DEPRECATED":
		return []string{"Deprecated term; use only for legacy mapping or when explicitly requested."}
	case "DISCOURAGED":
		return []string{"Discouraged term; prefer an active alternative when possible."}
	case "INACTIVE":
		return []string{"Inactive term; do not recommend unless explicitly requested."}
	default:
		return nil
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
