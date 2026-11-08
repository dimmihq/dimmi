/*
 * The working copy of a procedure while it is edited. Every change shows
 * at once and is saved shortly after, one save at a time, each naming the
 * lock version the last one returned; a save refused because someone else
 * saved first stops saving, so their work is never overwritten. Publishing
 * saves what is pending first, then publishes; what stops it is listed
 * beside the steps.
 */
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { toast } from "@/components/base/toast/toast";
import { ApiError } from "@/lib/api/client";
import {
  deleteProcedure,
  type Procedure,
  type ProcedureDefinition,
  type ProcedureIssue,
  publishProcedure,
  saveProcedure,
  unpublishProcedure,
} from "@/lib/api/procedures";

import type { SaveState } from "./use-workflow-draft";

interface Changes {
  name?: string;
  definition?: ProcedureDefinition;
}

const pause = 600;

function pending(changes: Changes): boolean {
  return changes.name !== undefined || changes.definition !== undefined;
}

export function useProcedureDraft(workspace: string, procedure: Procedure, onDeleted: () => void) {
  const queryClient = useQueryClient();
  const [definition, setDefinition] = useState(procedure.definition);
  const [name, setName] = useState(procedure.name);
  const [server, setServer] = useState(procedure);
  const [state, setState] = useState<SaveState>("saved");
  const [issues, setIssues] = useState<ProcedureIssue[]>([]);
  const [busy, setBusy] = useState(false);
  const current = useRef(definition);
  const lock = useRef(procedure.lock_version);
  const wanted = useRef<Changes>({});
  const timer = useRef<number | undefined>(undefined);
  const running = useRef<Promise<void> | null>(null);
  const stopped = useRef(false);
  const refresh = () => queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "procedures"] });

  const once = async () => {
    const changes = wanted.current;
    wanted.current = {};
    if (stopped.current || !pending(changes)) {
      return;
    }
    try {
      const next = await saveProcedure(workspace, procedure.id, { ...changes, lock_version: lock.current });
      lock.current = next.lock_version;
      setServer(next);
      setState(pending(wanted.current) ? "saving" : "saved");
    } catch (error) {
      wanted.current = { ...changes, ...wanted.current };
      stopped.current = error instanceof ApiError && error.code === "edit_conflict";
      setState(stopped.current ? "conflict" : "failed");
      toast.error(error instanceof Error ? error.message : "The procedure could not be saved.");
    }
  };

  const flush = async (): Promise<void> => {
    window.clearTimeout(timer.current);
    while (running.current) {
      await running.current;
    }
    running.current = once().finally(() => {
      running.current = null;
    });
    await running.current;
  };

  const queue = (changes: Changes) => {
    wanted.current = { ...wanted.current, ...changes };
    if (stopped.current) {
      return;
    }
    setState("saving");
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => void flush(), pause);
  };

  const leave = useRef(() => flush().then(refresh));
  leave.current = () => flush().then(refresh);
  useEffect(() => {
    const onLeave = leave;

    return () => {
      void onLeave.current();
    };
  }, []);

  const act = async (work: () => Promise<Procedure>) => {
    setBusy(true);
    try {
      await flush();
      const next = await work();
      lock.current = next.lock_version;
      setServer(next);
      setIssues([]);
      void refresh();
    } catch (error) {
      const found = error instanceof ApiError && Array.isArray(error.details.errors) ? error.details.errors : [];
      setIssues(found as ProcedureIssue[]);
      toast.error(error instanceof Error ? error.message : "That did not work.");
    } finally {
      setBusy(false);
    }
  };

  return {
    definition,
    name,
    server,
    state,
    issues,
    busy,
    change: (edit: (d: ProcedureDefinition) => ProcedureDefinition) => {
      const next = edit(current.current);
      current.current = next;
      setDefinition(next);
      setIssues([]);
      queue({ definition: next });
    },
    rename: (next: string) => {
      setName(next);
      if (next.trim()) {
        queue({ name: next.trim() });
      }
    },
    publish: () => act(() => publishProcedure(workspace, procedure.id)),
    unpublish: () => act(() => unpublishProcedure(workspace, procedure.id)),
    remove: async () => {
      await flush();
      try {
        await deleteProcedure(workspace, procedure.id);
        void refresh();
        onDeleted();
      } catch (error) {
        toast.error(error instanceof Error ? error.message : "The procedure could not be deleted.");
      }
    },
  };
}
