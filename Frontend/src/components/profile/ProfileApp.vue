<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref, watch, type PropType } from "vue";
import { Boxes, ShieldCheck, UserRound } from "lucide";
import AccountSettingsPanel from "./AccountSettingsPanel.vue";
import OwnerContentDrawer from "./OwnerContentDrawer.vue";
import OwnerContentPanel from "./OwnerContentPanel.vue";
import SecurityPanel from "./SecurityPanel.vue";
import { readAuthSession } from "../../lib/auth/session";
import { navigateWithPageProgress } from "../../lib/ui/page-progress";
import { showToast } from "../../lib/ui/toast";
import { useOwnerStore } from "../../stores/owner.store";
import type { PublicUser } from "../../lib/api/types";

const props = defineProps<{
  loginHref: string;
  initialUser: PublicUser;
}>();

type ProfileTabID = "profile" | "content" | "security";
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
const ready = ref(false);
const activeTab = ref<ProfileTabID>("profile");
const isAllowed = computed(() => ready.value && owner.isAuthenticated);

const tabs = computed(() => [
  {
    id: "profile" as const,
    label: "Profile",
    description: "Public identity and avatar",
    metric: owner.user?.avatar_image_id ? "Avatar set" : "No avatar",
    icon: UserRound as IconNode,
  },
  {
    id: "content" as const,
    label: "Content",
    description: "Packages and listing metadata",
    metric: `${owner.items.length} items`,
    icon: Boxes as IconNode,
  },
  {
    id: "security" as const,
    label: "Security",
    description: "Email and password",
    metric: owner.user?.email_verified_at ? "Verified" : "Pending",
    icon: ShieldCheck as IconNode,
  },
]);

function selectAdjacentTab(direction: 1 | -1) {
  const tabIDs = tabs.value.map((tab) => tab.id);
  const currentIndex = tabIDs.indexOf(activeTab.value);
  const nextIndex = (currentIndex + direction + tabIDs.length) % tabIDs.length;
  activeTab.value = tabIDs[nextIndex];
}

onMounted(() => {
  const session = readAuthSession();
  if (!session && !props.initialUser?.username) {
    navigateWithPageProgress(props.loginHref, "replace");
    return;
  }
  owner.bootstrapUser(props.initialUser);
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
    <span>Checking session...</span>
  </div>

  <div v-else-if="!isAllowed" class="profile-access-state" aria-live="polite">
    <span>Redirecting to sign in...</span>
  </div>

  <div v-else class="profile-shell">
    <nav class="profile-tabs" role="tablist" aria-label="Profile sections">
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
        @click="activeTab = tab.id"
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
        <AccountSettingsPanel />
      </div>
      <div v-else-if="activeTab === 'content'" class="profile-tab-panel__grid">
        <OwnerContentPanel />
        <OwnerContentDrawer />
      </div>
      <div v-else class="profile-tab-panel__grid">
        <SecurityPanel />
      </div>
    </section>
  </div>
</template>
