const $ = (id) => document.getElementById(id);
const messages = $("messages");
let recording = null;
let armedUntil = 0;
let speaking = false;
let restartTimer = null;

const hour = new Date().getHours();
$("greeting").textContent = `Good ${hour < 12 ? "morning" : hour < 17 ? "afternoon" : "evening"}, Sir.`;

function addMessage(who, text) {
  const row = document.createElement("div");
  row.className = `message ${who}`;
  const label = document.createElement("span");
  label.textContent = who === "user" ? "YOU" : "NOX";
  const body = document.createElement("p");
  body.textContent = text;
  row.append(label, body);
  messages.append(row);
  messages.scrollTop = messages.scrollHeight;
}

function loadVoices() {
  if (!window.speechSynthesis) return;
  const select = $("voiceChoice");
  const voices = speechSynthesis.getVoices().filter((voice) => voice.lang.toLowerCase().startsWith("en"));
  if (!voices.length) return;
  const saved = localStorage.getItem("noxVoice");
  const preferred = ["Samantha", "Ava", "Karen", "Daniel", "Google UK English Female"];
  const selected = voices.find((voice) => voice.name === saved)
    || preferred.map((name) => voices.find((voice) => voice.name.includes(name))).find(Boolean)
    || voices.find((voice) => voice.localService)
    || voices[0];
  select.replaceChildren(...voices.map((voice) => {
    const option = document.createElement("option");
    option.value = voice.name;
    option.textContent = `${voice.name} (${voice.lang})`;
    option.selected = voice.name === selected.name;
    return option;
  }));
}

loadVoices();
if (window.speechSynthesis) speechSynthesis.onvoiceschanged = loadVoices;
$("voiceChoice").addEventListener("change", (event) => localStorage.setItem("noxVoice", event.target.value));

function speak(text) {
  if (!$("speakReplies").checked || !window.speechSynthesis) return Promise.resolve();
  speechSynthesis.cancel();
  speaking = true;
  const utterance = new SpeechSynthesisUtterance(text);
  const selected = speechSynthesis.getVoices().find((voice) => voice.name === $("voiceChoice").value);
  if (selected) {
    utterance.voice = selected;
    utterance.lang = selected.lang;
  }
  utterance.rate = 0.94;
  utterance.pitch = 1;
  return new Promise((resolve) => {
    const finished = () => { speaking = false; resolve(); };
    utterance.onend = finished;
    utterance.onerror = finished;
    speechSynthesis.speak(utterance);
  });
}

async function runCommand(text) {
  text = text.trim();
  if (!text) return;
  addMessage("user", text);
  try {
    const response = await fetch("/v1/commands", {
      method: "POST",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify({text}),
    });
    const data = await response.json();
    const reply = data.message || data.error || "No response.";
    addMessage("nox", reply);
    if (response.ok) speak(reply);
  } catch {
    addMessage("nox", "I couldn't reach the Nox service.");
  }
}

$("commandForm").addEventListener("submit", async (event) => {
  event.preventDefault();
  const input = $("commandInput");
  const text = input.value;
  input.value = "";
  await runCommand(text);
});
document.querySelectorAll("[data-command]").forEach((button) => {
  button.addEventListener("click", () => runCommand(button.dataset.command));
});

function restartHandsFree(delay = 300) {
  clearTimeout(restartTimer);
  if (!$("handsFree").checked || recording || speaking) return;
  restartTimer = setTimeout(() => {
    if ($("handsFree").checked && !recording && !speaking) {
      toggleRecording(true).catch(showMicrophoneError);
    }
  }, delay);
}

function showMicrophoneError(error) {
  recording = null;
  $("voiceButton").classList.remove("recording");
  $("voiceHint").textContent = "Microphone unavailable";
  addMessage("nox", `Microphone error: ${error.message}`);
}

