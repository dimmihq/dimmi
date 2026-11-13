/*
 * The server's answers in the shapes the views draw. The config becomes an
 * options update merged over the defaults, so a key the server leaves out
 * keeps its built-in value. Conversations and messages from the store
 * become rows and bubbles, with times written the way the visitor reads
 * them: a clock time today, a short date before.
 */

import type * as api from "../client/store";
import type { ActionCard, Attachment, Content, Conversation, Link, Member, Message, Source } from "../content";
import type { Appearance, OptionsUpdate, Tab, Texts } from "../options";
import type { Surface } from "./rules";

export interface Config {
  company?: { name?: string; logo_url?: string | null };
  appearance?: Partial<Appearance> & {
    launcher?: { style?: string; shape?: string };
  };
  texts?: Record<string, string>;
  tabs?: string[];
  surface?: { version?: number; anonymous?: Surface };
  branding?: { show?: boolean; url?: string };
  features?: { attachments?: boolean; max_attachment_mb?: number; identity_mode?: string; opener?: boolean };
  realtime?: { url?: string; token_url?: string; polling_interval_ms?: number };
  honeypot_field?: string;
  customer_status?: boolean;
  delay_notice?: boolean;
  home_links?: Link[];
  locale?: string;
  dir?: string;
}

export interface Availability {
  online: boolean;
  reply_time_text: string;
  team: { name: string; avatar_url: string | null }[];
}

const knownTabs: Tab[] = ["home", "messages", "changelog", "help"];

/*
 * fromConfig turns the config into an options update.
 */
export function fromConfig(config: Config): OptionsUpdate {
  const update: OptionsUpdate = {};
  const a = config.appearance;
  if (a) {
    update.appearance = {
      ...pick(a, [
        "primary_color",
        "header_color",
        "button_color",
        "theme",
        "position",
        "padding_x",
        "padding_y",
        "font",
        "sound",
      ]),
      launcher: {
        ...(a.launcher?.style === "bubble" || a.launcher?.style === "tab" || a.launcher?.style === "hidden"
          ? { style: a.launcher.style }
          : {}),
        ...(a.launcher?.shape === "circle" || a.launcher?.shape === "squircle" ? { shape: a.launcher.shape } : {}),
        ...(config.texts?.launcher_label ? { label: config.texts.launcher_label } : {}),
      },
    } as OptionsUpdate["appearance"];
  }

  if (config.texts) {
    update.texts = Object.fromEntries(
      Object.entries(config.texts).filter(([, word]) => typeof word === "string" && word !== ""),
    ) as Partial<Texts>;
  }
  Object.assign(update, language(config));

  const tabs = (config.tabs ?? []).filter((tab): tab is Tab => knownTabs.includes(tab as Tab));
  if (tabs.length > 0) {
    update.tabs = tabs;
  }
  if (config.branding?.show !== undefined) {
    update.show_branding = config.branding.show;
  }
  if (config.branding?.url) {
    update.branding_url = config.branding.url;
  }

  return update;
}

/*
 * language is the language the server settled for the visitor and which
 * way it reads, kept for the dates and times the messenger writes.
 */
function language(config: Config): OptionsUpdate {
  if (!config.locale) {
    return {};
  }
  reading.locale = config.locale;

  return { locale: config.locale, dir: config.dir === "rtl" ? "rtl" : "ltr" };
}

/*
 * content builds what the views draw from the store and what the server
 * said about the team.
 */
/*
 * links are the customer's links for the home screen, each titled in the
 * visitor's language when the customer translated it.
 */
export function links(config: Config): Link[] {
  return (config.home_links ?? []).map((link, i) => ({
    ...link,
    title: config.texts?.[`home_link_${i}`] || link.title,
  }));
}

export function content(
  base: Content,
  config: Config,
  availability: Availability | undefined,
  store: api.Store,
  open: string | undefined,
  now: number,
): Content {
  return {
    ...base,
    company: config.company?.name ?? base.company,
    logo: config.company?.logo_url ?? null,
    links: links(config),
    reply_time: availability?.reply_time_text ?? base.reply_time,
    team: availability ? availability.team.map((member): Member => ({ name: member.name })) : base.team,
    conversations: store.conversations.map((c) => toConversation(c, now)),
    thread: open ? (store.messages.get(open) ?? []).flatMap((m) => toMessages(m, now)) : [],
    incoming: undefined,
  };
}

