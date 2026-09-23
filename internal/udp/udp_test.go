package udp

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"loinc-browser/internal/loinc"
	"loinc-browser/pkg/terminology"
)

func newTestService(t *testing.T) *terminology.Service {
	t.Helper()
	ctx := context.Background()
	releaseDir := writeUDPTestRelease(t)
	dbPath := filepath.Join(t.TempDir(), "loinc.sqlite")
	if _, err := loinc.Ingest(ctx, loinc.IngestOptions{ReleaseDir: releaseDir, DBPath: dbPath}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return terminology.NewService(func() (*loinc.Store, error) { return store, nil })
}

// startTestServer starts Serve on a loopback UDP socket and returns a connected client socket
// plus a cancel func that shuts the server down.
func startTestServer(t *testing.T, svc *terminology.Service) (client *net.UDPConn, cancel func()) {
	t.Helper()
	serverConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = Serve(ctx, serverConn, svc)
	}()

	clientConn, err := net.Dial("udp", serverConn.LocalAddr().String())
	if err != nil {
		stop()
		t.Fatalf("dial udp: %v", err)
	}
	udpConn := clientConn.(*net.UDPConn)
	t.Cleanup(func() {
		stop()
		<-done
		udpConn.Close()
	})
	return udpConn, stop
}

func roundTrip(t *testing.T, conn *net.UDPConn, req any) map[string]any {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, err := conn.Write(body); err != nil {
		t.Fatalf("write request: %v", err)
	}
	buf := make([]byte, 65507)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		t.Fatalf("unmarshal response: %v (%s)", err, buf[:n])
	}
	return resp
}

func TestLookupRoundTrip(t *testing.T) {
	svc := newTestService(t)
	conn, _ := startTestServer(t, svc)

	resp := roundTrip(t, conn, map[string]any{"id": "42", "op": "lookup", "code": "10000-1"})
	if resp["id"] != "42" {
		t.Fatalf("id not echoed: %+v", resp)
	}
	if resp["ok"] != true {
		t.Fatalf("ok = %+v", resp)
	}
	if resp["code"] != "10000-1" {
		t.Fatalf("code = %+v", resp)
	}
	if resp["display"] != "Cholesterol [Mass/volume] in Serum" {
		t.Fatalf("display = %+v", resp)
	}
	if resp["status"] != "ACTIVE" {
		t.Fatalf("status = %+v (want raw STATUS field)", resp)
	}
	if resp["class"] != "CHEM" {
		t.Fatalf("class = %+v", resp)
	}
}

func TestLookupNotFound(t *testing.T) {
	svc := newTestService(t)
	conn, _ := startTestServer(t, svc)

	resp := roundTrip(t, conn, map[string]any{"id": "47", "op": "lookup", "code": "9999-9"})
	if resp["ok"] != false || resp["error"] != "not-found" || resp["code"] != "9999-9" {
		t.Fatalf("not-found response = %+v", resp)
	}
}

func TestValidateRoundTrip(t *testing.T) {
	svc := newTestService(t)
	conn, _ := startTestServer(t, svc)

	resp := roundTrip(t, conn, map[string]any{"id": "43", "op": "validate", "code": "10000-1"})
	if resp["ok"] != true || resp["result"] != true {
		t.Fatalf("validate response = %+v", resp)
	}

	badDisplay := roundTrip(t, conn, map[string]any{"id": "43b", "op": "validate", "code": "10000-1", "display": "nonsense"})
	if badDisplay["ok"] != true || badDisplay["result"] != false {
		t.Fatalf("validate bad-display response = %+v", badDisplay)
	}
}

func TestSubsumesRoundTrip(t *testing.T) {
	svc := newTestService(t)
	conn, _ := startTestServer(t, svc)

	resp := roundTrip(t, conn, map[string]any{"id": "44", "op": "subsumes", "codeA": "LP2000-1", "codeB": "10000-1"})
	if resp["ok"] != true || resp["outcome"] != "subsumes" {
		t.Fatalf("subsumes response = %+v", resp)
	}
}

func TestTranslateRoundTrip(t *testing.T) {
	svc := newTestService(t)
	conn, _ := startTestServer(t, svc)

	resp := roundTrip(t, conn, map[string]any{"id": "45", "op": "translate", "code": "20000-8", "system": "http://loinc.org"})
	if resp["ok"] != true {
		t.Fatalf("translate response = %+v", resp)
	}
	matches, ok := resp["matches"].([]any)
	if !ok || len(matches) == 0 {
		t.Fatalf("translate matches = %+v", resp)
	}
	match := matches[0].(map[string]any)
	if match["code"] != "10000-1" || match["system"] != "http://loinc.org" {
		t.Fatalf("translate match = %+v", match)
	}
}

