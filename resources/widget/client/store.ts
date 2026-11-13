/*
 * What the messenger knows, in one place: the visitor's conversations and
 * the messages of those it opened, how many replies they have not read,
 * who on the team is typing and when the team last saw their message.
 * Answers from the server and events from the stream both land here, and
 * the views redraw from it, so a reply shows the same whether it came live
 * or from a poll. A message the visitor sent shows at once, under its
 * client id, and is replaced by the server's copy when that arrives.
 */

export interface Author {
  type: string;
  name: string | null;
  avatar_url: string | null;
}

export interface Attachment {
  id: string;
  filename: string;
  content_type: string;
  size_bytes: number;
  url: string | null;
  blocked?: boolean;
  width: number | null;
  height: number | null;
}

/*
 * AiAnswer is what an answer from the agent carries besides its text:
 * that the agent wrote it, the articles it drew on, and an action it
 * asked to run: a confirmation card for the visitor, or a call waiting
 * for the team.
 */
export interface AiAnswer {
  generated?: boolean;
  outcome?: string;
  sources?: { title: string; url?: string | null }[];
  confirmation?: {
    action_call_id: string;
    title?: string | null;
    text?: string | null;
    status: string;
    details?: { label: string; value: string }[] | null;
  } | null;
  action_call?: { id: string; state: string; summary?: string | null; title?: string | null } | null;
}

export interface Message {
  id: string;
  conversation_id: string;
  client_id?: string | null;
  author: Author;
  body: string | null;
  attachments: Attachment[];
  deleted?: boolean;
  created_at: string;
  read_at?: string | null;
  ai?: AiAnswer | null;
  choices?: { id: string; label: string }[] | null;
  choices_only?: boolean;
  article?: { slug: string; title: string; summary?: string; url?: string } | null;
  metadata?: { ai?: AiAnswer | null } | null;
  sending?: boolean;
  failed?: string;
}

export interface Conversation {
  id: string;
  number: number;
  status: string;
  subject: string | null;
  last_message: { id: string; author: Author; excerpt: string; created_at: string } | null;
  unread_count: number;
  ai_state: string;
  customer_status: string;
  widget_id: string | null;
  created_at: string;
  last_message_at: string | null;
  seen_by_team_at?: string | null;
  ticket?: import("../content").TicketBlock;
}

export interface Typing {
  name: string;
  avatarUrl: string | null;
  until: number;
}

export interface Seen {
  at: string;
  by?: { name: string; avatar_url: string | null };
}

export type Change = "conversations" | "messages" | "typing" | "seen" | "unread";

/*
 * typingFor is how long a teammate counts as typing without news.
 */
const typingFor = 6000;

export class Store {
  conversations: Conversation[] = [];
  messages = new Map<string, Message[]>();
  typing = new Map<string, Typing>();
  seen = new Map<string, Seen>();
  private listeners = new Set<(change: Change, conversation?: string) => void>();

  constructor(private readonly now: () => number) {}

  subscribe(fn: (change: Change, conversation?: string) => void): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  get unread(): number {
    return this.conversations.reduce((sum, c) => sum + c.unread_count, 0);
  }

  setConversations(list: Conversation[]) {
    for (const c of list) {
      this.noteSeen(c);
    }
    this.conversations = sortConversations(list);
    this.emit("conversations");
    this.emit("unread");
  }

  upsertConversation(conversation: Conversation) {
    this.noteSeen(conversation);
    const rest = this.conversations.filter((c) => c.id !== conversation.id);
    this.conversations = sortConversations([conversation, ...rest]);
    this.emit("conversations");
    this.emit("unread");
  }

  setMessages(conversation: string, list: Message[]) {
    const local = (this.messages.get(conversation) ?? []).filter((m) => m.sending || m.failed);
    this.messages.set(conversation, mergeMessages(list, local));
    this.emit("messages", conversation);
  }

