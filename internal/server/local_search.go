package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"

	"loinc-browser/internal/loinc"
	loincmcp "loinc-browser/internal/mcpserver"
)

const defaultLocalSearchIndexPath = "./data/loinc-search.bleve"

// localSearchBuiltAtKey is written as the last step of a successful build, so an index without it
// was interrupted mid-build.
const localSearchBuiltAtKey = "loinc-browser-local-search-built-at"

type localSearchService struct {
	path     string
	mu       sync.Mutex
	building atomic.Bool
}

type LocalSearchStatus struct {
	State         string            `json:"state"`
	IndexPath     string            `json:"indexPath"`
	DocCount      uint64            `json:"docCount"`
	UpdatedAt     string            `json:"updatedAt,omitempty"`
	FieldCoverage map[string]string `json:"fieldCoverage,omitempty"`
	Warnings      []string          `json:"warnings,omitempty"`
	Message       string            `json:"message,omitempty"`
	// Building is true while a rebuild runs; the previous index (if any) keeps serving until the
	// new one is swapped in.
	Building bool `json:"building,omitempty"`
}

type LocalSearchRequest struct {
	Scope  string `json:"scope"`
	Query  string `json:"query"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
	// MaxLimit overrides the default 100-row cap (0 keeps the default). The
	// Search API clone (searchapi.go) sets this to 500 per plan §5.
	MaxLimit int `json:"maxLimit,omitempty"`
	// SortBy is a bleve sort spec ("Field" or "-Field" for descending),
	// resolved by the caller via localSearchFieldAliases. Empty keeps the
	// default relevance order.
	SortBy string `json:"sortBy,omitempty"`
}

type LocalSearchResponse struct {
	Scope       string                    `json:"scope"`
	Query       string                    `json:"query"`
	Results     []loinc.LocalSearchResult `json:"results"`
	Total       uint64                    `json:"total"`
	Limit       int                       `json:"limit"`
	Offset      int                       `json:"offset"`
	Warnings    []string                  `json:"warnings,omitempty"`
	IndexStatus string                    `json:"indexStatus"`
}

func newLocalSearchService(path string) *localSearchService {
	path = strings.TrimSpace(path)
	if path == "" {
		path = defaultLocalSearchIndexPath
	}
	return &localSearchService{path: path}
}

// NewLuceneSearchFunc builds the mcpserver.LuceneSearchFunc that backs the loinc_lucene_search MCP
// tool, querying the local Bleve index at indexPath through getStore's current *loinc.Store. Both
// the HTTP server (server.New) and the stdio command (cmd/loinc-browser's `mcp` subcommand) share
// this one implementation so a missing/unbuilt index reports the same clear error on either
// transport.
func NewLuceneSearchFunc(indexPath string, getStore func() (*loinc.Store, error)) loincmcp.LuceneSearchFunc {
	localSearch := newLocalSearchService(indexPath)
	return func(ctx context.Context, scope, queryText string, rows, offset int) ([]loinc.LocalSearchResult, uint64, error) {
		store, err := getStore()
		if err != nil {
			return nil, 0, err
		}
		response, status, err := localSearch.query(ctx, store, LocalSearchRequest{
			Scope: scope, Query: queryText, Limit: rows, Offset: offset,
		})
		if err != nil {
			if status == http.StatusServiceUnavailable {
				return nil, 0, errors.New(searchAPIMissingIndex)
			}
			return nil, 0, err
		}
		return response.Results, response.Total, nil
	}
}

func (s *localSearchService) status(ctx context.Context, store *loinc.Store) LocalSearchStatus {
	status := LocalSearchStatus{
		State:         "missing",
		IndexPath:     s.path,
		FieldCoverage: localSearchFieldCoverage(),
	}
	status.Building = s.building.Load()
	if store == nil {
		status.State = "requires_reingest"
		status.Message = "local LOINC database is not loaded"
		return status
	}
	info, err := os.Stat(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			status.Message = "local Lucene index has not been built"
			return status
		}
		status.State = "error"
		status.Message = "local Lucene index status unavailable"
		return status
	}
	status.UpdatedAt = info.ModTime().Format(time.RFC3339)
	index, err := bleve.Open(s.path)
	if err != nil {
		status.State = "error"
		status.Message = "local Lucene index cannot be opened; rebuild it"
		return status
	}
	defer index.Close()
	count, err := index.DocCount()
	if err != nil {
		status.State = "error"
		status.Message = "local Lucene index document count unavailable"
		return status
	}
	status.DocCount = count
	if count == 0 {
		status.State = "missing"
		status.Message = "local Lucene index is empty; rebuild it"
		return status
	}
	builtAt, ok := indexBuiltAt(index)
	if !ok {
		status.State = "incomplete"
		status.Message = "local Lucene index build did not finish; rebuild it"
		return status
	}
	status.UpdatedAt = builtAt.Format(time.RFC3339)
	if importedAt, err := store.ImportedAt(ctx); err == nil && builtAt.Before(importedAt) {
		status.State = "stale"
		status.Message = "local Lucene index predates the current LOINC import; rebuild it"
		return status
	}
	status.State = "ready"
	status.Message = "local Lucene index is ready"
	if status.Building {
		status.Message = "local Lucene index is ready; a rebuild is in progress"
	}
	status.Warnings = localSearchCoverageWarnings()
	return status
}

func indexBuiltAt(index bleve.Index) (time.Time, bool) {
	raw, err := index.GetInternal([]byte(localSearchBuiltAtKey))
	if err != nil || len(raw) == 0 {
		return time.Time{}, false
	}
	builtAt, err := time.Parse(time.RFC3339, string(raw))
	return builtAt, err == nil
}

func (s *localSearchService) rebuild(ctx context.Context, store *loinc.Store) (LocalSearchStatus, error) {
	if store == nil {
		return LocalSearchStatus{State: "requires_reingest", IndexPath: s.path, Message: "local LOINC database is not loaded"}, errors.New("LOINC database is not loaded")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.building.Store(true)
	defer s.building.Store(false)
	// Build beside the live index and swap it in at the end, so the old index keeps serving during
	// the build and an interrupted build never replaces it.
	buildPath := s.path + ".building"
	if err := os.RemoveAll(buildPath); err != nil {
		return LocalSearchStatus{}, fmt.Errorf("remove leftover local Lucene build: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return LocalSearchStatus{}, fmt.Errorf("create local Lucene index directory: %w", err)
	}
	mapping := bleve.NewIndexMapping()
	mapping.DefaultField = "_all"
	index, err := bleve.New(buildPath, mapping)
	if err != nil {
		return LocalSearchStatus{}, fmt.Errorf("create local Lucene index: %w", err)
	}

	batch := index.NewBatch()
	count := 0
	if err := store.VisitLocalSearchDocuments(ctx, func(doc loinc.LocalSearchDocument) error {
		if err := batch.Index(doc.ID, doc.Fields); err != nil {
			return err
		}
		count++
		if batch.Size() >= 500 {
			if err := index.Batch(batch); err != nil {
				return err
			}
			batch = index.NewBatch()
		}
		return nil
	}); err != nil {
		_ = index.Close()
		return LocalSearchStatus{}, fmt.Errorf("index local Lucene documents: %w", err)
	}
	if batch.Size() > 0 {
		if err := index.Batch(batch); err != nil {
			_ = index.Close()
			return LocalSearchStatus{}, fmt.Errorf("commit local Lucene index batch: %w", err)
		}
	}
	if err := index.SetInternal([]byte(localSearchBuiltAtKey), []byte(time.Now().Format(time.RFC3339))); err != nil {
		_ = index.Close()
		return LocalSearchStatus{}, fmt.Errorf("write local Lucene index metadata: %w", err)
	}
	if err := index.Close(); err != nil {
		return LocalSearchStatus{}, fmt.Errorf("close local Lucene index: %w", err)
	}
	previousPath := s.path + ".previous"
	_ = os.RemoveAll(previousPath)
	if err := os.Rename(s.path, previousPath); err != nil && !os.IsNotExist(err) {
		return LocalSearchStatus{}, fmt.Errorf("move previous local Lucene index aside: %w", err)
	}
	if err := os.Rename(buildPath, s.path); err != nil {
		return LocalSearchStatus{}, fmt.Errorf("install rebuilt local Lucene index: %w", err)
	}
	_ = os.RemoveAll(previousPath)
	s.building.Store(false)
	status := s.status(ctx, store)
	status.DocCount = uint64(count)
	status.State = "ready"
	status.Message = "local Lucene index rebuilt"
	return status, nil
}

func (s *localSearchService) query(ctx context.Context, store *loinc.Store, request LocalSearchRequest) (LocalSearchResponse, int, error) {
	if store == nil {
		return LocalSearchResponse{}, http.StatusServiceUnavailable, errors.New("LOINC database is not loaded")
	}
	scope, ok := normalizeLocalSearchScope(request.Scope)
	if !ok {
		return LocalSearchResponse{}, http.StatusBadRequest, fmt.Errorf("unsupported local search scope %q", request.Scope)
	}
	limit := request.Limit
	if limit <= 0 {
		limit = 25
	}
	maxLimit := request.MaxLimit
	if maxLimit <= 0 {
		maxLimit = 100
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	offset := request.Offset
	if offset < 0 {
		offset = 0
	}
	index, err := bleve.Open(s.path)
	if err != nil {
		return LocalSearchResponse{}, http.StatusServiceUnavailable, errors.New("local Lucene index is not ready; rebuild it")
	}
	defer index.Close()
	if _, ok := indexBuiltAt(index); !ok {
		return LocalSearchResponse{}, http.StatusServiceUnavailable, errors.New("local Lucene index build did not finish; rebuild it")
	}

	queryText, warnings := rewriteLocalSearchQuery(scope, request.Query)
	var userQuery query.Query
	if strings.TrimSpace(queryText) == "" {
		userQuery = bleve.NewMatchAllQuery()
	} else {
		parsed, err := parseLocalLuceneQuery(queryText)
		if err != nil {
			return LocalSearchResponse{}, http.StatusBadRequest, fmt.Errorf("local advanced search query failed: %w", err)
		}
		userQuery = parsed
	}
	scopeQuery := bleve.NewTermQuery(scope)
	scopeQuery.SetField("scope")
	booleanQuery := bleve.NewBooleanQuery()
	booleanQuery.AddMust(scopeQuery)
	booleanQuery.AddMust(userQuery)
	searchRequest := bleve.NewSearchRequestOptions(booleanQuery, limit, offset, false)
	searchRequest.Fields = []string{"scope", "key"}
	if request.SortBy != "" {
		searchRequest.SortBy([]string{request.SortBy})
	}
	searchResult, err := index.SearchInContext(ctx, searchRequest)
	if err != nil {
		return LocalSearchResponse{}, http.StatusBadRequest, fmt.Errorf("local Lucene query failed: %w", err)
	}
	hits := make([]loinc.LocalSearchHit, 0, len(searchResult.Hits))
	for _, hit := range searchResult.Hits {
		hitScope, key := loinc.ParseLocalSearchDocID(hit.ID)
		if key == "" {
			continue
		}
		hits = append(hits, loinc.LocalSearchHit{ID: hit.ID, Scope: hitScope, Key: key, Score: hit.Score})
	}
	results, err := store.HydrateLocalSearchHits(ctx, hits)
	if err != nil {
		return LocalSearchResponse{}, http.StatusInternalServerError, err
	}
	return LocalSearchResponse{
		Scope:       scope,
		Query:       strings.TrimSpace(request.Query),
		Results:     results,
		Total:       searchResult.Total,
		Limit:       limit,
		Offset:      offset,
		Warnings:    warnings,
		IndexStatus: "ready",
	}, http.StatusOK, nil
}

// matchingKeys returns every key matching scope+queryText (up to limit), plus
// the true total hit count, so a caller can facet over the whole result set
// rather than just one page.
// ponytail: capped at limit docs scanned into Go; if the local catalogue's
// matched sets routinely exceed it, move faceting into bleve's native facet
// API instead of pulling every key.
func (s *localSearchService) matchingKeys(ctx context.Context, scope string, queryText string, limit int) ([]string, uint64, error) {
	index, err := bleve.Open(s.path)
	if err != nil {
		return nil, 0, errors.New("local Lucene index is not ready; rebuild it")
	}
	defer index.Close()
	if _, ok := indexBuiltAt(index); !ok {
		return nil, 0, errors.New("local Lucene index build did not finish; rebuild it")
	}
	rewritten, _ := rewriteLocalSearchQuery(scope, queryText)
	var userQuery query.Query
	if strings.TrimSpace(rewritten) == "" {
		userQuery = bleve.NewMatchAllQuery()
	} else {
		parsed, err := parseLocalLuceneQuery(rewritten)
		if err != nil {
			return nil, 0, fmt.Errorf("local advanced search query failed: %w", err)
		}
		userQuery = parsed
	}
	scopeQuery := bleve.NewTermQuery(scope)
	scopeQuery.SetField("scope")
	booleanQuery := bleve.NewBooleanQuery()
	booleanQuery.AddMust(scopeQuery)
	booleanQuery.AddMust(userQuery)
	searchRequest := bleve.NewSearchRequestOptions(booleanQuery, limit, 0, false)
	searchRequest.Fields = []string{"key"}
	result, err := index.SearchInContext(ctx, searchRequest)
	if err != nil {
		return nil, 0, fmt.Errorf("local Lucene query failed: %w", err)
	}
	keys := make([]string, 0, len(result.Hits))
	for _, hit := range result.Hits {
		_, key := loinc.ParseLocalSearchDocID(hit.ID)
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys, result.Total, nil
}

// searchAPISortField resolves a Search API `sortorder` value ("loinc_num" or
// "loinc_num desc") to a bleve sort spec ("-Field" for descending) via the
// same field aliases the query parser uses. ok is false for an unrecognized
// or blank sortorder, meaning "keep relevance order".
func searchAPISortField(scope string, sortorder string) (string, bool) {
	sortorder = strings.TrimSpace(sortorder)
	if sortorder == "" {
		return "", false
	}
	tokens := strings.Fields(sortorder)
	desc := len(tokens) > 1 && strings.EqualFold(tokens[1], "desc")
	canonical, ok := localSearchFieldAliases(scope)[strings.ToLower(tokens[0])]
	if !ok {
		return "", false
	}
	if desc {
		return "-" + canonical, true
	}
	return canonical, true
}

func (a *app) localSearchStatus(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeJSON(w, http.StatusOK, a.localSearch.status(r.Context(), nil))
		return
	}
	writeJSON(w, http.StatusOK, a.localSearch.status(r.Context(), store))
}

func (a *app) rebuildLocalSearch(w http.ResponseWriter, r *http.Request) {
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	status, err := a.localSearch.rebuild(r.Context(), store)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (a *app) localSearchQuery(w http.ResponseWriter, r *http.Request) {
	var request LocalSearchRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid local Lucene search request"))
		return
	}
	store, err := a.currentStore()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	response, status, err := a.localSearch.query(r.Context(), store, request)
	if err != nil {
		writeError(w, status, err)
		return
	}
	writeJSON(w, status, response)
}

func normalizeLocalSearchScope(scope string) (string, bool) {
	switch normalizedOfficialScope(scope) {
	case "loincs":
		return "loincs", true
	case "parts":
		return "parts", true
	case "answerlists":
		return "answerlists", true
	case "groups":
		return "groups", true
	default:
		return "", false
	}
}

var localSearchFieldPattern = regexp.MustCompile(`(^|[\s(+-])([A-Za-z][A-Za-z0-9.]*):`)
var localSearchLOINCCodePattern = regexp.MustCompile(`(?i)(^|[\s(])([+-]?)(LOINC|LOINC_NUM|LOINCNUM):(\d+)(?:-(\d|\?))?(\*)?`)
var localSearchBareLOINCNumberPattern = regexp.MustCompile(`^\d{1,7}-\d$`)

func rewriteLocalSearchQuery(scope string, raw string) (string, []string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	aliases := localSearchFieldAliases(scope)
	indexed := localSearchIndexedFields(scope)
	warnings := []string{}
	seenWarnings := map[string]bool{}
	if scope == "loincs" && localSearchBareLOINCNumberPattern.MatchString(raw) {
		// A bare "718-7" (no field prefix, no boolean operators) is a whole
		// LOINC number, not "718 AND NOT 7": the lexer below treats "-" as
		// the NOT operator, which would otherwise silently exclude the very
		// term being searched for (its own tokenized key contains "7").
		// Match it as a phrase against the LOINC-number key field instead.
		return `key:"` + raw + `"`, nil
	}
	if scope == "loincs" {
		raw = localSearchLOINCCodePattern.ReplaceAllStringFunc(raw, func(match string) string {
			matches := localSearchLOINCCodePattern.FindStringSubmatch(match)
			if len(matches) != 7 {
				return match
			}
			prefix := matches[1]
			sign := matches[2]
			root := matches[4]
			return prefix + sign + "key:" + root + "*"
		})
	}
	rewritten := localSearchFieldPattern.ReplaceAllStringFunc(raw, func(match string) string {
		prefix := match[:len(match)-len(strings.TrimLeft(match, " \t\r\n(+-"))]
		fieldWithColon := strings.TrimSpace(strings.TrimLeft(match, " \t\r\n(+-"))
		field := strings.TrimSuffix(fieldWithColon, ":")
		canonical, ok := aliases[strings.ToLower(field)]
		if !ok {
			message := fmt.Sprintf("field %q is not recognized for %s local Lucene search", field, scope)
			if !seenWarnings[message] {
				warnings = append(warnings, message)
				seenWarnings[message] = true
			}
			return match
		}
		if !indexed[canonical] {
			message := fmt.Sprintf("field %q is planned but not indexed from the current local database", canonical)
			if !seenWarnings[message] {
				warnings = append(warnings, message)
				seenWarnings[message] = true
			}
		}
		return prefix + canonical + ":"
	})
	return rewritten, warnings
}

func localSearchFieldAliases(scope string) map[string]string {
	out := map[string]string{}
	for _, field := range localSearchPlannedFields(scope) {
		out[strings.ToLower(field)] = field
	}
	out["key"] = "key"
	switch scope {
	case "loincs":
		out["loinc_num"] = "LOINC"
		out["loincnum"] = "LOINC"
		out["longcommonname"] = "LongName"
		out["longname"] = "LongName"
		out["time"] = "Timing"
		out["timeaspct"] = "Timing"
		out["scale_typ"] = "Scale"
		out["method_typ"] = "Method"
		out["commonlabresult"] = "CommonLabResult"
	case "parts":
		out["partnumber"] = "Partnumber"
		out["partnum"] = "Partnumber"
		out["name"] = "Name"
	case "answerlists":
		out["answerlistid"] = "AnswerList"
		out["loincanswerlistoid"] = "LOINCAnswerListOID"
	case "groups":
		out["groupid"] = "GroupId"
	}
	return out
}

func localSearchIndexedFields(scope string) map[string]bool {
	fields := map[string][]string{
		"loincs": {
			"key",
			"LOINC", "Component", "Property", "Timing", "System", "Scale", "Method", "Class",
			"LongName", "ShortName", "DisplayName", "Description", "Status", "OrderObs",
			"Rank", "CommonOrder", "Ranked", "CommonLabResult", "ComponentWordCount",
			"CoreComponent", "Methodless", "LabTest", "MassProperty", "SubstanceProperty",
			"SuperSystem", "TimeModifier", "Punctuation", "AnswerList", "AnswerListId",
			"AnswerListName", "MapToLOINC",
		},
		"parts":       {"Partnumber", "Part", "Name", "DisplayName", "Type", "Status", "ClassList"},
		"answerlists": {"AnswerList", "Name", "LOINCAnswerListOID", "ExternalListURL", "ExternallyDefined", "AnswerCount", "LoincCount", "AnswerCode", "AnswerCodeSystem", "CodeSystem", "AnswerDisplayText", "AnswerScore", "AnswerSequenceNum", "AnswerString", "AnswerStringDescription"},
		"groups":      {"Group", "GroupId", "Name", "Archetype", "ParentGroup", "Status", "VersionFirstReleased", "LoincCount"},
	}
	out := map[string]bool{}
	for _, field := range fields[scope] {
		out[field] = true
	}
	return out
}

func localSearchPlannedFields(scope string) []string {
	switch scope {
	case "loincs":
		return []string{"AllowMethodSpecific", "AnswerList", "AnswerListId", "AnswerListName", "AnswerListType", "AskAtOrderEntry", "AssociatedObservations", "AttachmentUnitsRequired", "Categorization", "ClassHierarchy", "ComponentHierarchy", "MethodHierarchy", "MultiAxialHierarchy", "SystemHierarchy", "CommonOrder", "Ranked", "CommonLabResult", "ComponentWordCount", "CoreComponent", "Description", "DisplayName", "ExUCUMunits", "ExUnits", "Formula", "HL7AttachmentStructure", "HL7FieldSubId", "LabTest", "LForms", "LongName", "MapToLOINC", "MassProperty", "Methodless", "NonroutineChallenge", "OrderObs", "OtherCopyright", "PanelType", "Pharma", "Punctuation", "Rank", "RelatedCodes", "ShortName", "Status", "StatusReason", "StatusText", "SubstanceProperty", "SuperSystem", "SurveyQuestionSource", "SurveyQuestionText", "TimeModifier", "Type", "TypeName", "UniversalLabOrders", "ValidHL7AttachmentRequest", "VersionLastChanged", "LOINC", "Component", "Property", "Timing", "System", "Scale", "Method", "Class"}
	case "parts":
		return []string{"Partnumber", "Part", "Name", "Abbreviation", "Article", "Book", "Citation", "ClassList", "CreatedOn", "Description", "DisplayName", "Image", "MolecularWeight", "OriginalForm", "PackageInsert", "Synonyms", "TechnicalBrief", "Type", "WebContent", "Status"}
	case "answerlists":
		return []string{"AnswerList", "Name", "Description", "AnswerCode", "AnswerCodeSystem", "LOINCAnswerListOID", "AnswerCount", "AnswerDisplayText", "AnswerScore", "AnswerSequenceNum", "AnswerString", "AnswerStringDescription", "CodeSystem", "ExternalAnswerListOID", "ExternalListURL", "ExternallyDefined", "LoincCount", "SourceName"}
	case "groups":
		return []string{"Group", "GroupId", "Name", "Archetype", "ParentGroup", "Status", "VersionFirstReleased", "LoincCount"}
	default:
		return nil
	}
}

func localSearchFieldCoverage() map[string]string {
	coverage := map[string]string{}
	for _, scope := range []string{"loincs", "parts", "answerlists", "groups"} {
		indexed := localSearchIndexedFields(scope)
		for _, field := range localSearchPlannedFields(scope) {
			key := scope + "." + field
			if indexed[field] {
				coverage[key] = "indexed"
			} else {
				coverage[key] = "requires_expanded_ingest"
			}
		}
	}
	return coverage
}

func localSearchCoverageWarnings() []string {
	warnings := []string{
		"Some official fields require expanded ingest before they can be indexed locally.",
		"Exact Regenstrief ranking parity is not promised by the local Lucene index.",
	}
	sort.Strings(warnings)
	return warnings
}

type localLuceneTokenKind int

const (
	localLuceneTokenEOF localLuceneTokenKind = iota
	localLuceneTokenWord
	localLuceneTokenPhrase
	localLuceneTokenAnd
	localLuceneTokenOr
	localLuceneTokenNot
	localLuceneTokenPlus
	localLuceneTokenMinus
	localLuceneTokenColon
	localLuceneTokenLParen
	localLuceneTokenRParen
	localLuceneTokenRange
)

type localLuceneToken struct {
	kind       localLuceneTokenKind
	value      string
	rangeStart string
	rangeEnd   string
	inclusive  bool
}

type localLuceneParser struct {
	tokens []localLuceneToken
	pos    int
}

func parseLocalLuceneQuery(raw string) (query.Query, error) {
	tokens, err := lexLocalLuceneQuery(raw)
	if err != nil {
		return nil, err
	}
	parser := localLuceneParser{tokens: tokens}
	parsed, err := parser.parseOr("")
	if err != nil {
		return nil, err
	}
	if parser.peek().kind != localLuceneTokenEOF {
		return nil, fmt.Errorf("unexpected token %q", parser.peek().value)
	}
	return parsed, nil
}

func (p *localLuceneParser) parseOr(field string) (query.Query, error) {
	left, err := p.parseAnd(field)
	if err != nil {
		return nil, err
	}
	for p.match(localLuceneTokenOr) {
		right, err := p.parseAnd(field)
		if err != nil {
			return nil, err
		}
		left = bleve.NewDisjunctionQuery(left, right)
	}
	return left, nil
}

func (p *localLuceneParser) parseAnd(field string) (query.Query, error) {
	parts := []query.Query{}
	stops := []query.Query{}
	for {
		stop := p.atBareStopWord()
		part, err := p.parseUnary(field)
		if err != nil {
			return nil, err
		}
		if stop {
			stops = append(stops, part)
		} else {
			parts = append(parts, part)
		}
		switch p.peek().kind {
		case localLuceneTokenEOF, localLuceneTokenRParen, localLuceneTokenOr:
			// Stop words only count when nothing else is left to match.
			if len(parts) == 0 {
				parts = stops
			}
			return conjunctionForLocalLucene(parts), nil
		case localLuceneTokenAnd:
			p.next()
		}
	}
}

// atBareStopWord reports whether the next token is a plain word (not a field name, phrase, or
// wildcard/fuzzy term) that is a stop word.
func (p *localLuceneParser) atBareStopWord() bool {
	token := p.peek()
	if token.kind != localLuceneTokenWord || !loinc.IsStopWord(token.value) {
		return false
	}
	return p.pos+1 >= len(p.tokens) || p.tokens[p.pos+1].kind != localLuceneTokenColon
}

func (p *localLuceneParser) parseUnary(field string) (query.Query, error) {
	if p.match(localLuceneTokenPlus) {
		return p.parseUnary(field)
	}
	if p.match(localLuceneTokenMinus) || p.match(localLuceneTokenNot) {
		child, err := p.parseUnary(field)
		if err != nil {
			return nil, err
		}
		bq := bleve.NewBooleanQuery()
		bq.AddMustNot(child)
		return bq, nil
	}
	return p.parsePrimary(field)
}

func (p *localLuceneParser) parsePrimary(field string) (query.Query, error) {
	token := p.next()
	switch token.kind {
	case localLuceneTokenWord:
		if p.match(localLuceneTokenColon) {
			nextField := token.value
			if p.match(localLuceneTokenLParen) {
				child, err := p.parseOr(nextField)
				if err != nil {
					return nil, err
				}
				if !p.match(localLuceneTokenRParen) {
					return nil, errors.New("missing closing parenthesis")
				}
				return child, nil
			}
			if p.peek().kind == localLuceneTokenRange {
				rangeToken := p.next()
				return localLuceneRangeQuery(nextField, rangeToken.rangeStart, rangeToken.rangeEnd, rangeToken.inclusive), nil
			}
			return p.parsePrimary(nextField)
		}
		return localLuceneTermQuery(field, token.value), nil
	case localLuceneTokenPhrase:
		return localLucenePhraseQuery(field, token.value), nil
	case localLuceneTokenLParen:
		child, err := p.parseOr(field)
		if err != nil {
			return nil, err
		}
		if !p.match(localLuceneTokenRParen) {
			return nil, errors.New("missing closing parenthesis")
		}
		return child, nil
	case localLuceneTokenRange:
		return localLuceneRangeQuery(field, token.rangeStart, token.rangeEnd, token.inclusive), nil
	default:
		return nil, fmt.Errorf("unexpected token %q", token.value)
	}
}

func conjunctionForLocalLucene(parts []query.Query) query.Query {
	if len(parts) == 1 {
		return parts[0]
	}
	bq := bleve.NewBooleanQuery()
	for _, part := range parts {
		bq.AddMust(part)
	}
	return bq
}

func localLuceneTermQuery(field string, raw string) query.Query {
	value, fuzzy := strings.CutSuffix(raw, "~")
	fuzziness := 2
	if !fuzzy {
		if idx := strings.LastIndex(value, "~"); idx > 0 {
			if parsedFuzziness, err := strconv.Atoi(value[idx+1:]); err == nil {
				fuzzy = true
				value = value[:idx]
				fuzziness = parsedFuzziness
			}
		}
	}
	value = unescapeLocalLuceneValue(value)
	var q query.FieldableQuery
	if strings.ContainsAny(value, "*?") {
		q = bleve.NewWildcardQuery(value)
	} else {
		mq := bleve.NewMatchQuery(value)
		if fuzzy {
			mq.SetFuzziness(fuzziness)
		}
		q = mq
	}
	if field != "" {
		q.SetField(field)
	}
	return q
}

func localLucenePhraseQuery(field string, value string) query.Query {
	q := bleve.NewMatchPhraseQuery(unescapeLocalLuceneValue(value))
	if field != "" {
		q.SetField(field)
	}
	return q
}

func localLuceneRangeQuery(field string, start string, end string, inclusive bool) query.Query {
	start = unescapeLocalLuceneValue(start)
	end = unescapeLocalLuceneValue(end)
	startFloat, startErr := strconv.ParseFloat(start, 64)
	endFloat, endErr := strconv.ParseFloat(end, 64)
	if startErr == nil && endErr == nil {
		q := bleve.NewNumericRangeInclusiveQuery(&startFloat, &endFloat, &inclusive, &inclusive)
		if field != "" {
			q.SetField(field)
		}
		return q
	}
	q := bleve.NewTermRangeInclusiveQuery(start, end, &inclusive, &inclusive)
	if field != "" {
		q.SetField(field)
	}
	return q
}

func (p *localLuceneParser) peek() localLuceneToken {
	if p.pos >= len(p.tokens) {
		return localLuceneToken{kind: localLuceneTokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *localLuceneParser) next() localLuceneToken {
	token := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return token
}

func (p *localLuceneParser) match(kind localLuceneTokenKind) bool {
	if p.peek().kind != kind {
		return false
	}
	p.pos++
	return true
}

func lexLocalLuceneQuery(raw string) ([]localLuceneToken, error) {
	tokens := []localLuceneToken{}
	for i := 0; i < len(raw); {
		ch := raw[i]
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
			i++
			continue
		}
		switch ch {
		case '+':
			tokens = append(tokens, localLuceneToken{kind: localLuceneTokenPlus, value: "+"})
			i++
			continue
		case '-':
			tokens = append(tokens, localLuceneToken{kind: localLuceneTokenMinus, value: "-"})
			i++
			continue
		case ':':
			tokens = append(tokens, localLuceneToken{kind: localLuceneTokenColon, value: ":"})
			i++
			continue
		case '(':
			tokens = append(tokens, localLuceneToken{kind: localLuceneTokenLParen, value: "("})
			i++
			continue
		case ')':
			tokens = append(tokens, localLuceneToken{kind: localLuceneTokenRParen, value: ")"})
			i++
			continue
		case '"':
			value, next, err := scanLocalLuceneQuoted(raw, i)
			if err != nil {
				return nil, err
			}
			i = next
			if i < len(raw) && raw[i] == '~' {
				i++
				for i < len(raw) && raw[i] >= '0' && raw[i] <= '9' {
					i++
				}
			}
			tokens = append(tokens, localLuceneToken{kind: localLuceneTokenPhrase, value: value})
			continue
		case '[', '{':
			token, next, err := scanLocalLuceneRange(raw, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token)
			i = next
			continue
		}
		start := i
		escaped := false
		for i < len(raw) {
			if escaped {
				escaped = false
				i++
				continue
			}
			if raw[i] == '\\' {
				escaped = true
				i++
				continue
			}
			if strings.ContainsRune(" \t\r\n:+-()[]{}", rune(raw[i])) {
				break
			}
			i++
		}
		if start == i {
			return nil, fmt.Errorf("unsupported character %q", raw[i])
		}
		value := raw[start:i]
		switch strings.ToUpper(value) {
		case "AND":
			tokens = append(tokens, localLuceneToken{kind: localLuceneTokenAnd, value: value})
		case "OR":
			tokens = append(tokens, localLuceneToken{kind: localLuceneTokenOr, value: value})
		case "NOT":
			tokens = append(tokens, localLuceneToken{kind: localLuceneTokenNot, value: value})
		default:
			tokens = append(tokens, localLuceneToken{kind: localLuceneTokenWord, value: value})
		}
	}
	tokens = append(tokens, localLuceneToken{kind: localLuceneTokenEOF})
	return tokens, nil
}

func scanLocalLuceneQuoted(raw string, start int) (string, int, error) {
	var builder strings.Builder
	escaped := false
	for i := start + 1; i < len(raw); i++ {
		if escaped {
			builder.WriteByte(raw[i])
			escaped = false
			continue
		}
		if raw[i] == '\\' {
			escaped = true
			continue
		}
		if raw[i] == '"' {
			return builder.String(), i + 1, nil
		}
		builder.WriteByte(raw[i])
	}
	return "", start, errors.New("unterminated quoted phrase")
}

func scanLocalLuceneRange(raw string, start int) (localLuceneToken, int, error) {
	open := raw[start]
	close := byte(']')
	inclusive := true
	if open == '{' {
		close = '}'
		inclusive = false
	}
	end := strings.IndexByte(raw[start+1:], close)
	if end < 0 {
		return localLuceneToken{}, start, errors.New("unterminated range")
	}
	body := strings.TrimSpace(raw[start+1 : start+1+end])
	parts := strings.SplitN(body, " TO ", 2)
	if len(parts) != 2 {
		return localLuceneToken{}, start, errors.New("range must use TO")
	}
	return localLuceneToken{
		kind:       localLuceneTokenRange,
		value:      raw[start : start+end+2],
		rangeStart: strings.TrimSpace(parts[0]),
		rangeEnd:   strings.TrimSpace(parts[1]),
		inclusive:  inclusive,
	}, start + end + 2, nil
}

func unescapeLocalLuceneValue(value string) string {
	var builder strings.Builder
	escaped := false
	for i := 0; i < len(value); i++ {
		if escaped {
			builder.WriteByte(value[i])
			escaped = false
			continue
		}
		if value[i] == '\\' {
			escaped = true
			continue
		}
		builder.WriteByte(value[i])
	}
	if escaped {
		builder.WriteByte('\\')
	}
	return builder.String()
}
