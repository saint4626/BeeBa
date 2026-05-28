import { authHeaders, browserJSON } from "./browser";
import type { UploadedImage } from "./types";

export interface ContentImageUploadOptions {
  altText?: string;
  isPrimary?: boolean;
  sortOrder?: number;
}

export async function uploadAvatar(accessToken: string, file: File, altText = ""): Promise<UploadedImage> {
  const form = new FormData();
  form.set("file", file);
  if (altText) {
    form.set("alt_text", altText);
  }
  const response = await browserJSON<{ data: UploadedImage }>("/me/avatar", {
    method: "POST",
    headers: authHeaders(accessToken),
    body: form,
  });
  return response.data;
}

export async function uploadContentImage(
  accessToken: string,
  contentID: string,
  file: File,
  options: ContentImageUploadOptions = {},
): Promise<UploadedImage> {
  const form = new FormData();
  form.set("file", file);
  if (options.altText) {
    form.set("alt_text", options.altText);
  }
  if (options.isPrimary !== undefined) {
    form.set("is_primary", String(options.isPrimary));
  }
  if (options.sortOrder !== undefined) {
    form.set("sort_order", String(options.sortOrder));
  }
  const response = await browserJSON<{ data: UploadedImage }>(`/me/content/${contentID}/images`, {
    method: "POST",
    headers: authHeaders(accessToken),
    body: form,
  });
  return response.data;
}

export async function listContentImages(accessToken: string, contentID: string): Promise<UploadedImage[]> {
  const response = await browserJSON<{ data: UploadedImage[] }>(`/me/content/${contentID}/images`, {
    headers: authHeaders(accessToken),
  });
  return response.data;
}

export async function updateContentImage(
  accessToken: string,
  contentID: string,
  imageID: string,
  input: { altText?: string; isPrimary?: boolean; sortOrder?: number },
): Promise<UploadedImage> {
  const response = await browserJSON<{ data: UploadedImage }>(`/me/content/${contentID}/images/${imageID}`, {
    method: "PATCH",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify({
      alt_text: input.altText,
      is_primary: input.isPrimary,
      sort_order: input.sortOrder,
    }),
  });
  return response.data;
}

export async function deleteContentImage(accessToken: string, contentID: string, imageID: string): Promise<void> {
  await browserJSON(`/me/content/${contentID}/images/${imageID}`, {
    method: "DELETE",
    headers: authHeaders(accessToken),
  });
}