  /*
   * addLocal shows a message the visitor just sent, before the server has it.
   */
  addLocal(message: Message) {
    const list = this.messages.get(message.conversation_id) ?? [];
    this.messages.set(message.conversation_id, [...list, message]);
    this.emit("messages", message.conversation_id);
  }

  /*
   * moveLocal files the visitor's pending messages of a new conversation
   * under the id the server gave it.
   */
  moveLocal(from: string, to: string) {
    const list = this.messages.get(from);
    if (!list || from === to) {
      return;
    }
    this.messages.delete(from);
    const moved = list.map((m) => ({ ...m, conversation_id: to }));
    this.messages.set(to, mergeMessages(this.messages.get(to) ?? [], moved));
    this.emit("messages", to);
  }

  /*
   * dropFailed removes the visitor's messages refused for the reason and
   * returns them, so they can be sent again.
   */
  dropFailed(conversation: string, reason: string): Message[] {
    const list = this.messages.get(conversation) ?? [];
    const dropped = list.filter((m) => m.failed === reason);
    this.messages.set(
      conversation,
      list.filter((m) => m.failed !== reason),
    );
    this.emit("messages", conversation);

    return dropped;
  }

  failLocal(conversation: string, clientId: string, reason: string) {
    const list = this.messages.get(conversation) ?? [];
    this.messages.set(
      conversation,
      list.map((m) => (m.client_id === clientId ? { ...m, sending: false, failed: reason } : m)),
    );
    this.emit("messages", conversation);
  }

  /*
   * putMessage adds or replaces a message the server sent, matching the
   * visitor's own pending copy by client id, and says whether the message
   * was new to the conversation rather than one it already showed.
   */
  putMessage(message: Message): boolean {
    const conversation = message.conversation_id;
    const before = this.messages.get(conversation) ?? [];
    const known = before.some((m) => m.id === message.id);
    const list = before.filter((m) => m.id !== message.id && !(message.client_id && m.client_id === message.client_id));
    this.messages.set(conversation, mergeMessages(list, [message]));
    if (message.author.type !== "contact") {
      this.typing.delete(conversation);
      this.emit("typing", conversation);
    }
    this.emit("messages", conversation);

    return !known;
  }

  setTyping(conversation: string, typing: Typing | undefined) {
    if (typing) {
      this.typing.set(conversation, typing);
    } else {
      this.typing.delete(conversation);
    }
    this.emit("typing", conversation);
  }

  /*
   * typingIn names who types in the conversation now, if anyone.
   */
  typingIn(conversation: string): Typing | undefined {
    const typing = this.typing.get(conversation);
    if (typing && typing.until <= this.now()) {
      this.typing.delete(conversation);
      return undefined;
    }
    return typing;
  }

  /*
   * noteSeen keeps when the team last read a conversation the server
   * listed, unless a receipt already told of a later read. A conversation
   * without the time, such as one a stream event carries, changes nothing.
   */
  private noteSeen(c: Conversation) {
    const known = this.seen.get(c.id);
    if (c.seen_by_team_at && (!known || Date.parse(known.at) < Date.parse(c.seen_by_team_at))) {
      this.seen.set(c.id, { at: c.seen_by_team_at });
    }
  }

  /*
   * dropLocal removes the visitor's own copy of a message, such as one
   * refused that they send again, and returns it.
   */
  dropLocal(conversation: string, clientId: string): Message | undefined {
    const list = this.messages.get(conversation) ?? [];
    const found = list.find((m) => m.client_id === clientId && m.failed);
    this.messages.set(
      conversation,
      list.filter((m) => m !== found),
    );

    return found;
  }

  setSeen(conversation: string, seen: Seen) {
    this.seen.set(conversation, seen);
    this.emit("seen", conversation);
  }

  markRead(conversation: string) {
    const found = this.conversations.find((c) => c.id === conversation);
    if (found && found.unread_count > 0) {
      this.upsertConversation({ ...found, unread_count: 0 });
    }
  }

