package terminology

import (
	"context"
	"strings"

	"loinc-browser/internal/loinc"
)

// conceptMapKind identifies which store queries a ConceptMap's rows come from (§4.9).
type conceptMapKind int

const (
	mapKindIEEE conceptMapKind = iota
	mapKindPlaybook
	mapKindPartRelated
	mapKindLocalMapTo
)

// conceptMapBase is one served map pair (forward id + reverse id) from the §4.9 catalogue.
// ReverseID is "" for the local loinc-map-to extension, which has no published reverse.
type conceptMapBase struct {
	ForwardID     string
	ReverseID     string
	TargetURI     string
	Kind          conceptMapKind
	ExtCodeSystem string // only set for mapKindPartRelated
}

// conceptMapCatalog is the full served ConceptMap catalogue (§4.9), in the order upstream tries
// them for a "no url" $translate: IEEE, term-level RadLex, then the eight PartRelatedCodeMapping
// systems, then the local loinc-map-to extension. Not served (no release data): loinc-to-phenx
// and the CMS maps; PartRelatedCodeMapping's fdasis.nlm.nih.gov and genenames.org rows have no
// upstream map id and mint none here.
var conceptMapCatalog = []conceptMapBase{
	{ForwardID: "loinc-to-ieee-11073-10101", ReverseID: "ieee-11073-10101-to-loinc", TargetURI: "urn:iso:std:iso:11073:10101", Kind: mapKindIEEE},
	{ForwardID: "loinc-to-radlex", ReverseID: "radlex-to-loinc", TargetURI: "http://radlex.org", Kind: mapKindPlaybook},
	{ForwardID: "loinc-parts-to-radlex", ReverseID: "radlex-to-loinc-parts", TargetURI: "http://www.radlex.org", Kind: mapKindPartRelated, ExtCodeSystem: "http://www.radlex.org"},
	{ForwardID: "loinc-parts-to-rxnorm", ReverseID: "rxnorm-to-loinc-parts", TargetURI: "http://www.nlm.nih.gov/research/umls/rxnorm", Kind: mapKindPartRelated, ExtCodeSystem: "http://www.nlm.nih.gov/research/umls/rxnorm"},
	{ForwardID: "loinc-parts-to-pubchem", ReverseID: "pubchem-to-loinc-parts", TargetURI: "http://pubchem.ncbi.nlm.nih.gov", Kind: mapKindPartRelated, ExtCodeSystem: "http://pubchem.ncbi.nlm.nih.gov"},
	{ForwardID: "loinc-parts-to-snomed-ct", ReverseID: "snomed-ct-to-loinc-parts", TargetURI: "http://snomed.info/sct", Kind: mapKindPartRelated, ExtCodeSystem: "http://snomed.info/sct"},
	{ForwardID: "loinc-parts-to-chebi", ReverseID: "chebi-to-loinc-parts", TargetURI: "https://www.ebi.ac.uk/chebi", Kind: mapKindPartRelated, ExtCodeSystem: "https://www.ebi.ac.uk/chebi"},
	{ForwardID: "loinc-parts-to-ncbi-clinvar", ReverseID: "ncbi-clinvar-to-loinc-parts", TargetURI: "https://www.ncbi.nlm.nih.gov/clinvar", Kind: mapKindPartRelated, ExtCodeSystem: "https://www.ncbi.nlm.nih.gov/clinvar"},
	{ForwardID: "loinc-parts-to-ncbi-gene", ReverseID: "ncbi-gene-to-loinc-parts", TargetURI: "https://www.ncbi.nlm.nih.gov/gene", Kind: mapKindPartRelated, ExtCodeSystem: "https://www.ncbi.nlm.nih.gov/gene"},
	{ForwardID: "loinc-parts-to-ncbi-taxonomy", ReverseID: "ncbi-taxonomy-to-loinc-parts", TargetURI: "https://www.ncbi.nlm.nih.gov/taxonomy", Kind: mapKindPartRelated, ExtCodeSystem: "https://www.ncbi.nlm.nih.gov/taxonomy"},
	{ForwardID: "loinc-map-to", ReverseID: "", TargetURI: loincSystem, Kind: mapKindLocalMapTo},
}

