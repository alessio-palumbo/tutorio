.PHONY: dev build test fmt tidy

dev:
	wails dev

build:
	wails build

test:
	go test ./...

fmt:
	gofmt -w cmd internal

tidy:
	go mod tidy