export function toConversation(c: api.Conversation, now: number): Conversation {
  const last = c.last_message;

  return {
    id: c.id,
    number: c.number,
    title: last?.author.type === "contact" ? (c.subject ?? "You") : (last?.author.name ?? c.subject ?? ""),
    preview: last?.excerpt ?? "",
    ago: when(last?.created_at ?? c.created_at, now),
    unread: c.unread_count,
    status: c.customer_status,
    ticket: c.ticket,
  };
}

/*
 * toMessages shows a message with several files as one bubble per file
 * after its text, each keyed apart, so every file is seen and a new one
 * arrives on its own.
 */
export function toMessages(m: api.Message, now: number): Message[] {
  const first = toMessage(m, now);
  const rest = m.attachments.slice(1).map((file, index): Message => {
    const out: Message = { from: first.from, at: first.at, id: m.id, key: `${first.key}:file-${index + 1}` };
    if (first.author) {
      out.author = first.author;
    }
    out.attachment = toAttachment(file);

    return out;
  });

  return [first, ...rest];
}

export function toMessage(m: api.Message, now: number): Message {
  const from = m.author.type === "contact" ? "visitor" : m.author.type === "agent" ? "agent" : "team";
  const out: Message = {
    from,
    at: m.sending ? "" : when(m.created_at, now),
    id: m.id,
    key: m.client_id ? `c:${m.client_id}` : m.id,
  };
  if (from !== "visitor" && m.author.name) {
    out.author = m.author.name;
  }
  if (from === "agent") {
    Object.assign(out, agentParts(m));
  }
  if (m.body) {
    out.body = m.body;
  }
  const file = m.attachments[0];
  if (file) {
    out.attachment = toAttachment(file);
  }
  Object.assign(out, workflowParts(m));
  if (m.sending) {
    out.state = "sending";
  } else if (m.failed) {
    out.state = "failed";
  }

  return out;
}

/*
 * workflowParts is what a workflow's message carries beside its text: the
 * answers it offers, whether they are the only way on, and the help
 * article it answered with.
 */
function workflowParts(m: api.Message): Pick<Message, "choices" | "choicesOnly" | "article"> {
  const choices = m.choices ?? [];

  return {
    ...(choices.length > 0 ? { choices } : {}),
    ...(choices.length > 0 && m.choices_only ? { choicesOnly: true } : {}),
    ...(m.article ? { article: m.article } : {}),
  };
}

/*
 * agentParts is what an agent's reply carries besides its text: the
 * agent's picture, the articles it cites and how its run ended.
 */
function agentParts(m: api.Message): Pick<Message, "avatar" | "sources" | "outcome" | "action"> {
  const ai = m.ai ?? m.metadata?.ai;
  const sources = toSources(ai);
  const action = toAction(ai);

  return {
    ...(m.author.avatar_url ? { avatar: m.author.avatar_url } : {}),
    ...(sources.length > 0 ? { sources } : {}),
    ...(ai?.outcome ? { outcome: ai.outcome } : {}),
    ...(action ? { action } : {}),
  };
}

const actionStates: Record<string, ActionCard["state"]> = {
  awaiting_verification: "verify",
  awaiting_confirmation: "ask",
  pending_confirmation: "ask",
  pending_approval: "waiting",
  confirmed: "done",
  approved: "done",
  running: "done",
  succeeded: "done",
  declined: "declined",
  canceled: "canceled",
  expired: "expired",
  failed: "failed",
};

/*
 * toAction reads the action an answer carries: the confirmation card
 * the server rendered, or a call that waits for the team. A card with
 * details asks with the action's title, since the details already list
 * what it will do. Statuses are read in either spelling the server uses.
 */
