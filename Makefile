.PHONY: sqlc migrate-status migrate-up migrate-down migrate-create run build vet

sqlc:
	sqlc generate

migrate-status:
	goose status

migrate-up:
	goose up

migrate-down:
	goose down

migrate-create:
	@read -p "Enter migration name: " name; \
	goose create $$name sql

run:
	go run main.go

build:
	go build -v ./...

vet:
	go vet ./...
