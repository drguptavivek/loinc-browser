package loinc

import "testing"

func TestMakeFTSQueryDropsStopWords(t *testing.T) {
	for query, want := range map[string]string{
		"glucose for blood":  "glucose* blood*",
		"Hepatitis A":        "hepatitis* a",
		"for":                "for*", // nothing else to match, so the stop word stays
		"routine of the day": "routine* day*",
	} {
		if got := makeFTSQuery(query); got != want {
			t.Errorf("makeFTSQuery(%q) = %q, want %q", query, got, want)
		}
	}
}
