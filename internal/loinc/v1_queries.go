package loinc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func (s *Store) TermFit(ctx context.Context, loincNum string) (TermFit, error) {
	term, err := s.Term(ctx, loincNum)
	if err != nil {
		return TermFit{}, err
	}
	fit := TermFit{
		LOINCNum:        term.LOINCNum,
		Status:          term.Status,
		Deprecated:      strings.EqualFold(term.Status, "DEPRECATED"),
		Discouraged:     strings.EqualFold(term.Status, "DISCOURAGED"),
		Inactive:        strings.EqualFold(term.Status, "INACTIVE"),
		OrderObs:        term.OrderObs,
		UsageTypes:      term.UsageTypes,
		CommonTestRank:  term.CommonTestRank,
		CommonOrderRank: term.CommonOrderRank,
		Links:           termLinks(term.LOINCNum),
	}
	checks := []struct {
		target *bool
		query  string
		args   []any
	}{
		{&fit.HasAnswerLists, `select exists(select 1 from loinc_answer_list_links where loinc_num = ? collate nocase)`, []any{term.LOINCNum}},
		{&fit.HasPanelItems, `select exists(select 1 from panel_items where parent_loinc_num = ? collate nocase)`, []any{term.LOINCNum}},
		{&fit.HasPanelMemberships, `select exists(select 1 from panel_items where child_loinc_num = ? collate nocase)`, []any{term.LOINCNum}},
		{&fit.HasHierarchy, `select exists(select 1 from hierarchy_concepts where loinc_num = ? collate nocase)`, []any{term.LOINCNum}},
	}
	for _, check := range checks {
		var exists bool
		if err := s.db.QueryRowContext(ctx, check.query, check.args...).Scan(&exists); err != nil {
			return TermFit{}, fmt.Errorf("load term fit for %s: %w", term.LOINCNum, err)
		}
		*check.target = exists
	}
	return fit, nil
}

func (s *Store) TermRelationshipGroups(ctx context.Context, loincNum string) (TermRelationshipGroups, error) {
	term, err := s.TermWithAccessories(ctx, loincNum)
	if err != nil {
		return TermRelationshipGroups{}, err
	}
	groups := TermRelationshipGroups{
		LOINCNum:         term.LOINCNum,
		MapTo:            term.MapTo,
		MappedFrom:       []MapTo{},
		Parts:            term.Parts,
		AnswerLists:      term.AnswerLists,
		PanelMemberships: []TermAccessory{},
		PanelItems:       []TermAccessory{},
		Groups:           term.Groups,
		Hierarchy:        term.Hierarchy,
		Links:            termLinks(term.LOINCNum),
	}
	for _, panel := range term.Panels {
		switch panel.Kind {
		case "panel-membership":
			groups.PanelMemberships = append(groups.PanelMemberships, panel)
		case "panel-child":
			groups.PanelItems = append(groups.PanelItems, panel)
		}
	}
	rows, err := s.db.QueryContext(ctx, `select loinc_num, target_loinc_num, comment from loinc_map_to where target_loinc_num = ? collate nocase order by loinc_num`, term.LOINCNum)
	if err != nil {
		return TermRelationshipGroups{}, fmt.Errorf("load mapped-from rows for %s: %w", term.LOINCNum, err)
	}
	defer rows.Close()
	for rows.Next() {
		var item MapTo
		if err := rows.Scan(&item.LOINC, &item.MapTo, &item.Comment); err != nil {
			return TermRelationshipGroups{}, fmt.Errorf("scan mapped-from row for %s: %w", term.LOINCNum, err)
		}
		groups.MappedFrom = append(groups.MappedFrom, item)
	}
	if err := rows.Err(); err != nil {
		return TermRelationshipGroups{}, fmt.Errorf("iterate mapped-from rows for %s: %w", term.LOINCNum, err)
	}
	return groups, nil
}

