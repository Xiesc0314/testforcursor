package proxy

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ListenAddr      string
	UpstreamURL     *url.URL
	HealthPath      string
	ShutdownTimeout time.Duration
	RequestTimeout  time.Duration
}

func LoadConfigFromEnv() (Config, error) {
	listenAddr := getenv("LISTEN_ADDR", ":8080")
	healthPath := getenv("HEALTH_PATH", "/healthz")

	upstreamRaw := os.Getenv("UPSTREAM_URL")
	if upstreamRaw == "" {
		return Config{}, errors.New("UPSTREAM_URL is required")
	}
	upstreamURL, err := url.Parse(upstreamRaw)
	if err != nil {
		return Config{}, err
	}
	if upstreamURL.Scheme == "" || upstreamURL.Host == "" {
		return Config{}, errors.New("UPSTREAM_URL must include scheme and host, e.g. http://example:8080")
	}

	shutdownTimeout, err := parseDurationSecondsEnv("SHUTDOWN_TIMEOUT_SECONDS", 10)
	if err != nil {
		return Config{}, err
	}
	requestTimeout, err := parseDurationSecondsEnv("REQUEST_TIMEOUT_SECONDS", 0)
	if err != nil {
		return Config{}, err
	}

	return Config{
		ListenAddr:      listenAddr,
		UpstreamURL:     upstreamURL,
		HealthPath:      healthPath,
		ShutdownTimeout: shutdownTimeout,
		RequestTimeout:  requestTimeout,
	}, nil
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

func parseDurationSecondsEnv(k string, defSeconds int) (time.Duration, error) {
	raw := os.Getenv(k)
	if raw == "" {
		return time.Duration(defSeconds) * time.Second, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, errors.New(k + " must be >= 0")
	}
	return time.Duration(n) * time.Second, nil
}

