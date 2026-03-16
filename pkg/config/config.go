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
	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBName         string
	ServerPort     string
	JWTSecret      string
	ResendAPIKey    string
	ResendFromEmail string
	WorkerPoolSize  int
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
		DBHost:         os.Getenv("DB_HOST"),
		DBPort:         os.Getenv("DB_PORT"),
		DBUser:         os.Getenv("DB_USER"),
		DBPassword:     os.Getenv("DB_PASSWORD"),
		DBName:         os.Getenv("DB_NAME"),
		ServerPort:     os.Getenv("SERVER_PORT"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		ResendAPIKey:    os.Getenv("RESEND_API_KEY"),
		ResendFromEmail: os.Getenv("RESEND_FROM_EMAIL"),
		WorkerPoolSize:  workerPoolSize,
	}
}

func RequireEnv(key string) {

	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("%s", "Missing environment variable: "+key))
	}
}
