package loinc

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type LocalSearchDocument struct {
	ID     string
	Scope  string
	Key    string
	Fields map[string]any
}

type LocalSearchHit struct {
	ID    string  `json:"id"`
	Scope string  `json:"scope"`
	Key   string  `json:"key"`
	Score float64 `json:"score"`
}

type LocalSearchResult struct {
	LocalSearchHit
	Result any `json:"result"`
}

func (s *Store) VisitLocalSearchDocuments(ctx context.Context, visit func(LocalSearchDocument) error) error {
	answerIDs, answerNames, err := s.loincAnswerListLookups(ctx)
	if err != nil {
		return err
	}
	mapTargets, err := s.loincMapToLookup(ctx)
	if err != nil {
		return err
	}
	if err := s.visitLocalLoincDocuments(ctx, answerIDs, answerNames, mapTargets, visit); err != nil {
		return err
	}
	if err := s.visitLocalPartDocuments(ctx, visit); err != nil {
		return err
	}
	if err := s.visitLocalAnswerListDocuments(ctx, visit); err != nil {
		return err
	}
	if err := s.visitLocalGroupDocuments(ctx, visit); err != nil {
		return err
	}
	return nil
}

func (s *Store) HydrateLocalSearchHits(ctx context.Context, hits []LocalSearchHit) ([]LocalSearchResult, error) {
	out := make([]LocalSearchResult, 0, len(hits))
	for _, hit := range hits {
		item := LocalSearchResult{LocalSearchHit: hit}
		switch hit.Scope {
		case "loincs":
			term, err := s.Term(ctx, hit.Key)
			if err != nil {
				return nil, err
			}
			item.Result = SearchResult{
				LOINCNum:        term.LOINCNum,
				LongCommonName:  term.LongCommonName,
				ShortName:       term.ShortName,
				Component:       term.Component,
				Property:        term.Property,
				System:          term.System,
				Scale:           term.Scale,
				Method:          term.Method,
				Class:           term.Class,
				Status:          term.Status,
				OrderObs:        term.OrderObs,
				CommonTestRank:  term.CommonTestRank,
				CommonOrderRank: term.CommonOrderRank,
				UsageTypes:      term.UsageTypes,
				Rank:            hit.Score,
				Links:           termLinks(term.LOINCNum),
			}
		case "parts":
			part, err := s.Part(ctx, hit.Key)
			if err != nil {
				return nil, err
			}
			item.Result = part
		case "answerlists":
			answerList, err := s.AnswerList(ctx, hit.Key)
			if err != nil {
				return nil, err
			}
			item.Result = answerList
		case "groups":
			group, err := s.Group(ctx, hit.Key)
			if err != nil {
				return nil, err
			}
			item.Result = group
		default:
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Store) visitLocalLoincDocuments(ctx context.Context, answerIDs map[string][]string, answerNames map[string][]string, mapTargets map[string][]string, visit func(LocalSearchDocument) error) error {
	rows, err := s.db.QueryContext(ctx, `select
		loinc_num, long_common_name, short_name, component, property, time_aspect,
		system, scale, method, class, status, definition, consumer_name, related_names,
		order_obs, display_name, common_test_rank, common_order_rank
		from loinc_terms`)
	if err != nil {
		return fmt.Errorf("load local search LOINC documents: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var term Term
		if err := rows.Scan(
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
			return fmt.Errorf("scan local search LOINC document: %w", err)
		}
		answerListIDs := answerIDs[term.LOINCNum]
		answerListNames := answerNames[term.LOINCNum]
		fields := baseLocalSearchFields("loincs", term.LOINCNum)
		put(fields, "LOINC", term.LOINCNum)
		put(fields, "Component", term.Component)
		put(fields, "Property", term.Property)
		put(fields, "Timing", term.TimeAspect)
		put(fields, "System", term.System)
		put(fields, "Scale", term.Scale)
		put(fields, "Method", term.Method)
		put(fields, "Class", term.Class)
		put(fields, "LongName", term.LongCommonName)
		put(fields, "ShortName", term.ShortName)
		put(fields, "DisplayName", term.DisplayName)
		put(fields, "Description", term.Definition)
		put(fields, "Status", term.Status)
		put(fields, "OrderObs", term.OrderObs)
		put(fields, "Rank", term.CommonTestRank)
		put(fields, "CommonOrderRank", term.CommonOrderRank)
		put(fields, "CommonOrder", term.CommonOrderRank > 0)
		put(fields, "Ranked", term.CommonTestRank > 0)
		put(fields, "CommonLabResult", term.CommonTestRank > 0)
		put(fields, "ComponentWordCount", len(strings.Fields(term.Component)))
		put(fields, "CoreComponent", coreComponent(term.Component))
		put(fields, "Methodless", strings.TrimSpace(term.Method) == "")
		put(fields, "LabTest", strings.EqualFold(term.Class, "CHEM") || strings.Contains(strings.ToUpper(term.Class), "LAB"))
		put(fields, "MassProperty", strings.HasPrefix(strings.ToUpper(term.Property), "M"))
		put(fields, "SubstanceProperty", strings.HasPrefix(strings.ToUpper(term.Property), "S"))
		put(fields, "SuperSystem", suffixAfterCaret(term.System))
		put(fields, "TimeModifier", suffixAfterCaret(term.TimeAspect))
		put(fields, "Punctuation", punctuationNames(term.Component, term.Property, term.TimeAspect, term.System, term.Scale, term.Method))
		put(fields, "AnswerList", len(answerListIDs) > 0)
		put(fields, "AnswerListId", answerListIDs)
		put(fields, "AnswerListName", answerListNames)
		put(fields, "MapToLOINC", mapTargets[term.LOINCNum])
		put(fields, "_all", strings.Join(compactStrings(
			term.LOINCNum, term.LongCommonName, term.ShortName, term.Component, term.Property,
			term.TimeAspect, term.System, term.Scale, term.Method, term.Class, term.Status,
			term.Definition, term.ConsumerName, term.RelatedNames, term.DisplayName,
			strings.Join(answerListIDs, " "), strings.Join(answerListNames, " "), strings.Join(mapTargets[term.LOINCNum], " "),
		), " "))
		if err := visit(LocalSearchDocument{ID: "loinc:" + term.LOINCNum, Scope: "loincs", Key: term.LOINCNum, Fields: fields}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate local search LOINC documents: %w", err)
	}
	return nil
}

func (s *Store) visitLocalPartDocuments(ctx context.Context, visit func(LocalSearchDocument) error) error {
	classLists, err := s.partClassListLookup(ctx)
	if err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `select part_number, part_type_name, part_name, part_display_name, status from parts`)
	if err != nil {
		return fmt.Errorf("load local search part documents: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var part Part
		if err := rows.Scan(&part.PartNumber, &part.PartTypeName, &part.PartName, &part.PartDisplayName, &part.Status); err != nil {
			return fmt.Errorf("scan local search part document: %w", err)
		}
		fields := baseLocalSearchFields("parts", part.PartNumber)
		put(fields, "Partnumber", part.PartNumber)
		put(fields, "Part", part.PartName)
		put(fields, "Name", part.PartName)
		put(fields, "DisplayName", part.PartDisplayName)
		put(fields, "Type", part.PartTypeName)
		put(fields, "Status", part.Status)
		put(fields, "ClassList", classLists[part.PartNumber])
		put(fields, "_all", strings.Join(compactStrings(part.PartNumber, part.PartName, part.PartDisplayName, part.PartTypeName, part.Status, strings.Join(classLists[part.PartNumber], " ")), " "))
		if err := visit(LocalSearchDocument{ID: "part:" + part.PartNumber, Scope: "parts", Key: part.PartNumber, Fields: fields}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate local search part documents: %w", err)
	}
	return nil
}

func (s *Store) visitLocalAnswerListDocuments(ctx context.Context, visit func(LocalSearchDocument) error) error {
	answerData, err := s.answerListLookup(ctx)
	if err != nil {
		return err
	}
	loincCounts, err := s.answerListLOINCCounts(ctx)
	if err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `select answer_list_id, answer_list_name, answer_list_oid, ext_defined_yn, ext_defined_answer_list_code_system, ext_defined_answer_list_link from answer_lists`)
	if err != nil {
		return fmt.Errorf("load local search answer-list documents: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, name, oid, extDefined, extCodeSystem, extURL string
		if err := rows.Scan(&id, &name, &oid, &extDefined, &extCodeSystem, &extURL); err != nil {
			return fmt.Errorf("scan local search answer-list document: %w", err)
		}
		data := answerData[id]
		fields := baseLocalSearchFields("answerlists", id)
		put(fields, "AnswerList", id)
		put(fields, "Name", name)
		put(fields, "LOINCAnswerListOID", oid)
		put(fields, "ExternalListURL", extURL)
		put(fields, "ExternallyDefined", truthy(extDefined))
		put(fields, "AnswerCount", data.count)
		put(fields, "LoincCount", loincCounts[id])
		put(fields, "AnswerCode", append(data.localCodes, data.extCodes...))
		put(fields, "AnswerCodeSystem", append(data.localCodeSystems, data.extCodeSystems...))
		put(fields, "CodeSystem", append(data.localCodeSystems, data.extCodeSystems...))
		put(fields, "AnswerDisplayText", data.displayText)
		put(fields, "AnswerScore", data.scores)
		put(fields, "AnswerSequenceNum", data.sequences)
		put(fields, "AnswerString", data.answerStrings)
		put(fields, "AnswerStringDescription", data.descriptions)
		put(fields, "_all", strings.Join(compactStrings(id, name, oid, extCodeSystem, extURL, strings.Join(data.displayText, " "), strings.Join(data.localCodes, " "), strings.Join(data.extCodes, " "), strings.Join(data.descriptions, " ")), " "))
		if err := visit(LocalSearchDocument{ID: "answerlist:" + id, Scope: "answerlists", Key: id, Fields: fields}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate local search answer-list documents: %w", err)
	}
	return nil
}

func (s *Store) visitLocalGroupDocuments(ctx context.Context, visit func(LocalSearchDocument) error) error {
	counts, err := s.groupLOINCCounts(ctx)
	if err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `select g.group_id, g.parent_group_id, coalesce(pg.parent_group, ''), g.group_name, g.archetype, g.status, g.version_first_released
		from loinc_groups g left join parent_groups pg on pg.parent_group_id = g.parent_group_id`)
	if err != nil {
		return fmt.Errorf("load local search group documents: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var group LOINCGroup
		var parentGroup string
		if err := rows.Scan(&group.GroupID, &group.ParentGroupID, &parentGroup, &group.GroupName, &group.Archetype, &group.Status, &group.VersionFirstReleased); err != nil {
			return fmt.Errorf("scan local search group document: %w", err)
		}
		fields := baseLocalSearchFields("groups", group.GroupID)
		put(fields, "Group", group.GroupID)
		put(fields, "GroupId", group.GroupID)
		put(fields, "Name", group.GroupName)
		put(fields, "Archetype", group.Archetype)
		put(fields, "ParentGroup", parentGroup)
		put(fields, "Status", group.Status)
		put(fields, "VersionFirstReleased", group.VersionFirstReleased)
		put(fields, "LoincCount", counts[group.GroupID])
		put(fields, "_all", strings.Join(compactStrings(group.GroupID, group.GroupName, group.Archetype, parentGroup, group.Status, group.VersionFirstReleased), " "))
		if err := visit(LocalSearchDocument{ID: "group:" + group.GroupID, Scope: "groups", Key: group.GroupID, Fields: fields}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate local search group documents: %w", err)
	}
	return nil
}

func baseLocalSearchFields(scope string, key string) map[string]any {
	return map[string]any{
		"scope": scope,
		"key":   key,
	}
}

func put(fields map[string]any, key string, value any) {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return
		}
	case []string:
		filtered := compactStrings(typed...)
		if len(filtered) == 0 {
			return
		}
		value = filtered
	case []int:
		if len(typed) == 0 {
			return
		}
	case nil:
		return
	}
	fields[key] = value
}

func compactStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func suffixAfterCaret(value string) string {
	parts := strings.SplitN(value, "^", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func coreComponent(value string) string {
	value = strings.TrimSpace(value)
	for _, sep := range []string{".", "^", "/"} {
		if idx := strings.Index(value, sep); idx > 0 {
			return strings.TrimSpace(value[:idx])
		}
	}
	return value
}

func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "y", "yes", "true", "1", "externallydefined":
		return true
	default:
		return false
	}
}

var punctuationTokens = map[rune]string{
	'&': "ampersand",
	'*': "asterisk",
	'{': "brace",
	'}': "brace",
	'^': "caret",
	':': "colon",
	'=': "equal",
	'>': "greaterthan",
	'-': "hyphen",
	'<': "lessthan",
	'(': "parenthesis",
	')': "parenthesis",
	'.': "period",
	'%': "percent",
	'+': "plus",
	';': "semicolon",
	'/': "slash",
}

func punctuationNames(values ...string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		for _, char := range value {
			if name, ok := punctuationTokens[char]; ok {
				seen[name] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func (s *Store) loincAnswerListLookups(ctx context.Context) (map[string][]string, map[string][]string, error) {
	rows, err := s.db.QueryContext(ctx, `select loinc_num, answer_list_id, answer_list_name from loinc_answer_list_links order by loinc_num, answer_list_id`)
	if err != nil {
		return nil, nil, fmt.Errorf("load answer-list lookup: %w", err)
	}
	defer rows.Close()
	ids := map[string][]string{}
	names := map[string][]string{}
	for rows.Next() {
		var loincNum, id, name string
		if err := rows.Scan(&loincNum, &id, &name); err != nil {
			return nil, nil, err
		}
		appendUnique(ids, loincNum, id)
		appendUnique(names, loincNum, name)
	}
	return ids, names, rows.Err()
}

func (s *Store) loincMapToLookup(ctx context.Context) (map[string][]string, error) {
	return groupedStringLookup(ctx, s.db, `select loinc_num, target_loinc_num from loinc_map_to order by loinc_num, target_loinc_num`)
}

func (s *Store) partClassListLookup(ctx context.Context) (map[string][]string, error) {
	return groupedStringLookup(ctx, s.db, `select distinct part_number, class from loinc_part_links l join loinc_terms t on t.loinc_num = l.loinc_num where class <> '' order by part_number, class`)
}

func (s *Store) answerListLOINCCounts(ctx context.Context) (map[string]int, error) {
	return groupedIntLookup(ctx, s.db, `select answer_list_id, count(distinct loinc_num) from loinc_answer_list_links group by answer_list_id`)
}

func (s *Store) groupLOINCCounts(ctx context.Context) (map[string]int, error) {
	return groupedIntLookup(ctx, s.db, `select group_id, count(distinct loinc_num) from group_loinc_terms group by group_id`)
}

type answerListIndexData struct {
	count            int
	localCodes       []string
	localCodeSystems []string
	extCodes         []string
	extCodeSystems   []string
	displayText      []string
	scores           []string
	sequences        []int
	answerStrings    []string
	descriptions     []string
}

func (s *Store) answerListLookup(ctx context.Context) (map[string]answerListIndexData, error) {
	rows, err := s.db.QueryContext(ctx, `select answer_list_id, answer_string_id, local_answer_code, local_answer_code_system,
		sequence_number, display_text, ext_code_id, ext_code_display_name, ext_code_system, description, score
		from answer_list_answers order by answer_list_id, sequence_number`)
	if err != nil {
		return nil, fmt.Errorf("load answer-list answer lookup: %w", err)
	}
	defer rows.Close()
	out := map[string]answerListIndexData{}
	for rows.Next() {
		var id, answerString, localCode, localCodeSystem, displayText, extCode, extDisplay, extSystem, description, score string
		var sequence int
		if err := rows.Scan(&id, &answerString, &localCode, &localCodeSystem, &sequence, &displayText, &extCode, &extDisplay, &extSystem, &description, &score); err != nil {
			return nil, err
		}
		item := out[id]
		item.count++
		item.localCodes = appendIfNotBlank(item.localCodes, localCode)
		item.localCodeSystems = appendIfNotBlank(item.localCodeSystems, localCodeSystem)
		item.extCodes = appendIfNotBlank(item.extCodes, extCode)
		item.extCodeSystems = appendIfNotBlank(item.extCodeSystems, extSystem)
		item.displayText = appendIfNotBlank(item.displayText, displayText)
		item.displayText = appendIfNotBlank(item.displayText, extDisplay)
		item.scores = appendIfNotBlank(item.scores, score)
		item.sequences = append(item.sequences, sequence)
		item.answerStrings = appendIfNotBlank(item.answerStrings, answerString)
		item.descriptions = appendIfNotBlank(item.descriptions, description)
		out[id] = item
	}
	return out, rows.Err()
}

func groupedStringLookup(ctx context.Context, db *sql.DB, query string) (map[string][]string, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		appendUnique(out, key, value)
	}
	return out, rows.Err()
}

func groupedIntLookup(ctx context.Context, db *sql.DB, query string) (map[string]int, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var key string
		var value int
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		out[key] = value
	}
	return out, rows.Err()
}

func appendUnique(values map[string][]string, key string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	for _, existing := range values[key] {
		if strings.EqualFold(existing, value) {
			return
		}
	}
	values[key] = append(values[key], value)
}

func appendIfNotBlank(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	return append(values, value)
}

var localSearchDocIDRegexp = regexp.MustCompile(`^([^:]+):(.+)$`)

func ParseLocalSearchDocID(id string) (string, string) {
	matches := localSearchDocIDRegexp.FindStringSubmatch(id)
	if len(matches) != 3 {
		return "", ""
	}
	scope := matches[1]
	switch scope {
	case "loinc":
		scope = "loincs"
	case "part":
		scope = "parts"
	case "answerlist":
		scope = "answerlists"
	case "group":
		scope = "groups"
	}
	return scope, matches[2]
}

func stringToNumber(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}
	return value
}
