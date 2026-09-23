package terminology

import (
	"context"
	"strings"
	"time"

	"loinc-browser/internal/loinc"
)

// Service is the Mode A public API: a thin wrapper around a store getter. The server hot-swaps
// its store after an upload import, so Service never captures a *loinc.Store — every method
// calls getStore() fresh, per §2.
type Service struct {
	getStore func() (*loinc.Store, error)
	// closeStore is set only for a Service Open built (a Service that owns its store); nil for
	// one built via NewService, which wraps a caller-owned getter it must not close.
	closeStore func() error
}

// NewService builds a Service around a store getter, e.g. the server's currentStore function.
func NewService(getStore func() (*loinc.Store, error)) *Service {
	return &Service{getStore: getStore}
}

// store resolves the current store, translating a getter failure into the 503 "exception"
// OperationOutcome §4.12 calls for when the store is not loaded.
func (s *Service) store() (*loinc.Store, *OutcomeError) {
	store, err := s.getStore()
	if err != nil || store == nil {
		text := "LOINC database is not loaded"
		if err != nil {
			text = err.Error()
		}
		return nil, &OutcomeError{Status: 503, Code: "exception", Text: text}
	}
	return store, nil
}

// releaseVersion loads the loaded LOINC version, translating ErrNotFound into an exception.
func (s *Service) releaseVersion(ctx context.Context, store *loinc.Store) (string, *OutcomeError) {
	version, err := store.ReleaseVersion(ctx)
	if err != nil {
		return "", &OutcomeError{Status: 503, Code: "exception", Text: "LOINC release version is not available"}
	}
	return version, nil
}

// checkSystemVersion validates the `system` and `version` request parameters shared by $lookup,
// $validate-code, and $subsumes (§4.0): system, if given, must be http://loinc.org; version, if
// given, must equal the loaded version or be a prefix of it. notFoundText is the OutcomeError
// text to use on a mismatch (callers phrase it differently per operation).
func checkSystemVersion(system, version, loadedVersion, notFoundText string) *OutcomeError {
	if system != "" && !strings.EqualFold(strings.TrimSpace(system), loincSystem) {
		return notFoundError(notFoundText)
	}
	if version != "" && !strings.HasPrefix(loadedVersion, strings.TrimSpace(version)) {
		return notFoundError(notFoundText)
	}
	return nil
}

func normalizeCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// now is overridable in tests; production code always uses time.Now().
var now = time.Now
