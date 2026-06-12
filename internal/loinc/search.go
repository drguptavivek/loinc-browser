package loinc

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	_ "modernc.org/sqlite"
)

type Store struct {
	db    *sql.DB
	cache *objectCache
}

func OpenStore(dbPath string, options StoreOptions) (*Store, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, errors.New("database path is required")
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := ensureRuntimeIndexes(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{
		db:    db,
		cache: newObjectCache(options.CacheEntries),
	}, nil
}

func ensureRuntimeIndexes(db *sql.DB) error {
	var hasAccessories int
	if err := db.QueryRow(`select count(*) from sqlite_master where type = 'table' and name = 'term_accessories'`).Scan(&hasAccessories); err != nil {
		return fmt.Errorf("check runtime tables: %w", err)
	}
	if hasAccessories == 0 {
		return nil
	}
	statements := []string{
		`create index if not exists idx_term_accessories_kind_code_loinc on term_accessories(kind, code, loinc_num)`,
		`create index if not exists idx_term_accessories_kind_id on term_accessories(kind, id)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("ensure runtime index: %w", err)
		}
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Search(ctx context.Context, params SearchParams) (SearchResponse, error) {
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	where, args := filterClauses(params, "t")
	query := strings.TrimSpace(params.Query)
	ftsQuery := makeFTSQuery(query)
	exactLOINC := loincNumberRegexp.MatchString(query)
	if exactLOINC {
		where = append(where, "t.loinc_num = ? collate nocase")
		args = append(args, query)
		ftsQuery = ""
	} else if ftsQuery != "" {
		where = append(where, "loinc_terms_fts match ?")
		args = append(args, ftsQuery)
	}

	base := `from loinc_terms t`
	if ftsQuery != "" {
		base += ` join loinc_terms_fts f on f.loinc_num = t.loinc_num`
	}
	if len(where) > 0 {
		base += ` where ` + strings.Join(where, " and ")
	}

	countQuery := `select count(*) ` + base
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return SearchResponse{}, fmt.Errorf("count search results: %w", err)
	}

	order := `order by nullif(t.common_test_rank, 0), t.long_common_name`
	selectRank := `0.0 as rank`
	if ftsQuery != "" {
		selectRank = `bm25(loinc_terms_fts) as rank`
		order = `order by rank, nullif(t.common_test_rank, 0), t.long_common_name`
	}

	searchQuery := `select
		t.loinc_num, t.long_common_name, t.short_name, t.component, t.property,
		t.system, t.scale, t.method, t.class, t.status, t.order_obs, ` + selectRank + ` ` + base + ` ` + order + ` limit ? offset ?`
	searchArgs := append(args, limit, offset)
	rows, err := s.db.QueryContext(ctx, searchQuery, searchArgs...)
	if err != nil {
		return SearchResponse{}, fmt.Errorf("search LOINC terms: %w", err)
	}
	defer rows.Close()

	results := make([]SearchResult, 0, limit)
	for rows.Next() {
		var result SearchResult
		if err := rows.Scan(
			&result.LOINCNum,
			&result.LongCommonName,
			&result.ShortName,
			&result.Component,
			&result.Property,
			&result.System,
			&result.Scale,
			&result.Method,
			&result.Class,
			&result.Status,
			&result.OrderObs,
			&result.Rank,
		); err != nil {
			return SearchResponse{}, fmt.Errorf("scan search result: %w", err)
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return SearchResponse{}, fmt.Errorf("iterate search results: %w", err)
	}

	return SearchResponse{
		Results: results,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		Query:   query,
	}, nil
}

func (s *Store) Term(ctx context.Context, loincNum string) (Term, error) {
	return s.term(ctx, loincNum, false)
}

func (s *Store) TermWithAccessories(ctx context.Context, loincNum string) (Term, error) {
	return s.term(ctx, loincNum, true)
}

func (s *Store) term(ctx context.Context, loincNum string, includeAccessories bool) (Term, error) {
	key := strings.ToUpper(strings.TrimSpace(loincNum))
	if key == "" {
		return Term{}, errors.New("LOINC number is required")
	}
	cacheKey := key
	if includeAccessories {
		cacheKey += ":REL"
	}
	if term, ok := s.cache.getTerm(cacheKey); ok {
		return term, nil
	}

	row := s.db.QueryRowContext(ctx, `select
		loinc_num, long_common_name, short_name, component, property, time_aspect,
		system, scale, method, class, status, definition, consumer_name, related_names,
		order_obs, display_name, raw_json
		from loinc_terms where loinc_num = ? collate nocase`, key)

	var term Term
	var rawJSON string
	if err := row.Scan(
		&term.LOINCNum,
		&term.LongCommonName,
		&term.ShortName,
		&term.Component,
		&term.Property,
		&term.TimeAspect,
		&term.System,
		&term.Scale,
		&term.Method,
		&term.Class,
		&term.Status,
		&term.Definition,
		&term.ConsumerName,
		&term.RelatedNames,
		&term.OrderObs,
		&term.DisplayName,
		&rawJSON,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Term{}, ErrNotFound
		}
		return Term{}, fmt.Errorf("load term %s: %w", key, err)
	}
	if err := json.Unmarshal([]byte(rawJSON), &term.Fields); err != nil {
		return Term{}, fmt.Errorf("decode term fields %s: %w", key, err)
	}
	if includeAccessories {
		if err := s.loadTermAccessories(ctx, &term); err != nil {
			return Term{}, err
		}
	} else {
		term.MapTo = []MapTo{}
		term.Parts = []TermAccessory{}
		term.AnswerLists = []TermAccessory{}
		term.Panels = []TermAccessory{}
		term.Groups = []TermAccessory{}
		term.Hierarchy = []TermAccessory{}
	}

	s.cache.setTerm(cacheKey, term)
	return term, nil
}

func (s *Store) loadTermAccessories(ctx context.Context, term *Term) error {
	term.MapTo = []MapTo{}
	term.Parts = []TermAccessory{}
	term.AnswerLists = []TermAccessory{}
	term.Panels = []TermAccessory{}
	term.Groups = []TermAccessory{}
	term.Hierarchy = []TermAccessory{}
	mapRows, err := s.db.QueryContext(ctx, `select loinc_num, map_to, comment from map_to where loinc_num = ? collate nocase order by map_to`, term.LOINCNum)
	if err != nil {
		return fmt.Errorf("load MapTo rows for %s: %w", term.LOINCNum, err)
	}
	defer mapRows.Close()
	for mapRows.Next() {
		var item MapTo
		if err := mapRows.Scan(&item.LOINC, &item.MapTo, &item.Comment); err != nil {
			return fmt.Errorf("scan MapTo row for %s: %w", term.LOINCNum, err)
		}
		term.MapTo = append(term.MapTo, item)
	}
	if err := mapRows.Err(); err != nil {
		return fmt.Errorf("iterate MapTo rows for %s: %w", term.LOINCNum, err)
	}

	rows, err := s.db.QueryContext(ctx, `select kind, code, title, subtitle, raw_json
		from term_accessories where loinc_num = ? collate nocase order by kind, title, code limit 500`, term.LOINCNum)
	if err != nil {
		return fmt.Errorf("load accessory rows for %s: %w", term.LOINCNum, err)
	}
	defer rows.Close()
	for rows.Next() {
		var item TermAccessory
		var rawJSON string
		if err := rows.Scan(&item.Kind, &item.Code, &item.Title, &item.Subtitle, &rawJSON); err != nil {
			return fmt.Errorf("scan accessory row for %s: %w", term.LOINCNum, err)
		}
		if err := json.Unmarshal([]byte(rawJSON), &item.Fields); err != nil {
			return fmt.Errorf("decode accessory row for %s: %w", term.LOINCNum, err)
		}
		switch item.Kind {
		case "part-primary", "part-supplementary":
			term.Parts = append(term.Parts, item)
		case "answer-list":
			term.AnswerLists = append(term.AnswerLists, item)
		case "panel-membership", "panel-child":
			term.Panels = append(term.Panels, item)
		case "group":
			term.Groups = append(term.Groups, item)
		case "hierarchy":
			term.Hierarchy = append(term.Hierarchy, item)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate accessory rows for %s: %w", term.LOINCNum, err)
	}
	return nil
}

func (s *Store) TermRelationships(ctx context.Context, loincNum string) (TermRelationshipGraph, error) {
	key := strings.ToUpper(strings.TrimSpace(loincNum))
	if key == "" {
		return TermRelationshipGraph{}, errors.New("LOINC number is required")
	}
	if graph, ok := s.cache.getGraph(key); ok {
		return graph, nil
	}
	if _, err := s.Term(ctx, key); err != nil {
		return TermRelationshipGraph{}, err
	}

	graph := TermRelationshipGraph{
		LOINCNum:       key,
		OutgoingMapTo:  []MapTo{},
		IncomingMapTo:  []MapTo{},
		SharedConcepts: []RelationshipConcept{},
	}
	outRows, err := s.db.QueryContext(ctx, `select loinc_num, map_to, comment from map_to where loinc_num = ? collate nocase order by map_to`, key)
	if err != nil {
		return TermRelationshipGraph{}, fmt.Errorf("load outgoing MapTo rows for %s: %w", key, err)
	}
	defer outRows.Close()
	for outRows.Next() {
		var item MapTo
		if err := outRows.Scan(&item.LOINC, &item.MapTo, &item.Comment); err != nil {
			return TermRelationshipGraph{}, fmt.Errorf("scan outgoing MapTo row for %s: %w", key, err)
		}
		graph.OutgoingMapTo = append(graph.OutgoingMapTo, item)
	}
	if err := outRows.Err(); err != nil {
		return TermRelationshipGraph{}, fmt.Errorf("iterate outgoing MapTo rows for %s: %w", key, err)
	}

	inRows, err := s.db.QueryContext(ctx, `select loinc_num, map_to, comment from map_to where map_to = ? collate nocase order by loinc_num`, key)
	if err != nil {
		return TermRelationshipGraph{}, fmt.Errorf("load incoming MapTo rows for %s: %w", key, err)
	}
	defer inRows.Close()
	for inRows.Next() {
		var item MapTo
		if err := inRows.Scan(&item.LOINC, &item.MapTo, &item.Comment); err != nil {
			return TermRelationshipGraph{}, fmt.Errorf("scan incoming MapTo row for %s: %w", key, err)
		}
		graph.IncomingMapTo = append(graph.IncomingMapTo, item)
	}
	if err := inRows.Err(); err != nil {
		return TermRelationshipGraph{}, fmt.Errorf("iterate incoming MapTo rows for %s: %w", key, err)
	}

	conceptRows, err := s.db.QueryContext(ctx, `select kind, code, title, subtitle, raw_json
		from term_accessories where loinc_num = ? collate nocase order by kind, title, code limit 100`, key)
	if err != nil {
		return TermRelationshipGraph{}, fmt.Errorf("load relationship concepts for %s: %w", key, err)
	}
	defer conceptRows.Close()
	seen := map[string]bool{}
	for conceptRows.Next() {
		var concept RelationshipConcept
		var rawJSON string
		if err := conceptRows.Scan(&concept.Kind, &concept.Code, &concept.Title, &concept.Subtitle, &rawJSON); err != nil {
			return TermRelationshipGraph{}, fmt.Errorf("scan relationship concept for %s: %w", key, err)
		}
		identity := concept.Kind + "\x00" + concept.Code
		if seen[identity] {
			continue
		}
		seen[identity] = true
		if err := json.Unmarshal([]byte(rawJSON), &concept.Fields); err != nil {
			return TermRelationshipGraph{}, fmt.Errorf("decode relationship concept for %s: %w", key, err)
		}
		if concept.Code == "" {
			continue
		}
		if err := s.loadConceptNeighbors(ctx, key, &concept); err != nil {
			return TermRelationshipGraph{}, err
		}
		graph.SharedConcepts = append(graph.SharedConcepts, concept)
	}
	if err := conceptRows.Err(); err != nil {
		return TermRelationshipGraph{}, fmt.Errorf("iterate relationship concepts for %s: %w", key, err)
	}
	s.cache.setGraph(key, graph)
	return graph, nil
}

func (s *Store) loadConceptNeighbors(ctx context.Context, loincNum string, concept *RelationshipConcept) error {
	concept.RelatedTerms = []TermSummary{}
	if err := s.db.QueryRowContext(ctx, `select count(distinct loinc_num) from term_accessories
		where kind = ? and code = ? and loinc_num <> ?`, concept.Kind, concept.Code, loincNum).Scan(&concept.RelatedTotal); err != nil {
		return fmt.Errorf("count related terms for %s %s: %w", concept.Kind, concept.Code, err)
	}
	rows, err := s.db.QueryContext(ctx, `select distinct t.loinc_num, t.long_common_name, t.short_name, t.status, t.system, t.class
		from term_accessories a
		join loinc_terms t on t.loinc_num = a.loinc_num
		where a.kind = ? and a.code = ? and a.loinc_num <> ?
		order by nullif(t.common_test_rank, 0), t.long_common_name
		limit 20`, concept.Kind, concept.Code, loincNum)
	if err != nil {
		return fmt.Errorf("load related terms for %s %s: %w", concept.Kind, concept.Code, err)
	}
	defer rows.Close()
	for rows.Next() {
		var term TermSummary
		if err := rows.Scan(&term.LOINCNum, &term.LongCommonName, &term.ShortName, &term.Status, &term.System, &term.Class); err != nil {
			return fmt.Errorf("scan related term for %s %s: %w", concept.Kind, concept.Code, err)
		}
		concept.RelatedTerms = append(concept.RelatedTerms, term)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate related terms for %s %s: %w", concept.Kind, concept.Code, err)
	}
	return nil
}

var ErrNotFound = errors.New("not found")

func (s *Store) Facets(ctx context.Context) (Facets, error) {
	if facets, ok := s.cache.getFacets(); ok {
		return facets, nil
	}
	facets := Facets{
		Classes:     map[string]int{},
		Statuses:    map[string]int{},
		Systems:     map[string]int{},
		TimeAspects: map[string]int{},
		Scales:      map[string]int{},
		Methods:     map[string]int{},
		Properties:  map[string]int{},
		OrderObs:    map[string]int{},
	}
	queries := []struct {
		column string
		target map[string]int
	}{
		{"class", facets.Classes},
		{"status", facets.Statuses},
		{"system", facets.Systems},
		{"time_aspect", facets.TimeAspects},
		{"scale", facets.Scales},
		{"method", facets.Methods},
		{"property", facets.Properties},
		{"order_obs", facets.OrderObs},
	}
	for _, query := range queries {
		if err := s.loadFacet(ctx, query.column, query.target); err != nil {
			return Facets{}, err
		}
	}
	s.cache.setFacets(facets)
	return facets, nil
}

func (s *Store) CacheStats() CacheStats {
	return s.cache.stats()
}

func (s *Store) SourceOrganizations(ctx context.Context) ([]SourceOrganization, error) {
	rows, err := s.db.QueryContext(ctx, `select id, copyright_id, name, copyright, terms_of_use, url, raw_json from source_organizations order by name`)
	if err != nil {
		return nil, fmt.Errorf("load source organizations: %w", err)
	}
	defer rows.Close()
	var items []SourceOrganization
	for rows.Next() {
		var item SourceOrganization
		var rawJSON string
		if err := rows.Scan(&item.ID, &item.CopyrightID, &item.Name, &item.Copyright, &item.TermsOfUse, &item.URL, &rawJSON); err != nil {
			return nil, fmt.Errorf("scan source organization: %w", err)
		}
		if err := json.Unmarshal([]byte(rawJSON), &item.Fields); err != nil {
			return nil, fmt.Errorf("decode source organization %s: %w", item.ID, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate source organizations: %w", err)
	}
	return items, nil
}

func (s *Store) BrowseAccessories(ctx context.Context, params AccessoryBrowseParams) (AccessoryBrowseResponse, error) {
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}
	kind := strings.TrimSpace(params.Kind)
	query := strings.TrimSpace(params.Query)
	cacheKey := fmt.Sprintf("kind=%s\x00q=%s\x00limit=%d\x00offset=%d", kind, query, limit, offset)
	if response, ok := s.cache.getAccessory(cacheKey); ok {
		return response, nil
	}
	var where []string
	var args []any
	if kind != "" {
		where = append(where, "a.kind = ?")
		args = append(args, kind)
	}
	if query != "" {
		where = append(where, "(a.loinc_num like ? collate nocase or a.code like ? collate nocase or a.title like ? collate nocase or a.subtitle like ? collate nocase)")
		like := "%" + query + "%"
		args = append(args, like, like, like, like)
	}
	base := "from term_accessories a"
	if len(where) > 0 {
		base += " where " + strings.Join(where, " and ")
	}
	var total int
	if err := s.db.QueryRowContext(ctx, "select count(*) "+base, args...).Scan(&total); err != nil {
		return AccessoryBrowseResponse{}, fmt.Errorf("count accessory rows: %w", err)
	}
	selectBase := base
	if len(where) > 0 {
		selectBase = "from term_accessories a left join loinc_terms t on t.loinc_num = a.loinc_num where " + strings.Join(where, " and ")
	} else {
		selectBase = "from term_accessories a left join loinc_terms t on t.loinc_num = a.loinc_num"
	}
	orderBy := `order by a.kind, a.id`
	if query != "" {
		orderBy = `order by a.kind, a.title, a.code, a.loinc_num`
	}
	rows, err := s.db.QueryContext(ctx, `select a.kind, a.loinc_num, coalesce(t.long_common_name, ''), coalesce(t.short_name, ''), coalesce(t.status, ''), a.code, a.title, a.subtitle, a.raw_json `+selectBase+` `+orderBy+` limit ? offset ?`, append(args, limit, offset)...)
	if err != nil {
		return AccessoryBrowseResponse{}, fmt.Errorf("browse accessory rows: %w", err)
	}
	defer rows.Close()
	results := make([]AccessoryRecord, 0, limit)
	for rows.Next() {
		var item AccessoryRecord
		var rawJSON string
		if err := rows.Scan(&item.Kind, &item.LOINCNum, &item.LongCommonName, &item.ShortName, &item.Status, &item.Code, &item.Title, &item.Subtitle, &rawJSON); err != nil {
			return AccessoryBrowseResponse{}, fmt.Errorf("scan accessory row: %w", err)
		}
		if err := json.Unmarshal([]byte(rawJSON), &item.Fields); err != nil {
			return AccessoryBrowseResponse{}, fmt.Errorf("decode accessory row: %w", err)
		}
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		return AccessoryBrowseResponse{}, fmt.Errorf("iterate accessory rows: %w", err)
	}
	response := AccessoryBrowseResponse{Results: results, Total: total, Limit: limit, Offset: offset, Query: query, Kind: kind}
	s.cache.setAccessory(cacheKey, response)
	return response, nil
}

func (s *Store) loadFacet(ctx context.Context, column string, target map[string]int) error {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(
		`select %s, count(*) from loinc_terms where %s <> '' group by %s order by count(*) desc, %s limit 500`,
		column,
		column,
		column,
		column,
	))
	if err != nil {
		return fmt.Errorf("load facet %s: %w", column, err)
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		var count int
		if err := rows.Scan(&value, &count); err != nil {
			return fmt.Errorf("scan facet %s: %w", column, err)
		}
		target[value] = count
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate facet %s: %w", column, err)
	}
	return nil
}

func filterClauses(params SearchParams, alias string) ([]string, []any) {
	var where []string
	var args []any
	add := func(column string, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		where = append(where, fmt.Sprintf("%s.%s = ?", alias, column))
		args = append(args, value)
	}
	addMany := func(column string, single string, values []string) {
		seen := map[string]bool{}
		filtered := make([]string, 0, len(values)+1)
		for _, value := range append([]string{single}, values...) {
			value = strings.TrimSpace(value)
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			filtered = append(filtered, value)
		}
		if len(filtered) == 0 {
			return
		}
		if len(filtered) == 1 {
			add(column, filtered[0])
			return
		}
		placeholders := strings.TrimRight(strings.Repeat("?,", len(filtered)), ",")
		where = append(where, fmt.Sprintf("%s.%s in (%s)", alias, column, placeholders))
		for _, value := range filtered {
			args = append(args, value)
		}
	}
	add("class", params.Class)
	add("system", params.System)
	add("property", params.Property)
	addMany("status", params.Status, params.Statuses)
	addMany("time_aspect", params.TimeAspect, params.TimeAspects)
	addMany("scale", params.Scale, params.Scales)
	addMany("method", params.Method, params.Methods)
	addMany("order_obs", params.OrderObs, params.OrderObsValues)
	if strings.TrimSpace(params.Status) == "" && len(params.Statuses) == 0 {
		where = append(where, fmt.Sprintf("%s.status <> ?", alias))
		args = append(args, "DEPRECATED")
	}
	return where, args
}

var ftsTokenRegexp = regexp.MustCompile(`[[:alnum:]]+`)
var loincNumberRegexp = regexp.MustCompile(`^\d+-\d+$`)

func makeFTSQuery(query string) string {
	tokens := ftsTokenRegexp.FindAllString(strings.ToLower(query), -1)
	if len(tokens) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if len(token) == 1 {
			parts = append(parts, token)
			continue
		}
		parts = append(parts, token+"*")
	}
	return strings.Join(parts, " ")
}
