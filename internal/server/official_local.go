package server

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"loinc-browser/internal/loinc"
)

var loincNumberPattern = regexp.MustCompile(`(?i)\b\d{1,7}-\d\b`)

func (a *app) attachOfficialLocalMatches(ctx context.Context, response *OfficialSearchResponse) {
	local := &OfficialLocalIntegration{
		Available: false,
		LOINCNums: []string{},
		Matches:   map[string]OfficialLocalMatch{},
	}
	response.Local = local

	loincNums := officialPayloadLOINCNums(response.Payload)
	local.LOINCNums = loincNums
	if len(loincNums) == 0 {
		local.Message = "official response did not include LOINC numbers to match locally"
		return
	}

	store, err := a.currentStore()
	if err != nil {
		local.Message = "local LOINC database is not loaded"
		for _, loincNum := range loincNums {
			local.Matches[loincNum] = OfficialLocalMatch{LOINCNum: loincNum, Found: false}
		}
		local.Missing = len(loincNums)
		return
	}

	local.Available = true
	for _, loincNum := range loincNums {
		term, err := store.Term(ctx, loincNum)
		if err != nil {
			local.Matches[loincNum] = OfficialLocalMatch{LOINCNum: loincNum, Found: false}
			local.Missing++
			continue
		}
		local.Matches[loincNum] = OfficialLocalMatch{
			LOINCNum: loincNum,
			Found:    true,
			Term:     officialLocalTermSummary(term),
			LocalURL: "/api/v1/terms/" + term.LOINCNum,
		}
		local.Matched++
	}
	if local.Matched == 0 && local.Missing == 0 {
		local.Message = "no local matches were checked"
	}
}

func officialPayloadLOINCNums(payload any) []string {
	seen := map[string]struct{}{}
	var out []string
	var visit func(any)
	visit = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				if isLikelyLOINCField(key) {
					for _, loincNum := range loincNumbersFromValue(child) {
						if _, ok := seen[loincNum]; !ok {
							seen[loincNum] = struct{}{}
							out = append(out, loincNum)
						}
					}
					continue
				}
				visit(child)
			}
		case []any:
			for _, child := range typed {
				visit(child)
			}
		case []map[string]any:
			for _, child := range typed {
				visit(child)
			}
		}
	}
	visit(payload)
	sort.Strings(out)
	return out
}

func isLikelyLOINCField(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(key))
	switch normalized {
	case "loinc", "loincnum", "loincnumber", "loincode", "loinccode", "code":
		return true
	default:
		return strings.Contains(normalized, "loinc")
	}
}

func loincNumbersFromValue(value any) []string {
	switch typed := value.(type) {
	case string:
		return normalizeLOINCNumbers(loincNumberPattern.FindAllString(typed, -1))
	case []any:
		var out []string
		for _, item := range typed {
			out = append(out, loincNumbersFromValue(item)...)
		}
		return normalizeLOINCNumbers(out)
	case map[string]any:
		var out []string
		for _, child := range typed {
			out = append(out, loincNumbersFromValue(child)...)
		}
		return normalizeLOINCNumbers(out)
	default:
		return nil
	}
}

func normalizeLOINCNumbers(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToUpper(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func officialLocalTermSummary(term loinc.Term) *OfficialLocalTermSummary {
	return &OfficialLocalTermSummary{
		LOINCNum:       term.LOINCNum,
		LongCommonName: term.LongCommonName,
		ShortName:      term.ShortName,
		Status:         term.Status,
		System:         term.System,
		Class:          term.Class,
		Property:       term.Property,
		Scale:          term.Scale,
	}
}
