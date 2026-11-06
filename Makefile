# go.mod pins the toolchain; put its go first on PATH so tools that shell out
# to go (coverage of packages without tests) use the same version.
export PATH := $(shell go env GOROOT)/bin:$(PATH)

GOLANGCI_LINT := ./bin/golangci-lint
GOLANGCI_LINT_VERSION := v2.15.0
SQLC := ./bin/sqlc
SQLC_VERSION := v1.30.0
AIR := ./bin/air
AIR_VERSION := v1.67.4

# Packages whose statements count toward the coverage gates.
FRAMEWORK_PKGS := ./framework/...
APP_PKGS := ./app/...
# Packages whose behaviour depends on the edition, tested again in the
# cloud build.
CLOUD_PKGS := ./app/... ./config/... ./routes/... ./bootstrap/... ./tests/...

.PHONY: test-sweep e2e pseudo-locale tools lint-tools fmt lint lint-go lint-web test test-go test-app-role test-cloud test-timing test-web test-dev cover cover-app services gen sqlc views build dev dev-server dev-web dev-db web-build web-test

APP ?= dimmi

services:
	docker compose up -d --wait

tools: lint-tools
	GOBIN=$(CURDIR)/bin go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)
	GOBIN=$(CURDIR)/bin go install github.com/air-verse/air@$(AIR_VERSION)
	pnpm install
	pnpm exec playwright install chromium

lint-tools:
	GOBIN=$(CURDIR)/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

# Compiles database/queries into app/models (sqlc.yaml).
sqlc:
	$(SQLC) generate

# Compiles the templ views in resources/views, with the templ version go.mod pins.
TEMPL := go run github.com/a-h/templ/cmd/templ@$(shell go list -m -f '{{.Version}}' github.com/a-h/templ)
views:
	$(TEMPL) fmt resources/views
	$(TEMPL) generate -path resources/views

fmt:
	$(GOLANGCI_LINT) fmt
	pnpm format

# Dev-only commands (make:* generators, route:lint) sit behind the dev build
# tag, so lint and tests run once without it and once with it.
lint: lint-go lint-web

lint-go:
	$(GOLANGCI_LINT) config verify
	$(GOLANGCI_LINT) run ./...
	$(GOLANGCI_LINT) run --build-tags dev ./...
	$(GOLANGCI_LINT) run --build-tags cloud ./...
	go vet -tags emailmatrix ./tests/emailmatrix/...

lint-web:
	pnpm lint
	pnpm typecheck

# Tests read the same variables the application does; ENV_FILE picks the
# environment (CI passes its own file).
ENV_FILE ?= .env.testing
WITH_ENV := set -a && . ./$(ENV_FILE) && set +a &&

test: test-go test-app-role test-cloud test-timing test-web test-dev

# SHARD=2/4 runs only the second of four even parts of a pass's packages,
# so CI can spread a pass across machines; without it the whole pass runs.
SHARD ?=
SHARD_PKGS = $(if $(SHARD),"$$(node scripts/test-shard.ts $(1) $(SHARD) $(2))",$(2))

test-go:
	pkgs=$(call SHARD_PKGS,,./...) && $(WITH_ENV) go test -race -shuffle=on $$pkgs

# The feature tests again as the cloud's application role, where row level
# security applies to every query.
test-app-role:
	pkgs=$(call SHARD_PKGS,,./tests/feature/...) && \
		$(WITH_ENV) DIMMI_TEST_APPLICATION_ROLE=1 go test -race -shuffle=on $$pkgs

test-cloud:
	pkgs=$(call SHARD_PKGS,-tags cloud,$(CLOUD_PKGS)) && $(WITH_ENV) go test -race -shuffle=on -tags cloud $$pkgs

# Timing comparisons skip themselves under the race detector, which distorts
# them, so they get a pass of their own without it, one test at a time so no
# comparison shares the machine with another.
test-timing:
	$(WITH_ENV) go test -count=1 -p=1 -parallel=1 -run 'TestEnumerationTiming|TestPasswordResetUpdate|TestAudienceBudget' ./tests/feature/auth/ ./tests/feature/access/

test-web:
	pnpm test

test-dev:
	$(WITH_ENV) go test -race -tags dev ./framework/foundation/...

