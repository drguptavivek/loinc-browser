// summary.go implements FHIR R4's `_summary`/`_elements` search result parameters (plan §4.13)
// as one post-processing step applied to resource-returning responses: read, search (per
// entry.resource; the Bundle envelope is untouched), metadata, and $expand. Parameters-returning
// operations ($lookup, $validate-code, $subsumes, $translate) and OperationOutcomes are left
// unchanged, as the spec leaves them out of scope.
package fhirhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"loinc-browser/pkg/terminology"
)

type summaryMode int

const (
	summaryModeNone summaryMode = iota
	summaryModeTrue
	summaryModeText
	summaryModeData
	summaryModeCount
	summaryModeElements
)

// summaryFilter is the parsed `_summary`/`_elements` request. A nil *summaryFilter means "no
// filtering" (mode false/absent).
type summaryFilter struct {
	mode     summaryMode
	elements []string
}

// subsettedTag is the meta.tag every filtered resource carries (plan §4.13).
var subsettedTag = json.RawMessage(`{"system":"http://terminology.hl7.org/CodeSystem/v3-ObservationValue","code":"SUBSETTED"}`)

// parseSummaryFilter reads `_summary`/`_elements` from query. Combining them, or an unknown
// `_summary` value, is a 400 "invalid". `_summary=count` is 400 "invalid" unless allowCount
// (search-type Bundles only, §4.13): reads, metadata, and $expand return a single resource, not
// a searchset, so count has nothing to count there.
func parseSummaryFilter(query url.Values, allowCount bool) (*summaryFilter, *terminology.OutcomeError) {
	summaryRaw, hasSummary := query["_summary"]
	elementsRaw, hasElements := query["_elements"]
	if hasSummary && hasElements {
		return nil, &terminology.OutcomeError{Status: 400, Code: "invalid", Text: "_summary and _elements cannot be combined"}
	}
	if hasElements {
		return &summaryFilter{mode: summaryModeElements, elements: strings.Split(elementsRaw[0], ",")}, nil
	}
	if !hasSummary {
		return nil, nil
	}
	switch summaryRaw[0] {
	case "true":
		return &summaryFilter{mode: summaryModeTrue}, nil
	case "text":
		return &summaryFilter{mode: summaryModeText}, nil
	case "data":
		return &summaryFilter{mode: summaryModeData}, nil
	case "count":
		if !allowCount {
			return nil, &terminology.OutcomeError{Status: 400, Code: "invalid", Text: "_summary=count is only valid on a search"}
		}
		return &summaryFilter{mode: summaryModeCount}, nil
	case "false":
		return nil, nil
	default:
		return nil, &terminology.OutcomeError{Status: 400, Code: "invalid", Text: "Invalid _summary value: " + summaryRaw[0]}
	}
}

// writeFHIRSummary writes resource as FHIR JSON, applying filter first (nil writes resource
// unfiltered, same as writeFHIR).
func writeFHIRSummary(w http.ResponseWriter, status int, resource any, filter *summaryFilter) {
	if filter == nil {
		writeFHIR(w, status, resource)
		return
	}
	data, err := json.Marshal(resource)
	if err != nil {
		writeOutcome(w, http.StatusInternalServerError, "exception", "failed to encode resource")
		return
	}
	filtered, ferr := applySummaryFilter(data, filter)
	if ferr != nil {
		writeOutcomeError(w, ferr)
		return
	}
	writeFHIR(w, status, json.RawMessage(filtered))
}

// applySummaryBundle applies filter to a search-type Bundle (plan §4.13): `_summary=count`
// drops entry entirely, keeping only type/total/link[self]; every other mode filters each
// entry.resource in place. A nil filter is a no-op.
func applySummaryBundle(r *http.Request, bundle *terminology.Bundle, filter *summaryFilter) *terminology.OutcomeError {
	if filter == nil {
		return nil
	}
	if filter.mode == summaryModeCount {
		bundle.Entry = nil
		bundle.Link = []terminology.BundleLink{{Relation: "self", URL: requestURL(r, r.URL.Query())}}
		return nil
	}
	for i := range bundle.Entry {
		data, err := json.Marshal(bundle.Entry[i].Resource)
		if err != nil {
			return &terminology.OutcomeError{Status: 500, Code: "exception", Text: "failed to encode resource"}
		}
		filtered, ferr := applySummaryFilter(data, filter)
		if ferr != nil {
			return ferr
		}
		bundle.Entry[i].Resource = json.RawMessage(filtered)
	}
	return nil
}

// jsonPair is one key/raw-value entry of a JSON object, in source order.
type jsonPair struct {
	key   string
	value json.RawMessage
}

