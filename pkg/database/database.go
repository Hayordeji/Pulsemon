package database

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"time"

	"Pulsemon/pkg/config"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Connect establishes a PostgreSQL connection via GORM and runs all pending
// golang-migrate migrations from the migrations/ directory.
func Connect(cfg config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(2 * time.Minute)

	slog.Info("database connection pool configured",
		"max_open_conns", 25,
		"max_idle_conns", 10,
		"conn_max_lifetime", "5m",
		"conn_max_idle_time", "2m")

	// Build the postgres connection URL for golang-migrate.
	migrateDSN := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBSSLMode,
	)

	if err := runMigrations(migrateDSN); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// runMigrations applies all pending SQL migrations from the migrations/ directory.
func runMigrations(dsn string) error {
	migrationsPath := getMigrationsPath()

	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		dsn,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

// getMigrationsPath returns the absolute path to the migrations directory,
// resolved relative to this source file's location so it works regardless
// of the working directory the binary is started from.
func getMigrationsPath() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		// Fallback: assume migrations/ is in the current working directory.
		return "migrations"
	}
	// This file lives at pkg/database/database.go — go up two levels to the project root.
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	convertedPath := filepath.ToSlash(projectRoot)
	return filepath.ToSlash(filepath.Join(convertedPath, "migrations"))
}
