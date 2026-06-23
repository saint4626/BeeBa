<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import { getPublicAPIBaseURL } from "../../lib/api/client";
import { publicMediaURL } from "../../lib/api/media-url";
import { ui, type Locale } from "../../lib/i18n";
import { buildContentPath } from "../../lib/seo/content-url";
import type { APIListResponse, CatalogTag, Category, CursorPagination, PublicContentItem } from "../../lib/api/types";

type IconName = "worlds" | "avatars" | "props" | "prefabs" | "download" | "heart" | "comment";

const props = defineProps<{
  initialItems: PublicContentItem[];
  initialPagination?: CursorPagination;
  categories: Category[];
  tags: CatalogTag[];
  query: string;
  activeTags: string[];
  sort: "newest" | "likes" | "downloads";
  includeNSFW: boolean;
  activeCategory?: Category["slug"];
  baseCatalogPath: string;
  categoryParamMode: "path" | "query";
  author: string;
  locale: Locale;
  publicAPIBaseURL: string;
  error?: string;
}>();

const t = ui[props.locale].catalog;
const items = ref<PublicContentItem[]>(props.initialItems);
const nextCursor = ref(props.initialPagination?.next_cursor ?? "");
const loading = ref(false);
const loadError = ref("");
const sentinel = ref<HTMLElement | null>(null);
let observer: IntersectionObserver | null = null;

onMounted(() => {
  if (!sentinel.value) return;
  observer = new IntersectionObserver((entries) => {
    if (entries.some((entry) => entry.isIntersecting)) {
      loadMore();
    }
  }, { root: document.querySelector("[data-site-scroll]"), rootMargin: "520px 0px" });
  observer.observe(sentinel.value);
});

onBeforeUnmount(() => {
  observer?.disconnect();
});

async function loadMore() {
  if (!nextCursor.value || loading.value) return;
  loading.value = true;
  loadError.value = "";
  try {
    const response = await fetch(buildAPIURL(nextCursor.value), { headers: { Accept: "application/json" } });
    if (!response.ok) {
      throw new Error(`Catalog request failed with status ${response.status}`);
    }
    const payload = await response.json() as APIListResponse<PublicContentItem>;
    items.value = mergeItems(items.value, payload.data);
    nextCursor.value = payload.pagination?.next_cursor ?? "";
  } catch (caught) {
    loadError.value = caught instanceof Error ? caught.message : t.unavailableTitle;
  } finally {
    loading.value = false;
  }
}

function buildAPIURL(cursor: string) {
  const params = new URLSearchParams();
  if (props.activeCategory) params.set("category", props.activeCategory);
  if (props.author) params.set("author", props.author);
  if (props.query) params.set("q", props.query);
  if (props.activeTags.length) params.set("tags", props.activeTags.join(","));
  if (props.sort) params.set("sort", props.sort);
  if (props.includeNSFW) params.set("include_nsfw", "true");
  params.set("cursor", cursor);
  params.set("limit", String(props.initialPagination?.limit ?? 25));
  const endpoint = props.query ? "/search" : "/content";
  return `${getPublicAPIBaseURL()}${endpoint}?${params.toString()}`;
}

function mergeItems(current: PublicContentItem[], incoming: PublicContentItem[]) {
  const seen = new Set(current.map((item) => item.id));
  return current.concat(incoming.filter((item) => !seen.has(item.id)));
}

function catalogHref(category?: string) {
  const params = new URLSearchParams();
  if (props.query) params.set("q", props.query);
  if (props.categoryParamMode === "query" && category) params.set("category", category);
  if (props.activeTags.length) params.set("tags", props.activeTags.join(","));
  if (props.sort !== "newest") params.set("sort", props.sort);
  if (props.includeNSFW) params.set("include_nsfw", "true");
  const suffix = params.toString();
  const base = props.categoryParamMode === "path" && category ? `${props.baseCatalogPath}/${category}` : props.baseCatalogPath;
  return `${base}${suffix ? `?${suffix}` : ""}`;
}