export function toAction(ai: api.AiAnswer | null | undefined): ActionCard | undefined {
  const card = ai?.confirmation;
  if (card?.action_call_id) {
    const details = card.details ?? [];
    const text = details.length > 0 && card.title ? `${card.title}?` : (card.text ?? card.title ?? "");
    return {
      id: card.action_call_id,
      card: true,
      title: card.title ?? "",
      text,
      details,
      state: actionStates[card.status] ?? "ask",
    };
  }
  const call = ai?.action_call;
  if (!call?.id) {
    return undefined;
  }

  return {
    id: call.id,
    card: false,
    title: call.title ?? "",
    text: call.summary ?? call.title ?? "",
    details: [],
    state: actionStates[call.state] ?? "waiting",
  };
}

/*
 * toSources keeps the articles an answer cites that have a title, and
 * their addresses only when they are plain web links.
 */
export function toSources(ai: api.AiAnswer | null | undefined): Source[] {
  return (ai?.sources ?? []).flatMap((source): Source[] => {
    if (!source.title) {
      return [];
    }
    const url = source.url && /^https?:\/\//.test(source.url) ? source.url : undefined;

    return [url ? { title: source.title, url } : { title: source.title }];
  });
}

function toAttachment(file: api.Attachment): Attachment {
  const kind = file.content_type.startsWith("image/")
    ? "image"
    : file.content_type.startsWith("audio/")
      ? "voice"
      : "file";

  const out: Attachment = { kind, name: file.filename, size: size(file.size_bytes) };
  if (file.url) {
    out.url = file.url;
  }
  if (file.width && file.height) {
    out.width = file.width;
    out.height = file.height;
  }

  return out;
}

export function size(bytes: number): string {
  return bytes < 1024 * 1024 ? `${Math.max(1, Math.round(bytes / 1024))} KB` : `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

/*
 * when writes a time as a clock time today, else as a short date.
 */
export function when(at: string, now: number): string {
  const time = new Date(at);
  if (Number.isNaN(time.getTime())) {
    return "";
  }
  const since = Math.max(0, now - time.getTime());
  const ago = howLongAgo(since);
  if (ago) {
    return ago;
  }
  const today = new Date(now);
  if (time.toDateString() === today.toDateString()) {
    return time.toLocaleTimeString(reading.locale, { hour: "2-digit", minute: "2-digit", hourCycle: "h23" });
  }

  return time.toLocaleDateString(reading.locale, { month: "short", day: "numeric" });
}

const minute = 60_000;
const hour = 60 * minute;

/*
 * recent is how far back a time is still read as a distance from now.
 * Beyond it a clock time or a date places a message better than a
 * growing count of hours does.
 */
const recent = 6 * hour;

/*
 * howLongAgo writes a time near enough to now as the distance from it,
 * the way a reader thinks of it: now, a few minutes, a few hours. Older
 * than a day is a clock time or a date, which is easier to place than
 * "17 days ago". It is nothing when the time is too old for this, and
 * nothing where the browser cannot write the words.
 */
function howLongAgo(since: number): string {
  const rtf = relative();
  if (!rtf || since >= recent) {
    return "";
  }
  if (since < minute) {
    return rtf.format(0, "second");
  }
  if (since < hour) {
    return rtf.format(-Math.floor(since / minute), "minute");
  }

  return rtf.format(-Math.floor(since / hour), "hour");
}

/*
 * relative is the visitor's way of saying how long ago something was,
 * made once per language.
 */
let relativeFor: { locale?: string; rtf?: Intl.RelativeTimeFormat } = {};

function relative(): Intl.RelativeTimeFormat | undefined {
  if (typeof Intl?.RelativeTimeFormat !== "function") {
    return undefined;
  }
  if (relativeFor.locale !== reading.locale) {
    relativeFor = {
      locale: reading.locale,
      rtf: new Intl.RelativeTimeFormat(reading.locale, { numeric: "auto", style: "long" }),
    };
  }

  return relativeFor.rtf;
}

/*
 * reading is the language the visitor reads the messenger in, as the
 * server settled it, which dates and times are written in. Until the
 * config says, it is the browser's.
 */
export const reading: { locale?: string } = {};

function pick<T extends object, K extends keyof T>(source: T, keys: K[]): Partial<Pick<T, K>> {
  const out: Partial<Pick<T, K>> = {};
  for (const key of keys) {
    if (source[key] !== undefined) {
      out[key] = source[key];
    }
  }
  return out;
}
