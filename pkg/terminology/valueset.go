package terminology

import (
	"context"
	"strings"

	"loinc-browser/internal/loinc"
)

// embedThreshold is the compose.include.concept embedding cutoff (§4.6.3): a ValueSet's
// definition embeds its full concept list when it has this many members or fewer; larger sets
// emit the intensional filter form instead.
const embedThreshold = 10000

// ValueSetIncludeConcept is one compose.include.concept entry.
type ValueSetIncludeConcept struct {
	Code    string `json:"code"`
	Display string `json:"display,omitempty"`
}

// ValueSetIncludeFilter is one compose.include.filter entry (§4.7.1).
type ValueSetIncludeFilter struct {
	Property string `json:"property"`
	Op       string `json:"op"`
	Value    string `json:"value"`
}

// ValueSetInclude is one compose.include or compose.exclude entry.
type ValueSetInclude struct {
	System   string                   `json:"system,omitempty"`
	Concept  []ValueSetIncludeConcept `json:"concept,omitempty"`
	Filter   []ValueSetIncludeFilter  `json:"filter,omitempty"`
	ValueSet []string                 `json:"valueSet,omitempty"`
}

// ValueSetCompose is ValueSet.compose.
type ValueSetCompose struct {
	Include []ValueSetInclude `json:"include,omitempty"`
	Exclude []ValueSetInclude `json:"exclude,omitempty"`
}

// ExpansionParameter is one expansion.parameter entry (echoes effective offset/count, §4.7).
type ExpansionParameter struct {
	Name         string `json:"name"`
	ValueInteger *int   `json:"valueInteger,omitempty"`
}

// ExpansionDesignation is one expansion.contains[].designation entry (§4.7 includeDesignations).
type ExpansionDesignation struct {
	Language string  `json:"language,omitempty"`
	Use      *Coding `json:"use,omitempty"`
	Value    string  `json:"value"`
}

// ExpansionContains is one expansion.contains entry.
type ExpansionContains struct {
	System      string                 `json:"system,omitempty"`
	Code        string                 `json:"code"`
	Display     string                 `json:"display,omitempty"`
	Inactive    bool                   `json:"inactive,omitempty"`
	Designation []ExpansionDesignation `json:"designation,omitempty"`
}

// Expansion is ValueSet.expansion (§4.7).
type Expansion struct {
	ID         string               `json:"id,omitempty"`
	Identifier string               `json:"identifier,omitempty"`
	Timestamp  string               `json:"timestamp"`
	Total      int                  `json:"total"`
	Offset     int                  `json:"offset"`
	Parameter  []ExpansionParameter `json:"parameter,omitempty"`
	Contains   []ExpansionContains  `json:"contains,omitempty"`
}

// ValueSet is the served ValueSet resource (§4.6, §4.7).
type ValueSet struct {
	ResourceType string           `json:"resourceType"`
	ID           string           `json:"id"`
	URL          string           `json:"url"`
	Identifier   []Identifier     `json:"identifier,omitempty"`
	Version      string           `json:"version"`
	Name         string           `json:"name,omitempty"`
	Status       string           `json:"status"`
	Experimental bool             `json:"experimental,omitempty"`
	Publisher    string           `json:"publisher"`
	Contact      []ContactDetail  `json:"contact,omitempty"`
	Description  string           `json:"description,omitempty"`
	Copyright    string           `json:"copyright"`
	Compose      *ValueSetCompose `json:"compose,omitempty"`
	Expansion    *Expansion       `json:"expansion,omitempty"`
}

func valueSetContact() []ContactDetail {
	return []ContactDetail{{Name: loincPublisher, Telecom: []ContactPoint{{System: "url", Value: loincSystem}}}}
}

// composeForm says how a resolved ValueSet's compose section is rendered (§4.6.2, §4.6.3).
type composeForm int

const (
	composeConcept   composeForm = iota // embed compose.include.concept (count decides, or forced)
	composeFilter                       // emit compose.include.filter (always intensional)
	composeValueSets                    // emit compose.include.valueSet[] (parent-group ValueSets)
)

