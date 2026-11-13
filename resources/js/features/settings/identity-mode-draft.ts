/*
 * The draft of the workspace's sign-in mode: the mode picked but not
 * saved yet, and the save that sends it, held for the page's save bar.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { ApiError } from "@/lib/api/client";
import { type IdentityMode, identitySettingsQuery, updateIdentitySettings } from "@/lib/api/identity";

import { useRecentAuth } from "./recent-auth";

/*
 * useIdentityModeDraft holds the sign-in mode picked but not saved yet.
 * Save sends it, asking for the password first when the server wants a
 * recent one; closing that prompt keeps the draft. When the server
 * refuses required for too few signed-in starts, the section asks
 * whether to switch anyway. Discard puts the saved mode back.
 */
export function useIdentityModeDraft(workspace: string, hasPassword: boolean) {
  const queryClient = useQueryClient();
  const enabled = workspace !== "";
  const settings = useQuery({ ...identitySettingsQuery(workspace), enabled });
  const { run, prompt } = useRecentAuth(hasPassword);
  const [pending, setPending] = useState<IdentityMode | null>(null);
  const [refused, setRefused] = useState<ApiError | null>(null);
  const saved = settings.data?.mode ?? "optional";
  const save = useMutation({
    mutationFn: (force: boolean) =>
      run(() => updateIdentitySettings(workspace, { mode: pending ?? saved, ...(force ? { force } : {}) })),
    onSuccess: async (done) => {
      await queryClient.invalidateQueries({ queryKey: identitySettingsQuery(workspace).queryKey });
      if (done) {
        setPending(null);
      }
    },
    onError: (error) => {
      if (error instanceof ApiError && error.code === "rollout_guard") {
        setRefused(error);
      }
    },
  });

  return {
    enabled,
    loading: enabled && settings.isPending,
    mode: pending ?? saved,
    dirty: pending !== null,
    save,
    refused,
    prompt,
    error: save.error && !refused ? save.error.message : null,
    set: (mode: IdentityMode) => setPending(mode === saved ? null : mode),
    dismissRefusal: () => setRefused(null),
    discard: () => {
      save.reset();
      setRefused(null);
      setPending(null);
    },
  };
}

export type IdentityModeDraft = ReturnType<typeof useIdentityModeDraft>;