// conceptMapDescriptor is one resolved direction (forward or reverse) of a conceptMapBase.
type conceptMapDescriptor struct {
	Base     conceptMapBase
	Reversed bool
}

// ID returns the served slug for this direction, or "" if this direction is not served (the
// reverse of loinc-map-to).
func (d conceptMapDescriptor) ID() string {
	if d.Reversed {
		return d.Base.ReverseID
	}
	return d.Base.ForwardID
}

func (d conceptMapDescriptor) SourceURI() string {
	if d.Reversed {
		return d.Base.TargetURI
	}
	return loincSystem
}

func (d conceptMapDescriptor) TargetURI() string {
	if d.Reversed {
		return loincSystem
	}
	return d.Base.TargetURI
}

// reversed returns the opposite direction of the same base map. Its ID() is "" when that
// direction is not served.
func (d conceptMapDescriptor) reversed() conceptMapDescriptor {
	return conceptMapDescriptor{Base: d.Base, Reversed: !d.Reversed}
}

// resolveConceptMapID looks up a served ConceptMap by its id (either direction's slug).
func resolveConceptMapID(id string) (conceptMapDescriptor, bool) {
	id = strings.TrimSpace(id)
	for _, base := range conceptMapCatalog {
		if strings.EqualFold(id, base.ForwardID) {
			return conceptMapDescriptor{Base: base}, true
		}
		if base.ReverseID != "" && strings.EqualFold(id, base.ReverseID) {
			return conceptMapDescriptor{Base: base, Reversed: true}, true
		}
	}
	return conceptMapDescriptor{}, false
}

// conceptMapIDFromURL extracts the id from a canonical "http://loinc.org/cm/{id}" url, or "" if
// url does not have that shape.
func conceptMapIDFromURL(url string) string {
	const prefix = loincSystem + "/cm/"
	url = strings.TrimSpace(url)
	if strings.HasPrefix(url, prefix) {
		return strings.TrimPrefix(url, prefix)
	}
	return ""
}

// allConceptMapDescriptors lists every served direction, forward and reverse, in catalogue order.
func allConceptMapDescriptors() []conceptMapDescriptor {
	out := make([]conceptMapDescriptor, 0, len(conceptMapCatalog)*2)
	for _, base := range conceptMapCatalog {
		out = append(out, conceptMapDescriptor{Base: base})
		if base.ReverseID != "" {
			out = append(out, conceptMapDescriptor{Base: base, Reversed: true})
		}
	}
	return out
}

// descriptorsForSourceSystem lists every served direction whose source system equals system, in
// catalogue order — the "no url" $translate candidate list (§4.10).
func descriptorsForSourceSystem(system string) []conceptMapDescriptor {
	var out []conceptMapDescriptor
	for _, d := range allConceptMapDescriptors() {
		if strings.EqualFold(system, d.SourceURI()) {
			out = append(out, d)
		}
	}
	return out
}

// conceptMapRows fetches a descriptor's underlying rows: code == "" lists every row (for the
// ConceptMap read's group), otherwise it filters to the one code on whichever side is the
// "source" for this direction (a $translate lookup).
func (s *Service) conceptMapRows(ctx context.Context, store *loinc.Store, d conceptMapDescriptor, code string, limit int) ([]loinc.FHIRConceptMapRow, error) {
	switch d.Base.Kind {
	case mapKindIEEE:
		if d.Reversed {
			return store.FHIRIEEERows(ctx, "", code, limit)
		}
		return store.FHIRIEEERows(ctx, code, "", limit)
	case mapKindPlaybook:
		if d.Reversed {
			return store.FHIRPlaybookRows(ctx, "", code, limit)
		}
		return store.FHIRPlaybookRows(ctx, code, "", limit)
	case mapKindPartRelated:
		if d.Reversed {
			return store.FHIRPartRelatedRows(ctx, d.Base.ExtCodeSystem, "", code, limit)
		}
		return store.FHIRPartRelatedRows(ctx, d.Base.ExtCodeSystem, code, "", limit)
	case mapKindLocalMapTo:
		if d.Reversed {
			return store.FHIRMapToRows(ctx, "", code, limit)
		}
		return store.FHIRMapToRows(ctx, code, "", limit)
	default:
		return nil, nil
	}
}

