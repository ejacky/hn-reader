package fetcher

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

// ErrBlockedURL is returned when a URL points at a non-public address or uses
// a scheme/port we do not allow. Callers should map it to HTTP 400.
var ErrBlockedURL = errors.New("url is not allowed")

// ErrNotStory is returned when an HN item id is not a story.
var ErrNotStory = errors.New("hacker news item is not a story")

var blockedNets = mustCIDRs(
	"0.0.0.0/8",
	"100.64.0.0/10",
	"169.254.0.0/16",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"240.0.0.0/4",
	"2001:db8::/32",
)

func mustCIDRs(cidrs ...string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			panic("invalid blocked CIDR: " + c)
		}
		out = append(out, n)
	}
	return out
}

// IsBlockedIP reports whether ip is loopback, private, link-local, multicast,
// unspecified, or in a reserved/special-purpose range.
func IsBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	for _, n := range blockedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// ValidateURL performs the syntactic and DNS checks used before fetching a
// user-supplied article URL. The authoritative check is repeated in
// safeDialContext to close the DNS-rebinding window.
func ValidateURL(ctx context.Context, raw string) (*url.URL, error) {
	if raw == "" {
		return nil, fmt.Errorf("empty url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("%w: scheme %q", ErrBlockedURL, u.Scheme)
	}
	if u.User != nil {
		return nil, fmt.Errorf("%w: userinfo is not allowed", ErrBlockedURL)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("invalid url: missing host")
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "http" {
			port = "80"
		} else {
			port = "443"
		}
	}
	if port != "80" && port != "443" {
		return nil, fmt.Errorf("%w: port %s", ErrBlockedURL, port)
	}

	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if ip := net.ParseIP(host); ip != nil {
		if IsBlockedIP(ip) {
			return nil, fmt.Errorf("%w: %s", ErrBlockedURL, ip)
		}
		return u, nil
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") ||
		strings.HasSuffix(host, ".home.arpa") {
		return nil, fmt.Errorf("%w: internal hostname %s", ErrBlockedURL, host)
	}
	if !strings.Contains(host, ".") {
		return nil, fmt.Errorf("%w: single-label hostname %s", ErrBlockedURL, host)
	}

	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
	if err != nil {
		// DNS failure is a normal fetch failure, not an SSRF decision.
		return nil, fmt.Errorf("resolve %s: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("resolve %s: no addresses", host)
	}
	allowed := false
	for _, ip := range ips {
		if !IsBlockedIP(ip.IP) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("%w: %s resolves only to blocked addresses", ErrBlockedURL, host)
	}
	return u, nil
}
