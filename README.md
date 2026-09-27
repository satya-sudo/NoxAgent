# Nox

Nox is a local-first voice assistant with a Go backend. The initial build controls
Google Search and YouTube Music in a Chromium browser. Linux is the deployment
target. It can be used through either the text console or a browser-based
push-to-talk interface.

## Current capabilities

- Local time intent
- Google searches opened in the browser with concise visible results returned to the console
- YouTube Music search/play, pause, resume, next, previous, volume, and now-playing
- Persistent timers and alarms with restart recovery
- Linux desktop notifications through `notify-send`
- An interactive text console for office/development use
- A local dashboard with push-to-talk voice commands and spoken replies
- Local speech recognition through `whisper.cpp`
- Home Assistant light, switch, scene, and sensor control
- Optional local `llama.cpp` conversational and intent fallback
- A narrow browser-action protocol—Nox cannot send arbitrary JavaScript
- Chromium Manifest V3 extension
- Linux user-level systemd unit

## Requirements

- Linux for deployment (development works on other Go-supported systems)
- Go 1.24 or newer
- Google Chrome, Chromium, or a compatible browser
- An existing YouTube Music browser session

## Run in development

```sh
make test
make run
```

In another terminal:

```sh
go run ./cmd/noxctl "what time is it"
go run ./cmd/noxctl "search Google for weather in Delhi"
go run ./cmd/noxctl "play Numb on YouTube Music"
```

Or start the interactive console:

```sh
go run ./cmd/noxctl
```

Open the local dashboard at [http://127.0.0.1:7080/](http://127.0.0.1:7080/).
Allow microphone access, press the microphone orb, speak, and press it again to
submit. Saying **“Hey Nox”** makes Nox reply **“Sir.”** Enable **Hands-free
“Hey Nox”** to keep the microphone open: background speech is ignored until the
wake phrase is detected, and Nox then accepts a follow-up command for nine
seconds.

## Voice setup

Nox runs speech recognition locally by invoking `whisper-cli`. Configure the
binary and a downloaded whisper.cpp model before starting Nox:

```sh
export NOX_WHISPER_BIN=/path/to/whisper-cli
export NOX_WHISPER_MODEL=/path/to/ggml-base.en.bin
export NOX_WHISPER_LANGUAGE=en
make run
```

For development on macOS, install whisper.cpp with Homebrew:

```sh
brew install whisper-cpp
```

The dashboard uses the browser's speech-synthesis engine for replies. Speech
recognition remains on the local computer; no cloud speech API is required.
The current interface is push-to-talk. Always-listening Linux wake-word
detection is a separate deployment milestone.

Timer and alarm examples:

```text
set a timer for 10 minutes
set a timer for 1 hour and 30 minutes
set an alarm for 7:30 AM
set an alarm for 7 tomorrow morning
list timers
list alarms
cancel timer abc123
cancel all alarms
```

Nox returns a short ID when creating an alarm or timer. Use that ID to cancel a
specific item. On Linux, install `notify-send` (commonly provided by
`libnotify-bin`) to receive desktop notifications. State is stored at
`$XDG_CONFIG_HOME/nox/state.json`, normally `~/.config/nox/state.json`.

## Home Assistant

Create a long-lived access token from your Home Assistant profile, then configure
Nox without committing the token:

```sh
export NOX_HOME_ASSISTANT_URL=http://homeassistant.local:8123
export NOX_HOME_ASSISTANT_TOKEN=your-long-lived-token
make run
```

Supported commands include:

```text
turn on the living room lights
turn off the coffee machine switch
activate scene movie night
status of bedroom temperature
```

Nox resolves the friendly names reported by Home Assistant. State-changing
commands are restricted to `light`, `switch`, and `scene` entities; arbitrary
Home Assistant services cannot be called through a text command.

## Optional local LLM

Deterministic commands are always attempted first. For commands that do not
match a built-in rule, Nox can use an OpenAI-compatible local server such as
`llama.cpp`:

```sh
llama-server -m /path/to/model.gguf --host 127.0.0.1 --port 8080

export NOX_LLM_URL=http://127.0.0.1:8080
export NOX_LLM_MODEL=local
make run
```

The model cannot create arbitrary operations. Its JSON output is validated
against Nox's fixed tool allowlist before anything is executed. If no LLM URL is
configured, unknown commands return an ordinary unsupported-command error.

For the macOS reference setup and an Intel i7/32 GB Linux target, start with the
official Qwen3 4B Q4 model:

```sh
brew install llama.cpp # macOS; install a llama.cpp release on Linux
llama-server -hf Qwen/Qwen3-4B-GGUF:Q4_K_M \
  --alias nox-local --host 127.0.0.1 --port 8080 -c 4096 --jinja

export NOX_LLM_URL=http://127.0.0.1:8080
export NOX_LLM_MODEL=nox-local
make run
```

Nox disables extended model thinking for voice latency, retains the most recent
six conversation turns, and records successful tool results so follow-up
requests can refer to prior commands. The dashboard reports the model as online
only after its health endpoint responds.

Browser commands wait for the extension to report success. If the extension is
not installed or the browser is closed, they time out after 15 seconds.

## Install the development extension

1. Open `chrome://extensions` in Chrome or Chromium.
2. Enable **Developer mode**.
3. Choose **Load unpacked**.
4. Select this repository's `extension` directory.
5. Start `noxd` and keep YouTube Music signed in.

The extension can access only the local Nox endpoint, Google Search, and YouTube
Music. Reload it from the extensions page after changing its JavaScript.

## HTTP API

Submit a command:

```sh
curl -sS http://127.0.0.1:7080/v1/commands \
  -H 'Content-Type: application/json' \
  -d '{"text":"pause the music"}'
```

Health check:

```sh
curl -sS http://127.0.0.1:7080/healthz
```

Browser extension connection status:

```sh
curl -sS http://127.0.0.1:7080/v1/browser/status
```

Combined runtime status:

```sh
curl -sS http://127.0.0.1:7080/v1/status
```

## Linux service

Build and install the daemon for the current user:

```sh
go build -o "$HOME/.local/bin/noxd" ./cmd/noxd
mkdir -p "$HOME/.config/systemd/user"
mkdir -p "$HOME/.config/nox"
cp deployments/systemd/nox.service "$HOME/.config/systemd/user/nox.service"
cat > "$HOME/.config/nox/nox.env" <<'EOF'
NOX_HOME_ASSISTANT_URL=http://homeassistant.local:8123
NOX_HOME_ASSISTANT_TOKEN=replace-with-your-token
# Optional local llama.cpp endpoint:
# NOX_LLM_URL=http://127.0.0.1:8080
# NOX_LLM_MODEL=local
EOF
chmod 600 "$HOME/.config/nox/nox.env"
systemctl --user daemon-reload
systemctl --user enable --now nox
```

Inspect logs with:

```sh
journalctl --user -u nox -f
```

## Next slice

The next voice slice will add always-listening Linux wake-word detection, voice
activity detection, and native offline speech output. Reminders, recurring
schedules, and richer dashboard activity history remain planned.
