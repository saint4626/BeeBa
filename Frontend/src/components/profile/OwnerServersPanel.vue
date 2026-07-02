<script setup lang="ts">
import { ChevronDown } from "lucide";
import { computed, defineComponent, h, onBeforeUnmount, onMounted, reactive, ref, type PropType, watch } from "vue";
import { ui, type Locale } from "../../lib/i18n";
import { normalizeServerLanguageValue, serverLanguageLabel, serverLanguageOptions } from "../../lib/servers/languages";
import { showToast } from "../../lib/ui/toast";
import { useOwnerStore } from "../../stores/owner.store";
import type { OwnerServerCreateInput, OwnerServerItem, OwnerServerProbeResult } from "../../lib/api/types";

type IconNode = Array<[string, Record<string, string>]>;

const Icon = defineComponent({
  name: "OwnerServerInlineIcon",
  props: {
    node: { type: Array as PropType<IconNode>, required: true },
    size: { type: Number, default: 15 },
  },
  setup(iconProps) {
    return () => h("svg", {
      viewBox: "0 0 24 24",
      width: iconProps.size,
      height: iconProps.size,
      fill: "none",
      stroke: "currentColor",
      "stroke-width": 2,
      "stroke-linecap": "round",
      "stroke-linejoin": "round",
      "aria-hidden": "true",
    }, iconProps.node.map(([tag, attrs]) => h(tag, attrs)));
  },
});

const props = withDefaults(defineProps<{ locale?: Locale }>(), { locale: "en" });
const t = ui[props.locale].ownerServers;
const serverT = ui[props.locale].servers;
const common = ui[props.locale].common;
const owner = useOwnerStore();
const probing = ref(false);
const ChevronDownIcon = ChevronDown as IconNode;

const form = reactive({
  name: "",
  connection: "",
  description: "",
  visibility: "public" as "public" | "unlisted",
  region: "",
  language: "",
  tags: "",
  rules: "",
  discordURL: "",
  websiteURL: "",
});

const selected = computed(() => owner.selectedServer);
const mode = computed(() => selected.value ? "edit" : "create");
const visibilityDropdown = ref<HTMLDetailsElement | null>(null);
const languageDropdown = ref<HTMLDetailsElement | null>(null);
const selectedVisibilityLabel = computed(() => form.visibility === "unlisted" ? t.unlistedVisibility : t.publicVisibility);
const selectedLanguageLabel = computed(() => serverLanguageLabel(form.language, props.locale, serverT.anyLanguage));
const languageOptions = computed(() => serverLanguageOptions(props.locale, serverT.anyLanguage, form.language));

watch(selected, (server) => {
  if (!server) {
    resetForm();
    return;
  }
  form.name = server.name;
  form.connection = server.connection_string;
  form.description = server.description;
  form.visibility = server.visibility;
  form.region = server.region;
  form.language = server.language;
  form.tags = server.tags.map((tag) => tag.name).join(", ");
  form.rules = server.rules;
  form.discordURL = server.discord_url ?? "";
  form.websiteURL = server.website_url ?? "";
}, { immediate: true });

onMounted(() => {
  document.addEventListener("click", closeDropdownsFromOutside);
  window.addEventListener("scroll", closeServerDropdowns, { passive: true });
  document.querySelector<HTMLElement>("[data-site-scroll]")?.addEventListener("scroll", closeServerDropdowns, { passive: true });
});

onBeforeUnmount(() => {
  document.removeEventListener("click", closeDropdownsFromOutside);
  window.removeEventListener("scroll", closeServerDropdowns);
  document.querySelector<HTMLElement>("[data-site-scroll]")?.removeEventListener("scroll", closeServerDropdowns);
});

async function submit() {
  let input = formInput();
  if (!input.connection_string) {
    showToast(t.connectionRequired, "error");
    return;
  }
  if (!input.name) {
    const probe = await probeConnection({ quietSuccess: true });
    if (!probe) {
      return;
    }
    if (!probe?.server_name?.trim()) {
      showToast(t.nameRequired, "error");
      return;
    }
    input = formInput();
  }

  try {
    if (selected.value) {
      await owner.updateSelectedServer(input);
      return;
    }
    await owner.createServer(input);
  } catch {
    // The shared ProfileApp watcher displays owner.error.
  }
}

async function probeConnection(options: { quietSuccess?: boolean } = {}): Promise<OwnerServerProbeResult | null> {
  const connection = form.connection.trim();
  if (!connection) {
    showToast(t.connectionRequired, "error");
    return null;
  }

  probing.value = true;
  try {
    const result = await owner.probeServerConnection(connection);
    const serverName = result?.server_name?.trim() ?? "";
    if (serverName && !form.name.trim()) {
      form.name = serverName;
    }
    if (!options.quietSuccess) {
      showToast(serverName ? t.probeLoaded : t.probeNoName, serverName ? "success" : "info");
    }
    return result ?? null;
  } catch (caught) {
    const message = caught instanceof Error && caught.message ? caught.message : t.probeFailed;
    showToast(message, "error");
    return null;
  } finally {
    probing.value = false;
  }
}

