package web

import (
	"net"
	"net/http"
	"strings"
	"sync"
)

var (
	cfCIDRsOnce sync.Once
	cfNets      []*net.IPNet
)

var cloudflareCIDRs = []string{
	// IPv4
	"173.245.48.0/20",
	"103.21.244.0/22",
	"103.22.200.0/22",
	"103.31.4.0/22",
	"141.101.64.0/18",
	"108.162.192.0/18",
	"190.93.240.0/20",
	"188.114.96.0/20",
	"197.234.240.0/22",
	"198.41.128.0/17",
	"162.158.0.0/15",
	"104.16.0.0/13",
	"104.24.0.0/14",
	"172.64.0.0/13",
	"131.0.72.0/22",
	// IPv6
	"2400:cb00::/32",
	"2606:4700::/32",
	"2803:f800::/32",
	"2405:b500::/32",
	"2405:8100::/32",
	"2a06:98c0::/29",
	"2c0f:f248::/32",
}

func initCFNets() {
	cfCIDRsOnce.Do(func() {
		for _, cidr := range cloudflareCIDRs {
			_, ipNet, err := net.ParseCIDR(cidr)
			if err == nil {
				cfNets = append(cfNets, ipNet)
			}
		}
	})
}

func IsFromCloudflare(remoteAddr string) bool {
	initCFNets()

	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	for _, cfNet := range cfNets {
		if cfNet.Contains(ip) {
			return true
		}
	}
	return false
}

func ExtractRealClientIP(r *http.Request) string {
	remoteAddr := r.RemoteAddr
	fromCF := IsFromCloudflare(remoteAddr)

	if fromCF {
		if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
			return cfIP
		}
		if trueClientIP := r.Header.Get("True-Client-IP"); trueClientIP != "" {
			return trueClientIP
		}
	}

	if fwdFor := r.Header.Get("X-Forwarded-For"); fwdFor != "" {
		ips := strings.Split(fwdFor, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}
