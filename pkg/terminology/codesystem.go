package terminology

import (
	"context"
	"strings"
)

const (
	loincCodeSystemName  = "LOINC"
	loincCodeSystemTitle = "LOINC Code System"
	loincPublisher       = "Regenstrief Institute, Inc."
	loincCopyright       = "This material contains content from LOINC (http://loinc.org). LOINC is copyright Regenstrief Institute, Inc. and the Logical Observation Identifiers Names and Codes (LOINC) Committee and is available at no cost under the license at http://loinc.org/license. LOINC® is a registered United States trademark of Regenstrief Institute, Inc."
	loincDescription     = "LOINC is a freely available international standard for tests, measurements, and observations"
	loincIdentifierOID   = "urn:oid:2.16.840.1.113883.6.1"
)

// ContactDetail is a minimal FHIR ContactDetail, used for CodeSystem.contact and
// ValueSet.contact. Name is unset (and omitted) for CodeSystem, which upstream never carries it
// for; ValueSet resources do (§4.6.1 exemplars).
type ContactDetail struct {
	Name    string         `json:"name,omitempty"`
	Telecom []ContactPoint `json:"telecom,omitempty"`
}

// ContactPoint is a minimal FHIR ContactPoint.
type ContactPoint struct {
	System string `json:"system,omitempty"`
	Value  string `json:"value,omitempty"`
}

// Identifier is a minimal FHIR Identifier.
type Identifier struct {
	System string `json:"system,omitempty"`
	Value  string `json:"value,omitempty"`
}

// CodeSystemFilterOut is one CodeSystem.filter entry as emitted on the wire.
type CodeSystemFilterOut struct {
	Code        string   `json:"code"`
	Description string   `json:"description,omitempty"`
	Operator    []string `json:"operator"`
	Value       string   `json:"value,omitempty"`
}

// CodeSystemPropertyOut is one CodeSystem.property entry as emitted on the wire.
type CodeSystemPropertyOut struct {
	Code        string `json:"code"`
	URI         string `json:"uri,omitempty"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
}

// CodeSystem is the http://loinc.org CodeSystem resource (§4.2).
type CodeSystem struct {
	ResourceType     string                  `json:"resourceType"`
	ID               string                  `json:"id"`
	URL              string                  `json:"url"`
	Identifier       []Identifier            `json:"identifier,omitempty"`
	Version          string                  `json:"version"`
	Name             string                  `json:"name"`
	Title            string                  `json:"title"`
	Status           string                  `json:"status"`
	Experimental     bool                    `json:"experimental"`
	Publisher        string                  `json:"publisher"`
	Contact          []ContactDetail         `json:"contact,omitempty"`
	Description      string                  `json:"description"`
	Copyright        string                  `json:"copyright"`
	CaseSensitive    bool                    `json:"caseSensitive"`
	ValueSet         string                  `json:"valueSet"`
	HierarchyMeaning string                  `json:"hierarchyMeaning"`
	Compositional    bool                    `json:"compositional"`
	VersionNeeded    bool                    `json:"versionNeeded"`
	Content          string                  `json:"content"`
	Filter           []CodeSystemFilterOut   `json:"filter,omitempty"`
	Property         []CodeSystemPropertyOut `json:"property,omitempty"`
}

func buildCodeSystem(version string) CodeSystem {
	filters := make([]CodeSystemFilterOut, 0, len(codeSystemFilters))
	for _, f := range codeSystemFilters {
		filters = append(filters, CodeSystemFilterOut{Code: f.Code, Description: f.Description, Operator: f.Operator, Value: f.Value})
	}
	properties := make([]CodeSystemPropertyOut, 0, len(codeSystemProperties))
	for _, p := range codeSystemProperties {
		properties = append(properties, CodeSystemPropertyOut{Code: p.Code, URI: p.URI, Description: p.Description, Type: p.Type})
	}
	return CodeSystem{
		ResourceType:     "CodeSystem",
		ID:               "loinc-" + version,
		URL:              loincSystem,
		Identifier:       []Identifier{{System: "urn:ietf:rfc:3986", Value: loincIdentifierOID}},
		Version:          version,
		Name:             loincCodeSystemName,
		Title:            loincCodeSystemTitle,
		Status:           "active",
		Experimental:     false,
		Publisher:        loincPublisher,
		Contact:          []ContactDetail{{Telecom: []ContactPoint{{System: "url", Value: loincSystem}}}},
		Description:      loincDescription,
		Copyright:        loincCopyright,
		CaseSensitive:    false,
		ValueSet:         loincSystem + "/vs",
		HierarchyMeaning: "is-a",
		Compositional:    false,
		VersionNeeded:    false,
		Content:          "not-present",
		Filter:           filters,
		Property:         properties,
	}
}

// CodeSystemResource implements the CodeSystem read interaction (§4.2). Both the bare id
// ("loinc") and the versioned id ("loinc-2.82") resolve to the one loaded CodeSystem.
func (s *Service) CodeSystemResource(ctx context.Context, id string) (*CodeSystem, *OutcomeError) {
	store, outcomeErr := s.store()
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	version, outcomeErr := s.releaseVersion(ctx, store)
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	if id != "loinc" && id != "loinc-"+version {
		return nil, notFoundError("Code system not found = " + id)
	}
	cs := buildCodeSystem(version)
	return &cs, nil
}

// BundleEntry is one Bundle.entry.
type BundleEntry struct {
	FullURL  string `json:"fullUrl,omitempty"`
	Resource any    `json:"resource"`
}

// BundleLink is one Bundle.link.
type BundleLink struct {
	Relation string `json:"relation"`
	URL      string `json:"url"`
}

// Bundle is a minimal FHIR searchset Bundle.
type Bundle struct {
	ResourceType string        `json:"resourceType"`
	Type         string        `json:"type"`
	Total        int           `json:"total"`
	Link         []BundleLink  `json:"link,omitempty"`
	Entry        []BundleEntry `json:"entry,omitempty"`
}

// SearchCodeSystem implements CodeSystem search-type (§4.2): `?url=http://loinc.org[&version=]`
// returns the one loaded CodeSystem as a single-entry searchset Bundle; any other url or a
// version that does not match the loaded release returns an empty Bundle.
func (s *Service) SearchCodeSystem(ctx context.Context, url, version string) (*Bundle, *OutcomeError) {
	store, outcomeErr := s.store()
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	loadedVersion, outcomeErr := s.releaseVersion(ctx, store)
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	bundle := &Bundle{ResourceType: "Bundle", Type: "searchset"}
	if url != "" && !strings.EqualFold(strings.TrimSpace(url), loincSystem) {
		return bundle, nil
	}
	if version != "" && !strings.HasPrefix(loadedVersion, strings.TrimSpace(version)) {
		return bundle, nil
	}
	cs := buildCodeSystem(loadedVersion)
	bundle.Total = 1
	bundle.Entry = []BundleEntry{{Resource: cs}}
	return bundle, nil
}
