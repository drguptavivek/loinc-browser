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
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"

	"loinc-browser/internal/loinc"
)

const defaultLocalSearchIndexPath = "./data/loinc-search.bleve"

type localSearchService struct {
	path string
	mu   sync.Mutex
}

type LocalSearchStatus struct {
	State         string            `json:"state"`
	IndexPath     string            `json:"indexPath"`
	DocCount      uint64            `json:"docCount"`
	UpdatedAt     string            `json:"updatedAt,omitempty"`
	FieldCoverage map[string]string `json:"fieldCoverage,omitempty"`
	Warnings      []string          `json:"warnings,omitempty"`
	Message       string            `json:"message,omitempty"`
}

type LocalSearchRequest struct {
	Scope  string `json:"scope"`
	Query  string `json:"query"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
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

func (s *localSearchService) status(ctx context.Context, store *loinc.Store) LocalSearchStatus {
	status := LocalSearchStatus{
		State:         "missing",
		IndexPath:     s.path,
		FieldCoverage: localSearchFieldCoverage(),
	}
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
	status.State = "ready"
	status.Message = "local Lucene index is ready"
	status.Warnings = localSearchCoverageWarnings()
	_ = ctx
	return status
}

func (s *localSearchService) rebuild(ctx context.Context, store *loinc.Store) (LocalSearchStatus, error) {
	if store == nil {
		return LocalSearchStatus{State: "requires_reingest", IndexPath: s.path, Message: "local LOINC database is not loaded"}, errors.New("LOINC database is not loaded")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.RemoveAll(s.path); err != nil {
		return LocalSearchStatus{}, fmt.Errorf("remove local Lucene index: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return LocalSearchStatus{}, fmt.Errorf("create local Lucene index directory: %w", err)
	}
	mapping := bleve.NewIndexMapping()
	mapping.DefaultField = "_all"
	index, err := bleve.New(s.path, mapping)
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
		return LocalSearchStatus{}, fmt.Errorf("index local Lucene documents: %w", err)
	}
	if batch.Size() > 0 {
		if err := index.Batch(batch); err != nil {
			return LocalSearchStatus{}, fmt.Errorf("commit local Lucene index batch: %w", err)
		}
	}
	if err := index.SetInternal([]byte("loinc-browser-local-search-built-at"), []byte(time.Now().Format(time.RFC3339))); err != nil {
		_ = index.Close()
		return LocalSearchStatus{}, fmt.Errorf("write local Lucene index metadata: %w", err)
	}
	if err := index.Close(); err != nil {
		return LocalSearchStatus{}, fmt.Errorf("close local Lucene index: %w", err)
	}
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
	if limit > 100 {
		limit = 100
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

func rewriteLocalSearchQuery(scope string, raw string) (string, []string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	aliases := localSearchFieldAliases(scope)
	indexed := localSearchIndexedFields(scope)
	warnings := []string{}
	seenWarnings := map[string]bool{}
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
	first, err := p.parseUnary(field)
	if err != nil {
		return nil, err
	}
	parts = append(parts, first)
	for {
		switch p.peek().kind {
		case localLuceneTokenEOF, localLuceneTokenRParen, localLuceneTokenOr:
			return conjunctionForLocalLucene(parts), nil
		case localLuceneTokenAnd:
			p.next()
		}
		next, err := p.parseUnary(field)
		if err != nil {
			return nil, err
		}
		parts = append(parts, next)
	}
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
