-include .env
export

export PROJECT_ROOT=$(shell pwd)

env-up:
	@docker compose up -d --wait chat-postgres port-forwarder

env-down:
	@docker compose stop chat-postgres port-forwarder

env-cleanup:
	@read -p "Are you sure you want to remove the database volume? (y/N): " ans; \
	if [ "$$ans" = "y" ] || [ "$$ans" = "Y" ]; then \
		docker compose down chat-postgres port-forwarder && \
		sudo rm -rf out/pgdata && \
		echo "Cleanup completed."; \
	else \
		echo "Aborting cleanup."; \
	fi

migrate-create:
	@if [ -z "$(seq)" ]; then \
		echo "Error: Please provide a sequence number using 'make migrate-create seq=<number>'"; \
		exit 1; \
	fi; \
	PROJECT_ROOT=$(PROJECT_ROOT) docker compose run --rm grpcchat-postgres-migrate \
		create \
		-ext sql \
		-dir migrations \
		-seq "$(seq)"

migrate-up:
	@make migrate-action action=up

migrate-down:
	@make migrate-action action=down

migrate-force:
	@make migrate-action action="force $(v)"

migrate-action:
	@if [ -z "$(action)" ]; then \
		echo "Error: Please provide an action using 'make migrate-action action=<up|down>'"; \
		exit 1; \
	fi; \
	PROJECT_ROOT=$(PROJECT_ROOT) docker compose run --rm  grpcchat-postgres-migrate \
		-path /migrations \
		-database postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@chat-postgres:5432/$(POSTGRES_DB)?sslmode=disable \
		$(action)

logs-cleanup:
	@read -p "Are you sure you want to remove the logs? (y/N): " ans; \
	if [ "$$ans" = "y" ] || [ "$$ans" = "Y" ]; then \
		sudo rm -rf ${PROJECT_ROOT}/out/logs && \
		echo "Cleanup completed."; \
	else \
		echo "Aborting cleanup."; \
	fi

env-port-forward:
	@docker compose up -d port-forwarder
    
env-port-close:
	@docker compose down port-forwarder

grpc-chat:
	@go run ./cmd/chat

grpc-codegen:
	@mkdir -p ./gen/grpc/chat_v1
	@protoc --proto_path=api/grpc \
		--go_out=./gen/grpc/chat_v1 --go_opt=paths=source_relative \
		--go-grpc_out=./gen/grpc/chat_v1 --go-grpc_opt=paths=source_relative \
		chat_v1.proto

.PHONY: build server client test test-integration check setup
GO_PACKAGES := ./cmd/... ./core/... ./internal/... ./pkg/... ./gen/...

setup:
	@test -f .env || cp .env.example .env

build:
	@mkdir -p bin
	@go build -o bin/chat-server ./cmd/chat
	@go build -o bin/chat-cli ./cmd/chat-cli

server: grpc-chat

client:
	@go run ./cmd/chat-cli

test:
	@go test -race $(GO_PACKAGES)

test-integration:
	@test -n "$$TEST_DATABASE_URL" || (echo 'Set TEST_DATABASE_URL to a dedicated database ending in _test'; exit 1)
	@go test -race -count=1 -v ./internal/integration

check:
	@go vet $(GO_PACKAGES)
	@test -z "$$(gofmt -l cmd core internal pkg gen)" || (echo 'Run gofmt'; exit 1)
