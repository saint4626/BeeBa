import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { changeEmail, changePassword, getCurrentUser, resendEmailVerification, updateProfile } from "../lib/api/auth";
import { clearAuthSession, publishSessionUser, readAuthSession } from "../lib/auth/session";
import { listContentImages, updateContentImage, deleteContentImage, uploadAvatar, uploadContentImage } from "../lib/api/media";
import { deleteOwnedContent, listOwnedContent, updateOwnedContent } from "../lib/api/uploads";
import { createOwnedServer, deleteOwnedServer, listOwnedServers, probeOwnedServerConnection, requestOwnedServerVerification, updateOwnedServer } from "../lib/api/servers";
import { OWNER_TIMING } from "../lib/config/runtime";
import type {
  OwnerContentItem,
  OwnerContentUpdateInput,
  OwnerServerCreateInput,
  OwnerServerItem,
  OwnerServerUpdateInput,
  OwnerStorageUsage,
  PublicUser,
  UploadedImage,
} from "../lib/api/types";

interface RefreshOptions {
  silent?: boolean;
}

const LIVE_CONTENT_STATUSES = new Set(["draft", "uploaded", "pending_scan", "pending_moderation"]);
const LIVE_SCAN_STATUSES = new Set(["pending", "running"]);
const LIVE_IMAGE_STATUSES = new Set(["pending", "running"]);
const LIVE_SERVER_CHECK_STATUSES = new Set(["pending", "running"]);
const LIVE_SERVER_STATUSES = new Set(["draft", "pending_verification", "pending_moderation"]);

