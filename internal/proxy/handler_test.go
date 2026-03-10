package proxy

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestHealthz(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	u, _ := url.Parse(upstream.URL)
	h, err := NewHandler(Config{
		ListenAddr:      ":0",
		UpstreamURL:     u,
		HealthPath:      "/healthz",
		ShutdownTimeout: 1 * time.Second,
		RequestTimeout:  0,
	}, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://example/healthz", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); body != "ok\n" {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestProxyAddsRequestIDAndForwardedHeaders(t *testing.T) {
	var gotXFF, gotXFH, gotXFP, gotRID string
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotXFF = r.Header.Get("X-Forwarded-For")
		gotXFH = r.Header.Get("X-Forwarded-Host")
		gotXFP = r.Header.Get("X-Forwarded-Proto")
		gotRID = r.Header.Get("X-Request-Id")
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("upstream\n"))
	}))
	t.Cleanup(upstream.Close)

	u, _ := url.Parse(upstream.URL)
	h, err := NewHandler(Config{
		ListenAddr:      ":0",
		UpstreamURL:     u,
		HealthPath:      "/healthz",
		ShutdownTimeout: 1 * time.Second,
		RequestTimeout:  0,
	}, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://example/some/path", nil)
	req.RemoteAddr = "203.0.113.9:12345"
	req.Host = "original.example"

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	if gotPath != "/some/path" {
		t.Fatalf("expected path /some/path, got %q", gotPath)
	}
	if gotXFF == "" || !strings.Contains(gotXFF, "203.0.113.9") {
		t.Fatalf("expected X-Forwarded-For to contain client ip, got %q", gotXFF)
	}
	if gotXFH != "original.example" {
		t.Fatalf("expected X-Forwarded-Host=original.example, got %q", gotXFH)
	}
	if gotXFP == "" {
		t.Fatalf("expected X-Forwarded-Proto set, got empty")
	}
	if gotRID == "" {
		t.Fatalf("expected X-Request-Id set, got empty")
	}
	if rr.Header().Get("X-Request-Id") == "" {
		t.Fatalf("expected response to include X-Request-Id header")
	}
}

func TestProxyRequestTimeout(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	u, _ := url.Parse(upstream.URL)
	h, err := NewHandler(Config{
		ListenAddr:      ":0",
		UpstreamURL:     u,
		HealthPath:      "/healthz",
		ShutdownTimeout: 1 * time.Second,
		RequestTimeout:  50 * time.Millisecond,
	}, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "http://example/slow", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusGatewayTimeout && rr.Code != http.StatusBadGateway {
		t.Fatalf("expected 504 or 502, got %d", rr.Code)
	}
}