async function verify(server: OwnerServerItem) {
  try {
    await owner.verifyServer(server.id);
  } catch {
    // The shared ProfileApp watcher displays owner.error.
  }
}

async function remove(server: OwnerServerItem) {
  if (!window.confirm(`${t.delete}: ${server.name}?`)) return;
  try {
    await owner.deleteServer(server.id);
  } catch {
    // The shared ProfileApp watcher displays owner.error.
  }
}

async function copyConnection(server: OwnerServerItem) {
  try {
    await navigator.clipboard.writeText(server.connection_string);
    showToast(t.connectionCopied, "success");
  } catch {
    showToast(common.copyFailed, "error");
  }
}

function formInput(): OwnerServerCreateInput & { connection_string: string } {
  return {
    name: form.name.trim(),
    connection_string: form.connection.trim(),
    description: form.description.trim(),
    visibility: form.visibility,
    region: form.region.trim(),
    language: normalizeServerLanguageValue(form.language),
    tags: form.tags.split(",").map((tag) => tag.trim()).filter(Boolean),
    rules: form.rules.trim(),
    discord_url: form.discordURL.trim(),
    website_url: form.websiteURL.trim(),
  };
}

function resetForm() {
  form.name = "";
  form.connection = "";
  form.description = "";
  form.visibility = "public";
  form.region = "";
  form.language = "";
  form.tags = "";
  form.rules = "";
  form.discordURL = "";
  form.websiteURL = "";
}

function clearSelection() {
  owner.selectedServerID = "";
}

function selectVisibility(value: "public" | "unlisted") {
  form.visibility = value;
  visibilityDropdown.value?.removeAttribute("open");
}

function selectLanguage(value: string) {
  form.language = value;
  languageDropdown.value?.removeAttribute("open");
}

function onDropdownToggle(active: "visibility" | "language") {
  const current = active === "visibility" ? visibilityDropdown.value : languageDropdown.value;
  const other = active === "visibility" ? languageDropdown.value : visibilityDropdown.value;
  if (current?.open) {
    other?.removeAttribute("open");
  }
}

function closeServerDropdowns() {
  visibilityDropdown.value?.removeAttribute("open");
  languageDropdown.value?.removeAttribute("open");
}

function closeDropdownsFromOutside(event: MouseEvent) {
  const target = event.target;
  if (!(target instanceof Node)) return;
  if (visibilityDropdown.value?.contains(target) || languageDropdown.value?.contains(target)) {
    return;
  }
  closeServerDropdowns();
}

function statusLabel(value: string) {
  return value.replaceAll("_", " ");
}

function playerLabel(server: OwnerServerItem) {
  if (typeof server.check.online_players === "number" && typeof server.check.max_players === "number") {
    return `${server.check.online_players}/${server.check.max_players}`;
  }
  return statusLabel(server.check.status);
}
</script>

