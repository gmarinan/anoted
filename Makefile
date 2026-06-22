.PHONY: build test lint run clean build-windows build-windows-helper

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

run: build
	./bin/$(BINARY) watch

clean:
	rm -rf bin/
