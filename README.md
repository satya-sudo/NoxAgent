# Nox

Nox is a local-first personal voice assistant with a Go backend. It is designed
to provide an Alexa/Google Home-style experience while keeping speech,
conversation, automation, and state on hardware you control.

The current reference build runs on macOS. Linux is the deployment target, with
an Intel i7 and 32 GB RAM as the baseline machine. Nox can be controlled from a
web dashboard, an interactive terminal, or its HTTP API.

> **Project status:** active development. Core commands, local speech
> recognition, local conversation, browser control, timers, alarms, and Home
> Assistant integration are working. Native Linux wake-word detection and native
> offline speech output are still planned.

## What Nox can do

- Respond to **“Hey Nox”** with **“Sir.”**
- Accept push-to-talk or hands-free commands from the local dashboard.
- Answer general questions using a local Qwen model through `llama.cpp`.
- Remember the latest six conversation turns for contextual follow-ups.
- Search Google and read concise visible results.
- Control YouTube Music in Chromium: search/play, pause/resume, next/previous,
  volume, and now playing.
- Create, list, and cancel persistent timers and alarms.
- Recover scheduled timers and alarms after a Nox restart.
- Control Home Assistant lights, switches, scenes, and read entity state.
- Check unread Gmail and search message metadata through read-only OAuth.
- Run without a cloud speech or conversational API.

## Architecture

```text
Browser microphone / terminal / HTTP client
                    |
                    v
             Go API and dashboard
                    |
        +-----------+------------+
        |           |            |
        v           v            v
  whisper.cpp   Intent rules   Qwen + llama.cpp
    local STT     fast path    conversation fallback
        |           |            |
        +-----------+------------+
                    |
          Validated action allowlist
                    |
       +------------+-------------+
       |            |             |
       v            v             v
 Browser bridge     Scheduler       Home Assistant
 Google/YT Music    timers/alarms   local REST API
```

Deterministic rules handle common commands first. Unknown or conversational
requests are sent to the local LLM. Model output is validated against a fixed
action allowlist; the LLM cannot execute shell commands, arbitrary JavaScript,
or unregistered operations.

## Requirements

### macOS development/reference setup

- Apple Silicon or Intel Mac
- Go 1.24 or newer
- Chrome, Chromium, or another Chromium-compatible browser
- Homebrew
- A signed-in YouTube Music browser session for music control

### Linux deployment target

- Modern 64-bit Linux
- Intel i7-class CPU and 32 GB RAM recommended
- Go 1.24 or newer for source builds
- Chromium-compatible browser
- `notify-send`, normally supplied by `libnotify-bin`
- `whisper.cpp` and `llama.cpp`

The default Qwen3 4B Q4 model occupies approximately 2.5 GB. It runs on CPU and
fits comfortably in 32 GB RAM. Larger models may improve answer quality but
increase voice-response latency.

## Quick start on macOS

### 1. Install the local runtimes

```sh
brew install whisper-cpp llama.cpp
```

### 2. Download a Whisper model

From the repository root:

```sh
mkdir -p models
curl -L \
  https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.en.bin \
  -o models/ggml-base.en.bin
```

The `models` directory is ignored by Git.

### 3. Start the local LLM

In terminal one:

```sh
llama-server -hf Qwen/Qwen3-4B-GGUF:Q4_K_M \
  --alias nox-local \
  --host 127.0.0.1 \
  --port 8080 \
  -c 4096 \
  --jinja
```

The first run downloads the model. Later runs use the local cache.

### 4. Start Nox

In terminal two, from the repository root:

```sh
export NOX_WHISPER_BIN="$(command -v whisper-cli)"
export NOX_WHISPER_MODEL="$PWD/models/ggml-base.en.bin"
export NOX_WHISPER_LANGUAGE=en
export NOX_LLM_URL=http://127.0.0.1:8080
export NOX_LLM_MODEL=nox-local

make run
```

Nox listens only on `127.0.0.1:7080` by default.

### 5. Install the browser bridge

1. Open `chrome://extensions`.
2. Enable **Developer mode**.
3. Select **Load unpacked**.
4. Select the repository's `extension` directory.
5. Keep Chromium open and sign in to YouTube Music.

Reload the extension after changing files inside `extension/`.

