import { apiGet } from "./client";
import type { APIListResponse, CatalogFilter, CatalogTag, Category, PublicContentDetail, PublicContentItem, PublicProfile } from "./types";

export async function listCategories(): Promise<Category[]> {
  const response = await apiGet<{ data: Category[] }>("/categories");
  return response.data;
}

export async function listTags(): Promise<CatalogTag[]> {
  const response = await apiGet<{ data: CatalogTag[] }>("/tags");
  return response.data;
}

export async function listPublicContent(filter: CatalogFilter): Promise<APIListResponse<PublicContentItem>> {
  const params = new URLSearchParams();
  if (filter.category) {
    params.set("category", filter.category);
  }
  if (filter.author) {
    params.set("author", filter.author);
  }
  if (filter.q) {
    params.set("q", filter.q);
  }
  if (filter.tags?.length) {
    params.set("tags", filter.tags.join(","));
  }
  if (filter.include_nsfw) {
    params.set("include_nsfw", "true");
  }
  if (filter.sort) {
    params.set("sort", filter.sort);
  }
  if (filter.cursor) {
    params.set("cursor", filter.cursor);
  }
  if (filter.limit) {
    params.set("limit", String(filter.limit));
  }

  const query = params.toString();
  const endpoint = filter.q ? "/search" : "/content";
  return apiGet<APIListResponse<PublicContentItem>>(`${endpoint}${query ? `?${query}` : ""}`);
}

export async function getPublicContent(contentID: string): Promise<PublicContentDetail> {
  const response = await apiGet<{ data: PublicContentDetail }>(`/content/${contentID}`);
  return response.data;
}

export async function getPublicProfile(username: string): Promise<PublicProfile> {
  const response = await apiGet<{ data: PublicProfile }>(`/users/${encodeURIComponent(username)}`);
  return response.data;
}
