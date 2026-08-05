.PHONY: build test lint lint-windows run clean build-windows build-windows-helper

BINARY=anoted
VERSION?=dev

build:
	go build -ldflags "-s -w" -o bin/$(BINARY) ./cmd/anoted

build-windows:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o bin/$(BINARY).exe ./cmd/anoted

build-windows-helper:
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o bin/windows-recorder.exe ./tools/windows-recorder

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
