SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
export GOTOOLCHAIN := auto

BINARY_NAME := trip-service
OUT_DIR := out
OPENAPI_SPEC := contracts/openapi/trip-service.openapi.yaml
OPENAPI_OUTPUT := internal/generated/api.gen.go

ENV := set -a; source .env.example; if [ -f .env ]; then source .env; fi; set +a;
PACKAGES := ./internal/domain/... ./internal/service/...

.PHONY: build run test test-integration lint fmt clean deps migrate migrate-down generate generate-api generate-mocks check mutation

build:
	mkdir -p $(OUT_DIR)
	go build -o $(OUT_DIR)/$(BINARY_NAME) ./cmd/trip-service

run:
	$(ENV) go run ./cmd/trip-service

test:
	mkdir -p $(OUT_DIR)
	go test -race -coverprofile=$(OUT_DIR)/coverage.out -coverpkg=$(shell go list $(PACKAGES) | paste -sd,) ./...
	go tool cover -func=$(OUT_DIR)/coverage.out

test-integration:
	$(ENV) go test -race -tags=integration -count=1 ./internal/repositories/...

lint:
	mkdir -p $(OUT_DIR)
	golangci-lint run
	golangci-lint fmt --diff --diff-colored=false | tee $(OUT_DIR)/format.diff
	test ! -s $(OUT_DIR)/format.diff

fmt:
	golangci-lint fmt

migrate:
	$(ENV) go run ./cmd/migrator up

migrate-down:
	$(ENV) go run ./cmd/migrator down

generate: generate-api generate-mocks

generate-mocks:
	go generate ./internal/...

generate-api:
	mkdir -p $(dir $(OPENAPI_OUTPUT))
	go tool oapi-codegen \
		-generate types,gin-server \
		-include-operation-ids createTrip,getTrip,finishTrip,createTripPosition,listTripPositions,health,ready \
		-package api \
		-o $(OPENAPI_OUTPUT) \
		$(OPENAPI_SPEC)

deps:
	go mod tidy

define run-mutation
go tool gremlins unleash ./$(1) --workers=2 --timeout-coefficient=50 \
	--output=reports/mutation/after/$(notdir $(1)).json
endef

mutation:
	mkdir -p reports/mutation/after
	$(call run-mutation,internal/domain/models)
	$(call run-mutation,internal/service/trip/create)
	$(call run-mutation,internal/service/trip/get)
	$(call run-mutation,internal/service/trip/finish)

check: lint test test-integration

clean:
	rm -rf $(OUT_DIR)
