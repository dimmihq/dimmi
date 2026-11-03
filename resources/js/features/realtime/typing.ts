/*
 * Who is typing in a conversation right now. The server relays each
 * start and stop; a start that is not renewed fades after six seconds,
 * so a closed tab never leaves someone typing forever.
 */
import { useEffect, useState } from "react";

import type { Envelope } from "@/lib/realtime/bridge";
import { channels } from "@/lib/realtime/keys";
import { useChannel } from "@/lib/realtime/use-channel";

export const typingExpiry = 6000;

/*
 * Typist is someone typing: who, by type and id, and until when the
 * screen shows it without a new report.
 */
export interface Typist {
  key: string;
  type: string;
  id: string | null;
  until: number;
}

interface TypingData {
  author?: { type?: string; id?: string | null };
}

/*
 * applyTyping returns who types after the broadcast, at the time given,
 * dropping anyone whose report expired.
 */
export function applyTyping(typists: Typist[], envelope: Envelope, now: number): Typist[] {
  const live = typists.filter((typist) => typist.until > now);
  if (envelope.event !== "typing.started" && envelope.event !== "typing.stopped") {
    return live;
  }

  const author = (envelope.data as TypingData | null)?.author;
  const type = author?.type ?? "contact";
  const id = author?.id ?? null;
  const key = `${type}:${id ?? ""}`;
  const others = live.filter((typist) => typist.key !== key);

  return envelope.event === "typing.started" ? [...others, { key, type, id, until: now + typingExpiry }] : others;
}

/*
 * useTyping returns who types in the conversation, by its public id.
 */
export function useTyping(conversation: string | null): Typist[] {
  const [typists, setTypists] = useState<Typist[]>([]);

  useChannel(conversation ? channels.conversation(conversation) : null, {
    onEnvelope: (envelope) => setTypists((current) => applyTyping(current, envelope, Date.now())),
  });

  useEffect(() => {
    if (typists.length === 0) {
      return;
    }

    const next = Math.min(...typists.map((typist) => typist.until)) - Date.now();
    const timer = window.setTimeout(
      () => setTypists((current) => current.filter((typist) => typist.until > Date.now())),
      Math.max(next, 0),
    );

    return () => window.clearTimeout(timer);
  }, [typists]);

  return typists;
}
