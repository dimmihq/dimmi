/*
 * Translations of a help center's articles and collections into the
 * other languages it offers. The default language's text is the source;
 * a translation is saved as a working copy, published on its own, and
 * outdated once the source is published with other words.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";
import type { ArticleVersion } from "./help-centers";

export interface Language {
  object: "language";
  code: string;
  name: string;
  native_name: string;
}

export interface ArticleTranslation {
  object: "article_translation";
  article_id: string;
  locale: string;
  title: string;
  slug: string;
  description: string | null;
  body_markdown?: string;
  status: "draft" | "published";
  has_unpublished_changes: boolean;
  outdated: boolean;
  lock_version: number;
  word_count: number;
  published_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface ArticleTranslationChange {
  title?: string;
  slug?: string;
  description?: string;
  body_markdown?: string;
  lock_version?: number;
}

/*
 * TranslationState is where one article stands in one language.
 */
export interface TranslationState {
  object: "translation_state";
  article_id: string;
  title: string;
  status: "draft" | "published" | "archived";
  collection_id: string | null;
  locale: string;
  state: "missing" | "draft" | "published";
  outdated: boolean;
}

export interface CollectionTranslation {
  object: "collection_translation";
  collection_id: string;
  locale: string;
  name: string;
  slug: string;
  description: string | null;
  outdated: boolean;
}

const article = (id: string, locale?: string) =>
  `/articles/${encodeURIComponent(id)}/translations${locale ? `/${encodeURIComponent(locale)}` : ""}`;

export function languagesQuery(workspace: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "help-center-languages"],
    queryFn: () => request<Language[]>("GET", "/help-center-languages", undefined, { workspace }),
    staleTime: Infinity,
  });
}

export function articleTranslationsQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "articles", id, "translations"],
    queryFn: () => request<ArticleTranslation[]>("GET", article(id), undefined, { workspace }),
  });
}

export function articleTranslationQuery(workspace: string, id: string, locale: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "articles", id, "translations", locale],
    queryFn: () => request<ArticleTranslation>("GET", article(id, locale), undefined, { workspace }),
    retry: false,
  });
}

export function translationVersionsQuery(workspace: string, id: string, locale: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "articles", id, "translations", locale, "versions"],
    queryFn: () => request<ArticleVersion[]>("GET", `${article(id, locale)}/versions`, undefined, { workspace }),
  });
}

export function translationStatesQuery(workspace: string, helpCenter: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "help-centers", helpCenter, "translation-states"],
    queryFn: () =>
      request<TranslationState[]>(
        "GET",
        `/help-centers/${encodeURIComponent(helpCenter)}/translation-states`,
        undefined,
        { workspace },
      ),
  });
}

export function collectionTranslationsQuery(workspace: string, helpCenter: string) {
  return queryOptions({
    queryKey: ["workspace", workspace, "help-centers", helpCenter, "collection-translations"],
    queryFn: () =>
      request<CollectionTranslation[]>(
        "GET",
        `/help-centers/${encodeURIComponent(helpCenter)}/collection-translations`,
        undefined,
        { workspace },
      ),
  });
}

export function saveArticleTranslation(
  workspace: string,
  id: string,
  locale: string,
  change: ArticleTranslationChange,
): Promise<ArticleTranslation> {
  return request<ArticleTranslation>("PUT", article(id, locale), change, { workspace });
}

export function publishArticleTranslation(workspace: string, id: string, locale: string): Promise<ArticleTranslation> {
  return request<ArticleTranslation>("POST", `${article(id, locale)}/publications`, {}, { workspace });
}

export function unpublishArticleTranslation(
  workspace: string,
  id: string,
  locale: string,
): Promise<ArticleTranslation> {
  return request<ArticleTranslation>("DELETE", `${article(id, locale)}/publications`, undefined, { workspace });
}

export function markTranslationCurrent(workspace: string, id: string, locale: string): Promise<ArticleTranslation> {
  return request<ArticleTranslation>("POST", `${article(id, locale)}/reviews`, {}, { workspace });
}

export function deleteArticleTranslation(workspace: string, id: string, locale: string): Promise<void> {
  return request<void>("DELETE", article(id, locale), undefined, { workspace });
}

export function restoreTranslationVersion(
  workspace: string,
  id: string,
  locale: string,
  version: string,
): Promise<ArticleTranslation> {
  return request<ArticleTranslation>(
    "POST",
    `${article(id, locale)}/restorations`,
    { version_id: version },
    { workspace },
  );
}

export function saveCollectionTranslation(
  workspace: string,
  collection: string,
  locale: string,
  change: { name: string; slug?: string; description?: string },
): Promise<CollectionTranslation> {
  return request<CollectionTranslation>(
    "PUT",
    `/collections/${encodeURIComponent(collection)}/translations/${encodeURIComponent(locale)}`,
    change,
    { workspace },
  );
}

export function deleteCollectionTranslation(workspace: string, collection: string, locale: string): Promise<void> {
  return request<void>(
    "DELETE",
    `/collections/${encodeURIComponent(collection)}/translations/${encodeURIComponent(locale)}`,
    undefined,
    { workspace },
  );
}