async function toggleRecording(automatic = false) {
  if (recording) {
    await recording.stop();
    return;
  }
  if (window.speechSynthesis) speechSynthesis.cancel();
  const stream = await navigator.mediaDevices.getUserMedia({
    audio: {channelCount: 1, echoCancellation: true, noiseSuppression: true, autoGainControl: true},
  });
  const context = new AudioContext({latencyHint: "interactive"});
  const source = context.createMediaStreamSource(stream);
  const processor = context.createScriptProcessor(2048, 1, 1);
  const silentOutput = context.createGain();
  silentOutput.gain.value = 0;
  const chunks = [];
  let capturedSamples = 0;
  const startedAt = performance.now();
  let stopping = false;
  let speechDetected = false;
  let lastSpeechAt = 0;

  processor.onaudioprocess = (event) => {
    const samples = new Float32Array(event.inputBuffer.getChannelData(0));
    chunks.push(samples);
    capturedSamples += samples.length;
    let energy = 0;
    for (const sample of samples) energy += sample * sample;
    const level = Math.min(1, Math.sqrt(energy / samples.length) * 8);
    $("voiceButton").style.setProperty("--level", level.toFixed(2));
    const now = performance.now();
    if (automatic && !speechDetected) {
      const preRollSamples = context.sampleRate * 0.5;
      while (capturedSamples > preRollSamples && chunks.length > 1) {
        capturedSamples -= chunks.shift().length;
      }
    }
    if (level > 0.075) {
      if (!speechDetected && automatic) {
        recording.speechTimeout = setTimeout(() => recording?.stop(), 15000);
      }
      speechDetected = true;
      lastSpeechAt = now;
    }
    if (automatic && speechDetected && now - lastSpeechAt > 850 && now - startedAt > 1000) {
      recording?.stop();
    }
  };
  source.connect(processor);
  processor.connect(silentOutput);
  silentOutput.connect(context.destination);

  const stop = async (submit = true) => {
    if (stopping) return;
    stopping = true;
    clearTimeout(recording?.timeout);
    clearTimeout(recording?.speechTimeout);
    processor.disconnect();
    source.disconnect();
    silentOutput.disconnect();
    stream.getTracks().forEach((track) => track.stop());
    recording = null;
    $("voiceButton").classList.remove("recording");
    $("voiceButton").style.removeProperty("--level");
    $("voiceHint").textContent = "Transcribing…";
    const wav = encodeWav(chunks, context.sampleRate, 16000);
    await context.close();
    if (!submit || (automatic && !speechDetected)) {
      $("voiceHint").textContent = "Waiting for “Hey Nox”…";
      restartHandsFree();
      return;
    }
    if (performance.now() - startedAt < 450) {
      $("voiceHint").textContent = "Hold a little longer, then speak";
      restartHandsFree(700);
      return;
    }
    try {
      const response = await fetch("/v1/voice/commands", {
        method: "POST",
        headers: {
          "Content-Type": "audio/wav",
          "X-Nox-Require-Wake": automatic && Date.now() > armedUntil ? "true" : "false",
        },
        body: wav,
      });
      const data = await response.json();
      if (data.ignored) {
        $("voiceHint").textContent = "Waiting for “Hey Nox”…";
        return;
      }
      if (data.transcript) addMessage("user", data.transcript);
      if (response.ok) {
        const reply = data.reply?.message || "Done.";
        addMessage("nox", reply);
        armedUntil = data.reply?.intent === "wake.greet" ? Date.now() + 9000 : 0;
        await speak(reply);
      } else if (data.transcript) {
        addMessage("nox", `I heard “${data.transcript}”, but I couldn't match it to a command.`);
      } else {
        addMessage("nox", data.error || "I couldn't understand the recording.");
      }
    } catch {
      addMessage("nox", "I couldn't reach the Nox service.");
    } finally {
      $("voiceHint").textContent = automatic ? "Waiting for “Hey Nox”…" : "Tap to speak";
      restartHandsFree();
    }
  };

  recording = {stop, automatic};
  if (!automatic) recording.timeout = setTimeout(stop, 15000);
  $("voiceButton").classList.add("recording");
  $("voiceHint").textContent = automatic ? "Waiting for “Hey Nox”…" : "Listening… tap when finished";
}

