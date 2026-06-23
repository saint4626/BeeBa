// @ts-check
import { defineConfig, envField } from 'astro/config';
import node from '@astrojs/node';
import sitemap from '@astrojs/sitemap';
import vue from '@astrojs/vue';

const chunkSizeWarningLimitKiB = 650;
const siteURL = process.env.PUBLIC_SITE_URL ?? 'http://localhost:4321';
const absoluteSiteURL = siteURL.endsWith('/') ? siteURL : `${siteURL}/`;
const contentSitemapURL = new URL('content-sitemap.xml', absoluteSiteURL).href;
const catalogCategorySitemapPaths = [
  '/catalog/worlds',
  '/catalog/avatars',
  '/catalog/props',
  '/catalog/prefabs',
  '/ru/catalog/worlds',
  '/ru/catalog/avatars',
  '/ru/catalog/props',
  '/ru/catalog/prefabs',
];
const sitemapCustomPages = catalogCategorySitemapPaths.map((path) => new URL(path, absoluteSiteURL).href);
const siteHostname = new URL(absoluteSiteURL).hostname;
const localHostnames = new Set(['localhost', '127.0.0.1', '::1']);
const connectSrc = [
  "connect-src 'self'",
  ...(localHostnames.has(siteHostname) ? ['http://localhost:*', 'http://127.0.0.1:*'] : []),
  'https://challenges.cloudflare.com',
].join(' ');
const sitemapExcludedPathPrefixes = [
  '/admin',
  '/404',
  '/content-sitemap.xml',
  '/confirm-email-change',
  '/confirm-password-change',
  '/login',
  '/profile',
  '/upload',
  '/verify-email',
];
const isSitemapPageAllowed = (page) => {
  const path = new URL(page).pathname.replace(/^\/ru(?=\/|$)/, '') || '/';
  return !sitemapExcludedPathPrefixes.some((prefix) => path === prefix || path.startsWith(`${prefix}/`));
};
const addDefaultSitemapAlternate = (item) => {
  const links = item.links ?? [];
  const defaultLink = links.find((link) => link.lang === 'en');
  if (!defaultLink || links.some((link) => link.lang === 'x-default')) {
    return item;
  }
  return {
    ...item,
    links: [...links, { url: defaultLink.url, lang: 'x-default' }],
  };
};

