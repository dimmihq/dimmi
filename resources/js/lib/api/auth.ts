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
  two_factor_enabled: boolean;
}

export interface WorkspaceSummary {
  id: string;
  name: string;
  slug: string;
  role: string;
}

export interface Instance {
  edition: string;
  version: string;
}

export interface Me {
  object: "me";
  user: User;
  workspaces: WorkspaceSummary[];
  instance: Instance;
}

/*
 * ChallengeMethod is one way the person may finish signing in: a second
 * factor they hold, such as an authenticator app or their recovery codes.
 */
export interface ChallengeMethod {
  method: string;
  factor_id: string;
  name: string;
  preferred: boolean;
}

/*
 * TwoFactorChallenge is the answer to a correct first step on an account
 * protected by a second factor: no session yet, only the ways to finish.
 */
export interface TwoFactorChallenge {
  object: "two_factor_challenge";
  expires_at: string;
  methods: ChallengeMethod[];
}

/*
 * SignInResult is what a sign-in answers: the caller once signed in, or
 * the challenge still to answer.
 */
export type SignInResult = Me | TwoFactorChallenge;

export function isChallenge(result: SignInResult): result is TwoFactorChallenge {
  return result.object === "two_factor_challenge";
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

export function signIn(email: string, password: string): Promise<SignInResult> {
  return request<SignInResult>("POST", "/sessions", { grant: "password", email, password });
}

export function signInWithLink(token: string): Promise<SignInResult> {
  return request<SignInResult>("POST", "/sessions", { grant: "magic_link", token });
}

export function signInWithCode(email: string, code: string): Promise<SignInResult> {
  return request<SignInResult>("POST", "/sessions", { grant: "email_code", email, code });
}

export function signInWithSecondFactor(method: string, code: string): Promise<Me> {
  return request<Me>("POST", "/sessions", { grant: "two_factor", method, code });
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

export function resetPassword(id: string, token: string, password: string): Promise<SignInResult> {
  return request<SignInResult>("PATCH", `/password-resets/${encodeURIComponent(id)}`, { token, password });
}

export function verifyEmail(id: string, token: string): Promise<unknown> {
  return request<unknown>("PATCH", `/email-verifications/${encodeURIComponent(id)}`, { token });
}

export function signOut(): Promise<void> {
  return request<void>("DELETE", "/sessions/current");
}
