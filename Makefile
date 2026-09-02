.PHONY: build test test-race lint lint-windows run clean build-windows build-windows-helper readme-shots shot

BINARY=anoted
# Derived from git so every build identifies its commit; override with
# `make build VERSION=v1.2.3` for a release.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -s -w -X anoted/internal/buildinfo.version=$(VERSION)

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/anoted

build-windows:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY).exe ./cmd/anoted

build-windows-helper:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/windows-recorder.exe ./tools/windows-recorder

test:
	go test ./...

lint:
	go vet ./...
	@test -z "$$(gofmt -l ./cmd ./internal ./tools)" || (echo "run gofmt -w ." && exit 1)
	@command -v golangci-lint >/dev/null 2>&1 \
		&& golangci-lint run \
		|| echo "golangci-lint not installed; skipping (see .golangci.yml)"

# Windows-tagged code is never compiled by `make lint` on Linux, so bugs there
# reach users uncaught. This checks the packages that do not need cgo; a full
# check needs MinGW-w64 and is what `make build-windows` does.
#
# detector, setup, transcribe and recorder cannot be added here: on Windows they
# all reach internal/wasapi through internal/audio, which needs cgo. That is a
# large share of the Windows-specific surface, and the only thing that really
# covers it is the windows-latest job in .github/workflows/ci.yml.
lint-windows:
	CGO_ENABLED=0 GOOS=windows go build \
		./internal/autostart/ ./internal/config/ ./internal/session/ \
		./internal/folderpicker/ ./internal/open/ ./internal/platform/ \
		./internal/logging/ ./internal/tray/ ./tools/windows-recorder/

test-race:
	go test -race ./...

run: build
	./bin/$(BINARY) watch

# Render README screenshots from the real TUI components (requires `freeze`).
# Install once: go install github.com/charmbracelet/freeze@latest
#
# Pipe the frame into freeze. A freeze JSON config with width/height 0 captures
# empty window chrome; flags here are the working path.
readme-shots:
	@command -v freeze >/dev/null 2>&1 || { echo "install freeze: go install github.com/charmbracelet/freeze@latest"; exit 1; }
	mkdir -p docs/assets
	go build -trimpath -ldflags "-s -w -X anoted/internal/buildinfo.version=v0.1.0" -o bin/readme-shots ./tools/readme-shots
	$(MAKE) -s shot SCENE=recording OUT=docs/assets/home-recording.png
	$(MAKE) -s shot SCENE=transcribe OUT=docs/assets/home-transcribe.png
	$(MAKE) -s shot SCENE=doctor OUT=docs/assets/doctor.png
	$(MAKE) -s shot SCENE=config OUT=docs/assets/config.png

shot:
	COLORTERM=truecolor TERM=xterm-256color ./bin/readme-shots $(SCENE) | freeze \
		--window \
		--background "#16141F" \
		-p 24 \
		--border.radius 12 \
		--border.width 1 \
		--border.color "#544C8C" \
		--font.family "JetBrainsMono Nerd Font" \
		--font.size 14 \
		-o $(OUT)

clean:
	rm -rf bin/