// rowEquivalence normalizes a row's CSV equivalence value, defaulting playbook's blank column to
// "relatedto" and forcing loinc-map-to to "equivalent" (§4.10).
func rowEquivalence(d conceptMapDescriptor, row loinc.FHIRConceptMapRow) string {
	if d.Base.Kind == mapKindLocalMapTo {
		return "equivalent"
	}
	equivalence := strings.ToLower(strings.TrimSpace(row.Equivalence))
	if equivalence == "" {
		equivalence = "relatedto"
	}
	return equivalence
}

// namedContact is a FHIR ContactDetail with the "name" field the exemplars carry (codesystem.go's
// ContactDetail omits it; this package keeps its own copy rather than editing a file owned by a
// concurrently-running phase).
type namedContact struct {
	Name    string         `json:"name,omitempty"`
	Telecom []ContactPoint `json:"telecom,omitempty"`
}

func loincContact() []namedContact {
	return []namedContact{{Name: loincPublisher, Telecom: []ContactPoint{{System: "url", Value: loincSystem}}}}
}

// ConceptMapGroup is one ConceptMap.group entry (§4.9).
type ConceptMapGroup struct {
	Source  string              `json:"source"`
	Target  string              `json:"target"`
	Element []ConceptMapElement `json:"element"`
}

// ConceptMapElement is one ConceptMap.group.element entry.
type ConceptMapElement struct {
	Code    string                    `json:"code"`
	Display string                    `json:"display,omitempty"`
	Target  []ConceptMapElementTarget `json:"target"`
}

// ConceptMapElementTarget is one ConceptMap.group.element.target entry.
type ConceptMapElementTarget struct {
	Code        string `json:"code"`
	Display     string `json:"display,omitempty"`
	Equivalence string `json:"equivalence"`
}

// ConceptMapExtension is a minimal FHIR Extension, used only for the read cap's truncation note
// (§4.9; no upstream exemplar defines a canonical url for this, so it is our own local marker).
type ConceptMapExtension struct {
	URL         string `json:"url"`
	ValueString string `json:"valueString,omitempty"`
}

// ConceptMap is a served http://loinc.org/cm/{id} ConceptMap resource (§4.9).
type ConceptMap struct {
	ResourceType string                `json:"resourceType"`
	ID           string                `json:"id"`
	URL          string                `json:"url"`
	Version      string                `json:"version"`
	Title        string                `json:"title"`
	Status       string                `json:"status"`
	Publisher    string                `json:"publisher"`
	Contact      []namedContact        `json:"contact,omitempty"`
	SourceUri    string                `json:"sourceUri"`
	TargetUri    string                `json:"targetUri"`
	Group        []ConceptMapGroup     `json:"group,omitempty"`
	Extension    []ConceptMapExtension `json:"extension,omitempty"`
}

// conceptMapReadCap is the maximum number of elements embedded in a ConceptMap read (§4.9).
// ponytail: a fixed cap rather than real pagination of the read interaction; $translate is the
// documented way to look up a specific code once a map exceeds this.
const conceptMapReadCap = 1000

func (s *Service) buildConceptMapResource(ctx context.Context, store *loinc.Store, version string, d conceptMapDescriptor, embedGroup bool) (*ConceptMap, *OutcomeError) {
	cm := &ConceptMap{
		ResourceType: "ConceptMap",
		ID:           d.ID(),
		URL:          loincSystem + "/cm/" + d.ID(),
		Version:      version,
		Title:        d.ID(),
		Status:       "active",
		Publisher:    loincPublisher,
		Contact:      loincContact(),
		SourceUri:    d.SourceURI(),
		TargetUri:    d.TargetURI(),
	}
	if !embedGroup {
		return cm, nil
	}
	rows, err := s.conceptMapRows(ctx, store, d, "", conceptMapReadCap+1)
	if err != nil {
		return nil, &OutcomeError{Status: 503, Code: "exception", Text: err.Error()}
	}
	truncated := len(rows) > conceptMapReadCap
	if truncated {
		rows = rows[:conceptMapReadCap]
	}
	elements := make([]ConceptMapElement, 0, len(rows))
	for _, row := range rows {
		srcCode, srcDisplay, tgtCode, tgtDisplay := row.SourceCode, row.SourceDisplay, row.TargetCode, row.TargetDisplay
		if d.Reversed {
			srcCode, srcDisplay, tgtCode, tgtDisplay = tgtCode, tgtDisplay, srcCode, srcDisplay
		}
		elements = append(elements, ConceptMapElement{
			Code:    srcCode,
			Display: srcDisplay,
			Target:  []ConceptMapElementTarget{{Code: tgtCode, Display: tgtDisplay, Equivalence: rowEquivalence(d, row)}},
		})
	}
	cm.Group = []ConceptMapGroup{{Source: cm.SourceUri, Target: cm.TargetUri, Element: elements}}
	if truncated {
		cm.Extension = []ConceptMapExtension{{
			URL:         loincSystem + "/fhir/StructureDefinition/conceptmap-truncated",
			ValueString: "This ConceptMap has more than 1000 mappings; use $translate to look up a specific code.",
		}}
	}
	return cm, nil
}

