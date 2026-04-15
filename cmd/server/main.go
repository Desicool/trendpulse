package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"trendpulse/internal/api"
	apihandler "trendpulse/internal/api/handler"
	"trendpulse/internal/calculator"
	"trendpulse/internal/calculator/sigmoid"
	"trendpulse/internal/config"
	badgerrepo "trendpulse/internal/repository/badger"
	"trendpulse/internal/scheduler"
)

func main() {
	// --- Config ---
	cfgPath := "configs/config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// --- Storage ---
	store, err := badgerrepo.Open(cfg.Storage.Badger.Dir, cfg.Storage.Badger.InMemory)
	if err != nil {
		slog.Error("failed to open badger store", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	// --- Repositories ---
	trendRepo := badgerrepo.NewTrendRepository(store)
	signalRepo := badgerrepo.NewSignalRepository(store)
	statsRepo := badgerrepo.NewStatsRepository(store)
	categoryIndex := badgerrepo.NewCategoryIndex(store)

	// --- Calculator registry ---
	registry := calculator.NewRegistry()

	ds := cfg.Calculator.DefaultStrategy
	pt := ds.PhaseThresholds
	sigCfg := sigmoid.Config{
		Weights: sigmoid.WeightsConfig{
			ViewAcceleration:  ds.Weights.ViewAcceleration,
			PostGrowthRate:    ds.Weights.PostGrowthRate,
			CreatorGrowthRate: ds.Weights.CreatorGrowthRate,
			EngagementSurge:   ds.Weights.EngagementSurge,
			ViewConcentration: ds.Weights.ViewConcentration,
		},
		Bias:          ds.Bias,
		LookbackShort: ds.LookbackShort.Duration,
		LookbackAccel: ds.LookbackAccel.Duration,
		PhaseThresholds: calculator.PhaseConfig{
			RisingAccelThreshold:      pt.RisingAccelThreshold,
			RisingEngagementThreshold: pt.RisingEngagementThreshold,
			PeakingGrowthRateMax:      pt.PeakingGrowthRateMax,
			PeakingGrowthRateMin:      pt.PeakingGrowthRateMin,
		},
		SignalLookback: cfg.Scheduler.SignalLookback.Duration,
	}
	if err := registry.Register(sigmoid.NewSigmoidV1(sigCfg)); err != nil {
		slog.Error("failed to register sigmoid_v1 strategy", "error", err)
		os.Exit(1)
	}

	// --- Scheduler ---
	schedCfg := scheduler.Config{
		Interval:       cfg.Scheduler.Interval.Duration,
		SignalLookback: cfg.Scheduler.SignalLookback.Duration,
	}
	sched := scheduler.NewScheduler(trendRepo, signalRepo, statsRepo, registry, schedCfg)

	// --- HTTP router ---
	trendHandler := apihandler.NewTrendHandler(
		trendRepo,
		statsRepo,
		cfg.Scheduler.ActiveStrategy,
		cfg.API.DefaultPageSize,
		cfg.API.MaxPageSize,
		cfg.API.RisingDefaultLimit,
		cfg.API.RisingMinScore,
	)
	ingestHandler := apihandler.NewIngestHandler(trendRepo, signalRepo, categoryIndex)

	httpHandler := api.NewRouter(trendHandler, ingestHandler, sched)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      httpHandler,
		ReadTimeout:  cfg.Server.ReadTimeout.Duration,
		WriteTimeout: cfg.Server.WriteTimeout.Duration,
	}

	// --- Background scheduler ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := sched.Run(ctx); err != nil && err != context.Canceled {
			slog.Error("scheduler stopped unexpectedly", "error", err)
		}
	}()

	// --- HTTP server with graceful shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("server shutting down")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
