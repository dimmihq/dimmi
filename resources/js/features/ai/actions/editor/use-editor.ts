/*
 * The action editor's state: the working copy as typed, the last saved
 * action, what is wrong with the copy, and saving and publishing. A new
 * action is created on its first save and the page moves to its own
 * address. Problems show beside their fields only once the person tried
 * to save, so a fresh form is not covered in red; the server's own
 * refusals land beside the same fields. Publishing saves first when
 * there are unsaved changes, then freezes the copy as a new version.
 */
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { toast } from "@/components/base/toast/toast";
import { ApiError } from "@/lib/api/client";

import {
  type ActionConnection,
  type AiAction,
  actionQuery,
  actionsKey,
  createAction,
  publishAction,
  restoreVersion,
  updateAction,
} from "../api";
import { changed, type Draft, definitionOf, draftOf, newDraft, type Problems, problemsOf } from "../draft";

const fieldNames: Record<string, string> = {
  path_template: "path",
  mcp_tool_name: "tool",
  input_schema: "fields",
  requires_verified_contact: "identity",
};

function serverProblems(error: unknown): Problems {
  if (!(error instanceof ApiError)) {
    return {};
  }

  return Object.fromEntries(
    Object.entries(error.fields).map(([field, message]) => [fieldNames[field.split(".")[0] ?? ""] ?? field, message]),
  );
}

export function useEditor(workspace: string, action: AiAction | null, connection: ActionConnection) {
  const client = useQueryClient();
  const navigate = useNavigate();
  const [draft, setDraft] = useState<Draft>(() => (action ? draftOf(action) : newDraft(connection)));
  const [saved, setSaved] = useState<AiAction | null>(action);
  const [tried, setTried] = useState(false);
  const [refused, setRefused] = useState<Problems>({});
  const [busy, setBusy] = useState<"save" | "publish" | null>(null);
  const problems = problemsOf(draft, connection);
  const dirty = changed(draft, saved, connection.kind);
  const shown: Problems = tried ? { ...problems, ...refused } : refused;
  const refresh = () => client.invalidateQueries({ queryKey: actionsKey(workspace) });

  const update = (change: Partial<Draft> | ((before: Draft) => Draft)) => {
    setDraft((before) => (typeof change === "function" ? change(before) : { ...before, ...change }));
    setRefused({});
  };

  const store = async (): Promise<AiAction | null> => {
    setTried(true);
    if (Object.keys(problems).length > 0) {
      toast.error("Not saved", { description: "Fix the fields marked in red first." });
      return null;
    }
    try {
      const body = definitionOf(draft, connection.kind);
      const result = saved ? await updateAction(workspace, saved.id, body) : await createAction(workspace, body);
      setSaved(result);
      client.setQueryData(actionQuery(workspace, result.id).queryKey, result);
      await refresh();
      if (!saved) {
        await navigate({ to: "/d/settings/actions/$action", params: { action: result.id }, replace: true });
      }
      return result;
    } catch (error) {
      setRefused(serverProblems(error));
      toast.error("Not saved", { description: error instanceof ApiError ? error.message : "Try again." });
      return null;
    }
  };

  const save = async () => {
    setBusy("save");
    const result = await store();
    setBusy(null);
    if (result) {
      toast.success("Draft saved", {
        description: result.published_version ? "Customers still get the live version until you publish." : undefined,
      });
    }
    return result;
  };

  const publish = async () => {
    setBusy("publish");
    try {
      const current = dirty || !saved ? await store() : saved;
      if (!current) {
        return;
      }
      const version = await publishAction(workspace, current.id);
      setSaved({ ...current, status: "published", published_version: version, has_unpublished_changes: false });
      toast.success(`Version ${version.number} is live`, { description: "dimmi uses it from the next message." });
      await refresh();
    } catch (error) {
      toast.error("Not published", { description: error instanceof ApiError ? error.message : "Try again." });
    } finally {
      setBusy(null);
    }
  };

  const restore = async (version: { id: string; number: number }) => {
    if (!saved) {
      return;
    }
    try {
      const result = await restoreVersion(workspace, saved.id, version.id);
      setSaved(result);
      setDraft(draftOf(result));
      toast.success(`Version ${version.number} restored as the draft`, {
        description: "Publish to make it live again.",
      });
      await refresh();
    } catch (error) {
      toast.error("Not restored", { description: error instanceof ApiError ? error.message : "Try again." });
    }
  };

  return { draft, update, saved, setSaved, problems: shown, dirty, busy, save, publish, restore, store };
}

export type Editor = ReturnType<typeof useEditor>;
