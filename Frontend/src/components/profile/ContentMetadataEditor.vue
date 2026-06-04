<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ui, type Locale } from "../../lib/i18n";
import { showToast } from "../../lib/ui/toast";
import { useOwnerStore } from "../../stores/owner.store";

const owner = useOwnerStore();
const props = withDefaults(defineProps<{ locale?: Locale }>(), { locale: "en" });
const t = ui[props.locale].profile;
const title = ref("");
const description = ref("");
const visibility = ref<"public" | "private">("public");
const nsfw = ref(false);
const tags = ref("");
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
  },
  { immediate: true },
);

async function submitMetadata() {
  if (!owner.selectedItem) {
    showToast(t.selectBeforeEdit, "info");
    return;
  }
  const normalizedTitle = title.value.trim();
  if (!normalizedTitle) {
    showToast(t.titleRequired, "error");
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
    showToast(caught instanceof Error ? caught.message : t.metadataFailed, "error");
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <section class="panel profile-metadata-panel" aria-labelledby="metadata-title">
    <div class="panel__head">
      <p class="eyebrow">{{ t.metadataEyebrow }}</p>
      <h2 id="metadata-title">{{ t.editListing }}</h2>
    </div>

    <div v-if="!owner.selectedItem" class="message message--warning">{{ t.selectBeforeEdit }}</div>

    <form class="form-grid" :class="{ 'form-grid--disabled': !owner.selectedItem }" @submit.prevent="submitMetadata">
      <label class="field">
        {{ t.titleLabel }}
        <input v-model="title" type="text" maxlength="140" :disabled="disabled" required />
      </label>

      <label class="field">
        {{ t.descriptionLabel }}
        <textarea v-model="description" maxlength="4000" rows="5" :disabled="disabled"></textarea>
      </label>

      <div class="form-row">
        <label class="field">
          {{ t.visibilityLabel }}
          <select v-model="visibility" :disabled="disabled">
            <option value="public">{{ t.publicVisibility }}</option>
            <option value="private">{{ t.privateVisibility }}</option>
          </select>
        </label>
        <label class="check-row">
          <input v-model="nsfw" type="checkbox" :disabled="disabled" />
          NSFW
        </label>
      </div>

      <label class="field">
        {{ t.tagsLabel }}
        <input v-model="tags" type="text" maxlength="700" :disabled="disabled" :placeholder="t.tagsPlaceholder" />
      </label>

      <button class="button button--primary" type="submit" :disabled="disabled">
        {{ saving ? ui[props.locale].common.saving : t.saveMetadata }}
      </button>
    </form>
  </section>
</template>
