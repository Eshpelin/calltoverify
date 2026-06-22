// Command coordinator is the CallToVerify control plane: it owns verification
// sessions, the number pool, inbound matching, and developer webhooks.
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
	"github.com/Eshpelin/calltoverify/coordinator/internal/auth"
	"github.com/Eshpelin/calltoverify/coordinator/internal/config"
	"github.com/Eshpelin/calltoverify/coordinator/internal/ratelimit"
	"github.com/Eshpelin/calltoverify/coordinator/internal/store"
	"github.com/Eshpelin/calltoverify/coordinator/internal/verify"
	"github.com/Eshpelin/calltoverify/coordinator/internal/webhook"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	rootCtx := context.Background()

	st, err := store.New(rootCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("store init", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	if err := waitForDB(rootCtx, st, logger); err != nil {
		logger.Error("database unreachable", "err", err)
		os.Exit(1)
	}
	if err := st.Migrate(rootCtx); err != nil {
		logger.Error("migrate", "err", err)
		os.Exit(1)
	}
	logger.Info("migrations applied")

	wh := webhook.New(logger)
	limiter := ratelimit.New(60, 10) // 60 inbound/min per sender, burst 10
	svc := verify.NewService(st, wh, limiter, cfg.DefaultCodeLen, cfg.DefaultTTL)
	nonces := auth.NewNonceCache(10 * time.Minute)
	server := api.NewServer(logger, cfg, st, svc, nonces)

	if cfg.AdminToken == "" {
		logger.Warn("CTV_ADMIN_TOKEN not set; /admin provisioning API is disabled")
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Background sweep: expire stale pending sessions.
	sweepCtx, stopSweep := context.WithCancel(rootCtx)
	defer stopSweep()
	go expireLoop(sweepCtx, st, logger)

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
	stopSweep()
	ctx, cancel := context.WithTimeout(rootCtx, 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "err", err)
	}
}

// waitForDB retries the initial connection so the Coordinator can start alongside
// Postgres (for example under docker compose) before it is ready.
func waitForDB(ctx context.Context, st *store.Store, logger *slog.Logger) error {
	var err error
	for i := 0; i < 30; i++ {
		if err = st.Ping(ctx); err == nil {
			return nil
		}
		logger.Info("waiting for database", "attempt", i+1)
		time.Sleep(time.Second)
	}
	return err
}

func expireLoop(ctx context.Context, st *store.Store, logger *slog.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := st.ExpireDue(ctx); err != nil {
				logger.Warn("expire sweep", "err", err)
			} else if n > 0 {
				logger.Info("sessions expired", "count", n)
			}
		}
	}
}
