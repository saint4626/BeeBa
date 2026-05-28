const origin = process.env.ORIGIN ?? "https://status.beeba.org";
const appOrigin = origin.includes("127.0.0.1") || origin.includes("localhost")
  ? "http://127.0.0.1:8088"
  : "https://beeba.org";

const seedSiteData = {
  title: ".BEEBA Status",
  siteName: ".BEEBA Status",
  siteURL: origin,
  home: "/",
  logo: "/beeba-logo.webp",
  favicon: "/beeba-favicon.png",
  metaTags: [
    {
      key: "description",
      value: "Public status page for BeeBa services, API, search, and object storage.",
    },
    {
      key: "og:description",
      value: "Public status page for BeeBa services, API, search, and object storage.",
    },
    { key: "og:image", value: `${origin}/beeba-logo.webp` },
    {
      key: "og:title",
      value: ".BEEBA Status",
    },
    {
      key: "og:type",
      value: "website",
    },
    {
      key: "og:site_name",
      value: ".BEEBA Status",
    },
    {
      key: "twitter:card",
      value: "summary_large_image",
    },
    {
      key: "twitter:image",
      value: `${origin}/beeba-logo.webp`,
    },
    {
      key: "twitter:title",
      value: ".BEEBA Status",
    },
    {
      key: "twitter:description",
      value: "Public status page for BeeBa services, API, search, and object storage.",
    },
  ],
  nav: [
    { name: ".BEEBA", iconURL: "", url: appOrigin },
    { name: "API Reference", iconURL: "", url: `${appOrigin}/api-reference` },
  ],
  hero: {
    title: ".BEEBA service status",
    subtitle: "Live availability for the catalog, API, search, and object storage.",
  },
  footerHTML: `<div class="beeba-status-footer">
    <p>
      <strong><span>.BEE</span>BA</strong> Status · powered by <a href="https://kener.ing" target="_blank" rel="noreferrer">Kener</a>
    </p>
</div>`,
  i18n: {
    defaultLocale: "en",
    locales: [{ code: "en", name: "English", selected: true, disabled: false }],
  },
  pattern: "none",
  analytics: [],
  theme: "dark",
  themeToggle: "NO",
  tzToggle: "YES",
  barStyle: "PARTIAL",
  barRoundness: "SHARP",
  summaryStyle: "CURRENT",
  colors: {
    UP: "#ffd700",
    DOWN: "#ff5757",
    DEGRADED: "#ffa800",
    MAINTENANCE: "#ffffff",
    ACCENT: "#ffd700",
    ACCENT_FOREGROUND: "#000000",
  },
  colorsDark: {
    UP: "#ffd700",
    DOWN: "#ff5757",
    DEGRADED: "#ffa800",
    MAINTENANCE: "#ffffff",
    ACCENT: "#ffd700",
    ACCENT_FOREGROUND: "#000000",
  },
  font: {
    cssSrc: "",
    family: "Varela",
  },
  customCSS: `
@font-face {
  font-family: "Varela";
  src: url("/varela-v17-latin.ttf") format("truetype");
  font-weight: 400;
  font-style: normal;
  font-display: swap;
}

:root,
.dark {
  color-scheme: dark;
}

html,
body {
  background: #0f0f0f !important;
  color: #ffffff;
  font-family: "Varela", Arial, sans-serif !important;
}

body {
  letter-spacing: 0;
}

body::before {
  content: "";
  position: fixed;
  inset: 0;
  pointer-events: none;
  background:
    linear-gradient(180deg, rgba(255, 215, 0, 0.08), transparent 220px),
    radial-gradient(circle at 50% 0, rgba(255, 168, 0, 0.1), transparent 34rem);
  z-index: -1;
}

main {
  background: #0f0f0f !important;
}

main > .mx-auto {
  max-width: 1180px !important;
  padding-left: 1rem;
  padding-right: 1rem;
}

nav,
header {
  background: #ffd700 !important;
  color: #000000 !important;
  border-color: rgba(0, 0, 0, 0.18) !important;
}

nav a,
header a,
nav button,
header button {
  color: #000000 !important;
}

nav img,
header img {
  object-fit: contain;
}

.kener-public > .fixed:first-child > div > div {
  border-radius: 6px !important;
  gap: 0.5rem;
  padding: 0.35rem !important;
}

.kener-public > .fixed:first-child nav ul {
  gap: 0.5rem !important;
}

.kener-public > .fixed:first-child nav,
.kener-public > .fixed:first-child nav > div,
.kener-public > .fixed:first-child nav ul,
.kener-public > .fixed:first-child [data-navigation-menu-root],
.kener-public > .fixed:first-child [data-navigation-menu-list] {
  background: transparent !important;
  border-color: transparent !important;
  box-shadow: none !important;
}

.kener-public > .fixed:first-child a,
.kener-public > .fixed:first-child button {
  min-height: 2.25rem !important;
  border-radius: 6px !important;
  padding-left: 0.9rem !important;
  padding-right: 0.9rem !important;
}

.kener-public > .fixed:first-child a:first-child {
  padding-left: 0.65rem !important;
}

.kener-public > .fixed:first-child nav a {
  background: #ffd700 !important;
  border-color: rgba(0, 0, 0, 0.18) !important;
}

.kener-public > .fixed:first-child nav a:hover,
.kener-public > .fixed:first-child button:hover {
  background: #ffa800 !important;
}

.kener-public > .fixed:first-child img {
  border-radius: 4px !important;
}

.bg-up,
[class*="bg-up"] {
  border-radius: 9999px !important;
  aspect-ratio: 1 / 1;
}

a {
  color: #ffd700;
}

.beeba-status-footer {
  margin: 2.5rem auto 1rem;
  max-width: 1180px;
  width: 100%;
  border: 1px solid rgba(0, 0, 0, 0.28);
  border-radius: 6px;
  padding: 0.8rem 1rem;
  color: #000000;
  background: #ffd700;
  text-align: center;
}

.beeba-status-footer p {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.45rem;
  margin: 0;
  font-size: 0.82rem;
}

.beeba-status-footer span,
.beeba-status-footer a {
  color: #000000;
}

.beeba-status-footer a {
  display: inline-flex;
  align-items: center;
  min-height: 1.75rem;
  border-radius: 6px;
  padding: 0.2rem 0.55rem;
  text-decoration: underline;
  text-underline-offset: 4px;
}

.card,
[class*="card"] {
  border-radius: 6px !important;
}

.kener-public span.bg-up,
.kener-public span[class*="bg-up"] {
  width: 1rem !important;
  height: 1rem !important;
  min-width: 1rem !important;
  border-radius: 9999px !important;
}

button,
[role="button"],
a {
  transition: color 160ms ease, background-color 160ms ease, border-color 160ms ease, opacity 160ms ease;
}
`,
  socialPreviewImage: "/beeba-logo.webp",
  categories: [{ name: "BeeBa", description: "Core BeeBa production services", isHidden: false }],
  homeIncidentCount: 5,
  homeIncidentStartTimeWithin: 30,
  homeDataMaxDays: {
    desktop: {
      maxDays: 90,
      selectableDays: [1, 7, 14, 30, 60, 90],
    },
    mobile: {
      maxDays: 90,
      selectableDays: [1, 7, 14, 30, 60, 90],
    },
  },
  kenerTheme: "dark",
  showSiteStatus: "YES",
  subscriptionsSettings: {
    enable: false,
    methods: {
      emails: {
        incidents: false,
        maintenances: false,
      },
    },
  },
  subMenuOptions: {
    showShareBadgeMonitor: true,
    showShareEmbedMonitor: true,
  },
  dataRetentionPolicy: {
    enabled: true,
    retentionDays: 90,
  },
  eventDisplaySettings: {
    incidents: {
      enabled: true,
      ongoing: { show: true },
      resolved: { show: true, maxCount: 5, daysInPast: 7 },
    },
    maintenances: {
      enabled: true,
      ongoing: {
        show: true,
      },
      past: { show: true, maxCount: 5, daysInPast: 7 },
      upcoming: { show: true, maxCount: 5, daysInFuture: 7 },
    },
  },
  globalPageVisibilitySettings: {
    showSwitcher: false,
    forceExclusivity: false,
  },
  dateAndTimeFormat: {
    datePlusTime: "PPp",
    dateOnly: "PP",
    timeOnly: "p",
  },
  sitemap: {
    mode: "off",
    urls: [],
  },
  globalMaintenanceNotificationSettings: {
    event_types: {
      created: false,
      reminder: true,
      started: true,
      ended: true,
    },
    reminder_buffer_hours: 1,
  },
};

export default seedSiteData;
