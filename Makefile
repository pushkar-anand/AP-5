GO ?= go

.PHONY: build vet test fmt

build:
	$(GO) build ./...

vet:
	$(GO) vet ./...

test:
	$(GO) test -race -count=1 ./...

fmt:
	gofmt -w .
