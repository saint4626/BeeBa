import { authHeaders, browserJSON } from "./browser";
import { PAGE_LIMITS } from "../config/runtime";
import type { APIListResponse, ContentReport, LikeResult, PublicComment } from "./types";

export async function listComments(contentID: string): Promise<PublicComment[]> {
  const response = await browserJSON<APIListResponse<PublicComment>>(`/content/${contentID}/comments?limit=${PAGE_LIMITS.socialComments}`);
  return response.data;
}

export async function createComment(accessToken: string, contentID: string, body: string): Promise<PublicComment> {
  const response = await browserJSON<{ data: PublicComment }>(`/content/${contentID}/comments`, {
    method: "POST",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify({ body }),
  });
  return response.data;
}

export async function likeContent(accessToken: string, contentID: string): Promise<LikeResult> {
  const response = await browserJSON<{ data: LikeResult }>(`/content/${contentID}/like`, {
    method: "POST",
    headers: authHeaders(accessToken),
  });
  return response.data;
}

export async function unlikeContent(accessToken: string, contentID: string): Promise<LikeResult> {
  const response = await browserJSON<{ data: LikeResult }>(`/content/${contentID}/like`, {
    method: "DELETE",
    headers: authHeaders(accessToken),
  });
  return response.data;
}

export async function reportContent(
  accessToken: string,
  contentID: string,
  input: { reason: string; details: string },
): Promise<ContentReport> {
  const response = await browserJSON<{ data: ContentReport }>(`/content/${contentID}/report`, {
    method: "POST",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify(input),
  });
  return response.data;
}