// resolvedValueSet is what the catalogue produces for a served ValueSet id or an inline POSTed
// compose (§4.6.1, §4.7.1): its metadata plus how to compute its members. Exactly one of
// termSource or answerListID is set; answer lists (LL) use their own dedicated table because
// their order is by sequence, not any expression over loinc_terms.
type resolvedValueSet struct {
	id           string
	url          string
	name         string
	description  string
	identifier   []Identifier
	experimental bool

	form      composeForm
	filterOut []ValueSetIncludeFilter // rendered when form == composeFilter
	valueSets []string                // rendered when form == composeValueSets (child group URLs)

	termSource   *loinc.FHIRTermValueSetSource // nil when answerListID is set
	answerListID string
}

// cacheKey is the loinc.Store.CachedCountTermSource/CachedEmbedTermSource key for this
// resolvedValueSet's term source, or "" when its membership can vary between calls with the same
// id and must never be cached: today only resolveInlineValueSet's POSTed compose, whose r.id is
// always the constant "inline" regardless of the actual compose. Every other id (a named catalog
// entry, or an LG/LP code) is stable for the life of this Store.
func (r *resolvedValueSet) cacheKey() string {
	if r.id == "inline" {
		return ""
	}
	return r.id
}

// namedValueSet is one statically-known catalogue entry (§4.6.1), everything except the dynamic
// LL/LG/implicit-LP sets.
type namedValueSet struct {
	id           string
	name         string
	description  string
	identifier   []Identifier
	experimental bool
	forceFilter  bool // always intensional even when small (loinc-all, loinc-top-ranked)
	filterOut    []ValueSetIncludeFilter
	source       func(ctx context.Context, store *loinc.Store) loinc.FHIRTermValueSetSource
}