# Drops the databases and roles killed test runs left on the test server.
test-sweep:
	$(WITH_ENV) DIMMI_DATABASE__URL="$$DATABASE_URL" DIMMI_REDIS__URL="$$REDIS_URL" go run -tags dev ./cmd/$(APP) test:sweep

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
	$(WITH_ENV) go test -race -count=1 -tags cloud -coverpkg=$(APP_PKGS) -coverprofile=coverage-app-cloud.out ./tests/...
	tail -n +2 coverage-app-cloud.out >> coverage-app.out
	grep -v -e '\.sql\.go:' -e 'app/models/db\.go:' -e 'app/models/models\.go:' coverage-app.out > coverage-app.filtered.out
	go tool cover -func=coverage-app.filtered.out | tail -1

# Local development: the Go server on :8080, rebuilt by air on every Go
# change, and the Vite dev server on :5173 for hot updates of the browser
# app. Open http://localhost:8080/d. Settings come from dimmi.yaml, copied
# from dimmi.example.yaml on first run. `make dev TAGS=cloud` runs the
# cloud edition.
dev: dev-db pseudo-locale
	@test -f dimmi.yaml || cp dimmi.example.yaml dimmi.yaml
	$(MAKE) -j2 dev-server dev-web

dev-server:
	$(AIR) --build.cmd "go build -tags '$(TAGS)' -o ./tmp/dimmi ./cmd/dimmi"

dev-web:
	pnpm dev

dev-db:
	@docker compose exec -T postgres psql -U postgres -tAc "SELECT 1 FROM pg_database WHERE datname = 'dimmi'" | grep -q 1 \
		|| docker compose exec -T postgres psql -U postgres -c 'CREATE DATABASE dimmi'

# Builds both binaries with the browser app embedded: bin/dimmi, the
# self-hosted edition operators run, and bin/dimmi-cloud, the cloud
# edition with the cloud-only code.
build: web-build
	go build -trimpath -o bin/dimmi ./cmd/dimmi
	go build -trimpath -tags cloud -o bin/dimmi-cloud ./cmd/dimmi

# Builds the browser app into public/build, which the next go build embeds.
web-build:
	pnpm build

web-test:
	pnpm typecheck
	pnpm test

# End-to-end tests: builds both binaries, then drives Chromium in light
# and dark against the release cloud binary, started fresh on :18090
# with its own database (tests/e2e/server.ts). Needs `make services`.
# E2E_ARGS passes options to Playwright, such as --shard=1/3.
e2e: pseudo-locale build
	pnpm e2e $(E2E_ARGS)

# Checks the infrastructure code without reaching any account: credentials
# are hidden from the toolchain, so a machine that holds some cannot use them
# by accident. deploy/cdk and deploy/dns have their own lockfiles and are not
# part of the browser app.
INFRA_ENV := env -u AWS_ACCESS_KEY_ID -u AWS_SECRET_ACCESS_KEY -u AWS_SESSION_TOKEN -u AWS_PROFILE -u AWS_DEFAULT_PROFILE \
	AWS_SHARED_CREDENTIALS_FILE=/dev/null AWS_CONFIG_FILE=/dev/null AWS_EC2_METADATA_DISABLED=true \
	CDK_DISABLE_CLI_TELEMETRY=true

.PHONY: infra-check email-matrix

infra-check:
	cd deploy/cdk && pnpm install --frozen-lockfile
	cd deploy/cdk && $(INFRA_ENV) pnpm run typecheck
	cd deploy/cdk && $(INFRA_ENV) pnpm run synth:all
	cd deploy/cdk && $(INFRA_ENV) pnpm run test
	cd deploy/dns && pnpm install --frozen-lockfile
	cd deploy/dns && $(INFRA_ENV) pnpm run typecheck
	cd deploy/dns && $(INFRA_ENV) pnpm run test
	go test -count=1 ./tests/arch/ -run '^TestDeploy'

# Sends real mail through real mailboxes and refuses to start without their
# credentials. Each run needs the owner's approval. ARGS takes -adapter <name>
# and -record, for example make email-matrix ARGS="-adapter postmark -record".
email-matrix:
	EMAIL_MATRIX_ARGS="$(ARGS)" EMAIL_MATRIX_ROOT="$(CURDIR)" \
		go test -tags emailmatrix -count=1 -timeout 3h -v -run '^TestEmailMatrix$$' ./tests/emailmatrix/
# The pseudo locale en-XA, generated from the English messages for
# development and the end-to-end tests; never checked in or served by a
# release, whose app.locales leaves it out.
pseudo-locale:
	node scripts/pseudo-locale.ts
