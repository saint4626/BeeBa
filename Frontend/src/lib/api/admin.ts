import { authHeaders, browserJSON } from "./browser";
import { PAGE_LIMITS } from "../config/runtime";
import type {
  AdminStatus,
  AdminJob,
  AdminFile,
  AdminFileRescanResult,
  AdminCategory,
  AuditLogEntry,
  CatalogTag,
  AdminUser,
  APIListResponse,
  ModeratedComment,
  ModeratedContent,
  ModeratedReport,
  ModerationCommentItem,
  ModerationQueueItem,
  ModerationReportItem,
} from "./types";

const adminPageLimit = String(PAGE_LIMITS.admin);

export async function getAdminStatus(accessToken: string): Promise<AdminStatus> {
  const response = await browserJSON<{ data: AdminStatus }>("/admin/status", {
    headers: authHeaders(accessToken),
  });
  return response.data;
}

export async function listModerationQueue(accessToken: string, status: string): Promise<ModerationQueueItem[]> {
  const params = new URLSearchParams({ limit: adminPageLimit });
  if (status) {
    params.set("status", status);
  }
  const response = await browserJSON<APIListResponse<ModerationQueueItem>>(`/admin/moderation/content?${params}`, {
    headers: authHeaders(accessToken),
  });
  return response.data;
}

export async function approveContent(accessToken: string, contentID: string): Promise<void> {
  await browserJSON(`/admin/content/${contentID}/approve`, {
    method: "POST",
    headers: authHeaders(accessToken),
  });
}

export async function rejectContent(accessToken: string, contentID: string, reason: string): Promise<ModeratedContent> {
  const response = await browserJSON<{ data: ModeratedContent }>(`/admin/content/${contentID}/reject`, {
    method: "POST",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify({ reason }),
  });
  return response.data;
}

export async function hideContent(accessToken: string, contentID: string, reason: string): Promise<ModeratedContent> {
  const response = await browserJSON<{ data: ModeratedContent }>(`/admin/content/${contentID}/hide`, {
    method: "POST",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify({ reason }),
  });
  return response.data;
}

export async function restoreContent(accessToken: string, contentID: string, reason: string): Promise<ModeratedContent> {
  const response = await browserJSON<{ data: ModeratedContent }>(`/admin/content/${contentID}/restore`, {
    method: "POST",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify({ reason }),
  });
  return response.data;
}

export async function listModerationComments(accessToken: string, status: string): Promise<ModerationCommentItem[]> {
  const params = new URLSearchParams({ limit: adminPageLimit });
  if (status) {
    params.set("status", status);
  }
  const response = await browserJSON<APIListResponse<ModerationCommentItem>>(`/admin/moderation/comments?${params}`, {
    headers: authHeaders(accessToken),
  });
  return response.data;
}

export async function approveComment(accessToken: string, commentID: string): Promise<ModeratedComment> {
  const response = await browserJSON<{ data: ModeratedComment }>(`/admin/comments/${commentID}/approve`, {
    method: "POST",
    headers: authHeaders(accessToken),
  });
  return response.data;
}

export async function hideComment(accessToken: string, commentID: string, reason: string): Promise<ModeratedComment> {
  const response = await browserJSON<{ data: ModeratedComment }>(`/admin/comments/${commentID}/hide`, {
    method: "POST",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify({ reason }),
  });
  return response.data;
}

export async function listModerationReports(accessToken: string, status: string): Promise<ModerationReportItem[]> {
  const params = new URLSearchParams({ limit: adminPageLimit });
  if (status) {
    params.set("status", status);
  }
  const response = await browserJSON<APIListResponse<ModerationReportItem>>(`/admin/moderation/reports?${params}`, {
    headers: authHeaders(accessToken),
  });
  return response.data;
}

export async function reviewReport(
  accessToken: string,
  reportID: string,
  status: "in_review" | "resolved" | "rejected",
  reason: string,
): Promise<ModeratedReport> {
  const response = await browserJSON<{ data: ModeratedReport }>(`/admin/reports/${reportID}/status`, {
    method: "POST",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify({ status, reason }),
  });
  return response.data;
}

export async function listAdminUsers(accessToken: string, query: string, role: string): Promise<AdminUser[]> {
  const params = new URLSearchParams({ limit: adminPageLimit });
  if (query) params.set("q", query);
  if (role) params.set("role", role);
  const response = await browserJSON<APIListResponse<AdminUser>>(`/admin/users?${params}`, {
    headers: authHeaders(accessToken),
  });
  return response.data;
}

export async function banAdminUser(accessToken: string, userID: string, reason: string): Promise<AdminUser> {
  const response = await browserJSON<{ data: AdminUser }>(`/admin/users/${userID}/ban`, {
    method: "POST",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify({ reason }),
  });
  return response.data;
}

export async function unbanAdminUser(accessToken: string, userID: string, reason: string): Promise<AdminUser> {
  const response = await browserJSON<{ data: AdminUser }>(`/admin/users/${userID}/unban`, {
    method: "POST",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify({ reason }),
  });
  return response.data;
}

