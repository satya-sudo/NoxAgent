const sleep = (milliseconds) =>
  new Promise((resolve) => setTimeout(resolve, milliseconds));

chrome.runtime.onMessage.addListener((action, _sender, sendResponse) => {
  executeAction(action)
    .then(sendResponse)
    .catch((error) => sendResponse({ ok: false, message: error.message || String(error) }));
  return true;
});

async function executeAction(action) {
  switch (action.type) {
    case "youtube_music.play":
      await playFirstResult();
      return { ok: true, message: `Playing ${action.query} on YouTube Music.` };
    case "youtube_music.pause":
      clickPlayerButton(["Pause"]);
      return { ok: true, message: "Music paused." };
    case "youtube_music.resume":
      clickPlayerButton(["Play"]);
      return { ok: true, message: "Resuming music." };
    case "youtube_music.next":
      clickPlayerButton(["Next"]);
      return { ok: true, message: "Skipping to the next song." };
    case "youtube_music.previous":
      clickPlayerButton(["Previous"]);
      return { ok: true, message: "Going back to the previous song." };
    case "youtube_music.volume":
      setVolume(action.value);
      return { ok: true, message: `Volume set to ${action.value} percent.` };
    case "youtube_music.now_playing":
      return { ok: true, message: nowPlaying() };
    default:
      throw new Error(`Unsupported YouTube Music action: ${action.type}`);
  }
}

async function playFirstResult() {
  const selectors = [
    "ytmusic-responsive-list-item-renderer > a.yt-simple-endpoint[href^='watch']",
    "ytmusic-shelf-renderer ytmusic-responsive-list-item-renderer a.yt-simple-endpoint[href^='watch']",
    "ytmusic-card-shelf-renderer a.yt-simple-endpoint[href^='watch']",
    "ytmusic-responsive-list-item-renderer #play-button",
  ];

  for (let attempt = 0; attempt < 20; attempt += 1) {
    for (const selector of selectors) {
      const target = document.querySelector(selector);
      if (target) {
        target.click();
        return;
      }
    }
    await sleep(500);
  }
  throw new Error("No playable YouTube Music search result was found");
}

function clickPlayerButton(labels) {
  const buttons = [...document.querySelectorAll("button, tp-yt-paper-icon-button")];
  const target = buttons.find((button) => {
    const label = (button.getAttribute("aria-label") || button.title || "").trim();
    return labels.some((expected) => label.toLowerCase().startsWith(expected.toLowerCase()));
  });
  if (!target) {
    throw new Error(`Could not find the ${labels[0].toLowerCase()} control`);
  }
  target.click();
}

function setVolume(percent) {
  const slider = document.querySelector("#volume-slider") ||
    document.querySelector("tp-yt-paper-slider[aria-label*='volume' i]") ||
    document.querySelector("input[type='range'][aria-label*='volume' i]");
  if (!slider) {
    throw new Error("Could not find the YouTube Music volume control");
  }

  const value = Math.max(0, Math.min(100, Number(percent)));
  if (slider instanceof HTMLInputElement) {
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value").set;
    setter.call(slider, String(value));
  } else {
    slider.value = value;
    slider.setAttribute("value", String(value));
  }
  slider.dispatchEvent(new Event("input", { bubbles: true, composed: true }));
  slider.dispatchEvent(new Event("change", { bubbles: true, composed: true }));
}

function nowPlaying() {
  const title = document.querySelector("ytmusic-player-bar .title")?.textContent?.trim();
  const byline = document.querySelector("ytmusic-player-bar .byline")?.textContent?.trim();
  if (!title) {
    return "Nothing is playing on YouTube Music.";
  }
  return byline ? `${title}, ${byline}.` : `${title}.`;
}
