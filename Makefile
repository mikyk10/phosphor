BINARY := phosphor
ENTRYPOINT := main.go

.DEFAULT_GOAL := build

build: $(BINARY)
$(BINARY):
	go build -ldflags "-X main.version=$(shell echo "$${GITHUB_REF_NAME:-dev}") \
	-X main.commit=$(shell echo "$${GITHUB_SHA:0:7}")" \
	-o $(BINARY) $(ENTRYPOINT)

.PHONY: clean test lint
clean:
	rm -rf $(BINARY)

test:
	go test -v ./...

lint:
	golangci-lint run
