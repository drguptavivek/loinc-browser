// Package udp implements Mode E (docs/FHIR_TERMINOLOGY_PLAN.md §6): a compact JSON-over-UDP
// micro-protocol for the same-host/DC case where even a Unix socket's overhead is unwanted. One
// request datagram maps to zero or one response datagram, calling the same pkg/terminology
// Service methods the HTTP and UDS transports use, so there is exactly one code path per
// operation.
package udp

import (
	"context"
	"encoding/json"
	"net"
	"net/url"
	"runtime"
	"strconv"
	"sync"
	"time"

	"loinc-browser/pkg/terminology"
)

// maxDatagramSize is large enough for any request this protocol defines; requests are a handful
// of short fields.
const maxDatagramSize = 65507

// maxResponseSize is the wire budget from §6: a response over this many bytes is replaced with a
// truncated/use-http response instead of being sent partially.
const maxResponseSize = 1400

// requestTimeout bounds each datagram's service call so one slow lookup cannot starve the worker
// pool.
const requestTimeout = 2 * time.Second

// maxIDSize/maxFieldSize bound the request fields the wire-budget's truncated/use-http fallback
// (handleDatagram) has no other way to shrink: "id" is echoed back verbatim in every response
// including the truncated one, and every string field is echoed into that fallback's "href". A
// request that abuses either (up to maxDatagramSize, ~65KB) would make even the "truncated"
// response exceed maxResponseSize, defeating the §6 wire budget the truncation exists to
// guarantee. Both limits are generous for this protocol's real inputs (LOINC codes, urls, short
// filters).
const (
	maxIDSize    = 128
	maxFieldSize = 128
)

// request is the shared shape of every op's input (§6). Unused fields are simply left zero for a
// given op.
type request struct {
	ID      json.RawMessage `json:"id"`
	Op      string          `json:"op"`
	Code    string          `json:"code,omitempty"`
	Display string          `json:"display,omitempty"`
	CodeA   string          `json:"codeA,omitempty"`
	CodeB   string          `json:"codeB,omitempty"`
	URL     string          `json:"url,omitempty"`
	System  string          `json:"system,omitempty"`
	Filter  string          `json:"filter,omitempty"`
	Count   *int            `json:"count,omitempty"`
	Offset  *int            `json:"offset,omitempty"`
}

// withinSizeLimits reports whether every field that gets echoed back into a response (directly,
// or via hrefFor's URL encoding) is small enough that the response is still guaranteed to respect
// maxResponseSize -- see maxIDSize/maxFieldSize.
func (r request) withinSizeLimits() bool {
	if len(r.ID) > maxIDSize {
		return false
	}
	for _, field := range []string{r.Code, r.Display, r.CodeA, r.CodeB, r.URL, r.System, r.Filter} {
		if len(field) > maxFieldSize {
			return false
		}
	}
	return true
}

type datagram struct {
	addr net.Addr
	data []byte
}

// Serve reads request datagrams from conn until ctx is cancelled or conn is closed, dispatching
// each to a bounded worker pool and writing back the compact JSON response. It closes conn on
// ctx.Done so callers get a clean shutdown by cancelling ctx (see cmd/loinc-browser
// serveUntilShutdown).
func Serve(ctx context.Context, conn net.PacketConn, svc *terminology.Service) error {
	// ponytail: one worker per core is plenty for a same-host/DC micro-protocol; add a tunable
	// pool size if this ever saturates under real load.
	workers := runtime.GOMAXPROCS(0)
	jobs := make(chan datagram, workers*4)

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for dg := range jobs {
				resp := handleDatagram(ctx, svc, dg.data)
				if resp != nil {
					_, _ = conn.WriteTo(resp, dg.addr)
				}
			}
		}()
	}

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	buf := make([]byte, maxDatagramSize)
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			close(jobs)
			wg.Wait()
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		data := make([]byte, n)
		copy(data, buf[:n])
		select {
		case jobs <- datagram{addr: addr, data: data}:
		case <-ctx.Done():
		}
	}
}

