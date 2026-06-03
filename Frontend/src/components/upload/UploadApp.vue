<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref, type PropType } from "vue";
import { CheckCircle2, ChevronDown, Eye, FileArchive, ImagePlus, KeyRound, Lock, UploadCloud } from "lucide";
import { readAuthSession } from "../../lib/auth/session";
import { uploadContentImage } from "../../lib/api/media";
import { uploadContentPackage } from "../../lib/api/uploads";
import { BYTE_UNITS, UPLOAD_LIMITS, megabytesFromBytes } from "../../lib/config/runtime";
import type { ContentUploadCreated, PublicUser } from "../../lib/api/types";
import { showToast } from "../../lib/ui/toast";

type IconNode = Array<[string, Record<string, string>]>;

const props = withDefaults(
  defineProps<{
    initialUser?: PublicUser | null;
    loginHref?: string;
  }>(),
  {
    initialUser: null,
    loginHref: "/login?next=%2Fupload",
  },
);

const Icon = defineComponent({
  name: "LucideInlineIcon",
  props: {
    node: { type: Array as PropType<IconNode>, required: true },
    size: { type: Number, default: 16 },
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

const UploadCloudIcon = UploadCloud as IconNode;
const FileArchiveIcon = FileArchive as IconNode;
const ImagePlusIcon = ImagePlus as IconNode;
const EyeIcon = Eye as IconNode;
const LockIcon = Lock as IconNode;
const KeyRoundIcon = KeyRound as IconNode;
const CheckIcon = CheckCircle2 as IconNode;
const ChevronDownIcon = ChevronDown as IconNode;

const maxPreviewBytes = UPLOAD_LIMITS.maxImageBytes;
const maxPreviewMegabytes = megabytesFromBytes(maxPreviewBytes);
const categories = [
  { value: "worlds", label: "Worlds" },
  { value: "avatars", label: "Avatars" },
  { value: "props", label: "Props" },
  { value: "prefabs", label: "Prefabs" },
] as const;

const currentUser = ref<PublicUser | null>(null);
const category = ref("worlds");
const categoryDropdown = ref<HTMLDetailsElement | null>(null);
const title = ref("");
const description = ref("");
const visibility = ref<"public" | "private">("public");
const unlockPassword = ref("");
const nsfw = ref(false);
const selectedFile = ref<File | null>(null);
const previewFile = ref<File | null>(null);
const lastUpload = ref<ContentUploadCreated | null>(null);
const uploadLoading = ref(false);
const uploadProgress = ref(0);
const uploadFormElement = ref<HTMLFormElement | null>(null);

const isAuthenticated = computed(() => Boolean(currentUser.value));
const selectedFileName = computed(() => selectedFile.value?.name ?? "Choose .bee package");
const previewFileName = computed(() => previewFile.value?.name ?? "Optional PNG/JPEG preview");
const selectedFileSize = computed(() => selectedFile.value ? formatBytes(selectedFile.value.size) : "");
const previewFileSize = computed(() => previewFile.value ? formatBytes(previewFile.value.size) : "");
const loginHref = computed(() => props.loginHref);
const selectedCategoryLabel = computed(() => categories.find((item) => item.value === category.value)?.label ?? "Worlds");
const uploadButtonLabel = computed(() => {
  if (!uploadLoading.value) return "Upload to quarantine";
  if (uploadProgress.value >= 100 && previewFile.value) return "Queuing preview...";
  return `Uploading ${uploadProgress.value}%`;
});
const uploadProgressStyle = computed(() => ({
  "--upload-progress": `${uploadLoading.value ? uploadProgress.value : 0}%`,
}));

onMounted(() => {
  currentUser.value = props.initialUser;
  const session = readAuthSession();
  if (session) {
    currentUser.value = session.user;
  }
});

function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement;
  selectedFile.value = input.files?.[0] ?? null;
}

function onPreviewChange(event: Event) {
  const input = event.target as HTMLInputElement;
  previewFile.value = input.files?.[0] ?? null;
}

function selectCategory(value: string) {
  category.value = value;
  categoryDropdown.value?.removeAttribute("open");
}

async function submitUpload() {
  if (!isAuthenticated.value) {
    showToast("Sign in before uploading content.", "error");
    return;
  }

  const validationError = validateUpload();
  if (validationError) {
    showToast(validationError, "error");
    return;
  }

  uploadLoading.value = true;
  uploadProgress.value = 0;
  try {
    const created = await uploadContentPackage({
      accessToken: "",
      category: category.value,
      title: title.value.trim(),
      description: description.value.trim(),
      visibility: visibility.value,
      unlockPassword: unlockPassword.value,
      nsfw: nsfw.value,
      file: selectedFile.value as File,
      onProgress: ({ percent }) => {
        uploadProgress.value = percent;
      },
    });
    uploadProgress.value = 100;
    lastUpload.value = created;

    if (previewFile.value) {
      await uploadContentImage("", created.content_id, previewFile.value, {
        isPrimary: true,
        sortOrder: 0,
      });
      showToast("Package and primary preview are queued.", "success");
    } else {
      showToast("Package is queued for scan.", "success");
    }

    resetUploadForm();
  } catch (caught) {
    showToast(caught instanceof Error ? caught.message : "Upload failed.", "error");
  } finally {
    uploadLoading.value = false;
    uploadProgress.value = 0;
  }
}

function validateUpload() {
  if (!selectedFile.value) {
    return "Choose a .bee file.";
  }
  if (!selectedFile.value.name.toLowerCase().endsWith(".bee")) {
    return "Only .bee files are accepted.";
  }
  if (!title.value.trim()) {
    return "Title is required.";
  }
  if (!unlockPassword.value.trim()) {
    return "Basis unlock password is required.";
  }
  if (previewFile.value) {
    if (previewFile.value.size > maxPreviewBytes) {
      return `Preview image must be ${maxPreviewMegabytes} MB or smaller.`;
    }
    if (previewFile.value.type !== "image/png" && previewFile.value.type !== "image/jpeg") {
      return "Preview must be a PNG or JPEG image.";
    }
  }
  return "";
}

function resetUploadForm() {
  uploadFormElement.value?.reset();
  title.value = "";
  description.value = "";
  unlockPassword.value = "";
  nsfw.value = false;
  selectedFile.value = null;
  previewFile.value = null;
}

function formatBytes(value: number) {
  if (value < BYTE_UNITS.kib) return `${value} B`;
  if (value < BYTE_UNITS.mib) return `${(value / BYTE_UNITS.kib).toFixed(1)} KB`;
  if (value < BYTE_UNITS.gib) return `${(value / BYTE_UNITS.mib).toFixed(1)} MB`;
  return `${(value / BYTE_UNITS.gib).toFixed(2)} GB`;
}
</script>

<template>
  <section v-if="!isAuthenticated" class="upload-gate" aria-labelledby="upload-gate-title">
    <div>
      <p class="eyebrow">Authentication required</p>
      <h2 id="upload-gate-title">Sign in to upload Basis assets.</h2>
      <p>Package uploads, preview images, and owner library actions are available only after authentication.</p>
    </div>
    <a class="button button--primary" :href="loginHref">Sign in</a>
  </section>

  <div v-else class="upload-workspace">
    <section class="panel upload-form-panel" aria-labelledby="upload-form-title">
      <div class="panel__head">
        <div>
          <p class="eyebrow">Package upload</p>
          <h2 id="upload-form-title">New Basis asset</h2>
          <p>Submit the `.bee` file first. The backend stores it in quarantine, then worker scan and moderation decide publication.</p>
        </div>
      </div>

      <form ref="uploadFormElement" class="upload-form" :aria-busy="uploadLoading" @submit.prevent="submitUpload">
        <div class="upload-form__files">
          <label class="upload-drop upload-drop--package">
            <input type="file" accept=".bee" @change="onFileChange" />
            <Icon :node="FileArchiveIcon" :size="28" />
            <strong>{{ selectedFileName }}</strong>
            <span>{{ selectedFileSize || ".bee packages only" }}</span>
          </label>

          <label class="upload-drop">
            <input type="file" accept="image/png,image/jpeg" @change="onPreviewChange" />
            <Icon :node="ImagePlusIcon" :size="28" />
            <strong>{{ previewFileName }}</strong>
            <span>{{ previewFileSize || `Primary card preview, up to ${maxPreviewMegabytes} MB` }}</span>
          </label>
        </div>

        <div class="upload-form__grid">
          <label class="field">
            Title
            <input v-model="title" type="text" maxlength="160" required />
          </label>
          <label class="field">
            Category
            <details ref="categoryDropdown" class="ui-dropdown ui-dropdown--start upload-category-dropdown">
              <summary aria-label="Choose content category">
                <span>{{ selectedCategoryLabel }}</span>
                <Icon :node="ChevronDownIcon" :size="15" />
              </summary>
              <div class="ui-dropdown__menu">
                <button
                  v-for="item in categories"
                  :key="item.value"
                  class="ui-dropdown__item"
                  :class="{ 'ui-dropdown__item--active': category === item.value }"
                  type="button"
                  :aria-pressed="category === item.value"
                  @click="selectCategory(item.value)"
                >
                  {{ item.label }}
                </button>
              </div>
            </details>
          </label>
        </div>

        <label class="field">
          Description
          <textarea v-model="description" maxlength="5000" rows="5" />
        </label>

        <div class="upload-options">
          <label class="upload-option" :class="{ 'upload-option--active': visibility === 'public' }">
            <input v-model="visibility" type="radio" value="public" />
            <Icon :node="EyeIcon" />
            <span>
              <strong>Public catalog</strong>
              <small>Password is shown on public cards after moderation.</small>
            </span>
          </label>
          <label class="upload-option" :class="{ 'upload-option--active': visibility === 'private' }">
            <input v-model="visibility" type="radio" value="private" />
            <Icon :node="LockIcon" />
            <span>
              <strong>Private library</strong>
              <small>Hidden from the catalog unless you share the link.</small>
            </span>
          </label>
        </div>

        <div class="upload-form__grid upload-form__grid--compact">
          <label class="field">
            Basis unlock password
            <span class="field-control-icon">
              <Icon :node="KeyRoundIcon" />
              <input v-model="unlockPassword" type="text" maxlength="256" required />
            </span>
          </label>
          <label class="upload-nsfw">
            <input v-model="nsfw" type="checkbox" />
            <span>NSFW</span>
          </label>
        </div>

        <button
          class="button button--primary upload-submit"
          type="submit"
          :class="{ 'upload-submit--progress': uploadLoading }"
          :disabled="uploadLoading"
          :style="uploadProgressStyle"
        >
          <span class="upload-submit__progress" aria-hidden="true"></span>
          <span class="upload-submit__label">
            <Icon :node="UploadCloudIcon" />
            {{ uploadButtonLabel }}
          </span>
        </button>
      </form>
    </section>

    <aside class="upload-side">
      <section v-if="lastUpload" class="panel upload-complete-card" aria-labelledby="last-upload-title">
        <Icon :node="CheckIcon" :size="22" />
        <div>
          <p class="eyebrow">Last upload</p>
          <h2 id="last-upload-title">{{ lastUpload.status }}</h2>
          <p>{{ lastUpload.original_filename }} / {{ formatBytes(lastUpload.file_size) }}</p>
          <code>{{ lastUpload.file_hash_sha256 }}</code>
        </div>
      </section>
    </aside>
  </div>
</template>
