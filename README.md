# anoted

Cross-platform TUI for meeting detection and local audio recording.

**Privacy notice:** You are responsible for complying with applicable laws, company policies, and obtaining consent from meeting participants before recording. anoted never uploads audio to the cloud and does not record secretly — recording state is always visible in the TUI.

## Features (MVP)

- Bubble Tea v2 terminal UI
- Meeting detection (mock, Linux, Windows)
- Local audio recording with pluggable backends
- SQLite session store
- YAML configuration
- `anoted doctor` for dependency checks

## Requirements

- Go 1.22+
- Linux: optional `pw-cat`, `ffmpeg`, `pactl`, `xdotool`, `wmctrl`
- Windows 10+: native WASAPI capture (single `anoted.exe`; build on Windows or cross-compile with MinGW + `CGO_ENABLED=1`)
- WSL2: TUI runs in Linux; real Windows audio via `windows-recorder.exe` (future)

## Install

### Linux (native)

```bash
git clone <repo-url> anoted
cd anoted
make build
sudo mv bin/anoted /usr/local/bin/   # optional
anoted doctor
anoted watch
```

### Windows (native)

Single executable with in-process WASAPI (system loopback + microphone). Config: `%AppData%\anoted\config.yaml`. Recordings default to `~\Music\anoted\`.

```powershell
go build -o anoted.exe ./cmd/anoted
.\anoted.exe setup
.\anoted.exe doctor
.\anoted.exe watch
```

Or cross-compile from Linux (requires MinGW-w64 for CGO):

```bash
CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc make build-windows
```

**Limitations:** loopback capture follows Windows privacy policies; meeting detection uses window titles (`MainWindowTitle`) and process names.

### WSL2

1. Build and run the TUI inside WSL2:

```bash
make build
./bin/anoted watch
```

2. Build the Windows helper on Windows (or cross-compile):

```bash
make build-windows-helper
# Copy bin/windows-recorder.exe to e.g. C:\Program Files\anoted\
```

anoted detects WSL2 and will use the helper for real Windows audio when the protocol is implemented.

## Commands

| Command | Description |
|---------|-------------|
| `anoted setup` | Guided setup (pick xdotool/wmctrl, install, save config) |
| `anoted` / `anoted watch` | Start the TUI |
| `anoted status` | Print detection and recorder status |
| `anoted sessions` | List recorded sessions |
| `anoted config` | Show config file |
| `anoted doctor` | Check OS, tools, output path |
| `anoted autostart enable --record` | Start at login and enable auto-record |

## Hands-free recording (Linux)

To start anoted when you log in and record meetings automatically:

```bash
anoted autostart enable --record
```

This creates `~/.config/autostart/anoted.desktop` and sets `auto_record: true` in your config. The TUI opens in a terminal at login; a visible recording indicator is shown while capturing.

You can also toggle **launch_at_login** and **auto_record** in Config → General. You are responsible for participant consent and local recording laws.

### System tray icon

When `privacy.tray_indicator` is true (default), anoted shows an icon in the system tray:

- **Watching** — app is running in the background
- **Recording** — red dot while a meeting is being captured

Right-click the icon for **Open recordings folder** or **Quit**. The tray complements the TUI; recording state is also shown in the terminal title and Home screen.

On GNOME, if the icon does not appear, install `snixembed`. On **i3/i3bar**, `snixembed` is **required** (Flameshot/Sunshine use the older XEmbed tray directly):

```bash
sudo pacman -S snixembed
snixembed --fork    # add to i3 config before anoted, or let anoted start it
anoted watch
```

Add to your i3 config (`~/.config/i3/config`):

```
exec --no-startup-id snixembed --fork
exec --no-startup-id anoted watch
```

### Workspace rules (Hyprland / Sway)

anoted runs inside your terminal. To pin it to a workspace, set `desktop.autostart_terminal` and `desktop.wm_class` in config, then re-run `anoted autostart enable`:

```yaml
desktop:
  wm_class: anoted
  autostart_terminal: ["alacritty", "--class", "anoted", "-e"]
```

Hyprland example:

```
windowrulev2 = workspace 3 silent, class:^(anoted)$
```

Other terminals: `kitty --class=anoted`, `foot --app-id=anoted`.

## TUI keys

| Key | Action |
|-----|--------|
| `1`–`4` | Switch screens: Home, Doctor, Sessions, Config |
| `Tab` | Switch audio subsection (Output / Microphone) on Home |
| `q` | Quit |
| `r` | Start/stop manual recording |
| `a` | Toggle auto-record |
| `y` / `n` | Confirm or dismiss auto-record prompt |
| `Ctrl+S` | Save config (Config screen) |
| `↑`/`↓` | Navigate audio devices (Home) or sessions list, or YAML editor lines |
| `Enter` | Select audio device (Home) / new line in config editor |
| `o` / `p` | Open session folder / play recording (Sessions) |
| `R` | Refresh current screen |

## Configuration

Default config is created at:

- Linux: `~/.config/anoted/config.yaml`
- Windows: `%AppData%\anoted\config.yaml`

Recordings are saved under `~/Music/anoted/` by default:

```
~/Music/anoted/YYYY-MM-DD_HH-mm-ss_<provider>/
├── recording.wav
├── transcript.txt    # after Whisper (if enabled in output_formats)
├── transcript.srt
├── transcript.md     # optional Obsidian note with meeting frontmatter
└── metadata.json
```

### Transcription (Whisper)

```yaml
transcription:
  auto_after_recording: false   # transcribe when recording stops
  backend: auto                 # auto, openai-whisper, whisper-cpp
  model: turbo                  # tiny, base, small, medium, large, turbo (recommended)
  device: auto                  # cpu, cuda, auto
  gpu_layers: 0                 # whisper.cpp only (99 = full GPU)
  model_path: ""                # path to ggml model for whisper.cpp
  language: ""                  # empty = auto-detect
  output_formats: [txt, srt]    # txt, srt, vtt, json, md
  output_dir: ""                # empty = same folder as recording; or e.g. ~/vault/meetings
  markdown:
    filename: transcript.md
    tags: [meeting]
    cssclasses: [meeting]
    weekday_class: true         # add monday, tuesday, … to cssclasses
```

`transcript.md` includes YAML frontmatter (start/end time, duration, provider, platform, tags) and the full transcript text below. Enable `md` in `output_formats` to generate it. Set `transcription.output_dir` to write transcripts to a shared folder (e.g. your Obsidian vault); each meeting gets a unique filename derived from the recording session folder name.

Install: run `anoted setup` (installs a local venv at `~/.local/share/anoted/whisper-venv`, no sudo). Optional: `sudo pacman -S python-openai-whisper` or `yay -S whisper.cpp` for GPU.

### Desktop (open folder / play file)

In **Sessions**, press **`f`** to choose how folders open (auto-detect, Dolphin, Thunar, xdg-open, etc.). Skips disk-usage apps like Baobab when using auto.

Play recording (`p`) uses `xdg-open`.

## Development

```bash
make test
make lint
make run

# Mock detector + dummy recorder (no real audio):
./bin/anoted watch --mock-detector --dummy-recorder
```

## Architecture

```
cmd/anoted          CLI entrypoint
internal/tui         Bubble Tea UI (platform-agnostic)
internal/detector    Meeting detection (build tags)
internal/recorder    Audio backends (build tags)
internal/session     SQLite + metadata
internal/config      YAML config
internal/platform    OS / WSL2 detection
internal/doctor      Dependency checks
tools/windows-recorder   Windows WASAPI helper (WSL2; optional)
```

## License

TBD
