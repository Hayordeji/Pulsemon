package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"time"

	"Pulsemon/internal/alerts"
	"Pulsemon/internal/dashboard"
	"Pulsemon/internal/processor"
	"Pulsemon/internal/scheduler"
	"Pulsemon/internal/services"
	"Pulsemon/internal/worker"
	"Pulsemon/pkg/config"
	"Pulsemon/pkg/database"

	"github.com/gin-gonic/gin"
	"github.com/lmittmann/tint"
)

func main() {
	//set slog as default logger
	logger := slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		Level:      slog.LevelDebug,
		TimeFormat: time.Kitchen, // Customize time format
	}))
	slog.SetDefault(logger)

	// Load configuration from environment
	cfg := config.Load()

	// Connect to PostgreSQL and run migrations
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	// Confirm connection is alive
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to get underlying sql.DB: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("database ping failed: %v", err)
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
	handler := services.NewServiceHandler(svc)

	// Create dashboard repository and handler
	dashboardRepo := dashboard.NewDashboardRepository(db)
	dashboardHandler := dashboard.NewDashboardHandler(dashboardRepo)

	// Create alert engine and processor
	alertEngine := alerts.NewAlertEngine(db, cfg)
	proc := processor.NewProcessor(db, results, alertEngine)

	// Start scheduler, worker pool, and result processor in background
	ctx := context.Background()
	go sched.Start(ctx)
	go workerPool.Start(ctx)
	go proc.Start(ctx)

	// Start Gin router
	router := gin.Default()
	handler.RegisterRoutes(router)
	dashboardHandler.RegisterRoutes(router)

	// Run server
	log.Printf("server starting on port %s", cfg.ServerPort)
	router.Run(":" + cfg.ServerPort)
}