export const useOwnerStore = defineStore("owner", () => {
  const accessToken = ref("");
  const user = ref<PublicUser | null>(null);
  const items = ref<OwnerContentItem[]>([]);
  const serverItems = ref<OwnerServerItem[]>([]);
  const storageUsage = ref<OwnerStorageUsage | null>(null);
  const imagesByContent = ref<Record<string, UploadedImage[]>>({});
  const selectedContentID = ref("");
  const selectedServerID = ref("");
  const loading = ref(false);
  const syncing = ref(false);
  const error = ref("");
  const notice = ref("");
  let ownerSyncTimer: ReturnType<typeof window.setTimeout> | undefined;

  const isAuthenticated = computed(() => Boolean(user.value));
  const selectedItem = computed(() => items.value.find((item) => item.id === selectedContentID.value) ?? null);
  const selectedServer = computed(() => serverItems.value.find((item) => item.id === selectedServerID.value) ?? null);
  const selectedImages = computed(() => imagesByContent.value[selectedContentID.value] ?? []);

  function logout() {
    resetSession();
    notice.value = "Signed out from this browser session.";
  }

  function bootstrapUser(initialUser: PublicUser) {
    user.value = initialUser;
    publishSessionUser(initialUser);
    queueMicrotask(() => {
      refreshLibrary().finally(startOwnerRealtime).catch(() => undefined);
    });
  }

  async function refreshLibrary(options: RefreshOptions = {}) {
    if (!user.value) return;
    if (!options.silent) {
      loading.value = true;
      error.value = "";
    }
    syncing.value = true;
    try {
      const response = await listOwnedContent(accessToken.value);
      const servers = await listOwnedServers(accessToken.value);
      items.value = response.data;
      serverItems.value = servers;
      storageUsage.value = response.storage;
      if (selectedContentID.value && !items.value.some((item) => item.id === selectedContentID.value)) {
        selectedContentID.value = "";
      }
      if (selectedServerID.value && !serverItems.value.some((item) => item.id === selectedServerID.value)) {
        selectedServerID.value = "";
      }
      await Promise.allSettled(items.value.map((item) => refreshImages(item.id)));
    } catch (caught) {
      if (!options.silent) {
        error.value = caught instanceof Error ? caught.message : "Failed to load owner content.";
        throw caught;
      }
    } finally {
      syncing.value = false;
      if (!options.silent) {
        loading.value = false;
      }
    }
  }

  async function selectContent(contentID: string) {
    selectedContentID.value = contentID;
    await refreshImages(contentID);
    kickOwnerRealtime();
  }

  async function updateSelectedMetadata(input: OwnerContentUpdateInput) {
    if (!user.value || !selectedContentID.value) return;
    loading.value = true;
    error.value = "";
    try {
      const updated = await updateOwnedContent(accessToken.value, selectedContentID.value, input);
      items.value = items.value.map((item) => (item.id === updated.id ? updated : item));
      selectedContentID.value = updated.id;
      notice.value = updated.status === "pending_moderation"
        ? "Content updated and returned to moderation."
        : "Content metadata updated.";
      kickOwnerRealtime();
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : "Failed to update content metadata.";
      throw caught;
    } finally {
      loading.value = false;
    }
  }

  function selectServer(serverID: string) {
    selectedServerID.value = serverID;
    kickOwnerRealtime();
  }

  async function createServer(input: OwnerServerCreateInput) {
    if (!user.value) return;
    loading.value = true;
    error.value = "";
    try {
      const created = await createOwnedServer(accessToken.value, input);
      serverItems.value = [created, ...serverItems.value.filter((item) => item.id !== created.id)];
      selectedServerID.value = created.id;
      notice.value = "Server added and queued for availability check.";
      kickOwnerRealtime(OWNER_TIMING.processingSyncIntervalMs);
      return created;
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : "Failed to add server.";
      throw caught;
    } finally {
      loading.value = false;
    }
  }

  async function updateSelectedServer(input: OwnerServerUpdateInput) {
    if (!user.value || !selectedServerID.value) return;
    loading.value = true;
    error.value = "";
    try {
      const updated = await updateOwnedServer(accessToken.value, selectedServerID.value, input);
      serverItems.value = serverItems.value.map((item) => (item.id === updated.id ? updated : item));
      selectedServerID.value = updated.id;
      notice.value = "Server updated.";
      kickOwnerRealtime(OWNER_TIMING.processingSyncIntervalMs);
      return updated;
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : "Failed to update server.";
      throw caught;
    } finally {
      loading.value = false;
    }
  }

  async function verifyServer(serverID = selectedServerID.value) {
    if (!user.value || !serverID) return;
    loading.value = true;
    error.value = "";
    try {
      const updated = await requestOwnedServerVerification(accessToken.value, serverID);
      serverItems.value = serverItems.value.map((item) => (item.id === updated.id ? updated : item));
      selectedServerID.value = updated.id;
      notice.value = "Server availability check queued.";
      kickOwnerRealtime(OWNER_TIMING.processingSyncIntervalMs);
      return updated;
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : "Failed to queue server check.";
      throw caught;
    } finally {
      loading.value = false;
    }
  }

  async function probeServerConnection(connectionString: string) {
    if (!user.value) return undefined;
    return probeOwnedServerConnection(accessToken.value, connectionString);
  }

  async function deleteServer(serverID: string) {
    if (!user.value || !serverID) return;
    loading.value = true;
    error.value = "";
    try {
      await deleteOwnedServer(accessToken.value, serverID);
      serverItems.value = serverItems.value.filter((item) => item.id !== serverID);
      if (selectedServerID.value === serverID) {
        selectedServerID.value = "";
      }
      notice.value = "Server deleted.";
      kickOwnerRealtime();
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : "Failed to delete server.";
      throw caught;
    } finally {
      loading.value = false;
    }
  }

  async function deleteContent(contentID: string) {
    if (!user.value || !contentID) return;
    loading.value = true;
    error.value = "";
    try {
      await deleteOwnedContent(accessToken.value, contentID);
      items.value = items.value.filter((item) => item.id !== contentID);
      const { [contentID]: _deletedImages, ...remainingImages } = imagesByContent.value;
      imagesByContent.value = remainingImages;
      if (selectedContentID.value === contentID) {
        selectedContentID.value = "";
      }
      notice.value = "Content deleted.";
      kickOwnerRealtime();
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : "Failed to delete content.";
      throw caught;
    } finally {
      loading.value = false;
    }
  }

  async function refreshImages(contentID = selectedContentID.value) {
    if (!user.value || !contentID) return undefined;
    const images = await listContentImages(accessToken.value, contentID);
    imagesByContent.value = {
      ...imagesByContent.value,
      [contentID]: images,
    };
    return images;
  }

  async function setAvatar(file: File) {
    if (!user.value) return;
    const image = await uploadAvatar(accessToken.value, file, `${user.value.username} avatar`);
    notice.value = "Avatar uploaded and queued for processing.";
    await refreshUserUntilAvatarReady(image.id);
  }

  async function updateAccountProfile(displayName: string) {
    if (!user.value) return;
    loading.value = true;
    error.value = "";
    try {
      user.value = await updateProfile(accessToken.value, { display_name: displayName });
      publishSessionUser(user.value);
      notice.value = "Profile updated.";
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : "Failed to update profile.";
      throw caught;
    } finally {
      loading.value = false;
    }
  }

  async function updatePassword(currentPassword: string, newPassword: string) {
    if (!user.value) return;
    loading.value = true;
    error.value = "";
    try {
      const accepted = await changePassword(accessToken.value, {
        current_password: currentPassword,
        new_password: newPassword,
      });
      user.value = accepted.user;
      publishSessionUser(user.value);
      return accepted;
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : "Failed to change password.";
      throw caught;
    } finally {
      loading.value = false;
    }
  }

  async function requestEmailChange(currentPassword: string, newEmail: string) {
    if (!user.value) return;
    loading.value = true;
    error.value = "";
    try {
      const accepted = await changeEmail(accessToken.value, {
        current_password: currentPassword,
        new_email: newEmail,
      });
      user.value = accepted.user;
      publishSessionUser(user.value);
      return accepted;
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : "Failed to request email change.";
      throw caught;
    } finally {
      loading.value = false;
    }
  }

  async function requestEmailVerification() {
    if (!user.value) return;
    loading.value = true;
    error.value = "";
    try {
      user.value = await resendEmailVerification(accessToken.value);
      publishSessionUser(user.value);
      notice.value = user.value.email_verified_at ? "Email is already verified." : "Verification email queued.";
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : "Failed to request email verification.";
      throw caught;
    } finally {
      loading.value = false;
    }
  }

  async function refreshCurrentUser() {
    user.value = await getCurrentUser(accessToken.value);
    publishSessionUser(user.value);
    return user.value;
  }

  async function refreshUserUntilAvatarReady(imageID: string) {
    for (let attempt = 0; attempt < 20; attempt += 1) {
      await delay(attempt === 0 ? 800 : 1500);
      try {
        const freshUser = await refreshCurrentUser();
        if (freshUser.avatar_image_id === imageID) {
          notice.value = "Avatar processed and updated.";
          return;
        }
      } catch {
        // Keep the successful upload state; a later profile refresh will resync the avatar.
      }
    }
    notice.value = "Avatar uploaded. It will appear after processing finishes.";
  }

  async function addContentImage(file: File, altText: string, isPrimary: boolean, options: { silent?: boolean } = {}) {
    if (!user.value || !selectedContentID.value) return undefined;
    const image = await uploadContentImage(accessToken.value, selectedContentID.value, file, {
      altText,
      isPrimary,
      sortOrder: selectedImages.value.length,
    });
    if (!options.silent) {
      notice.value = "Image queued for processing.";
    }
    await refreshImages();
    kickOwnerRealtime(OWNER_TIMING.processingSyncIntervalMs);
    return image;
  }

  async function makePrimary(imageID: string) {
    if (!user.value || !selectedContentID.value) return;
    await updateContentImage(accessToken.value, selectedContentID.value, imageID, { isPrimary: true });
    await refreshImages();
    notice.value = "Primary preview updated.";
    kickOwnerRealtime();
  }

  async function updateImageText(imageID: string, altText: string, sortOrder: number) {
    if (!user.value || !selectedContentID.value) return;
    await updateContentImage(accessToken.value, selectedContentID.value, imageID, { altText, sortOrder });
    await refreshImages();
    notice.value = "Image metadata updated.";
    kickOwnerRealtime();
  }

  async function removeImage(imageID: string) {
    if (!user.value || !selectedContentID.value) return;
    await deleteContentImage(accessToken.value, selectedContentID.value, imageID);
    await refreshImages();
    notice.value = "Image removed.";
    kickOwnerRealtime();
  }

  function resetSession() {
    stopOwnerRealtime();
    accessToken.value = "";
    user.value = null;
    items.value = [];
    serverItems.value = [];
    storageUsage.value = null;
    imagesByContent.value = {};
    selectedContentID.value = "";
    selectedServerID.value = "";
    clearAuthSession();
  }

  function startOwnerRealtime() {
    if (!isBrowser() || !user.value || ownerSyncTimer) return;
    scheduleOwnerRealtime(ownerSyncDelay());
  }

  function kickOwnerRealtime(delayMs = 0) {
    if (!isBrowser() || !user.value) return;
    stopOwnerRealtime();
    scheduleOwnerRealtime(delayMs);
  }

  function scheduleOwnerRealtime(delayMs: number) {
    ownerSyncTimer = window.setTimeout(runOwnerRealtime, Math.max(0, delayMs));
  }

  async function runOwnerRealtime() {
    ownerSyncTimer = undefined;
    if (!user.value) return;

    try {
      await refreshLibrary({ silent: true });
    } finally {
      if (user.value) {
        scheduleOwnerRealtime(ownerSyncDelay());
      }
    }
  }

  function stopOwnerRealtime() {
    if (!ownerSyncTimer) return;
    window.clearTimeout(ownerSyncTimer);
    ownerSyncTimer = undefined;
  }

  function refreshOwnerFromVisibility() {
    if (document.visibilityState === "visible") {
      kickOwnerRealtime();
      return;
    }
    stopOwnerRealtime();
  }

  function ownerSyncDelay() {
    return hasLiveOwnerWork() ? OWNER_TIMING.processingSyncIntervalMs : OWNER_TIMING.syncIntervalMs;
  }

  function hasLiveOwnerWork() {
    return items.value.some((item) =>
      LIVE_CONTENT_STATUSES.has(item.status) ||
      LIVE_SCAN_STATUSES.has(item.file?.scan_status ?? ""),
    ) || serverItems.value.some((item) =>
      LIVE_SERVER_STATUSES.has(item.status) ||
      LIVE_SERVER_CHECK_STATUSES.has(item.check.status),
    ) || Object.values(imagesByContent.value).some((images) =>
      images.some((image) => LIVE_IMAGE_STATUSES.has(image.processing_status)),
    );
  }

  if (typeof window !== "undefined") {
    const session = readAuthSession();
    if (session) {
      user.value = session.user;
      queueMicrotask(() => {
        refreshLibrary().finally(startOwnerRealtime).catch(() => undefined);
      });
    }
    window.addEventListener("beeba:session-logout", resetSession);
    window.addEventListener("pagehide", stopOwnerRealtime);
    document.addEventListener("visibilitychange", refreshOwnerFromVisibility);
  }

  return {
    accessToken,
    user,
    items,
    serverItems,
    storageUsage,
    imagesByContent,
    selectedContentID,
    selectedServerID,
    loading,
    syncing,
    error,
    notice,
    isAuthenticated,
    selectedItem,
    selectedServer,
    selectedImages,
    logout,
    bootstrapUser,
    refreshLibrary,
    selectContent,
    updateSelectedMetadata,
    selectServer,
    createServer,
    updateSelectedServer,
    verifyServer,
    probeServerConnection,
    deleteServer,
    deleteContent,
    refreshImages,
    setAvatar,
    updateAccountProfile,
    updatePassword,
    requestEmailChange,
    requestEmailVerification,
    refreshCurrentUser,
    addContentImage,
    makePrimary,
    updateImageText,
    removeImage,
  };
});

function isBrowser() {
  return typeof window !== "undefined";
}

function delay(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}
