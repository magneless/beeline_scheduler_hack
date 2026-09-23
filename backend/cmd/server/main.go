package main

import (
	"context"
	"errors"
	"fmt"
	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/data"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/httpapi"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/plans"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/runs"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/storage"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/testkit"
)

func main() {
	if e := run(); e != nil {
		slog.Error("server stopped", "error", e)
		os.Exit(1)
	}
}
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	mode := env("DEPENDENCY_MODE", "integrated")
	if mode != "stub" && mode != "integrated" && mode != "integrated-demo" {
		return fmt.Errorf("DEPENDENCY_MODE must be integrated or stub")
	}
	solverMode := env("SOLVER_MODE", "optimized")
	var solveMode c.SolveMode
	switch solverMode {
	case string(c.SolveModeBaseline):
		solveMode = c.SolveModeBaseline
	case string(c.SolveModeOptimized):
		solveMode = c.SolveModeOptimized
	default:
		return fmt.Errorf("SOLVER_MODE must be baseline or optimized")
	}
	startup, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	store, e := storage.Open(startup, url)
	if e != nil {
		return e
	}
	defer store.Close()
	if e = store.Migrate(startup); e != nil {
		return e
	}
	release, e := store.AcquireWorker(startup)
	if e != nil {
		return e
	}
	defer release()
	if e = store.Recover(startup); e != nil {
		return e
	}
	fixture, e := testkit.Load(env("FIXTURE_PATH", "docs/contracts/examples/backend_flow.json"))
	if e != nil {
		return e
	}
	importer := &data.Importer{Geo: testkit.Geocoder{}, Root: env("DATASET_DIR", "datasets/original"), Prepared: map[string]c.Snapshot{"contract-example": fixture.Snapshot}}
	api := &httpapi.Server{Store: store, Importer: importer}
	var planService c.PlanService = &testkit.Plans{Reader: store, Fixture: fixture}
	if mode != "stub" {
		geodata, issues, err := configuredGeo()
		if err != nil {
			return err
		}
		importer.Geo = geodata
		service, err := plans.New(store, geodata, planner.New(), plans.Options{TimeLimitMS: 1000, Mode: solveMode, Issues: issues})
		if err != nil {
			return err
		}
		planService = service
	}
	worker := &runs.Worker{Store: store, Plans: planService, Timeout: 5 * time.Minute}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); worker.Serve(ctx) }()
	defer wg.Wait()
	defer stop()
	server := &http.Server{Addr: env("HTTP_ADDR", "127.0.0.1:8080"), Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 6 * time.Minute, IdleTimeout: 60 * time.Second}
	done := make(chan error, 1)
	go func() {
		slog.Info("integration mode", "mode", mode, "geodata", env("GEO_PROVIDER", "osm"), "solver", solveMode)
		slog.Info("HTTP listening", "address", server.Addr)
		done <- server.ListenAndServe()
	}()
	select {
	case e := <-done:
		if !errors.Is(e, http.ErrServerClosed) {
			return e
		}
		return nil
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
