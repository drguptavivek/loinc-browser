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
	Query           string
	Class           string
	Status          string
	Statuses        []string
	UsageType       string
	RankMode        string
	Sort            string
	System          string
	TimeAspect      string
	TimeAspects     []string
	Scale           string
	Scales          []string
	Method          string
	Methods         []string
	Property        string
	OrderObs        string
	OrderObsValues  []string
	RankedOnly      bool
	HierarchyCode   string
	HierarchyNodeID string
	PartNumber      string
	PartLinkSet     string
	GroupID         string
	AnswerListID    string
	PanelParent     string
	PanelOnly       bool
	Limit           int
	Offset          int
}

type TermListParams = SearchParams

type Links map[string]string

type PageLinks struct {
	Self string `json:"self,omitempty"`
	Next string `json:"next,omitempty"`
	Prev string `json:"prev,omitempty"`
}

type Page[T any] struct {
	Results []T       `json:"results"`
	Total   int       `json:"total"`
	Limit   int       `json:"limit"`
	Offset  int       `json:"offset"`
	HasMore bool      `json:"hasMore"`
	Links   PageLinks `json:"_links"`
}

type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
	Limit   int            `json:"limit"`
	Offset  int            `json:"offset"`
	HasMore bool           `json:"hasMore"`
	Query   string         `json:"query"`
	Links   Links          `json:"_links,omitempty"`
}

type SearchResult struct {
	LOINCNum        string   `json:"loincNum"`
	LongCommonName  string   `json:"longCommonName"`
	ShortName       string   `json:"shortName"`
	Component       string   `json:"component"`
	Property        string   `json:"property"`
	System          string   `json:"system"`
	Scale           string   `json:"scale"`
	Method          string   `json:"method"`
	Class           string   `json:"class"`
	Status          string   `json:"status"`
	OrderObs        string   `json:"orderObs"`
	CommonTestRank  int      `json:"commonTestRank"`
	CommonOrderRank int      `json:"commonOrderRank"`
	UsageTypes      []string `json:"usageTypes"`
	Rank            float64  `json:"rank"`
	Links           Links    `json:"_links,omitempty"`
}

type Term struct {
	LOINCNum        string            `json:"loincNum"`
	LongCommonName  string            `json:"longCommonName"`
	ShortName       string            `json:"shortName"`
	Component       string            `json:"component"`
	Property        string            `json:"property"`
	TimeAspect      string            `json:"timeAspect"`
	System          string            `json:"system"`
	Scale           string            `json:"scale"`
	Method          string            `json:"method"`
	Class           string            `json:"class"`
	Status          string            `json:"status"`
	Definition      string            `json:"definition"`
	ConsumerName    string            `json:"consumerName"`
	RelatedNames    string            `json:"relatedNames"`
	OrderObs        string            `json:"orderObs"`
	DisplayName     string            `json:"displayName"`
	CommonTestRank  int               `json:"commonTestRank"`
	CommonOrderRank int               `json:"commonOrderRank"`
	UsageTypes      []string          `json:"usageTypes"`
	Links           Links             `json:"_links,omitempty"`
	Fields          map[string]string `json:"fields"`
	MapTo           []MapTo           `json:"mapTo"`
	Parts           []TermAccessory   `json:"parts"`
	AnswerLists     []TermAccessory   `json:"answerLists"`
	Panels          []TermAccessory   `json:"panels"`
	Groups          []TermAccessory   `json:"groups"`
	Hierarchy       []TermAccessory   `json:"hierarchy"`
}

type TermDetail = Term

