package main

import (
	"Pulsemon/models"
	"Pulsemon/pkg/config"
	"Pulsemon/pkg/database"
	"log"
)

func main() {

	// Load configuration from environment
	cfg := config.Load()

	// Connect to PostgreSQL
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

	log.Println("database connected successfully")

	// Run migrations — creates or updates tables to match model definitions
	err = db.AutoMigrate(
		&models.User{},
		&models.Service{},
		&models.ProbeResult{},
		&models.Alert{},
	)
	if err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	log.Println("migrations ran successfully")

	log.Printf("server starting on port %s", cfg.ServerPort)
}