func TestExpandRoundTrip(t *testing.T) {
	svc := newTestService(t)
	conn, _ := startTestServer(t, svc)

	resp := roundTrip(t, conn, map[string]any{"id": "46", "op": "expand", "url": "http://loinc.org/vs/LL1000-1"})
	if resp["ok"] != true {
		t.Fatalf("expand response = %+v", resp)
	}
	if total, _ := resp["total"].(float64); total != 2 {
		t.Fatalf("expand total = %+v", resp)
	}
	contains, ok := resp["contains"].([]any)
	if !ok || len(contains) != 2 {
		t.Fatalf("expand contains = %+v", resp)
	}
}

func TestExpandTruncatesOversizeResponse(t *testing.T) {
	svc := newTestService(t)
	conn, _ := startTestServer(t, svc)

	resp := roundTrip(t, conn, map[string]any{"id": "48", "op": "expand", "url": "http://loinc.org/vs", "count": 1000})
	if resp["ok"] != false || resp["truncated"] != true || resp["error"] != "use-http" {
		t.Fatalf("truncated response = %+v", resp)
	}
	href, _ := resp["href"].(string)
	if href != "/fhir/ValueSet/$expand?count=1000&url=http%3A%2F%2Floinc.org%2Fvs" {
		t.Fatalf("href = %q", href)
	}
}

func TestUnknownOpIsBadRequest(t *testing.T) {
	svc := newTestService(t)
	conn, _ := startTestServer(t, svc)

	resp := roundTrip(t, conn, map[string]any{"id": "x", "op": "nonsense"})
	if resp["ok"] != false || resp["error"] != "bad-request" {
		t.Fatalf("bad-request response = %+v", resp)
	}
}

func TestMissingRequiredParamIsBadRequest(t *testing.T) {
	svc := newTestService(t)
	conn, _ := startTestServer(t, svc)

	resp := roundTrip(t, conn, map[string]any{"id": "y", "op": "lookup"})
	if resp["ok"] != false || resp["error"] != "bad-request" {
		t.Fatalf("bad-request response = %+v", resp)
	}
}

// TestOversizeFieldGetsNoResponse guards item 8's udp.go review finding: a request field that
// gets echoed back verbatim (directly, or url-encoded into hrefFor's "href") had no size limit,
// so a request with a large enough "filter"/"id" could make even the truncated/use-http fallback
// response exceed maxResponseSize -- defeating the §6 wire-budget guarantee it exists to enforce.
func TestOversizeFieldGetsNoResponse(t *testing.T) {
	svc := newTestService(t)
	conn, _ := startTestServer(t, svc)

	oversizeFilter := make([]byte, maxFieldSize+1)
	for i := range oversizeFilter {
		oversizeFilter[i] = 'a'
	}
	body, err := json.Marshal(map[string]any{"id": "1", "op": "expand", "url": "http://loinc.org/vs", "filter": string(oversizeFilter)})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, err := conn.Write(body); err != nil {
		t.Fatalf("write request: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	buf := make([]byte, 512)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("expected no response to an oversize field, got one")
	}
}

func TestRequestWithinSizeLimits(t *testing.T) {
	ok := request{ID: json.RawMessage(`"1"`), Code: "718-7", Filter: "glucose"}
	if !ok.withinSizeLimits() {
		t.Fatal("expected a normal-size request to pass")
	}

	bigID := request{ID: json.RawMessage(make([]byte, maxIDSize+1))}
	if bigID.withinSizeLimits() {
		t.Fatal("expected an oversize id to fail")
	}

	bigFilter := request{Filter: string(make([]byte, maxFieldSize+1))}
	if bigFilter.withinSizeLimits() {
		t.Fatal("expected an oversize field to fail")
	}
}

func TestMalformedJSONGetsNoResponse(t *testing.T) {
	svc := newTestService(t)
	conn, _ := startTestServer(t, svc)

	if _, err := conn.Write([]byte("{not json")); err != nil {
		t.Fatalf("write malformed request: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	buf := make([]byte, 512)
	if _, err := conn.Read(buf); err == nil {
		t.Fatalf("expected no response to malformed JSON, got one")
	}
}
