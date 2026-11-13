/*
 * The draft of a connected engineering tool's settings: the picks and the
 * pasted secret changed but not saved yet, held for the integration
 * page's save bar.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { toast } from "@/components/base/toast/toast";
import {
  type IssueSettings,
  type IssueSettingsChange,
  issueKeys,
  issueSettingsQuery,
  updateIssueSettings,
} from "@/lib/api/ticket-issues";

/*
 * useIssueSyncDraft holds the tool's settings changed but not saved yet,
 * laid over them as saved. A change back to the saved value drops out,
 * and so does an empty secret. Save sends the rest in one update.
 */
export function useIssueSyncDraft(workspace: string, installation: string, enabled: boolean) {
  const queryClient = useQueryClient();
  const { data: saved } = useQuery({ ...issueSettingsQuery(workspace, installation), enabled });
  const [pending, setPending] = useState<IssueSettingsChange>({});
  const save = useMutation({
    mutationFn: () =>
      updateIssueSettings(workspace, installation, {
        ...pending,
        ...(pending.signing_secret ? { signing_secret: pending.signing_secret.trim() } : {}),
      }),
    onSuccess: (next) => {
      queryClient.setQueryData(issueKeys.settings(workspace, installation), next);
      setPending({});
      toast.success("Sync settings saved");
    },
  });
  const { signing_secret: secret = "", ...picked } = pending;

  return {
    settings: saved && { ...saved, ...picked },
    secret,
    dirty: Object.keys(pending).length > 0,
    save,
    set: (change: IssueSettingsChange) =>
      setPending((before) => {
        const next: Record<string, unknown> = { ...before, ...change };
        for (const [key, value] of Object.entries(change)) {
          const same =
            key === "signing_secret"
              ? String(value).trim() === ""
              : JSON.stringify(value) === JSON.stringify(saved?.[key as keyof IssueSettings]);
          if (same) {
            delete next[key];
          }
        }

        return next as IssueSettingsChange;
      }),
    discard: () => {
      save.reset();
      setPending({});
    },
  };
}

export type IssueSyncDraft = ReturnType<typeof useIssueSyncDraft>;
