.PHONY: build test run fmt

build:
	go build ./cmd/...

test:
	go test ./...

run:
	go run ./cmd/noxd

fmt:
	gofmt -w ./cmd ./internal
