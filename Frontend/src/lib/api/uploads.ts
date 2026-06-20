import { authHeaders, browserJSON } from "./browser";
import { getPublicAPIBaseURL } from "./client";
import { createContentUploadFormData } from "./upload-form-data";
import { PAGE_LIMITS } from "../config/runtime";
import type {
  ContentUploadCreated,
  OwnerContentDownloadLink,
  OwnerContentItem,
  OwnerContentList,
  OwnerContentListResponse,
  OwnerContentUpdateInput,
  OwnerStorageUsage,
} from "./types";

export async function uploadContentPackage(input: {
  accessToken: string;
  category: string;
  title: string;
  description: string;
  visibility: "public" | "private";
  unlockPassword: string;
  nsfw: boolean;
  file: File;
  onProgress?: (progress: { loaded: number; total: number; percent: number }) => void;
}): Promise<ContentUploadCreated> {
  const body = createContentUploadFormData(input);

  const response = await uploadMultipart<{ data: ContentUploadCreated }>("/content/uploads", body, {
    accessToken: input.accessToken,
    onProgress: input.onProgress,
  });

  return response.data;
}

export async function listOwnedContent(accessToken: string): Promise<OwnerContentList> {
  const items: OwnerContentItem[] = [];
  const seenCursors = new Set<string>();
  let cursor = "";
  let storage: OwnerStorageUsage | null = null;

  do {
    const params = new URLSearchParams({ limit: String(PAGE_LIMITS.ownerContent) });
    if (cursor) {
      params.set("cursor", cursor);
      seenCursors.add(cursor);
    }

    const response = await browserJSON<OwnerContentListResponse>(`/me/content?${params.toString()}`, {
      headers: authHeaders(accessToken),
    });
    items.push(...response.data);
    storage = response.storage ?? storage;

    const nextCursor = response.pagination?.next_cursor ?? "";
    if (nextCursor && seenCursors.has(nextCursor)) {
      throw new Error("Owner content pagination loop detected.");
    }
    cursor = nextCursor;
  } while (cursor);

  return { data: items, storage };
}

function uploadMultipart<T>(
  path: string,
  body: FormData,
  options: {
    accessToken: string;
    onProgress?: (progress: { loaded: number; total: number; percent: number }) => void;
  },
): Promise<T> {
  return new Promise((resolve, reject) => {
    const request = new XMLHttpRequest();
    request.open("POST", `${getPublicAPIBaseURL()}${path}`);
    request.withCredentials = true;
    request.responseType = "text";
    request.setRequestHeader("Accept", "application/json");

    const token = options.accessToken.trim();
    if (token) {
      request.setRequestHeader("Authorization", `Bearer ${token}`);
    }

    const csrfToken = readCookie("beeba_csrf_token");
    if (csrfToken) {
      request.setRequestHeader("X-CSRF-Token", csrfToken);
    }

    request.upload.onprogress = (event) => {
      if (!event.lengthComputable || !options.onProgress) return;
      const percent = Math.min(99, Math.max(0, Math.round((event.loaded / event.total) * 100)));
      options.onProgress({
        loaded: event.loaded,
        total: event.total,
        percent,
      });
    };

    request.onload = () => {
      if (request.status >= 200 && request.status < 300) {
        try {
          resolve(JSON.parse(request.responseText) as T);
        } catch {
          reject(new Error("Upload returned an invalid response."));
        }
        return;
      }

      reject(new Error(uploadErrorMessage(request)));
    };

    request.onerror = () => reject(new Error("Upload failed. Check your connection and try again."));
    request.onabort = () => reject(new Error("Upload was cancelled."));
    request.send(body);
  });
}

function uploadErrorMessage(request: XMLHttpRequest): string {
  try {
    const payload = JSON.parse(request.responseText);
    if (payload?.error?.message) {
      return payload.error.message;
    }
  } catch {
    // Keep stable generic message for non-JSON failures.
  }
  return `Upload failed with status ${request.status}`;
}

function readCookie(name: string): string {
  if (typeof document === "undefined") return "";
  const prefix = `${name}=`;
  const cookie = document.cookie
    .split(";")
    .map((entry) => entry.trim())
    .find((entry) => entry.startsWith(prefix));
  return cookie ? decodeURIComponent(cookie.slice(prefix.length)) : "";
}

export async function updateOwnedContent(
  accessToken: string,
  contentID: string,
  input: OwnerContentUpdateInput,
): Promise<OwnerContentItem> {
  const response = await browserJSON<{ data: OwnerContentItem }>(`/me/content/${contentID}`, {
    method: "PATCH",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify(input),
  });
  return response.data;
}

export async function getOwnedContentDownloadLink(accessToken: string, contentID: string): Promise<OwnerContentDownloadLink> {
  const response = await browserJSON<{ data: OwnerContentDownloadLink }>(
    `/me/content/${encodeURIComponent(contentID)}/download-link`,
    {
      method: "POST",
      headers: authHeaders(accessToken),
    },
  );
  return response.data;
}

export async function deleteOwnedContent(accessToken: string, contentID: string): Promise<void> {
  await browserJSON<void>(`/me/content/${contentID}`, {
    method: "DELETE",
    headers: authHeaders(accessToken),
  });
}
