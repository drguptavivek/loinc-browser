package terminology

import "context"

const softwareName = "loinc-browser"

// CapabilityOperation is one CapabilityStatement.rest.resource.operation.
type CapabilityOperation struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

// CapabilitySearchParam is one CapabilityStatement.rest.resource.searchParam.
type CapabilitySearchParam struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// CapabilityResource is one CapabilityStatement.rest.resource.
type CapabilityResource struct {
	Type        string                  `json:"type"`
	Interaction []CapabilityInteraction `json:"interaction"`
	SearchParam []CapabilitySearchParam `json:"searchParam,omitempty"`
	Operation   []CapabilityOperation   `json:"operation,omitempty"`
}

// CapabilityInteraction is one CapabilityStatement.rest.resource.interaction.
type CapabilityInteraction struct {
	Code string `json:"code"`
}

// CapabilityRest is one CapabilityStatement.rest entry.
type CapabilityRest struct {
	Mode     string               `json:"mode"`
	Resource []CapabilityResource `json:"resource"`
}

// CapabilitySoftware is CapabilityStatement.software.
type CapabilitySoftware struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// CapabilityImplementation is CapabilityStatement.implementation.
type CapabilityImplementation struct {
	Description string `json:"description"`
	URL         string `json:"url"`
}

// CapabilityStatement is the served CapabilityStatement resource (§4.1).
type CapabilityStatement struct {
	ResourceType   string                   `json:"resourceType"`
	FhirVersion    string                   `json:"fhirVersion"`
	Kind           string                   `json:"kind"`
	Format         []string                 `json:"format"`
	Status         string                   `json:"status"`
	Publisher      string                   `json:"publisher"`
	Software       CapabilitySoftware       `json:"software"`
	Implementation CapabilityImplementation `json:"implementation"`
	Rest           []CapabilityRest         `json:"rest"`
}

// Capabilities builds the plain CapabilityStatement (§4.1, `GET /fhir/metadata`).
func (s *Service) Capabilities(ctx context.Context) (*CapabilityStatement, *OutcomeError) {
	store, outcomeErr := s.store()
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	version, outcomeErr := s.releaseVersion(ctx, store)
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	codeSystemOps := []CapabilityOperation{
		{Name: "lookup", Definition: "http://hl7.org/fhir/OperationDefinition/CodeSystem-lookup"},
		{Name: "validate-code", Definition: "http://hl7.org/fhir/OperationDefinition/CodeSystem-validate-code"},
		{Name: "subsumes", Definition: "http://hl7.org/fhir/OperationDefinition/CodeSystem-subsumes"},
	}
	valueSetOps := []CapabilityOperation{
		{Name: "expand", Definition: "http://hl7.org/fhir/OperationDefinition/ValueSet-expand"},
		{Name: "validate-code", Definition: "http://hl7.org/fhir/OperationDefinition/ValueSet-validate-code"},
	}
	conceptMapOps := []CapabilityOperation{
		{Name: "translate", Definition: "http://hl7.org/fhir/OperationDefinition/ConceptMap-translate"},
	}
	return &CapabilityStatement{
		ResourceType: "CapabilityStatement",
		FhirVersion:  "4.0.1",
		Kind:         "instance",
		Format:       []string{"application/fhir+json", "json"},
		Status:       "active",
		Publisher:    loincPublisher,
		Software:     CapabilitySoftware{Name: softwareName, Version: version},
		Implementation: CapabilityImplementation{
			Description: "Local LOINC FHIR terminology server",
			URL:         loincSystem,
		},
		Rest: []CapabilityRest{{
			Mode: "server",
			Resource: []CapabilityResource{
				{
					// codeSystemSearchHandler (internal/fhirhttp/routes.go) only reads url/version;
					// there is no search-type "code" parameter, so it is not advertised here.
					Type:        "CodeSystem",
					Interaction: []CapabilityInteraction{{Code: "read"}, {Code: "search-type"}},
					SearchParam: []CapabilitySearchParam{{Name: "url", Type: "uri"}, {Name: "version", Type: "string"}},
					Operation:   codeSystemOps,
				},
				{
					// valueSetSearchHandler (internal/fhirhttp/valueset.go) reads url, _id, and
					// name (with its :contains/:in modifiers).
					Type:        "ValueSet",
					Interaction: []CapabilityInteraction{{Code: "read"}, {Code: "search-type"}},
					SearchParam: []CapabilitySearchParam{{Name: "url", Type: "uri"}, {Name: "name", Type: "string"}, {Name: "_id", Type: "token"}},
					Operation:   valueSetOps,
				},
				{
					// conceptMapSearchHandler (internal/fhirhttp/conceptmap.go) reads url,
					// source-system, target-system, source-code, and target-code.
					Type:        "ConceptMap",
					Interaction: []CapabilityInteraction{{Code: "read"}, {Code: "search-type"}},
					SearchParam: []CapabilitySearchParam{
						{Name: "url", Type: "uri"},
						{Name: "source-system", Type: "uri"},
						{Name: "target-system", Type: "uri"},
						{Name: "source-code", Type: "token"},
						{Name: "target-code", Type: "token"},
					},
					Operation: conceptMapOps,
				},
				{
					// questionnaireSearchHandler (internal/fhirhttp/questionnaire.go) reads url.
					Type:        "Questionnaire",
					Interaction: []CapabilityInteraction{{Code: "read"}, {Code: "search-type"}},
					SearchParam: []CapabilitySearchParam{{Name: "url", Type: "uri"}},
				},
			},
		}},
	}, nil
}

