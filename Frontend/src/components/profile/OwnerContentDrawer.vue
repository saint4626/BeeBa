<script setup lang="ts">
import { computed, defineComponent, h, ref, watch, type PropType } from "vue";
import { Edit3, Images, KeyRound, Link, Trash2, X } from "lucide";
import ContentMetadataEditor from "./ContentMetadataEditor.vue";
import GalleryManager from "./GalleryManager.vue";
import { publicAPIURL } from "../../lib/api/client";
import { basisContentDownloadPath } from "../../lib/api/download-url";
import { getOwnedContentDownloadLink } from "../../lib/api/uploads";
import { BYTE_UNITS } from "../../lib/config/runtime";
import { ui, type Locale } from "../../lib/i18n";
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
const props = withDefaults(defineProps<{ locale?: Locale }>(), { locale: "en" });
const locale = props.locale;
const t = ui[locale].profile;
const selected = computed(() => owner.selectedItem);
const activePanel = ref<DrawerPanelID | null>(null);
const pendingDeleteID = ref("");
const pendingCopyID = ref("");
const expanded = computed(() => activePanel.value !== null);

watch(
  () => owner.selectedContentID,
  () => {
    activePanel.value = null;
    pendingDeleteID.value = "";
    pendingCopyID.value = "";
  },
);

function statusLabel(value?: string) {
  return (value || "unknown").replaceAll("_", " ");
}

function formatBytes(value?: number) {
  if (!value) return t.noFileSize;
  if (value < BYTE_UNITS.kib) return `${value} B`;
  if (value < BYTE_UNITS.mib) return `${(value / BYTE_UNITS.kib).toFixed(1)} KB`;
  if (value < BYTE_UNITS.gib) return `${(value / BYTE_UNITS.mib).toFixed(1)} MB`;
  return `${(value / BYTE_UNITS.gib).toFixed(2)} GB`;
}

function canCopyOwnerDownload(item: OwnerContentItem) {
  return item.status === "published" && item.file?.scan_status === "clean";
}

async function ownerDownloadURL(item: OwnerContentItem) {
  if (item.visibility === "private") {
    const link = await getOwnedContentDownloadLink(owner.accessToken, item.id);
    return publicAPIURL(link.download_path);
  }
  return publicAPIURL(basisContentDownloadPath(item));
}

async function copyText(value: string, label: string) {
  try {
    await navigator.clipboard.writeText(value);
    showToast(label, "success");
  } catch {
    showToast(ui[locale].common.copyFailed, "error");
  }
}

async function copyOwnerDownload(item: OwnerContentItem) {
  if (!canCopyOwnerDownload(item)) {
    showToast(t.linkAfterProcessingToast, "info");
    return;
  }
  pendingCopyID.value = item.id;
  try {
    await copyText(await ownerDownloadURL(item), t.ownerLinkCopied);
  } catch (caught) {
    showToast(caught instanceof Error ? caught.message : ui[locale].common.copyFailed, "error");
  } finally {
    pendingCopyID.value = "";
  }
}

async function copyPassword(item: OwnerContentItem) {
  if (!item.unlock_password) {
    showToast(t.noPasswordStored, "info");
    return;
  }
  await copyText(item.unlock_password, t.passwordCopied);
}

async function deleteItem(item: OwnerContentItem) {
  if (pendingDeleteID.value !== item.id) {
    pendingDeleteID.value = item.id;
    showToast(`${t.deleteAgainPrefix} "${item.title}"${t.deleteAgainSuffix}`, "info");
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
        <p class="eyebrow">{{ t.selectedPackage }}</p>
        <h2 id="selected-content-title">{{ selected.title }}</h2>
        <p>
          {{ selected.category.name }} - {{ selected.visibility }} -
          {{ formatBytes(selected.file?.file_size) }} -
          {{ statusLabel(selected.status) }} / {{ statusLabel(selected.file?.scan_status) }}
        </p>
      </div>
      <button class="owner-item__icon-action" type="button" :aria-label="t.closeSelected" @click="closeDrawer">
        <Icon :node="CloseIcon" />
      </button>
    </div>

    <div class="profile-content-drawer__actions" :aria-label="t.ownerActionsLabel">
      <button
        class="button button--primary"
        type="button"
        :aria-pressed="activePanel === 'details'"
        @click="togglePanel('details')"
      >
        <Icon :node="EditIcon" />
        <span>{{ activePanel === "details" ? t.hideMetadata : t.editMetadata }}</span>
      </button>
      <button
        class="button button--secondary"
        type="button"
        :aria-pressed="activePanel === 'media'"
        @click="togglePanel('media')"
      >
        <Icon :node="ImagesIcon" />
        <span>{{ activePanel === "media" ? t.hideMedia : t.manageMedia }}</span>
      </button>
      <button
        class="button button--secondary"
        type="button"
        :disabled="!canCopyOwnerDownload(selected) || pendingCopyID === selected.id"
        :title="canCopyOwnerDownload(selected) ? t.copyOwnerLinkTitle : t.availableAfterProcessing"
        @click="copyOwnerDownload(selected)"
      >
        <Icon :node="LinkIcon" />
        <span>{{ t.copyBasisLink }}</span>
      </button>
      <button
        class="button button--secondary"
        type="button"
        :disabled="!selected.unlock_password"
        :title="selected.unlock_password ? t.copyPassword : t.noPasswordStored"
        @click="copyPassword(selected)"
      >
        <Icon :node="KeyRoundIcon" />
        <span>{{ t.copyPassword }}</span>
      </button>
      <button class="button button--secondary owner-item__delete" type="button" :disabled="owner.loading" @click="deleteItem(selected)">
        <Icon :node="TrashIcon" />
        <span>{{ pendingDeleteID === selected.id ? t.confirmDelete : t.delete }}</span>
      </button>
    </div>

    <p v-if="selected.moderation" class="owner-item__feedback">
      {{ statusLabel(selected.moderation.action) }}: {{ selected.moderation.reason }}
    </p>

    <div v-if="activePanel" class="profile-content-drawer__editor">
      <ContentMetadataEditor v-if="activePanel === 'details'" :locale="locale" />
      <GalleryManager v-else :locale="locale" />
    </div>
  </section>
</template>
