.PHONY: test test-integration lint fmt build

test:
	go test -race -count=1 ./...

test-integration:
	go test -race -count=1 -tags=integration ./...

lint:
	golangci-lint run

fmt:
	gofmt -w .

build:
	go build ./...
