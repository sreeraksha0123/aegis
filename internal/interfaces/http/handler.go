package handler

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/SilentPlaces/rate_limiter/internal/application/ports"
	"github.com/SilentPlaces/rate_limiter/internal/application/service"
)

// HTTPHandler applies rate limiting before reverse proxying requests to backend Nginx.
type HTTPHandler struct {
	LimiterService *service.LimiterService
	Logger         ports.Logger
	Proxy          *httputil.ReverseProxy
	BackendURL     *url.URL
}

func NewHTTPHandler(limiter *service.LimiterService, log ports.Logger, backend string) (*HTTPHandler, error) {
	parsedURL, err := url.Parse(backend)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(parsedURL)

	// Customize director to preserve forwarded headers
	proxy.Director = func(req *http.Request) {
		log.Info("director called", ports.Field{Key: "url", Val: req.URL.String()})

		// Preserve original path instead of letting originalDirector override it
		req.URL.Scheme = parsedURL.Scheme
		req.URL.Host = parsedURL.Host
		req.URL.Path = req.URL.Path // <- this keeps the original path
		req.URL.RawQuery = req.URL.RawQuery
		req.Host = parsedURL.Host

		// Preserve client's IP chain
		if xfwd := req.Header.Get("X-Forwarded-For"); xfwd == "" {
			req.Header.Set("X-Forwarded-For", getClientIP(req))
		}

		// Optional debug
		req.Header.Set("X-Rate-Limiter", "checked")
	}

	return &HTTPHandler{
		LimiterService: limiter,
		Logger:         log,
		Proxy:          proxy,
		BackendURL:     parsedURL,
	}, nil
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Logger.Info("proxying request", ports.Field{Key: "url", Val: r.URL.String()}, ports.Field{Key: "method", Val: r.Method})
	clientIP := getClientIP(r)
	key := r.Header.Get("X-Rate-Limit-Rule")
	h.Logger.Info("checking rate limit", ports.Field{Key: "ip", Val: clientIP}, ports.Field{Key: "route", Val: key})

	info, err := h.LimiterService.AllowWithInfo(r.Context(), clientIP, key)
	if err != nil {
		h.Logger.Error("limiter check failed", ports.Field{Key: "err", Val: err})
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.setRateLimitHeaders(w, info)

	if !info.Allowed {
		h.Logger.Info("rate limit exceeded",
			ports.Field{Key: "ip", Val: clientIP},
			ports.Field{Key: "route key", Val: key},
			ports.Field{Key: "method", Val: r.Method},
			ports.Field{Key: "path", Val: r.URL.Path},
		)
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	h.Logger.Info("rate limit not exceeded",
		ports.Field{Key: "ip", Val: clientIP},
		ports.Field{Key: "route key", Val: key},
		ports.Field{Key: "method", Val: r.Method},
		ports.Field{Key: "path", Val: r.URL.Path},
	)

	h.Logger.Info("proxying allowed request",
		ports.Field{Key: "ip", Val: clientIP},
		ports.Field{Key: "route", Val: key},
	)

	h.Proxy.ServeHTTP(w, r)
}

func (h *HTTPHandler) setRateLimitHeaders(w http.ResponseWriter, info ports.RateLimitInfo) {
	if info.Limit > 0 {
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", info.Limit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", info.Remaining))
	}
	if info.ResetTime > 0 {
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", info.ResetTime))
	}
}

func getClientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i != -1 {
		ip = ip[:i]
	}
	return ip
}
