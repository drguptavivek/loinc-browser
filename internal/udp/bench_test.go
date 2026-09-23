package udp

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"

	"loinc-browser/internal/loinc"
	"loinc-browser/pkg/terminology"
)

// BenchmarkUDPLookup measures a full request/response round trip for "lookup" over a real UDP
// socket against the real local DB (docs/FHIR_TERMINOLOGY_PLAN.md §2 Mode E budget: ≤1ms), on
// top of the in-process cost pkg/terminology's BenchmarkLookupTerm already measures.
func BenchmarkUDPLookup(b *testing.B) {
	dbPath := os.Getenv("LOINC_TEST_DB")
	if dbPath == "" {
		b.Skip("LOINC_TEST_DB is not set; skipping UDP round-trip benchmark")
	}
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 4})
	if err != nil {
		b.Fatalf("open %s: %v", dbPath, err)
	}
	b.Cleanup(func() { store.Close() })
	svc := terminology.NewService(func() (*loinc.Store, error) { return store, nil })

	serverConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("listen udp: %v", err)
	}
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = Serve(ctx, serverConn, svc)
	}()
	b.Cleanup(func() {
		stop()
		<-done
	})

	conn, err := net.Dial("udp", serverConn.LocalAddr().String())
	if err != nil {
		b.Fatalf("dial udp: %v", err)
	}
	b.Cleanup(func() { conn.Close() })

	req, err := json.Marshal(map[string]any{"id": "bench", "op": "lookup", "code": "718-7"})
	if err != nil {
		b.Fatalf("marshal request: %v", err)
	}
	buf := make([]byte, maxDatagramSize)
	roundTrip := func() {
		if _, err := conn.Write(req); err != nil {
			b.Fatalf("write request: %v", err)
		}
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, err := conn.Read(buf); err != nil {
			b.Fatalf("read response: %v", err)
		}
	}
	roundTrip() // warm-up

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		roundTrip()
	}
}
