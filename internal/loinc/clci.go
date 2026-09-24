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

// clciSet is the deployment's common-codes list: a curated LOINC subset with a locally used name
// per code (Common Lab Codes for India by default; any CSV with a LOINC code column and a name
// column works, see LoadCommonCodes). Names and an SQL list of the codes back a "common codes"
// filter and ranking prior. Process-wide like the mapper's guide; nil when not loaded.
type clciSet struct {
	label  string // e.g. "Common Lab Codes for India"
	names  map[string]string
	inList string // "('1751-7','1798-8',...)", codes checked against loincNumberRegexp
	byName []clciEntry
}

// clciEntry is one local name as a word set, for matching a query to the name itself.
type clciEntry struct {
	code  string
	words map[string]bool
}

var clci atomic.Pointer[clciSet]

// CLCILabel is the label LoadCLCI stores; clciName is only populated (kept for the external
// mapper and MCP clients) when the loaded list carries this label.
const CLCILabel = "Common Lab Codes for India"

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

// LoadCLCI reads the CLCI release CSV ("General Name", "LOINC Code", FSN, LCN) as the
// deployment's common-codes list. A missing file is not an error: CLCI is optional content the
// user downloads from nrces.in.
func LoadCLCI(path string) (int, error) {
	return LoadCommonCodes(path, CLCILabel)
}

// LoadCommonCodes reads a deployment's common-codes list: any CSV with one column of LOINC
// numbers and another column of the local name labs use for that code. It works for CLCI
// ("General Name","LOINC Code",...) and for any other deployment's list (US, AU, hospital-local)
// with different column names or order. The code column is detected by scanning data rows for
// loincNumberRegexp; the local name is the first other non-empty column of that row. The header
// row is optional and is skipped only when its own code-column cell is not a LOINC number.
// A missing file is not an error: this is optional content the deployment supplies.
func LoadCommonCodes(path, label string) (int, error) {
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
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	reader.FieldsPerRecord = -1 // header and data rows may differ in width across sources
	records, err := reader.ReadAll()
	if err != nil {
		return 0, fmt.Errorf("read common codes %s: %w", path, err)
	}
	if len(records) == 0 {
		return 0, nil
	}
	codeCol := detectLOINCColumn(records)
	if codeCol < 0 {
		return 0, fmt.Errorf("common codes %s: no column has LOINC numbers", path)
	}
	rows := records
	if len(rows) > 0 && !loincNumberRegexp.MatchString(strings.TrimSpace(cell(rows[0], codeCol))) {
		rows = rows[1:] // header row: its code cell isn't a LOINC number
	}
	set := clciSet{label: label, names: make(map[string]string, len(rows))}
	codes := make([]string, 0, len(rows))
	for _, record := range rows {
		code := strings.TrimSpace(cell(record, codeCol))
		if !loincNumberRegexp.MatchString(code) {
			continue // also keeps the SQL list below safe to inline
		}
		name := firstOtherColumn(record, codeCol)
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

// ResetCommonCodes clears the loaded common-codes list. It exists for tests outside this package
// (e.g. internal/server) that load a list and must not leak it into later tests.
func ResetCommonCodes() {
	clci.Store(nil)
}

// CommonCodesInfo returns the loaded common-codes list's label and code count, or ("", 0) when
// none is loaded.
func CommonCodesInfo() (string, int) {
	if set := clci.Load(); set != nil {
		return set.label, len(set.names)
	}
	return "", 0
}

// detectLOINCColumn returns the index of the column with the most loincNumberRegexp matches
// across rows (skipping a possible header row's mismatch), or -1 if no column has any.
func detectLOINCColumn(records [][]string) int {
	width := 0
	for _, r := range records {
		if len(r) > width {
			width = len(r)
		}
	}
	best, bestCount := -1, 0
	for col := 0; col < width; col++ {
		count := 0
		for _, r := range records {
			if loincNumberRegexp.MatchString(strings.TrimSpace(cell(r, col))) {
				count++
			}
		}
		if count > bestCount {
			best, bestCount = col, count
		}
	}
	return best
}

// firstOtherColumn returns the first non-empty column of record other than skip.
func firstOtherColumn(record []string, skip int) string {
	for i, v := range record {
		if i == skip {
			continue
		}
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// cell returns record[i], or "" when the row is shorter than i.
func cell(record []string, i int) string {
	if i < 0 || i >= len(record) {
		return ""
	}
	return record[i]
}

// clciName returns loincNum's local name from the loaded common-codes list, but only when that
// list is CLCI: the external mapper and MCP clients read clciName specifically, so a non-CLCI
// deployment list must not populate it. Use localName for the list-agnostic name.
func clciName(loincNum string) string {
	if set := clci.Load(); set != nil && set.label == CLCILabel {
		return set.names[loincNum]
	}
	return ""
}

// localName returns loincNum's name from whichever common-codes list is loaded, CLCI or not.
func localName(loincNum string) string {
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

// clciNameMatchMin is how much a query must share with a local name (shared words over all
// words of both) to pin that name's codes: "creatinine urine" = "Creatinine, Urine" (1.0), while
// "urine" alone (0.5) or "potassium serum" vs "Potassium, Blood" (0.33) do not pin.
const clciNameMatchMin = 0.75

// clciNameMatches returns the codes whose local name matches query, best match first.
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
