/*
 * A help center's feature requests: what customers ask for and vote on
 * in its portal, split across boards and moved through the workspace's
 * own statuses by the team, with replies and internal notes, and linked
 * to the conversations they came up in. Moving a request can email its
 * voters once, with a note and the changelog update that shipped it.
 */
import { keepPreviousData, queryOptions } from "@tanstack/react-query";

import { request, requestBody, requestPage } from "./client";

/*
 * PostStatus is the key of one of the workspace's statuses, such as
 * under_review or a key the team added.
 */
export type PostStatus = string;
export type StatusCategory = "open" | "planned" | "in_progress" | "done" | "closed";
export type BoardVisibility = "public" | "private" | "internal";
export type Moderation = "approved" | "pending" | "spam";
export type Priority = "nice" | "important" | "must";
export type PostSort = "trending" | "top" | "new";

export interface FeedbackPost {
  id: string;
  object: "feedback_post";
  help_center_id: string;
  board_id: string | null;
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
  board?: string;
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
 * FeedbackStatus is a stage a request moves through, by its key, with the
 * name and color the portal shows, the category that says what it means
 * and whether the public roadmap shows it as a column.
 */
export interface FeedbackStatus {
  id: PostStatus;
  name: string;
  slug?: string;
  color: string;
  category: StatusCategory;
  roadmap: boolean;
}

/*
 * statuses are the stages in the built-in order, which a workspace starts
 * with and the app shows while the workspace's own load.
 */
export const statuses: FeedbackStatus[] = [
  { id: "in_progress", name: "In progress", color: "#f59e0b", category: "in_progress", roadmap: true },
  { id: "planned", name: "Planned", color: "#3b82f6", category: "planned", roadmap: true },
  { id: "under_review", name: "Under consideration", color: "#a1a1aa", category: "open", roadmap: true },
  { id: "shipped", name: "Shipped", color: "#16a34a", category: "done", roadmap: true },
  { id: "closed", name: "Closed", color: "#71717a", category: "closed", roadmap: false },
];

export const categories: { id: StatusCategory; name: string; description: string }[] = [
  { id: "open", name: "Open", description: "New requests start in the first open status." },
  { id: "planned", name: "Planned", description: "The team means to build it." },
  { id: "in_progress", name: "In progress", description: "Being built now." },
  { id: "done", name: "Done", description: "Shipped. Voters can hear about it." },
  { id: "closed", name: "Closed", description: "Won't be built. Never on the roadmap." },
];

/*
 * FeedbackBoard is one board of a help center's requests, such as
 * feature requests or bugs, with who may see it on the portal.
 */
export interface FeedbackBoard {
  id: string;
  object: "feedback_board";
  help_center_id: string;
  name: string;
  slug: string;
  description: string;
  visibility: BoardVisibility;
  approval_required: boolean;
  position: number;
  post_count: number;
}

export interface BoardChange {
  name?: string;
  slug?: string;
  description?: string;
  visibility?: BoardVisibility;
  approval_required?: boolean;
}

export const visibilities: { id: BoardVisibility; name: string; description: string }[] = [
  { id: "public", name: "Public", description: "Anyone can read it and signed-in customers post." },
  { id: "private", name: "Signed-in customers", description: "Only customers who signed in see it." },
  { id: "internal", name: "Internal", description: "Only your team sees it, never the portal." },
];

/*
 * LinkedConversation is a conversation a request came up in, with its
 * customer.
 */
export interface LinkedConversation {
  conversation_id: string;
  conversation_number: number;
  subject: string | null;
  conversation_status: string;
  contact_id: string | null;
  name: string | null;
  email: string | null;
  company: string | null;
  linked_by_team: boolean;
  linked_at: string;
}

export const priorities: { id: Priority; name: string }[] = [
  { id: "nice", name: "Nice to have" },
  { id: "important", name: "Important" },
  { id: "must", name: "Must have" },
];

export const feedbackKeys = {
  posts: (workspace: string, helpCenter: string) => ["feedback", workspace, helpCenter, "posts"] as const,
  post: (workspace: string, id: string) => ["feedback", workspace, "post", id] as const,
  statuses: (workspace: string) => ["feedback", workspace, "statuses"] as const,
  boards: (workspace: string, helpCenter: string) => ["feedback", workspace, helpCenter, "boards"] as const,
  conversation: (workspace: string, conversation: string) =>
    ["feedback", workspace, "conversation", conversation] as const,
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

export function createStatus(
  workspace: string,
  status: { name: string; color?: string; category?: StatusCategory; roadmap?: boolean },
): Promise<FeedbackStatus> {
  return request<FeedbackStatus>("POST", "/feedback-statuses", status, { workspace });
}

export function updateStatus(
  workspace: string,
  key: string,
  change: { name?: string; color?: string; category?: StatusCategory; roadmap?: boolean },
): Promise<FeedbackStatus> {
  return request<FeedbackStatus>("PATCH", `/feedback-statuses/${key}`, change, { workspace });
}

/*
 * deleteStatus removes a status, moving its requests to moveTo.
 */
export function deleteStatus(workspace: string, key: string, moveTo: string): Promise<void> {
  const query = moveTo ? `?move_to=${encodeURIComponent(moveTo)}` : "";
  return request<void>("DELETE", `/feedback-statuses/${key}${query}`, undefined, { workspace });
}

export function boardsQuery(workspace: string, helpCenter: string) {
  return queryOptions({
    queryKey: feedbackKeys.boards(workspace, helpCenter),
    queryFn: () =>
      request<FeedbackBoard[]>("GET", `/help-centers/${helpCenter}/feedback-boards`, undefined, { workspace }),
    enabled: workspace !== "" && helpCenter !== "",
  });
}

export function createBoard(workspace: string, helpCenter: string, board: BoardChange): Promise<FeedbackBoard> {
  return request<FeedbackBoard>("POST", `/help-centers/${helpCenter}/feedback-boards`, board, { workspace });
}

export function updateBoard(workspace: string, id: string, change: BoardChange): Promise<FeedbackBoard> {
  return request<FeedbackBoard>("PATCH", `/feedback-boards/${id}`, change, { workspace });
}

/*
 * deleteBoard removes a board, moving its requests onto moveTo.
 */
export function deleteBoard(workspace: string, id: string, moveTo: string): Promise<void> {
  const query = moveTo ? `?move_to=${encodeURIComponent(moveTo)}` : "";
  return request<void>("DELETE", `/feedback-boards/${id}${query}`, undefined, { workspace });
}

export function reorderBoards(workspace: string, helpCenter: string, order: string[]): Promise<FeedbackBoard[]> {
  return request<FeedbackBoard[]>(
    "PATCH",
    `/help-centers/${helpCenter}/feedback-boards`,
    { boards: order },
    { workspace },
  );
}

/*
 * conversationPostsQuery reads the requests a conversation is linked to.
 */
export function conversationPostsQuery(workspace: string, conversation: string) {
  return queryOptions({
    queryKey: feedbackKeys.conversation(workspace, conversation),
    queryFn: () => requestPage<FeedbackPost>(`/conversations/${conversation}/feedback-posts`, { workspace }),
    enabled: workspace !== "" && conversation !== "",
  });
}

/*
 * linkConversation links a conversation to a request, or to a new one
 * posted from it, and votes for its customer.
 */
export function linkConversation(
  workspace: string,
  conversation: string,
  link: { post_id: string; priority?: Priority } | { title: string; body_markdown?: string; priority?: Priority },
): Promise<FeedbackPost> {
  return request<FeedbackPost>("POST", `/conversations/${conversation}/feedback-posts`, link, { workspace });
}

export function unlinkConversation(workspace: string, conversation: string, post: string): Promise<void> {
  return request<void>("DELETE", `/conversations/${conversation}/feedback-posts/${post}`, undefined, { workspace });
}

export function linkedConversationsQuery(workspace: string, id: string) {
  return queryOptions({
    queryKey: [...feedbackKeys.post(workspace, id), "conversations"] as const,
    queryFn: () => requestPage<LinkedConversation>(`/feedback-posts/${id}/conversations`, { workspace }),
    enabled: workspace !== "",
  });
}

export function postsQuery(workspace: string, helpCenter: string, f: PostFilter) {
  const params = new URLSearchParams({ sort: f.sort, page: String(f.page) });
  for (const [key, value] of [
    ["board_id", f.board],
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
  post: { title: string; body_markdown?: string; contact_id?: string; board_id?: string },
): Promise<FeedbackPost> {
  return request<FeedbackPost>("POST", `/help-centers/${helpCenter}/feedback-posts`, post, { workspace });
}

export function updatePost(
  workspace: string,
  id: string,
  change:
    | { title?: string; body_markdown?: string; board_id?: string }
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
