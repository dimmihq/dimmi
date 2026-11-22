/*
 * An agent's working copy while it is edited in Agent Studio, kept the
 * way a procedure's is. Every change shows at once and is saved shortly
 * after, one save at a time, each naming the lock version the last one
 * returned; a save refused because someone else saved first stops
 * saving, so their work is never overwritten. Publishing and pausing
 * save what is pending first. A copy the server wrote, such as a
 * restored version, is adopted whole.
 */
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { toast } from "@/components/base/toast/toast";
import {
  type Agent,
  type AgentDefinition,
  type AgentPatch,
  type AgentView,
  deleteAgent,
  pauseAgent,
  publishAgent,
  saveAgent,
} from "@/lib/api/agents";
import { ApiError } from "@/lib/api/client";

import type { SaveState } from "../automation/use-workflow-draft";

const pause = 600;

function pending(changes: AgentPatch): boolean {
  return Object.keys(changes).length > 0;
}

export type AgentDraft = ReturnType<typeof useAgentDraft>;

export function useAgentDraft(workspace: string, view: AgentView, onDeleted: () => void) {
  const queryClient = useQueryClient();
  const [server, setServer] = useState<Agent>(view.agent);
  const [draft, setDraft] = useState<AgentDefinition>(view.agent.draft);
  const [state, setState] = useState<SaveState>("saved");
  const [busy, setBusy] = useState(false);
  const [reasons, setReasons] = useState<string[]>([]);
  const [generation, setGeneration] = useState(0);
  const current = useRef(draft);
  const lock = useRef(view.agent.lock_version);
  const wanted = useRef<AgentPatch>({});
  const timer = useRef<number | undefined>(undefined);
  const running = useRef<Promise<void> | null>(null);
  const stopped = useRef(false);
  const refresh = () => queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "agents"] });

  const take = (next: AgentView) => {
    lock.current = next.agent.lock_version;
    setServer(next.agent);
    queryClient.setQueryData(["workspace", workspace, "agents", next.agent.id], next);
  };

  const once = async () => {
    const changes = wanted.current;
    wanted.current = {};
    if (stopped.current || !pending(changes)) {
      return;
    }
    try {
      take(await saveAgent(workspace, view.agent.id, { ...changes, lock_version: lock.current }));
      setState(pending(wanted.current) ? "saving" : "saved");
    } catch (error) {
      wanted.current = { ...changes, ...wanted.current };
      stopped.current = error instanceof ApiError && error.code === "edit_conflict";
      setState(stopped.current ? "conflict" : "failed");
      toast.error(error instanceof Error ? error.message : "The agent could not be saved.");
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

  const leave = useRef(() => flush().then(refresh));
  leave.current = () => flush().then(refresh);
  useEffect(() => {
    const onLeave = leave;

    return () => {
      void onLeave.current();
    };
  }, []);

  const act = async (work: () => Promise<AgentView>, done: string) => {
    setBusy(true);
    try {
      await flush();
      take(await work());
      setReasons([]);
      toast.success(done);
      void refresh();
    } catch (error) {
      const found = error instanceof ApiError && Array.isArray(error.details.reasons) ? error.details.reasons : [];
      setReasons(found as string[]);
      toast.error(error instanceof Error ? error.message : "That did not work.");
    } finally {
      setBusy(false);
    }
  };

  return {
    id: view.agent.id,
    draft,
    server,
    state,
    busy,
    reasons,
    generation,
    flush,
    /* change applies an edit to the working copy and queues it for saving. */
    change: (patch: AgentPatch) => {
      const next = {
        ...current.current,
        ...patch,
        placement: { ...current.current.placement, ...patch.placement },
      } as AgentDefinition;
      current.current = next;
      setDraft(next);
      setReasons([]);
      wanted.current = { ...wanted.current, ...patch };
      if (stopped.current) {
        return;
      }
      setState("saving");
      window.clearTimeout(timer.current);
      timer.current = window.setTimeout(() => void flush(), pause);
    },
    /* adopt takes a copy the server wrote, such as a restored version, as the working copy. */
    adopt: (next: AgentView) => {
      wanted.current = {};
      take(next);
      current.current = next.agent.draft;
      setDraft(next.agent.draft);
      setReasons([]);
      setState("saved");
      setGeneration((n) => n + 1);
      void refresh();
    },
    publish: (note = "") =>
      act(
        () => publishAgent(workspace, view.agent.id, note),
        server.status === "live" ? "Changes published" : `${current.current.name} is live`,
      ),
    pause: () => act(() => pauseAgent(workspace, view.agent.id), `${current.current.name} is off`),
    remove: async () => {
      await flush();
      try {
        await deleteAgent(workspace, view.agent.id);
        void refresh();
        onDeleted();
      } catch (error) {
        toast.error(error instanceof Error ? error.message : "The agent could not be deleted.");
      }
    },
  };
}
