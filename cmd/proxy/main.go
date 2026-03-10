package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"proxy-service-component/internal/proxy"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	cfg, err := proxy.LoadConfigFromEnv()
	if err != nil {
		logger.Fatalf("config error: %v", err)
	}

	h, err := proxy.NewHandler(cfg, logger)
	if err != nil {
		logger.Fatalf("init error: %v", err)
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Printf("listening on %s, upstream=%s", cfg.ListenAddr, cfg.UpstreamURL.String())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Printf("shutdown signal received")
	case err := <-errCh:
		logger.Fatalf("server error: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