func namedValueSets() []namedValueSet {
	return []namedValueSet{
		{
			id:          "loinc-all",
			name:        "LOINC codes",
			description: "This value set includes every LOINC code.",
			forceFilter: true,
			source: func(_ context.Context, _ *loinc.Store) loinc.FHIRTermValueSetSource {
				return loinc.FHIRTermValueSetSource{From: "loinc_terms t", Where: "1=1", NoDuplicates: true}
			},
		},
		{
			id:          "loinc-top-ranked",
			name:        "LOINC Top Ranked Tests",
			description: "This value set includes the LOINC codes ranked by test order volume (COMMON_TEST_RANK > 0).",
			forceFilter: true,
			filterOut:   []ValueSetIncludeFilter{{Property: "COMMON_TEST_RANK", Op: "=", Value: ">0"}},
			source: func(_ context.Context, _ *loinc.Store) loinc.FHIRTermValueSetSource {
				return loinc.FHIRTermValueSetSource{From: "loinc_terms t", Where: "t.common_test_rank > 0", NoDuplicates: true}
			},
		},
		{
			id:           "deprecated-loinc-terms",
			name:         "Deprecated LOINC Terms",
			description:  "This value set contains all LOINC terms that have a Status of \"Deprecated\". In LOINC, this status means that the concept should not be used, but it is retained in LOINC for historical purposes.",
			experimental: true,
			identifier:   []Identifier{{System: loincSystem, Value: "deprecated-loinc-terms"}, {System: "urn:ietf:rfc:3986", Value: "urn:oid:1.3.6.1.4.1.12009.10.2.10"}},
			source: func(_ context.Context, _ *loinc.Store) loinc.FHIRTermValueSetSource {
				return loinc.FHIRTermValueSetSource{From: "loinc_terms t", Where: "t.status = 'DEPRECATED'", NoDuplicates: true}
			},
		},
		{
			id:          "loinc-document-ontology",
			name:        "LOINC Document Ontology",
			description: "A value set of LOINC codes from the Document Ontology.",
			identifier:  []Identifier{{System: loincSystem, Value: "loinc-document-ontology"}},
			source: func(ctx context.Context, store *loinc.Store) loinc.FHIRTermValueSetSource {
				return rawJoinSource(ctx, store, "AccessoryFiles/DocumentOntology/DocumentOntology.csv", "LoincNumber")
			},
		},
		{
			id:          "loinc-rsna-radiology-playbook",
			name:        "LOINC RSNA Radiology Playbook",
			description: "A value set of LOINC codes from the RSNA Radiology Playbook.",
			identifier:  []Identifier{{System: loincSystem, Value: "loinc-rsna-radiology-playbook"}},
			source: func(ctx context.Context, store *loinc.Store) loinc.FHIRTermValueSetSource {
				return rawJoinSource(ctx, store, "AccessoryFiles/LoincRsnaRadiologyPlaybook/LoincRsnaRadiologyPlaybook.csv", "LoincNumber")
			},
		},
		{
			id:           "top-lab-orders",
			name:         "Universal laboratory order codes",
			description:  "This value set represents the top LOINC codes as ranked by laboratory order volume. The collection references values in the COMMON_ORDER_RANK CodeSystem property. See details at https://loinc.org/usage/orders/.",
			experimental: true,
			identifier:   []Identifier{{System: loincSystem, Value: "top-lab-orders"}, {System: "urn:ietf:rfc:3986"}},
			source: func(_ context.Context, _ *loinc.Store) loinc.FHIRTermValueSetSource {
				return loinc.FHIRTermValueSetSource{From: "loinc_terms t", Where: "t.common_order_rank > 0", EmbedOrder: "t.common_order_rank", NoDuplicates: true}
			},
		},
		{
			id:           "valid-hl7-attachment-requests",
			name:         "Valid HL7 Attachment Requests",
			description:  "This value set includes LOINC terms that can be sent by a payer as part of an HL7 attachment request for additional information. This set includes 1) the preferred (generic) document codes that have clinically-relevant HL7 implementation guides, and 2) the HL7 Attachments Work Group -approved document codes that don't yet have an implementation guide.",
			experimental: true,
			identifier:   []Identifier{{System: loincSystem, Value: "valid-hl7-attachment-requests"}, {System: "urn:ietf:rfc:3986", Value: "urn:oid:1.3.6.1.4.1.12009.10.2.6"}},
			filterOut:    []ValueSetIncludeFilter{{Property: "ValidHL7AttachmentRequest", Op: "=", Value: "Y"}},
			source: func(ctx context.Context, store *loinc.Store) loinc.FHIRTermValueSetSource {
				return rawColumnFilterSource(ctx, store, `trim(r."ValidHL7AttachmentRequest") = 'Y'`)
			},
		},
		{
			id:          "valid-hl7-attachment-responses",
			name:        "Valid HL7 Attachment Responses",
			description: "This value set includes LOINC terms that have an HL7 Attachment structure defined (with or without an implementation guide).",
			filterOut:   []ValueSetIncludeFilter{{Property: "HL7_ATTACHMENT_STRUCTURE", Op: "=", Value: ">present"}},
			source: func(ctx context.Context, store *loinc.Store) loinc.FHIRTermValueSetSource {
				return rawColumnFilterSource(ctx, store, `trim(coalesce(r."HL7_ATTACHMENT_STRUCTURE",'')) <> ''`)
			},
		},
		{
			id:          "valid-hl7-attachment-responses-ig-exists",
			name:        "Valid HL7 Attachment Responses (implementation guide exists)",
			description: "This value set includes LOINC terms with an HL7 Attachment structure whose implementation guide exists.",
			filterOut:   []ValueSetIncludeFilter{{Property: "HL7_ATTACHMENT_STRUCTURE", Op: "=", Value: "IG exists"}},
			source: func(ctx context.Context, store *loinc.Store) loinc.FHIRTermValueSetSource {
				return rawColumnFilterSource(ctx, store, `trim(coalesce(r."HL7_ATTACHMENT_STRUCTURE",'')) = 'IG exists'`)
			},
		},
		{
			id:          "valid-hl7-attachment-responses-no-ig-exists",
			name:        "Valid HL7 Attachment Responses (no implementation guide)",
			description: "This value set includes LOINC terms with an HL7 Attachment structure whose implementation guide does not yet exist.",
			filterOut:   []ValueSetIncludeFilter{{Property: "HL7_ATTACHMENT_STRUCTURE", Op: "=", Value: "No IG exists"}},
			source: func(ctx context.Context, store *loinc.Store) loinc.FHIRTermValueSetSource {
				return rawColumnFilterSource(ctx, store, `trim(coalesce(r."HL7_ATTACHMENT_STRUCTURE",'')) = 'No IG exists'`)
			},
		},
		{
			id:          "loinc-universal-order-set",
			name:        "LOINC Universal Lab Orders Value Set",
			description: "A value set of LOINC codes from the LOINC Universal Lab Orders Value Set.",
			identifier:  []Identifier{{System: loincSystem, Value: "loinc-universal-order-set"}},
			source: func(ctx context.Context, store *loinc.Store) loinc.FHIRTermValueSetSource {
				return rawJoinSource(ctx, store, "AccessoryFiles/LoincUniversalLabOrdersValueSet/LoincUniversalLabOrdersValueSet.csv", "LOINC_NUM")
			},
		},
		{
			id:          "loinc-imaging-document-codes",
			name:        "LOINC Imaging Document Codes",
			description: "A value set of LOINC codes for imaging documents.",
			identifier:  []Identifier{{System: loincSystem, Value: "loinc-imaging-document-codes"}},
			source: func(ctx context.Context, store *loinc.Store) loinc.FHIRTermValueSetSource {
				return rawJoinSource(ctx, store, "AccessoryFiles/ImagingDocuments/ImagingDocumentCodes.csv", "LOINC_NUM")
			},
		},
	}
}

