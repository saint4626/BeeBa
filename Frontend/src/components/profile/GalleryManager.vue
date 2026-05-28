<script setup lang="ts">
import { computed, ref } from "vue";
import { useOwnerStore } from "../../stores/owner.store";
import type { UploadedImage } from "../../lib/api/types";

const owner = useOwnerStore();
const imageFile = ref<File | null>(null);
const altText = ref("");
const primary = ref(false);
const loading = ref(false);
const localError = ref("");

const imageFileName = computed(() => imageFile.value?.name ?? "No image selected");
const selectedContentIsPublic = computed(() => owner.selectedItem?.status === "published" && owner.selectedItem.visibility === "public");
const displayableImages = computed(() =>
  owner.selectedImages.filter((image) => image.processing_status === "processed" && selectedContentIsPublic.value),
);
const unavailableImages = computed(() =>
  owner.selectedImages.filter((image) => image.processing_status !== "processed" || !selectedContentIsPublic.value),
);

function onImageChange(event: Event) {
  const input = event.target as HTMLInputElement;
  imageFile.value = input.files?.[0] ?? null;
}

async function submitImage() {
  localError.value = "";
  if (!owner.selectedContentID) {
    localError.value = "Select content before uploading images.";
    return;
  }
  if (!imageFile.value) {
    localError.value = "Choose a JPEG or PNG image.";
    return;
  }
  loading.value = true;
  try {
    await owner.addContentImage(imageFile.value, altText.value.trim(), primary.value);
    imageFile.value = null;
    altText.value = "";
    primary.value = false;
  } catch (caught) {
    localError.value = caught instanceof Error ? caught.message : "Image upload failed.";
  } finally {
    loading.value = false;
  }
}

async function saveImage(image: UploadedImage) {
  const alt = window.prompt("Alt text", image.alt_text) ?? image.alt_text;
  const sort = Number(window.prompt("Sort order", String(image.sort_order ?? 0)) ?? image.sort_order ?? 0);
  if (!Number.isInteger(sort) || sort < 0 || sort > 1000) {
    localError.value = "Sort order must be a whole number from 0 to 1000.";
    return;
  }
  await owner.updateImageText(image.id, alt, sort);
}

async function deleteImage(image: UploadedImage) {
  const confirmed = window.confirm("Remove this image from the content gallery?");
  if (!confirmed) return;
  await owner.removeImage(image.id);
}
</script>

<template>
  <section class="panel profile-gallery-panel" aria-labelledby="gallery-title">
    <div class="panel__head">
      <p class="eyebrow">Content media</p>
      <h2 id="gallery-title">Preview and gallery</h2>
    </div>

    <div v-if="!owner.selectedItem" class="message message--warning">Select an owned package to manage its images.</div>

    <form class="form-grid" :class="{ 'form-grid--disabled': !owner.selectedItem }" @submit.prevent="submitImage">
      <label class="file-drop">
        <input type="file" accept="image/png,image/jpeg" :disabled="!owner.selectedItem" @change="onImageChange" />
        <span>{{ imageFileName }}</span>
      </label>
      <label class="field">
        Alt text
        <input v-model="altText" type="text" maxlength="500" :disabled="!owner.selectedItem" />
      </label>
      <label class="check-row">
        <input v-model="primary" type="checkbox" :disabled="!owner.selectedItem" />
        Use as primary preview
      </label>
      <button class="button button--primary" type="submit" :disabled="loading || !owner.selectedItem">
        {{ loading ? "Uploading..." : "Upload image" }}
      </button>
    </form>

    <div v-if="localError" class="message message--error">{{ localError }}</div>

    <div v-if="unavailableImages.length > 0" class="managed-gallery managed-gallery--pending" aria-label="Images waiting for processing or publication">
      <article v-for="image in unavailableImages" :key="image.id" class="managed-gallery__item managed-gallery__item--pending">
        <div class="managed-gallery__placeholder" aria-hidden="true">IMG</div>
        <div>
          <strong>{{ image.is_primary ? "Primary preview" : "Gallery image" }}</strong>
          <small>
            {{
              image.processing_status === "processed" && !selectedContentIsPublic
                ? "processed / preview available after public publication"
                : `${image.processing_status} / ${image.mime_type_detected} / ${image.file_size} bytes`
            }}
          </small>
          <span>{{ image.alt_text || "No alt text" }}</span>
        </div>
      </article>
    </div>

    <div v-if="displayableImages.length > 0" class="managed-gallery">
      <article v-for="image in displayableImages" :key="image.id" class="managed-gallery__item">
        <img :src="image.url" :alt="image.alt_text" />
        <div>
          <strong>{{ image.is_primary ? "Primary preview" : "Gallery image" }}</strong>
          <small>{{ image.processing_status }} / {{ image.width }}x{{ image.height }}</small>
          <span>{{ image.alt_text || "No alt text" }}</span>
        </div>
        <div class="managed-gallery__actions">
          <button class="button button--secondary" type="button" :disabled="image.is_primary" @click="owner.makePrimary(image.id)">Primary</button>
          <button class="button button--secondary" type="button" @click="saveImage(image)">Edit</button>
          <button class="button button--secondary" type="button" @click="deleteImage(image)">Remove</button>
        </div>
      </article>
    </div>
  </section>
</template>
