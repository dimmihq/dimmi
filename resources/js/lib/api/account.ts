/*
 * The person's own account: their profile, picture, password and address,
 * and deleting it all. None of it needs a workspace.
 */
import { queryOptions } from "@tanstack/react-query";

import type { User } from "./auth";
import { request } from "./client";

export interface ProfileChanges {
  name?: string;
  locale?: string | null;
  timezone?: string | null;
}

export function updateProfile(changes: ProfileChanges): Promise<User> {
  return request<User>("PATCH", "/user", changes);
}

export function uploadAvatar(file: File): Promise<User> {
  const form = new FormData();
  form.append("avatar", file);

  return request<User>("PUT", "/user/avatar", form);
}

export function removeAvatar(): Promise<void> {
  return request<void>("DELETE", "/user/avatar");
}

export interface PasswordChange {
  object: "password_change";
  tokens_remaining: number;
}

export function changePassword(password: string, currentPassword?: string): Promise<PasswordChange> {
  return request<PasswordChange>("PATCH", "/user/password", { current_password: currentPassword, password });
}

export interface EmailChange {
  object: "email_change";
  id: string;
  email: string;
  status: "pending" | "confirmed";
}

export function requestEmailChange(email: string): Promise<EmailChange> {
  return request<EmailChange>("POST", "/email-changes", { email });
}

export function confirmEmailChange(id: string, token: string): Promise<EmailChange> {
  return request<EmailChange>("PATCH", `/email-changes/${encodeURIComponent(id)}`, { token });
}

export interface DeletionWorkspace {
  id: string;
  name: string;
  slug: string;
}

export interface DeletionPreview {
  object: "user_deletion_preview";
  blocking: DeletionWorkspace[];
  will_be_deleted: DeletionWorkspace[];
}

export const deletionPreviewQuery = queryOptions({
  queryKey: ["user", "deletion-preview"],
  queryFn: () => request<DeletionPreview>("GET", "/user/deletion-preview"),
});

export function deleteAccount(confirmEmail: string): Promise<void> {
  return request<void>("DELETE", "/user", { confirm_email: confirmEmail });
}
