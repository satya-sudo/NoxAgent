chrome.runtime.onMessage.addListener((action, _sender, sendResponse) => {
  if (action.type !== "browser.google_search") {
    return false;
  }
  sendResponse({ ok: true, message: extractGoogleAnswer(action.query) });
  return false;
});

function extractGoogleAnswer(query) {
  const weather = extractWeather();
  if (weather) {
    return weather;
  }

  const directAnswer = firstText([
    "[data-attrid='wa:/description']",
    "[data-attrid='description']",
    ".hgKElc",
    "[data-md='83']",
  ]);
  if (directAnswer) {
    return directAnswer;
  }

  const featured = document.querySelector("#search blockquote")?.textContent?.trim();
  if (featured) {
    return featured;
  }

  const firstHeading = document.querySelector("#search a h3");
  if (firstHeading) {
    const result = firstHeading.closest("div[data-snhf]") ||
      firstHeading.closest("div.MjjYud") ||
      firstHeading.parentElement?.parentElement?.parentElement;
    const snippet = result?.querySelector("[data-sncf], .VwiC3b")?.textContent?.trim();
    if (snippet) {
      return `${firstHeading.textContent.trim()}: ${snippet}`;
    }
  }

  return `I opened Google results for ${query}, but could not find a concise answer on the page.`;
}

function extractWeather() {
  let temperature = document.querySelector("#wob_tm")?.textContent?.trim();
  if (!temperature) {
    return extractSemanticWeather();
  }
  if (!temperature) {
    return "";
  }
  const location = document.querySelector("#wob_loc")?.textContent?.trim();
  const condition = document.querySelector("#wob_dc")?.textContent?.trim();
  const precipitation = document.querySelector("#wob_pp")?.textContent?.trim();
  const humidity = document.querySelector("#wob_hm")?.textContent?.trim();
  const wind = document.querySelector("#wob_ws")?.textContent?.trim();
  const unit = document.querySelector("#wob_ttm")?.textContent?.includes("F") ? "F" : "C";

  const details = [
    location,
    `${temperature} degrees ${unit}`,
    condition,
    precipitation ? `precipitation ${precipitation}` : "",
    humidity ? `humidity ${humidity}` : "",
    wind ? `wind ${wind}` : "",
  ].filter(Boolean);
  return details.join(", ") + ".";
}

function extractSemanticWeather() {
  const heading = [...document.querySelectorAll("h2")].find(
    (element) => element.textContent?.trim() === "Weather result",
  );
  if (!heading) {
    return "";
  }
  const card = heading.closest(".MjjYud") || heading.parentElement;
  if (!card) {
    return "";
  }

  const documentHeadings = [...document.querySelectorAll("h2, [role='heading']")];
  const weatherIndex = documentHeadings.indexOf(heading);
  const location = documentHeadings
    .slice(0, weatherIndex)
    .reverse()
    .find((element) => element.getAttribute("aria-level") === "2")
    ?.textContent?.trim();
  const temperatureLabel = document.querySelector("text[aria-label*='Celsius']")
    ?.getAttribute("aria-label") || "";
  const temperature = temperatureLabel.match(/\d+/)?.[0];
  if (!temperature) {
    return "";
  }

  const condition = card.querySelector("img[alt]")?.getAttribute("alt")?.trim();
  const precipitation = shortestText(card, "Precipitation:");
  const humidity = shortestText(card, "Humidity:");
  const windText = shortestText(card, "Wind:");
  const wind = windText.match(/Wind:\s*[\d.]+\s*(?:km\/h|mph)/)?.[0] || windText;
  return [location, `${temperature} degrees C`, condition, precipitation, humidity, wind]
    .filter(Boolean)
    .join(", ") + ".";
}

function shortestText(root, prefix) {
  return [...root.querySelectorAll("div, span")]
    .map((element) => element.textContent?.trim() || "")
    .filter((text) => text.startsWith(prefix) && text.length < 80)
    .sort((left, right) => left.length - right.length)[0] || "";
}

function firstText(selectors) {
  for (const selector of selectors) {
    const text = document.querySelector(selector)?.textContent?.trim();
    if (text) {
      return text;
    }
  }
  return "";
}
