# go.mod pins the toolchain; put its go first on PATH so tools that shell out
# to go (coverage of packages without tests) use the same version.
export PATH := $(shell go env GOROOT)/bin:$(PATH)

GOLANGCI_LINT := ./bin/golangci-lint
GOLANGCI_LINT_VERSION := v2.15.0
SQLC := ./bin/sqlc
SQLC_VERSION := v1.30.0

# Packages whose statements count toward the coverage gates.
FRAMEWORK_PKGS := ./framework/...
APP_PKGS := ./app/...

.PHONY: tools fmt lint test cover cover-app services gen sqlc

APP ?= dimmi

services:
	docker compose up -d --wait

tools:
	GOBIN=$(CURDIR)/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	GOBIN=$(CURDIR)/bin go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)

# Compiles database/queries into app/models (sqlc.yaml).
sqlc:
	$(SQLC) generate

fmt:
	$(GOLANGCI_LINT) fmt

# Dev-only commands (make:* generators, route:lint) sit behind the dev build
# tag, so lint and tests run once without it and once with it.
lint:
	$(GOLANGCI_LINT) config verify
	$(GOLANGCI_LINT) run ./...
	$(GOLANGCI_LINT) run --build-tags dev ./...

# Tests read the same variables the application does; ENV_FILE picks the
# environment (CI passes its own file).
ENV_FILE ?= .env.testing
WITH_ENV := set -a && . ./$(ENV_FILE) && set +a &&

test:
	$(WITH_ENV) go test -race -shuffle=on ./...
	$(WITH_ENV) go test -race -tags dev ./framework/foundation/...

# Runs a dev-only command, e.g. make gen ARGS="make:action CloseTicket".
gen:
	go run -tags dev ./cmd/$(APP) $(ARGS)

cover:
	$(WITH_ENV) go test -race -count=1 -coverpkg=$(FRAMEWORK_PKGS) -coverprofile=coverage.out ./framework/...
	go tool cover -func=coverage.out | tail -1

# Business logic is covered by the integration tests under tests/, so the
# app gate measures app packages while those run. Generated queries are
# left out of the profile.
cover-app:
	$(WITH_ENV) go test -race -count=1 -coverpkg=$(APP_PKGS) -coverprofile=coverage-app.out ./tests/...
	grep -v -e '\.sql\.go:' -e 'app/models/db\.go:' -e 'app/models/models\.go:' coverage-app.out > coverage-app.filtered.out
	go tool cover -func=coverage-app.filtered.out | tail -1
