package terminology

import "loinc-browser/internal/loinc"

// OpenOptions configures Open.
type OpenOptions struct {
	// CacheEntries bounds the Store's small in-process object cache (default: a reasonable
	// built-in size when left zero; see loinc.newObjectCache).
	CacheEntries int
}

// Open opens dbPath read-only and returns a Service backed by it (plan §2 Mode A: an external
// Go module reads the same normalized SQLite file a running loinc-browser server has open,
// without importing loinc-browser/internal/loinc, which Go's internal-package rule forbids
// outside this module). The store is opened with SQLite's `mode=ro` URI flag so it can share a
// WAL-mode file with a concurrent writer; every lazy CREATE INDEX path in internal/loinc is
// skipped for a read-only store (logged once), so a $lookup/$expand/etc. still answers correctly,
// just without the speed-up an index gives on an old database that predates that index.
//
// Call (*Service).Close when done with it.
func Open(dbPath string, opts OpenOptions) (*Service, error) {
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{
		CacheEntries: opts.CacheEntries,
		ReadOnly:     true,
	})
	if err != nil {
		return nil, err
	}
	return &Service{getStore: func() (*loinc.Store, error) { return store, nil }, closeStore: store.Close}, nil
}

// Close releases the database connection Open opened. Safe to call once; a Service built via
// NewService (no owned store) has nothing to close and returns nil.
func (s *Service) Close() error {
	if s.closeStore == nil {
		return nil
	}
	return s.closeStore()
}
