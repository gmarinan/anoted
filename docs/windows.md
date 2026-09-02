# anoted on Windows

Record meetings with WASAPI (system loopback + microphone) and transcribe them locally. Nothing is uploaded.

Native Windows is the real capture path. WSL2 cannot hear Windows audio by itself — see [WSL2](#wsl2) if that is your daily driver.

## What you need

- Windows 10 or 11, 64-bit
- [Go 1.24+](https://go.dev/dl/) to build from source
- A C compiler for cgo (the WASAPI backend uses [malgo](https://github.com/gen2brain/malgo)). On Windows, [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) or the compiler that ships with [MSYS2](https://www.msys2.org/) is enough
- Microphone permission in **Settings → Privacy → Microphone**

The usual path is to download `anoted-windows-amd64.zip` from [Releases](https://github.com/gmarinan/anoted/releases) (**Nightly** tracks `main`). That zip is built on `windows-latest` with cgo, so you can skip the compiler.

The exe is unsigned for now. SmartScreen may show “Windows protected your PC”; More info → Run anyway until we add Authenticode signing.

## Install from a release

Unzip and run in [Windows Terminal](https://aka.ms/terminal):

```powershell
.\anoted.exe setup
.\anoted.exe doctor
.\anoted.exe watch
```

## Build from source

From PowerShell or cmd, in a directory you own:

```powershell
git clone https://github.com/gmarinan/anoted.git
cd anoted
go build -o anoted.exe ./cmd/anoted

.\anoted.exe setup
.\anoted.exe doctor
.\anoted.exe watch
```

Cross-compiling from Linux needs MinGW-w64 and cgo:

```bash
CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc make build-windows
```

## First recording

1. Run `.\anoted.exe watch` in Windows Terminal (or another modern terminal — the TUI wants a dark color-capable window).
2. Press `r` to start. The header shows **RECORDING**. Leave that visible; it is the consent signal.
3. Press `r` to stop.
4. Highlight the session, press `t` to transcribe, `o` to open Explorer on the folder.

Config: `%AppData%\anoted\config.yaml`  
Recordings: `%USERPROFILE%\Music\anoted\` unless you change `output_dir`.

```
Music\anoted\YYYY-MM-DD_HH-mm-ss_<provider>\
  recording.wav
  transcript.txt
  transcript.srt
  metadata.json
```

## How capture works

anoted uses WASAPI shared-mode **loopback** for what the other people said, and a regular **capture** stream for your mic. Both are mixed into one WAV.

Windows can refuse loopback for protected content or when the privacy policy of the endpoint says no. anoted does not try to bypass that. If Doctor reports a recorder problem, check:

- The default playback device is the one actually playing the meeting
- Exclusive-mode apps are not holding the device
- Microphone access is allowed for the terminal you launched anoted from

## Meeting detection

On Windows, detection reads `MainWindowTitle` and process names. Typical matches are Google Meet and Microsoft Teams tabs or windows. If a browser hides the title, press `r` and record manually.

## Transcription

`anoted setup` (or Doctor, key `i`) installs a local Python venv for Whisper. No cloud key, no installer wizard from OpenAI.

GPU acceleration is optional (`g` on the Doctor tab) and downloads a large CUDA wheel. CPU + the `turbo` model is enough for short meetings.

```powershell
.\anoted.exe transcribe "$env:USERPROFILE\Music\anoted\<session>"
```

## WSL2

The TUI can run inside WSL2, but Linux-in-WSL cannot capture the Windows meeting. The intended split is:

1. Build and run the TUI in WSL2 (`make build && ./bin/anoted watch`).
2. Build the helper on Windows (or cross-compile):

```bash
make build-windows-helper
# copy bin/windows-recorder.exe somewhere Windows can run it,
# e.g. C:\Program Files\anoted\
```

anoted already detects WSL2 and will prefer the helper once that path is in place. Until the helper protocol is fully wired, **native `anoted.exe` on Windows is the reliable way to record**. Use WSL2 for development of the TUI if you like; use native Windows for the actual meeting.

## Troubleshooting

| Symptom | What to try |
|---------|-------------|
| `go build` fails on cgo | Install a GCC, or download the Release zip instead |
| Doctor: recorder unusable | Confirm default playback + mic in Windows sound settings |
| Silent loopback | Some apps use exclusive mode; close them, or pick another output device in Config |
| Meet not detected | Record with `r`; titles are best-effort |
| Terminal looks broken | Use Windows Terminal, not legacy `conhost` |
| Whisper install fails | Run Doctor's `i` from an unelevated prompt with network access |

`anoted doctor` is the report to paste into a GitHub issue.
