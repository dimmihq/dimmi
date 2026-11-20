/*
 * Where a conversation stands on WhatsApp, for the composer: read only
 * for conversations on WhatsApp, nothing while a note is written, read
 * again as soon as a new message lands in the thread, since a customer's
 * message opens their window, how long free text can still go out, and
 * the template a reply is sent as when one is picked with /.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { variableOf } from "@/features/broadcasts/writer/variables";
import { contextValues } from "@/features/saved-replies/render";
import { failureText } from "@/features/settings/teams";
import { meQuery } from "@/lib/api/auth";
import { messagesQuery } from "@/lib/api/inbox";
import {
  sendWhatsAppTemplate,
  type WhatsAppConversation,
  type WhatsAppTemplate,
  type WhatsAppVariable,
  whatsappConversationQuery,
} from "@/lib/api/whatsapp";

/*
 * useWhatsAppWindow reads where the conversation stands on WhatsApp, for
 * a conversation on WhatsApp only.
 */
export function useWhatsAppWindow(workspace: string, conversation: string, channelType: string, note: boolean) {
  const whatsapp = channelType === "whatsapp";
  const query = useQuery({ ...whatsappConversationQuery(workspace, conversation), enabled: whatsapp });
  const newest = useQuery({
    ...messagesQuery(workspace, conversation),
    enabled: false,
    select: (messages) => messages.at(-1)?.id ?? null,
  }).data;
  const { refetch } = query;

  useEffect(() => {
    if (whatsapp && newest) {
      void refetch();
    }
  }, [whatsapp, newest, refetch]);

  return note ? undefined : query.data;
}

/*
 * timeLeft says how long free text can still go out, rounded down,
 * zero once the window closed.
 */
export function timeLeft(expiresAt: string | null, now = Date.now()): string | null {
  if (!expiresAt) {
    return null;
  }

  const minutes = Math.floor((Date.parse(expiresAt) - now) / 60_000);
  if (minutes <= 0) {
    return null;
  }

  return minutes >= 60 ? `${Math.floor(minutes / 60)}h left` : `${minutes}m left`;
}

/*
 * TemplateReply is a template picked with / in the composer: the
 * template, its variables as filled in so far, and how to send it.
 */
export interface TemplateReply {
  picked: WhatsAppTemplate | null;
  values: Record<string, string>;
  missing: boolean;
  sending: boolean;
  error: Error | null;
  find: (query: string) => WhatsAppTemplate[];
  pick: (template: WhatsAppTemplate) => void;
  clear: () => void;
  setValue: (key: string, value: string) => void;
  send: () => Promise<void>;
}

/*
 * variableKey names a variable the way a template's values are keyed.
 */
export function variableKey(v: { part: string; key: string }): string {
  return `${v.part}.${v.key}`;
}

/*
 * guess fills the variables dimmi knows the meaning of: each one a
 * template written in dimmi binds to a customer detail takes that detail,
 * or its fallback. A template from WhatsApp Manager says nothing of its
 * variables, so its first takes the customer's first name, as most do.
 */
function guess(t: WhatsAppTemplate, state: WhatsAppConversation | undefined, sender: string): Record<string, string> {
  const known = contextValues({ contact: state?.contact, sender });
  const out: Record<string, string> = {};

  for (const v of t.variables) {
    const bound = t.bindings[variableKey(v)];
    const value = bound ? known[bound.key] || bound.fallback : undefined;
    if (value) {
      out[variableKey(v)] = value;
    }
  }

  const body = t.variables.find((v) => v.part === "body");
  if (Object.keys(t.bindings).length === 0 && body && known["contact.first_name"]) {
    out[variableKey(body)] = known["contact.first_name"];
  }

  return out;
}

/*
 * variableLabel names a variable for the teammate filling it in: the
 * detail it stands for when the template says, or its place.
 */
export function variableLabel(t: WhatsAppTemplate, v: WhatsAppVariable): string {
  const key = t.bindings[variableKey(v)]?.key;
  if (!key) {
    return `${v.part === "header" ? "Header" : "Body"} {{${v.key}}}`;
  }

  const known = variableOf(key)?.label;
  const words = key.replace(/^[a-z_]+\./, "").replaceAll("_", " ");

  return known ?? words.charAt(0).toUpperCase() + words.slice(1);
}

/*
 * useTemplateReply keeps the template picked for the reply, finds the
 * number's sendable templates by name for /, and sends the picked one.
 */
export function useTemplateReply(
  workspace: string,
  conversation: string,
  state: WhatsAppConversation | undefined,
  onSent?: () => void,
): TemplateReply {
  const client = useQueryClient();
  const sender = useQuery(meQuery).data?.user.name ?? "";
  const [picked, setPicked] = useState<WhatsAppTemplate | null>(null);
  const [values, setValues] = useState<Record<string, string>>({});

  const send = useMutation({
    mutationFn: () =>
      sendWhatsAppTemplate(workspace, conversation, {
        name: picked?.name ?? "",
        language: picked?.language ?? "",
        values,
        client_message_id: crypto.randomUUID(),
      }),
    onSuccess: async () => {
      setPicked(null);
      setValues({});
      await Promise.all([
        client.invalidateQueries({ queryKey: ["workspace", workspace, "conversation", conversation] }),
        client.invalidateQueries({ queryKey: ["workspace", workspace, "whatsapp", "conversations", conversation] }),
      ]);
      onSent?.();
    },
  });

  return {
    picked,
    values,
    missing: picked?.variables.some((v) => !values[variableKey(v)]?.trim()) ?? true,
    sending: send.isPending,
    error: send.error,
    find: (query) => {
      const q = query.trim().toLowerCase();
      return (state?.templates ?? []).filter((t) => t.sendable && t.name.toLowerCase().includes(q)).slice(0, 8);
    },
    pick: (template) => {
      send.reset();
      setPicked(template);
      setValues(guess(template, state, sender));
    },
    clear: () => {
      send.reset();
      setPicked(null);
      setValues({});
    },
    setValue: (key, value) => setValues((now) => ({ ...now, [key]: value })),
    send: async () => {
      await send.mutateAsync().catch(() => undefined);
    },
  };
}

/*
 * Outgoing is what the composer's send button does: send the picked
 * template, or the typed text, which a closed WhatsApp window refuses.
 */
export interface Outgoing {
  send: (then?: "close" | "snooze") => void;
  disabled: boolean;
  failure: string | null;
}

/*
 * outgoing routes the send button: to the picked template when there is
 * one, else to the typed text unless the window closed.
 */
export function outgoing(
  reply: TemplateReply,
  state: WhatsAppConversation | undefined,
  text: { send: (then?: "close" | "snooze") => unknown; sending: boolean; ready: boolean; failure: string | null },
): Outgoing {
  if (reply.picked) {
    return {
      send: () => void reply.send(),
      disabled: reply.missing || reply.sending,
      failure: reply.error ? failureText(reply.error) : null,
    };
  }

  const closed = state !== undefined && !state.window.open;

  return {
    send: (then) => void text.send(then),
    disabled: closed || !text.ready || text.sending,
    failure: text.failure,
  };
}
