/*
 * A help center's feature requests: what customers ask for and vote on
 * in its portal, moved through statuses by the team, with replies and
 * internal notes. Moving a request can email its voters once, with a
 * note and the changelog update that shipped it.
 */
import { keepPreviousData, queryOptions } from "@tanstack/react-query";

import { request, requestBody, requestPage } from "./client";

export type PostStatus = "under_review" | "planned" | "in_progress" | "shipped" | "closed";
export type Moderation = "approved" | "pending" | "spam";
export type Priority = "nice" | "important" | "must";
export type PostSort = "trending" | "top" | "new";

export interface FeedbackPost {
  id: string;
  object: "feedback_post";
  help_center_id: string;
  number: number;
  title: string;
  slug: string;
  body_markdown: string;
  status: PostStatus;
  moderation: Moderation;
  author: string;
  author_contact_id: string | null;
  vote_count: number;
  comment_count: number;
  merged_into_id: string | null;
  changelog_entry_id: string | null;
  path: string;
  status_changed_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface FeedbackVoter {
  contact_id: string;
  name: string | null;
  email: string | null;
  avatar_url: string | null;
  company: string | null;
  priority: Priority;
  by_team: boolean;
  created_at: string;
}

export interface FeedbackComment {
  id: string;
  parent_id: string | null;
  body: string;
  internal: boolean;
  author_name: string;
  from_team: boolean;
  created_at: string;
}

export interface PostList {
  data: FeedbackPost[];
  meta: { total?: number; per_page: number };
  counts: Record<PostStatus | "pending", number>;
}

export interface PostFilter {
  status?: PostStatus;
  moderation?: Moderation;
  query?: string;
  sort: PostSort;
  page: number;
}

/*
 * StatusChange moves a request: notify emails its voters and author,
 * with message as the team's note.
 */
export interface StatusChange {
  status: PostStatus;
  notify?: boolean;
  message?: string;
  changelog_entry_id?: string;
}

export const postsPerPage = 25;

/*
 * FeedbackStatus is a stage a request moves through, with the name and
 * color the portal shows.
 */
export interface FeedbackStatus {
  id: PostStatus;
  name: string;
  color: string;
}

/*
 * statuses are the stages in the built-in order, which a workspace keeps
 * until it saves its own and the app shows while that order loads.
 */
export const statuses: FeedbackStatus[] = [
  { id: "in_progress", name: "In progress", color: "#f59e0b" },
  { id: "planned", name: "Planned", color: "#3b82f6" },
  { id: "under_review", name: "Under consideration", color: "#a1a1aa" },
  { id: "shipped", name: "Shipped", color: "#16a34a" },
  { id: "closed", name: "Closed", color: "#71717a" },
];

export const priorities: { id: Priority; name: string }[] = [
  { id: "nice", name: "Nice to have" },
  { id: "important", name: "Important" },
  { id: "must", name: "Must have" },
];

export const feedbackKeys = {
  posts: (workspace: string, helpCenter: string) => ["feedback", workspace, helpCenter, "posts"] as const,
  post: (workspace: string, id: string) => ["feedback", workspace, "post", id] as const,
  statuses: (workspace: string) => ["feedback", workspace, "statuses"] as const,
};

/*
 * statusesQuery reads the workspace's statuses in the order it chose.
 */
export function statusesQuery(workspace: string) {
  return queryOptions({
    queryKey: feedbackKeys.statuses(workspace),
    queryFn: () => request<FeedbackStatus[]>("GET", "/feedback-statuses", undefined, { workspace }),
    enabled: workspace !== "",
  });
}

/*
 * reorderStatuses saves every status in its new order, first to last.
 */
export function reorderStatuses(workspace: string, order: PostStatus[]): Promise<FeedbackStatus[]> {
  return request<FeedbackStatus[]>("PATCH", "/feedback-statuses", { statuses: order }, { workspace });
}

export function postsQuery(workspace: string, helpCenter: string, f: PostFilter) {
  const params = new URLSearchParams({ sort: f.sort, page: String(f.page) });
  for (const [key, value] of [
    ["status", f.status],
    ["moderation", f.moderation],
    ["query", f.query],
  ] as const) {
    if (value) {
      params.set(key, value);
    }
  }

  return queryOptions({
    queryKey: [...feedbackKeys.posts(workspace, helpCenter), f] as const,
    queryFn: () =>
      requestBody<PostList>("GET", `/help-centers/${helpCenter}/feedback-posts?${params}`, undefined, { workspace }),
    placeholderData: keepPreviousData,
    enabled: workspace !== "" && helpCenter !== "",
  });
}

export function postQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: feedbackKeys.post(workspace, id),
    queryFn: () => request<FeedbackPost>("GET", `/feedback-posts/${id}`, undefined, { workspace }),
    enabled: workspace !== "",
  });
}

export function votersQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: [...feedbackKeys.post(workspace, id), "voters"] as const,
    queryFn: () => requestPage<FeedbackVoter>(`/feedback-posts/${id}/votes`, { workspace }),
    enabled: workspace !== "",
  });
}

export function commentsQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: [...feedbackKeys.post(workspace, id), "comments"] as const,
    queryFn: () => request<FeedbackComment[]>("GET", `/feedback-posts/${id}/comments`, undefined, { workspace }),
    enabled: workspace !== "",
  });
}

export function createPost(
  workspace: string,
  helpCenter: string,
  post: { title: string; body_markdown?: string; contact_id?: string },
): Promise<FeedbackPost> {
  return request<FeedbackPost>("POST", `/help-centers/${helpCenter}/feedback-posts`, post, { workspace });
}

export function updatePost(
  workspace: string,
  id: string,
  change:
    | { title?: string; body_markdown?: string }
    | StatusChange
    | { moderation: "approved" | "spam" }
    | {
        merge_into_id: string;
      },
): Promise<FeedbackPost> {
  return request<FeedbackPost>("PATCH", `/feedback-posts/${id}`, change, { workspace });
}

export function deletePost(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", `/feedback-posts/${id}`, undefined, { workspace });
}

export function voteFor(
  workspace: string,
  id: string,
  vote: { contact_id: string; priority: Priority },
): Promise<FeedbackPost> {
  return request<FeedbackPost>("POST", `/feedback-posts/${id}/votes`, vote, { workspace });
}

export function removeVote(workspace: string, id: string, contact: string): Promise<FeedbackPost> {
  return request<FeedbackPost>("DELETE", `/feedback-posts/${id}/votes/${contact}`, undefined, { workspace });
}

export function addComment(
  workspace: string,
  id: string,
  comment: { body: string; internal?: boolean; parent_id?: string },
): Promise<FeedbackComment> {
  return request<FeedbackComment>("POST", `/feedback-posts/${id}/comments`, comment, { workspace });
}

export function deleteComment(workspace: string, id: string): Promise<void> {
  return request<void>("DELETE", `/feedback-comments/${id}`, undefined, { workspace });
}