// rawJoinSource builds a term source over loinc_terms joined to a raw release CSV table on
// loincColumn, deduped and ordered by that table's first-seen row order for embedding (§3, §4.6.1
// document-ontology/playbook/universal-order-set/imaging-document-codes). ok reports whether the
// backing raw table exists (RawTable may be absent on an older DB, §3).
func rawJoinSource(ctx context.Context, store *loinc.Store, relPath, loincColumn string) loinc.FHIRTermValueSetSource {
	table, ok := store.RawTable(ctx, relPath)
	if !ok {
		return loinc.FHIRTermValueSetSource{From: "loinc_terms t", Where: "0=1"}
	}
	// LOINC numbers have no letters, so "collate nocase" here was a no-op on values but still
	// stopped SQLite using a plain index on the join column, forcing a nested-loop scan of the
	// whole raw table per candidate term (~135ms measured for the RSNA playbook set with
	// count=3). EnsureRawIndex makes the join an indexed lookup instead.
	_ = store.EnsureRawIndex(ctx, table, loincColumn)
	quoted := quoteSQLIdent(table)
	quotedCol := quoteSQLIdent(loincColumn)
	return loinc.FHIRTermValueSetSource{
		From:       "loinc_terms t join " + quoted + " r on r." + quotedCol + " = t.loinc_num",
		Where:      "1=1",
		EmbedOrder: `r."_row_number"`,
	}
}

// rawColumnFilterSource builds a term source over loinc_terms joined to the raw Loinc.csv table,
// filtered by a WHERE clause referencing that joined alias r (§4.6.1 HL7-attachment sets).
func rawColumnFilterSource(ctx context.Context, store *loinc.Store, whereOnR string) loinc.FHIRTermValueSetSource {
	table, ok := store.RawTable(ctx, "LoincTable/Loinc.csv")
	if !ok {
		return loinc.FHIRTermValueSetSource{From: "loinc_terms t", Where: "0=1"}
	}
	// See rawJoinSource: "collate nocase" is a no-op on all-numeric LOINC_NUM values but defeats
	// a plain index on the join column (~88ms measured for valid-hl7-attachment-requests).
	_ = store.EnsureRawIndex(ctx, table, "LOINC_NUM")
	return loinc.FHIRTermValueSetSource{
		From:       "loinc_terms t join " + quoteSQLIdent(table) + ` r on r."LOINC_NUM" = t.loinc_num`,
		Where:      whereOnR,
		EmbedOrder: `r."_row_number"`,
	}
}

func quoteSQLIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

// resolveNamedValueSet looks up id among the static catalogue entries.
func resolveNamedValueSet(ctx context.Context, store *loinc.Store, id string) (*resolvedValueSet, bool) {
	for _, n := range namedValueSets() {
		if n.id != id {
			continue
		}
		src := n.source(ctx, store)
		resolved := &resolvedValueSet{
			id: n.id, url: loincSystem + "/vs/" + n.id, name: n.name, description: n.description,
			identifier: n.identifier, experimental: n.experimental, termSource: &src,
		}
		if n.forceFilter {
			resolved.form = composeFilter
			resolved.filterOut = n.filterOut
		}
		return resolved, true
	}
	return nil, false
}

// idFromURL strips the "http://loinc.org/vs/" prefix from a canonical ValueSet URL, or returns
// "" if url is not under that base (§4.6.1). The bare "http://loinc.org/vs" (all LOINC) maps to
// "loinc-all".
func idFromURL(url string) string {
	url = strings.TrimSpace(url)
	if strings.EqualFold(url, loincSystem+"/vs") {
		return "loinc-all"
	}
	prefix := loincSystem + "/vs/"
	if len(url) > len(prefix) && strings.EqualFold(url[:len(prefix)], prefix) {
		return url[len(prefix):]
	}
	return ""
}

// resolveValueSet resolves a ValueSet id (or the id extracted from a canonical url) to its
// catalogue definition (§4.6.1). notFoundText names the id/url for the 404 diagnostic.
func (s *Service) resolveValueSet(ctx context.Context, store *loinc.Store, id string) (*resolvedValueSet, *OutcomeError) {
	if strings.EqualFold(id, "loinc-all") {
		r, _ := resolveNamedValueSet(ctx, store, "loinc-all")
		return r, nil
	}
	if named, ok := resolveNamedValueSet(ctx, store, id); ok {
		return named, nil
	}
	upper := normalizeCode(id)
	switch {
	case strings.HasPrefix(upper, "LL"):
		list, err := store.AnswerList(ctx, upper)
		if err != nil {
			return nil, notFoundError("Failed to find matching value set")
		}
		var ident []Identifier
		if list.AnswerListOID != "" {
			ident = []Identifier{{System: "urn:ietf:rfc:3986", Value: "urn:oid:" + list.AnswerListOID}}
		}
		return &resolvedValueSet{
			id: upper, url: loincSystem + "/vs/" + upper, name: list.AnswerListName,
			identifier: ident, answerListID: upper, form: composeConcept,
		}, nil
	case strings.HasPrefix(upper, "LG"):
		if group, err := store.Group(ctx, upper); err == nil {
			src := loinc.FHIRTermValueSetSource{
				From:       "loinc_terms t join group_loinc_terms gt on gt.loinc_num = t.loinc_num",
				Where:      "gt.group_id = ?",
				Args:       []any{group.GroupID},
				EmbedOrder: "t.loinc_num", ExpandOrder: "t.loinc_num",
			}
			return &resolvedValueSet{
				id: upper, url: loincSystem + "/vs/" + upper, name: group.GroupName,
				termSource: &src, form: composeConcept,
			}, nil
		}
		if exists, err := store.ParentGroupExists(ctx, upper); err == nil && exists {
			childIDs, err := store.FHIRChildGroupIDs(ctx, upper)
			if err != nil || len(childIDs) == 0 {
				return nil, notFoundError("Failed to find matching value set")
			}
			urls := make([]string, len(childIDs))
			for i, childID := range childIDs {
				urls[i] = loincSystem + "/vs/" + childID
			}
			src := loinc.FHIRTermValueSetSource{
				From:       "loinc_terms t join group_loinc_terms gt on gt.loinc_num = t.loinc_num join loinc_groups g on g.group_id = gt.group_id",
				Where:      "g.parent_group_id = ?",
				Args:       []any{upper},
				EmbedOrder: "t.loinc_num", ExpandOrder: "t.loinc_num",
			}
			return &resolvedValueSet{
				id: upper, url: loincSystem + "/vs/" + upper, termSource: &src,
				form: composeValueSets, valueSets: urls,
			}, nil
		}
		return nil, notFoundError("Failed to find matching value set")
	case strings.HasPrefix(upper, "LP"):
		if _, err := store.Part(ctx, upper); err != nil {
			return nil, notFoundError("Failed to find matching value set")
		}
		src := loinc.FHIRTermValueSetSource{
			From:  "loinc_terms t join hierarchy_subtree_terms st on st.loinc_num = t.loinc_num join hierarchy_occurrences o on o.node_id = st.node_id join hierarchy_concepts c on c.code = o.code",
			Where: "c.code = ?",
			Args:  []any{upper},
		}
		return &resolvedValueSet{
			id: upper, url: loincSystem + "/vs/" + upper,
			name:       "LOINC Value Set from Multi-Axial Hierarchy code " + upper,
			termSource: &src, form: composeFilter,
			filterOut: []ValueSetIncludeFilter{{Property: "ancestor", Op: "=", Value: upper}},
		}, nil
	}
	return nil, notFoundError("Failed to find matching value set")
}

