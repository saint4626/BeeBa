import { contentMetaDescription } from "./content-url.ts";
import { serverMetaDescription } from "./server-url.ts";
import type { Locale } from "../i18n";
import type { PublicContentItem, PublicProfile, PublicServerItem } from "../api/types";

export type JsonLdNode = Record<string, unknown>;

interface BreadcrumbEntry {
  name: string;
  path: string;
}

export function globalJsonLd(site: URL): JsonLdNode[] {
  const siteURL = new URL("/", site).href;
  const logoURL = new URL("/brand/beeba-logo-512.png", site).href;
  return [
    {
      "@context": "https://schema.org",
      "@type": "Organization",
      name: ".BEEBA",
      url: siteURL,
      logo: logoURL,
    },
    {
      "@context": "https://schema.org",
      "@type": "WebSite",
      name: ".BEEBA",
      url: siteURL,
      potentialAction: {
        "@type": "SearchAction",
        target: `${new URL("/catalog", site).href}?q={search_term_string}`,
        "query-input": "required name=search_term_string",
      },
    },
  ];
}

export function breadcrumbJsonLd(site: URL, entries: BreadcrumbEntry[]): JsonLdNode {
  return {
    "@context": "https://schema.org",
    "@type": "BreadcrumbList",
    itemListElement: entries.map((entry, index) => ({
      "@type": "ListItem",
      position: index + 1,
      name: entry.name,
      item: new URL(entry.path, site).href,
    })),
  };
}

export function contentJsonLd(
  site: URL,
  item: PublicContentItem,
  locale: Locale,
  canonicalPath: string,
  imageURL?: string,
): JsonLdNode {
  const authorName = item.author.display_name || item.author.username;
  return removeEmptyValues({
    "@context": "https://schema.org",
    "@type": "CreativeWork",
    name: item.title,
    description: contentMetaDescription(item, locale),
    url: new URL(canonicalPath, site).href,
    image: imageURL ? new URL(imageURL, site).href : undefined,
    datePublished: item.published_at,
    dateModified: item.updated_at,
    genre: item.category.name,
    author: {
      "@type": "Person",
      name: authorName,
      alternateName: item.author.username,
      url: new URL(`/users/${encodeURIComponent(item.author.username)}`, site).href,
    },
    interactionStatistic: [
      {
        "@type": "InteractionCounter",
        interactionType: { "@type": "DownloadAction" },
        userInteractionCount: item.downloads_count,
      },
      {
        "@type": "InteractionCounter",
        interactionType: { "@type": "LikeAction" },
        userInteractionCount: item.likes_count,
      },
    ],
  });
}

export function profileJsonLd(site: URL, profile: PublicProfile, canonicalPath: string, imageURL?: string): JsonLdNode {
  const displayName = profile.display_name || profile.username;
  return removeEmptyValues({
    "@context": "https://schema.org",
    "@type": "ProfilePage",
    url: new URL(canonicalPath, site).href,
    dateModified: profile.updated_at,
    mainEntity: {
      "@type": "Person",
      name: displayName,
      alternateName: profile.username,
      image: imageURL ? new URL(imageURL, site).href : undefined,
    },
  });
}

export function serverCatalogJsonLd(site: URL, canonicalPath: string, entries: BreadcrumbEntry[]): JsonLdNode[] {
  return [
    breadcrumbJsonLd(site, entries),
    {
      "@context": "https://schema.org",
      "@type": "CollectionPage",
      name: "BeeBa Basis VR Server Catalog",
      url: new URL(canonicalPath, site).href,
      isPartOf: {
        "@type": "WebSite",
        name: ".BEEBA",
        url: new URL("/", site).href,
      },
    },
  ];
}

export function serverJsonLd(site: URL, server: PublicServerItem, locale: Locale, canonicalPath: string): JsonLdNode {
  const ownerName = server.owner.display_name || server.owner.username;
  return removeEmptyValues({
    "@context": "https://schema.org",
    "@type": "CreativeWork",
    name: server.name,
    description: serverMetaDescription(server, locale),
    url: new URL(canonicalPath, site).href,
    datePublished: server.published_at,
    dateModified: server.updated_at,
    genre: "Basis VR server",
    isAccessibleForFree: true,
    author: {
      "@type": "Person",
      name: ownerName,
      alternateName: server.owner.username,
      url: new URL(`/users/${encodeURIComponent(server.owner.username)}`, site).href,
    },
    interactionStatistic: typeof server.check.online_players === "number" ? [
      {
        "@type": "InteractionCounter",
        interactionType: { "@type": "JoinAction" },
        userInteractionCount: server.check.online_players,
      },
    ] : undefined,
  });
}

export function serializeJsonLd(node: JsonLdNode): string {
  return JSON.stringify(node).replaceAll("<", "\\u003c");
}

function removeEmptyValues<T extends JsonLdNode>(node: T): T {
  return Object.fromEntries(Object.entries(node).filter(([, value]) => value !== undefined && value !== null && value !== "")) as T;
}