$("voiceButton").addEventListener("click", () => {
  toggleRecording(false).catch(showMicrophoneError);
});

$("handsFree").addEventListener("change", async (event) => {
  localStorage.setItem("noxHandsFree", event.target.checked ? "true" : "false");
  if (event.target.checked) {
    restartHandsFree(0);
  } else if (recording?.automatic) {
    await recording.stop(false);
    $("voiceHint").textContent = "Tap to speak";
  }
});

function encodeWav(chunks, inputRate, outputRate) {
  const total = chunks.reduce((count, chunk) => count + chunk.length, 0);
  const merged = new Float32Array(total);
  let position = 0;
  chunks.forEach((chunk) => {
    merged.set(chunk, position);
    position += chunk.length;
  });
  const threshold = 0.006;
  let first = 0;
  let last = merged.length - 1;
  while (first < merged.length && Math.abs(merged[first]) < threshold) first++;
  while (last > first && Math.abs(merged[last]) < threshold) last--;
  const padding = Math.floor(inputRate * 0.2);
  first = Math.max(0, first - padding);
  last = Math.min(merged.length - 1, last + padding);
  const trimmed = first < merged.length ? merged.subarray(first, last + 1) : merged;
  const ratio = inputRate / outputRate;
  const length = Math.max(1, Math.floor(trimmed.length / ratio));
  const samples = new Float32Array(length);
  for (let index = 0; index < length; index++) {
    const start = Math.floor(index * ratio);
    const end = Math.max(start + 1, Math.min(trimmed.length, Math.floor((index + 1) * ratio)));
    let sum = 0;
    for (let sourceIndex = start; sourceIndex < end; sourceIndex++) sum += trimmed[sourceIndex];
    samples[index] = sum / (end - start);
  }
  const buffer = new ArrayBuffer(44 + length * 2);
  const view = new DataView(buffer);
  const write = (offset, value) => [...value].forEach((char, index) => view.setUint8(offset + index, char.charCodeAt(0)));
  write(0, "RIFF"); view.setUint32(4, 36 + length * 2, true); write(8, "WAVE"); write(12, "fmt ");
  view.setUint32(16, 16, true); view.setUint16(20, 1, true); view.setUint16(22, 1, true);
  view.setUint32(24, outputRate, true); view.setUint32(28, outputRate * 2, true);
  view.setUint16(32, 2, true); view.setUint16(34, 16, true); write(36, "data");
  view.setUint32(40, length * 2, true);
  for (let index = 0; index < length; index++) {
    const sample = Math.max(-1, Math.min(1, samples[index]));
    view.setInt16(44 + index * 2, sample < 0 ? sample * 32768 : sample * 32767, true);
  }
  return buffer;
}

async function refreshStatus() {
  try {
    const status = await (await fetch("/v1/status")).json();
    $("browserStatus").textContent = status.browser.connected ? "BROWSER ONLINE" : "BROWSER OFFLINE";
    $("browserStatus").classList.toggle("online", status.browser.connected);
    $("llmStatus").textContent = status.llm?.available ? "LLM ONLINE" : status.llm?.configured ? "LLM STARTING" : "LLM OFFLINE";
    $("llmStatus").classList.toggle("online", Boolean(status.llm?.available));
    if (!recording) {
      $("voiceHint").textContent = status.voice.configured
        ? ($("handsFree").checked ? "Waiting for “Hey Nox”…" : "Tap to speak")
        : "Voice model not configured";
    }
    $("voiceButton").disabled = !status.voice.configured;
  } catch {
    $("coreStatus").textContent = "CORE OFFLINE";
    $("coreStatus").classList.remove("online");
  }
}

refreshStatus();
setInterval(refreshStatus, 10000);
