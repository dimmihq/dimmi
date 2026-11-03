/*
 * The account endpoints the sign-in pages call, and the queries for the
 * server's sign-in options and the signed-in caller.
 */
import { queryOptions } from "@tanstack/react-query";

import { ApiError, request } from "./client";

export interface Provider {
  key: string;
  name: string;
  protocol: string;
  redirect_url: string;
}

export interface AuthConfig {
  edition: string;
  signup_mode: "open" | "invite_only";
  magic_link: boolean;
  mail_configured: boolean;
  password_min_length: number;
  providers: Provider[];
}

export interface User {
  id: string;
  name: string;
  email: string;
  email_verified: boolean;
  has_password: boolean;
}

export interface WorkspaceSummary {
  id: string;
  name: string;
  slug: string;
  role: string;
}

export interface Me {
  user: User;
  workspaces: WorkspaceSummary[];
}

export interface Message {
  message: string;
}

export const authConfigQuery = queryOptions({
  queryKey: ["auth-config"],
  queryFn: () => request<AuthConfig>("GET", "/auth/config"),
  staleTime: 5 * 60 * 1000,
});

/*
 * meQuery is the signed-in caller, or null for a browser without a
 * session, kept like any answer so routes ask the server once.
 */
export const meQuery = queryOptions({
  queryKey: ["me"],
  queryFn: async (): Promise<Me | null> => {
    try {
      return await request<Me>("GET", "/me");
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        return null;
      }

      throw error;
    }
  },
  retry: false,
});

export function signIn(email: string, password: string): Promise<Me> {
  return request<Me>("POST", "/sessions", { grant: "password", email, password });
}

export function signInWithLink(token: string): Promise<Me> {
  return request<Me>("POST", "/sessions", { grant: "magic_link", token });
}

export function signInWithCode(email: string, code: string): Promise<Me> {
  return request<Me>("POST", "/sessions", { grant: "email_code", email, code });
}

export function signUp(input: { name: string; email: string; password: string }): Promise<Me> {
  return request<Me>("POST", "/users", input);
}

export function sendMagicLink(email: string): Promise<Message> {
  return request<Message>("POST", "/magic-links", { email });
}

export function requestPasswordReset(email: string): Promise<Message> {
  return request<Message>("POST", "/password-resets", { email });
}

export function resetPassword(id: string, token: string, password: string): Promise<Me> {
  return request<Me>("PATCH", `/password-resets/${encodeURIComponent(id)}`, { token, password });
}

export function verifyEmail(id: string, token: string): Promise<unknown> {
  return request<unknown>("PATCH", `/email-verifications/${encodeURIComponent(id)}`, { token });
}

export function signOut(): Promise<void> {
  return request<void>("DELETE", "/sessions/current");
}
