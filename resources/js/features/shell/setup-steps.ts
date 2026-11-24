/*
 * The first steps of a new workspace, each checked off from what the
 * workspace has done: a messenger that booted on a site, mail that
 * arrived, a teammate invited, knowledge for the agent, and a coding
 * agent connected over MCP. The sidebar's checklist and Home both read
 * them here, so they never disagree.
 */
import { useQuery } from "@tanstack/react-query";

import { connectedAppsQuery } from "@/lib/api/connected-apps";
import { emailAddressesQuery } from "@/lib/api/email";
import { invitationsQuery } from "@/lib/api/invitations";
import { sourcesQuery } from "@/lib/api/knowledge-sources";
import { widgetSettingsQuery } from "@/lib/api/widget-settings";
import { membersQuery } from "@/lib/api/workspace";

export interface Step {
  label: string;
  /* why says what the step gets the workspace, for where there is room to say it. */
  why: string;
  href: string;
  done: boolean;
}

/* How often an open list asks again, so a step done elsewhere ticks on its own. */
const checkEvery = 10_000;

/*
 * useSteps reads what the workspace has set up so far. It is null while
 * that loads, and for a person who may not change settings. While
 * watching it asks again every few seconds.
 */
export function useSteps(workspace: string, enabled: boolean, watching = false): Step[] | null {
  const on = {
    enabled: enabled && workspace !== "",
    retry: false,
    refetchInterval: watching ? checkEvery : (false as const),
  };
  const widgets = useQuery({ ...widgetSettingsQuery(workspace), ...on });
  const addresses = useQuery({ ...emailAddressesQuery(workspace), ...on });
  const members = useQuery({ ...membersQuery(workspace), ...on });
  const invitations = useQuery({ ...invitationsQuery(workspace), ...on });
  const sources = useQuery({ ...sourcesQuery(workspace), ...on });
  const apps = useQuery({ ...connectedAppsQuery, ...on });

  if (!enabled || widgets.isPending || addresses.isPending || members.isPending) {
    return null;
  }

  return [
    {
      label: "Install the messenger",
      why: "Customers write to you from your site or app.",
      href: "/d/settings/messenger",
      done: (widgets.data ?? []).some((widget) => widget.last_boot_at),
    },
    {
      label: "Connect your email",
      why: "Mail to your support address lands in the inbox.",
      href: "/d/settings/email",
      done: (addresses.data ?? []).some((address) => address.last_inbound_at || address.inbound_verified_at),
    },
    {
      label: "Invite your team",
      why: "Conversations get assigned and nobody answers twice.",
      href: "/d/settings/members",
      done: (members.data?.length ?? 0) > 1 || (invitations.data?.length ?? 0) > 0,
    },
    {
      label: "Add knowledge for the agent",
      why: "Point it at your docs and it answers from them.",
      href: "/d/settings/knowledge",
      done: (sources.data?.length ?? 0) > 0,
    },
    {
      label: "Connect a coding agent over MCP",
      why: "Your own agent reads and acts on the workspace.",
      href: "/d/settings/connected-apps",
      done: (apps.data ?? []).some((app) => !app.workspace_id || app.workspace_id === workspace),
    },
  ];
}
