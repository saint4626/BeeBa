<script setup lang="ts">
import { defineComponent, h, type PropType } from "vue";
import { KeyRound, Link, Trash2 } from "lucide";
import { useOwnerStore } from "../../stores/owner.store";
import { getPublicAPIBaseURL } from "../../lib/api/client";
import { BYTE_UNITS } from "../../lib/config/runtime";
import { showToast } from "../../lib/ui/toast";
import type { OwnerContentItem, OwnerModerationFeedback, UploadedImage } from "../../lib/api/types";

const owner = useOwnerStore();
type IconNode = Array<[string, Record<string, string>]>;
const LinkIcon = Link as IconNode;
const KeyRoundIcon = KeyRound as IconNode;
const TrashIcon = Trash2 as IconNode;

const Icon = defineComponent({
  name: "LucideInlineIcon",
  props: {
    node: { type: Array as PropType<IconNode>, required: true },
    size: { type: Number, default: 15 },
  },
  setup(props) {
    return () =>
      h(
        "svg",
        {
          width: props.size,
          height: props.size,
          viewBox: "0 0 24 24",
          fill: "none",
          stroke: "currentColor",
          "stroke-width": "2",
          "stroke-linecap": "round",
          "stroke-linejoin": "round",
          "aria-hidden": "true",
          focusable: "false",
        },
        props.node.map(([tag, attrs]) => h(tag, attrs)),
      );
  },
});

function moderationLabel(moderation: OwnerModerationFeedback) {
  return moderation.action === "hide" ? "Hidden" : "Rejected";
}

function primaryImage(item: OwnerContentItem): UploadedImage | null {
  const images = owner.imagesByContent[item.id] ?? [];
  return images.find((image) => image.is_primary) ?? images[0] ?? null;
}

function ownerImageURL(image: UploadedImage) {
  return image.url.replace("/api/v1/media/", "/api/v1/me/media/");
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

function canCopyOwnerDownload(item: OwnerContentItem) {
  return item.status === "published" && item.file?.scan_status === "clean";
}

function ownerDownloadURL(item: OwnerContentItem) {
  return new URL(`${getPublicAPIBaseURL()}/me/content/${item.id}/download`, window.location.href).href;
}

async function copyText(value: string, label: string) {
  try {
    await navigator.clipboard.writeText(value);
    showToast(label, "success");
  } catch {
    showToast("Could not copy to clipboard", "error");
  }
}

async function copyOwnerDownload(item: OwnerContentItem) {
  if (!canCopyOwnerDownload(item)) {
    showToast("Download link is available after clean processing.", "info");
    return;
  }
  await copyText(ownerDownloadURL(item), "Owner download link copied");
}

async function copyPassword(item: OwnerContentItem) {
  if (!item.unlock_password) {
    showToast("No Basis password is stored for this package.", "info");
    return;
  }
  await copyText(item.unlock_password, "Basis password copied");
}

async function deleteItem(item: OwnerContentItem) {
  if (!window.confirm(`Delete "${item.title}"? This removes it from your library and the public catalog.`)) return;
  await owner.deleteContent(item.id);
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

    <div v-else class="owner-list owner-list--select">
      <article
        v-for="item in owner.items"
        :key="item.id"
        class="owner-item owner-item--button"
        :class="{ active: item.id === owner.selectedContentID }"
      >
        <button class="owner-item__select" type="button" @click="owner.selectContent(item.id)">
          <span v-if="canRenderImage(item)" class="owner-item__preview">
            <img :src="ownerImageURL(primaryImage(item) as UploadedImage)" :alt="primaryImage(item)?.alt_text || item.title" loading="lazy" />
          </span>
          <span v-else class="owner-item__preview owner-item__preview--placeholder">
            {{ item.category.name.slice(0, 2).toUpperCase() }}
            <em v-if="primaryImage(item)">{{ statusLabel(primaryImage(item)?.processing_status || "image pending") }}</em>
          </span>
          <span class="owner-item__main">
            <strong>{{ item.title }}</strong>
            <small class="owner-item__meta">
              <span>{{ item.category.name }} / {{ item.visibility }}</span>
              <span>{{ formatBytes(item.file?.file_size) }}</span>
            </small>
          </span>
        </button>

        <span class="owner-item__badges" aria-label="Package state">
          <span class="status-badge" :class="`status-badge--${item.status}`">
            {{ statusLabel(item.status) }}
          </span>
          <span class="status-badge" :class="`status-badge--${item.file?.scan_status || 'no_file'}`">
            {{ statusLabel(item.file?.scan_status || "no file") }}
          </span>
          <span v-if="item.nsfw" class="status-badge status-badge--nsfw">NSFW</span>
        </span>

        <div class="owner-item__actions">
          <span class="owner-item__tool-label">
            {{ item.visibility === "private" ? "Private owner tools" : "Owner tools" }}
          </span>
          <div class="owner-item__tool-buttons">
            <button
              class="owner-item__icon-action"
              type="button"
              :disabled="!canCopyOwnerDownload(item)"
              :aria-label="canCopyOwnerDownload(item) ? 'Copy owner download link' : 'Download link is available after clean processing'"
              :title="canCopyOwnerDownload(item) ? 'Copy owner download link' : 'Available after clean processing'"
              @click="copyOwnerDownload(item)"
            >
              <Icon :node="LinkIcon" />
            </button>
            <button
              class="owner-item__icon-action"
              type="button"
              :disabled="!item.unlock_password"
              :aria-label="item.unlock_password ? 'Copy Basis password' : 'No Basis password stored'"
              :title="item.unlock_password ? 'Copy Basis password' : 'No Basis password stored'"
              @click="copyPassword(item)"
            >
              <Icon :node="KeyRoundIcon" />
            </button>
          </div>

          <button class="button button--secondary owner-item__delete" type="button" :disabled="owner.loading" @click="deleteItem(item)">
            <Icon :node="TrashIcon" />
            <span>Delete</span>
          </button>
        </div>

        <small v-if="item.moderation" class="owner-item__feedback">
          {{ moderationLabel(item.moderation) }}: {{ item.moderation.reason }}
        </small>
      </article>
    </div>
  </section>
</template>
