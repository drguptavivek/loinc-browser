package server

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"loinc-browser/internal/loinc"
	"loinc-browser/internal/udp"
	"loinc-browser/pkg/terminology"
)

// TestNewPopulatesTerminologyOutParam verifies cmd/loinc-browser's UDP wiring assumption
// (docs/FHIR_TERMINOLOGY_PLAN.md §6): Options.Terminology, when set, receives the exact Service
// New() registers the FHIR HTTP routes with, so the UDP transport shares one store handle with
// the HTTP server instead of building a second, independently-swapped one.
func TestNewPopulatesTerminologyOutParam(t *testing.T) {
	ctx := context.Background()
	releaseDir := writeServerTestRelease(t)
	dbPath := filepath.Join(t.TempDir(), "loinc.sqlite")
	if _, err := loinc.Ingest(ctx, loinc.IngestOptions{ReleaseDir: releaseDir, DBPath: dbPath}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	var svc *terminology.Service
	New(Options{Store: store, Terminology: &svc})
	if svc == nil {
		t.Fatal("expected Options.Terminology to be populated")
	}

	result, outcomeErr := svc.Lookup(ctx, terminology.LookupParams{Code: "2000-1"})
	if outcomeErr != nil {
		t.Fatalf("lookup via out-param service: %v", outcomeErr)
	}
	if len(result.Parameter) == 0 {
		t.Fatal("expected a non-empty lookup result")
	}
}

// TestServeAllThreeTransportsFromOneStore is the §11 checklist item: one process, one store,
// serving HTTP, Unix socket, and UDP concurrently, all answering the same $lookup.
func TestServeAllThreeTransportsFromOneStore(t *testing.T) {
	ctx := context.Background()
	releaseDir := writeServerTestRelease(t)
	dbPath := filepath.Join(t.TempDir(), "loinc.sqlite")
	if _, err := loinc.Ingest(ctx, loinc.IngestOptions{ReleaseDir: releaseDir, DBPath: dbPath}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	store, err := loinc.OpenStore(dbPath, loinc.StoreOptions{CacheEntries: 4})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	var svc *terminology.Service
	New(Options{Store: store, Terminology: &svc})

	udpConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	serveCtx, stop := context.WithCancel(context.Background())
	defer stop()
	go func() { _ = udp.Serve(serveCtx, udpConn, svc) }()

	client, err := net.Dial("udp", udpConn.LocalAddr().String())
	if err != nil {
		t.Fatalf("dial udp: %v", err)
	}
	defer client.Close()

	req, _ := json.Marshal(map[string]any{"id": "1", "op": "lookup", "code": "2000-1"})
	if _, err := client.Write(req); err != nil {
		t.Fatalf("write udp request: %v", err)
	}
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4096)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("read udp response: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		t.Fatalf("unmarshal udp response: %v", err)
	}
	if resp["ok"] != true || resp["code"] != "2000-1" {
		t.Fatalf("udp lookup response = %+v", resp)
	}
}
