import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { changePassword, getCurrentUser, resendEmailVerification, updateProfile } from "../lib/api/auth";
import { clearAuthSession, publishSessionUser, readAuthSession } from "../lib/auth/session";
import { listContentImages, updateContentImage, deleteContentImage, uploadAvatar, uploadContentImage } from "../lib/api/media";
import { deleteOwnedContent, listOwnedContent, updateOwnedContent } from "../lib/api/uploads";
import type { OwnerContentItem, OwnerContentUpdateInput, PublicUser, UploadedImage } from "../lib/api/types";

export const useOwnerStore = defineStore("owner", () => {
  const accessToken = ref("");
  const user = ref<PublicUser | null>(null);
  const items = ref<OwnerContentItem[]>([]);
  const imagesByContent = ref<Record<string, UploadedImage[]>>({});
  const selectedContentID = ref("");
  const loading = ref(false);
  const error = ref("");
  const notice = ref("");

  const isAuthenticated = computed(() => Boolean(user.value));
  const selectedItem = computed(() => items.value.find((item) => item.id === selectedContentID.value) ?? null);
  const selectedImages = computed(() => imagesByContent.value[selectedContentID.value] ?? []);

  function logout() {
    resetSession();
    notice.value = "Signed out from this browser session.";
  }

  function bootstrapUser(initialUser: PublicUser) {
    user.value = initialUser;
    publishSessionUser(initialUser);
    queueMicrotask(() => {
      refreshLibrary().catch(() => undefined);
    });
  }

  async function refreshLibrary() {
    if (!user.value) return;
    loading.value = true;
    error.value = "";
    try {
      items.value = await listOwnedContent(accessToken.value);
      if (!selectedContentID.value && items.value[0]) {
        selectedContentID.value = items.value[0].id;
      }
      await Promise.allSettled(items.value.map((item) => refreshImages(item.id)));
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : "Failed to load owner content.";
      throw caught;
    } finally {
      loading.value = false;
    }
  }

  async function selectContent(contentID: string) {
    selectedContentID.value = contentID;
    await refreshImages(contentID);
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
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : "Failed to update content metadata.";
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
        selectedContentID.value = items.value[0]?.id ?? "";
      }
      notice.value = "Content deleted.";
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : "Failed to delete content.";
      throw caught;
    } finally {
      loading.value = false;
    }
  }

  async function refreshImages(contentID = selectedContentID.value) {
    if (!user.value || !contentID) return;
    imagesByContent.value = {
      ...imagesByContent.value,
      [contentID]: await listContentImages(accessToken.value, contentID),
    };
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
      await changePassword(accessToken.value, {
        current_password: currentPassword,
        new_password: newPassword,
      });
      notice.value = "Password changed. Other sessions were revoked.";
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : "Failed to change password.";
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

  async function addContentImage(file: File, altText: string, isPrimary: boolean) {
    if (!user.value || !selectedContentID.value) return;
    await uploadContentImage(accessToken.value, selectedContentID.value, file, {
      altText,
      isPrimary,
      sortOrder: selectedImages.value.length,
    });
    notice.value = "Image queued for processing.";
    await refreshImages();
  }

  async function makePrimary(imageID: string) {
    if (!user.value || !selectedContentID.value) return;
    await updateContentImage(accessToken.value, selectedContentID.value, imageID, { isPrimary: true });
    await refreshImages();
  }

  async function updateImageText(imageID: string, altText: string, sortOrder: number) {
    if (!user.value || !selectedContentID.value) return;
    await updateContentImage(accessToken.value, selectedContentID.value, imageID, { altText, sortOrder });
    await refreshImages();
  }

  async function removeImage(imageID: string) {
    if (!user.value || !selectedContentID.value) return;
    await deleteContentImage(accessToken.value, selectedContentID.value, imageID);
    await refreshImages();
  }

  function resetSession() {
    accessToken.value = "";
    user.value = null;
    items.value = [];
    imagesByContent.value = {};
    selectedContentID.value = "";
    clearAuthSession();
  }

  if (typeof window !== "undefined") {
    const session = readAuthSession();
    if (session) {
      user.value = session.user;
      queueMicrotask(() => {
        refreshLibrary().catch(() => undefined);
      });
    }
    window.addEventListener("beeba:session-logout", resetSession);
  }

  return {
    accessToken,
    user,
    items,
    imagesByContent,
    selectedContentID,
    loading,
    error,
    notice,
    isAuthenticated,
    selectedItem,
    selectedImages,
    logout,
    bootstrapUser,
    refreshLibrary,
    selectContent,
    updateSelectedMetadata,
    deleteContent,
    refreshImages,
    setAvatar,
    updateAccountProfile,
    updatePassword,
    requestEmailVerification,
    refreshCurrentUser,
    addContentImage,
    makePrimary,
    updateImageText,
    removeImage,
  };
});

function delay(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}