### 6. Open the dashboard

Open [http://127.0.0.1:7080/](http://127.0.0.1:7080/).

- Allow microphone access.
- Press the orb once to record and again to submit.
- Enable **Hands-free “Hey Nox”** for wake-gated listening.
- Choose a system voice from the reply voice menu.

In hands-free mode, ordinary room speech is ignored until Nox detects its wake
phrase. A wake-only utterance receives **“Sir.”** and opens a nine-second window
for the next command. A combined phrase such as **“Hey Nox, play Not Sober”**
can be handled in one utterance.

## Terminal usage

Run one command:

```sh
go run ./cmd/noxctl "what time is it"
go run ./cmd/noxctl "set a timer for 10 minutes"
go run ./cmd/noxctl "search Google for weather in Delhi"
go run ./cmd/noxctl "play Not Sober on YouTube Music"
```

Start the interactive console:

```sh
go run ./cmd/noxctl
```

The console prompt is `nox>`.

## Supported commands

Examples are intentionally conversational; the local LLM can map additional
natural phrasings to the same registered actions.

| Area | Examples |
| --- | --- |
| Wake | `Hey Nox`, `Hi Nox` |
| Time | `What time is it?`, `Tell me the time` |
| Search | `Search Google for weather in Delhi`, `Google Go tutorials` |
| Music | `Play Numb on YouTube Music`, `pause music`, `resume`, `next track`, `previous track` |
| Music state | `What song is playing?`, `set volume to 40%` |
| Timers | `Set a timer for 10 minutes`, `list timers`, `cancel timer abc123`, `cancel all timers` |
| Alarms | `Set an alarm for 7:30 AM`, `set an alarm for 7 tomorrow morning`, `list alarms` |
| Home | `Turn on the living room lights`, `turn off the coffee machine switch` |
| Scenes | `Activate scene movie night` |
| Sensors | `Status of bedroom temperature` |
| Gmail | `Check my email`, `search Gmail for from:alice` |
| Conversation | `Tell me something about the Moon`, then `How long does it take to orbit us?` |

Nox returns a short identifier when it creates an alarm or timer. Use that
identifier to cancel a specific entry.

## Configuration

Nox is configured with environment variables.

| Variable | Default | Purpose |
| --- | --- | --- |
| `NOX_ADDRESS` | `127.0.0.1:7080` | API and dashboard listen address |
| `NOX_ACTION_TIMEOUT` | `15s` | Maximum wait for a browser action |
| `NOX_STATE_PATH` | OS user config directory | Persistent timer/alarm state file |
| `NOX_WHISPER_BIN` | unset | Path to `whisper-cli` |
| `NOX_WHISPER_MODEL` | unset | Path to a Whisper GGML model |
| `NOX_WHISPER_LANGUAGE` | `en` | Whisper language code |
| `NOX_LLM_URL` | unset | Local OpenAI-compatible server base URL |
| `NOX_LLM_MODEL` | `local` | Model or server alias |
| `NOX_LLM_API_KEY` | unset | Optional local-server API key |
| `NOX_HOME_ASSISTANT_URL` | unset | Home Assistant base URL |
| `NOX_HOME_ASSISTANT_TOKEN` | unset | Home Assistant long-lived access token |
| `NOX_GMAIL_CREDENTIALS_PATH` | unset | Google OAuth web-client JSON; enables Gmail |
| `NOX_GMAIL_TOKEN_PATH` | OS user config directory | Protected local OAuth token file |
| `NOX_GMAIL_REDIRECT_URL` | `http://127.0.0.1:7080/v1/integrations/gmail/auth/callback` | OAuth callback URL |

Voice recognition is enabled only when both `NOX_WHISPER_BIN` and
`NOX_WHISPER_MODEL` are set. The LLM is optional; without it, registered
deterministic commands continue to work and unknown commands return an
unsupported-command error.

## Home Assistant

Create a long-lived access token in the Home Assistant profile and start Nox
with:

```sh
export NOX_HOME_ASSISTANT_URL=http://homeassistant.local:8123
export NOX_HOME_ASSISTANT_TOKEN=replace-with-your-token
make run
```

Nox resolves Home Assistant friendly names. State changes are restricted to
`light`, `switch`, and `scene` entities. Tokens should be stored in a protected
environment file and must not be committed.

## Gmail integration

The initial Gmail integration is deliberately read-only. It can report its
connection state, answer `check my email`, search message metadata, and retrieve
snippets, but it cannot
send, modify, archive, or delete mail.

1. Create a project in Google Cloud and enable the Gmail API.
2. Configure the OAuth consent screen.
3. Create an **OAuth client ID** with application type **Web application**.
4. Add this exact authorized redirect URI:
   `http://127.0.0.1:7080/v1/integrations/gmail/auth/callback`
5. Download the client JSON outside the repository and start Nox with:

```sh
export NOX_GMAIL_CREDENTIALS_PATH="$HOME/.config/nox/gmail-client.json"
make run
```

Start authorization and open the returned `authorization_url` in a browser:

```sh
curl -sS http://127.0.0.1:7080/v1/integrations/gmail/auth/start
```

Google redirects back to Nox after consent. The refresh token is saved by
default under the OS user configuration directory with mode `0600`. OAuth
client files and Gmail token files are ignored by Git; neither should be
committed. If Nox listens at another address, configure the same exact URL in
Google Cloud and `NOX_GMAIL_REDIRECT_URL`.

## HTTP API

### Health and runtime status

```sh
curl -sS http://127.0.0.1:7080/healthz
curl -sS http://127.0.0.1:7080/v1/status
```

`/v1/status` reports browser bridge, speech recognition, and local LLM
availability, plus Gmail configuration and connection state.

### Gmail status and read-only message listing

```sh
curl -sS http://127.0.0.1:7080/v1/integrations/gmail/status
curl -sS 'http://127.0.0.1:7080/v1/integrations/gmail/messages?limit=10&query=is%3Aunread'
```

The `query` value uses Gmail search syntax. The result contains sender,
subject, date, snippet, message ID, and thread ID; message bodies are not
returned. The maximum page size is 50.

### Submit a text command

```sh
curl -sS http://127.0.0.1:7080/v1/commands \
  -H 'Content-Type: application/json' \
  -d '{"text":"pause the music"}'
```

### Submit recorded speech

```sh
curl -sS http://127.0.0.1:7080/v1/voice/commands \
  -H 'Content-Type: audio/wav' \
  --data-binary @command.wav
```

Audio must be WAV. The request limit is 25 MB. The dashboard records mono audio
and converts it to 16 kHz PCM before submission.

### Browser bridge status

```sh
curl -sS http://127.0.0.1:7080/v1/browser/status
```

The browser action endpoints are intended for the Nox extension, not general
clients.

## Browser bridge security

The Manifest V3 extension is limited to:

- `http://127.0.0.1:7080/*`
- `https://www.google.com/*`
- `https://music.youtube.com/*`

Nox sends typed actions such as `youtube_music.pause` or
`browser.google_search`; it does not transmit arbitrary JavaScript for browser
execution. Browser automation still depends on the current page structure, so
Google or YouTube Music interface changes can require selector updates.

## State, privacy, and safety

- Speech transcription runs through the configured local Whisper binary.
- Conversation runs through the configured local LLM server.
- Timer and alarm state is stored locally as JSON.
- The default API and model commands bind to loopback addresses only.
- LLM-selected actions are checked against a fixed allowlist.
- Home Assistant credentials are read from environment variables.
- Gmail uses the read-only OAuth scope and stores its refresh token locally in
  a file protected with mode `0600`.
- Google Search and YouTube Music necessarily communicate with their respective
  websites through the signed-in browser.

On macOS, timers and alarms use a system notification and sound. On Linux, Nox
uses `notify-send`. Native continuous alarm ringing, snooze, and voice dismissal
are not implemented yet.

## Linux installation

Install `whisper.cpp`, `llama.cpp`, Chromium, and desktop notifications using
the packages appropriate for the Linux distribution. Then build Nox:

```sh
mkdir -p "$HOME/.local/bin" "$HOME/.config/systemd/user" "$HOME/.config/nox"
go build -o "$HOME/.local/bin/noxd" ./cmd/noxd
cp deployments/systemd/nox.service "$HOME/.config/systemd/user/nox.service"
```

Create `$HOME/.config/nox/nox.env`:

```sh
NOX_WHISPER_BIN=/usr/local/bin/whisper-cli
NOX_WHISPER_MODEL=/home/your-user/.local/share/nox/ggml-base.en.bin
NOX_WHISPER_LANGUAGE=en
NOX_LLM_URL=http://127.0.0.1:8080
NOX_LLM_MODEL=nox-local

# Optional Home Assistant integration:
# NOX_HOME_ASSISTANT_URL=http://homeassistant.local:8123
# NOX_HOME_ASSISTANT_TOKEN=replace-with-your-token
```

Protect the configuration and enable the service:

```sh
chmod 600 "$HOME/.config/nox/nox.env"
systemctl --user daemon-reload
systemctl --user enable --now nox
journalctl --user -u nox -f
```

The included unit starts the Go daemon. Run `llama-server` separately or add a
dedicated user service for it before starting Nox.

## Development

```sh
make fmt
make test
make build
```

Additional useful checks:

```sh
go vet ./...
go test -race ./...
GOOS=linux GOARCH=amd64 go build ./cmd/noxd ./cmd/noxctl
node --check extension/background.js
node --check extension/google-search.js
node --check extension/youtube-music.js
node --check internal/api/ui/app.js
```

### Repository layout

```text
cmd/noxd/                 Go daemon entry point
cmd/noxctl/               Command-line client
internal/api/             HTTP API and embedded dashboard
internal/assistant/       Command orchestration
internal/intent/          Deterministic intent parser
internal/llm/             Local OpenAI-compatible LLM client
internal/voice/           whisper.cpp command wrapper
internal/browser/         Typed browser-action broker
internal/scheduler/       Persistent timers and alarms
internal/homeassistant/   Home Assistant client
internal/gmail/           Gmail OAuth and read-only API client
internal/notify/          macOS/Linux notifications
extension/                Chromium browser bridge
deployments/systemd/      Linux user service
```

## Troubleshooting

### `browser extension did not respond`

- Keep Chrome or Chromium open.
- Confirm the Nox Browser Bridge is enabled at `chrome://extensions`.
- Reload the extension after editing it.
- Check `http://127.0.0.1:7080/v1/browser/status`.
- Keep a signed-in YouTube Music tab available for music commands.

### `Nox is unavailable` or connection refused

- Confirm `noxd` is running.
- Check whether another process already occupies port `7080`.
- Open `http://127.0.0.1:7080/healthz`.

### `LLM OFFLINE`

- Start `llama-server` on port `8080`.
- Ensure `NOX_LLM_URL` is `http://127.0.0.1:8080`.
- Ensure the model alias matches `NOX_LLM_MODEL`.
- Check `http://127.0.0.1:8080/health`.

### Voice model not configured

- Set both `NOX_WHISPER_BIN` and `NOX_WHISPER_MODEL`.
- Verify that the binary and model paths exist.
- Allow microphone permission for the browser.
- Record for at least half a second before submitting.

### Speech is inaccurate

- Speak after the dashboard shows **Listening**.
- Reduce background noise and move closer to the microphone.
- Use a larger compatible Whisper model if the hardware can tolerate the added
  latency.
- Inspect the displayed transcript before diagnosing the intent parser.

### Search answers repeat the question

Reload the browser extension. The extension rejects page headings that merely
echo the query, but Google page changes can still affect extraction.

## Current limitations and roadmap

- Replace browser-level hands-free detection with a native Linux wake-word and
  voice-activity pipeline.
- Add consistent native offline speech output instead of browser system voices.
- Add barge-in so the user can interrupt a spoken response.
- Add continuous alarm ringing, snooze, and voice dismissal.
- Add reminders and recurring schedules.
- Add dashboard cards for read-only Gmail summaries after the OAuth foundation
  is validated.
- Add a noisy-room and accent evaluation suite.
- Harden browser automation against Google and YouTube Music UI changes.
- Package the Go daemon, model services, models, and permissions for a one-step
  Linux installation.

## Related projects

- [whisper.cpp](https://github.com/ggml-org/whisper.cpp)
- [llama.cpp](https://github.com/ggml-org/llama.cpp)
- [Qwen3-4B-GGUF](https://huggingface.co/Qwen/Qwen3-4B-GGUF)