export async function setAdminUserRoles(accessToken: string, userID: string, roles: string[]): Promise<AdminUser> {
  const response = await browserJSON<{ data: AdminUser }>(`/admin/users/${userID}/roles`, {
    method: "PATCH",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify({ roles }),
  });
  return response.data;
}

export async function listAuditLog(
  accessToken: string,
  filters: { action?: string; entity_type?: string; actor_user_id?: string; entity_id?: string; cursor?: string },
): Promise<{ items: AuditLogEntry[]; nextCursor: string }> {
  const params = new URLSearchParams({ limit: adminPageLimit });
  if (filters.action) params.set("action", filters.action);
  if (filters.entity_type) params.set("entity_type", filters.entity_type);
  if (filters.actor_user_id) params.set("actor_user_id", filters.actor_user_id);
  if (filters.entity_id) params.set("entity_id", filters.entity_id);
  if (filters.cursor) params.set("cursor", filters.cursor);
  const response = await browserJSON<APIListResponse<AuditLogEntry>>(`/admin/audit-log?${params}`, {
    headers: authHeaders(accessToken),
  });
  return {
    items: response.data,
    nextCursor: response.pagination?.next_cursor || "",
  };
}

export async function listAdminJobs(
  accessToken: string,
  filters: { queue?: string; status?: string; job_type?: string; cursor?: string },
): Promise<{ items: AdminJob[]; nextCursor: string }> {
  const params = new URLSearchParams({ limit: adminPageLimit });
  if (filters.queue) params.set("queue", filters.queue);
  if (filters.status) params.set("status", filters.status);
  if (filters.job_type) params.set("job_type", filters.job_type);
  if (filters.cursor) params.set("cursor", filters.cursor);
  const response = await browserJSON<APIListResponse<AdminJob>>(`/admin/jobs?${params}`, {
    headers: authHeaders(accessToken),
  });
  return {
    items: response.data,
    nextCursor: response.pagination?.next_cursor || "",
  };
}

export async function retryAdminJob(accessToken: string, jobID: string, reason: string): Promise<AdminJob> {
  const response = await browserJSON<{ data: AdminJob }>(`/admin/jobs/${jobID}/retry`, {
    method: "POST",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify({ reason }),
  });
  return response.data;
}

export async function listAdminFiles(
  accessToken: string,
  filters: { q?: string; scan_status?: string; bucket?: string; content_id?: string; hash?: string; cursor?: string },
): Promise<{ items: AdminFile[]; nextCursor: string }> {
  const params = new URLSearchParams({ limit: adminPageLimit });
  if (filters.q) params.set("q", filters.q);
  if (filters.scan_status) params.set("scan_status", filters.scan_status);
  if (filters.bucket) params.set("bucket", filters.bucket);
  if (filters.content_id) params.set("content_id", filters.content_id);
  if (filters.hash) params.set("hash", filters.hash);
  if (filters.cursor) params.set("cursor", filters.cursor);
  const response = await browserJSON<APIListResponse<AdminFile>>(`/admin/files?${params}`, {
    headers: authHeaders(accessToken),
  });
  return {
    items: response.data,
    nextCursor: response.pagination?.next_cursor || "",
  };
}

export async function rescanAdminFile(accessToken: string, fileID: string, reason: string): Promise<AdminFileRescanResult> {
  const response = await browserJSON<{ data: AdminFileRescanResult }>(`/admin/files/${fileID}/rescan`, {
    method: "POST",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify({ reason }),
  });
  return response.data;
}

export async function listAdminCategories(accessToken: string): Promise<AdminCategory[]> {
  const response = await browserJSON<APIListResponse<AdminCategory>>("/admin/categories", {
    headers: authHeaders(accessToken),
  });
  return response.data;
}

export async function updateAdminCategory(
  accessToken: string,
  categoryID: string,
  input: { name?: string; description?: string; icon_key?: string; sort_order?: number; is_active?: boolean },
): Promise<AdminCategory> {
  const response = await browserJSON<{ data: AdminCategory }>(`/admin/categories/${categoryID}`, {
    method: "PATCH",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify(input),
  });
  return response.data;
}

export async function listAdminTags(accessToken: string): Promise<CatalogTag[]> {
  const response = await browserJSON<APIListResponse<CatalogTag>>("/admin/tags", {
    headers: authHeaders(accessToken),
  });
  return response.data;
}

export async function createAdminTag(accessToken: string, input: { slug: string; name: string; is_system: boolean }): Promise<CatalogTag> {
  const response = await browserJSON<{ data: CatalogTag }>("/admin/tags", {
    method: "POST",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify(input),
  });
  return response.data;
}

export async function updateAdminTag(accessToken: string, tagID: string, input: { name?: string; is_system?: boolean }): Promise<CatalogTag> {
  const response = await browserJSON<{ data: CatalogTag }>(`/admin/tags/${tagID}`, {
    method: "PATCH",
    headers: authHeaders(accessToken, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify(input),
  });
  return response.data;
}
