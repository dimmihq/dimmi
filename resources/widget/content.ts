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
  ticket?: TicketBlock;
}

/*
 * TicketBlock is the request a conversation carries: its reference,
 * title and the state its customer reads, with the state's group.
 */
export interface TicketBlock {
  id?: string;
  reference: string;
  title: string;
  state: { label: string; group: string };
}

/*
 * TicketField is a field of a request's form, as the customer fills it.
 */
export interface TicketField {
  key: string;
  label: string;
  description?: string;
  data_type: string;
  options: string[];
  required: boolean;
  editable: boolean;
  show_when: { field: string; equals?: string[] } | null;
}

/*
 * TicketType is a kind of request the customer may open from Home.
 */
export interface TicketType {
  id: string;
  name: string;
  icon: string;
  description: string;
  fields: TicketField[];
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

/*
 * Token is one run of highlighted code: its text and its kind, keyword,
 * string, number or comment, none for plain text.
 */
export interface Token {
  class?: "keyword" | "string" | "number" | "comment";
  text: string;
}

export interface Block {
  kind: "p" | "h2" | "list" | "steps" | "code" | "note" | "image";
  text?: string;
  items?: string[];
  tokens?: Token[];
}

export interface Article {
  slug: string;
  title: string;
  summary: string;
  author: string;
  updated: string;
  body: Block[];
  /*
   * rated is what the visitor answered about the article, unset until
   * they answer.
   */
  rated?: "yes" | "no";
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

/*
 * Choice is one answer a question offers: the path the workflow takes
 * when the visitor taps it, and what the button reads.
 */
export interface Choice {
  id: string;
  label: string;
}

/*
 * ArticleCard is a help article a workflow answered with, which the
 * visitor opens in the messenger.
 */
export interface ArticleCard {
  slug: string;
  title: string;
  summary?: string;
  url?: string;
}

/*
 * Message is one thing said in a thread. choices are the answers a
 * workflow's question offers, which the visitor taps instead of typing;
 * choicesOnly means they are the only way on, so the composer waits;
 * and article a help article it answered with.
 */
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
  choices?: Choice[];
  choicesOnly?: boolean;
  article?: ArticleCard;
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
  ticket_types?: TicketType[];
}
