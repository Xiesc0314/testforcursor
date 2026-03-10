package proxy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
)

type Handler struct {
	mux *http.ServeMux
}

func NewHandler(cfg Config, logger *log.Logger) (*Handler, error) {
	if cfg.UpstreamURL == nil {
		return nil, errors.New("upstream url is nil")
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          200,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	upstream := *cfg.UpstreamURL
	rp := httputil.NewSingleHostReverseProxy(&upstream)
	rp.Transport = transport

	origDirector := rp.Director
	rp.Director = func(r *http.Request) {
		origDirector(r)
		setForwardedHeaders(r)
		ensureRequestID(r)
		r.Host = upstream.Host
	}

	rp.ModifyResponse = func(resp *http.Response) error {
		removeHopByHopHeaders(resp.Header)
		return nil
	}

	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		status := http.StatusBadGateway
		if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
		}
		logger.Printf("proxy error method=%s path=%s upstream=%s request_id=%s err=%v",
			r.Method, r.URL.Path, cfg.UpstreamURL.String(), r.Header.Get("X-Request-Id"), err)
		http.Error(w, http.StatusText(status), status)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(cfg.HealthPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok\n")
	})

	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		if cfg.RequestTimeout > 0 {
			var cancel context.CancelFunc
			ctx, c := context.WithTimeout(r.Context(), cfg.RequestTimeout)
			cancel = c
			r = r.Clone(ctx)
			defer cancel()
		}

		ensureRequestID(r)
		w.Header().Set("X-Request-Id", r.Header.Get("X-Request-Id"))
		removeHopByHopHeaders(r.Header)

		rw := &statusCapturingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		rp.ServeHTTP(rw, r)

		logger.Printf("request method=%s path=%s status=%d duration_ms=%d request_id=%s",
			r.Method, r.URL.Path, rw.status, time.Since(start).Milliseconds(), r.Header.Get("X-Request-Id"))
	})

	mux.Handle("/", proxyHandler)

	return &Handler{mux: mux}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

type statusCapturingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusCapturingResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func ensureRequestID(r *http.Request) {
	if r.Header.Get("X-Request-Id") != "" {
		return
	}
	var b [16]byte
	_, _ = rand.Read(b[:])
	r.Header.Set("X-Request-Id", hex.EncodeToString(b[:]))
}

func setForwardedHeaders(r *http.Request) {
	xff := r.Header.Get("X-Forwarded-For")
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && ip != "" {
		if xff == "" {
			r.Header.Set("X-Forwarded-For", ip)
		} else {
			r.Header.Set("X-Forwarded-For", xff+", "+ip)
		}
	}

	if r.Header.Get("X-Forwarded-Host") == "" && r.Host != "" {
		r.Header.Set("X-Forwarded-Host", r.Host)
	}
	if r.Header.Get("X-Forwarded-Proto") == "" {
		if r.TLS != nil {
			r.Header.Set("X-Forwarded-Proto", "https")
		} else {
			r.Header.Set("X-Forwarded-Proto", "http")
		}
	}
}

func removeHopByHopHeaders(h http.Header) {
	if c := h.Get("Connection"); c != "" {
		for _, f := range strings.Split(c, ",") {
			if k := strings.TrimSpace(f); k != "" {
				h.Del(k)
			}
		}
	}

	for _, k := range []string{
		"Connection",
		"Proxy-Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade",
	} {
		h.Del(k)
	}
}

