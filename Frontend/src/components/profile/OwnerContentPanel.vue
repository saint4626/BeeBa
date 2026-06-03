<script setup lang="ts">
import { useOwnerStore } from "../../stores/owner.store";
import { getPublicAPIBaseURL } from "../../lib/api/client";
import { publicMediaURL } from "../../lib/api/media-url";
import { BYTE_UNITS } from "../../lib/config/runtime";
import type { OwnerContentItem, UploadedImage } from "../../lib/api/types";

const owner = useOwnerStore();
const publicAPIBaseURL = getPublicAPIBaseURL();

function primaryImage(item: OwnerContentItem): UploadedImage | null {
  const images = owner.imagesByContent[item.id] ?? [];
  const processed = images.filter((image) => image.processing_status === "processed");
  return processed.find((image) => image.is_primary) ?? processed[0] ?? images.find((image) => image.is_primary) ?? images[0] ?? null;
}

function ownerImageURL(image: UploadedImage) {
  return image.url.replace("/api/v1/media/", "/api/v1/me/media/");
}

function ownerAvatarURL() {
  return publicMediaURL(publicAPIBaseURL, owner.user?.avatar_image_id);
}

function canRenderImage(item: OwnerContentItem) {
  return primaryImage(item)?.processing_status === "processed";
}

function statusLabel(value: string) {
  return value.replaceAll("_", " ");
}

function formatBytes(value?: number) {
  if (!value) return "No file size";
  if (value < BYTE_UNITS.kib) return `${value} B`;
  if (value < BYTE_UNITS.mib) return `${(value / BYTE_UNITS.kib).toFixed(1)} KB`;
  if (value < BYTE_UNITS.gib) return `${(value / BYTE_UNITS.mib).toFixed(1)} MB`;
  return `${(value / BYTE_UNITS.gib).toFixed(2)} GB`;
}

function categoryClass(item: OwnerContentItem) {
  const slug = item.category.slug || item.category.name.toLowerCase();
  if (slug.includes("avatar")) return "asset-card--avatar";
  if (slug.includes("prop")) return "asset-card--prop";
  if (slug.includes("prefab")) return "asset-card--prefab";
  return "asset-card--world";
}

function initials(value: string) {
  return value.trim().slice(0, 2).toUpperCase();
}
</script>

<template>
  <section class="panel profile-content-panel" aria-labelledby="profile-content-title">
    <div class="panel__head panel__head--row">
      <div>
        <p class="eyebrow">Owner library</p>
        <h2 id="profile-content-title">Packages</h2>
      </div>
      <button class="button button--secondary" type="button" :disabled="owner.loading || !owner.isAuthenticated" @click="owner.refreshLibrary">
        Refresh
      </button>
    </div>

    <div v-if="!owner.isAuthenticated" class="message message--warning">
      Register or login before managing content. Upload and media endpoints require a bearer session.
    </div>

    <div v-else-if="owner.items.length === 0" class="empty-state">
      <span class="empty-state__badge">No owner items</span>
      <div>
        <h2>Your uploads will appear here.</h2>
        <p>After upload, worker scan will move valid files toward moderation.</p>
      </div>
    </div>

    <div v-else class="asset-masonry owner-card-grid" aria-label="Owner packages">
      <article
        v-for="item in owner.items"
        :key="item.id"
        class="asset-card owner-card"
        :class="[categoryClass(item), { 'owner-card--active': item.id === owner.selectedContentID }]"
      >
        <button
          class="owner-card__select"
          type="button"
          :aria-pressed="item.id === owner.selectedContentID"
          @click="owner.selectContent(item.id)"
        >
          <img
            v-if="canRenderImage(item)"
            class="asset-card__image"
            :src="ownerImageURL(primaryImage(item) as UploadedImage)"
            :alt="primaryImage(item)?.alt_text || item.title"
            loading="lazy"
          />
          <span v-else class="asset-card__placeholder">
            <span>{{ initials(item.category.name) }}</span>
          </span>
          <span class="asset-card__shade" aria-hidden="true"></span>

          <span class="asset-card__top">
            <span class="asset-card__chip">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <circle cx="12" cy="12" r="10"></circle>
                <path d="M2 12h20"></path>
                <path d="M12 2a15.3 15.3 0 0 1 0 20"></path>
                <path d="M12 2a15.3 15.3 0 0 0 0 20"></path>
              </svg>
              <span>{{ item.category.name }}</span>
            </span>
            <span>.bee</span>
          </span>

          <span class="asset-card__body">
            <span class="asset-card__author">
              <span class="asset-card__avatar" aria-hidden="true">
                <img
                  v-if="owner.user?.avatar_image_id"
                  :src="ownerAvatarURL()"
                  alt=""
                  width="24"
                  height="24"
                  loading="lazy"
                  decoding="async"
                />
                <template v-else>{{ initials(owner.user?.username || item.title) }}</template>
              </span>
              <span>{{ owner.user?.display_name || owner.user?.username || "Owner" }}</span>
            </span>
            <strong class="asset-card__title">{{ item.title }}</strong>
            <span class="asset-card__metrics">
              <span>{{ item.visibility }}</span>
              <span>{{ formatBytes(item.file?.file_size) }}</span>
              <span>{{ statusLabel(item.status) }}</span>
              <span>{{ statusLabel(item.file?.scan_status || "no file") }}</span>
              <span v-if="item.nsfw">NSFW</span>
            </span>
          </span>
        </button>
      </article>
    </div>
  </section>
</template>