function currentHref(overrides: Record<string, string>) {
  const params = new URLSearchParams();
  if (props.query) params.set("q", props.query);
  if (props.categoryParamMode === "query" && props.activeCategory) params.set("category", props.activeCategory);
  if (props.activeTags.length) params.set("tags", props.activeTags.join(","));
  if (props.sort !== "newest") params.set("sort", props.sort);
  if (props.includeNSFW) params.set("include_nsfw", "true");
  Object.entries(overrides).forEach(([key, value]) => {
    if (value) {
      params.set(key, value);
    } else {
      params.delete(key);
    }
  });
  const base = props.categoryParamMode === "path" && props.activeCategory ? `${props.baseCatalogPath}/${props.activeCategory}` : props.baseCatalogPath;
  const suffix = params.toString();
  return `${base}${suffix ? `?${suffix}` : ""}`;
}

function cardHref(item: PublicContentItem) {
  return buildContentPath(props.locale, item);
}

function mediaURL(imageID?: string | null) {
  return publicMediaURL(props.publicAPIBaseURL, imageID);
}

function authorName(item: PublicContentItem) {
  return item.author.display_name || item.author.username;
}

function authorInitial(item: PublicContentItem) {
  return authorName(item).slice(0, 1).toUpperCase();
}

function categoryIcon(slug: Category["slug"]): IconName {
  return slug;
}

function categoryLabel(category: Category) {
  return ui[props.locale].categories[category.slug] ?? category.name;
}

function iconPath(name: IconName) {
  const paths: Record<IconName, string> = {
    worlds: "M12 2a10 10 0 1 0 0 20a10 10 0 0 0 0-20Zm0 0c2.5 2.7 3.8 6 3.8 10S14.5 19.3 12 22M12 2C9.5 4.7 8.2 8 8.2 12s1.3 7.3 3.8 10M2 12h20",
    avatars: "M20 21a8 8 0 0 0-16 0M12 11a4 4 0 1 0 0-8a4 4 0 0 0 0 8Z",
    props: "M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z M3.3 7 12 12l8.7-5M12 22V12",
    prefabs: "M10 22V7a1 1 0 0 0-1-1H4a1 1 0 0 0-1 1v15M21 22V4a1 1 0 0 0-1-1h-5a1 1 0 0 0-1 1v18M14 11h7M3 11h7M3 16h7M14 16h7",
    download: "M12 3v12m0 0 4-4m-4 4-4-4M5 21h14",
    heart: "M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.6l-1-1a5.5 5.5 0 0 0-7.8 7.8l1 1L12 21l7.8-7.6l1-1a5.5 5.5 0 0 0 0-7.8Z",
    comment: "M21 15a4 4 0 0 1-4 4H8l-5 3V7a4 4 0 0 1 4-4h10a4 4 0 0 1 4 4Z",
  };
  return paths[name];
}

function backToTop() {
  const scroller = document.querySelector<HTMLElement>("[data-site-scroll]");
  if (scroller) {
    scroller.scrollTo({ top: 0, behavior: "smooth" });
    return;
  }
  window.scrollTo({ top: 0, behavior: "smooth" });
}
</script>

