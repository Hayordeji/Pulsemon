package main

import (
	"context"
	"log"

	"Pulsemon/internal/scheduler"
	"Pulsemon/internal/services"
	"Pulsemon/pkg/config"
	"Pulsemon/pkg/database"

	"github.com/gin-gonic/gin"
)

func main() {

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
	log.Println("database connected and migrations applied successfully")

	// Create channels
	jobs := make(chan scheduler.ProbeJob, 100)

	// Create scheduler
	sched := scheduler.NewScheduler(db, jobs)

	// Register services, repositories and handlers
	repo := services.NewServiceRepository(db)
	svc := services.NewServiceService(repo, sched.Events())
	handler := services.NewServiceHandler(svc)

	// Start scheduler in background
	ctx := context.Background()
	go sched.Start(ctx)

	// Start Gin router
	router := gin.Default()
	handler.RegisterRoutes(router)

	// Run server
	log.Printf("server starting on port %s", cfg.ServerPort)
	router.Run(":" + cfg.ServerPort)
}
