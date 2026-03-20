package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost          string
	DBPort          string
	DBUser          string
	DBPassword      string
	DBName          string
	ServerPort      string
	JWTSecret       string
	ResendAPIKey    string
	ResendFromEmail string
	WorkerPoolSize  int
	AppEnv          string
	AllowedOrigins  string
	AdminEmail      string
	AdminPassword   string
	AdminUsername   string
	AppBaseURL      string
	DBSSLMode       string
}

func Load() Config {
	// go up two levels to reach project root
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	envPath := filepath.Join(projectRoot, ".env")

	err := godotenv.Load(envPath)
	if err != nil {
		panic("Error loading .env file")
	}
	workerPoolSize := 20
	if v, err := strconv.Atoi(os.Getenv("WORKER_POOL_SIZE")); err == nil && v > 0 {
		workerPoolSize = v
	}

	return Config{
		DBHost:          os.Getenv("DB_HOST"),
		DBPort:          os.Getenv("DB_PORT"),
		DBUser:          os.Getenv("DB_USER"),
		DBPassword:      os.Getenv("DB_PASSWORD"),
		DBName:          os.Getenv("DB_NAME"),
		ServerPort:      os.Getenv("SERVER_PORT"),
		JWTSecret:       os.Getenv("JWT_SECRET"),
		ResendAPIKey:    os.Getenv("RESEND_API_KEY"),
		ResendFromEmail: os.Getenv("RESEND_FROM_EMAIL"),
		WorkerPoolSize:  workerPoolSize,
		AppEnv:          os.Getenv("APP_ENV"),
		AllowedOrigins:  os.Getenv("ALLOWED_ORIGINS"),
		AdminEmail:      os.Getenv("ADMIN_EMAIL"),
		AdminPassword:   os.Getenv("ADMIN_PASSWORD"),
		AdminUsername:   os.Getenv("ADMIN_USERNAME"),
		AppBaseURL:      os.Getenv("APP_BASE_URL"),
		DBSSLMode:       os.Getenv("DB_SSL_MODE"),
	}
}

// Validate checks that all required configuration fields are set.
func (c Config) Validate() error {
	required := map[string]string{
		"DB_HOST":           c.DBHost,
		"DB_PORT":           c.DBPort,
		"DB_USER":           c.DBUser,
		"DB_PASSWORD":       c.DBPassword,
		"DB_NAME":           c.DBName,
		"JWT_SECRET":        c.JWTSecret,
		"RESEND_API_KEY":    c.ResendAPIKey,
		"RESEND_FROM_EMAIL": c.ResendFromEmail,
		"SERVER_PORT":       c.ServerPort,
		"ALLOWED_ORIGINS":   c.AllowedOrigins,
		"APP_BASE_URL":      c.AppBaseURL,
	}

	if c.DBSSLMode == "" {
		c.DBSSLMode = "disable"
	}

	for name, value := range required {
		if value == "" {
			return fmt.Errorf("missing required environment variable: %s", name)
		}
	}

	adminFieldsSet := c.AdminEmail != "" || c.AdminPassword != "" || c.AdminUsername != ""
	adminFieldsAllSet := c.AdminEmail != "" && c.AdminPassword != "" && c.AdminUsername != ""
	if adminFieldsSet && !adminFieldsAllSet {
		return fmt.Errorf("admin seed requires ADMIN_EMAIL, ADMIN_PASSWORD and ADMIN_USERNAME to all be set or all be empty")
	}

	return nil
}
