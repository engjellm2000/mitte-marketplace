.PHONY: fmt test test-race vet build run compose-up

fmt:
	go fmt ./...

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

build:
	go build ./cmd/server

run:
	go run ./cmd/server

compose-up:
	docker compose up --build
