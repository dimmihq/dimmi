/*
 * How strict the workspace is about who its messenger visitors are, with
 * the numbers that say whether making sign-in required is safe. Switching
 * modes needs a recent password, since required can lock out every
 * customer and off stops trusting signed-in history. When too few starts
 * were signed in, the server refuses required and the owner can switch
 * anyway, knowing the numbers. The mode picked waits in the page's
 * draft until the save bar sends it.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { AlertDialog } from "@/components/base/modal/modal";
import { ApiError } from "@/lib/api/client";
import {
  type IdentityMode,
  identitySettingsQuery,
  identityStatsQuery,
  updateIdentitySettings,
} from "@/lib/api/identity";

import { Choice } from "./controls";
import { modeDetail, modeLabels, statsSummary, topFailures } from "./identity-format";
import { Row, Section } from "./layout";
import { useRecentAuth } from "./recent-auth";

const modes = Object.keys(modeLabels) as IdentityMode[];

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

/*
 * IdentityModeSection shows the mode in the page's draft beside the last
 * week's numbers; the page's save bar sends it.
 */
export function IdentityModeSection({ workspace, draft }: { workspace: string; draft: IdentityModeDraft }) {
  const stats = useQuery({ ...identityStatsQuery(workspace), enabled: draft.enabled });
  const mode = draft.mode;
  const summary = stats.data ? statsSummary(stats.data) : null;
  const failures = stats.data ? topFailures(stats.data) : [];

  return (
    <Section
      title="Sign-in mode"
      description="Applies to every messenger in this workspace."
      loading={draft.loading}
      rows={2}
    >
      <Row label="Mode" description={modeDetail(mode)}>
        <Choice
          key={mode}
          label="Sign-in mode"
          options={modes.map((m) => modeLabels[m])}
          value={modeLabels[mode]}
          disabled={draft.save.isPending}
          onChange={(label) => {
            const next = modes.find((m) => modeLabels[m] === label);
            if (next) {
              draft.set(next);
            }
          }}
        />
      </Row>
      <Row label="Last 7 days" description={summary ?? "No messenger starts yet."}>
        {failures.map((f) => (
          <Chip key={f.code} variant="caption" color="orange">
            {`${f.code} · ${f.count}`}
          </Chip>
        ))}
      </Row>
      {draft.prompt}
      <AlertDialog
        isOpen={draft.refused !== null}
        onOpenChange={(open) => !open && draft.dismissRefusal()}
        title="Make sign-in required anyway?"
        description={`${draft.refused?.message ?? ""} Visitors who are not signed in will no longer see the messenger.`}
        confirmLabel="Make required"
        onConfirm={() => {
          draft.dismissRefusal();
          draft.save.mutate(true);
        }}
      />
    </Section>
  );
}