// https://astro.build/config
export default defineConfig({
  site: siteURL,
  trailingSlash: 'never',
  output: 'server',
  env: {
    schema: {
      PUBLIC_API_BASE_URL: envField.string({ context: 'client', access: 'public', default: '/api/v1' }),
      PUBLIC_STATUS_URL: envField.string({ context: 'client', access: 'public', default: 'http://127.0.0.1:8089' }),
      PUBLIC_SITE_URL: envField.string({ context: 'client', access: 'public', default: 'http://localhost:4321' }),
      PUBLIC_MAX_IMAGE_UPLOAD_BYTES: envField.number({ context: 'client', access: 'public', default: 8_388_608 }),
      PUBLIC_CATALOG_PAGE_LIMIT: envField.number({ context: 'client', access: 'public', default: 24 }),
      PUBLIC_HOME_SOURCE_LIMIT: envField.number({ context: 'client', access: 'public', default: 10 }),
      PUBLIC_HOME_NEWEST_LIMIT: envField.number({ context: 'client', access: 'public', default: 8 }),
      PUBLIC_HOME_FEATURED_LIMIT: envField.number({ context: 'client', access: 'public', default: 12 }),
      PUBLIC_PROFILE_ASSETS_LIMIT: envField.number({ context: 'client', access: 'public', default: 12 }),
      PUBLIC_OWNER_CONTENT_LIMIT: envField.number({ context: 'client', access: 'public', default: 12 }),
      PUBLIC_OWNER_SYNC_INTERVAL_MS: envField.number({ context: 'client', access: 'public', default: 10_000 }),
      PUBLIC_OWNER_PROCESSING_SYNC_INTERVAL_MS: envField.number({ context: 'client', access: 'public', default: 1_500 }),
      PUBLIC_SOCIAL_COMMENT_LIMIT: envField.number({ context: 'client', access: 'public', default: 24 }),
      PUBLIC_ADMIN_PAGE_LIMIT: envField.number({ context: 'client', access: 'public', default: 50 }),
      PUBLIC_ADMIN_POLL_INTERVAL_MS: envField.number({ context: 'client', access: 'public', default: 15_000 }),
      PUBLIC_TURNSTILE_SITE_KEY: envField.string({ context: 'client', access: 'public', default: '' }),
    },
  },
  build: {
    inlineStylesheets: 'always',
  },
  vite: {
    build: {
      modulePreload: false,
      chunkSizeWarningLimit: chunkSizeWarningLimitKiB,
      rollupOptions: {
        output: {
          manualChunks(id) {
            const normalized = id.replace(/\\/g, '/');
            if (normalized.includes('vite/preload-helper')) {
              return 'vite-preload-helper';
            }
            if (normalized.includes('commonjsHelpers')) {
              return 'commonjs-helpers';
            }
            if (!normalized.includes('node_modules')) return undefined;
            if (
              normalized.includes('/node_modules/vue/')
              || normalized.includes('/node_modules/@vue/')
              || normalized.includes('/node_modules/@vueuse/')
            ) {
              return 'vue-runtime';
            }
            if (normalized.includes('/node_modules/pinia/')) {
              return 'pinia';
            }
            if (normalized.includes('/node_modules/@codemirror/')) {
              const match = normalized.match(/\/node_modules\/(@codemirror\/[^/]+)/);
              return match ? match[1].replace('@', '').replace('/', '-') : 'codemirror';
            }
            if (normalized.includes('/node_modules/@lezer/')) {
              const match = normalized.match(/\/node_modules\/(@lezer\/[^/]+)/);
              return match ? match[1].replace('@', '').replace('/', '-') : 'lezer';
            }
            if (normalized.includes('/node_modules/@shikijs/') || normalized.includes('/node_modules/shiki/')) {
              const match = normalized.match(/\/node_modules\/(@shikijs\/[^/]+|shiki)/);
              return match ? match[1].replace('@', '').replace('/', '-') : 'shiki';
            }
            if (
              normalized.includes('/node_modules/@scalar/code-highlight/')
              || normalized.includes('/node_modules/highlight.js/')
              || normalized.includes('/node_modules/unified/')
              || normalized.includes('/node_modules/rehype')
              || normalized.includes('/node_modules/remark-')
              || normalized.includes('/node_modules/hast-')
              || normalized.includes('/node_modules/mdast-')
              || normalized.includes('/node_modules/micromark')
            ) {
              return 'markdown-processing';
            }
            const scalarCoreMatch = normalized.match(/\/node_modules\/(@scalar\/(?:api-reference|api-client|agent-chat))/);
            if (scalarCoreMatch) {
              return 'scalar-api-reference-core';
            }
            const scalarMatch = normalized.match(/\/node_modules\/(@scalar\/[^/]+)/);
            if (scalarMatch) {
              return scalarMatch[1].replace('@', '').replace('/', '-');
            }
            return undefined;
          },
        },
      },
    },
  },
  markdown: {
    syntaxHighlight: 'prism',
  },
  security: {
    csp: {
      scriptDirective: {
        resources: ["'self'", "https://challenges.cloudflare.com"],
      },
      directives: [
        "default-src 'self'",
        "base-uri 'self'",
        "object-src 'none'",
        "img-src 'self' data: blob:",
        "font-src 'self'",
        "frame-src https://challenges.cloudflare.com",
        connectSrc,
      ],
    },
  },
  devToolbar: {
    enabled: false,
  },
  adapter: node({
    mode: 'standalone',
  }),
  i18n: {
    locales: ['en', 'ru'],
    defaultLocale: 'en',
    routing: {
      prefixDefaultLocale: false,
    },
  },
  integrations: [
    vue({ appEntrypoint: '/src/vue-app' }),
    sitemap({
      customPages: sitemapCustomPages,
      customSitemaps: [contentSitemapURL],
      serialize: addDefaultSitemapAlternate,
      i18n: {
        defaultLocale: 'en',
        locales: {
          en: 'en',
          ru: 'ru',
        },
      },
      filter: isSitemapPageAllowed,
    }),
  ],
  server: {
    host: true,
    allowedHosts: ['frontend', 'caddy'],
  },
});
