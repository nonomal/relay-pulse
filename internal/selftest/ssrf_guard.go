package selftest

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
)

// SSRFGuard provides SSRF (Server-Side Request Forgery) protection
// by validating URLs and preventing access to private networks
type SSRFGuard struct {
	privateIPRanges []*net.IPNet
}

// NewSSRFGuard creates a new SSRF guard with predefined private IP ranges
func NewSSRFGuard() *SSRFGuard {
	privateRanges := []string{
		"10.0.0.0/8",     // Private network (Class A)
		"172.16.0.0/12",  // Private network (Class B)
		"192.168.0.0/16", // Private network (Class C)
		"127.0.0.0/8",    // Loopback
		"169.254.0.0/16", // Link-local (Cloud metadata services)
		"0.0.0.0/8",      // "This" network
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local addresses
		"fe80::/10",      // IPv6 link-local addresses
	}

	var ipNets []*net.IPNet
	for _, cidr := range privateRanges {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			ipNets = append(ipNets, block)
		}
	}

	return &SSRFGuard{
		privateIPRanges: ipNets,
	}
}

// ValidateURL validates a URL for SSRF protection
// Returns an error if the URL is not safe
func (g *SSRFGuard) ValidateURL(rawURL string) error {
	// Parse URL
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// 1. HTTPS only
	if u.Scheme != "https" {
		return fmt.Errorf("only HTTPS is allowed, got scheme: %s", u.Scheme)
	}

	// 2. Forbid userinfo (user:pass@host)
	if u.User != nil {
		return fmt.Errorf("userinfo not allowed in URL")
	}

	// 3. Get hostname
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("missing hostname in URL")
	}

	// 4. Forbid IP addresses (must be domain name)
	if g.isIPAddress(host) {
		return fmt.Errorf("IP addresses not allowed, use domain names only")
	}

	// 5. DNS resolution and private IP detection
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("DNS lookup failed for %s: %w", host, err)
	}

	if len(ips) == 0 {
		return fmt.Errorf("DNS lookup returned no IP addresses for %s", host)
	}

	// Check if any resolved IP is private
	for _, ip := range ips {
		if g.isPrivateIP(ip) {
			return fmt.Errorf("domain %s resolves to private IP: %s", host, ip)
		}
	}

	return nil
}

// isIPAddress checks if a hostname is an IP address (IPv4 or IPv6)
func (g *SSRFGuard) isIPAddress(host string) bool {
	// Try to parse as IP address
	ip := net.ParseIP(host)
	if ip != nil {
		return true
	}

	// Check for IPv4 pattern (simple regex)
	ipv4Pattern := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	if ipv4Pattern.MatchString(host) {
		return true
	}

	// Check for IPv6 pattern (contains colons)
	if regexp.MustCompile(`:`).MatchString(host) {
		return true
	}

	return false
}

// isPrivateIP checks if an IP address is in a private range
func (g *SSRFGuard) isPrivateIP(ip net.IP) bool {
	// Check against all private ranges
	for _, block := range g.privateIPRanges {
		if block.Contains(ip) {
			return true
		}
	}

	return false
}
