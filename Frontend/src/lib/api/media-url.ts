export function publicMediaURL(publicAPIBaseURL: string, imageID?: string | null): string {
  const trimmedImageID = imageID?.trim();
  if (!trimmedImageID) return "";
  return `${publicAPIBaseURL.replace(/\/+$/, "")}/media/${encodeURIComponent(trimmedImageID)}`;
}