// ReadConceptMap implements the ConceptMap read interaction (§4.9): the full resource with its
// group embedded, capped at conceptMapReadCap elements.
func (s *Service) ReadConceptMap(ctx context.Context, id string) (*ConceptMap, *OutcomeError) {
	store, outcomeErr := s.store()
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	version, outcomeErr := s.releaseVersion(ctx, store)
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	d, ok := resolveConceptMapID(id)
	if !ok {
		return nil, notFoundError("ConceptMap not found = " + id)
	}
	return s.buildConceptMapResource(ctx, store, version, d, true)
}

// ConceptMapSearchParams is ConceptMap search-type's input (§4.9).
type ConceptMapSearchParams struct {
	URL          string
	SourceSystem string
	TargetSystem string
	SourceCode   string
	TargetCode   string
	Count        int
	Offset       int
}

func clampSearchPage(count, offset int) (int, int) {
	if count <= 0 {
		count = 20
	}
	if count > 100 {
		count = 100
	}
	if offset < 0 {
		offset = 0
	}
	return count, offset
}

// SearchConceptMaps implements ConceptMap search-type (§4.9): entries carry url/version/title/
// status/publisher/contact/sourceUri/targetUri, never an embedded group.
func (s *Service) SearchConceptMaps(ctx context.Context, params ConceptMapSearchParams) (*Bundle, *OutcomeError) {
	store, outcomeErr := s.store()
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	version, outcomeErr := s.releaseVersion(ctx, store)
	if outcomeErr != nil {
		return nil, outcomeErr
	}

	var matched []conceptMapDescriptor
	for _, d := range allConceptMapDescriptors() {
		if params.URL != "" && !strings.EqualFold(strings.TrimSpace(params.URL), loincSystem+"/cm/"+d.ID()) {
			continue
		}
		if params.SourceSystem != "" && !strings.EqualFold(strings.TrimSpace(params.SourceSystem), d.SourceURI()) {
			continue
		}
		if params.TargetSystem != "" && !strings.EqualFold(strings.TrimSpace(params.TargetSystem), d.TargetURI()) {
			continue
		}
		if params.SourceCode != "" {
			rows, err := s.conceptMapRows(ctx, store, d, normalizeCode(params.SourceCode), 1)
			if err != nil {
				return nil, &OutcomeError{Status: 503, Code: "exception", Text: err.Error()}
			}
			if len(rows) == 0 {
				continue
			}
		}
		if params.TargetCode != "" {
			rd := d.reversed()
			if rd.ID() == "" {
				continue
			}
			rows, err := s.conceptMapRows(ctx, store, rd, normalizeCode(params.TargetCode), 1)
			if err != nil {
				return nil, &OutcomeError{Status: 503, Code: "exception", Text: err.Error()}
			}
			if len(rows) == 0 {
				continue
			}
		}
		matched = append(matched, d)
	}

	total := len(matched)
	count, offset := clampSearchPage(params.Count, params.Offset)
	bundle := &Bundle{ResourceType: "Bundle", Type: "searchset", Total: total}
	if offset >= total {
		return bundle, nil
	}
	end := offset + count
	if end > total {
		end = total
	}
	for _, d := range matched[offset:end] {
		cm, err := s.buildConceptMapResource(ctx, store, version, d, false)
		if err != nil {
			return nil, err
		}
		bundle.Entry = append(bundle.Entry, BundleEntry{Resource: cm})
	}
	return bundle, nil
}
