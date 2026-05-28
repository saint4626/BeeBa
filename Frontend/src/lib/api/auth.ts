import { authHeaders, browserJSON } from "./browser";
import type { PublicUser, TokenPair } from "./types";

export interface RegisterInput {
  email: string;
  username: string;
  password: string;
  display_name?: string;
}

export interface LoginInput {
  email: string;
  password: string;
}

export interface ProfileUpdateInput {
  display_name: string;
}

export interface PasswordChangeInput {
  current_password: string;
  new_password: string;
}

export async function register(input: RegisterInput): Promise<PublicUser> {
  const response = await browserJSON<{ data: PublicUser }>("/auth/register", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  return response.data;
}

export async function login(input: LoginInput): Promise<TokenPair> {
  const response = await browserJSON<{ data: TokenPair }>("/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  return response.data;
}

export async function getCurrentUser(token = ""): Promise<PublicUser> {
  const response = await browserJSON<{ data: PublicUser }>("/auth/me", {
    headers: authHeaders(token),
  });
  return response.data;
}

export async function updateProfile(token: string, input: ProfileUpdateInput): Promise<PublicUser> {
  const response = await browserJSON<{ data: PublicUser }>("/me/profile", {
    method: "PATCH",
    headers: authHeaders(token, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify(input),
  });
  return response.data;
}

export async function changePassword(token: string, input: PasswordChangeInput): Promise<void> {
  await browserJSON<void>("/me/password", {
    method: "PATCH",
    headers: authHeaders(token, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify(input),
  });
}

export async function resendEmailVerification(token = ""): Promise<PublicUser> {
  const response = await browserJSON<{ data: PublicUser }>("/auth/email/resend", {
    method: "POST",
    headers: authHeaders(token),
  });
  return response.data;
}

export async function logout(): Promise<void> {
  await browserJSON<void>("/auth/logout", {
    method: "POST",
  });
}

export async function verifyEmail(token: string): Promise<PublicUser> {
  const response = await browserJSON<{ data: PublicUser }>("/auth/email/verify", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token }),
  });
  return response.data;
}
