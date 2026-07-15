SHELL := /bin/bash
BIN := $(CURDIR)/bin

# Pinned tool versions so a fresh clone regenerates exactly what is committed.
TEMPL_VERSION    := v0.3.1020
SQLC_VERSION     := v1.31.1
GOOSE_VERSION    := v3.27.2
AIR_VERSION      := v1.65.3
TAILWIND_VERSION := v4.3.2
HTMX_VERSION     := 2.0.10

UNAME_S := $(shell uname -s | tr '[:upper:]' '[:lower:]')
UNAME_M := $(shell uname -m)
ifeq ($(UNAME_M),x86_64)
	TAILWIND_ARCH := x64
else
	TAILWIND_ARCH := $(UNAME_M)
endif
ifeq ($(UNAME_S),darwin)
	TAILWIND_OS := macos
else
	TAILWIND_OS := linux
endif

.PHONY: tools generate css build run dev test migration migrate-status clean

## tools: install code generators into ./bin and vendor htmx
tools:
	mkdir -p $(BIN) static/js
	GOBIN=$(BIN) go install github.com/a-h/templ/cmd/templ@$(TEMPL_VERSION)
	GOBIN=$(BIN) go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)
	GOBIN=$(BIN) go install github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)
	GOBIN=$(BIN) go install github.com/air-verse/air@$(AIR_VERSION)
	curl -fsSL -o $(BIN)/tailwindcss \
		https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/tailwindcss-$(TAILWIND_OS)-$(TAILWIND_ARCH)
	chmod +x $(BIN)/tailwindcss
	curl -fsSL -o static/js/htmx.min.js https://unpkg.com/htmx.org@$(HTMX_VERSION)/dist/htmx.min.js

## generate: regenerate templ views and sqlc queries
generate:
	$(BIN)/templ generate
	$(BIN)/sqlc generate

## css: compile tailwind to the embedded stylesheet
css:
	$(BIN)/tailwindcss -i static/css/input.css -o static/css/app.css --minify

build: generate css
	go build -o $(BIN)/server ./cmd/server

run: generate css
	go run ./cmd/server

test:
	go test ./...

## dev: live reload (templ + go) via air
dev:
	$(BIN)/air

## migration name=add_something: create a new goose migration
migration:
	$(BIN)/goose -dir internal/db/migrations create $(name) sql

migrate-status:
	$(BIN)/goose -dir internal/db/migrations sqlite3 app.db status

clean:
	rm -rf tmp *.db *.db-shm *.db-wal
