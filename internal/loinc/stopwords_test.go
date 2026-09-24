package loinc

import "testing"

func TestMakeFTSQueryDropsStopWords(t *testing.T) {
	for query, want := range map[string]string{
		"glucose for blood":  "(glucose OR glucose*) AND (blood OR blood* OR serum OR plasma)",
		"Hepatitis A":        "(hepatitis OR hepatitis*) AND a",
		"for":                "(for OR for*)", // nothing else to match, so the stop word stays
		"routine of the day": "(day OR day*)",
		"potassium test":     "(potassium OR potassium*)",
		"usg abdomen":        "(us) AND (abdomen OR abdomen*)",
		"esr":                `("sed rat" OR "sedimentation rate")`,
		"ncct neck":          `(ct AND "wo contrast") AND (neck OR neck*)`,
	} {
		if got := makeFTSQuery(query); got != want {
			t.Errorf("makeFTSQuery(%q) = %q, want %q", query, got, want)
		}
	}
}
