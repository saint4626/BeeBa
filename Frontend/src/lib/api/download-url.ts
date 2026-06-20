export interface OwnerDownloadPathItem {
  id: string;
}

export function basisContentDownloadPath(item: OwnerDownloadPathItem): string {
  const contentID = encodeURIComponent(item.id);
  return `/content/${contentID}/download`;
}
