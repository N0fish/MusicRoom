package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	trustedProxyMu    sync.RWMutex
	trustedProxyCIDRs []netip.Prefix
)

func setTrustedProxyCIDRs(p []netip.Prefix) {
	trustedProxyMu.Lock()
	trustedProxyCIDRs = append([]netip.Prefix(nil), p...)
	trustedProxyMu.Unlock()
}

func isTrustedProxyIP(ip netip.Addr) bool {
	trustedProxyMu.RLock()
	defer trustedProxyMu.RUnlock()
	for _, pr := range trustedProxyCIDRs {
		if pr.Contains(ip) {
			return true
		}
	}
	return false
}

func mustNewReverseProxy(target string) http.Handler {
	u, err := url.Parse(target)
	if err != nil {
		log.Fatalf("api-gateway: invalid service URL %q: %v", target, err)
	}
	fmt.Fprintf(os.Stderr, "url.Parsed: %s, target: %s\n", u.String(), target)
	fmt.Fprintln(os.Stderr, "ENVS", os.Getenv("HTTP_PROXY"), os.Getenv("HTTPS_PROXY"), os.Getenv("NO_PROXY"))
	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.Transport = &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}

	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Forwarded-Proto", "http")
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("proxy to %s error: %v", target, err)
		writeError(w, http.StatusBadGateway, "upstream service unavailable")
	}

	return proxy
}

func clientIP(r *http.Request) string {
	peer := remoteIP(r)

	peerAddr, err := netip.ParseAddr(peer)
	if err != nil || !isTrustedProxyIP(peerAddr) {
		return peer
	}

	if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
		if ip, err := netip.ParseAddr(xr); err == nil {
			return ip.String()
		}
	}

	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		first := strings.TrimSpace(strings.Split(xff, ",")[0])
		if ip, err := netip.ParseAddr(first); err == nil {
			return ip.String()
		}
	}

	return peer
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
