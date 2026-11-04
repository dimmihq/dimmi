/*
 * useDraft keeps one draft of the person's: a reply or note on a
 * conversation, or a new conversation. Typing writes the browser's copy
 * after 300 ms and saves to the server after a second; leaving the
 * conversation, switching mode, hiding the tab or closing the page saves
 * at once. On arrival it shows the newer of the copy and the server's
 * draft. Another device's newer save replaces the text when nothing is
 * being typed, and otherwise waits as a conflict the person settles.
 * Sending clears it: the server ends its own copy with the send.
 */
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState } from "react";

import { type Draft, type DraftMode, deleteDraft, draftsQuery, saveDraft } from "@/lib/api/drafts";
import { realtimeKeys } from "@/lib/realtime/keys";

import { type Copy, copyKey, findDraft, newest, readCopy, writeCopy } from "./draft-store";

const copyDelay = 300;
const saveDelay = 1000;

export interface DraftState {
  body: string;
  edit: (body: string) => void;
  flush: () => void;
  sent: () => void;
  conflict: Draft | null;
  takeTheirs: () => void;
  keepMine: () => void;
}

/*
 * useServerDraft reads the person's drafts and returns this one's.
 */
function useServerDraft(workspace: string, conversation: string | null, mode: DraftMode) {
  const { data } = useQuery({ ...draftsQuery(workspace), enabled: workspace !== "" });

  return findDraft(data, conversation, mode);
}

export function useDraft(workspace: string, conversation: string | null, mode: DraftMode): DraftState {
  const client = useQueryClient();
  const key = copyKey(workspace, conversation, mode);
  const stored = useServerDraft(workspace, conversation, mode);
  const [copy, setCopy] = useState<Copy>(() => newest(readCopy(key), stored));
  const [conflict, setConflict] = useState<Draft | null>(null);
  const latest = useRef(copy);
  const dirty = useRef(false);
  const timers = useRef<{ copy?: number; save?: number }>({});
  const inFlight = useRef<AbortController | null>(null);

  const remember = useCallback((next: Copy) => {
    latest.current = next;
    setCopy(next);
  }, []);

  const save = useCallback(
    (keepalive: boolean) => {
      window.clearTimeout(timers.current.save);
      timers.current.save = undefined;
      if (!dirty.current || workspace === "") {
        return;
      }
      const draft = latest.current;
      writeCopy(key, draft);
      inFlight.current?.abort();
      const controller = new AbortController();
      inFlight.current = controller;
      void persist({ workspace, conversation, mode, draft, keepalive, signal: controller.signal, stored })
        .then((saved) => {
          if (latest.current === draft) {
            dirty.current = false;
          }
          client.setQueryData<Draft[]>(realtimeKeys.drafts(workspace), (list) =>
            replace(list, saved, { conversation, mode }),
          );
        })
        .catch(() => undefined);
    },
    [client, conversation, key, mode, stored, workspace],
  );

  const edit = useCallback(
    (body: string) => {
      dirty.current = true;
      remember({ body, at: new Date().toISOString() });
      window.clearTimeout(timers.current.copy);
      window.clearTimeout(timers.current.save);
      timers.current.copy = window.setTimeout(() => writeCopy(key, latest.current), copyDelay);
      timers.current.save = window.setTimeout(() => save(false), saveDelay);
    },
    [key, remember, save],
  );

  const flush = useCallback(() => save(true), [save]);

  const sent = useCallback(() => {
    window.clearTimeout(timers.current.copy);
    window.clearTimeout(timers.current.save);
    timers.current = {};
    inFlight.current?.abort();
    dirty.current = false;
    writeCopy(key, null);
    remember({ body: "", at: new Date().toISOString() });
    client.setQueryData<Draft[]>(realtimeKeys.drafts(workspace), (list) => replace(list, null, { conversation, mode }));
  }, [client, conversation, key, mode, remember, workspace]);

  useArrivals(stored, latest, dirty, { remember, setConflict });
  useFlushOnLeave(flush);

  return {
    body: copy.body,
    edit,
    flush,
    sent,
    conflict,
    takeTheirs: () => {
      if (conflict) {
        dirty.current = false;
        writeCopy(key, null);
        remember({ body: conflict.body, at: conflict.client_updated_at });
      }
      setConflict(null);
    },
    keepMine: () => {
      setConflict(null);
      edit(latest.current.body);
    },
  };
}

/*
 * useArrivals takes a newer server draft from another device: straight
 * into the field when nothing is being typed, else as a conflict.
 */
function useArrivals(
  stored: Draft | undefined,
  latest: { current: Copy },
  dirty: { current: boolean },
  on: { remember: (copy: Copy) => void; setConflict: (draft: Draft | null) => void },
) {
  const { remember, setConflict } = on;
  useEffect(() => {
    if (!stored || Date.parse(stored.client_updated_at) <= Date.parse(latest.current.at)) {
      return;
    }
    if (stored.body === latest.current.body) {
      return;
    }
    if (dirty.current) {
      setConflict(stored);
      return;
    }
    remember({ body: stored.body, at: stored.client_updated_at });
  }, [stored, latest, dirty, remember, setConflict]);
}

/*
 * useFlushOnLeave saves at once when the tab hides, the page goes away or
 * the composer unmounts.
 */
function useFlushOnLeave(flush: () => void) {
  const current = useRef(flush);
  current.current = flush;
  useEffect(() => {
    const hidden = () => {
      if (document.visibilityState === "hidden") {
        current.current();
      }
    };
    const leave = () => current.current();
    document.addEventListener("visibilitychange", hidden);
    window.addEventListener("pagehide", leave);
    window.addEventListener("online", leave);

    return () => {
      document.removeEventListener("visibilitychange", hidden);
      window.removeEventListener("pagehide", leave);
      window.removeEventListener("online", leave);
      current.current();
    };
  }, []);
}

/*
 * persist writes the draft to the server, or discards the server's copy
 * once the text is empty.
 */
async function persist(p: {
  workspace: string;
  conversation: string | null;
  mode: DraftMode;
  draft: Copy;
  keepalive: boolean;
  signal: AbortSignal;
  stored: Draft | undefined;
}): Promise<Draft | null> {
  if (p.draft.body.trim() === "") {
    if (p.stored) {
      await deleteDraft(p.workspace, p.stored.id, { keepalive: p.keepalive });
    }
    return null;
  }

  return saveDraft(
    p.workspace,
    { conversation_id: p.conversation, mode: p.mode, body: p.draft.body, client_updated_at: p.draft.at },
    { keepalive: p.keepalive, signal: p.keepalive ? undefined : p.signal },
  );
}

/*
 * replace puts the saved draft in the cached list, or takes the
 * discarded one out.
 */
function replace(
  list: Draft[] | undefined,
  saved: Draft | null,
  of: { conversation: string | null; mode: DraftMode },
): Draft[] | undefined {
  const others = list?.filter((d) => !(d.conversation_id === of.conversation && d.mode === of.mode));
  if (!others) {
    return list;
  }

  return saved === null ? others : [saved, ...others];
}
