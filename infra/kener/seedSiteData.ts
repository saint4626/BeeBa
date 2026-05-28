const origin = process.env.ORIGIN ?? "https://status.beeba.org";
const appOrigin = origin.includes("127.0.0.1") || origin.includes("localhost")
  ? "http://127.0.0.1:8088"
  : "https://beeba.org";

const seedSiteData = {
  title: "BeeBa Status",
  siteName: "BeeBa Status",
  siteURL: origin,
  home: "/",
  logo: "/logo.png",
  favicon: "/logo96.png",
  metaTags: [
    {
      key: "description",
      value: "Public service status for BeeBa.",
    },
    {
      key: "og:description",
      value: "Public service status for BeeBa.",
    },
    {
      key: "og:title",
      value: "BeeBa Status",
    },
    {
      key: "og:type",
      value: "website",
    },
    {
      key: "og:site_name",
      value: "BeeBa Status",
    },
    {
      key: "twitter:card",
      value: "summary",
    },
    {
      key: "twitter:title",
      value: "BeeBa Status",
    },
    {
      key: "twitter:description",
      value: "Public service status for BeeBa.",
    },
  ],
  nav: [
    { name: "BeeBa", iconURL: "", url: appOrigin },
    { name: "API Reference", iconURL: "", url: `${appOrigin}/api-reference` },
  ],
  hero: {
    title: "BeeBa service status",
    subtitle: "Availability for the catalog, API, search, and object storage.",
  },
  footerHTML: `<div class="container relative mt-4 max-w-[655px]">
  <div class="block items-center gap-4 px-8 md:flex-row md:gap-2 md:px-0 mx-auto">
    <p class="text-center text-xs leading-loose text-muted-foreground">
      BeeBa status page powered by <a href="https://kener.ing" target="_blank" rel="noreferrer" class="font-medium underline underline-offset-4 hover:text-accent-foreground">Kener</a>.
    </p>
  </div>
</div>`,
  i18n: {
    defaultLocale: "en",
    locales: [{ code: "en", name: "English", selected: true, disabled: false }],
  },
  pattern: "none",
  analytics: [],
  theme: "none",
  themeToggle: "YES",
  tzToggle: "YES",
  barStyle: "PARTIAL",
  barRoundness: "SHARP",
  summaryStyle: "CURRENT",
  colors: {
    UP: "#27c486",
    DOWN: "#ff5757",
    DEGRADED: "#f5c518",
    MAINTENANCE: "#45d6ff",
    ACCENT: "#f4f4f5",
    ACCENT_FOREGROUND: "#27c486",
  },
  colorsDark: {
    UP: "#27c486",
    DOWN: "#ff5757",
    DEGRADED: "#f5c518",
    MAINTENANCE: "#45d6ff",
    ACCENT: "#1a1a2e",
    ACCENT_FOREGROUND: "#27c486",
  },
  font: {
    cssSrc: "",
    family: "",
  },
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
