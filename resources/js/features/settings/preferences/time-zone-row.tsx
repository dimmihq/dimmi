/*
 * The person's own time zone, or the workspace's when they chose none.
 * Picking saves at once. A browser somewhere else only earns a quiet offer
 * to switch: nothing here ever changes the setting on its own.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo } from "react";

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

export function TimeZoneRows() {
  const me = useAccount();
  const queryClient = useQueryClient();
  const { data: workspace } = useQuery(workspaceQuery(me.workspace.id));
  const fallback = workspace?.timezone ?? "UTC";
  const browser = browserZone();

  const save = useMutation({
    mutationFn: (timezone: string | null) => updateProfile({ timezone }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["me"] }),
  });

  const choices = useMemo<ZoneChoice[]>(
    () => [
      { id: followWorkspace, label: `Workspace default (${fallback})` },
      ...zoneChoices(me.timezone, Intl.supportedValuesOf("timeZone")).map((zone) => ({ id: zone, label: zone })),
    ],
    [fallback, me.timezone],
  );

  return (
    <>
      <Row label="Time zone" description={save.error?.message ?? "Used for times in the inbox and in notifications."}>
        <ZonePicker
          key={`${me.timezone}-${fallback}`}
          label="Time zone"
          choices={choices}
          selected={me.timezone ?? followWorkspace}
          onPick={(key) => save.mutate(key === followWorkspace ? null : key)}
        />
      </Row>
      {shouldOfferZone(me.timezone, fallback, browser) ? (
        <Row label={`Your browser is on ${browser}`} description="Times show in the workspace's zone until you switch.">
          <Action onPress={() => save.mutate(browser)}>{`Use ${browser}`}</Action>
        </Row>
      ) : null}
    </>
  );
}