// TerminologyCapabilityVersion is one TerminologyCapabilities.codeSystem.version.
type TerminologyCapabilityVersion struct {
	Code          string `json:"code"`
	IsDefault     bool   `json:"isDefault"`
	Compositional bool   `json:"compositional"`
}

// TerminologyCapabilityCodeSystem is one TerminologyCapabilities.codeSystem entry.
type TerminologyCapabilityCodeSystem struct {
	URI     string                         `json:"uri"`
	Version []TerminologyCapabilityVersion `json:"version"`
}

// TerminologyCapabilityTranslation is TerminologyCapabilities.translation: whether $translate
// needs a client-supplied ConceptMap (it never does here -- every served mapping is one of our
// own local ConceptMaps, §4.9-§4.10).
type TerminologyCapabilityTranslation struct {
	NeedsMap bool `json:"needsMap"`
}

// TerminologyCapabilities is the served TerminologyCapabilities resource (§4.1,
// `GET /fhir/metadata?mode=terminology`).
type TerminologyCapabilities struct {
	ResourceType string                            `json:"resourceType"`
	Status       string                            `json:"status"`
	Kind         string                            `json:"kind"`
	Software     CapabilitySoftware                `json:"software"`
	CodeSystem   []TerminologyCapabilityCodeSystem `json:"codeSystem"`
	Translation  TerminologyCapabilityTranslation  `json:"translation"`
}

// TerminologyCapabilities builds the served TerminologyCapabilities resource.
func (s *Service) TerminologyCapabilities(ctx context.Context) (*TerminologyCapabilities, *OutcomeError) {
	store, outcomeErr := s.store()
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	version, outcomeErr := s.releaseVersion(ctx, store)
	if outcomeErr != nil {
		return nil, outcomeErr
	}
	return &TerminologyCapabilities{
		ResourceType: "TerminologyCapabilities",
		Status:       "active",
		Kind:         "capability",
		Software:     CapabilitySoftware{Name: softwareName, Version: version},
		CodeSystem: []TerminologyCapabilityCodeSystem{{
			URI:     loincSystem,
			Version: []TerminologyCapabilityVersion{{Code: version, IsDefault: true, Compositional: false}},
		}},
		Translation: TerminologyCapabilityTranslation{NeedsMap: false},
	}, nil
}
