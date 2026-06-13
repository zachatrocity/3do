.PHONY: dev build test fmt docker-build

dev:
	go run ./cmd/3do

build:
	mkdir -p bin
	go build -o bin/3do ./cmd/3do

test:
	go test ./...

fmt:
	gofmt -w ./cmd ./internal

docker-build:
	docker build -t 3do:local .
