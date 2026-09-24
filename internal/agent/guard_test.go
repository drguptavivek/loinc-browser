package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDialControlLocalOnlyRejectsMetadataAndPublicAllowsPrivate(t *testing.T) {
	cases := []struct {
		name    string
		address string
		allow   bool
	}{
		{"cloud metadata (link-local)", "169.254.169.254:80", false},
		{"public IP", "93.184.216.34:80", false},
		{"IPv6 link-local", "[fe80::1]:80", false},
		{"loopback", "127.0.0.1:1234", true},
		{"RFC1918 10.x", "10.0.0.5:1234", true},
		{"RFC1918 192.168.x", "192.168.1.5:1234", true},
		{"IPv6 loopback", "[::1]:1234", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := dialControlLocalOnly("tcp", c.address, nil)
			if allowed := err == nil; allowed != c.allow {
				t.Fatalf("dialControlLocalOnly(%q) err = %v, want allow=%v", c.address, err, c.allow)
			}
		})
	}
}

func TestNewHTTPClientDoesNotFollowRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer redirector.Close()

	client := NewHTTPClient(true)
	resp, err := client.Get(redirector.URL)
	if err != nil {
		t.Fatalf("GET redirector: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("status = %d, want %d (redirect not followed)", resp.StatusCode, http.StatusFound)
	}
}

func TestNewHTTPClientLocalOnlyRefusesNonLocalDial(t *testing.T) {
	client := NewHTTPClient(true)
	// 169.254.169.254 is never local/private (see isPrivateOrLocal); the request must fail at
	// dial time rather than actually attempting the connection.
	_, err := client.Get("http://169.254.169.254/latest/meta-data/")
	if err == nil {
		t.Fatal("expected local-only client to refuse a link-local metadata address, got nil error")
	}
}
