<script setup lang="ts">
import { computed, defineComponent, h, onBeforeUnmount, ref, watch, type PropType } from "vue";
import { ImagePlus } from "lucide";
import { UPLOAD_LIMITS, megabytesFromBytes } from "../../lib/config/runtime";
import { showToast } from "../../lib/ui/toast";
import { useOwnerStore } from "../../stores/owner.store";

type IconNode = Array<[string, Record<string, string>]>;

const Icon = defineComponent({
  name: "LucideInlineIcon",
  props: {
    node: { type: Array as PropType<IconNode>, required: true },
    size: { type: Number, default: 18 },
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
const maxAvatarBytes = UPLOAD_LIMITS.maxImageBytes;
const maxAvatarMegabytes = megabytesFromBytes(maxAvatarBytes);

const owner = useOwnerStore();
const loading = ref(false);
const avatarLoadFailed = ref(false);
const localPreviewURL = ref("");

const avatarURL = computed(() => {
  if (!owner.user?.avatar_image_id || avatarLoadFailed.value) return "";
  return `/api/v1/media/${owner.user.avatar_image_id}`;
});
const visibleAvatarURL = computed(() => localPreviewURL.value || avatarURL.value);
const initials = computed(() => owner.user?.username?.slice(0, 2).toUpperCase() || "BB");

watch(
  () => owner.user?.avatar_image_id,
  () => {
    avatarLoadFailed.value = false;
    clearLocalPreview();
  },
);

onBeforeUnmount(clearLocalPreview);

async function onAvatarChange(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0] ?? null;
  input.value = "";
  if (!file) {
    return;
  }
  const validationError = validateAvatar(file);
  if (validationError) {
    showToast(validationError, "error");
    return;
  }

  setLocalPreview(file);
  loading.value = true;
  try {
    await owner.setAvatar(file);
  } catch (caught) {
    clearLocalPreview();
    showToast(caught instanceof Error ? caught.message : "Avatar upload failed.", "error");
  } finally {
    loading.value = false;
  }
}

function validateAvatar(file: File) {
  if (file.size <= 0) {
    return "Avatar image must not be empty.";
  }
  if (file.size > maxAvatarBytes) {
    return `Avatar image must be ${maxAvatarMegabytes} MB or smaller.`;
  }
  if (file.type !== "image/png" && file.type !== "image/jpeg") {
    return "Choose a PNG or JPEG avatar.";
  }
  return "";
}

function setLocalPreview(file: File) {
  clearLocalPreview();
  localPreviewURL.value = URL.createObjectURL(file);
}

function clearLocalPreview() {
  if (localPreviewURL.value) {
    URL.revokeObjectURL(localPreviewURL.value);
    localPreviewURL.value = "";
  }
}
</script>

<template>
  <div class="profile-avatar-panel" aria-labelledby="avatar-title">
    <div class="profile-avatar-panel__head">
      <h3 id="avatar-title">Avatar</h3>
      <p>Click the avatar to replace it. PNG or JPEG up to {{ maxAvatarMegabytes }} MB.</p>
    </div>

    <label
      class="avatar-uploader"
      :class="{ 'avatar-uploader--loading': loading, 'avatar-uploader--disabled': !owner.isAuthenticated }"
      aria-label="Upload new avatar"
    >
      <input type="file" accept="image/png,image/jpeg" :disabled="loading || !owner.isAuthenticated" @change="onAvatarChange" />
      <span class="avatar-preview avatar-preview--interactive">
        <img v-if="visibleAvatarURL" :src="visibleAvatarURL" alt="" @error="avatarLoadFailed = true" />
        <span v-else>{{ initials }}</span>
        <span class="avatar-uploader__overlay" aria-hidden="true">
          <Icon :node="ImagePlusIcon" />
          <span>{{ loading ? "Uploading" : "Change" }}</span>
        </span>
      </span>
    </label>

    <p class="profile-avatar-panel__hint">
      New uploads replace the current avatar. Old avatar media is removed after the server accepts the new file.
    </p>
  </div>
</template>
