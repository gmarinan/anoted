# anoted on Linux

Record meetings and transcribe them on the same machine. Nothing is uploaded.

## What you need

- Linux with a desktop session (X11 or Wayland)
- [Go 1.24+](https://go.dev/dl/) to build
- PipeWire (preferred) or PulseAudio
- Optional, but useful:
  - `pw-cat` and/or `ffmpeg` — capture backends
  - `pactl` — device listing
  - `xdotool` or `wmctrl` — window-title meeting detection
  - `snixembed` — tray icon on GNOME and i3

Arch example:

```bash
sudo pacman -S go pipewire-audio pipewire-pulse extra/pipewire-audio
# optional:
sudo pacman -S ffmpeg xdotool wmctrl snixembed
```

Debian / Ubuntu names differ (`golang-go`, `pipewire-pulse`, `ffmpeg`, `xdotool`).

## Build and run

```bash
git clone https://github.com/gmarinan/anoted.git
cd anoted
make build
sudo install -m 755 bin/anoted /usr/local/bin/anoted

anoted setup
anoted doctor
anoted watch
```

`anoted setup` can install a local Whisper venv under `~/.local/share/anoted/` (no sudo). GPU support is a separate Doctor action (`g`) because it pulls a large PyTorch wheel.

## First recording

1. Open the TUI with `anoted watch`.
2. Press `r`. The header shows a red **RECORDING** badge — leave that visible.
3. Press `r` again to stop.
4. Select the session, press `t` to transcribe, `o` to open the folder.

Files land in `~/Music/anoted/` by default:

```
~/Music/anoted/YYYY-MM-DD_HH-mm-ss_<provider>/
├── recording.wav
├── transcript.txt
├── transcript.srt
├── transcript.md     # if md is in output_formats
└── metadata.json
```

Config: `~/.config/anoted/config.yaml`.

## Meeting detection

`detection.mode` can be `mic`, `window`, `both`, or `none`.

- **mic** — treats an in-use capture stream as "something is happening". Good on Wayland, where window titles are often hidden.
- **window** — matches Meet / Teams titles via `xdotool` or `wmctrl`. Needs those tools, and a compositor that still exposes titles.
- **both** — either signal can start a meeting.

anoted will not scrape Wayland in ways the compositor forbids. If titles are missing, use mic mode or press `r`.

## Hands-free (optional)

Auto-record is **off** by default on purpose. To start at login and record detected meetings:

```bash
anoted autostart enable --record
```

That writes `~/.config/autostart/anoted.desktop` and sets `auto_record: true`. You can still require a `y` confirmation (`auto_record_requires_confirmation: true`). You are responsible for consent.

### Tray icon

With `privacy.tray_indicator: true` (the default) anoted shows **Watching** or **Recording**. Right-click opens the recordings folder or quits.

On GNOME, install `snixembed` if the icon never appears. On **i3/i3bar** it is required:

```bash
sudo pacman -S snixembed
snixembed --fork
anoted watch
```

i3 config:

```
exec --no-startup-id snixembed --fork
exec --no-startup-id anoted watch
```

### Pin the window (Hyprland / Sway)

anoted runs inside your terminal. Set a class, re-enable autostart, then write a window rule:

```yaml
desktop:
  wm_class: anoted
  autostart_terminal: ["alacritty", "--class", "anoted", "-e"]
```

```
windowrulev2 = workspace 3 silent, class:^(anoted)$
```

Other terminals: `kitty --class=anoted`, `foot --app-id=anoted`.

## Transcription

Press `t` in the session list, or set `transcription.auto_after_recording: true`.

`faster-whisper` is usually the fastest local backend once the CTranslate2 model is on disk. It is opt-in so the first launch does not download gigabytes unannounced:

```bash
anoted transcribe ~/Music/anoted/<session> --backend faster-whisper
```

Doctor can install Whisper (`i`) and CUDA wheels (`g`) without leaving the TUI.

## Troubleshooting

| Symptom | What to try |
|---------|-------------|
| Doctor says no recorder | Install `pw-cat` or `ffmpeg`; check PipeWire is the active session |
| Equalizer is flat while you talk | Wrong monitor device — Config → Audio → `system_monitor` |
| Meet is not detected on Wayland | Switch detection mode to `mic`, or record with `r` |
| No tray icon | `snixembed --fork`, then restart anoted |
| Whisper missing | Doctor tab, press `i` |

`anoted doctor` is the support report. Paste it in an issue if you want help.
