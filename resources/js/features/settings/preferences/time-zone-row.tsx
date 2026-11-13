/*
 * The person's own time zone, or the workspace's when they chose none.
 * A pick waits in the Preferences page's draft until its save bar sends
 * it. A browser somewhere else only earns a quiet offer to switch, which
 * fills the draft too: nothing here ever changes the setting on its own.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";

import { updateProfile } from "@/lib/api/account";
import { workspaceQuery } from "@/lib/api/workspace";
import { browserZone, shouldOfferZone, zoneChoices } from "@/lib/location";

import { Action } from "../controls";
import { Row } from "../layout";
import { useAccount } from "../use-account";
import { type ZoneChoice, ZonePicker } from "./zone-picker";

/*
 * followWorkspace is the choice that clears the person's own time zone.
 */
const followWorkspace = "workspace-default";

/*
 * useTimeZoneDraft holds the person's time zone picked but not saved
 * yet: a zone, null to follow the workspace, or undefined while nothing
 * differs from what is saved. Save sends it and reads the person again;
 * Discard puts it back.
 */
export function useTimeZoneDraft() {
  const me = useAccount();
  const queryClient = useQueryClient();
  const [pending, setPending] = useState<string | null | undefined>(undefined);
  const save = useMutation({
    mutationFn: () => updateProfile({ timezone: pending ?? null }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["me"] });
      setPending(undefined);
    },
  });

  return {
    pending,
    dirty: pending !== undefined,
    save,
    pick: (zone: string | null) => setPending(zone === (me.timezone ?? null) ? undefined : zone),
    discard: () => setPending(undefined),
  };
}

export type TimeZoneDraft = ReturnType<typeof useTimeZoneDraft>;

export function TimeZoneRows({ draft }: { draft: TimeZoneDraft }) {
  const me = useAccount();
  const { data: workspace } = useQuery(workspaceQuery(me.workspace.id));
  const fallback = workspace?.timezone ?? "UTC";
  const browser = browserZone();
  const chosen = draft.pending === undefined ? me.timezone : draft.pending;

  const choices = useMemo<ZoneChoice[]>(
    () => [
      { id: followWorkspace, label: `Workspace default (${fallback})` },
      ...zoneChoices(chosen ?? null, Intl.supportedValuesOf("timeZone")).map((zone) => ({ id: zone, label: zone })),
    ],
    [fallback, chosen],
  );

  return (
    <>
      <Row label="Time zone" description="Used for times in the inbox and in notifications.">
        <ZonePicker
          key={`${chosen}-${fallback}`}
          label="Time zone"
          choices={choices}
          selected={chosen ?? followWorkspace}
          onPick={(key) => draft.pick(key === followWorkspace ? null : key)}
        />
      </Row>
      {shouldOfferZone(chosen ?? null, fallback, browser) ? (
        <Row label={`Your browser is on ${browser}`} description="Times show in the workspace's zone until you switch.">
          <Action onPress={() => draft.pick(browser)}>{`Use ${browser}`}</Action>
        </Row>
      ) : null}
    </>
  );
}