// handleDatagram decodes one request and dispatches it. Malformed JSON returns nil (no
// response, per §6); every other outcome returns a JSON-encoded response, truncated to
// use-http if it would exceed maxResponseSize.
func handleDatagram(ctx context.Context, svc *terminology.Service, data []byte) []byte {
	var req request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil
	}
	if !req.withinSizeLimits() {
		// No response the size-budget fallback below could build for this request is guaranteed
		// to fit either, so this is dropped exactly like malformed JSON (§6).
		return nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	var body map[string]any
	switch req.Op {
	case "lookup":
		body = handleLookup(reqCtx, svc, req)
	case "validate":
		body = handleValidate(reqCtx, svc, req)
	case "subsumes":
		body = handleSubsumes(reqCtx, svc, req)
	case "translate":
		body = handleTranslate(reqCtx, svc, req)
	case "expand":
		body = handleExpand(reqCtx, svc, req)
	default:
		body = badRequest()
	}
	body["id"] = req.ID

	out, err := json.Marshal(body)
	if err != nil || len(out) <= maxResponseSize {
		return out
	}

	truncated, err := json.Marshal(map[string]any{
		"id":        req.ID,
		"ok":        false,
		"truncated": true,
		"error":     "use-http",
		"href":      hrefFor(req),
	})
	if err != nil {
		return nil
	}
	if len(truncated) <= maxResponseSize {
		return truncated
	}
	// Belt and suspenders alongside withinSizeLimits above: drop "href" rather than send a
	// "truncated" response that itself breaks the wire budget it exists to guarantee.
	minimal, err := json.Marshal(map[string]any{"id": req.ID, "ok": false, "truncated": true, "error": "use-http"})
	if err != nil {
		return nil
	}
	return minimal
}

func handleLookup(ctx context.Context, svc *terminology.Service, req request) map[string]any {
	result, outcomeErr := svc.Lookup(ctx, terminology.LookupParams{Code: req.Code})
	if outcomeErr != nil {
		return outcomeResponse(outcomeErr, req.Code)
	}
	out := map[string]any{
		"ok":      true,
		"code":    topValue(result.Parameter, "code"),
		"display": topValue(result.Parameter, "display"),
		"version": topValue(result.Parameter, "version"),
	}
	if status := findPropertyString(result.Parameter, "STATUS"); status != "" {
		out["status"] = status
	}
	if class := findPropertyCodingDisplay(result.Parameter, "CLASS"); class != "" {
		out["class"] = class
	}
	return out
}

func handleValidate(ctx context.Context, svc *terminology.Service, req request) map[string]any {
	result, outcomeErr := svc.ValidateCode(ctx, terminology.ValidateCodeParams{Code: req.Code, Display: req.Display})
	if outcomeErr != nil {
		return outcomeResponse(outcomeErr, req.Code)
	}
	return map[string]any{"ok": true, "result": topValue(result.Parameter, "result") == "true"}
}

func handleSubsumes(ctx context.Context, svc *terminology.Service, req request) map[string]any {
	result, outcomeErr := svc.Subsumes(ctx, terminology.SubsumesParams{CodeA: req.CodeA, CodeB: req.CodeB})
	if outcomeErr != nil {
		return outcomeResponse(outcomeErr, req.CodeA)
	}
	return map[string]any{"ok": true, "outcome": topValue(result.Parameter, "outcome")}
}

func handleTranslate(ctx context.Context, svc *terminology.Service, req request) map[string]any {
	result, outcomeErr := svc.Translate(ctx, terminology.TranslateParams{Code: req.Code, URL: req.URL, System: req.System})
	if outcomeErr != nil {
		return outcomeResponse(outcomeErr, req.Code)
	}
	matches := []map[string]any{}
	for _, p := range result.Parameter {
		if p.Name != "match" {
			continue
		}
		match := map[string]any{}
		for _, part := range p.Part {
			switch part.Name {
			case "equivalence":
				if part.ValueCode != nil {
					match["equivalence"] = *part.ValueCode
				}
			case "concept":
				if part.ValueCoding != nil {
					match["system"] = part.ValueCoding.System
					match["code"] = part.ValueCoding.Code
					match["display"] = part.ValueCoding.Display
				}
			}
		}
		matches = append(matches, match)
	}
	return map[string]any{"ok": true, "matches": matches}
}

