# meetctl

Cross-platform TUI for meeting detection and local audio recording.

**Privacy notice:** You are responsible for complying with applicable laws, company policies, and obtaining consent from meeting participants before recording. meetctl never uploads audio to the cloud and does not record secretly — recording state is always visible in the TUI.

## Features (MVP)

- Bubble Tea v2 terminal UI
- Meeting detection (mock, Linux, Windows)
- Local audio recording with pluggable backends
- SQLite session store
- YAML configuration
- `meetctl doctor` for dependency checks

## Requirements

- Go 1.22+
- Linux: optional `pw-cat`, `ffmpeg`, `pactl`, `xdotool`, `wmctrl`
- Windows: WASAPI helper (skeleton; full capture coming later)
- WSL2: TUI runs in Linux; real Windows audio via `windows-recorder.exe`

## Install

### Linux (native)

```bash
git clone <repo-url> meetctl
cd meetctl
make build
sudo mv bin/meetctl /usr/local/bin/   # optional
meetctl doctor
meetctl watch
```

### Windows (native)

```powershell
go build -o meetctl.exe ./cmd/meetctl
.\meetctl.exe doctor
.\meetctl.exe watch
```

Or cross-compile from Linux:

```bash
make build-windows
```

### WSL2

1. Build and run the TUI inside WSL2:

```bash
make build
./bin/meetctl watch
```

2. Build the Windows helper on Windows (or cross-compile):

```bash
make build-windows-helper
# Copy bin/windows-recorder.exe to e.g. C:\Program Files\meetctl\
```

meetctl detects WSL2 and will use the helper for real Windows audio when the protocol is implemented.

## Commands

| Command | Description |
|---------|-------------|
| `meetctl setup` | Guided setup (pick xdotool/wmctrl, install, save config) |
| `meetctl` / `meetctl watch` | Start the TUI |
| `meetctl status` | Print detection and recorder status |
| `meetctl sessions` | List recorded sessions |
| `meetctl config` | Show config file |
| `meetctl doctor` | Check OS, tools, output path |

## TUI keys

| Key | Action |
|-----|--------|
| `q` | Quit |
| `r` | Start/stop manual recording |
| `a` | Toggle auto-record |
| `o` | Audio devices (system monitor + microphone) |
| `d` | Doctor / debug screen |
| `s` | Sessions list |

## Configuration

Default config is created at:

- Linux: `~/.config/meetctl/config.yaml`
- Windows: `%AppData%\meetctl\config.yaml`

Recordings are saved under `~/Music/meetctl/` by default:

```
~/Music/meetctl/YYYY-MM-DD_HH-mm-ss_<provider>/
├── recording.wav
└── metadata.json
```

## Development

```bash
make test
make lint
make run

# Mock detector + dummy recorder (no real audio):
./bin/meetctl watch --mock-detector --dummy-recorder
```

## Architecture

```
cmd/meetctl          CLI entrypoint
internal/tui         Bubble Tea UI (platform-agnostic)
internal/detector    Meeting detection (build tags)
internal/recorder    Audio backends (build tags)
internal/session     SQLite + metadata
internal/config      YAML config
internal/platform    OS / WSL2 detection
internal/doctor      Dependency checks
tools/windows-recorder   Windows WASAPI helper (skeleton)
```

## License

TBD
