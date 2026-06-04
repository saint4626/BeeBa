<script setup lang="ts">
import { computed, defineComponent, h, onBeforeUnmount, ref, watch, type PropType } from "vue";
import { ImagePlus, Save, Star, Trash2, UploadCloud, X } from "lucide";
import { BYTE_UNITS, UPLOAD_LIMITS, megabytesFromBytes } from "../../lib/config/runtime";
import { ui, type Locale } from "../../lib/i18n";
import { showToast } from "../../lib/ui/toast";
import { useOwnerStore } from "../../stores/owner.store";
import type { UploadedImage } from "../../lib/api/types";

type IconNode = Array<[string, Record<string, string>]>;

interface QueuedImage {
  id: string;
  file: File;
  previewURL: string;
  altText: string;
  isPrimary: boolean;
}

const Icon = defineComponent({
  name: "GalleryInlineIcon",
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

const ImagePlusIcon = ImagePlus as IconNode;
const SaveIcon = Save as IconNode;
const StarIcon = Star as IconNode;
const TrashIcon = Trash2 as IconNode;
const UploadIcon = UploadCloud as IconNode;
const RemoveIcon = X as IconNode;

const owner = useOwnerStore();
const props = withDefaults(defineProps<{ locale?: Locale }>(), { locale: "en" });
const t = ui[props.locale].profile;
const fileInput = ref<HTMLInputElement | null>(null);
const queuedImages = ref<QueuedImage[]>([]);
const activeImageID = ref("");
const selectedAltText = ref("");
const selectedSortOrder = ref(0);
const dragActive = ref(false);
const uploading = ref(false);
const uploadIndex = ref(0);
const imageAction = ref<"" | "save" | "primary" | "delete">("");
const pendingDeleteImageID = ref("");

const maxImageMegabytes = computed(() => megabytesFromBytes(UPLOAD_LIMITS.maxImageBytes));
const sortedImages = computed(() =>
  [...owner.selectedImages].sort((left, right) => {
    if (Boolean(left.is_primary) !== Boolean(right.is_primary)) return left.is_primary ? -1 : 1;
    return (left.sort_order ?? 0) - (right.sort_order ?? 0);
  }),
);
const selectedImage = computed(
  () =>
    sortedImages.value.find((image) => image.id === activeImageID.value) ??
    sortedImages.value.find((image) => image.is_primary) ??
    sortedImages.value[0] ??
    null,
);
const selectedImageURL = computed(() => (selectedImage.value ? ownerImageURL(selectedImage.value) : ""));
const uploadButtonLabel = computed(() => {
  if (!uploading.value) return queuedImages.value.length > 1 ? t.uploadImages : t.uploadImage;
  return `${t.uploadingImagesPrefix} ${uploadIndex.value}/${queuedImages.value.length}`;
});

watch(
  () => owner.selectedContentID,
  () => {
    clearQueue();
    activeImageID.value = "";
  },
);

watch(
  () => sortedImages.value.map((image) => `${image.id}:${image.is_primary}:${image.processing_status}`).join("|"),
  () => {
    if (!sortedImages.value.some((image) => image.id === activeImageID.value)) {
      activeImageID.value = sortedImages.value.find((image) => image.is_primary)?.id ?? sortedImages.value[0]?.id ?? "";
    }
  },
  { immediate: true },
);

watch(
  selectedImage,
  (image) => {
    selectedAltText.value = image?.alt_text ?? "";
    selectedSortOrder.value = image?.sort_order ?? 0;
    pendingDeleteImageID.value = "";
  },
  { immediate: true },
);

onBeforeUnmount(() => {
  clearQueue();
});

function ownerImageURL(image: UploadedImage) {
  if (image.processing_status !== "processed" || !image.url) return "";
  return image.url.replace("/api/v1/media/", "/api/v1/me/media/");
}

function formatBytes(value?: number) {
  if (!value) return "0 B";
  if (value < BYTE_UNITS.kib) return `${value} B`;
  if (value < BYTE_UNITS.mib) return `${(value / BYTE_UNITS.kib).toFixed(1)} KB`;
  if (value < BYTE_UNITS.gib) return `${(value / BYTE_UNITS.mib).toFixed(1)} MB`;
  return `${(value / BYTE_UNITS.gib).toFixed(2)} GB`;
}

function chooseFiles() {
  if (!owner.selectedItem || uploading.value) return;
  fileInput.value?.click();
}

function onInputChange(event: Event) {
  const input = event.target as HTMLInputElement;
  queueFiles(input.files);
  input.value = "";
}

function onDrop(event: DragEvent) {
  dragActive.value = false;
  queueFiles(event.dataTransfer?.files);
}

function queueFiles(files: FileList | null | undefined) {
  if (!owner.selectedItem) {
    showToast(t.selectPackageBeforeImages, "info");
    return;
  }

  const nextFiles = Array.from(files ?? []);
  if (nextFiles.length === 0) return;

  let accepted = 0;
  for (const file of nextFiles) {
    if (!["image/jpeg", "image/png"].includes(file.type)) {
      showToast(`${file.name} ${t.imageTypeSuffix}`, "error");
      continue;
    }
    if (file.size > UPLOAD_LIMITS.maxImageBytes) {
      showToast(`${file.name} exceeds ${maxImageMegabytes.value} MB.`, "error");
      continue;
    }

    queuedImages.value.push({
      id: queueID(),
      file,
      previewURL: URL.createObjectURL(file),
      altText: "",
      isPrimary: shouldBecomePrimary(),
    });
    accepted += 1;
  }

  if (accepted > 0) {
    showToast(`${accepted} ${t.imagesAddedSuffix}`, "info");
  }
}

function shouldBecomePrimary() {
  return !owner.selectedImages.some((image) => image.is_primary) && !queuedImages.value.some((image) => image.isPrimary);
}

function setQueuedPrimary(imageID: string) {
  queuedImages.value = queuedImages.value.map((image) => ({
    ...image,
    isPrimary: image.id === imageID,
  }));
}

function removeQueued(imageID: string) {
  const removed = queuedImages.value.find((image) => image.id === imageID);
  if (!removed) return;
  URL.revokeObjectURL(removed.previewURL);
  queuedImages.value = queuedImages.value.filter((image) => image.id !== imageID);
  if (removed.isPrimary && shouldBecomePrimary() && queuedImages.value[0]) {
    queuedImages.value[0].isPrimary = true;
  }
}

function clearQueue() {
  queuedImages.value.forEach((image) => URL.revokeObjectURL(image.previewURL));
  queuedImages.value = [];
  uploadIndex.value = 0;
}

async function uploadQueuedImages() {
  if (!owner.selectedItem || queuedImages.value.length === 0 || uploading.value) return;

  uploading.value = true;
  uploadIndex.value = 0;
  try {
    const uploadBatch = [...queuedImages.value];
    for (let index = 0; index < uploadBatch.length; index += 1) {
      uploadIndex.value = index + 1;
      const image = uploadBatch[index];
      await owner.addContentImage(image.file, image.altText.trim(), image.isPrimary, { silent: true });
    }
    clearQueue();
    showToast(`${uploadBatch.length} ${t.imagesQueuedSuffix}`, "success");
  } catch (caught) {
    showToast(caught instanceof Error ? caught.message : t.imageUploadFailed, "error");
  } finally {
    uploading.value = false;
    uploadIndex.value = 0;
  }
}

async function saveSelectedImage() {
  if (!selectedImage.value || imageAction.value) return;
  const sortOrder = Number(selectedSortOrder.value);
  if (!Number.isInteger(sortOrder) || sortOrder < 0 || sortOrder > 1000) {
    showToast(t.sortOrderInvalid, "error");
    return;
  }
  imageAction.value = "save";
  try {
    await owner.updateImageText(selectedImage.value.id, selectedAltText.value.trim(), sortOrder);
  } catch (caught) {
    showToast(caught instanceof Error ? caught.message : t.imageMetadataFailed, "error");
  } finally {
    imageAction.value = "";
  }
}

async function makeSelectedPrimary() {
  if (!selectedImage.value || selectedImage.value.is_primary || imageAction.value) return;
  imageAction.value = "primary";
  try {
    await owner.makePrimary(selectedImage.value.id);
  } catch (caught) {
    showToast(caught instanceof Error ? caught.message : t.primaryUpdateFailed, "error");
  } finally {
    imageAction.value = "";
  }
}

async function deleteSelectedImage() {
  if (!selectedImage.value || imageAction.value) return;
  if (pendingDeleteImageID.value !== selectedImage.value.id) {
    pendingDeleteImageID.value = selectedImage.value.id;
    showToast(t.removeConfirm, "info");
    return;
  }
  imageAction.value = "delete";
  try {
    await owner.removeImage(selectedImage.value.id);
  } catch (caught) {
    showToast(caught instanceof Error ? caught.message : t.imageRemovalFailed, "error");
  } finally {
    pendingDeleteImageID.value = "";
    imageAction.value = "";
  }
}

function queueID() {
  return typeof crypto !== "undefined" && "randomUUID" in crypto
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(16).slice(2)}`;
}
</script>

<template>
  <section class="profile-gallery-panel" aria-labelledby="gallery-title">
    <div class="panel__head">
      <p class="eyebrow">{{ t.contentMedia }}</p>
      <h2 id="gallery-title">{{ t.previewGallery }}</h2>
      <p>{{ t.previewGalleryCopy }}</p>
    </div>

    <div v-if="!owner.selectedItem" class="empty-state empty-state--compact">
      <span class="empty-state__badge">{{ t.noPackageSelected }}</span>
      <p>{{ t.selectPackageImages }}</p>
    </div>

    <div v-else class="profile-gallery-editor">
      <div class="media-viewer">
        <div class="media-viewer__stage">
          <img v-if="selectedImageURL" :src="selectedImageURL" :alt="selectedImage?.alt_text || owner.selectedItem.title" />
          <span v-else class="media-viewer__placeholder">
            <Icon :node="ImagePlusIcon" :size="24" />
            <span>{{ selectedImage ? selectedImage.processing_status : t.noImagesYet }}</span>
          </span>
        </div>

        <div v-if="sortedImages.length > 0" class="media-strip" :aria-label="t.existingImagesLabel">
          <button
            v-for="image in sortedImages"
            :key="image.id"
            class="media-thumb"
            :class="{ 'media-thumb--active': selectedImage?.id === image.id }"
            type="button"
            :aria-pressed="selectedImage?.id === image.id"
            @click="activeImageID = image.id"
          >
            <img v-if="ownerImageURL(image)" :src="ownerImageURL(image)" :alt="image.alt_text || owner.selectedItem.title" />
            <span v-else>{{ image.processing_status }}</span>
            <em v-if="image.is_primary">{{ t.primary }}</em>
          </button>
        </div>
      </div>

      <div class="media-tools">
        <div v-if="selectedImage" class="media-selected-card">
          <div class="media-selected-card__head">
            <div>
              <p class="eyebrow">{{ t.selectedImage }}</p>
              <strong>{{ selectedImage.is_primary ? t.primaryPreview : t.galleryImage }}</strong>
              <span>{{ selectedImage.processing_status }} / {{ formatBytes(selectedImage.file_size) }}</span>
            </div>
            <button
              class="button button--secondary"
              type="button"
              :disabled="selectedImage.is_primary || Boolean(imageAction)"
              @click="makeSelectedPrimary"
            >
              <Icon :node="StarIcon" />
              <span>{{ imageAction === "primary" ? ui[props.locale].common.saving : t.setPrimary }}</span>
            </button>
          </div>

          <div class="media-selected-card__fields">
            <label class="field">
              {{ t.altText }}
              <input v-model="selectedAltText" type="text" maxlength="500" />
            </label>
            <label class="field">
              {{ t.sort }}
              <input v-model.number="selectedSortOrder" type="number" min="0" max="1000" step="1" />
            </label>
          </div>

          <div class="media-selected-card__actions">
            <button class="button button--primary" type="button" :disabled="Boolean(imageAction)" @click="saveSelectedImage">
              <Icon :node="SaveIcon" />
              <span>{{ imageAction === "save" ? ui[props.locale].common.saving : t.saveImage }}</span>
            </button>
            <button class="button button--secondary owner-item__delete" type="button" :disabled="Boolean(imageAction)" @click="deleteSelectedImage">
              <Icon :node="TrashIcon" />
              <span>{{ imageAction === "delete" ? t.removing : pendingDeleteImageID === selectedImage.id ? t.confirmRemove : t.removeImage }}</span>
            </button>
          </div>
        </div>

        <div
          class="media-upload-zone"
          :class="{ 'media-upload-zone--active': dragActive }"
          @dragenter.prevent="dragActive = true"
          @dragover.prevent="dragActive = true"
          @dragleave.prevent="dragActive = false"
          @drop.prevent="onDrop"
        >
          <input
            ref="fileInput"
            class="sr-only"
            type="file"
            accept="image/png,image/jpeg"
            multiple
            :disabled="uploading"
            @change="onInputChange"
          />
          <Icon :node="UploadIcon" :size="22" />
          <div>
            <strong>{{ t.dropImages }}</strong>
            <span>{{ t.imageLimitPrefix }} {{ maxImageMegabytes }} {{ t.imageLimitSuffix }}</span>
          </div>
          <button class="button button--secondary" type="button" :disabled="uploading" @click="chooseFiles">
            {{ t.chooseImages }}
          </button>
        </div>

        <div v-if="queuedImages.length > 0" class="media-upload-queue" :aria-label="t.queuedImagesLabel">
          <article v-for="image in queuedImages" :key="image.id" class="media-queue-card">
            <img :src="image.previewURL" :alt="image.altText || image.file.name" />
            <div class="media-queue-card__body">
              <strong>{{ image.file.name }}</strong>
              <span>{{ formatBytes(image.file.size) }}</span>
              <input v-model="image.altText" type="text" maxlength="500" :placeholder="t.altText" />
            </div>
            <label class="media-queue-card__primary">
              <input type="radio" name="queued-primary-image" :checked="image.isPrimary" @change="setQueuedPrimary(image.id)" />
              {{ t.primary }}
            </label>
            <button class="owner-item__icon-action" type="button" aria-label="Remove image from upload queue" @click="removeQueued(image.id)">
              <Icon :node="RemoveIcon" />
            </button>
          </article>

          <div class="media-upload-queue__actions">
            <button class="button button--ghost" type="button" :disabled="uploading" @click="clearQueue">{{ t.clearQueue }}</button>
            <button class="button button--primary" type="button" :disabled="uploading" @click="uploadQueuedImages">
              <Icon :node="UploadIcon" />
              <span>{{ uploadButtonLabel }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
