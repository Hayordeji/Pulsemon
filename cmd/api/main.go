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

	"Pulsemon/internal/alerts"
	"Pulsemon/internal/auth"
	"Pulsemon/internal/dashboard"
	"Pulsemon/internal/health"
	"Pulsemon/internal/processor"
	"Pulsemon/internal/purge"
	"Pulsemon/internal/scheduler"
	"Pulsemon/internal/services"
	"Pulsemon/internal/worker"
	"Pulsemon/pkg/config"
	"Pulsemon/pkg/database"
	"Pulsemon/pkg/middleware"

	"github.com/gin-gonic/gin"
	"github.com/lmittmann/tint"

	docs "Pulsemon/docs"

	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           Pulsemon
// @version         1.0
// @description     A multi-tenant backend that probes HTTP/HTTPS endpoints,
//
//	tracks latency, monitors SLA compliance, inspects SSL
//	certificates, and sends email alerts.
//
// @host            localhost:8080
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     JWT token.
func main() {
	//set slog as default logger
	logger := slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		Level:      slog.LevelDebug,
		TimeFormat: time.Kitchen, // Customize time format
	}))
	slog.SetDefault(logger)

	// Load configuration from environment
	cfg := config.Load()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration",
			"error", err)
		os.Exit(1)
	}

	// Connect to PostgreSQL and run migrations
	db, err := database.Connect(cfg)
	if err != nil {
		slog.Error("database connection failed",
			"error", err)
		os.Exit(1)
	}

	// Confirm connection is alive
	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("failed to get underlying sql.DB",
			"error", err)
		os.Exit(1)
	}
	if err := sqlDB.Ping(); err != nil {
		slog.Error("database ping failed",
			"error", err)
		os.Exit(1)
	}
	slog.Info("database connected and migrations applied successfully")

	// Create channels
	jobs := make(chan scheduler.ProbeJob, 100)
	results := make(chan worker.ProbeResult, 100)

	// Create scheduler
	sched := scheduler.NewScheduler(db, jobs)

	// Create worker pool
	prober := worker.NewHTTPProber()
	workerPool := worker.NewWorkerPool(jobs, results, cfg.WorkerPoolSize, prober)

	// Register services, repositories and handlers
	repo := services.NewServiceRepository(db)
	svc := services.NewServiceService(repo, sched.Events())
	serviceHandler := services.NewServiceHandler(svc)

	// Create dashboard repository and handler
	dashboardRepo := dashboard.NewDashboardRepository(db)
	dashboardHandler := dashboard.NewDashboardHandler(dashboardRepo)

	// Create auth services,repository and handler
	authRepo := auth.NewAuthRepository(db)
	authSvc := auth.NewAuthService(authRepo, cfg)
	authHandler := auth.NewAuthHandler(authSvc)

	// Create alert engine and processor
	alertEngine := alerts.NewAlertEngine(db, cfg)
	proc := processor.NewProcessor(db, results, alertEngine)

	// Create purger
	purger := purge.NewPurger(db)

	// Create cancellable context with OS signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-quit
		slog.Info("shutdown signal received",
			"signal", sig.String())
		cancel()
	}()

	// Start background goroutines
	go sched.Start(ctx)
	go workerPool.Start(ctx)
	go proc.Start(ctx)
	go purger.Start(ctx)

	// Create health handler
	healthHandler := health.NewHealthHandler(db)

	// Create rate limiter
	rateLimiter := middleware.NewRateLimiter()

	// Start Gin router
	router := gin.Default()

	// Apply global rate limit to all routes
	router.Use(rateLimiter.Global())

	//Setup  scalar
	if cfg.AppEnv != "production" {
		docs.SwaggerInfo.BasePath = "/api/v1"
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	}

	// Unprotected routes
	v1 := router.Group("/api/v1")
	{
		healthHandler.RegisterRoutes(v1)
		authHandler.RegisterRoutes(v1, rateLimiter)
	}

	// Protected routes with JWT middleware
	api := router.Group("/api/v1", middleware.AuthMiddleware(cfg.JWTSecret))
	{
		dashboardHandler.RegisterRoutes(api, rateLimiter)
		serviceHandler.RegisterRoutes(api, rateLimiter)
	}

	// Graceful HTTP server
	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error",
				"error", err)
			cancel()
		}
	}()

	slog.Info("server started",
		"port", cfg.ServerPort)

	<-ctx.Done()

	slog.Info("shutting down server gracefully")

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown",
			"error", err)
	}

	slog.Info("server stopped")
}