func handleExpand(ctx context.Context, svc *terminology.Service, req request) map[string]any {
	result, outcomeErr := svc.Expand(ctx, terminology.ExpandParams{URL: req.URL, Filter: req.Filter, Offset: req.Offset, Count: req.Count})
	if outcomeErr != nil {
		return outcomeResponse(outcomeErr, req.URL)
	}
	contains := []map[string]any{}
	total := 0
	if result.Expansion != nil {
		total = result.Expansion.Total
		for _, c := range result.Expansion.Contains {
			contains = append(contains, map[string]any{"code": c.Code, "display": c.Display})
		}
	}
	return map[string]any{"ok": true, "total": total, "contains": contains}
}

// outcomeResponse maps a *terminology.OutcomeError to the §6 error vocabulary: "not-found" (with
// the queried identifier echoed back) when the service reports one, "bad-request" for every other
// shape (missing/invalid params, an unavailable store).
func outcomeResponse(err *terminology.OutcomeError, queried string) map[string]any {
	if err.Code == "not-found" {
		return map[string]any{"ok": false, "error": "not-found", "code": queried}
	}
	return map[string]any{"ok": false, "error": "bad-request"}
}

func badRequest() map[string]any {
	return map[string]any{"ok": false, "error": "bad-request"}
}

// topValue reads a top-level Parameters entry's ValueCode or ValueString by name (e.g. "code",
// "display", "version", "outcome", "result").
func topValue(params []terminology.Parameter, name string) string {
	for _, p := range params {
		if p.Name != name {
			continue
		}
		if p.ValueCode != nil {
			return *p.ValueCode
		}
		if p.ValueString != nil {
			return *p.ValueString
		}
	}
	return ""
}

// findPropertyString returns a $lookup "property" part's raw string value for the given property
// code (e.g. "STATUS" -> "ACTIVE"), as opposed to the FHIR-translated top-level "status"
// parameter ("active"/"retired").
func findPropertyString(params []terminology.Parameter, code string) string {
	for _, p := range params {
		if p.Name != "property" {
			continue
		}
		if propertyCode(p) == code {
			for _, part := range p.Part {
				if part.Name == "value" && part.ValueString != nil {
					return *part.ValueString
				}
			}
		}
	}
	return ""
}

// findPropertyCodingDisplay returns a $lookup "property" part's valueCoding.display for the
// given property code (e.g. "CLASS" -> "HEM/BC").
func findPropertyCodingDisplay(params []terminology.Parameter, code string) string {
	for _, p := range params {
		if p.Name != "property" {
			continue
		}
		if propertyCode(p) == code {
			for _, part := range p.Part {
				if part.Name == "value" && part.ValueCoding != nil {
					return part.ValueCoding.Display
				}
			}
		}
	}
	return ""
}

func propertyCode(p terminology.Parameter) string {
	for _, part := range p.Part {
		if part.Name == "code" && part.ValueCode != nil {
			return *part.ValueCode
		}
	}
	return ""
}

// hrefFor builds the URL-encoded HTTP equivalent of a request, for the truncated/use-http
// response (§6).
func hrefFor(req request) string {
	switch req.Op {
	case "lookup":
		return "/fhir/CodeSystem/$lookup?" + url.Values{"code": {req.Code}}.Encode()
	case "validate":
		v := url.Values{"code": {req.Code}}
		if req.Display != "" {
			v.Set("display", req.Display)
		}
		return "/fhir/CodeSystem/$validate-code?" + v.Encode()
	case "subsumes":
		return "/fhir/CodeSystem/$subsumes?" + url.Values{"codeA": {req.CodeA}, "codeB": {req.CodeB}}.Encode()
	case "translate":
		v := url.Values{"code": {req.Code}}
		if req.URL != "" {
			v.Set("url", req.URL)
		}
		if req.System != "" {
			v.Set("system", req.System)
		}
		return "/fhir/ConceptMap/$translate?" + v.Encode()
	case "expand":
		v := url.Values{"url": {req.URL}}
		if req.Filter != "" {
			v.Set("filter", req.Filter)
		}
		if req.Count != nil {
			v.Set("count", strconv.Itoa(*req.Count))
		}
		if req.Offset != nil {
			v.Set("offset", strconv.Itoa(*req.Offset))
		}
		return "/fhir/ValueSet/$expand?" + v.Encode()
	default:
		return ""
	}
}