// buildValueSet renders a resolvedValueSet's non-expansion fields (§4.6.1, §4.6.3): metadata plus
// compose, choosing concept-embed vs filter form by member count when the source allows either.
func (s *Service) buildValueSet(ctx context.Context, store *loinc.Store, version string, r *resolvedValueSet) (*ValueSet, *OutcomeError) {
	vs := &ValueSet{
		ResourceType: "ValueSet", ID: r.id, URL: r.url, Identifier: r.identifier,
		Version: version, Name: r.name, Status: "active", Experimental: r.experimental,
		Publisher: loincPublisher, Contact: valueSetContact(), Description: r.description,
		Copyright: loincCopyright,
	}
	include := ValueSetInclude{System: loincSystem}
	switch {
	case r.form == composeValueSets:
		include.ValueSet = r.valueSets
	case r.form == composeFilter:
		include.Filter = r.filterOut
	case r.answerListID != "":
		items, err := store.FHIRAnswerListExpandPage(ctx, r.answerListID, "", 0, embedThreshold+1)
		if err != nil {
			return nil, &OutcomeError{Status: 503, Code: "exception", Text: err.Error()}
		}
		for _, item := range items {
			include.Concept = append(include.Concept, ValueSetIncludeConcept{Code: item.Code, Display: item.Display})
		}
	default:
		total, err := store.CachedCountTermSource(ctx, r.cacheKey(), *r.termSource)
		if err != nil {
			return nil, &OutcomeError{Status: 503, Code: "exception", Text: err.Error()}
		}
		if total > embedThreshold {
			include.Filter = r.filterOut
		} else {
			refs, err := store.CachedEmbedTermSource(ctx, r.cacheKey(), *r.termSource)
			if err != nil {
				return nil, &OutcomeError{Status: 503, Code: "exception", Text: err.Error()}
			}
			for _, ref := range refs {
				include.Concept = append(include.Concept, ValueSetIncludeConcept{Code: ref.Code, Display: ref.Display})
			}
		}
	}
	vs.Compose = &ValueSetCompose{Include: []ValueSetInclude{include}}
	return vs, nil
}

// ReadValueSet implements ValueSet read (§4.6.3): id, or id-version, resolve to the one served
// definition. version, when present, must match the loaded release.
func (s *Service) ReadValueSet(ctx context.Context, id string) (*ValueSet, *OutcomeError) {
	store, outcomeErr := s.store()
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	version, outcomeErr := s.releaseVersion(ctx, store)
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	id = strings.TrimSuffix(id, "-"+version)
	resolved, outcomeErr := s.resolveValueSet(ctx, store, id)
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	return s.buildValueSet(ctx, store, version, resolved)
}

