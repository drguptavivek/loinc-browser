package loinc

import "time"

type IngestOptions struct {
	ReleaseDir string
	DBPath     string
}

type IngestSummary struct {
	TermCount  int
	DBPath     string
	ReleaseDir string
	ImportedAt time.Time
}

type StoreOptions struct {
	CacheEntries int
}

type SearchParams struct {
	Query          string
	Class          string
	Status         string
	Statuses       []string
	System         string
	TimeAspect     string
	TimeAspects    []string
	Scale          string
	Scales         []string
	Method         string
	Methods        []string
	Property       string
	OrderObs       string
	OrderObsValues []string
	Limit          int
	Offset         int
}

type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
	Limit   int            `json:"limit"`
	Offset  int            `json:"offset"`
	Query   string         `json:"query"`
}

type SearchResult struct {
	LOINCNum       string  `json:"loincNum"`
	LongCommonName string  `json:"longCommonName"`
	ShortName      string  `json:"shortName"`
	Component      string  `json:"component"`
	Property       string  `json:"property"`
	System         string  `json:"system"`
	Scale          string  `json:"scale"`
	Method         string  `json:"method"`
	Class          string  `json:"class"`
	Status         string  `json:"status"`
	OrderObs       string  `json:"orderObs"`
	Rank           float64 `json:"rank"`
}

type Term struct {
	LOINCNum       string            `json:"loincNum"`
	LongCommonName string            `json:"longCommonName"`
	ShortName      string            `json:"shortName"`
	Component      string            `json:"component"`
	Property       string            `json:"property"`
	TimeAspect     string            `json:"timeAspect"`
	System         string            `json:"system"`
	Scale          string            `json:"scale"`
	Method         string            `json:"method"`
	Class          string            `json:"class"`
	Status         string            `json:"status"`
	Definition     string            `json:"definition"`
	ConsumerName   string            `json:"consumerName"`
	RelatedNames   string            `json:"relatedNames"`
	OrderObs       string            `json:"orderObs"`
	DisplayName    string            `json:"displayName"`
	Fields         map[string]string `json:"fields"`
	MapTo          []MapTo           `json:"mapTo"`
	Parts          []TermAccessory   `json:"parts"`
	AnswerLists    []TermAccessory   `json:"answerLists"`
	Panels         []TermAccessory   `json:"panels"`
	Groups         []TermAccessory   `json:"groups"`
	Hierarchy      []TermAccessory   `json:"hierarchy"`
}

type MapTo struct {
	LOINC   string `json:"loinc"`
	MapTo   string `json:"mapTo"`
	Comment string `json:"comment"`
}

type TermAccessory struct {
	Kind     string            `json:"kind"`
	Code     string            `json:"code"`
	Title    string            `json:"title"`
	Subtitle string            `json:"subtitle"`
	Fields   map[string]string `json:"fields"`
}

type TermRelationshipGraph struct {
	LOINCNum       string                `json:"loincNum"`
	OutgoingMapTo  []MapTo               `json:"outgoingMapTo"`
	IncomingMapTo  []MapTo               `json:"incomingMapTo"`
	SharedConcepts []RelationshipConcept `json:"sharedConcepts"`
}

type RelationshipConcept struct {
	Kind         string            `json:"kind"`
	Code         string            `json:"code"`
	Title        string            `json:"title"`
	Subtitle     string            `json:"subtitle"`
	Fields       map[string]string `json:"fields"`
	RelatedTotal int               `json:"relatedTotal"`
	RelatedTerms []TermSummary     `json:"relatedTerms"`
}

type TermSummary struct {
	LOINCNum       string `json:"loincNum"`
	LongCommonName string `json:"longCommonName"`
	ShortName      string `json:"shortName"`
	Status         string `json:"status"`
	System         string `json:"system"`
	Class          string `json:"class"`
}

type AccessoryBrowseParams struct {
	Kind   string
	Query  string
	Limit  int
	Offset int
}

type AccessoryBrowseResponse struct {
	Results []AccessoryRecord `json:"results"`
	Total   int               `json:"total"`
	Limit   int               `json:"limit"`
	Offset  int               `json:"offset"`
	Query   string            `json:"query"`
	Kind    string            `json:"kind"`
}

type AccessoryRecord struct {
	TermAccessory
	LOINCNum       string `json:"loincNum"`
	LongCommonName string `json:"longCommonName"`
	ShortName      string `json:"shortName"`
	Status         string `json:"status"`
}

type SourceOrganization struct {
	ID          string            `json:"id"`
	CopyrightID string            `json:"copyrightId"`
	Name        string            `json:"name"`
	Copyright   string            `json:"copyright"`
	TermsOfUse  string            `json:"termsOfUse"`
	URL         string            `json:"url"`
	Fields      map[string]string `json:"fields"`
}

type Facets struct {
	Classes     map[string]int `json:"classes"`
	Statuses    map[string]int `json:"statuses"`
	Systems     map[string]int `json:"systems"`
	TimeAspects map[string]int `json:"timeAspects"`
	Scales      map[string]int `json:"scales"`
	Methods     map[string]int `json:"methods"`
	Properties  map[string]int `json:"properties"`
	OrderObs    map[string]int `json:"orderObs"`
}

type CacheStats struct {
	TermHits            int64 `json:"termHits"`
	TermMisses          int64 `json:"termMisses"`
	RelationshipHits    int64 `json:"relationshipHits"`
	RelationshipMisses  int64 `json:"relationshipMisses"`
	AccessoryHits       int64 `json:"accessoryHits"`
	AccessoryMisses     int64 `json:"accessoryMisses"`
	FacetHits           int64 `json:"facetHits"`
	FacetMisses         int64 `json:"facetMisses"`
	TermEntries         int   `json:"termEntries"`
	RelationshipEntries int   `json:"relationshipEntries"`
	AccessoryEntries    int   `json:"accessoryEntries"`
	FacetEntries        int   `json:"facetEntries"`
}
