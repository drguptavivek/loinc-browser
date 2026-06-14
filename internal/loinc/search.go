package loinc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
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
	if err := configureRuntimePragmas(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{
		db:    db,
		cache: newObjectCache(options.CacheEntries),
	}, nil
}

func configureRuntimePragmas(db *sql.DB) error {
	statements := []string{
		`pragma foreign_keys = on`,
		`pragma journal_mode = wal`,
		`pragma synchronous = normal`,
		`pragma temp_store = memory`,
		`pragma busy_timeout = 5000`,
		`pragma mmap_size = 268435456`,
		`pragma optimize`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return fmt.Errorf("configure runtime sqlite pragma %q: %w", statement, err)
		}
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Search(ctx context.Context, params SearchParams) (SearchResponse, error) {
	params = NormalizeTermListParams(params)
	limit := params.Limit
	offset := params.Offset

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
	rankColumn := termRankColumn(params.RankMode)
	if params.RankedOnly {
		where = append(where, "t."+rankColumn+" > 0")
	}

	base := `from loinc_terms t`
	baseArgs := []any{}
	if hierarchyNodeID := firstNonBlank(params.HierarchyNodeID, params.HierarchyCode); hierarchyNodeID != "" {
		base += ` join hierarchy_subtree_terms hst on hst.loinc_num = t.loinc_num and hst.node_id = ?`
		baseArgs = append(baseArgs, hierarchyNodeID)
	}
	if partNumber := strings.TrimSpace(params.PartNumber); partNumber != "" {
		base += ` join loinc_part_links scope_part on scope_part.loinc_num = t.loinc_num and scope_part.part_number = ? collate nocase`
		baseArgs = append(baseArgs, partNumber)
		if linkSet := strings.TrimSpace(params.PartLinkSet); linkSet != "" {
			base += ` and scope_part.link_set = ?`
			baseArgs = append(baseArgs, strings.ToLower(linkSet))
		}
	}
	if groupID := strings.TrimSpace(params.GroupID); groupID != "" {
		base += ` join group_loinc_terms scope_group on scope_group.loinc_num = t.loinc_num and scope_group.group_id = ? collate nocase`
		baseArgs = append(baseArgs, groupID)
	}
	if answerListID := strings.TrimSpace(params.AnswerListID); answerListID != "" {
		base += ` join loinc_answer_list_links scope_answer on scope_answer.loinc_num = t.loinc_num and scope_answer.answer_list_id = ? collate nocase`
		baseArgs = append(baseArgs, answerListID)
	}
	if panelParent := strings.TrimSpace(params.PanelParent); panelParent != "" {
		base += ` join panel_items scope_panel on scope_panel.child_loinc_num = t.loinc_num and scope_panel.parent_loinc_num = ? collate nocase`
		baseArgs = append(baseArgs, panelParent)
	}
	if params.PanelOnly {
		base += ` join (select distinct parent_loinc_num from panel_items) scope_panels on scope_panels.parent_loinc_num = t.loinc_num`
	}
	if ftsQuery != "" {
		base += ` join loinc_terms_fts on loinc_terms_fts.loinc_num = t.loinc_num`
	}
	if len(where) > 0 {
		base += ` where ` + strings.Join(where, " and ")
	}

	countQuery := `select count(distinct t.loinc_num) ` + base
	var total int
	countArgs := append(baseArgs, args...)
	if err := s.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return SearchResponse{}, fmt.Errorf("count search results: %w", err)
	}

	order := termOrderClause(params.Sort, rankColumn, ftsQuery != "")
	selectRank := `0.0 as rank`
	if ftsQuery != "" {
		selectRank = `bm25(loinc_terms_fts) as rank`
	}

	searchQuery := `select distinct
		t.loinc_num, t.long_common_name, t.short_name, t.component, t.property,
		t.system, t.scale, t.method, t.class, t.status, t.order_obs, t.common_test_rank,
		t.common_order_rank, ` + selectRank + ` ` + base + ` ` + order + ` limit ? offset ?`
	searchArgs := append(append(baseArgs, args...), limit, offset)
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
			&result.CommonTestRank,
			&result.CommonOrderRank,
			&result.Rank,
		); err != nil {
			return SearchResponse{}, fmt.Errorf("scan search result: %w", err)
		}
		result.UsageTypes = usageTypes(result.OrderObs)
		result.Links = termLinks(result.LOINCNum)
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
		HasMore: offset+limit < total,
		Query:   query,
		Links:   termListPageLinks("/api/v1/terms/search", params, total),
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
		order_obs, display_name, common_test_rank, common_order_rank
		from loinc_terms where loinc_num = ? collate nocase`, key)

	var term Term
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
		&term.CommonTestRank,
		&term.CommonOrderRank,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Term{}, ErrNotFound
		}
		return Term{}, fmt.Errorf("load term %s: %w", key, err)
	}
	term.UsageTypes = usageTypes(term.OrderObs)
	term.Links = termLinks(term.LOINCNum)
	term.Fields = termFields(term)
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

func termFields(term Term) map[string]string {
	return map[string]string{
		"LOINC_NUM":             term.LOINCNum,
		"COMPONENT":             term.Component,
		"PROPERTY":              term.Property,
		"TIME_ASPCT":            term.TimeAspect,
		"SYSTEM":                term.System,
		"SCALE_TYP":             term.Scale,
		"METHOD_TYP":            term.Method,
		"CLASS":                 term.Class,
		"STATUS":                term.Status,
		"CONSUMER_NAME":         term.ConsumerName,
		"RELATEDNAMES2":         term.RelatedNames,
		"SHORTNAME":             term.ShortName,
		"ORDER_OBS":             term.OrderObs,
		"LONG_COMMON_NAME":      term.LongCommonName,
		"DefinitionDescription": term.Definition,
		"DisplayName":           term.DisplayName,
		"COMMON_TEST_RANK":      strconv.Itoa(term.CommonTestRank),
		"COMMON_ORDER_RANK":     strconv.Itoa(term.CommonOrderRank),
	}
}

func (s *Store) loadTermAccessories(ctx context.Context, term *Term) error {
	term.MapTo = []MapTo{}
	term.Parts = []TermAccessory{}
	term.AnswerLists = []TermAccessory{}
	term.Panels = []TermAccessory{}
	term.Groups = []TermAccessory{}
	term.Hierarchy = []TermAccessory{}

	mapRows, err := s.db.QueryContext(ctx, `select loinc_num, target_loinc_num, comment from loinc_map_to where loinc_num = ? collate nocase order by target_loinc_num`, term.LOINCNum)
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

	partRows, err := s.db.QueryContext(ctx, `
		select l.link_set, l.part_number,
			coalesce(nullif(p.part_display_name, ''), nullif(l.part_name, ''), p.part_name) as title,
			coalesce(nullif(p.part_type_name, ''), l.part_type_name) as subtitle,
			l.part_name, l.part_code_system, l.part_type_name, l.link_type_name, l.property
		from loinc_part_links l
		left join parts p on p.part_number = l.part_number
		where l.loinc_num = ? collate nocase
		order by l.link_set, subtitle, l.part_number`, term.LOINCNum)
	if err != nil {
		return fmt.Errorf("load normalized part rows for %s: %w", term.LOINCNum, err)
	}
	defer partRows.Close()
	for partRows.Next() {
		var item TermAccessory
		var linkSet, partName, partCodeSystem, partTypeName, linkTypeName, property string
		if err := partRows.Scan(&linkSet, &item.Code, &item.Title, &item.Subtitle, &partName, &partCodeSystem, &partTypeName, &linkTypeName, &property); err != nil {
			return fmt.Errorf("scan normalized part row for %s: %w", term.LOINCNum, err)
		}
		item.Kind = "part-" + linkSet
		item.Fields = map[string]string{
			"linkSet":        linkSet,
			"partName":       partName,
			"partCodeSystem": partCodeSystem,
			"partTypeName":   partTypeName,
			"linkTypeName":   linkTypeName,
			"property":       property,
		}
		term.Parts = append(term.Parts, item)
	}
	if err := partRows.Err(); err != nil {
		return fmt.Errorf("iterate normalized part rows for %s: %w", term.LOINCNum, err)
	}

	answerRows, err := s.db.QueryContext(ctx, `
		select l.answer_list_id,
			coalesce(nullif(a.answer_list_name, ''), l.answer_list_name) as title,
			a.answer_list_oid, l.answer_list_link_type, l.applicable_context
		from loinc_answer_list_links l
		left join answer_lists a on a.answer_list_id = l.answer_list_id
		where l.loinc_num = ? collate nocase
		order by title, l.answer_list_id`, term.LOINCNum)
	if err != nil {
		return fmt.Errorf("load normalized answer list rows for %s: %w", term.LOINCNum, err)
	}
	defer answerRows.Close()
	for answerRows.Next() {
		var item TermAccessory
		var answerListOID, linkType, applicableContext string
		if err := answerRows.Scan(&item.Code, &item.Title, &answerListOID, &linkType, &applicableContext); err != nil {
			return fmt.Errorf("scan normalized answer list row for %s: %w", term.LOINCNum, err)
		}
		item.Kind = "answer-list"
		item.Subtitle = answerListOID
		item.Fields = map[string]string{
			"answerListId":       item.Code,
			"answerListOID":      answerListOID,
			"answerListLinkType": linkType,
			"applicableContext":  applicableContext,
		}
		term.AnswerLists = append(term.AnswerLists, item)
	}
	if err := answerRows.Err(); err != nil {
		return fmt.Errorf("iterate normalized answer list rows for %s: %w", term.LOINCNum, err)
	}

	panelMembershipRows, err := s.db.QueryContext(ctx, `
		select p.parent_loinc_num, p.parent_name, p.sequence, p.item_id, p.entry_type, p.data_type_in_form, coalesce(p.answer_list_id_override, ''),
			coalesce(parent.common_test_rank, 0), coalesce(parent.common_order_rank, 0),
			case
				when coalesce(parent.common_test_rank, 0) > 0 and coalesce(parent.common_order_rank, 0) > 0
					then min(parent.common_test_rank, parent.common_order_rank)
				when coalesce(parent.common_test_rank, 0) > 0 then parent.common_test_rank
				when coalesce(parent.common_order_rank, 0) > 0 then parent.common_order_rank
				else 0
			end as parent_rank
		from panel_items p
		left join loinc_terms parent on parent.loinc_num = p.parent_loinc_num
		where p.child_loinc_num = ? collate nocase
		order by case when parent_rank > 0 then 0 else 1 end, parent_rank, p.parent_name, p.parent_loinc_num, p.sequence`, term.LOINCNum)
	if err != nil {
		return fmt.Errorf("load normalized panel memberships for %s: %w", term.LOINCNum, err)
	}
	defer panelMembershipRows.Close()
	for panelMembershipRows.Next() {
		var item TermAccessory
		var sequence, parentCommonTestRank, parentCommonOrderRank, parentRank int
		var itemID, entryType, dataTypeInForm, answerListIDOverride string
		if err := panelMembershipRows.Scan(&item.Code, &item.Title, &sequence, &itemID, &entryType, &dataTypeInForm, &answerListIDOverride, &parentCommonTestRank, &parentCommonOrderRank, &parentRank); err != nil {
			return fmt.Errorf("scan normalized panel membership for %s: %w", term.LOINCNum, err)
		}
		item.Kind = "panel-membership"
		item.Subtitle = entryType
		item.Fields = map[string]string{
			"sequence":              strconv.Itoa(sequence),
			"itemId":                itemID,
			"entryType":             entryType,
			"dataTypeInForm":        dataTypeInForm,
			"answerListIdOverride":  answerListIDOverride,
			"parentCommonTestRank":  strconv.Itoa(parentCommonTestRank),
			"parentCommonOrderRank": strconv.Itoa(parentCommonOrderRank),
			"parentRank":            strconv.Itoa(parentRank),
		}
		term.Panels = append(term.Panels, item)
	}
	if err := panelMembershipRows.Err(); err != nil {
		return fmt.Errorf("iterate normalized panel memberships for %s: %w", term.LOINCNum, err)
	}

	panelItemRows, err := s.db.QueryContext(ctx, `
		select child_loinc_num, child_name, sequence, item_id, entry_type, data_type_in_form, coalesce(answer_list_id_override, '')
		from panel_items
		where parent_loinc_num = ? collate nocase
		order by sequence, child_name, child_loinc_num`, term.LOINCNum)
	if err != nil {
		return fmt.Errorf("load normalized panel items for %s: %w", term.LOINCNum, err)
	}
	defer panelItemRows.Close()
	for panelItemRows.Next() {
		var item TermAccessory
		var sequence int
		var itemID, entryType, dataTypeInForm, answerListIDOverride string
		if err := panelItemRows.Scan(&item.Code, &item.Title, &sequence, &itemID, &entryType, &dataTypeInForm, &answerListIDOverride); err != nil {
			return fmt.Errorf("scan normalized panel item for %s: %w", term.LOINCNum, err)
		}
		item.Kind = "panel-child"
		item.Subtitle = entryType
		item.Fields = map[string]string{
			"sequence":             strconv.Itoa(sequence),
			"itemId":               itemID,
			"entryType":            entryType,
			"dataTypeInForm":       dataTypeInForm,
			"answerListIdOverride": answerListIDOverride,
		}
		term.Panels = append(term.Panels, item)
	}
	if err := panelItemRows.Err(); err != nil {
		return fmt.Errorf("iterate normalized panel items for %s: %w", term.LOINCNum, err)
	}

	groupRows, err := s.db.QueryContext(ctx, `
		select g.group_id, g.group_name, g.archetype, gt.category, pg.parent_group_id, pg.parent_group
		from group_loinc_terms gt
		join loinc_groups g on g.group_id = gt.group_id
		left join parent_groups pg on pg.parent_group_id = g.parent_group_id
		where gt.loinc_num = ? collate nocase
		order by g.group_name, g.group_id`, term.LOINCNum)
	if err != nil {
		return fmt.Errorf("load normalized group rows for %s: %w", term.LOINCNum, err)
	}
	defer groupRows.Close()
	for groupRows.Next() {
		var item TermAccessory
		var category, parentGroupID, parentGroup string
		if err := groupRows.Scan(&item.Code, &item.Title, &item.Subtitle, &category, &parentGroupID, &parentGroup); err != nil {
			return fmt.Errorf("scan normalized group row for %s: %w", term.LOINCNum, err)
		}
		item.Kind = "group"
		item.Fields = map[string]string{
			"groupId":       item.Code,
			"category":      category,
			"archetype":     item.Subtitle,
			"parentGroupId": parentGroupID,
			"parentGroup":   parentGroup,
		}
		term.Groups = append(term.Groups, item)
	}
	if err := groupRows.Err(); err != nil {
		return fmt.Errorf("iterate normalized group rows for %s: %w", term.LOINCNum, err)
	}

	hierarchyRows, err := s.db.QueryContext(ctx, `
		select cast(n.node_id as text), n.path_key, c.code, c.label, coalesce(cast(n.parent_node_id as text), ''), n.depth, n.subtree_term_count
		from hierarchy_occurrences n
		join hierarchy_concepts c on c.code = n.code
		where c.loinc_num = ? collate nocase
		order by n.path_key`, term.LOINCNum)
	if err != nil {
		return fmt.Errorf("load normalized hierarchy rows for %s: %w", term.LOINCNum, err)
	}
	defer hierarchyRows.Close()
	for hierarchyRows.Next() {
		var item TermAccessory
		var nodeID, conceptCode, parentNodeID string
		var depth, subtreeTermCount int
		if err := hierarchyRows.Scan(&nodeID, &item.Code, &conceptCode, &item.Title, &parentNodeID, &depth, &subtreeTermCount); err != nil {
			return fmt.Errorf("scan normalized hierarchy row for %s: %w", term.LOINCNum, err)
		}
		item.Kind = "hierarchy"
		item.Subtitle = conceptCode
		item.Fields = map[string]string{
			"nodeId":           nodeID,
			"pathKey":          item.Code,
			"conceptCode":      conceptCode,
			"parentNodeId":     parentNodeID,
			"depth":            strconv.Itoa(depth),
			"subtreeTermCount": strconv.Itoa(subtreeTermCount),
		}
		term.Hierarchy = append(term.Hierarchy, item)
	}
	if err := hierarchyRows.Err(); err != nil {
		return fmt.Errorf("iterate normalized hierarchy rows for %s: %w", term.LOINCNum, err)
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
	outRows, err := s.db.QueryContext(ctx, `select loinc_num, target_loinc_num, comment from loinc_map_to where loinc_num = ? collate nocase order by target_loinc_num`, key)
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

	inRows, err := s.db.QueryContext(ctx, `select loinc_num, target_loinc_num, comment from loinc_map_to where target_loinc_num = ? collate nocase order by loinc_num`, key)
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

	conceptRows, err := s.db.QueryContext(ctx, `
		select distinct 'part-' || l.link_set as kind, l.part_number,
			coalesce(nullif(p.part_display_name, ''), nullif(l.part_name, ''), p.part_name) as title,
			coalesce(nullif(p.part_type_name, ''), l.part_type_name) as subtitle,
			l.link_set, l.part_type_name, l.property
		from loinc_part_links l
		left join parts p on p.part_number = l.part_number
		where l.loinc_num = ? collate nocase
		order by kind, title, l.part_number
		limit 100`, key)
	if err != nil {
		return TermRelationshipGraph{}, fmt.Errorf("load relationship concepts for %s: %w", key, err)
	}
	defer conceptRows.Close()
	seen := map[string]bool{}
	for conceptRows.Next() {
		var concept RelationshipConcept
		var linkSet, partTypeName, property string
		if err := conceptRows.Scan(&concept.Kind, &concept.Code, &concept.Title, &concept.Subtitle, &linkSet, &partTypeName, &property); err != nil {
			return TermRelationshipGraph{}, fmt.Errorf("scan relationship concept for %s: %w", key, err)
		}
		identity := concept.Kind + "\x00" + concept.Code
		if seen[identity] {
			continue
		}
		seen[identity] = true
		if concept.Code == "" {
			continue
		}
		concept.Fields = map[string]string{
			"linkSet":      linkSet,
			"partTypeName": partTypeName,
			"property":     property,
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
	linkSet := strings.TrimPrefix(concept.Kind, "part-")
	if concept.Kind == linkSet || linkSet == "" {
		return nil
	}
	if err := s.db.QueryRowContext(ctx, `select count(distinct loinc_num)
		from loinc_part_links
		where link_set = ? and part_number = ? collate nocase and loinc_num <> ? collate nocase`,
		linkSet, concept.Code, loincNum).Scan(&concept.RelatedTotal); err != nil {
		return fmt.Errorf("count related terms for %s %s: %w", concept.Kind, concept.Code, err)
	}
	rows, err := s.db.QueryContext(ctx, `select distinct
			t.loinc_num, t.long_common_name, t.short_name, t.display_name, t.status, t.order_obs,
			t.common_test_rank, t.common_order_rank, t.system, t.class, t.scale, t.property
		from loinc_part_links l
		join loinc_terms t on t.loinc_num = l.loinc_num
		where l.link_set = ? and l.part_number = ? collate nocase and l.loinc_num <> ? collate nocase
		order by case when t.common_test_rank > 0 then 0 else 1 end, t.common_test_rank, t.long_common_name
		limit 20`, linkSet, concept.Code, loincNum)
	if err != nil {
		return fmt.Errorf("load related terms for %s %s: %w", concept.Kind, concept.Code, err)
	}
	defer rows.Close()
	for rows.Next() {
		var term TermSummary
		if err := rows.Scan(
			&term.LOINCNum,
			&term.LongCommonName,
			&term.ShortName,
			&term.DisplayName,
			&term.Status,
			&term.OrderObs,
			&term.CommonTestRank,
			&term.CommonOrderRank,
			&term.System,
			&term.Class,
			&term.Scale,
			&term.Property,
		); err != nil {
			return fmt.Errorf("scan related term for %s %s: %w", concept.Kind, concept.Code, err)
		}
		term.UsageTypes = usageTypes(term.OrderObs)
		term.Links = termLinks(term.LOINCNum)
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
	rows, err := s.db.QueryContext(ctx, `select id, copyright_id, name, copyright, terms_of_use, url from source_organizations order by name`)
	if err != nil {
		return nil, fmt.Errorf("load source organizations: %w", err)
	}
	defer rows.Close()
	var items []SourceOrganization
	for rows.Next() {
		var item SourceOrganization
		if err := rows.Scan(&item.ID, &item.CopyrightID, &item.Name, &item.Copyright, &item.TermsOfUse, &item.URL); err != nil {
			return nil, fmt.Errorf("scan source organization: %w", err)
		}
		item.Fields = sourceOrganizationFields(item)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate source organizations: %w", err)
	}
	return items, nil
}

func sourceOrganizationFields(item SourceOrganization) map[string]string {
	return map[string]string{
		"ID":           item.ID,
		"COPYRIGHT_ID": item.CopyrightID,
		"NAME":         item.Name,
		"COPYRIGHT":    item.Copyright,
		"TERMS_OF_USE": item.TermsOfUse,
		"URL":          item.URL,
	}
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

	var base string
	var selectSQL string
	var orderBy string
	var args []any
	switch {
	case kind == "" || strings.HasPrefix(kind, "part-"):
		base, selectSQL, orderBy, args = normalizedPartBrowseSQL(kind, query)
	case kind == "answer-list":
		base, selectSQL, orderBy, args = normalizedAnswerListBrowseSQL(query)
	case kind == "group":
		base, selectSQL, orderBy, args = normalizedGroupBrowseSQL(query)
	case kind == "panel-membership":
		base, selectSQL, orderBy, args = normalizedPanelMembershipBrowseSQL(query)
	case kind == "panel-child":
		base, selectSQL, orderBy, args = normalizedPanelChildBrowseSQL(query)
	case kind == "hierarchy":
		base, selectSQL, orderBy, args = normalizedHierarchyBrowseSQL(query)
	default:
		response := AccessoryBrowseResponse{Results: []AccessoryRecord{}, Total: 0, Limit: limit, Offset: offset, HasMore: false, Query: query, Kind: kind, Links: accessoryPageLinks(kind, query, limit, offset, 0)}
		s.cache.setAccessory(cacheKey, response)
		return response, nil
	}

	var total int
	if err := s.db.QueryRowContext(ctx, "select count(*) "+base, args...).Scan(&total); err != nil {
		return AccessoryBrowseResponse{}, fmt.Errorf("count accessory rows: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, selectSQL+" "+base+" "+orderBy+" limit ? offset ?", append(args, limit, offset)...)
	if err != nil {
		return AccessoryBrowseResponse{}, fmt.Errorf("browse accessory rows: %w", err)
	}
	defer rows.Close()
	results := make([]AccessoryRecord, 0, limit)
	for rows.Next() {
		var item AccessoryRecord
		if err := rows.Scan(&item.Kind, &item.LOINCNum, &item.LongCommonName, &item.ShortName, &item.Status, &item.Code, &item.Title, &item.Subtitle); err != nil {
			return AccessoryBrowseResponse{}, fmt.Errorf("scan accessory row: %w", err)
		}
		item.Fields = map[string]string{}
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		return AccessoryBrowseResponse{}, fmt.Errorf("iterate accessory rows: %w", err)
	}
	response := AccessoryBrowseResponse{
		Results: results,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		HasMore: offset+limit < total,
		Query:   query,
		Kind:    kind,
		Links:   accessoryPageLinks(kind, query, limit, offset, total),
	}
	s.cache.setAccessory(cacheKey, response)
	return response, nil
}

func normalizedPartBrowseSQL(kind string, query string) (string, string, string, []any) {
	linkSet := strings.TrimPrefix(kind, "part-")
	where := []string{}
	args := []any{}
	if kind != "" && linkSet != kind {
		where = append(where, "l.link_set = ?")
		args = append(args, strings.ToLower(linkSet))
	}
	if query != "" {
		like := "%" + query + "%"
		where = append(where, `(t.loinc_num like ? collate nocase or t.long_common_name like ? collate nocase or l.part_number like ? collate nocase or l.part_name like ? collate nocase or p.part_display_name like ? collate nocase)`)
		args = append(args, like, like, like, like, like)
	}
	base := `from loinc_part_links l
		join loinc_terms t on t.loinc_num = l.loinc_num
		left join parts p on p.part_number = l.part_number`
	if len(where) > 0 {
		base += " where " + strings.Join(where, " and ")
	}
	selectSQL := `select 'part-' || l.link_set, t.loinc_num, t.long_common_name, t.short_name, t.status,
		l.part_number, coalesce(nullif(p.part_display_name, ''), nullif(l.part_name, ''), p.part_name),
		coalesce(nullif(p.part_type_name, ''), l.part_type_name)`
	return base, selectSQL, `order by l.link_set, l.part_number, t.long_common_name`, args
}

func normalizedAnswerListBrowseSQL(query string) (string, string, string, []any) {
	where, args := accessoryLikeWhere(query, "t.loinc_num", "t.long_common_name", "l.answer_list_id", "a.answer_list_name", "a.answer_list_oid")
	base := `from loinc_answer_list_links l
		join loinc_terms t on t.loinc_num = l.loinc_num
		left join answer_lists a on a.answer_list_id = l.answer_list_id`
	if len(where) > 0 {
		base += " where " + strings.Join(where, " and ")
	}
	selectSQL := `select 'answer-list', t.loinc_num, t.long_common_name, t.short_name, t.status,
		l.answer_list_id, coalesce(nullif(a.answer_list_name, ''), l.answer_list_name), a.answer_list_oid`
	return base, selectSQL, `order by a.answer_list_name, l.answer_list_id, t.long_common_name`, args
}

func normalizedGroupBrowseSQL(query string) (string, string, string, []any) {
	where, args := accessoryLikeWhere(query, "t.loinc_num", "t.long_common_name", "g.group_id", "g.group_name", "g.archetype")
	base := `from group_loinc_terms gt
		join loinc_terms t on t.loinc_num = gt.loinc_num
		join loinc_groups g on g.group_id = gt.group_id`
	if len(where) > 0 {
		base += " where " + strings.Join(where, " and ")
	}
	selectSQL := `select 'group', t.loinc_num, t.long_common_name, t.short_name, t.status,
		g.group_id, g.group_name, g.archetype`
	return base, selectSQL, `order by g.group_name, g.group_id, t.long_common_name`, args
}

func normalizedPanelMembershipBrowseSQL(query string) (string, string, string, []any) {
	where, args := accessoryLikeWhere(query, "child.loinc_num", "child.long_common_name", "parent.loinc_num", "parent.long_common_name", "p.parent_name")
	base := `from panel_items p
		join loinc_terms child on child.loinc_num = p.child_loinc_num
		join loinc_terms parent on parent.loinc_num = p.parent_loinc_num`
	if len(where) > 0 {
		base += " where " + strings.Join(where, " and ")
	}
	selectSQL := `select 'panel-membership', child.loinc_num, child.long_common_name, child.short_name, child.status,
		parent.loinc_num, coalesce(nullif(p.parent_name, ''), parent.long_common_name), p.entry_type`
	return base, selectSQL, `order by p.parent_name, p.sequence, child.long_common_name`, args
}

func normalizedPanelChildBrowseSQL(query string) (string, string, string, []any) {
	where, args := accessoryLikeWhere(query, "parent.loinc_num", "parent.long_common_name", "child.loinc_num", "child.long_common_name", "p.child_name")
	base := `from panel_items p
		join loinc_terms parent on parent.loinc_num = p.parent_loinc_num
		join loinc_terms child on child.loinc_num = p.child_loinc_num`
	if len(where) > 0 {
		base += " where " + strings.Join(where, " and ")
	}
	selectSQL := `select 'panel-child', parent.loinc_num, parent.long_common_name, parent.short_name, parent.status,
		child.loinc_num, coalesce(nullif(p.child_name, ''), child.long_common_name), p.entry_type`
	return base, selectSQL, `order by parent.long_common_name, p.sequence, child.long_common_name`, args
}

func normalizedHierarchyBrowseSQL(query string) (string, string, string, []any) {
	where, args := accessoryLikeWhere(query, "t.loinc_num", "t.long_common_name", "c.code", "c.label", "n.path_key")
	base := `from hierarchy_occurrences n
		join hierarchy_concepts c on c.code = n.code
		join loinc_terms t on t.loinc_num = c.loinc_num`
	if len(where) > 0 {
		base += " where " + strings.Join(where, " and ")
	}
	selectSQL := `select 'hierarchy', t.loinc_num, t.long_common_name, t.short_name, t.status,
		n.path_key, c.label, c.code`
	return base, selectSQL, `order by n.path_key, t.long_common_name`, args
}

func accessoryLikeWhere(query string, columns ...string) ([]string, []any) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	clauses := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns))
	like := "%" + query + "%"
	for _, column := range columns {
		clauses = append(clauses, column+" like ? collate nocase")
		args = append(args, like)
	}
	return []string{"(" + strings.Join(clauses, " or ") + ")"}, args
}

func (s *Store) HierarchyChildren(ctx context.Context, parentCode string, query string, navOnly bool) (HierarchyChildrenResponse, error) {
	parentCode = strings.TrimSpace(parentCode)
	query = strings.TrimSpace(query)
	var rows *sql.Rows
	var err error
	if query != "" {
		like := "%" + query + "%"
		rows, err = s.db.QueryContext(ctx, `
			select cast(n.node_id as text), c.code, c.label, coalesce(cast(n.parent_node_id as text), ''),
				n.path_key, n.subtree_term_count, case when c.node_kind = 'term' then 1 else 0 end,
				(select count(*) from hierarchy_edges e where e.parent_node_id = n.node_id) as child_count
			from hierarchy_occurrences n
			join hierarchy_concepts c on c.code = n.code
			where c.label like ? collate nocase or c.code like ? collate nocase or n.path_key like ? collate nocase
			order by n.subtree_term_count desc, c.label, n.path_key
			limit 100`, like, like, like)
	} else if parentCode == "" {
		rows, err = s.db.QueryContext(ctx, `
			select cast(n.node_id as text), c.code, c.label, coalesce(cast(n.parent_node_id as text), ''),
				n.path_key, n.subtree_term_count, case when c.node_kind = 'term' then 1 else 0 end,
				(select count(*) from hierarchy_edges e where e.parent_node_id = n.node_id) as child_count
			from hierarchy_occurrences n
			join hierarchy_concepts c on c.code = n.code
			where n.parent_node_id is null
			order by n.sequence, n.subtree_term_count desc, c.label
			limit 500`)
	} else {
		parentNodeID, resolveErr := s.resolveHierarchyNodeID(ctx, parentCode)
		if resolveErr != nil {
			return HierarchyChildrenResponse{}, resolveErr
		}
		rows, err = s.db.QueryContext(ctx, `
			select cast(n.node_id as text), c.code, c.label, coalesce(cast(n.parent_node_id as text), ''),
				n.path_key, n.subtree_term_count, case when c.node_kind = 'term' then 1 else 0 end,
				(select count(*) from hierarchy_edges child_edge where child_edge.parent_node_id = n.node_id) as child_count
			from hierarchy_edges edge
			join hierarchy_occurrences n on n.node_id = edge.child_node_id
			join hierarchy_concepts c on c.code = n.code
			where edge.parent_node_id = ?
			order by edge.sequence, n.subtree_term_count desc, c.label
			limit 500`, parentNodeID)
	}
	if err != nil {
		return HierarchyChildrenResponse{}, fmt.Errorf("load hierarchy children: %w", err)
	}
	defer rows.Close()
	items, err := scanHierarchyNodes(rows)
	if err != nil {
		return HierarchyChildrenResponse{}, err
	}
	return HierarchyChildrenResponse{
		ParentNodeID: parentCode,
		ParentCode:   parentCode,
		Query:        query,
		Results:      items,
		Links:        hierarchyChildrenLinks(parentCode),
	}, nil
}

func scanHierarchyNodes(rows *sql.Rows) ([]HierarchyNode, error) {
	items := []HierarchyNode{}
	for rows.Next() {
		var item HierarchyNode
		var isTerm int
		if err := rows.Scan(&item.NodeID, &item.Code, &item.Label, &item.ParentNodeID, &item.PathKey, &item.TermCount, &isTerm, &item.ChildCount); err != nil {
			return nil, fmt.Errorf("scan hierarchy child: %w", err)
		}
		item.ParentCode = item.ParentNodeID
		item.Path = item.PathKey
		item.IsTerm = isTerm != 0
		item.HasChildren = item.ChildCount > 0
		item.Links = hierarchyNodeLinks(item.NodeID)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate hierarchy children: %w", err)
	}
	return items, nil
}

func (s *Store) resolveHierarchyNodeID(ctx context.Context, key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", nil
	}
	if _, err := strconv.Atoi(key); err == nil {
		return key, nil
	}
	var nodeID string
	err := s.db.QueryRowContext(ctx, `
		select cast(n.node_id as text)
		from hierarchy_occurrences n
		where n.path_key = ? collate nocase or n.code = ? collate nocase
		order by n.depth, n.path_key
		limit 1`, key, key).Scan(&nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("resolve hierarchy node %s: %w", key, err)
	}
	return nodeID, nil
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
	statusValues, explicitStatus := normalizedStatusValues(params.Status, params.Statuses)
	if !explicitStatus {
		where = append(where, fmt.Sprintf("%s.status <> ?", alias))
		args = append(args, "INACTIVE")
	} else if len(statusValues) == 1 {
		where = append(where, fmt.Sprintf("%s.status = ?", alias))
		args = append(args, statusValues[0])
	} else if len(statusValues) > 1 {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(statusValues)), ",")
		where = append(where, fmt.Sprintf("%s.status in (%s)", alias, placeholders))
		for _, value := range statusValues {
			args = append(args, value)
		}
	}
	addMany("time_aspect", params.TimeAspect, params.TimeAspects)
	addMany("scale", params.Scale, params.Scales)
	addMany("method", params.Method, params.Methods)
	addMany("order_obs", params.OrderObs, params.OrderObsValues)
	switch strings.ToLower(strings.TrimSpace(params.UsageType)) {
	case "observation", "test":
		where = append(where, fmt.Sprintf("%s.order_obs in ('Observation', 'Both')", alias))
	case "order":
		where = append(where, fmt.Sprintf("%s.order_obs in ('Order', 'Both')", alias))
	}
	return where, args
}

func normalizedStatusValues(single string, values []string) ([]string, bool) {
	seen := map[string]bool{}
	filtered := make([]string, 0, len(values)+1)
	for _, value := range append([]string{single}, values...) {
		value = strings.ToUpper(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if value == "*" {
			return nil, true
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		filtered = append(filtered, value)
	}
	if len(filtered) == 0 {
		return nil, false
	}
	return filtered, true
}

func NormalizeTermListParams(params SearchParams) SearchParams {
	params.Query = strings.TrimSpace(params.Query)
	params.UsageType = normalizeChoice(params.UsageType, "any")
	params.RankMode = normalizeRankMode(params.RankMode)
	if strings.TrimSpace(params.Sort) == "" {
		if params.Query != "" {
			params.Sort = "relevance"
		} else {
			params.Sort = "usage"
		}
	} else {
		params.Sort = normalizeChoice(params.Sort, "relevance")
	}
	switch params.Sort {
	case "relevance", "usage", "alpha":
	default:
		params.Sort = "relevance"
	}
	if params.Limit <= 0 {
		params.Limit = 25
	}
	if params.Limit > 100 {
		params.Limit = 100
	}
	if params.Offset < 0 {
		params.Offset = 0
	}
	return params
}

func normalizeChoice(value string, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	return value
}

func normalizeRankMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "order", "orders":
		return "order"
	case "observation", "test", "tests":
		return "observation"
	default:
		return "observation"
	}
}

func termRankColumn(rankMode string) string {
	if normalizeRankMode(rankMode) == "order" {
		return "common_order_rank"
	}
	return "common_test_rank"
}

func termOrderClause(sortMode string, rankColumn string, hasFTS bool) string {
	usageOrder := fmt.Sprintf("case when t.%s > 0 then 0 else 1 end, t.%s, t.long_common_name, t.loinc_num", rankColumn, rankColumn)
	switch strings.ToLower(strings.TrimSpace(sortMode)) {
	case "alpha":
		return "order by t.long_common_name, t.loinc_num"
	case "usage":
		return "order by " + usageOrder
	case "relevance":
		if hasFTS {
			return "order by rank, " + usageOrder
		}
		return "order by " + usageOrder
	default:
		return "order by " + usageOrder
	}
}

func usageTypes(orderObs string) []string {
	switch strings.ToLower(strings.TrimSpace(orderObs)) {
	case "both":
		return []string{"order", "observation"}
	case "order":
		return []string{"order"}
	case "observation":
		return []string{"observation"}
	default:
		return []string{}
	}
}

func termLinks(loincNum string) Links {
	escaped := url.PathEscape(loincNum)
	return Links{
		"self":             "/api/v1/terms/" + escaped,
		"fit":              "/api/v1/terms/" + escaped + "/fit",
		"relationships":    "/api/v1/terms/" + escaped + "/relationships",
		"answerLists":      "/api/v1/terms/" + escaped + "/answer-lists",
		"panelMemberships": "/api/v1/terms/" + escaped + "/panel-memberships",
		"copyright":        "/api/v1/terms/" + escaped + "/copyright",
	}
}

func hierarchyNodeLinks(nodeID string) Links {
	escaped := url.PathEscape(nodeID)
	return Links{
		"self":     "/api/v1/hierarchy/nodes/" + escaped,
		"parents":  "/api/v1/hierarchy/nodes/" + escaped + "/parents",
		"children": "/api/v1/hierarchy/nodes/" + escaped + "/children",
		"terms":    "/api/v1/hierarchy/nodes/" + escaped + "/terms",
	}
}

func hierarchyChildrenLinks(parentNodeID string) Links {
	if strings.TrimSpace(parentNodeID) == "" {
		return Links{"self": "/api/v1/hierarchy/roots"}
	}
	return Links{"self": "/api/v1/hierarchy/nodes/" + url.PathEscape(parentNodeID) + "/children"}
}

func termListPageLinks(path string, params SearchParams, total int) Links {
	params = NormalizeTermListParams(params)
	links := Links{"self": termListURL(path, params, params.Offset)}
	if params.Offset+params.Limit < total {
		links["next"] = termListURL(path, params, params.Offset+params.Limit)
	}
	if params.Offset > 0 {
		prev := params.Offset - params.Limit
		if prev < 0 {
			prev = 0
		}
		links["prev"] = termListURL(path, params, prev)
	}
	return links
}

func termListURL(path string, params SearchParams, offset int) string {
	values := url.Values{}
	if params.Query != "" {
		values.Set("q", params.Query)
	}
	if params.Class != "" {
		values.Set("class", params.Class)
	}
	for _, status := range append([]string{params.Status}, params.Statuses...) {
		if strings.TrimSpace(status) != "" {
			values.Add("status", status)
		}
	}
	if params.UsageType != "" && params.UsageType != "any" {
		values.Set("usageType", params.UsageType)
	}
	if params.RankMode != "" {
		values.Set("rankMode", params.RankMode)
	}
	if params.Sort != "" {
		values.Set("sort", params.Sort)
	}
	if params.System != "" {
		values.Set("system", params.System)
	}
	if params.Property != "" {
		values.Set("property", params.Property)
	}
	if params.RankedOnly {
		values.Set("rankedOnly", "true")
	}
	if params.HierarchyNodeID != "" {
		values.Set("hierarchyNodeId", params.HierarchyNodeID)
	}
	values.Set("limit", strconv.Itoa(params.Limit))
	values.Set("offset", strconv.Itoa(offset))
	if encoded := values.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}

func accessoryPageLinks(kind string, query string, limit int, offset int, total int) Links {
	values := url.Values{}
	if kind != "" {
		values.Set("kind", kind)
	}
	if query != "" {
		values.Set("q", query)
	}
	values.Set("limit", strconv.Itoa(limit))
	values.Set("offset", strconv.Itoa(offset))
	self := "/api/v1/accessories?" + values.Encode()
	links := Links{"self": self}
	if offset+limit < total {
		values.Set("offset", strconv.Itoa(offset+limit))
		links["next"] = "/api/v1/accessories?" + values.Encode()
	}
	return links
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func containsValue(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), needle) {
			return true
		}
	}
	return false
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
