package loinc

import (
	"container/list"
	"sync"
)

// termSourceCacheMaxEntries bounds how many distinct term-source cache keys
// (CachedCountTermSource) can be held at once. Counts are plain ints, so this is the only
// ceiling they need.
const termSourceCacheMaxEntries = 4096

// termSourceListMaxEntries/termSourceListMaxTotalMembers bound the member-list caches
// (CachedEmbedTermSource, CachedExpandMembers): at most this many distinct lists, and at most
// this many members summed across all of them. Both apply -- whichever is hit first evicts the
// least-recently-used list. ponytail: fixed ceilings, not configurable; raise them (or make them
// a StoreOptions field) if a real deployment needs a bigger cache than this.
const (
	termSourceListMaxEntries      = 64
	termSourceListMaxTotalMembers = 500_000
)

// termSourceCache is a bounded LRU keyed by ValueSet term-source cache key, replacing the
// previous unbounded sync.Map (implicit `vs/{LP}` sets alone have thousands of possible keys, and
// an all-LOINC list is ~109k members -- an unbounded cache never frees any of that for the life of
// the process). sizeOf reports each value's "size" for maxTotalSize accounting; pass nil (with
// maxTotalSize 0) to bound by entry count alone.
type termSourceCache[T any] struct {
	mu           sync.Mutex
	maxEntries   int
	maxTotalSize int
	sizeOf       func(T) int
	order        *list.List
	entries      map[string]*list.Element
	totalSize    int
}

type termSourceCacheEntry[T any] struct {
	key   string
	value T
	size  int
}

func newTermSourceCache[T any](maxEntries, maxTotalSize int, sizeOf func(T) int) *termSourceCache[T] {
	return &termSourceCache[T]{
		maxEntries:   maxEntries,
		maxTotalSize: maxTotalSize,
		sizeOf:       sizeOf,
		order:        list.New(),
		entries:      make(map[string]*list.Element),
	}
}

func (c *termSourceCache[T]) get(key string) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	elem, ok := c.entries[key]
	if !ok {
		var zero T
		return zero, false
	}
	c.order.MoveToFront(elem)
	return elem.Value.(*termSourceCacheEntry[T]).value, true
}

func (c *termSourceCache[T]) set(key string, value T) {
	size := 1
	if c.sizeOf != nil {
		size = c.sizeOf(value)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.entries[key]; ok {
		c.totalSize -= elem.Value.(*termSourceCacheEntry[T]).size
		elem.Value = &termSourceCacheEntry[T]{key: key, value: value, size: size}
		c.totalSize += size
		c.order.MoveToFront(elem)
	} else {
		elem := c.order.PushFront(&termSourceCacheEntry[T]{key: key, value: value, size: size})
		c.entries[key] = elem
		c.totalSize += size
	}
	c.evict()
}

func (c *termSourceCache[T]) evict() {
	for len(c.entries) > c.maxEntries || (c.maxTotalSize > 0 && c.totalSize > c.maxTotalSize) {
		last := c.order.Back()
		if last == nil {
			return
		}
		entry := last.Value.(*termSourceCacheEntry[T])
		c.order.Remove(last)
		delete(c.entries, entry.key)
		c.totalSize -= entry.size
	}
}
