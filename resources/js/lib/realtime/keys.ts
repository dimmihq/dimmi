/*
 * The query keys realtime updates reach. Inbox queries must be cached
 * under these keys, every one under the workspace it belongs to, so a
 * broadcast can patch exactly what a screen fetched.
 */

export const realtimeKeys = {
  workspace: (workspace: string) => ["workspace", workspace] as const,
  members: (workspace: string) => ["workspace", workspace, "members"] as const,
  caller: (workspace: string) => ["workspace", workspace, "caller"] as const,
  conversations: (workspace: string) => ["workspace", workspace, "conversations"] as const,
  conversation: (workspace: string, id: string) => ["workspace", workspace, "conversation", id] as const,
  messages: (workspace: string, id: string) => ["workspace", workspace, "conversation", id, "messages"] as const,
  counts: (workspace: string) => ["workspace", workspace, "conversation-counts"] as const,
  preferences: (workspace: string) => ["workspace", workspace, "inbox-preferences"] as const,
  views: (workspace: string) => ["workspace", workspace, "conversation-views"] as const,
  me: () => ["me"] as const,
};

/*
 * The channels a workspace screen follows, named the way the server
 * names them.
 */
export const channels = {
  workspace: (workspace: string) => `workspace:${workspace}`,
  workspaceConversations: (workspace: string) => `workspace_conversations:${workspace}`,
  conversation: (conversation: string) => `conversation:${conversation}`,
  member: (member: string) => `member:${member}`,
};
