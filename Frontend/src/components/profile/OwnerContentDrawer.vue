<script setup lang="ts">
import { computed, defineComponent, h, ref, watch, type PropType } from "vue";
import { Edit3, Images, KeyRound, Link, Trash2, X } from "lucide";
import ContentMetadataEditor from "./ContentMetadataEditor.vue";
import GalleryManager from "./GalleryManager.vue";
import { getPublicAPIBaseURL } from "../../lib/api/client";
import { BYTE_UNITS } from "../../lib/config/runtime";
import { showToast } from "../../lib/ui/toast";
import { useOwnerStore } from "../../stores/owner.store";
import type { OwnerContentItem } from "../../lib/api/types";

type IconNode = Array<[string, Record<string, string>]>;
type DrawerPanelID = "details" | "media";

const LinkIcon = Link as IconNode;
const KeyRoundIcon = KeyRound as IconNode;
const TrashIcon = Trash2 as IconNode;
const CloseIcon = X as IconNode;
const EditIcon = Edit3 as IconNode;
const ImagesIcon = Images as IconNode;

const Icon = defineComponent({
  name: "OwnerDrawerInlineIcon",
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

const owner = useOwnerStore();
const selected = computed(() => owner.selectedItem);
const activePanel = ref<DrawerPanelID | null>(null);
const pendingDeleteID = ref("");
const expanded = computed(() => activePanel.value !== null);

watch(
  () => owner.selectedContentID,
  () => {
    activePanel.value = null;
    pendingDeleteID.value = "";
  },
);

function statusLabel(value?: string) {
  return (value || "unknown").replaceAll("_", " ");
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
  if (pendingDeleteID.value !== item.id) {
    pendingDeleteID.value = item.id;
    showToast(`Click Delete again to remove "${item.title}".`, "info");
    return;
  }
  try {
    await owner.deleteContent(item.id);
  } catch {
    // The owner store exposes the operation error through the shared toast watcher.
  } finally {
    pendingDeleteID.value = "";
  }
}

function closeDrawer() {
  activePanel.value = null;
  owner.selectedContentID = "";
}

function togglePanel(panelID: DrawerPanelID) {
  activePanel.value = activePanel.value === panelID ? null : panelID;
}
</script>

<template>
  <section
    v-if="selected"
    class="profile-content-drawer"
    :class="{ 'profile-content-drawer--expanded': expanded }"
    aria-labelledby="selected-content-title"
  >
    <div class="profile-content-drawer__head">
      <div>
        <p class="eyebrow">Selected package</p>
        <h2 id="selected-content-title">{{ selected.title }}</h2>
        <p>
          {{ selected.category.name }} - {{ selected.visibility }} -
          {{ formatBytes(selected.file?.file_size) }} -
          {{ statusLabel(selected.status) }} / {{ statusLabel(selected.file?.scan_status) }}
        </p>
      </div>
      <button class="owner-item__icon-action" type="button" aria-label="Close selected package panel" @click="closeDrawer">
        <Icon :node="CloseIcon" />
      </button>
    </div>

    <div class="profile-content-drawer__actions" aria-label="Owner package actions">
      <button
        class="button button--primary"
        type="button"
        :aria-pressed="activePanel === 'details'"
        @click="togglePanel('details')"
      >
        <Icon :node="EditIcon" />
        <span>{{ activePanel === "details" ? "Hide metadata" : "Edit metadata" }}</span>
      </button>
      <button
        class="button button--secondary"
        type="button"
        :aria-pressed="activePanel === 'media'"
        @click="togglePanel('media')"
      >
        <Icon :node="ImagesIcon" />
        <span>{{ activePanel === "media" ? "Hide media" : "Manage media" }}</span>
      </button>
      <button
        class="button button--secondary"
        type="button"
        :disabled="!canCopyOwnerDownload(selected)"
        :title="canCopyOwnerDownload(selected) ? 'Copy owner download link' : 'Available after clean processing'"
        @click="copyOwnerDownload(selected)"
      >
        <Icon :node="LinkIcon" />
        <span>Copy Basis link</span>
      </button>
      <button
        class="button button--secondary"
        type="button"
        :disabled="!selected.unlock_password"
        :title="selected.unlock_password ? 'Copy Basis password' : 'No Basis password stored'"
        @click="copyPassword(selected)"
      >
        <Icon :node="KeyRoundIcon" />
        <span>Copy password</span>
      </button>
      <button class="button button--secondary owner-item__delete" type="button" :disabled="owner.loading" @click="deleteItem(selected)">
        <Icon :node="TrashIcon" />
        <span>{{ pendingDeleteID === selected.id ? "Confirm delete" : "Delete" }}</span>
      </button>
    </div>

    <p v-if="selected.moderation" class="owner-item__feedback">
      {{ statusLabel(selected.moderation.action) }}: {{ selected.moderation.reason }}
    </p>

    <div v-if="activePanel" class="profile-content-drawer__editor">
      <ContentMetadataEditor v-if="activePanel === 'details'" />
      <GalleryManager v-else />
    </div>
  </section>
</template>