// ValueSetSearchParams is ValueSet search-type's input (§4.6.3).
type ValueSetSearchParams struct {
	URL          string
	ID           string
	Name         string // prefix match
	NameContains string // substring match (upstream name:in behaves this way too)
	Count        int
	Offset       int
}

// SearchValueSets implements ValueSet search-type (§4.6.3): url, name/name:in/name:contains, _id,
// _count (default 20, max 100), _offset. Name search covers answer lists, groups, and named sets.
func (s *Service) SearchValueSets(ctx context.Context, params ValueSetSearchParams) (*Bundle, *OutcomeError) {
	store, outcomeErr := s.store()
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	version, outcomeErr := s.releaseVersion(ctx, store)
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	bundle := &Bundle{ResourceType: "Bundle", Type: "searchset"}

	if params.URL != "" || params.ID != "" {
		id := params.ID
		if id == "" {
			id = idFromURL(params.URL)
			if id == "" {
				return bundle, nil
			}
		}
		resolved, err := s.resolveValueSet(ctx, store, id)
		if err != nil {
			return bundle, nil
		}
		vs, err := s.buildValueSet(ctx, store, version, resolved)
		if err != nil {
			return nil, err
		}
		bundle.Total = 1
		bundle.Entry = []BundleEntry{{Resource: vs}}
		return bundle, nil
	}

	entries, outcomeErr := s.searchCatalogEntries(ctx, store, params.Name, params.NameContains)
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	bundle.Total = len(entries)
	count := params.Count
	offset := params.Offset
	end := offset + count
	if offset > len(entries) {
		offset = len(entries)
	}
	if end > len(entries) {
		end = len(entries)
	}
	for _, entry := range entries[offset:end] {
		resolved, err := s.resolveValueSet(ctx, store, entry)
		if err != nil {
			continue
		}
		vs, err := s.buildValueSet(ctx, store, version, resolved)
		if err != nil {
			return nil, err
		}
		bundle.Entry = append(bundle.Entry, BundleEntry{Resource: vs})
	}
	return bundle, nil
}

// searchCatalogEntries returns the ids of every catalogue entry (named sets, answer lists,
// groups) matching a name filter, name-sorted (answer lists and groups) with matching named sets
// appended (§4.6.3: "name search covers answer lists and groups... plus named sets").
func (s *Service) searchCatalogEntries(ctx context.Context, store *loinc.Store, namePrefix, nameContains string) ([]string, *OutcomeError) {
	answerLists, err := store.FHIRSearchAnswerLists(ctx, namePrefix, nameContains)
	if err != nil {
		return nil, &OutcomeError{Status: 503, Code: "exception", Text: err.Error()}
	}
	groups, err := store.FHIRSearchGroups(ctx, namePrefix, nameContains)
	if err != nil {
		return nil, &OutcomeError{Status: 503, Code: "exception", Text: err.Error()}
	}
	merged := mergeCatalogEntriesByName(answerLists, groups)
	var ids []string
	for _, entry := range merged {
		ids = append(ids, entry.ID)
	}
	needle := strings.ToLower(namePrefix + nameContains)
	for _, n := range namedValueSets() {
		if needle == "" {
			ids = append(ids, n.id)
			continue
		}
		name := strings.ToLower(n.name)
		if namePrefix != "" && strings.HasPrefix(name, strings.ToLower(namePrefix)) {
			ids = append(ids, n.id)
		} else if nameContains != "" && strings.Contains(name, strings.ToLower(nameContains)) {
			ids = append(ids, n.id)
		}
	}
	return ids, nil
}

// mergeCatalogEntriesByName merges two name-sorted lists into one name-sorted list.
func mergeCatalogEntriesByName(a, b []loinc.FHIRCatalogEntry) []loinc.FHIRCatalogEntry {
	merged := make([]loinc.FHIRCatalogEntry, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if strings.Compare(a[i].Name, b[j].Name) <= 0 {
			merged = append(merged, a[i])
			i++
		} else {
			merged = append(merged, b[j])
			j++
		}
	}
	merged = append(merged, a[i:]...)
	merged = append(merged, b[j:]...)
	return merged
}
