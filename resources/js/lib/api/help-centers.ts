/*
 * A workspace's public docs: its help centers, their collections and
 * articles. An article is saved as a working copy and goes live only
 * when published; a save names the lock version it started from, so two
 * editors never overwrite each other.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export interface HelpCenterSettings {
  primary_color?: string;
  appearance?: "auto" | "light" | "dark";
  home_title?: string;
  home_subtitle?: string;
  footer_text?: string;
  contact_url?: string;
  login_url?: string;
  email_sign_in?: boolean;
  feedback_board?: boolean;
  feedback_approval?: boolean;
}

/*
 * defaultBrand is the color a help center wears until one is picked, the
 * same one its pages use.
 */
export const defaultBrand = "#10b981";

export interface HelpCenter {
  id: string;
  object: "help_center";
  name: string;
  subdomain: string;
  url: string;
  locale: string;
  is_default: boolean;
  status: "active" | "disabled";
  indexing_enabled: boolean;
  ask_ai_enabled: boolean;
  settings: HelpCenterSettings;
}

export interface Collection {
  id: string;
  object: "collection";
  help_center_id: string;
  parent_id: string | null;
  name: string;
  slug: string;
  description: string | null;
  icon: string | null;
  position: number;
}

export type ArticleStatus = "draft" | "published" | "archived";

export interface Article {
  id: string;
  object: "article";
  help_center_id: string;
  collection_id: string | null;
  title: string;
  slug: string;
  description: string | null;
  body_markdown?: string;
  status: ArticleStatus;
  visibility: "public" | "agent_only";
  audience_id: string | null;
  has_unpublished_changes: boolean;
  lock_version: number;
  word_count: number;
  published_at: string | null;
  updated_at: string;
}

export interface ArticleChange {
  title?: string;
  slug?: string;
  description?: string;
  body_markdown?: string;
  visibility?: "public" | "agent_only";
  collection_id?: string;
  lock_version?: number;
}

export function helpCentersQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "help-centers"],
    queryFn: () => request<HelpCenter[]>("GET", "/help-centers", undefined, { workspace }),
  });
}

export function collectionsQuery(workspace: string, helpCenter: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "help-centers", helpCenter, "collections"],
    queryFn: () =>
      request<Collection[]>("GET", `/help-centers/${encodeURIComponent(helpCenter)}/collections`, undefined, {
        workspace,
      }),
  });
}

export function articlesQuery(workspace: string, helpCenter: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "help-centers", helpCenter, "articles"],
    queryFn: () =>
      request<Article[]>("GET", `/help-centers/${encodeURIComponent(helpCenter)}/articles`, undefined, { workspace }),
  });
}

export function articleQuery(workspace: string, article: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "articles", article],
    queryFn: () => request<Article>("GET", `/articles/${encodeURIComponent(article)}`, undefined, { workspace }),
  });
}

/*
 * ArticleVersion is one kept version of an article: published, a
 * snapshot taken before a change, a restore or a suggestion.
 */
export interface ArticleVersion {
  id: string;
  number: number;
  kind: "published" | "snapshot" | "restored" | "suggestion";
  title: string;
  created_at: string;
}

export function articleVersionsQuery(workspace: string, article: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "articles", article, "versions"],
    queryFn: () =>
      request<ArticleVersion[]>("GET", `/articles/${encodeURIComponent(article)}/versions`, undefined, { workspace }),
  });
}

/*
 * uploadArticleImage keeps a picture for an article and returns the
 * address it is served at.
 */
export async function uploadArticleImage(workspace: string, file: File): Promise<string> {
  const form = new FormData();
  form.append("image", file);
  const kept = await request<{ url: string }>("POST", "/help-images", form, { workspace });

  return kept.url;
}

/*
 * restoreArticleVersion puts a kept version back into the article's
 * working copy.
 */
export function restoreArticleVersion(workspace: string, article: string, version: string): Promise<Article> {
  return request<Article>(
    "POST",
    `/articles/${encodeURIComponent(article)}/restorations`,
    { version_id: version },
    { workspace },
  );
}

export function updateHelpCenter(
  workspace: string,
  id: string,
  change: Partial<Pick<HelpCenter, "name" | "subdomain" | "indexing_enabled" | "ask_ai_enabled">> & {
    settings?: HelpCenterSettings;
  },
): Promise<HelpCenter> {
  return request<HelpCenter>("PATCH", `/help-centers/${encodeURIComponent(id)}`, change, { workspace });
}

export function createCollection(
  workspace: string,
  helpCenter: string,
  name: string,
  parent?: string,
): Promise<Collection> {
  return request<Collection>(
    "POST",
    `/help-centers/${encodeURIComponent(helpCenter)}/collections`,
    { name, ...(parent ? { parent_id: parent } : {}) },
    { workspace },
  );
}

/*
 * CollectionChange is a change to a collection: an empty parent_id moves
 * it to the top level.
 */
export interface CollectionChange {
  name?: string;
  parent_id?: string;
  position?: number;
}

export function updateCollection(workspace: string, id: string, change: CollectionChange): Promise<Collection> {
  return request<Collection>("PATCH", `/collections/${encodeURIComponent(id)}`, change, { workspace });
}

export function deleteCollection(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", `/collections/${encodeURIComponent(id)}`, undefined, { workspace });
}

export function createArticle(workspace: string, helpCenter: string, change: ArticleChange): Promise<Article> {
  return request<Article>("POST", `/help-centers/${encodeURIComponent(helpCenter)}/articles`, change, { workspace });
}

export function saveArticle(workspace: string, id: string, change: ArticleChange): Promise<Article> {
  return request<Article>("PATCH", `/articles/${encodeURIComponent(id)}`, change, { workspace });
}

export function setArticleStatus(workspace: string, id: string, status: "draft" | "archived"): Promise<Article> {
  return request<Article>("PATCH", `/articles/${encodeURIComponent(id)}`, { status }, { workspace });
}

/*
 * setArticleAudience keeps an article to a saved audience, or opens it
 * to everyone with null.
 */
export function setArticleAudience(workspace: string, id: string, audience: string | null): Promise<Article> {
  return request<Article>(
    "PATCH",
    `/articles/${encodeURIComponent(id)}/audience`,
    { audience_id: audience },
    {
      workspace,
    },
  );
}

export function publishArticle(workspace: string, id: string): Promise<Article> {
  return request<Article>("POST", `/articles/${encodeURIComponent(id)}/publications`, {}, { workspace });
}

export function deleteArticle(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", `/articles/${encodeURIComponent(id)}`, undefined, { workspace });
}
