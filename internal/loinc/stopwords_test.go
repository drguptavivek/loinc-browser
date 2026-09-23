package loinc

import "testing"

func TestMakeFTSQueryDropsStopWords(t *testing.T) {
	for query, want := range map[string]string{
		"glucose for blood":  "(glucose OR glucose*) AND (blood OR blood*)",
		"Hepatitis A":        "(hepatitis OR hepatitis*) AND a",
		"for":                "(for OR for*)", // nothing else to match, so the stop word stays
		"routine of the day": "(routine OR routine*) AND (day OR day*)",
	} {
		if got := makeFTSQuery(query); got != want {
			t.Errorf("makeFTSQuery(%q) = %q, want %q", query, got, want)
		}
	}
}
