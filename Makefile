.PHONY: build test run fmt

ifneq (,$(wildcard .env))
include .env
export
endif

GO_ENV := GOMODCACHE="$(CURDIR)/.cache/go-mod" GOCACHE="$(CURDIR)/.cache/go-build"

build:
	$(GO_ENV) go build ./cmd/...

test:
	$(GO_ENV) go test ./...

run:
	$(GO_ENV) go run ./cmd/noxd

fmt:
	gofmt -w ./cmd ./internal
