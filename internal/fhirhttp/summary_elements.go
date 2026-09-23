package fhirhttp

// summaryElement is the R4 summary/mandatory field set for one resource type, filtered to
// top-level fields only (paths with exactly one dot; nested _elements paths are not
// supported, §4.13). ChoicePrefixes holds base names of choice ([x]) elements (for example
// "source" for ConceptMap.source[x]), matched as a prefix against the actual serialized key
// (e.g. "sourceUri") since FHIR JSON never serializes the literal "[x]" name.
type summaryElement struct {
	Summary        map[string]bool
	Mandatory      map[string]bool
	ChoicePrefixes []string
}

// summaryElements is generated from docs/vendor/hl7/r4-summary-elements.json
// (Source: FHIR R4 (4.0.1) profiles-resources.json (https://hl7.org/fhir/R4/definitions.json.zip), elements with isSummary=true and top-level min>0 elements),
// restricted to top-level fields (paths with exactly one dot) per §4.13. Do not hand-pick
// entries; regenerate from that source file if it changes.
var summaryElements = map[string]summaryElement{
	"CodeSystem": {
		Summary: map[string]bool{
			"caseSensitive":    true,
			"compositional":    true,
			"contact":          true,
			"content":          true,
			"count":            true,
			"date":             true,
			"experimental":     true,
			"filter":           true,
			"hierarchyMeaning": true,
			"id":               true,
			"identifier":       true,
			"implicitRules":    true,
			"jurisdiction":     true,
			"meta":             true,
			"name":             true,
			"property":         true,
			"publisher":        true,
			"status":           true,
			"supplements":      true,
			"title":            true,
			"url":              true,
			"useContext":       true,
			"valueSet":         true,
			"version":          true,
			"versionNeeded":    true,
		},
		Mandatory: map[string]bool{
			"content": true,
			"status":  true,
		},
	},
	"ValueSet": {
		Summary: map[string]bool{
			"contact":       true,
			"date":          true,
			"experimental":  true,
			"id":            true,
			"identifier":    true,
			"immutable":     true,
			"implicitRules": true,
			"jurisdiction":  true,
			"meta":          true,
			"name":          true,
			"publisher":     true,
			"status":        true,
			"title":         true,
			"url":           true,
			"useContext":    true,
			"version":       true,
		},
		Mandatory: map[string]bool{
			"status": true,
		},
	},
	"ConceptMap": {
		Summary: map[string]bool{
			"contact":       true,
			"date":          true,
			"experimental":  true,
			"id":            true,
			"identifier":    true,
			"implicitRules": true,
			"jurisdiction":  true,
			"meta":          true,
			"name":          true,
			"publisher":     true,
			"status":        true,
			"title":         true,
			"url":           true,
			"useContext":    true,
			"version":       true,
		},
		Mandatory: map[string]bool{
			"status": true,
		},
		ChoicePrefixes: []string{"source", "target"},
	},
	"Questionnaire": {
		Summary: map[string]bool{
			"code":            true,
			"contact":         true,
			"date":            true,
			"effectivePeriod": true,
			"experimental":    true,
			"id":              true,
			"identifier":      true,
			"implicitRules":   true,
			"jurisdiction":    true,
			"meta":            true,
			"name":            true,
			"publisher":       true,
			"status":          true,
			"subjectType":     true,
			"title":           true,
			"url":             true,
			"useContext":      true,
			"version":         true,
		},
		Mandatory: map[string]bool{
			"status": true,
		},
	},
	"Bundle": {
		Summary: map[string]bool{
			"entry":         true,
			"id":            true,
			"identifier":    true,
			"implicitRules": true,
			"link":          true,
			"meta":          true,
			"signature":     true,
			"timestamp":     true,
			"total":         true,
			"type":          true,
		},
		Mandatory: map[string]bool{
			"type": true,
		},
	},
	"CapabilityStatement": {
		Summary: map[string]bool{
			"contact":             true,
			"date":                true,
			"document":            true,
			"experimental":        true,
			"fhirVersion":         true,
			"format":              true,
			"id":                  true,
			"implementation":      true,
			"implementationGuide": true,
			"implicitRules":       true,
			"imports":             true,
			"instantiates":        true,
			"jurisdiction":        true,
			"kind":                true,
			"messaging":           true,
			"meta":                true,
			"name":                true,
			"patchFormat":         true,
			"publisher":           true,
			"rest":                true,
			"software":            true,
			"status":              true,
			"title":               true,
			"url":                 true,
			"useContext":          true,
			"version":             true,
		},
		Mandatory: map[string]bool{
			"date":        true,
			"fhirVersion": true,
			"format":      true,
			"kind":        true,
			"status":      true,
		},
	},
	"TerminologyCapabilities": {
		Summary: map[string]bool{
			"contact":        true,
			"copyright":      true,
			"date":           true,
			"experimental":   true,
			"id":             true,
			"implementation": true,
			"implicitRules":  true,
			"jurisdiction":   true,
			"kind":           true,
			"lockedDate":     true,
			"meta":           true,
			"name":           true,
			"publisher":      true,
			"software":       true,
			"status":         true,
			"title":          true,
			"url":            true,
			"useContext":     true,
			"version":        true,
		},
		Mandatory: map[string]bool{
			"date":   true,
			"kind":   true,
			"status": true,
		},
	},
	"OperationOutcome": {
		Summary: map[string]bool{
			"id":            true,
			"implicitRules": true,
			"issue":         true,
			"meta":          true,
		},
		Mandatory: map[string]bool{
			"issue": true,
		},
	},
	"Parameters": {
		Summary: map[string]bool{
			"id":            true,
			"implicitRules": true,
			"meta":          true,
			"parameter":     true,
		},
	},
}
