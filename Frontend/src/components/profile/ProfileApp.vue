<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref, watch, type PropType } from "vue";
import { Boxes, RadioTower, ShieldCheck, UserRound } from "lucide";
import AccountSettingsPanel from "./AccountSettingsPanel.vue";
import OwnerContentDrawer from "./OwnerContentDrawer.vue";
import OwnerContentPanel from "./OwnerContentPanel.vue";
import OwnerServersPanel from "./OwnerServersPanel.vue";
import SecurityPanel from "./SecurityPanel.vue";
import { readAuthSession } from "../../lib/auth/session";
import { ui, type Locale } from "../../lib/i18n";
import { navigateWithPageProgress } from "../../lib/ui/page-progress";
import { showToast } from "../../lib/ui/toast";
import { useOwnerStore } from "../../stores/owner.store";
import type { PublicUser } from "../../lib/api/types";

const props = defineProps<{
  loginHref: string;
  initialUser: PublicUser;
  locale?: Locale;
}>();

type ProfileTabID = "profile" | "content" | "servers" | "security";
type IconNode = Array<[string, Record<string, string>]>;

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

const owner = useOwnerStore();
const locale = props.locale ?? "en";
const t = ui[locale].profile;
const ready = ref(false);
const activeTab = ref<ProfileTabID>("profile");
const isAllowed = computed(() => ready.value && owner.isAuthenticated);

const tabs = computed(() => [
  {
    id: "profile" as const,
    label: t.tabProfile,
    description: t.tabProfileDescription,
    metric: owner.user?.avatar_image_id ? t.avatarSet : t.noAvatar,
    icon: UserRound as IconNode,
  },
  {
    id: "content" as const,
    label: t.tabContent,
    description: t.tabContentDescription,
    metric: `${owner.items.length} ${t.itemsSuffix}`,
    icon: Boxes as IconNode,
  },
  {
    id: "servers" as const,
    label: t.tabServers,
    description: t.tabServersDescription,
    metric: `${owner.serverItems.length} ${t.serversSuffix}`,
    icon: RadioTower as IconNode,
  },
  {
    id: "security" as const,
    label: t.tabSecurity,
    description: t.tabSecurityDescription,
    metric: owner.user?.email_verified_at ? t.verified : t.pending,
    icon: ShieldCheck as IconNode,
  },
]);

function selectAdjacentTab(direction: 1 | -1) {
  const tabIDs = tabs.value.map((tab) => tab.id);
  const currentIndex = tabIDs.indexOf(activeTab.value);
  const nextIndex = (currentIndex + direction + tabIDs.length) % tabIDs.length;
  setActiveTab(tabIDs[nextIndex]);
}

function setActiveTab(tabID: ProfileTabID) {
  activeTab.value = tabID;
  if (typeof window !== "undefined") {
    window.history.replaceState(null, "", `#${tabID}`);
  }
}

function hashTab(): ProfileTabID | null {
  if (typeof window === "undefined") return null;
  const value = window.location.hash.replace(/^#/, "");
  return tabs.value.some((tab) => tab.id === value) ? value as ProfileTabID : null;
}

onMounted(() => {
  const session = readAuthSession();
  if (!session && !props.initialUser?.username) {
    navigateWithPageProgress(props.loginHref, "replace");
    return;
  }
  owner.bootstrapUser(props.initialUser);
  activeTab.value = hashTab() ?? activeTab.value;
  ready.value = true;
});

watch(
  () => owner.selectedContentID,
  async (contentID) => {
    if (contentID) {
      await owner.refreshImages(contentID);
    }
  },
);

watch(
  () => owner.notice,
  (message) => {
    if (!message) return;
    showToast(message, message.includes("will appear after processing") ? "info" : "success");
    owner.notice = "";
  },
);

watch(
  () => owner.error,
  (message) => {
    if (!message) return;
    showToast(message, "error");
    owner.error = "";
  },
);
</script>

<template>
  <div v-if="!ready" class="profile-access-state" aria-live="polite">
    <span>{{ t.checkingSession }}</span>
  </div>

  <div v-else-if="!isAllowed" class="profile-access-state" aria-live="polite">
    <span>{{ t.redirecting }}</span>
  </div>

  <div v-else class="profile-shell">
    <nav class="profile-tabs" role="tablist" :aria-label="t.tabsLabel">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        class="profile-tab"
        :class="{ 'profile-tab--active': activeTab === tab.id }"
        type="button"
        role="tab"
        :id="`profile-tab-${tab.id}`"
        :aria-selected="activeTab === tab.id"
        :aria-controls="`profile-panel-${tab.id}`"
        :tabindex="activeTab === tab.id ? 0 : -1"
        @click="setActiveTab(tab.id)"
        @keydown.left.prevent="selectAdjacentTab(-1)"
        @keydown.up.prevent="selectAdjacentTab(-1)"
        @keydown.right.prevent="selectAdjacentTab(1)"
        @keydown.down.prevent="selectAdjacentTab(1)"
      >
        <span class="profile-tab__label">
          <Icon :node="tab.icon" />
          <span>{{ tab.label }}</span>
        </span>
        <small>{{ tab.description }}</small>
        <em>{{ tab.metric }}</em>
      </button>
    </nav>

    <section
      class="profile-tab-panel"
      role="tabpanel"
      :id="`profile-panel-${activeTab}`"
      :aria-labelledby="`profile-tab-${activeTab}`"
      aria-live="polite"
    >
      <div v-if="activeTab === 'profile'" class="profile-tab-panel__grid">
        <AccountSettingsPanel :locale="locale" />
      </div>
      <div v-else-if="activeTab === 'content'" class="profile-tab-panel__grid">
        <OwnerContentPanel :locale="locale" />
        <OwnerContentDrawer :locale="locale" />
      </div>
      <div v-else-if="activeTab === 'servers'" class="profile-tab-panel__grid">
        <OwnerServersPanel :locale="locale" />
      </div>
      <div v-else class="profile-tab-panel__grid">
        <SecurityPanel :locale="locale" />
      </div>
    </section>
  </div>
</template>
