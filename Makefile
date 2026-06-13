.PHONY: dev build test fmt docker-build

dev:
	SESSION_SECRET=$${SESSION_SECRET:-local-development-session-secret-do-not-use-in-production} go run ./cmd/3do

build:
	mkdir -p bin
	go build -o bin/3do ./cmd/3do

test:
	go test ./...

fmt:
	gofmt -w ./cmd ./internal

docker-build:
	docker build -t 3do:local .
