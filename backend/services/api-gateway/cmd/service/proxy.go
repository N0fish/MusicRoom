package main

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func mustNewReverseProxy(target string) http.Handler {
	u, err := url.Parse(target)
	if err != nil {
		log.Fatalf("api-gateway: invalid service URL %q: %v", target, err)
	}
	proxy := httputil.NewSingleHostReverseProxy(u)

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
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return xr
	}
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		return xf
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
