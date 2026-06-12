package loinc

import (
	"container/list"
	"sync"
	"sync/atomic"
)

type objectCache struct {
	mu          sync.Mutex
	maxEntries  int
	terms       map[string]*list.Element
	termOrder   *list.List
	graphs      map[string]*list.Element
	graphOrder  *list.List
	accessories map[string]*list.Element
	accessoryOrder *list.List
	facets      *Facets
	termHits    atomic.Int64
	termMisses  atomic.Int64
	graphHits   atomic.Int64
	graphMisses atomic.Int64
	accessoryHits atomic.Int64
	accessoryMisses atomic.Int64
	facetHits   atomic.Int64
	facetMisses atomic.Int64
}

type termEntry struct {
	key  string
	term Term
}

type graphEntry struct {
	key   string
	graph TermRelationshipGraph
}

type accessoryEntry struct {
	key      string
	response AccessoryBrowseResponse
}

func newObjectCache(maxEntries int) *objectCache {
	if maxEntries <= 0 {
		maxEntries = 512
	}
	return &objectCache{
		maxEntries: maxEntries,
		terms:      make(map[string]*list.Element),
		termOrder:  list.New(),
		graphs:     make(map[string]*list.Element),
		graphOrder: list.New(),
		accessories: make(map[string]*list.Element),
		accessoryOrder: list.New(),
	}
}

func (c *objectCache) getTerm(key string) (Term, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	elem, ok := c.terms[key]
	if !ok {
		c.termMisses.Add(1)
		return Term{}, false
	}
	c.termOrder.MoveToFront(elem)
	c.termHits.Add(1)
	return elem.Value.(termEntry).term, true
}

func (c *objectCache) setTerm(key string, term Term) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.terms[key]; ok {
		elem.Value = termEntry{key: key, term: term}
		c.termOrder.MoveToFront(elem)
		return
	}
	elem := c.termOrder.PushFront(termEntry{key: key, term: term})
	c.terms[key] = elem
	if len(c.terms) <= c.maxEntries {
		return
	}
	last := c.termOrder.Back()
	if last == nil {
		return
	}
	c.termOrder.Remove(last)
	delete(c.terms, last.Value.(termEntry).key)
}

func (c *objectCache) getGraph(key string) (TermRelationshipGraph, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	elem, ok := c.graphs[key]
	if !ok {
		c.graphMisses.Add(1)
		return TermRelationshipGraph{}, false
	}
	c.graphOrder.MoveToFront(elem)
	c.graphHits.Add(1)
	return elem.Value.(graphEntry).graph, true
}

func (c *objectCache) setGraph(key string, graph TermRelationshipGraph) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.graphs[key]; ok {
		elem.Value = graphEntry{key: key, graph: graph}
		c.graphOrder.MoveToFront(elem)
		return
	}
	elem := c.graphOrder.PushFront(graphEntry{key: key, graph: graph})
	c.graphs[key] = elem
	if len(c.graphs) <= c.maxEntries {
		return
	}
	last := c.graphOrder.Back()
	if last == nil {
		return
	}
	c.graphOrder.Remove(last)
	delete(c.graphs, last.Value.(graphEntry).key)
}

func (c *objectCache) getAccessory(key string) (AccessoryBrowseResponse, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	elem, ok := c.accessories[key]
	if !ok {
		c.accessoryMisses.Add(1)
		return AccessoryBrowseResponse{}, false
	}
	c.accessoryOrder.MoveToFront(elem)
	c.accessoryHits.Add(1)
	return elem.Value.(accessoryEntry).response, true
}

func (c *objectCache) setAccessory(key string, response AccessoryBrowseResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.accessories[key]; ok {
		elem.Value = accessoryEntry{key: key, response: response}
		c.accessoryOrder.MoveToFront(elem)
		return
	}
	elem := c.accessoryOrder.PushFront(accessoryEntry{key: key, response: response})
	c.accessories[key] = elem
	if len(c.accessories) <= c.maxEntries {
		return
	}
	last := c.accessoryOrder.Back()
	if last == nil {
		return
	}
	c.accessoryOrder.Remove(last)
	delete(c.accessories, last.Value.(accessoryEntry).key)
}

func (c *objectCache) getFacets() (Facets, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.facets == nil {
		c.facetMisses.Add(1)
		return Facets{}, false
	}
	c.facetHits.Add(1)
	return cloneFacets(*c.facets), true
}

func (c *objectCache) setFacets(facets Facets) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cloned := cloneFacets(facets)
	c.facets = &cloned
}

func (c *objectCache) stats() CacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	facetEntries := 0
	if c.facets != nil {
		facetEntries = 1
	}
	return CacheStats{
		TermHits:            c.termHits.Load(),
		TermMisses:          c.termMisses.Load(),
		RelationshipHits:    c.graphHits.Load(),
		RelationshipMisses:  c.graphMisses.Load(),
		AccessoryHits:       c.accessoryHits.Load(),
		AccessoryMisses:     c.accessoryMisses.Load(),
		FacetHits:           c.facetHits.Load(),
		FacetMisses:         c.facetMisses.Load(),
		TermEntries:         len(c.terms),
		RelationshipEntries: len(c.graphs),
		AccessoryEntries:    len(c.accessories),
		FacetEntries:        facetEntries,
	}
}

func cloneFacets(f Facets) Facets {
	return Facets{
		Classes:    cloneIntMap(f.Classes),
		Statuses:   cloneIntMap(f.Statuses),
		Systems:    cloneIntMap(f.Systems),
		Scales:     cloneIntMap(f.Scales),
		Properties: cloneIntMap(f.Properties),
		OrderObs:   cloneIntMap(f.OrderObs),
	}
}

func cloneIntMap(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