<template>
  <section class="panel owner-servers-panel" aria-labelledby="owner-servers-title">
    <div class="panel__head panel__head--row">
      <div>
        <p class="eyebrow">{{ t.eyebrow }}</p>
        <h2 id="owner-servers-title">{{ t.title }}</h2>
        <p>{{ t.lead }}</p>
      </div>
      <button class="button button--secondary" type="button" :disabled="owner.loading" @click="owner.refreshLibrary">
        {{ t.refresh }}
      </button>
    </div>

    <div class="owner-servers-layout">
      <div class="owner-servers-list">
        <div v-if="owner.serverItems.length === 0" class="empty-state">
          <span class="empty-state__badge">{{ t.noServers }}</span>
          <div>
            <h2>{{ t.createTitle }}</h2>
            <p>{{ t.noServersCopy }}</p>
          </div>
        </div>
        <article
          v-for="server in owner.serverItems"
          :key="server.id"
          class="owner-server-card"
          :class="{ 'owner-server-card--active': owner.selectedServerID === server.id }"
        >
          <button class="owner-server-card__main" type="button" @click="owner.selectServer(server.id)">
            <span class="server-card__status" :data-status="server.check.status">{{ server.check.status === 'online' ? ui[props.locale].servers.online : ui[props.locale].servers.offline }}</span>
            <strong>{{ server.name }}</strong>
            <span>{{ server.host }}:{{ server.port }}</span>
            <span class="owner-server-card__meta">
              <em>{{ statusLabel(server.status) }}</em>
              <em>{{ playerLabel(server) }}</em>
              <em>{{ server.visibility }}</em>
            </span>
          </button>
          <div class="owner-server-card__actions">
            <button class="button button--secondary" type="button" :disabled="owner.loading" @click="copyConnection(server)">
              {{ t.copyConnection }}
            </button>
            <button class="button button--secondary" type="button" :disabled="owner.loading" @click="verify(server)">
              {{ t.verify }}
            </button>
            <button class="button button--secondary owner-server-card__danger" type="button" :disabled="owner.loading" @click="remove(server)">
              {{ t.delete }}
            </button>
          </div>
        </article>
      </div>

      <form class="owner-server-form" @submit.prevent="submit">
        <div class="owner-server-form__head">
          <p class="eyebrow">{{ mode === "edit" ? t.editTitle : t.createTitle }}</p>
          <button v-if="selected" class="button button--secondary" type="button" @click="clearSelection">
            {{ t.cancelEdit }}
          </button>
        </div>

        <label>
          <span>{{ t.name }} <span class="owner-server-form__required" aria-hidden="true">*</span></span>
          <input v-model="form.name" type="text" maxlength="80" aria-required="true" />
        </label>
        <label>
          <span>{{ t.connection }} <span class="owner-server-form__required" aria-hidden="true">*</span></span>
          <div class="owner-server-form__connection-row">
            <input v-model="form.connection" type="text" :placeholder="t.connectionHint" aria-required="true" />
            <button class="button button--secondary" type="button" :disabled="owner.loading || probing || !form.connection.trim()" @click="probeConnection()">
              {{ probing ? t.probing : t.probe }}
            </button>
          </div>
        </label>
        <label>
          <span>{{ t.description }}</span>
          <textarea v-model="form.description" rows="4" maxlength="1000"></textarea>
        </label>

        <div class="owner-server-form__grid">
          <div class="owner-server-form__field">
            <span>{{ t.visibility }}</span>
            <details ref="visibilityDropdown" class="ui-dropdown ui-dropdown--start owner-server-form__dropdown" data-ui-dropdown @toggle="onDropdownToggle('visibility')">
              <summary :aria-label="t.visibility">
                <span>{{ selectedVisibilityLabel }}</span>
                <Icon :node="ChevronDownIcon" :size="15" />
              </summary>
              <div class="ui-dropdown__menu">
                <button
                  class="ui-dropdown__item"
                  :class="{ 'ui-dropdown__item--active': form.visibility === 'public' }"
                  type="button"
                  :aria-pressed="form.visibility === 'public'"
                  @click="selectVisibility('public')"
                >
                  {{ t.publicVisibility }}
                </button>
                <button
                  class="ui-dropdown__item"
                  :class="{ 'ui-dropdown__item--active': form.visibility === 'unlisted' }"
                  type="button"
                  :aria-pressed="form.visibility === 'unlisted'"
                  @click="selectVisibility('unlisted')"
                >
                  {{ t.unlistedVisibility }}
                </button>
              </div>
            </details>
          </div>
          <label>
            <span>{{ t.region }}</span>
            <input v-model="form.region" type="text" maxlength="64" :placeholder="t.regionHint" />
          </label>
          <div class="owner-server-form__field owner-server-form__language">
            <span>{{ t.language }}</span>
            <details ref="languageDropdown" class="ui-dropdown ui-dropdown--start owner-server-form__dropdown" data-ui-dropdown @toggle="onDropdownToggle('language')">
              <summary :aria-label="t.language">
                <span>{{ selectedLanguageLabel }}</span>
                <Icon :node="ChevronDownIcon" :size="15" />
              </summary>
              <div class="ui-dropdown__menu">
                <button
                  v-for="option in languageOptions"
                  :key="option.value || 'any'"
                  class="ui-dropdown__item"
                  :class="{ 'ui-dropdown__item--active': normalizeServerLanguageValue(form.language) === option.value }"
                  type="button"
                  :aria-pressed="normalizeServerLanguageValue(form.language) === option.value"
                  @click="selectLanguage(option.value)"
                >
                  {{ option.label }}
                </button>
              </div>
            </details>
          </div>
        </div>

        <label>
          <span>{{ t.tags }}</span>
          <input v-model="form.tags" type="text" :placeholder="t.tagsPlaceholder" />
        </label>
        <label>
          <span>{{ t.rules }}</span>
          <textarea v-model="form.rules" rows="3" maxlength="1000"></textarea>
        </label>

        <div class="owner-server-form__grid">
          <label>
            <span>{{ t.discord }}</span>
            <input v-model="form.discordURL" type="url" inputmode="url" />
          </label>
          <label>
            <span>{{ t.website }}</span>
            <input v-model="form.websiteURL" type="url" inputmode="url" />
          </label>
        </div>

        <button class="button button--primary" type="submit" :disabled="owner.loading || probing">
          {{ owner.loading || probing ? common.saving : (mode === "edit" ? t.save : t.create) }}
        </button>
      </form>
    </div>
  </section>
</template>
