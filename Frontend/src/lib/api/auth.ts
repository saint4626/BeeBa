import { authHeaders, browserJSON } from "./browser";
import type { PublicUser, TokenPair } from "./types";

export interface RegisterInput {
  email: string;
  username: string;
  password: string;
  display_name?: string;
  turnstile_token?: string;
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

export interface EmailChangeInput {
  current_password: string;
  new_email: string;
}

export interface AccountChangeAccepted {
  user: PublicUser;
  confirmation_sent: boolean;
  pending_email?: string;
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

export async function changePassword(token: string, input: PasswordChangeInput): Promise<AccountChangeAccepted> {
  const response = await browserJSON<{ data: PublicUser; meta: { confirmation_sent: boolean } }>("/me/password", {
    method: "PATCH",
    headers: authHeaders(token, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify(input),
  });
  return {
    user: response.data,
    confirmation_sent: response.meta.confirmation_sent,
  };
}

export async function changeEmail(token: string, input: EmailChangeInput): Promise<AccountChangeAccepted> {
  const response = await browserJSON<{ data: PublicUser; meta: { confirmation_sent: boolean; pending_email: string } }>("/me/email", {
    method: "PATCH",
    headers: authHeaders(token, {
      "Content-Type": "application/json",
    }),
    body: JSON.stringify(input),
  });
  return {
    user: response.data,
    confirmation_sent: response.meta.confirmation_sent,
    pending_email: response.meta.pending_email,
  };
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

export async function confirmPasswordChange(token: string): Promise<PublicUser> {
  const response = await browserJSON<{ data: PublicUser }>("/auth/password/confirm", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token }),
  });
  return response.data;
}

export async function confirmEmailChange(token: string): Promise<PublicUser> {
  const response = await browserJSON<{ data: PublicUser }>("/auth/email/change/confirm", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token }),
  });
  return response.data;
}
