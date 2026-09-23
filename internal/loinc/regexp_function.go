package loinc

import (
	"container/list"
	"database/sql/driver"
	"fmt"
	"regexp"
	"sync"

	"modernc.org/sqlite"
)

// regexFilterMaxPatternLen bounds the $expand `regex` compose filter (§4.7.1): a longer pattern
// is rejected at the request boundary with a 400 "invalid", never handed to SQLite.
const regexFilterMaxPatternLen = 256

const regexpCacheMaxEntries = 256

// regexpCache is a small bounded LRU of compiled patterns shared by every open store. FHIR's
// `regex` filter is case-insensitive (LOINC codes/names have no case-sensitive semantics), so
// every distinct pattern is compiled once as "(?i)"+pattern and reused across rows and requests.
// ponytail: one global mutex guards this cache; if regex filters ever get hot enough for lock
// contention to show up in profiling, shard by a hash of the pattern instead.
var regexpCache = struct {
	mu      sync.Mutex
	entries map[string]*regexp.Regexp
	order   *list.List
	elems   map[string]*list.Element
}{
	entries: make(map[string]*regexp.Regexp),
	order:   list.New(),
	elems:   make(map[string]*list.Element),
}

// compileCachedRegexp compiles pattern (case-insensitively) or returns the already-compiled
// *regexp.Regexp for it.
func compileCachedRegexp(pattern string) (*regexp.Regexp, error) {
	regexpCache.mu.Lock()
	if re, ok := regexpCache.entries[pattern]; ok {
		regexpCache.order.MoveToFront(regexpCache.elems[pattern])
		regexpCache.mu.Unlock()
		return re, nil
	}
	regexpCache.mu.Unlock()

	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return nil, err
	}

	regexpCache.mu.Lock()
	defer regexpCache.mu.Unlock()
	if existing, ok := regexpCache.entries[pattern]; ok {
		return existing, nil
	}
	regexpCache.entries[pattern] = re
	regexpCache.elems[pattern] = regexpCache.order.PushFront(pattern)
	if len(regexpCache.entries) > regexpCacheMaxEntries {
		if oldest := regexpCache.order.Back(); oldest != nil {
			key := oldest.Value.(string)
			regexpCache.order.Remove(oldest)
			delete(regexpCache.entries, key)
			delete(regexpCache.elems, key)
		}
	}
	return re, nil
}

// ValidateRegexFilter enforces the $expand `regex` compose filter's request-time limits
// (§4.7.1): a pattern over regexFilterMaxPatternLen chars, or one that fails to compile, must be
// a 400 "invalid" at the caller (pkg/terminology), not a query-time SQLite error.
func ValidateRegexFilter(pattern string) error {
	if len(pattern) > regexFilterMaxPatternLen {
		return fmt.Errorf("regex filter pattern exceeds %d characters", regexFilterMaxPatternLen)
	}
	_, err := compileCachedRegexp(pattern)
	return err
}

// init registers the "regexp(pattern, value)" SQL scalar function once, at the driver level,
// before any store opens a connection: modernc.org/sqlite's RegisterDeterministicScalarFunction
// is process-global ("available to all new connections opened after executing"), so this must
// not be tied to a particular *Store or *sql.DB. It backs the $expand `regex` compose filter
// (§4.7.1), replacing the previous LIKE-substring approximation.
func init() {
	sqlite.MustRegisterDeterministicScalarFunction("regexp", 2, regexpSQLFunction)
}

func regexpSQLFunction(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	pattern, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("regexp: pattern must be text")
	}
	value, ok := args[1].(string)
	if !ok {
		// A NULL or non-text column value never matches; that is not a query error.
		return int64(0), nil
	}
	re, err := compileCachedRegexp(pattern)
	if err != nil {
		return nil, err
	}
	if re.MatchString(value) {
		return int64(1), nil
	}
	return int64(0), nil
}
