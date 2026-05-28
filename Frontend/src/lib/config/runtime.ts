import {
  PUBLIC_ADMIN_PAGE_LIMIT,
  PUBLIC_ADMIN_POLL_INTERVAL_MS,
  PUBLIC_CATALOG_PAGE_LIMIT,
  PUBLIC_HOME_FEATURED_LIMIT,
  PUBLIC_HOME_NEWEST_LIMIT,
  PUBLIC_HOME_SOURCE_LIMIT,
  PUBLIC_MAX_IMAGE_UPLOAD_BYTES,
  PUBLIC_OWNER_CONTENT_LIMIT,
  PUBLIC_PROFILE_ASSETS_LIMIT,
  PUBLIC_SOCIAL_COMMENT_LIMIT,
} from "astro:env/client";

export const BYTE_UNITS = {
  kib: 1024,
  mib: 1024 * 1024,
  gib: 1024 * 1024 * 1024,
} as const;

export const UPLOAD_LIMITS = {
  maxImageBytes: PUBLIC_MAX_IMAGE_UPLOAD_BYTES,
} as const;

export const PAGE_LIMITS = {
  admin: PUBLIC_ADMIN_PAGE_LIMIT,
  catalog: PUBLIC_CATALOG_PAGE_LIMIT,
  homeSource: PUBLIC_HOME_SOURCE_LIMIT,
  homeNewest: PUBLIC_HOME_NEWEST_LIMIT,
  homeFeatured: PUBLIC_HOME_FEATURED_LIMIT,
  ownerContent: PUBLIC_OWNER_CONTENT_LIMIT,
  profileAssets: PUBLIC_PROFILE_ASSETS_LIMIT,
  socialComments: PUBLIC_SOCIAL_COMMENT_LIMIT,
} as const;

export const ADMIN_TIMING = {
  pollIntervalMs: PUBLIC_ADMIN_POLL_INTERVAL_MS,
} as const;

export const HASH_DISPLAY = {
  prefixLength: 10,
  suffixLength: 10,
} as const;

export function megabytesFromBytes(bytes: number): number {
  return Math.round(bytes / BYTE_UNITS.mib);
}
