(() => {
  const key = "htb.analytics-consent";
  const banner = document.querySelector("#consent");
  const analyticsURL = "https://analytics.ben-to.fr/script.js";
  const websiteID = "2dfe3633-9cfe-43b8-95e1-41c22169a33a";
  let loaded = false;
  function loadAnalytics() {
    if (loaded || document.querySelector('script[src="' + analyticsURL + '"]')) return;
    loaded = true;
    const script = document.createElement("script");
    script.src = analyticsURL;
    script.defer = true;
    script.dataset.websiteId = websiteID;
    document.head.append(script);
  }
  function applyConsent(value) {
    if (value === "accepted") loadAnalytics();
    if (banner) banner.hidden = value === "accepted" || value === "rejected";
  }
  const saved = localStorage.getItem(key);
  applyConsent(saved);
  document.querySelectorAll("[data-consent]").forEach((button) => button.addEventListener("click", () => {
    const choice = button.dataset.consent;
    localStorage.setItem(key, choice);
    applyConsent(choice);
  }));
  document.querySelectorAll("[data-open-consent]").forEach((button) => button.addEventListener("click", () => {
    if (banner) banner.hidden = false;
  }));
  document.querySelectorAll(".languages a").forEach((link) => link.addEventListener("click", () => {
    localStorage.setItem("htb.language", link.hreflang);
  }));
  const preferredLanguage = localStorage.getItem("htb.language");
  if (preferredLanguage && preferredLanguage !== document.documentElement.lang && !location.search.includes("lang=")) {
    location.replace(location.pathname + "?lang=" + preferredLanguage);
  }
  const simulatorTabs = Array.from(document.querySelectorAll("[data-simulator-tab]"));
  function selectSimulatorTab(tab) {
    simulatorTabs.forEach((candidate) => {
      const selected = candidate === tab;
      candidate.setAttribute("aria-selected", String(selected));
      candidate.tabIndex = selected ? 0 : -1;
      const panel = document.getElementById(candidate.dataset.simulatorTab);
      if (panel) panel.hidden = !selected;
    });
  }
  simulatorTabs.forEach((tab, index) => {
    tab.addEventListener("click", () => selectSimulatorTab(tab));
    tab.addEventListener("keydown", (event) => {
      if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
      event.preventDefault();
      const next = event.key === "Home" ? 0 : event.key === "End" ? simulatorTabs.length - 1 : (index + (event.key === "ArrowRight" ? 1 : -1) + simulatorTabs.length) % simulatorTabs.length;
      simulatorTabs[next].focus();
      selectSimulatorTab(simulatorTabs[next]);
    });
  });
})();
