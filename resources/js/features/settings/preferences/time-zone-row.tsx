/*
 * The person's own time zone, or the workspace's when they chose none.
 * A pick waits in the Preferences page's draft until its save bar sends
 * it. A browser somewhere else only earns a quiet offer to switch, which
 * fills the draft too: nothing here ever changes the setting on its own.
 */
import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import { workspaceQuery } from "@/lib/api/workspace";
import { browserZone, shouldOfferZone, zoneChoices } from "@/lib/location";

import { Action } from "../controls";
import { Row } from "../layout";
import { useAccount } from "../use-account";
import type { TimeZoneDraft } from "./time-zone-draft";
import { type ZoneChoice, ZonePicker } from "./zone-picker";

/*
 * followWorkspace is the choice that clears the person's own time zone.
 */
const followWorkspace = "workspace-default";

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
