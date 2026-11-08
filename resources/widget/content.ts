/*
 * What the messenger shows besides its own chrome: the team, the visitor's
 * recent conversations and one open thread, the customer's links, help
 * collections and the changelog. A connected messenger builds these from the
 * server's answers; a preview uses the sample in preview.ts.
 */

export interface Member {
  name: string;
}

export interface Conversation {
  id?: string;
  number: number;
  title: string;
  preview: string;
  ago: string;
  unread?: number;
  status?: string;
}

/*
 * Incoming is a message the team sends while the visitor is elsewhere,
 * with the conversation it belongs to.
 */
export interface Incoming extends Message {
  conversation: number | string;
}

export interface Link {
  title: string;
  icon: "link" | "calendar" | "book" | "mail" | "phone" | "chat" | "video" | "help";
  url: string;
}

export interface Block {
  kind: "p" | "h2" | "list" | "steps" | "code" | "note" | "image";
  text?: string;
  items?: string[];
}

export interface Article {
  slug: string;
  title: string;
  summary: string;
  author: string;
  updated: string;
  body: Block[];
}

export interface Collection {
  slug: string;
  title: string;
  description: string;
  articles: string[];
}

export interface Entry {
  tag: string;
  title: string;
  summary: string;
  date: string;
  body: Block[];
}

export interface Attachment {
  kind: "file" | "image" | "voice";
  name: string;
  size?: string;
  duration?: string;
  url?: string;
  width?: number;
  height?: number;
}

export interface Source {
  title: string;
  url?: string;
}

/*
 * ActionCard is an action the agent asked to run, as the visitor sees it:
 * a question to confirm or cancel, then where it stands. sending marks a
 * choice on its way to the server. card is false for a call that only
 * reports its state, such as one the team still has to approve.
 */
export interface ActionCard {
  id: string;
  card: boolean;
  title: string;
  text: string;
  details: { label: string; value: string }[];
  state: "ask" | "verify" | "sending" | "waiting" | "done" | "declined" | "canceled" | "expired" | "failed";
  verify?: VerifyStep;
}

/*
 * VerifyStep is where a visitor stands in confirming their email for an
 * action: typing the address, or the code mailed to it.
 */
export interface VerifyStep {
  step: "email" | "code";
  email: string;
  masked?: string;
  error?: string;
  busy?: boolean;
}

export interface Message {
  id?: string;
  key?: string;
  state?: "sending" | "failed";
  from: "visitor" | "team" | "agent";
  author?: string;
  avatar?: string;
  outcome?: string;
  body?: string;
  attachment?: Attachment;
  sources?: Source[];
  action?: ActionCard;
  at: string;
}

export interface Content {
  company: string;
  logo?: string | null;
  reply_time: string;
  team: Member[];
  thread: Message[];
  incoming?: Incoming;
  conversations: Conversation[];
  links: Link[];
  collections: Collection[];
  articles: Article[];
  popular: string[];
  help_url?: string;
  changelog: Entry[];
  unread_changelog: number;
}
