/*
 * An audience being edited. Changes stay in the browser until saved, so
 * a half-written rule never reaches surfaces that use the audience; the
 * preview follows every edit 400 ms after it, through the same evaluator
 * the server uses. Saving names the lock version the editor started from
 * and marks the conditions the server refused where they are.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";

import {
  type Audience,
  type AudiencePreview,
  type AudienceRules,
  audienceQuery,
  audiencesQuery,
  deleteAudience,
  previewAudience,
  type RuleProblem,
  saveAudience,
} from "@/lib/api/audiences";
import { ApiError } from "@/lib/api/client";

import { type Form, toForm, toRules } from "./rules";

/*
 * problemsOf reads the refused conditions of a failed request by path,
 * and the problems no condition holds.
 */
export function problemsOf(error: unknown): { byPath: Record<string, string>; rest: string[] } {
  const byPath: Record<string, string> = {};
  const rest: string[] = [];
  const list = error instanceof ApiError ? (error.details.errors as RuleProblem[] | undefined) : undefined;
  for (const p of list ?? []) {
    if (p.path && p.path !== "rules" && p.path !== "") {
      byPath[p.path] = p.message;
    } else {
      rest.push(p.message);
    }
  }
  if (error instanceof Error && !list) {
    rest.push(error.message);
  }

  return { byPath, rest };
}

/*
 * sameJSON compares two JSON values whatever the order of their keys,
 * since the database stores objects with its own key order.
 */
export function sameJSON(a: unknown, b: unknown): boolean {
  return JSON.stringify(sorted(a)) === JSON.stringify(sorted(b));
}

function sorted(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(sorted);
  }
  if (value && typeof value === "object") {
    return Object.fromEntries(
      Object.keys(value)
        .sort()
        .map((key) => [key, sorted((value as Record<string, unknown>)[key])]),
    );
  }

  return value;
}

export function useAudienceDraft(workspace: string, audience: Audience, onDeleted: () => void) {
  const queryClient = useQueryClient();
  const [server, setServer] = useState(audience);
  const [name, setName] = useState(audience.name);
  const [description, setDescription] = useState(audience.description ?? "");
  const [form, setForm] = useState<Form | null>(() => toForm(audience.rules));
  const [preview, setPreview] = useState<AudiencePreview>();
  const [previewError, setPreviewError] = useState<unknown>(null);
  const [previewing, setPreviewing] = useState(false);

  const rules: AudienceRules = useMemo(() => (form ? toRules(form) : server.rules), [form, server.rules]);
  const rulesKey = JSON.stringify(rules);
  const changed = name !== server.name || description !== (server.description ?? "") || !sameJSON(rules, server.rules);

  useEffect(() => {
    let current = true;
    setPreviewing(true);
    const timer = setTimeout(() => {
      previewAudience(workspace, JSON.parse(rulesKey) as AudienceRules)
        .then((p) => {
          if (current) {
            setPreview(p);
            setPreviewError(null);
          }
        })
        .catch((error: unknown) => current && setPreviewError(error))
        .finally(() => current && setPreviewing(false));
    }, 400);

    return () => {
      current = false;
      clearTimeout(timer);
    };
  }, [workspace, rulesKey]);

  const save = useMutation({
    mutationFn: () =>
      saveAudience(workspace, server.id, { name, description, rules, lock_version: server.lock_version }),
    onSuccess: (saved) => {
      setServer({ ...saved, used_by: server.used_by });
      void queryClient.invalidateQueries({ queryKey: audiencesQuery(workspace).queryKey, exact: true });
      void queryClient.invalidateQueries({ queryKey: audienceQuery(workspace, server.id).queryKey });
    },
  });

  const remove = useMutation({
    mutationFn: () => deleteAudience(workspace, server.id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: audiencesQuery(workspace).queryKey });
      onDeleted();
    },
  });

  const problems = problemsOf(save.error ?? previewError);

  return {
    server,
    name,
    setName,
    description,
    setDescription,
    form,
    setForm,
    rules,
    changed,
    preview,
    previewing,
    problems,
    save,
    remove,
  };
}

export type AudienceDraft = ReturnType<typeof useAudienceDraft>;
