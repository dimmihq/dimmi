/*
 * The working copy of a workflow while the builder edits it. Every change
 * shows at once and is saved shortly after, one save at a time, each
 * naming the lock version the last one returned. A save refused because
 * someone else saved first stops saving, so their work is never
 * overwritten; reloading shows theirs. Publishing first saves what is
 * pending, then publishes exactly the copy that was saved, and what the
 * server found wrong with it shows beside the steps.
 */
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { toast } from "@/components/base/toast/toast";
import { ApiError } from "@/lib/api/client";
import {
  archiveWorkflow,
  type Issue,
  publishWorkflow,
  type Report,
  type Spec,
  saveWorkflow,
  setWorkflowStatus,
  type Workflow,
} from "@/lib/api/workflows";

export type SaveState = "saved" | "saving" | "conflict" | "failed";

interface Changes {
  spec?: Spec;
  name?: string;
}

const pause = 600;

const empty: Report = { errors: [], warnings: [] };

function issuesOf(error: unknown): Issue[] {
  if (!(error instanceof ApiError) || !Array.isArray(error.details.errors)) {
    return [];
  }

  return error.details.errors as Issue[];
}

function hasChanges(changes: Changes): boolean {
  return changes.spec !== undefined || changes.name !== undefined;
}

/*
 * useSaver queues changes and saves them one at a time.
 */
function useSaver(workspace: string, workflow: Workflow, onSaved: (next: Workflow) => void) {
  const [state, setState] = useState<SaveState>("saved");
  const lock = useRef(workflow.lock_version);
  const wanted = useRef<Changes>({});
  const timer = useRef<number | undefined>(undefined);
  const running = useRef<Promise<void> | null>(null);
  const stopped = useRef(false);

  const refused = (changes: Changes, error: unknown) => {
    wanted.current = { ...changes, ...wanted.current };
    const conflict = error instanceof ApiError && error.code === "edit_conflict";
    stopped.current = conflict;
    setState(conflict ? "conflict" : "failed");
    toast.error(error instanceof Error ? error.message : "The workflow could not be saved.");
  };

  const once = async () => {
    const changes = wanted.current;
    wanted.current = {};
    if (stopped.current || !hasChanges(changes)) {
      return;
    }
    try {
      const next = await saveWorkflow(workspace, workflow.id, { ...changes, lock_version: lock.current });
      lock.current = next.lock_version;
      onSaved(next);
      setState(hasChanges(wanted.current) ? "saving" : "saved");
    } catch (error) {
      refused(changes, error);
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

  return { state, queue, flush };
}

export function useWorkflowDraft(workspace: string, workflow: Workflow, onArchived: () => void) {
  const queryClient = useQueryClient();
  const [spec, setSpec] = useState<Spec>(() => workflow.spec as Spec);
  const [name, setName] = useState(workflow.name);
  const [server, setServer] = useState(workflow);
  const [found, setFound] = useState<Issue[] | null>(null);
  const [busy, setBusy] = useState(false);
  const current = useRef(spec);
  const latest = useRef(workflow);
  const refresh = () => queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "workflows"] });
  const saver = useSaver(workspace, workflow, (next) => {
    latest.current = next;
    setServer(next);
    setFound(null);
  });
  const leave = useRef(() => saver.flush().then(refresh));
  leave.current = () => saver.flush().then(refresh);

  useEffect(() => {
    const flushOnLeave = leave;

    return () => {
      void flushOnLeave.current();
    };
  }, []);

  const act = async (work: () => Promise<Workflow>) => {
    setBusy(true);
    try {
      await saver.flush();
      const next = await work();
      latest.current = next;
      setServer(next);
      setFound(null);
      void refresh();
    } catch (error) {
      const issues = issuesOf(error);
      setFound(issues.length > 0 ? issues : null);
      toast.error(error instanceof Error ? error.message : "That did not work.");
    } finally {
      setBusy(false);
    }
  };

  const report = found ? { errors: found, warnings: [] } : (server.report ?? empty);

  return {
    spec,
    name,
    server,
    report,
    busy,
    saveState: saver.state,
    change: (edit: (spec: Spec) => Spec) => {
      const next = edit(current.current);
      current.current = next;
      setSpec(next);
      saver.queue({ spec: next });
    },
    rename: (next: string) => {
      setName(next);
      if (next.trim()) {
        saver.queue({ name: next.trim() });
      }
    },
    publish: () =>
      act(async () => {
        const done = await publishWorkflow(workspace, workflow.id, {
          draft_hash: latest.current.draft_hash,
          acknowledge_warnings: true,
        });

        return done.workflow;
      }),
    setStatus: (status: "live" | "paused") => act(() => setWorkflowStatus(workspace, workflow.id, status)),
    archive: async () => {
      await saver.flush();
      try {
        await archiveWorkflow(workspace, workflow.id);
        void refresh();
        onArchived();
      } catch (error) {
        toast.error(error instanceof Error ? error.message : "The workflow could not be archived.");
      }
    },
  };
}
