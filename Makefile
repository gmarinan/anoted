.PHONY: build test lint lint-windows run clean build-windows build-windows-helper

BINARY=anoted
# Derived from git so every build identifies its commit; override with
# `make build VERSION=v1.2.3` for a release.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -s -w -X anoted/internal/buildinfo.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/anoted

build-windows:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY).exe ./cmd/anoted

build-windows-helper:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/windows-recorder.exe ./tools/windows-recorder

test:
	go test ./...

lint:
	go vet ./...
	@test -z "$$(gofmt -l .)" || (echo "run gofmt -w ." && exit 1)

# Windows-tagged code is never compiled by `make lint` on Linux, so bugs there
# reach users uncaught. This checks the packages that do not need cgo; a full
# check needs MinGW-w64 and is what `make build-windows` does.
lint-windows:
	CGO_ENABLED=0 GOOS=windows go build \
		./internal/autostart/ ./internal/config/ ./internal/session/ \
		./internal/folderpicker/ ./internal/open/ ./internal/platform/ \
		./internal/logging/ ./internal/tray/ ./tools/windows-recorder/

run: build
	./bin/$(BINARY) watch

clean:
	rm -rf bin/