// decodeOrderedObject walks a JSON object's top-level keys via json.Decoder tokens, preserving
// their source order (a map[string]any re-marshal would sort them, which §4.13 forbids).
func decodeOrderedObject(data []byte) ([]jsonPair, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("expected a JSON object")
	}
	var pairs []jsonPair
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := keyTok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		pairs = append(pairs, jsonPair{key: key, value: raw})
	}
	if _, err := dec.Token(); err != nil { // consume the closing '}'
		return nil, err
	}
	return pairs, nil
}

func encodeOrderedObject(pairs []jsonPair) []byte {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, p := range pairs {
		if i > 0 {
			buf.WriteByte(',')
		}
		keyJSON, _ := json.Marshal(p.key)
		buf.Write(keyJSON)
		buf.WriteByte(':')
		buf.Write(p.value)
	}
	buf.WriteByte('}')
	return buf.Bytes()
}

// applySummaryFilter filters one marshalled resource's top-level keys per filter (plan §4.13),
// preserving the order of the keys that remain, and adds the SUBSETTED meta.tag.
func applySummaryFilter(data []byte, filter *summaryFilter) ([]byte, *terminology.OutcomeError) {
	pairs, err := decodeOrderedObject(data)
	if err != nil {
		return nil, &terminology.OutcomeError{Status: 500, Code: "exception", Text: "failed to parse resource for _summary/_elements filtering"}
	}
	var resourceType string
	for _, p := range pairs {
		if p.key == "resourceType" {
			_ = json.Unmarshal(p.value, &resourceType)
			break
		}
	}
	elems := summaryElements[resourceType]

	keep := func(key string) bool {
		switch key {
		case "resourceType", "id", "meta":
			return true
		}
		if elems.Mandatory[key] {
			return true
		}
		switch filter.mode {
		case summaryModeData:
			return key != "text"
		case summaryModeText:
			return key == "text"
		case summaryModeTrue:
			if elems.Summary[key] {
				return true
			}
			for _, prefix := range elems.ChoicePrefixes {
				if strings.HasPrefix(key, prefix) {
					return true
				}
			}
			return false
		case summaryModeElements:
			for _, name := range filter.elements {
				if name == key {
					return true
				}
			}
			return false
		default:
			return false
		}
	}

	kept := make([]jsonPair, 0, len(pairs))
	for _, p := range pairs {
		if keep(p.key) {
			kept = append(kept, p)
		}
	}
	kept = ensureSubsettedTag(kept)
	return encodeOrderedObject(kept), nil
}

// ensureSubsettedTag adds the SUBSETTED coding to the resource's meta.tag (plan §4.13), merging
// with any tag already there, or inserting a new "meta" key (right after "id", or after
// "resourceType" if there is no "id") when the resource carries none.
func ensureSubsettedTag(pairs []jsonPair) []jsonPair {
	for i, p := range pairs {
		if p.key == "meta" {
			pairs[i].value = mergeSubsettedTag(p.value)
			return pairs
		}
	}
	insertAt := 0
	for i, p := range pairs {
		if p.key == "resourceType" {
			insertAt = i + 1
		}
		if p.key == "id" {
			insertAt = i + 1
			break
		}
	}
	metaPair := jsonPair{key: "meta", value: json.RawMessage(`{"tag":[` + string(subsettedTag) + `]}`)}
	out := make([]jsonPair, 0, len(pairs)+1)
	out = append(out, pairs[:insertAt]...)
	out = append(out, metaPair)
	out = append(out, pairs[insertAt:]...)
	return out
}

func mergeSubsettedTag(metaRaw json.RawMessage) json.RawMessage {
	pairs, err := decodeOrderedObject(metaRaw)
	if err != nil {
		return metaRaw
	}
	for i, p := range pairs {
		if p.key != "tag" {
			continue
		}
		var tags []json.RawMessage
		if err := json.Unmarshal(p.value, &tags); err != nil {
			return metaRaw
		}
		for _, t := range tags {
			if bytes.Contains(t, []byte("SUBSETTED")) && bytes.Contains(t, []byte("v3-ObservationValue")) {
				return metaRaw
			}
		}
		tags = append(tags, subsettedTag)
		newTags, err := json.Marshal(tags)
		if err != nil {
			return metaRaw
		}
		pairs[i].value = newTags
		return encodeOrderedObject(pairs)
	}
	pairs = append(pairs, jsonPair{key: "tag", value: json.RawMessage(`[` + string(subsettedTag) + `]`)})
	return encodeOrderedObject(pairs)
}
