export interface OwnerDownloadPathItem {
  id: string;
  visibility: string;
}

export function basisContentDownloadPath(item: OwnerDownloadPathItem): string {
  const contentID = encodeURIComponent(item.id);
  if (item.visibility === "public") {
    return `/content/${contentID}/download`;
  }
  return `/me/content/${contentID}/download`;
}
