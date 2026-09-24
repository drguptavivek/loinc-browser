package loinc

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
)

// embeddingRelatedNamesMax caps how much of RELATEDNAMES2 goes into a term's embedding text: the
// first synonyms carry the meaning, and the long tail is mostly format variants.
const embeddingRelatedNamesMax = 300

// layPhrases gives common tests the plain-language names patients and requests use, which LOINC's
// names, related names, and (algorithmic, alpha) consumer names never contain: nothing in 4548-4
// says "average blood sugar". Only meaning search uses them.
// ponytail: hand-written for ~35 top-ranked tests; extend here, then POST /api/v1/semantic/rebuild
// re-embeds just these terms. A term removed from the map keeps its old vector until a full
// rebuild (delete the embeddings file).
var layPhrases = map[string]string{
	"2345-7":  "blood sugar; blood glucose",
	"1558-6":  "fasting blood sugar; sugar test before breakfast",
	"4548-4":  "average blood sugar over the past 2 to 3 months; diabetes control test; glycated hemoglobin",
	"2093-3":  "total cholesterol",
	"13457-7": "bad cholesterol; LDL cholesterol",
	"2085-9":  "good cholesterol; HDL cholesterol",
	"2571-8":  "blood fats; triglycerides",
	"2160-0":  "kidney function test; creatinine",
	"62238-1": "kidney function; estimated GFR; eGFR",
	"98979-8": "kidney function; estimated GFR; eGFR",
	"1742-6":  "liver enzyme; liver function test; SGPT",
	"1920-8":  "liver enzyme; liver function test; SGOT",
	"1975-2":  "jaundice test; total bilirubin",
	"718-7":   "hemoglobin; anemia test; blood count",
	"777-3":   "platelets; clotting cells",
	"6690-2":  "white blood cells; white cell count; infection fighting cells",
	"58410-2": "complete blood count; CBC; full blood count",
	"57021-8": "complete blood count with differential; CBC with diff",
	"2951-2":  "sodium; salt level; electrolytes",
	"2823-3":  "potassium; electrolytes",
	"1988-5":  "inflammation marker; C-reactive protein; CRP",
	"4537-7":  "sed rate; ESR; inflammation marker",
	"30341-2": "sed rate; ESR; inflammation marker",
	"3016-3":  "thyroid test; thyroid function; TSH",
	"2276-4":  "iron stores; ferritin",
	"1989-3":  "vitamin D level; sunshine vitamin",
	"62292-8": "vitamin D level; total vitamin D",
	"5902-2":  "clotting time; blood thinner monitoring; PT",
	"6301-6":  "blood thinner monitoring; warfarin monitoring; INR",
	"2857-1":  "prostate cancer screening; PSA",
	"2106-3":  "urine pregnancy test",
	"3040-3":  "pancreas enzyme; lipase",
	"630-4":   "urine infection test; urine culture",
	"882-1":   "blood type and Rh; blood group",
	"883-9":   "blood type; blood group",
}

// LayPhraseTerms lists the terms with lay phrases, whose embedding text changes when they do.
func LayPhraseTerms() []string {
	terms := make([]string, 0, len(layPhrases))
	for num := range layPhrases {
		terms = append(terms, num)
	}
	sort.Strings(terms)
	return terms
}

// LayPhrasesVersion identifies the current lay phrases, so a meaning index can tell it is stale.
func LayPhrasesVersion() string {
	h := fnv.New64a()
	for _, num := range LayPhraseTerms() {
		fmt.Fprintf(h, "%s=%s\n", num, layPhrases[num])
	}
	return fmt.Sprintf("%x", h.Sum64())
}

// VisitEmbeddingDocuments calls visit with each term's LOINC number and the text to embed for
// meaning-based search: its long common name, its lay phrases if any, and the start of its related
// names. Deprecated terms are included; search filters them like any other status.
func (s *Store) VisitEmbeddingDocuments(ctx context.Context, visit func(loincNum, text string) error) error {
	rows, err := s.db.QueryContext(ctx, `select loinc_num, long_common_name, coalesce(related_names, '') from loinc_terms order by loinc_num`)
	if err != nil {
		return fmt.Errorf("list embedding documents: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var loincNum, name, related string
		if err := rows.Scan(&loincNum, &name, &related); err != nil {
			return fmt.Errorf("scan embedding document: %w", err)
		}
		if len(related) > embeddingRelatedNamesMax {
			related = related[:embeddingRelatedNamesMax]
		}
		text := strings.TrimSpace(name)
		if lay := layPhrases[loincNum]; lay != "" {
			text += " | " + lay
		}
		if related = strings.TrimSpace(related); related != "" {
			text += " | " + related
		}
		if err := visit(loincNum, text); err != nil {
			return err
		}
	}
	return rows.Err()
}
