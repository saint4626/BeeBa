import { authHeaders, browserJSON } from "./browser";
import { apiGet } from "./client";
import { PAGE_LIMITS } from "../config/runtime";
import type {
  APIListResponse,
  OwnerServerCreateInput,
  OwnerServerFilter,
  OwnerServerItem,
  OwnerServerListResponse,
  OwnerServerProbeResult,
  OwnerServerUpdateInput,
  PublicServerDetail,
  PublicServerItem,
  ServerCatalogFilter,
} from "./types";

export async function listPublicServers(filter: ServerCatalogFilter = {}): Promise<APIListResponse<PublicServerItem>> {
  const params = serverListParams(filter);
  const query = params.toString();
  return apiGet<APIListResponse<PublicServerItem>>(`/servers${query ? `?${query}` : ""}`);
}

export async function getPublicServer(serverID: string, init?: RequestInit): Promise<PublicServerDetail> {
  const response = await apiGet<{ data: PublicServerDetail }>(`/servers/${encodeURIComponent(serverID)}`, init);
  return response.data;
}

export async function listOwnedServers(accessToken: string, filter: OwnerServerFilter = {}): Promise<OwnerServerItem[]> {
  const items: OwnerServerItem[] = [];
  const seenCursors = new Set<string>();
  let cursor = filter.cursor ?? "";

  do {
    const params = new URLSearchParams();
    params.set("limit", String(filter.limit ?? PAGE_LIMITS.ownerContent));
    if (filter.q) params.set("q", filter.q);
    if (filter.status) params.set("status", filter.status);
    if (cursor) {
      params.set("cursor", cursor);
      seenCursors.add(cursor);
    }

    const response = await browserJSON<OwnerServerListResponse>(`/me/servers?${params.toString()}`, {
      headers: authHeaders(accessToken),
    });
    items.push(...response.data);

    const nextCursor = response.pagination?.next_cursor ?? "";
    if (nextCursor && seenCursors.has(nextCursor)) {
      throw new Error("Owner server pagination loop detected.");
    }
    cursor = nextCursor;
  } while (cursor);

  return items;
}

export async function createOwnedServer(accessToken: string, input: OwnerServerCreateInput): Promise<OwnerServerItem> {
  const response = await browserJSON<{ data: OwnerServerItem }>("/me/servers", {
    method: "POST",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify(input),
  });
  return response.data;
}

export async function updateOwnedServer(
  accessToken: string,
  serverID: string,
  input: OwnerServerUpdateInput,
): Promise<OwnerServerItem> {
  const response = await browserJSON<{ data: OwnerServerItem }>(`/me/servers/${encodeURIComponent(serverID)}`, {
    method: "PATCH",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify(input),
  });
  return response.data;
}

export async function requestOwnedServerVerification(accessToken: string, serverID: string): Promise<OwnerServerItem> {
  const response = await browserJSON<{ data: OwnerServerItem }>(`/me/servers/${encodeURIComponent(serverID)}/verify`, {
    method: "POST",
    headers: authHeaders(accessToken),
  });
  return response.data;
}

export async function probeOwnedServerConnection(accessToken: string, connectionString: string): Promise<OwnerServerProbeResult> {
  const response = await browserJSON<{ data: OwnerServerProbeResult }>("/me/servers/probe", {
    method: "POST",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify({ connection_string: connectionString }),
  });
  return response.data;
}

export async function deleteOwnedServer(accessToken: string, serverID: string): Promise<void> {
  await browserJSON<void>(`/me/servers/${encodeURIComponent(serverID)}`, {
    method: "DELETE",
    headers: authHeaders(accessToken),
  });
}

function serverListParams(filter: ServerCatalogFilter): URLSearchParams {
  const params = new URLSearchParams();
  if (filter.q) params.set("q", filter.q);
  if (filter.region) params.set("region", filter.region);
  if (filter.language) params.set("language", filter.language);
  if (filter.tags?.length) params.set("tags", filter.tags.join(","));
  if (filter.online) params.set("online", "true");
  if (filter.include_nsfw) params.set("include_nsfw", "true");
  if (filter.sort) params.set("sort", filter.sort);
  if (filter.cursor) params.set("cursor", filter.cursor);
  if (filter.limit) params.set("limit", String(filter.limit));
  return params;
}
