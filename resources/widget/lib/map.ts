/*
 * The server's answers in the shapes the views draw. The config becomes an
 * options update merged over the defaults, so a key the server leaves out
 * keeps its built-in value. Conversations and messages from the store
 * become rows and bubbles, with times written the way the visitor reads
 * them: a clock time today, a short date before.
 */

import type * as api from "../client/store";
import type { Attachment, Content, Conversation, Member, Message } from "../content";
import type { Appearance, OptionsUpdate, Tab, Texts } from "../options";

export interface Config {
  company?: { name?: string; logo_url?: string | null };
  appearance?: Partial<Appearance> & {
    launcher?: { style?: string; shape?: string };
  };
  texts?: Record<string, string>;
  tabs?: string[];
  branding?: { show?: boolean; url?: string };
  features?: { attachments?: boolean; max_attachment_mb?: number };
  realtime?: { url?: string; token_url?: string; polling_interval_ms?: number };
  honeypot_field?: string;
  customer_status?: boolean;
  delay_notice?: boolean;
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
export function fromConfig(config: Config, texts: Texts): OptionsUpdate {
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
    const known = Object.keys(texts) as (keyof Texts)[];
    update.texts = Object.fromEntries(
      known
        .filter((key) => typeof config.texts?.[key] === "string" && config.texts[key] !== "")
        .map((key) => [key, config.texts?.[key]]),
    );
  }

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
 * content builds what the views draw from the store and what the server
 * said about the team.
 */
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
  if (m.body) {
    out.body = m.body;
  }
  const file = m.attachments[0];
  if (file) {
    out.attachment = toAttachment(file);
  }
  if (m.sending) {
    out.state = "sending";
  } else if (m.failed) {
    out.state = "failed";
  }

  return out;
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
  const today = new Date(now);
  if (time.toDateString() === today.toDateString()) {
    return time.toTimeString().slice(0, 5);
  }

  return time.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

function pick<T extends object, K extends keyof T>(source: T, keys: K[]): Partial<Pick<T, K>> {
  const out: Partial<Pick<T, K>> = {};
  for (const key of keys) {
    if (source[key] !== undefined) {
      out[key] = source[key];
    }
  }
  return out;
}