<template>
  <div class="catalog-shell">
    <div class="catalog-category-row" :aria-label="t.categoriesLabel">
      <nav class="catalog-tabs" :aria-label="t.categoriesLabel">
        <a class="catalog-tab" :class="{ 'catalog-tab--active': !activeCategory }" :href="catalogHref()">{{ ui[props.locale].common.all }}</a>
        <a
          v-for="category in categories"
          :key="category.slug"
          class="catalog-tab"
          :class="{ 'catalog-tab--active': activeCategory === category.slug }"
          :href="catalogHref(category.slug)"
        >
          {{ categoryLabel(category) }}
        </a>
      </nav>
    </div>
    <div v-if="error" class="catalog-empty" role="status">
      <div class="catalog-empty__title">
        <span aria-hidden="true"></span>
        <strong>{{ t.unavailableTitle }}</strong>
        <span aria-hidden="true"></span>
      </div>
      <p>{{ error }}</p>
    </div>
    <div v-else-if="items.length === 0" class="catalog-empty" role="status">
      <div class="catalog-empty__title">
        <span aria-hidden="true"></span>
        <strong>{{ t.emptyTitle }}</strong>
        <span aria-hidden="true"></span>
      </div>
      <p>{{ t.emptyCopy }}</p>
    </div>
    <div v-else class="asset-masonry asset-masonry--featured catalog-grid">
      <a
        v-for="(item, index) in items"
        :key="item.id"
        class="asset-card"
        :class="`asset-card--${item.category.slug.slice(0, -1) || 'world'}`"
        :href="cardHref(item)"
        :aria-label="item.title"
        :data-motion-card="index < initialItems.length ? '' : null"
      >
        <img v-if="item.preview_image_id" class="asset-card__image" :src="mediaURL(item.preview_image_id)" alt="" :loading="index < 5 ? 'eager' : 'lazy'" />
        <span v-else class="asset-card__placeholder" aria-hidden="true">
          <span>{{ item.category.name.slice(0, 2).toUpperCase() }}</span>
        </span>
        <span class="asset-card__shade" aria-hidden="true"></span>
        <span class="asset-card__top">
          <span class="asset-card__chip">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path :d="iconPath(categoryIcon(item.category.slug))" /></svg>
            <span>{{ categoryLabel(item.category) }}</span>
          </span>
          <span>.bee</span>
        </span>
        <span class="asset-card__body">
          <span class="asset-card__author">
            <span class="asset-card__avatar" aria-hidden="true">
              <img
                v-if="item.author.avatar_image_id"
                :src="mediaURL(item.author.avatar_image_id)"
                alt=""
                width="24"
                height="24"
                loading="lazy"
                decoding="async"
              />
              <template v-else>{{ authorInitial(item) }}</template>
            </span>
            <span>{{ authorName(item) }}</span>
          </span>
          <span class="asset-card__title">{{ item.title }}</span>
          <span class="asset-card__metrics" :aria-label="t.metricsLabel">
            <span :aria-label="`${item.downloads_count} ${t.downloadsLabel}`"><svg viewBox="0 0 24 24" aria-hidden="true"><path :d="iconPath('download')" /></svg>{{ item.downloads_count.toLocaleString() }}</span>
            <span :aria-label="`${item.likes_count} ${t.likesLabel}`"><svg viewBox="0 0 24 24" aria-hidden="true"><path :d="iconPath('heart')" /></svg>{{ item.likes_count.toLocaleString() }}</span>
            <span :aria-label="`${item.comments_count} ${t.commentsLabel}`"><svg viewBox="0 0 24 24" aria-hidden="true"><path :d="iconPath('comment')" /></svg>{{ item.comments_count.toLocaleString() }}</span>
          </span>
        </span>
      </a>
    </div>

    <div ref="sentinel" class="catalog-sentinel" aria-live="polite">
      <span v-if="loading">{{ t.loadingMore }}</span>
      <a v-else-if="nextCursor" class="button button--secondary" :href="currentHref({ cursor: nextCursor })" @click.prevent="loadMore">{{ t.loadMore }}</a>
      <span v-if="loadError" class="catalog-sentinel__error">{{ loadError }}</span>
    </div>
    <div v-if="!nextCursor && items.length && !loading" class="catalog-end">
      <div class="catalog-end__title">
        <span aria-hidden="true"></span>
        <strong>{{ t.endTitle }}</strong>
        <span aria-hidden="true"></span>
      </div>
      <p>{{ t.endCopy }}</p>
      <button type="button" @click="backToTop">{{ t.backToTop }}</button>
    </div>
  </div>
</template>
