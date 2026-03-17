package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"Pulsemon/internal/alerts"
	"Pulsemon/internal/auth"
	"Pulsemon/internal/dashboard"
	"Pulsemon/internal/processor"
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
// @description     Type "Bearer" followed by a space and your JWT token.
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

	// Create auth services,repository and handler
	authRepo := auth.NewAuthRepository(db)
	authSvc := auth.NewAuthService(authRepo, cfg)
	authHandler := auth.NewAuthHandler(authSvc)

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

	//Setup  scalar
	if cfg.AppEnv != "production" {
		docs.SwaggerInfo.BasePath = "/api/v1"
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

		// router.GET("/docs", openapiui.WrapHandler(openapiui.Config{
		// 	SpecURL:      "/docs/swagger.json",
		// 	SpecFilePath: "../docs/swagger.json",
		// 	Title:        "Pulsemon API Documentation V1",
		// 	Theme:        "dark", // or "light ... I prefer dark mode "
		// }))
		// router.Static("/docs/swagger.json", "../docs/swagger.json")
	}

	//Unprotected Routes
	v1 := router.Group("/api/v1")
	{
		dashboardHandler.RegisterRoutes(v1)
		authHandler.RegisterRoutes(v1)
	}
	//Protected Routes
	api := router.Group("/api/v1", middleware.AuthMiddleware("my-secret-key"))
	{
		handler.RegisterRoutes(api)
	}

	// Run server
	log.Printf("server starting on port %s", cfg.ServerPort)
	router.Run(":" + cfg.ServerPort)
}

// PingExample godoc
// @Summary ping example
// @Schemes
// @Description do ping
// @Tags example
// @Accept json
// @Produce json
// @Success 200 {string} Helloworld
// @Router /example/helloworld [get]
func Helloworld(g *gin.Context) {
	g.JSON(http.StatusOK, "helloworld")
}