type TermFit struct {
	LOINCNum             string   `json:"loincNum"`
	Status               string   `json:"status"`
	Deprecated           bool     `json:"deprecated"`
	Discouraged          bool     `json:"discouraged"`
	Inactive             bool     `json:"inactive"`
	OrderObs             string   `json:"orderObs"`
	UsageTypes           []string `json:"usageTypes"`
	CommonTestRank       int      `json:"commonTestRank"`
	CommonOrderRank      int      `json:"commonOrderRank"`
	HasAnswerLists       bool     `json:"hasAnswerLists"`
	HasPanelItems        bool     `json:"hasPanelItems"`
	HasPanelMemberships  bool     `json:"hasPanelMemberships"`
	HasHierarchy         bool     `json:"hasHierarchy"`
	HasExternalCopyright bool     `json:"hasExternalCopyright"`
	Links                Links    `json:"_links,omitempty"`
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

type TermRelationshipGroups struct {
	LOINCNum         string          `json:"loincNum"`
	MapTo            []MapTo         `json:"mapTo"`
	MappedFrom       []MapTo         `json:"mappedFrom"`
	Parts            []TermAccessory `json:"parts"`
	AnswerLists      []TermAccessory `json:"answerLists"`
	PanelMemberships []TermAccessory `json:"panelMemberships"`
	PanelItems       []TermAccessory `json:"panelItems"`
	Groups           []TermAccessory `json:"groups"`
	Hierarchy        []TermAccessory `json:"hierarchy"`
	Links            Links           `json:"_links,omitempty"`
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
	LOINCNum        string   `json:"loincNum"`
	LongCommonName  string   `json:"longCommonName"`
	ShortName       string   `json:"shortName"`
	DisplayName     string   `json:"displayName,omitempty"`
	Status          string   `json:"status"`
	OrderObs        string   `json:"orderObs"`
	UsageTypes      []string `json:"usageTypes"`
	CommonTestRank  int      `json:"commonTestRank"`
	CommonOrderRank int      `json:"commonOrderRank"`
	System          string   `json:"system"`
	Class           string   `json:"class"`
	Scale           string   `json:"scale,omitempty"`
	Property        string   `json:"property,omitempty"`
	Links           Links    `json:"_links,omitempty"`
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
	HasMore bool              `json:"hasMore"`
	Query   string            `json:"query"`
	Kind    string            `json:"kind"`
	Links   Links             `json:"_links,omitempty"`
}

type HierarchyNode struct {
	NodeID       string `json:"nodeId"`
	Code         string `json:"code"`
	Label        string `json:"label"`
	ParentNodeID string `json:"parentNodeId"`
	ParentCode   string `json:"parentCode"`
	PathKey      string `json:"pathKey"`
	Path         string `json:"path"`
	TermCount    int    `json:"termCount"`
	ChildCount   int    `json:"childCount"`
	IsTerm       bool   `json:"isTerm"`
	HasChildren  bool   `json:"hasChildren"`
	Links        Links  `json:"_links,omitempty"`
}

type HierarchyChildrenResponse struct {
	ParentNodeID string          `json:"parentNodeId"`
	ParentCode   string          `json:"parentCode"`
	Query        string          `json:"query"`
	Results      []HierarchyNode `json:"results"`
	Links        Links           `json:"_links,omitempty"`
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

type AnswerList struct {
	AnswerListID   string `json:"answerListId"`
	AnswerListName string `json:"answerListName"`
	AnswerListOID  string `json:"answerListOid"`
	ExtDefinedYN   string `json:"extDefinedYn"`
	Links          Links  `json:"_links,omitempty"`
}

type AnswerListAnswer struct {
	AnswerListID          string `json:"answerListId"`
	AnswerStringID        string `json:"answerStringId"`
	LocalAnswerCode       string `json:"localAnswerCode"`
	LocalAnswerCodeSystem string `json:"localAnswerCodeSystem"`
	SequenceNumber        int    `json:"sequenceNumber"`
	DisplayText           string `json:"displayText"`
	ExtCodeID             string `json:"extCodeId"`
	ExtCodeDisplayName    string `json:"extCodeDisplayName"`
	ExtCodeSystem         string `json:"extCodeSystem"`
	Score                 string `json:"score"`
	Links                 Links  `json:"_links,omitempty"`
}

type PanelItem struct {
	ParentLOINCNum       string      `json:"parentLoincNum"`
	ChildLOINCNum        string      `json:"childLoincNum"`
	Sequence             int         `json:"sequence"`
	ItemID               string      `json:"itemId"`
	DisplayNameForForm   string      `json:"displayNameForForm"`
	ObservationRequired  string      `json:"observationRequired"`
	EntryType            string      `json:"entryType"`
	DataTypeInForm       string      `json:"dataTypeInForm"`
	AnswerListIDOverride string      `json:"answerListIdOverride"`
	ChildTerm            TermSummary `json:"childTerm"`
	Links                Links       `json:"_links,omitempty"`
}

type TermCopyright struct {
	LOINCNum             string               `json:"loincNum"`
	Status               string               `json:"status"`
	HasExternalCopyright bool                 `json:"hasExternalCopyright"`
	State                string               `json:"state"`
	SourceOrganizations  []SourceOrganization `json:"sourceOrganizations"`
	Links                Links                `json:"_links,omitempty"`
}

type Part struct {
	PartNumber      string `json:"partNumber"`
	PartTypeName    string `json:"partTypeName"`
	PartName        string `json:"partName"`
	PartDisplayName string `json:"partDisplayName"`
	Status          string `json:"status"`
	Links           Links  `json:"_links,omitempty"`
}

type LOINCGroup struct {
	GroupID              string `json:"groupId"`
	ParentGroupID        string `json:"parentGroupId"`
	GroupName            string `json:"groupName"`
	Archetype            string `json:"archetype"`
	Status               string `json:"status"`
	VersionFirstReleased string `json:"versionFirstReleased"`
	Links                Links  `json:"_links,omitempty"`
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
