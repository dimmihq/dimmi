/*
 * How strict the workspace is about who its messenger visitors are, with
 * the numbers that say whether making sign-in required is safe. Switching
 * modes needs a recent password, since required can lock out every
 * customer and off stops trusting signed-in history. When too few starts
 * were signed in, the server refuses required and the owner can switch
 * anyway, knowing the numbers. The mode picked waits in the page's
 * draft until the save bar sends it.
 */
import { useQuery } from "@tanstack/react-query";

import { Chip } from "@/components/base/badges/chip";
import { AlertDialog } from "@/components/base/modal/modal";
import { type IdentityMode, identityStatsQuery } from "@/lib/api/identity";

import { Choice } from "./controls";
import { modeDetail, modeLabels, statsSummary, topFailures } from "./identity-format";
import type { IdentityModeDraft } from "./identity-mode-draft";
import { Row, Section } from "./layout";

const modes = Object.keys(modeLabels) as IdentityMode[];

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
