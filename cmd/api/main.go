package main

import (
	"Pulsemon/internal/services"
	"Pulsemon/pkg/config"
	"Pulsemon/pkg/database"
	"log"

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

	//Register services, repositories and handlers
	repo := services.NewServiceRepository(db)
	svc := services.NewServiceService(repo)
	handler := services.NewServiceHandler(svc)

	router := gin.Default()
	handler.RegisterRoutes(router)

	//Run server
	log.Printf("server starting on port %s", cfg.ServerPort)
	router.Run(":" + cfg.ServerPort)

}
