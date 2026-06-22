// Command coordinator is the CallToVerify control plane: it owns verification
// sessions, the number pool, inbound matching, and developer webhooks.
//
// This entrypoint currently boots an HTTP server with health endpoints and a
// stubbed v1 API. Session persistence and matching land in Phase 1.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Eshpelin/calltoverify/coordinator/internal/api"
	"github.com/Eshpelin/calltoverify/coordinator/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           api.NewRouter(logger, cfg),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("coordinator listening", "addr", cfg.ListenAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "err", err)
	}
}
