<script setup lang="ts">
import { onMounted, ref, watch } from "vue";
import {
  createAdminTag,
  listAdminCategories,
  listAdminTags,
  updateAdminCategory,
  updateAdminTag,
} from "../../lib/api/admin";
import type { AdminCategory, CatalogTag } from "../../lib/api/types";
import { ui, type Locale } from "../../lib/i18n";
import { showToast } from "../../lib/ui/toast";

const props = defineProps<{
  accessToken: string;
  isAuthorized: boolean;
  refreshNonce?: number;
  locale?: Locale;
}>();

const emit = defineEmits<{
  count: [value: number];
}>();

const categories = ref<AdminCategory[]>([]);
const locale = props.locale ?? "en";
const t = ui[locale].adminPanels.taxonomy;
const common = ui[locale].adminPanels.common;
const tags = ref<CatalogTag[]>([]);
const tagSlug = ref("");
const tagName = ref("");
const tagIsSystem = ref(true);
const loading = ref(false);
const actionID = ref("");
const error = ref("");

onMounted(() => {
  void refreshTaxonomy(true);
});

watch(
  () => props.refreshNonce,
  () => {
    void refreshTaxonomy(true);
  },
);

async function refreshTaxonomy(silent = false) {
  if (!props.isAuthorized) return;
  loading.value = true;
  error.value = "";
  try {
    const [nextCategories, nextTags] = await Promise.all([
      listAdminCategories(props.accessToken),
      listAdminTags(props.accessToken),
    ]);
    categories.value = nextCategories;
    tags.value = nextTags;
    emit("count", categories.value.length + tags.value.length);
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.loadFailed;
    emit("count", categories.value.length + tags.value.length);
    if (!silent) {
      showToast(error.value, "error");
    }
  } finally {
    loading.value = false;
  }
}

async function toggleCategory(category: AdminCategory) {
  await patchCategory(category, { is_active: !category.is_active });
}

async function patchCategory(category: AdminCategory, input: { is_active?: boolean }) {
  actionID.value = category.id;
  error.value = "";
  try {
    await updateAdminCategory(props.accessToken, category.id, input);
    showToast(t.categoryUpdated, "success");
    await refreshTaxonomy(true);
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.categoryUpdateFailed;
    showToast(error.value, "error");
  } finally {
    actionID.value = "";
  }
}

async function createTag() {
  actionID.value = "create-tag";
  error.value = "";
  try {
    await createAdminTag(props.accessToken, {
      slug: tagSlug.value.trim(),
      name: tagName.value.trim(),
      is_system: tagIsSystem.value,
    });
    tagSlug.value = "";
    tagName.value = "";
    tagIsSystem.value = true;
    showToast(t.tagCreated, "success");
    await refreshTaxonomy(true);
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.tagCreateFailed;
    showToast(error.value, "error");
  } finally {
    actionID.value = "";
  }
}

async function toggleSystemTag(tag: CatalogTag) {
  actionID.value = tag.id;
  error.value = "";
  try {
    await updateAdminTag(props.accessToken, tag.id, { is_system: !tag.is_system });
    showToast(t.tagUpdated, "success");
    await refreshTaxonomy(true);
  } catch (caught) {
    error.value = caught instanceof Error ? caught.message : t.tagUpdateFailed;
    showToast(error.value, "error");
  } finally {
    actionID.value = "";
  }
}
</script>

<template>
  <section class="panel panel--wide" aria-labelledby="taxonomy-title">
    <div class="panel__head panel__head--row">
      <div>
        <p class="eyebrow">{{ t.eyebrow }}</p>
        <h2 id="taxonomy-title">{{ t.title }}</h2>
      </div>
      <button class="button button--secondary" type="button" :disabled="loading || !isAuthorized" @click="refreshTaxonomy()">{{ common.refresh }}</button>
    </div>

    <div v-if="!isAuthorized" class="message message--warning">{{ t.unauthorized }}</div>

    <div class="admin-controls">
      <label class="field">
        {{ t.tagSlug }}
        <input v-model="tagSlug" type="text" maxlength="80" placeholder="basis-ready" :disabled="!isAuthorized" />
      </label>
      <label class="field">
        {{ t.tagName }}
        <input v-model="tagName" type="text" maxlength="80" placeholder="Basis Ready" :disabled="!isAuthorized" />
      </label>
      <label class="field field--inline">
        <input v-model="tagIsSystem" type="checkbox" :disabled="!isAuthorized" />
        {{ t.systemTag }}
      </label>
      <button class="button button--secondary" type="button" :disabled="!isAuthorized || actionID === 'create-tag'" @click="createTag">{{ t.createTag }}</button>
    </div>

    <div class="moderation-list">
      <article v-for="category in categories" :key="category.id" class="moderation-item moderation-item--compact">
        <div class="moderation-item__main">
          <div class="content-card__topline">
            <span class="badge badge--bee">{{ category.slug }}</span>
            <span class="badge" :class="{ 'badge--nsfw': !category.is_active }">{{ category.is_active ? common.active : common.inactive }}</span>
          </div>
          <h3>{{ category.name }}</h3>
          <p>{{ category.description }}</p>
          <dl class="meta-grid">
            <div>
              <dt>{{ common.published }}</dt>
              <dd>{{ category.published_count }}</dd>
            </div>
            <div>
              <dt>{{ t.sort }}</dt>
              <dd>{{ category.sort_order }}</dd>
            </div>
            <div>
              <dt>{{ common.id }}</dt>
              <dd>{{ category.id }}</dd>
            </div>
          </dl>
        </div>
        <div class="moderation-actions">
          <button class="button button--secondary" type="button" :disabled="actionID === category.id" @click="toggleCategory(category)">
            {{ category.is_active ? t.deactivate : t.activate }}
          </button>
        </div>
      </article>
    </div>

    <div class="moderation-list">
      <article v-for="tag in tags" :key="tag.id" class="moderation-item moderation-item--compact">
        <div class="moderation-item__main">
          <div class="content-card__topline">
            <span class="badge badge--bee">{{ tag.slug }}</span>
            <span class="badge">{{ tag.is_system ? common.system : common.user }}</span>
          </div>
          <h3>{{ tag.name }}</h3>
          <p>{{ tag.published_count }} {{ t.publishedItems }}</p>
        </div>
        <div class="moderation-actions">
          <button class="button button--secondary" type="button" :disabled="actionID === tag.id" @click="toggleSystemTag(tag)">
            {{ tag.is_system ? t.makeUserTag : t.makeSystemTag }}
          </button>
        </div>
      </article>
    </div>
  </section>
</template>
