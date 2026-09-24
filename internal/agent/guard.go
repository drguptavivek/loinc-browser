package agent

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// CheckBaseURL validates an LLM endpoint base URL: http/https only, and when localOnly, only a
// host that resolves to loopback or RFC1918/IPv6-ULA private, or literally "localhost". This is
// an early, friendly check run on every save and every use of a base URL; it inspects the URL's
// declared host before any request is made, so it cannot by itself stop a DNS-rebinding attack
// (a host that resolves to a private IP here but a different IP by the time the request dials).
// NewHTTPClient closes that gap by checking the IP actually dialed.
func CheckBaseURL(raw string, localOnly bool) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("endpoint base URL is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid endpoint base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("endpoint base URL must use http or https")
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("endpoint base URL must include a host")
	}
	if !localOnly {
		return nil
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("endpoint host %q did not resolve: %w", host, err)
	}
	for _, ip := range ips {
		if isPrivateOrLocal(ip) {
			return nil
		}
	}
	return fmt.Errorf("endpoint host %q is not local or private; local-only mode refuses it (set LOINC_AGENT_LLM_LOCAL_ONLY=false to allow remote endpoints)", host)
}

// dialControlLocalOnly is a net.Dialer.Control func: it runs after the socket is created but
// before connect(), against the actual address about to be dialed rather than the URL's declared
// host, so it also catches a host that resolved to something allowed when CheckBaseURL ran but
// resolves elsewhere by the time of the real request (DNS rebinding). It ignores c (raw socket
// control is not needed here), which also makes it callable directly in tests.
func dialControlLocalOnly(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid dial address %q: %w", address, err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !isPrivateOrLocal(ip) {
		return fmt.Errorf("connection to %q refused: local-only mode allows only loopback or private addresses", address)
	}
	return nil
}

// isPrivateOrLocal reports whether ip is loopback or RFC1918/IPv6-ULA private (net.IP.IsPrivate
// covers both). Link-local unicast/multicast (169.254.0.0/16, fe80::/10 — how cloud metadata
// endpoints such as 169.254.169.254 are reached), the unspecified address, and multicast are
// deliberately NOT included: a server that only trusts "local" networks must not trust those.
func isPrivateOrLocal(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate()
}

// NewHTTPClient builds the *http.Client every agent LLM request must go through. When localOnly,
// its Dialer.Control checks the actual IP being connected to (not just the URL's declared host,
// which CheckBaseURL already validated but which DNS rebinding can change between check and
// dial), and it never follows redirects — a redirect response is handed back to the caller
// instead of being re-dialed, which would otherwise let a local endpoint redirect the client
// anywhere.
func NewHTTPClient(localOnly bool) *http.Client {
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	if localOnly {
		dialer.Control = dialControlLocalOnly
	}
	return &http.Client{
		Timeout:   5 * time.Minute,
		Transport: &http.Transport{DialContext: dialer.DialContext},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
