<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useOwnerStore } from "../../stores/owner.store";

const owner = useOwnerStore();
const title = ref("");
const description = ref("");
const visibility = ref<"public" | "private">("public");
const nsfw = ref(false);
const tags = ref("");
const localError = ref("");
const saving = ref(false);

const disabled = computed(() => !owner.selectedItem || saving.value || owner.loading);

watch(
  () => owner.selectedItem,
  (item) => {
    title.value = item?.title ?? "";
    description.value = item?.description ?? "";
    visibility.value = item?.visibility ?? "public";
    nsfw.value = item?.nsfw ?? false;
    tags.value = item?.tags.map((tag) => tag.name).join(", ") ?? "";
    localError.value = "";
  },
  { immediate: true },
);

async function submitMetadata() {
  localError.value = "";
  if (!owner.selectedItem) {
    localError.value = "Select content before editing metadata.";
    return;
  }
  const normalizedTitle = title.value.trim();
  if (!normalizedTitle) {
    localError.value = "Title is required.";
    return;
  }
  const tagList = tags.value
    .split(",")
    .map((tag) => tag.trim())
    .filter(Boolean);

  saving.value = true;
  try {
    await owner.updateSelectedMetadata({
      title: normalizedTitle,
      description: description.value.trim(),
      visibility: visibility.value,
      nsfw: nsfw.value,
      tags: tagList,
    });
  } catch (caught) {
    localError.value = caught instanceof Error ? caught.message : "Metadata update failed.";
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <section class="panel profile-metadata-panel" aria-labelledby="metadata-title">
    <div class="panel__head">
      <p class="eyebrow">Package metadata</p>
      <h2 id="metadata-title">Edit listing</h2>
    </div>

    <div v-if="!owner.selectedItem" class="message message--warning">Select an owned package before editing metadata.</div>

    <form class="form-grid" :class="{ 'form-grid--disabled': !owner.selectedItem }" @submit.prevent="submitMetadata">
      <label class="field">
        Title
        <input v-model="title" type="text" maxlength="140" :disabled="disabled" required />
      </label>

      <label class="field">
        Description
        <textarea v-model="description" maxlength="4000" rows="5" :disabled="disabled"></textarea>
      </label>

      <div class="form-row">
        <label class="field">
          Visibility
          <select v-model="visibility" :disabled="disabled">
            <option value="public">Public</option>
            <option value="private">Private</option>
          </select>
        </label>
        <label class="check-row">
          <input v-model="nsfw" type="checkbox" :disabled="disabled" />
          NSFW
        </label>
      </div>

      <label class="field">
        Tags
        <input v-model="tags" type="text" maxlength="700" :disabled="disabled" placeholder="world, quest, udon" />
      </label>

      <button class="button button--primary" type="submit" :disabled="disabled">
        {{ saving ? "Saving..." : "Save metadata" }}
      </button>
    </form>

    <div v-if="localError" class="message message--error">{{ localError }}</div>
  </section>
</template>
