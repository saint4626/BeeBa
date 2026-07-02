import type { PublicServerItem } from "../api/types";

type ServerConnectionSource = Pick<PublicServerItem, "host" | "port" | "has_password" | "connection_string">;

export interface ServerConnectionParts {
  host: string;
  port: string;
  password: string;
  hasPassword: boolean;
}

export function serverConnectionParts(source: ServerConnectionSource): ServerConnectionParts {
  const password = extractPassword(source.connection_string);
  return {
    host: source.host,
    port: String(source.port),
    password,
    hasPassword: source.has_password || password !== "",
  };
}

function extractPassword(connectionString: string) {
  const hashIndex = connectionString.indexOf("#");
  if (hashIndex < 0 || hashIndex >= connectionString.length - 1) {
    return "";
  }
  return connectionString.slice(hashIndex + 1).trim();
}
