export interface ContentUploadFormDataInput {
  category: string;
  title: string;
  description: string;
  visibility: "public" | "private";
  unlockPassword: string;
  nsfw: boolean;
  file: File;
}

export function createContentUploadFormData(input: ContentUploadFormDataInput): FormData {
  const body = new FormData();
  body.set("category", input.category);
  body.set("title", input.title);
  body.set("description", input.description);
  body.set("visibility", input.visibility);
  body.set("unlock_password", input.unlockPassword);
  body.set("nsfw", String(input.nsfw));
  body.set("file", input.file);
  return body;
}
