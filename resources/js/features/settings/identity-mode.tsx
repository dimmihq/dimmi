/*
 * How strict the workspace is about who its messenger visitors are, with
 * the numbers that say whether making sign-in required is safe. Switching
 * modes needs a recent password, since required can lock out every
 * customer and off stops trusting signed-in history. When too few starts
 * were signed in, the server refuses required and the owner can switch
 * anyway, knowing the numbers.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { AlertDialog } from "@/components/base/modal/modal";
import { Notification } from "@/components/base/notification/notification";
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

export function IdentityModeSection({ workspace, hasPassword }: { workspace: string; hasPassword: boolean }) {
  const queryClient = useQueryClient();
  const enabled = workspace !== "";
  const settings = useQuery({ ...identitySettingsQuery(workspace), enabled });
  const stats = useQuery({ ...identityStatsQuery(workspace), enabled });
  const { run, prompt } = useRecentAuth(hasPassword);
  const [refused, setRefused] = useState<ApiError | null>(null);
  const change = useMutation({
    mutationFn: (next: { mode: IdentityMode; force?: boolean }) => run(() => updateIdentitySettings(workspace, next)),
    onError: (error) => {
      if (error instanceof ApiError && error.code === "rollout_guard") {
        setRefused(error);
      }
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: identitySettingsQuery(workspace).queryKey }),
  });
  const mode = settings.data?.mode ?? "optional";
  const summary = stats.data ? statsSummary(stats.data) : null;
  const failures = stats.data ? topFailures(stats.data) : [];
  const failure = change.error && !refused ? change.error : null;

  return (
    <Section
      title="Sign-in mode"
      description="Applies to every messenger in this workspace."
      loading={enabled && settings.isPending}
      rows={2}
    >
      {failure ? (
        <div className="py-2.5 pe-2.5">
          <Notification status="error" title={failure.message} dismissible={false} role="alert" className="w-full" />
        </div>
      ) : null}
      <Row label="Mode" description={modeDetail(mode)}>
        <Choice
          key={`${mode}-${change.submittedAt}`}
          label="Sign-in mode"
          options={modes.map((m) => modeLabels[m])}
          value={modeLabels[mode]}
          disabled={change.isPending}
          onChange={(label) => {
            const next = modes.find((m) => modeLabels[m] === label);
            if (next && next !== mode) {
              change.mutate({ mode: next });
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
      {prompt}
      <AlertDialog
        isOpen={refused !== null}
        onOpenChange={(open) => !open && setRefused(null)}
        title="Make sign-in required anyway?"
        description={`${refused?.message ?? ""} Visitors who are not signed in will no longer see the messenger.`}
        confirmLabel="Make required"
        onConfirm={() => {
          setRefused(null);
          change.mutate({ mode: "required", force: true });
        }}
      />
    </Section>
  );
}
