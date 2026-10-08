SHELL := /bin/bash
BIN := $(CURDIR)/bin

# Pinned tool versions so a fresh clone regenerates exactly what is committed.
TEMPL_VERSION    := v0.3.1020
SQLC_VERSION     := v1.31.1
GOOSE_VERSION    := v3.27.2
AIR_VERSION      := v1.65.3
TAILWIND_VERSION := v4.3.2
HTMX_VERSION     := 2.0.10

# go install already verifies module checksums against GOSUMDB; the two plain
# curl downloads below do not, so their expected digests are pinned here and
# checked before anything is installed from them.
HTMX_SHA256                    := 71ea67185bfa8c98c39d31717c6fce5d852370fcdfd129db4543774d3145c0de
TAILWIND_SHA256_linux-x64      := 5036c4fb4328e0bcdbb6065c70d8ac9452e0d4c947113a788a8f94fd390425c1
TAILWIND_SHA256_linux-arm64    := 394ddccc2402cfa3abd97dfba56f3587781a3d6e6ce66e65ceada14beb7664b8
TAILWIND_SHA256_macos-x64      := cef8f110471e889c3c4409055cf8aff33076f58a081867b0dfc6534b290bfbb0
TAILWIND_SHA256_macos-arm64    := b800b0659dc64b9f03ede5660244d9415d777d5739ae2889280877ca37be742a

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
TAILWIND_SHA256 := $(TAILWIND_SHA256_$(TAILWIND_OS)-$(TAILWIND_ARCH))

.PHONY: tools generate css build run dev test adminhash migration migrate-status clean

## tools: install code generators into ./bin and vendor htmx
tools:
	mkdir -p $(BIN) static/js
	GOBIN=$(BIN) go install github.com/a-h/templ/cmd/templ@$(TEMPL_VERSION)
	GOBIN=$(BIN) go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)
	GOBIN=$(BIN) go install github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)
	GOBIN=$(BIN) go install github.com/air-verse/air@$(AIR_VERSION)
	test -n "$(TAILWIND_SHA256)" # unrecognised OS/arch for tailwindcss checksum pin
	curl -fsSL -o $(BIN)/tailwindcss \
		https://github.com/tailwindlabs/tailwindcss/releases/download/$(TAILWIND_VERSION)/tailwindcss-$(TAILWIND_OS)-$(TAILWIND_ARCH)
	echo "$(TAILWIND_SHA256)  $(BIN)/tailwindcss" | shasum -a 256 -c -
	chmod +x $(BIN)/tailwindcss
	curl -fsSL -o static/js/htmx.min.js https://unpkg.com/htmx.org@$(HTMX_VERSION)/dist/htmx.min.js
	echo "$(HTMX_SHA256)  static/js/htmx.min.js" | shasum -a 256 -c -

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

## adminhash: interactively generate an ADMIN_PASSWORD_HASH value
adminhash:
	go run ./cmd/adminhash

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