func (s *Store) HierarchyNode(ctx context.Context, nodeID string) (HierarchyNode, error) {
	resolved, err := s.resolveHierarchyNodeID(ctx, nodeID)
	if err != nil {
		return HierarchyNode{}, err
	}
	rows, err := s.db.QueryContext(ctx, `
		select cast(n.node_id as text), c.code, c.label, coalesce(cast(n.parent_node_id as text), ''),
			n.path_key, n.subtree_term_count, case when c.node_kind = 'term' then 1 else 0 end,
			(select count(*) from hierarchy_edges e where e.parent_node_id = n.node_id) as child_count
		from hierarchy_occurrences n
		join hierarchy_concepts c on c.code = n.code
		where n.node_id = ?
		limit 1`, resolved)
	if err != nil {
		return HierarchyNode{}, fmt.Errorf("load hierarchy node %s: %w", nodeID, err)
	}
	defer rows.Close()
	items, err := scanHierarchyNodes(rows)
	if err != nil {
		return HierarchyNode{}, err
	}
	if len(items) == 0 {
		return HierarchyNode{}, ErrNotFound
	}
	return items[0], nil
}

func (s *Store) HierarchyParents(ctx context.Context, nodeID string) ([]HierarchyNode, error) {
	resolved, err := s.resolveHierarchyNodeID(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		select cast(n.node_id as text), c.code, c.label, coalesce(cast(n.parent_node_id as text), ''),
			n.path_key, n.subtree_term_count, case when c.node_kind = 'term' then 1 else 0 end,
			(select count(*) from hierarchy_edges e where e.parent_node_id = n.node_id) as child_count
		from hierarchy_closure closure
		join hierarchy_occurrences n on n.node_id = closure.ancestor_node_id
		join hierarchy_concepts c on c.code = n.code
		where closure.descendant_node_id = ? and closure.depth > 0
		order by closure.depth desc`, resolved)
	if err != nil {
		return nil, fmt.Errorf("load hierarchy parents for %s: %w", nodeID, err)
	}
	defer rows.Close()
	return scanHierarchyNodes(rows)
}

func (s *Store) SearchPanels(ctx context.Context, params SearchParams) (SearchResponse, error) {
	params.PanelOnly = true
	return s.Search(ctx, params)
}

func (s *Store) PanelItems(ctx context.Context, parentLOINC string, limit int, offset int) (Page[PanelItem], error) {
	limit, offset = normalizePage(limit, offset, 100)
	parentLOINC = strings.TrimSpace(parentLOINC)
	if _, err := s.Term(ctx, parentLOINC); err != nil {
		return Page[PanelItem]{}, err
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `select count(*) from panel_items where parent_loinc_num = ? collate nocase`, parentLOINC).Scan(&total); err != nil {
		return Page[PanelItem]{}, fmt.Errorf("count panel items for %s: %w", parentLOINC, err)
	}
	rows, err := s.db.QueryContext(ctx, `
		select p.parent_loinc_num, p.child_loinc_num, p.sequence, p.item_id, p.display_name_for_form,
			p.observation_required_in_panel, p.entry_type, p.data_type_in_form, coalesce(p.answer_list_id_override, ''),
			t.loinc_num, t.long_common_name, t.short_name, t.display_name, t.status, t.order_obs,
			t.common_test_rank, t.common_order_rank, t.system, t.class, t.scale, t.property
		from panel_items p
		join loinc_terms t on t.loinc_num = p.child_loinc_num
		where p.parent_loinc_num = ? collate nocase
		order by p.sequence, p.child_name, p.child_loinc_num
		limit ? offset ?`, parentLOINC, limit, offset)
	if err != nil {
		return Page[PanelItem]{}, fmt.Errorf("load panel items for %s: %w", parentLOINC, err)
	}
	defer rows.Close()
	items := make([]PanelItem, 0, limit)
	for rows.Next() {
		var item PanelItem
		if err := rows.Scan(
			&item.ParentLOINCNum,
			&item.ChildLOINCNum,
			&item.Sequence,
			&item.ItemID,
			&item.DisplayNameForForm,
			&item.ObservationRequired,
			&item.EntryType,
			&item.DataTypeInForm,
			&item.AnswerListIDOverride,
			&item.ChildTerm.LOINCNum,
			&item.ChildTerm.LongCommonName,
			&item.ChildTerm.ShortName,
			&item.ChildTerm.DisplayName,
			&item.ChildTerm.Status,
			&item.ChildTerm.OrderObs,
			&item.ChildTerm.CommonTestRank,
			&item.ChildTerm.CommonOrderRank,
			&item.ChildTerm.System,
			&item.ChildTerm.Class,
			&item.ChildTerm.Scale,
			&item.ChildTerm.Property,
		); err != nil {
			return Page[PanelItem]{}, fmt.Errorf("scan panel item for %s: %w", parentLOINC, err)
		}
		item.ChildTerm.UsageTypes = usageTypes(item.ChildTerm.OrderObs)
		item.ChildTerm.Links = termLinks(item.ChildTerm.LOINCNum)
		item.Links = Links{
			"childTerm": "/api/v1/terms/" + url.PathEscape(item.ChildLOINCNum),
			"parent":    "/api/v1/panels/" + url.PathEscape(item.ParentLOINCNum),
		}
		if item.AnswerListIDOverride != "" {
			item.Links["answerListOverride"] = "/api/v1/answer-lists/" + url.PathEscape(item.AnswerListIDOverride)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page[PanelItem]{}, fmt.Errorf("iterate panel items for %s: %w", parentLOINC, err)
	}
	return Page[PanelItem]{Results: items, Total: total, Limit: limit, Offset: offset, HasMore: offset+limit < total, Links: pageLinks("/api/v1/panels/"+url.PathEscape(parentLOINC)+"/items", limit, offset, total)}, nil
}

func (s *Store) SearchAnswerLists(ctx context.Context, query string, limit int, offset int) (Page[AnswerList], error) {
	limit, offset = normalizePage(limit, offset, 25)
	where := ""
	args := []any{}
	if strings.TrimSpace(query) != "" {
		like := "%" + strings.TrimSpace(query) + "%"
		where = ` where answer_list_id like ? collate nocase or answer_list_name like ? collate nocase or answer_list_oid like ? collate nocase`
		args = append(args, like, like, like)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `select count(*) from answer_lists`+where, args...).Scan(&total); err != nil {
		return Page[AnswerList]{}, fmt.Errorf("count answer lists: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `select answer_list_id, answer_list_name, answer_list_oid, ext_defined_yn from answer_lists`+where+` order by answer_list_name, answer_list_id limit ? offset ?`, append(args, limit, offset)...)
	if err != nil {
		return Page[AnswerList]{}, fmt.Errorf("search answer lists: %w", err)
	}
	defer rows.Close()
	items, err := scanAnswerLists(rows, limit)
	if err != nil {
		return Page[AnswerList]{}, err
	}
	return Page[AnswerList]{Results: items, Total: total, Limit: limit, Offset: offset, HasMore: offset+limit < total, Links: pageLinks("/api/v1/answer-lists/search", limit, offset, total)}, nil
}

func (s *Store) AnswerList(ctx context.Context, answerListID string) (AnswerList, error) {
	var item AnswerList
	err := s.db.QueryRowContext(ctx, `select answer_list_id, answer_list_name, answer_list_oid, ext_defined_yn from answer_lists where answer_list_id = ? collate nocase`, strings.TrimSpace(answerListID)).Scan(&item.AnswerListID, &item.AnswerListName, &item.AnswerListOID, &item.ExtDefinedYN)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AnswerList{}, ErrNotFound
		}
		return AnswerList{}, fmt.Errorf("load answer list %s: %w", answerListID, err)
	}
	item.Links = answerListLinks(item.AnswerListID)
	return item, nil
}

func (s *Store) AnswerListAnswers(ctx context.Context, answerListID string, limit int, offset int) (Page[AnswerListAnswer], error) {
	limit, offset = normalizePage(limit, offset, 100)
	if _, err := s.AnswerList(ctx, answerListID); err != nil {
		return Page[AnswerListAnswer]{}, err
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `select count(*) from answer_list_answers where answer_list_id = ? collate nocase`, answerListID).Scan(&total); err != nil {
		return Page[AnswerListAnswer]{}, fmt.Errorf("count answers for %s: %w", answerListID, err)
	}
	rows, err := s.db.QueryContext(ctx, `
		select answer_list_id, answer_string_id, local_answer_code, local_answer_code_system,
			sequence_number, display_text, ext_code_id, ext_code_display_name, ext_code_system, score
		from answer_list_answers
		where answer_list_id = ? collate nocase
		order by sequence_number, display_text
		limit ? offset ?`, answerListID, limit, offset)
	if err != nil {
		return Page[AnswerListAnswer]{}, fmt.Errorf("load answers for %s: %w", answerListID, err)
	}
	defer rows.Close()
	items := make([]AnswerListAnswer, 0, limit)
	for rows.Next() {
		var item AnswerListAnswer
		if err := rows.Scan(&item.AnswerListID, &item.AnswerStringID, &item.LocalAnswerCode, &item.LocalAnswerCodeSystem, &item.SequenceNumber, &item.DisplayText, &item.ExtCodeID, &item.ExtCodeDisplayName, &item.ExtCodeSystem, &item.Score); err != nil {
			return Page[AnswerListAnswer]{}, fmt.Errorf("scan answer for %s: %w", answerListID, err)
		}
		item.Links = Links{"answerList": "/api/v1/answer-lists/" + url.PathEscape(item.AnswerListID)}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page[AnswerListAnswer]{}, fmt.Errorf("iterate answers for %s: %w", answerListID, err)
	}
	return Page[AnswerListAnswer]{Results: items, Total: total, Limit: limit, Offset: offset, HasMore: offset+limit < total, Links: pageLinks("/api/v1/answer-lists/"+url.PathEscape(answerListID)+"/answers", limit, offset, total)}, nil
}

func (s *Store) TermAnswerLists(ctx context.Context, loincNum string, limit int, offset int) (Page[AnswerList], error) {
	limit, offset = normalizePage(limit, offset, 25)
	if _, err := s.Term(ctx, loincNum); err != nil {
		return Page[AnswerList]{}, err
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `select count(distinct answer_list_id) from loinc_answer_list_links where loinc_num = ? collate nocase`, loincNum).Scan(&total); err != nil {
		return Page[AnswerList]{}, fmt.Errorf("count term answer lists for %s: %w", loincNum, err)
	}
	rows, err := s.db.QueryContext(ctx, `
		select distinct a.answer_list_id, coalesce(nullif(a.answer_list_name, ''), l.answer_list_name), a.answer_list_oid, a.ext_defined_yn
		from loinc_answer_list_links l
		join answer_lists a on a.answer_list_id = l.answer_list_id
		where l.loinc_num = ? collate nocase
		order by a.answer_list_name, a.answer_list_id
		limit ? offset ?`, loincNum, limit, offset)
	if err != nil {
		return Page[AnswerList]{}, fmt.Errorf("load term answer lists for %s: %w", loincNum, err)
	}
	defer rows.Close()
	items, err := scanAnswerLists(rows, limit)
	if err != nil {
		return Page[AnswerList]{}, err
	}
	return Page[AnswerList]{Results: items, Total: total, Limit: limit, Offset: offset, HasMore: offset+limit < total, Links: pageLinks("/api/v1/terms/"+url.PathEscape(loincNum)+"/answer-lists", limit, offset, total)}, nil
}

func (s *Store) SearchParts(ctx context.Context, query string, limit int, offset int) (Page[Part], error) {
	limit, offset = normalizePage(limit, offset, 25)
	where := ""
	args := []any{}
	if strings.TrimSpace(query) != "" {
		like := "%" + strings.TrimSpace(query) + "%"
		where = ` where part_number like ? collate nocase or part_name like ? collate nocase or part_display_name like ? collate nocase or part_type_name like ? collate nocase`
		args = append(args, like, like, like, like)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `select count(*) from parts`+where, args...).Scan(&total); err != nil {
		return Page[Part]{}, fmt.Errorf("count parts: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `select part_number, part_type_name, part_name, part_display_name, status from parts`+where+` order by part_name, part_number limit ? offset ?`, append(args, limit, offset)...)
	if err != nil {
		return Page[Part]{}, fmt.Errorf("search parts: %w", err)
	}
	defer rows.Close()
	items := make([]Part, 0, limit)
	for rows.Next() {
		var item Part
		if err := rows.Scan(&item.PartNumber, &item.PartTypeName, &item.PartName, &item.PartDisplayName, &item.Status); err != nil {
			return Page[Part]{}, fmt.Errorf("scan part: %w", err)
		}
		item.Links = partLinks(item.PartNumber)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page[Part]{}, fmt.Errorf("iterate parts: %w", err)
	}
	return Page[Part]{Results: items, Total: total, Limit: limit, Offset: offset, HasMore: offset+limit < total, Links: pageLinks("/api/v1/parts/search", limit, offset, total)}, nil
}

func (s *Store) Part(ctx context.Context, partNumber string) (Part, error) {
	var item Part
	err := s.db.QueryRowContext(ctx, `select part_number, part_type_name, part_name, part_display_name, status from parts where part_number = ? collate nocase`, strings.TrimSpace(partNumber)).Scan(&item.PartNumber, &item.PartTypeName, &item.PartName, &item.PartDisplayName, &item.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Part{}, ErrNotFound
		}
		return Part{}, fmt.Errorf("load part %s: %w", partNumber, err)
	}
	item.Links = partLinks(item.PartNumber)
	return item, nil
}

func (s *Store) SearchGroups(ctx context.Context, query string, limit int, offset int) (Page[LOINCGroup], error) {
	limit, offset = normalizePage(limit, offset, 25)
	where := ""
	args := []any{}
	if strings.TrimSpace(query) != "" {
		like := "%" + strings.TrimSpace(query) + "%"
		where = ` where g.group_id like ? collate nocase or g.group_name like ? collate nocase or g.archetype like ? collate nocase or pg.parent_group like ? collate nocase`
		args = append(args, like, like, like, like)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `select count(*) from loinc_groups g left join parent_groups pg on pg.parent_group_id = g.parent_group_id`+where, args...).Scan(&total); err != nil {
		return Page[LOINCGroup]{}, fmt.Errorf("count groups: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `select g.group_id, g.parent_group_id, g.group_name, g.archetype, g.status, g.version_first_released from loinc_groups g left join parent_groups pg on pg.parent_group_id = g.parent_group_id`+where+` order by g.group_name, g.group_id limit ? offset ?`, append(args, limit, offset)...)
	if err != nil {
		return Page[LOINCGroup]{}, fmt.Errorf("search groups: %w", err)
	}
	defer rows.Close()
	items := make([]LOINCGroup, 0, limit)
	for rows.Next() {
		var item LOINCGroup
		if err := rows.Scan(&item.GroupID, &item.ParentGroupID, &item.GroupName, &item.Archetype, &item.Status, &item.VersionFirstReleased); err != nil {
			return Page[LOINCGroup]{}, fmt.Errorf("scan group: %w", err)
		}
		item.Links = groupLinks(item.GroupID)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return Page[LOINCGroup]{}, fmt.Errorf("iterate groups: %w", err)
	}
	return Page[LOINCGroup]{Results: items, Total: total, Limit: limit, Offset: offset, HasMore: offset+limit < total, Links: pageLinks("/api/v1/groups/search", limit, offset, total)}, nil
}

func (s *Store) Group(ctx context.Context, groupID string) (LOINCGroup, error) {
	var item LOINCGroup
	err := s.db.QueryRowContext(ctx, `select group_id, parent_group_id, group_name, archetype, status, version_first_released from loinc_groups where group_id = ? collate nocase`, strings.TrimSpace(groupID)).Scan(&item.GroupID, &item.ParentGroupID, &item.GroupName, &item.Archetype, &item.Status, &item.VersionFirstReleased)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LOINCGroup{}, ErrNotFound
		}
		return LOINCGroup{}, fmt.Errorf("load group %s: %w", groupID, err)
	}
	item.Links = groupLinks(item.GroupID)
	return item, nil
}

func (s *Store) SourceOrganization(ctx context.Context, id string) (SourceOrganization, error) {
	var item SourceOrganization
	err := s.db.QueryRowContext(ctx, `select id, copyright_id, name, copyright, terms_of_use, url from source_organizations where id = ? collate nocase`, strings.TrimSpace(id)).Scan(&item.ID, &item.CopyrightID, &item.Name, &item.Copyright, &item.TermsOfUse, &item.URL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SourceOrganization{}, ErrNotFound
		}
		return SourceOrganization{}, fmt.Errorf("load source organization %s: %w", id, err)
	}
	item.Fields = sourceOrganizationFields(item)
	return item, nil
}

func (s *Store) TermCopyright(ctx context.Context, loincNum string) (TermCopyright, error) {
	term, err := s.Term(ctx, loincNum)
	if err != nil {
		return TermCopyright{}, err
	}
	return TermCopyright{
		LOINCNum:             term.LOINCNum,
		Status:               term.Status,
		HasExternalCopyright: false,
		State:                "unknown",
		SourceOrganizations:  []SourceOrganization{},
		Links: Links{
			"term": "/api/v1/terms/" + url.PathEscape(term.LOINCNum),
			"self": "/api/v1/terms/" + url.PathEscape(term.LOINCNum) + "/copyright",
		},
	}, nil
}

func scanAnswerLists(rows *sql.Rows, limit int) ([]AnswerList, error) {
	items := make([]AnswerList, 0, limit)
	for rows.Next() {
		var item AnswerList
		if err := rows.Scan(&item.AnswerListID, &item.AnswerListName, &item.AnswerListOID, &item.ExtDefinedYN); err != nil {
			return nil, fmt.Errorf("scan answer list: %w", err)
		}
		item.Links = answerListLinks(item.AnswerListID)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate answer lists: %w", err)
	}
	return items, nil
}

func normalizePage(limit int, offset int, defaultLimit int) (int, int) {
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func pageLinks(path string, limit int, offset int, total int) PageLinks {
	self := path + "?limit=" + strconv.Itoa(limit) + "&offset=" + strconv.Itoa(offset)
	links := PageLinks{Self: self}
	if offset+limit < total {
		links.Next = path + "?limit=" + strconv.Itoa(limit) + "&offset=" + strconv.Itoa(offset+limit)
	}
	if offset > 0 {
		prev := offset - limit
		if prev < 0 {
			prev = 0
		}
		links.Prev = path + "?limit=" + strconv.Itoa(limit) + "&offset=" + strconv.Itoa(prev)
	}
	return links
}

func answerListLinks(answerListID string) Links {
	escaped := url.PathEscape(answerListID)
	return Links{
		"self":    "/api/v1/answer-lists/" + escaped,
		"answers": "/api/v1/answer-lists/" + escaped + "/answers",
		"terms":   "/api/v1/answer-lists/" + escaped + "/terms",
	}
}

func partLinks(partNumber string) Links {
	escaped := url.PathEscape(partNumber)
	return Links{
		"self":  "/api/v1/parts/" + escaped,
		"terms": "/api/v1/parts/" + escaped + "/terms",
	}
}

func groupLinks(groupID string) Links {
	escaped := url.PathEscape(groupID)
	return Links{
		"self":  "/api/v1/groups/" + escaped,
		"terms": "/api/v1/groups/" + escaped + "/terms",
	}
}
