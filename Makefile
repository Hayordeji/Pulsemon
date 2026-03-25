include .env
export

DB_URL=postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSL_MODE)

.PHONY: migrate-up migrate-down migrate-create migrate-version migrate-force run build tidy

migrate-up:
	migrate -path migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path migrations -database "$(DB_URL)" down 1

migrate-create:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name

migrate-version:
	migrate -path migrations -database "$(DB_URL)" version

migrate-force:
	@read -p "Force version: " version; \
	migrate -path migrations -database "$(DB_URL)" force $$version

run:
	go run cmd/api/main.go

build:
	go build -o bin/pulsemon cmd/api/main.go

tidy:
	go mod tidy
