const NOX_API = "http://127.0.0.1:7080";
const YOUTUBE_MUSIC = "https://music.youtube.com/";
const KEEPALIVE_URL = "ws://127.0.0.1:7080/v1/browser/keepalive";

let polling = false;
let keepaliveSocket;
let reconnectTimer;

const delay = (milliseconds) =>
  new Promise((resolve) => setTimeout(resolve, milliseconds));

async function poll() {
  if (polling) {
    return;
  }
  polling = true;
  try {
    for (;;) {
      try {
      const response = await fetch(`${NOX_API}/v1/browser/actions/next?wait=20s`);
      if (response.status === 204) {
        continue;
      }
      if (!response.ok) {
        throw new Error(`Nox returned HTTP ${response.status}`);
      }

      const action = await response.json();
      let result;
      try {
        result = await execute(action);
      } catch (error) {
        result = { ok: false, message: error.message || String(error) };
      }

      await fetch(`${NOX_API}/v1/browser/actions/${action.id}/complete`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(result),
      });
      } catch (error) {
        console.debug("Nox daemon is unavailable", error);
        await delay(2000);
      }
    }
  } finally {
    polling = false;
  }
}

function connectKeepalive() {
  if (keepaliveSocket &&
      (keepaliveSocket.readyState === WebSocket.OPEN ||
       keepaliveSocket.readyState === WebSocket.CONNECTING)) {
    return;
  }
  clearTimeout(reconnectTimer);
  keepaliveSocket = new WebSocket(KEEPALIVE_URL);
  keepaliveSocket.onopen = () => poll();
  keepaliveSocket.onmessage = () => poll();
  keepaliveSocket.onerror = () => keepaliveSocket.close();
  keepaliveSocket.onclose = () => {
    reconnectTimer = setTimeout(connectKeepalive, 2000);
  };
}

chrome.alarms.create("nox-reconnect", { periodInMinutes: 0.5 });
chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === "nox-reconnect") {
    connectKeepalive();
    poll();
  }
});

async function execute(action) {
  if (action.type === "browser.google_search") {
    const url = `https://www.google.com/search?q=${encodeURIComponent(action.query)}`;
    const tab = await chrome.tabs.create({ url, active: true });
    await waitForTab(tab.id);
    return sendToTab(tab.id, action, "Google search bridge did not respond");
  }

  if (!action.type.startsWith("youtube_music.")) {
    throw new Error(`Unsupported browser action: ${action.type}`);
  }

  const tab = await getYouTubeMusicTab();
  await chrome.tabs.update(tab.id, { active: true });

  if (action.type === "youtube_music.play") {
    const url = `${YOUTUBE_MUSIC}search?q=${encodeURIComponent(action.query)}`;
    await chrome.tabs.update(tab.id, { url, active: true });
    await waitForTab(tab.id);
  }

  return sendToTab(tab.id, action, "YouTube Music bridge did not respond");
}

async function getYouTubeMusicTab() {
  const tabs = await chrome.tabs.query({ url: "https://music.youtube.com/*" });
  if (tabs.length > 0) {
    return tabs[0];
  }
  const tab = await chrome.tabs.create({ url: YOUTUBE_MUSIC, active: true });
  await waitForTab(tab.id);
  return tab;
}

async function waitForTab(tabId, timeoutMilliseconds = 15000) {
  const current = await chrome.tabs.get(tabId);
  if (current.status === "complete") {
    return;
  }

  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => {
      chrome.tabs.onUpdated.removeListener(listener);
      reject(new Error("YouTube Music took too long to load"));
    }, timeoutMilliseconds);

    function listener(updatedTabId, changeInfo) {
      if (updatedTabId === tabId && changeInfo.status === "complete") {
        clearTimeout(timeout);
        chrome.tabs.onUpdated.removeListener(listener);
        resolve();
      }
    }

    chrome.tabs.onUpdated.addListener(listener);
  });
}

async function sendToTab(tabId, action, failureMessage) {
  let lastError;
  for (let attempt = 0; attempt < 10; attempt += 1) {
    try {
      return await chrome.tabs.sendMessage(tabId, action);
    } catch (error) {
      lastError = error;
      await delay(500);
    }
  }
  throw lastError || new Error(failureMessage);
}

connectKeepalive();
poll();