  /*
   * setCallStatus moves the action a message of the conversation carries
   * to a new status, such as confirmed once the visitor pressed Confirm.
   */
  setCallStatus(conversation: string, call: string, status: string) {
    const list = this.messages.get(conversation) ?? [];
    const found = list.find((m) => {
      const ai = m.ai ?? m.metadata?.ai;
      return ai?.confirmation?.action_call_id === call || ai?.action_call?.id === call;
    });
    const ai = found?.ai ?? found?.metadata?.ai;
    if (!found || !ai) {
      return;
    }
    const next = {
      ...ai,
      ...(ai.confirmation ? { confirmation: { ...ai.confirmation, status } } : {}),
      ...(ai.action_call ? { action_call: { ...ai.action_call, state: status } } : {}),
    };
    this.putMessage({ ...found, ai: next, metadata: { ...found.metadata, ai: next } });
  }

  /*
   * apply takes one event from the stream or a poll.
   */
  apply(event: string, data: Record<string, unknown>) {
    switch (event) {
      case "message.created":
      case "message.updated":
      case "message.deleted":
        this.messageEvent(event, data);
        return;
      case "conversation.created":
      case "conversation.closed":
      case "conversation.reopened":
      case "conversation.updated":
        if (data.conversation && typeof data.conversation === "object") {
          this.upsertConversation(data.conversation as Conversation);
        }
        return;
      case "conversation.seen_by_team":
        this.setSeen(String(data.conversation_id), {
          at: String(data.seen_at),
          by: data.seen_by as Seen["by"],
        });
        return;
      case "typing.started":
      case "typing.stopped":
        this.typingEvent(event, data);
        return;
      default:
    }
  }

  private messageEvent(event: string, data: Record<string, unknown>) {
    const message = data.message as Message | undefined;
    if (!message) {
      return;
    }
    const added = this.putMessage(message);

    const found = this.conversations.find((c) => c.id === message.conversation_id);
    if (!found) {
      return;
    }
    const fromTeam = message.author.type !== "contact";
    const fresh = event === "message.created" && fromTeam && added;
    this.upsertConversation({
      ...found,
      customer_status: typeof data.customer_status === "string" ? data.customer_status : found.customer_status,
      unread_count: found.unread_count + (fresh ? 1 : 0),
      last_message:
        event === "message.created"
          ? { id: message.id, author: message.author, excerpt: message.body ?? "", created_at: message.created_at }
          : found.last_message,
      last_message_at: event === "message.created" ? message.created_at : found.last_message_at,
    });
  }

  private typingEvent(event: string, data: Record<string, unknown>) {
    const conversation = String(data.conversation_id);
    if (event === "typing.stopped") {
      this.setTyping(conversation, undefined);
      return;
    }
    const author = (data.author ?? data) as { name?: string; avatar_url?: string | null };
    this.setTyping(conversation, {
      name: author.name ?? "",
      avatarUrl: author.avatar_url ?? null,
      until: this.now() + typingFor,
    });
  }

  private emit(change: Change, conversation?: string) {
    for (const fn of this.listeners) {
      fn(change, conversation);
    }
  }
}

function sortConversations(list: Conversation[]): Conversation[] {
  const at = (c: Conversation) => Date.parse(c.last_message_at ?? c.created_at) || 0;
  return [...list].sort((a, b) => at(b) - at(a));
}

function mergeMessages(server: Message[], local: Message[]): Message[] {
  const known = new Set(server.map((m) => m.client_id).filter(Boolean));
  const kept = local.filter((m) => !m.client_id || !known.has(m.client_id));
  const ids = new Set<string>();
  const out: Message[] = [];
  for (const m of [...server, ...kept]) {
    if (!ids.has(m.id)) {
      ids.add(m.id);
      out.push(m);
    }
  }
  return out.sort((a, b) => (Date.parse(a.created_at) || 0) - (Date.parse(b.created_at) || 0));
}
