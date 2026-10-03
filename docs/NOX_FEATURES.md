# Nox — Private, Local, Yours

Nox is a self-hosted voice assistant built to deliver an Alexa or Google
Home-style experience on hardware you control. Its Go backend coordinates local
speech recognition, a local language model, browser media control, schedules,
smart-home devices, and personal integrations without making a cloud assistant
the center of your home.

The reference development build runs on macOS. Linux is the deployment target,
with an Intel i7 and 32 GB RAM as the baseline system.

## The experience

Say **“Hey Nox”** and Nox replies **“Sir.”** Continue with a natural command or
combine the wake phrase and request:

> “Hey Nox, play Not Sober.”

> “Hey Nox, set a timer for ten minutes.”

> “Hey Nox, check my email.”

Nox can also be used through its web dashboard, terminal client, or HTTP API.

## Current capabilities

### Voice and conversation

- Local speech-to-text through `whisper.cpp`.
- Push-to-talk and wake-gated hands-free modes in the dashboard.
- Wake-phrase handling for “Hey Nox” and common transcription variations.
- General conversation and follow-up questions through a local Qwen model
  running on `llama.cpp`.
- Short conversational memory for contextual follow-ups.
- Browser-provided spoken replies on the current macOS interface.

### YouTube Music

Nox controls an authenticated YouTube Music session directly through its
Chromium browser bridge. It can:

- Search for and play a song, artist, album, or general music request.
- Pause and resume playback.
- Skip to the next or previous track.
- Change playback volume.
- Report what is currently playing.

No Spotify account or Spotify API is required.

### Search and live information

- Search Google through the browser bridge.
- Read concise visible search results back to the user.
- Use search for changing information such as weather and current events.
- Use the local model for stable knowledge and ordinary conversation.

### Timers and alarms

- Create timers using natural durations.
- Create alarms for a time today or tomorrow.
- List active timers and alarms.
- Cancel one schedule by its short identifier or cancel all of a type.
- Persist schedules across Nox restarts.
- Deliver native notifications and sounds on macOS and Linux.

### Smart-home control

Through Home Assistant, Nox can:

- Turn lights and switches on or off.
- Activate scenes.
- Read sensor and entity state.
- Resolve human-friendly device names.

Home Assistant state changes are restricted to registered entity types instead
of allowing arbitrary service calls.

### WiZ local lighting

Nox can discover and control WiZ devices directly on the LAN without Home
Assistant or a cloud API. The native Go integration currently supports power,
brightness, and warm/cool color temperature, including commands such as:

> “Nox, make the T-Beamer warm and 20%.”

Communication stays on the local network using the WiZ UDP protocol.

### Gmail

The Gmail integration uses Google OAuth and requests read-only access. Nox can:

- Connect a Gmail account through a browser authorization flow.
- Check unread inbox messages.
- Search mail using Gmail search expressions.
- Read sender, subject, date, and a short message snippet.

Nox cannot send, edit, archive, or delete email in the current implementation.
OAuth tokens are stored locally in a protected file and are excluded from Git.

### Interfaces

- Responsive local web dashboard.
- Interactive `nox>` terminal client.
- JSON HTTP API for commands, voice recordings, status, and integrations.
- Typed Chromium extension for Google Search and YouTube Music control.

## Privacy and safety

Nox is designed around local execution and narrowly scoped tools:

- Speech recognition runs on the Nox machine.
- Conversation runs against the configured local model server.
- The API listens on `127.0.0.1` by default.
- LLM-selected actions are checked against a fixed allowlist.
- The model cannot run shell commands or arbitrary browser JavaScript.
- Timer, alarm, conversation, and integration state remains local.
- Gmail uses the read-only OAuth scope.
- Home Assistant and OAuth credentials are never intended for source control.

Google Search, YouTube Music, Gmail, and Home Assistant naturally communicate
with their respective services when those integrations are used.

## How Nox works

```text
Microphone / dashboard / terminal / HTTP client
                       |
                       v
                  Go backend
                       |
        +--------------+---------------+
        |              |               |
        v              v               v
  whisper.cpp    intent rules     Qwen + llama.cpp
   local STT      fast path      conversation fallback
        |              |               |
        +--------------+---------------+
                       |
              validated tool actions
                       |
        +--------------+---------------+
        |              |               |
        v              v               v
 Browser bridge    Scheduler     Home Assistant / Gmail
```

Common commands use deterministic intent rules for speed and reliability. More
natural or conversational requests fall back to the local LLM. Any tool action
selected by the model must still pass Nox's validation layer before execution.

## Example commands

| Area | Try saying |
| --- | --- |
| Wake | “Hey Nox” |
| Conversation | “Tell me something interesting about the Moon” |
| Search | “Search Google for weather in Delhi” |
| Music | “Play Not Sober on YouTube Music” |
| Playback | “Pause the music” or “Next track” |
| Timer | “Set a timer for ten minutes” |
| Alarm | “Set an alarm for 7:30 tomorrow morning” |
| Smart home | “Turn on the living room lights” |
| Sensors | “What is the status of bedroom temperature?” |
| Gmail | “Check my email” |
| Gmail search | “Search Gmail for from:alice” |

## Roadmap to a complete home assistant

- Native Linux wake-word detection independent of the browser.
- Voice activity detection for cleaner start and end-of-speech handling.
- Consistent offline text-to-speech with selectable local voices.
- Barge-in so speech can interrupt a long Nox reply.
- Continuous alarms with snooze and spoken dismissal.
- Reminders, recurring schedules, routines, and household profiles.
- Dashboard cards for Gmail, devices, schedules, and currently playing media.
- Improved far-field and noisy-room microphone performance.
- One-command Linux installation and system services for Nox, Whisper, and the
  local LLM.

## Project status

Nox is under active development. The Go daemon, local conversation, local
speech recognition, browser bridge, YouTube Music control, Google Search,
scheduler, Home Assistant integration, Gmail read-only integration, terminal,
and dashboard are implemented. The roadmap items above are not yet complete and
should not be presented as current functionality.
