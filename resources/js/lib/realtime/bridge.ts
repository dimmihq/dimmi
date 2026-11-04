/*
 * Turns what the realtime server broadcasts into changes of the query
 * cache, so every screen showing the data updates without a refetch
 * where it can, and refetches only what a broadcast cannot settle.
 *
 * A broadcast is applied once: replay after a reconnect and the live
 * copy share an id. The screen that made a change already shows it, so
 * a broadcast carrying one of its own client mutation ids is dropped.
 * A broadcast too large to send carries only a reference, and the
 * screens that show it refetch. Who views a conversation patches the
 * rows that show it and nothing else.
 */
import type { QueryClient, QueryKey } from "@tanstack/react-query";

import { appendRecord, type Identified, upsertRecord } from "./cache";
import { realtimeKeys } from "./keys";

/*
 * Envelope is one broadcast as the server sends it.
 */
export interface Envelope {
  id?: string;
  event: string;
  server_time: string;
  data: unknown;
  meta?: Record<string, string>;
}

interface ObjectRecord extends Identified {
  object: string;
  conversation_id?: string;
}

const seenLimit = 500;

/*
 * MutationIds hands out ids for a screen's own changes, sent in the
 * Dimmi-Client-Mutation-Id header, and recognizes them when they come
 * back in a broadcast.
 */
export class MutationIds {
  private readonly own = new Set<string>();

  next(): string {
    const id = crypto.randomUUID();
    this.own.add(id);
    if (this.own.size > seenLimit) {
      const oldest = this.own.values().next().value;
      if (oldest !== undefined) {
        this.own.delete(oldest);
      }
    }
    return id;
  }

  isOwn(id: string | undefined): boolean {
    return id !== undefined && this.own.has(id);
  }
}

function isObjectRecord(value: unknown): value is ObjectRecord {
  return (
    typeof value === "object" &&
    value !== null &&
    typeof (value as ObjectRecord).id === "string" &&
    typeof (value as ObjectRecord).object === "string"
  );
}

/*
 * isViewers reports whether the data says who views a conversation.
 */
function isViewers(value: unknown): value is Identified & { viewers: unknown[] } {
  return (
    typeof value === "object" &&
    value !== null &&
    typeof (value as Identified).id === "string" &&
    Array.isArray((value as { viewers?: unknown }).viewers)
  );
}

/*
 * isReference reports whether the broadcast carries only a reference to
 * a record too large to send.
 */
function isReference(record: ObjectRecord): boolean {
  return Object.keys(record).every((key) => key === "id" || key === "object" || key === "updated_at");
}

/*
 * Bridge applies broadcasts to one query client.
 */
export class Bridge {
  private readonly seen = new Set<string>();
  readonly mutations = new MutationIds();

  constructor(private readonly client: QueryClient) {}

  /*
   * apply changes the cache for one broadcast on a channel of the
   * workspace, and reports whether it was applied.
   */
  apply(workspace: string, channel: string, envelope: Envelope): boolean {
    if (this.duplicate(envelope.id) || this.mutations.isOwn(envelope.meta?.client_mutation_id)) {
      return false;
    }

    const [namespace] = channel.split(":");

    if (envelope.event === "conversation.viewers_changed" && isViewers(envelope.data)) {
      this.patchLists(workspace, envelope.data, false);
    } else if (envelope.event === "inbox_preferences.updated") {
      this.refetch(realtimeKeys.preferences(workspace), realtimeKeys.counts(workspace));
    } else if (envelope.event === "draft.updated") {
      this.refetch(realtimeKeys.drafts(workspace));
    } else if (envelope.event.startsWith("conversation_view.")) {
      this.refetch(realtimeKeys.views(workspace), realtimeKeys.counts(workspace));
    } else if (envelope.event === "workspace.updated") {
      void this.client.invalidateQueries({ queryKey: realtimeKeys.workspace(workspace), exact: true });
      this.refetch(realtimeKeys.me());
    } else if (envelope.event.startsWith("member.")) {
      this.refetch(realtimeKeys.members(workspace), realtimeKeys.caller(workspace));
    } else if (namespace === "member") {
      this.refetch(realtimeKeys.counts(workspace));
    } else if (isObjectRecord(envelope.data)) {
      this.applyRecord(workspace, namespace, envelope.event, envelope.data);
    }

    return true;
  }

  /*
   * resync refetches everything realtime keeps current in the
   * workspace, after the server said it could not replay what a screen
   * missed.
   */
  resync(workspace: string) {
    this.refetch(
      realtimeKeys.conversations(workspace),
      ["workspace", workspace, "conversation"],
      realtimeKeys.counts(workspace),
    );
  }

  private applyRecord(workspace: string, namespace: string, event: string, record: ObjectRecord) {
    if (record.object === "message") {
      this.applyMessage(workspace, event, record);
      return;
    }

    if (record.object !== "conversation") {
      return;
    }

    if (isReference(record)) {
      this.refetch(realtimeKeys.conversation(workspace, record.id), realtimeKeys.conversations(workspace));
      return;
    }

    this.patch(realtimeKeys.conversation(workspace, record.id), record);
    if (namespace === "workspace") {
      this.patchLists(workspace, record);
      this.refetch(realtimeKeys.counts(workspace));
    }
  }

  private applyMessage(workspace: string, event: string, record: ObjectRecord) {
    const conversation = record.conversation_id;
    if (!conversation || isReference(record)) {
      this.refetch(["workspace", workspace, "conversation"]);
      return;
    }

    const key = realtimeKeys.messages(workspace, conversation);
    const cached = this.client.getQueryData(key);
    if (cached === undefined) {
      return;
    }

    const patched = upsertRecord(cached, record);
    if (patched.found) {
      this.client.setQueryData(key, patched.data);
      return;
    }

    const appended = event === "message.created" ? appendRecord(cached, record) : patched;
    if (appended.found) {
      this.client.setQueryData(key, appended.data);
      return;
    }

    this.refetch(key);
  }

  /*
   * patchLists replaces the row in every cached list that has it, and
   * refetches the lists that do not, since the change may move the
   * conversation into them.
   */
  private patchLists(workspace: string, record: Identified, refetchMissing = true) {
    for (const [key, cached] of this.client.getQueriesData({ queryKey: realtimeKeys.conversations(workspace) })) {
      const patched = upsertRecord(cached, record);
      if (patched.found) {
        this.client.setQueryData(key, patched.data);
      } else if (refetchMissing) {
        this.refetch(key);
      }
    }
  }

  private patch(key: QueryKey, record: ObjectRecord) {
    const cached = this.client.getQueryData(key);
    if (cached === undefined) {
      return;
    }

    const patched = upsertRecord(cached, record);
    if (patched.found) {
      this.client.setQueryData(key, patched.data);
    }
  }

  private refetch(...keys: QueryKey[]) {
    for (const queryKey of keys) {
      void this.client.invalidateQueries({ queryKey });
    }
  }

  private duplicate(id: string | undefined): boolean {
    if (id === undefined) {
      return false;
    }
    if (this.seen.has(id)) {
      return true;
    }
    this.seen.add(id);
    if (this.seen.size > seenLimit) {
      const oldest = this.seen.values().next().value;
      if (oldest !== undefined) {
        this.seen.delete(oldest);
      }
    }
    return false;
  }
}
