package loinc

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

// MapperGuideEntry is one test's row from LOINC's Mapper's Guide to Top 2000++ US Lab Tests,
// extracted locally by scripts/extract-top2000-mapper-guide.py.
type MapperGuideEntry struct {
	ExampleUCUM string
	Comment     string
}

// mapperGuide is process-wide so it survives store reopens after an import; nil when not loaded.
var mapperGuide atomic.Pointer[map[string]MapperGuideEntry]

// LoadMapperGuide reads the extracted mapper's guide CSV (loinc_num, ..., example_ucum, ...,
// comment, ...) and annotates later search results and terms with its example unit and comment.
// A missing file is not an error: the guide is optional licensed content the user adds.
func LoadMapperGuide(path string) (int, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return 0, fmt.Errorf("read mapper guide %s: %w", path, err)
	}
	if len(records) == 0 {
		return 0, nil
	}
	column := map[string]int{}
	for i, name := range records[0] {
		column[name] = i
	}
	numCol, okNum := column["loinc_num"]
	ucumCol, okUCUM := column["example_ucum"]
	commentCol, okComment := column["comment"]
	if !okNum || !okUCUM || !okComment {
		return 0, fmt.Errorf("mapper guide %s: want loinc_num, example_ucum, and comment columns", path)
	}
	entries := make(map[string]MapperGuideEntry, len(records)-1)
	for _, record := range records[1:] {
		entry := MapperGuideEntry{ExampleUCUM: strings.TrimSpace(record[ucumCol]), Comment: strings.TrimSpace(record[commentCol])}
		if entry != (MapperGuideEntry{}) {
			entries[strings.TrimSpace(record[numCol])] = entry
		}
	}
	mapperGuide.Store(&entries)
	return len(entries), nil
}

func mapperGuideEntry(loincNum string) MapperGuideEntry {
	if entries := mapperGuide.Load(); entries != nil {
		return (*entries)[loincNum]
	}
	return MapperGuideEntry{}
}
