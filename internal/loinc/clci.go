package loinc

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
)

// clciSet is Common Lab Codes for India (CLCI), NRCeS/C-DAC's curated subset of LOINC for Indian
// labs: each code's "General Name" (the name Indian labs use) and an SQL list of the codes for the
// clci filter and ranking prior. Process-wide like the mapper's guide; nil when not loaded.
type clciSet struct {
	names  map[string]string
	inList string // "('1751-7','1798-8',...)", codes checked against loincNumberRegexp
	byName []clciEntry
}

// clciEntry is one General Name as a word set, for matching a query to the name itself.
type clciEntry struct {
	code  string
	words map[string]bool
}

var clci atomic.Pointer[clciSet]

// FindCLCIFile returns the newest common-lab-codes-for-india.csv under dataDir, either in an
// extracted release folder (common-lab-codes-for-india-YYYYMMDD/) or in common_codes/; "" if none.
func FindCLCIFile(dataDir string) string {
	matches, _ := filepath.Glob(filepath.Join(dataDir, "common-lab-codes-for-india*", "common-lab-codes-for-india.csv"))
	sort.Strings(matches)
	if len(matches) > 0 {
		return matches[len(matches)-1]
	}
	path := filepath.Join(dataDir, "common_codes", "common-lab-codes-for-india.csv")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

// LoadCLCI reads the CLCI release CSV ("General Name", "LOINC Code", FSN, LCN). A missing file is
// not an error: CLCI is optional content the user downloads from nrces.in.
func LoadCLCI(path string) (int, error) {
	if path == "" {
		return 0, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	// A spreadsheet-saved copy may start with a UTF-8 BOM, which breaks the quoted first header.
	records, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")))).ReadAll()
	if err != nil {
		return 0, fmt.Errorf("read CLCI %s: %w", path, err)
	}
	if len(records) == 0 {
		return 0, nil
	}
	header := records[0]
	if len(header) < 2 || strings.TrimSpace(header[0]) != "General Name" || strings.TrimSpace(header[1]) != "LOINC Code" {
		return 0, fmt.Errorf("CLCI %s: want \"General Name\", \"LOINC Code\" as the first columns", path)
	}
	set := clciSet{names: make(map[string]string, len(records)-1)}
	codes := make([]string, 0, len(records)-1)
	for _, record := range records[1:] {
		code, name := strings.TrimSpace(record[1]), strings.TrimSpace(record[0])
		if !loincNumberRegexp.MatchString(code) {
			continue // also keeps the SQL list below safe to inline
		}
		if _, seen := set.names[code]; !seen {
			codes = append(codes, "'"+code+"'")
		}
		set.names[code] = name
		set.byName = append(set.byName, clciEntry{code: code, words: wordSet(name)})
	}
	if len(codes) == 0 {
		return 0, nil
	}
	set.inList = "(" + strings.Join(codes, ",") + ")"
	clci.Store(&set)
	return len(set.names), nil
}

func clciName(loincNum string) string {
	if set := clci.Load(); set != nil {
		return set.names[loincNum]
	}
	return ""
}

func clciInList() string {
	if set := clci.Load(); set != nil {
		return set.inList
	}
	return ""
}

// clciNameMatchMin is how much a query must share with a General Name (shared words over all
// words of both) to pin that name's codes: "creatinine urine" = "Creatinine, Urine" (1.0), while
// "urine" alone (0.5) or "potassium serum" vs "Potassium, Blood" (0.33) do not pin.
const clciNameMatchMin = 0.75

// clciNameMatches returns the codes whose General Name matches query, best match first.
// Names are not unique ("Urea Nitrogen (BUN), Urine" has three codes), so several can match.
func clciNameMatches(query string) []string {
	set := clci.Load()
	if set == nil {
		return nil
	}
	queryWords := wordSet(query)
	if len(queryWords) == 0 {
		return nil
	}
	type match struct {
		code  string
		score float64
	}
	var matches []match
	for _, entry := range set.byName {
		shared := 0
		for word := range queryWords {
			if entry.words[word] {
				shared++
			}
		}
		if score := float64(shared) / float64(len(queryWords)+len(entry.words)-shared); score >= clciNameMatchMin {
			matches = append(matches, match{entry.code, score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score > matches[j].score })
	codes := make([]string, len(matches))
	for i, m := range matches {
		codes[i] = m.code
	}
	return codes
}

// wordSet is the lowercased words of s without stop and generic words.
func wordSet(s string) map[string]bool {
	words := map[string]bool{}
	for _, word := range DropStopWords(ftsTokenRegexp.FindAllString(strings.ToLower(s), -1)) {
		words[word] = true
	}
	return words
}
