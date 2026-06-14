GO ?= go

.PHONY: build vet test fmt

build:
	$(GO) build -o ./bin/ap5 ./cmd/ap5

vet:
	$(GO) vet ./...

test:
	$(GO) test -race -count=1 ./...

fmt:
	gofmt -w .

run: build
	./bin/ap5 serve --config config.yaml