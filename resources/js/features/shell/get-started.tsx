/*
 * The first steps of a new workspace, at the foot of the sidebar. Each
 * step checks itself off from what the workspace has done: a messenger
 * that booted on a site, mail that arrived, a teammate invited, knowledge
 * for the agent, and a coding agent connected over MCP. Each leads to the
 * page that sets it up. The list starts folded, so the sidebar's own
 * pages stay in view on short screens; the person opens it with one
 * click, and the browser remembers. It shows to people who can change
 * settings, and goes away once every step is done.
 */
import { RiArrowDownSLine, RiCheckboxBlankCircleLine, RiCheckboxCircleFill } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import { connectedAppsQuery } from "@/lib/api/connected-apps";
import { emailAddressesQuery } from "@/lib/api/email";
import { invitationsQuery } from "@/lib/api/invitations";
import { sourcesQuery } from "@/lib/api/knowledge-sources";
import { widgetSettingsQuery } from "@/lib/api/widget-settings";
import { callerQuery, membersQuery } from "@/lib/api/workspace";
import { cx } from "@/utils/cx";

const key = "dimmi:get-started-folded";

function folded(): boolean {
  try {
    return window.localStorage.getItem(key) !== "0";
  } catch {
    return true;
  }
}

interface Step {
  label: string;
  href: string;
  done: boolean;
}

/*
 * useSteps reads what the workspace has set up so far.
 */
function useSteps(workspace: string, enabled: boolean): Step[] | null {
  const on = { enabled: enabled && workspace !== "", retry: false } as const;
  const widgets = useQuery({ ...widgetSettingsQuery(workspace), ...on });
  const addresses = useQuery({ ...emailAddressesQuery(workspace), ...on });
  const members = useQuery({ ...membersQuery(workspace), ...on });
  const invitations = useQuery({ ...invitationsQuery(workspace), ...on });
  const sources = useQuery({ ...sourcesQuery(workspace), ...on });
  const apps = useQuery({ ...connectedAppsQuery, enabled: on.enabled, retry: false });

  if (!enabled || widgets.isPending || addresses.isPending || members.isPending) {
    return null;
  }

  return [
    {
      label: "Install the messenger",
      href: "/d/settings/messenger",
      done: (widgets.data ?? []).some((widget) => widget.last_boot_at),
    },
    {
      label: "Connect your email",
      href: "/d/settings/email",
      done: (addresses.data ?? []).some((address) => address.last_inbound_at || address.inbound_verified_at),
    },
    {
      label: "Invite your team",
      href: "/d/settings/members",
      done: (members.data?.length ?? 0) > 1 || (invitations.data?.length ?? 0) > 0,
    },
    {
      label: "Add knowledge for the agent",
      href: "/d/settings/knowledge",
      done: (sources.data?.length ?? 0) > 0,
    },
    {
      label: "Connect a coding agent over MCP",
      href: "/d/settings/connected-apps",
      done: (apps.data ?? []).some((app) => !app.workspace_id || app.workspace_id === workspace),
    },
  ];
}

export function GetStarted({ workspace }: { workspace: string }) {
  const [isFolded, setFolded] = useState(folded);
  const { data: caller } = useQuery({ ...callerQuery(workspace), enabled: workspace !== "" });
  const steps = useSteps(workspace, caller?.member?.permissions?.includes("settings:read") ?? false);

  if (!steps) {
    return null;
  }
  const done = steps.filter((step) => step.done).length;
  if (done === steps.length) {
    return null;
  }

  const toggle = () => {
    setFolded(!isFolded);
    try {
      window.localStorage.setItem(key, isFolded ? "0" : "1");
    } catch {
      // Blocked storage keeps the choice for this page only.
    }
  };

  return (
    <section aria-label="Get started" className="flex w-full flex-col rounded-xl bg-background-tertiary-default p-2">
      <button
        type="button"
        aria-expanded={!isFolded}
        onClick={toggle}
        className="flex w-full cursor-pointer items-center justify-between rounded-lg px-1 py-0.5 outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <span className="text-body-medium text-text-primary">Get started</span>
        <span className="flex items-center gap-1 text-caption-1-medium text-text-tertiary">
          {done}/{steps.length}
          <RiArrowDownSLine
            aria-hidden
            className={cx("size-4 transition-transform duration-200", isFolded && "-rotate-90")}
          />
        </span>
      </button>
      {isFolded ? null : (
        <ul className="mt-1.5 flex flex-col gap-0.5">
          {steps.map((step) => (
            <li key={step.label}>
              <Link
                to={step.href}
                className={cx(
                  "flex items-center gap-2 rounded-lg px-1 py-1 text-body-regular outline-none transition-colors",
                  "hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring",
                  step.done ? "text-text-tertiary line-through" : "text-text-secondary",
                )}
              >
                {step.done ? (
                  <RiCheckboxCircleFill aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
                ) : (
                  <RiCheckboxBlankCircleLine aria-hidden className="size-4 shrink-0 text-foreground-icon-tertiary" />
                )}
                {step.label}
                <span className="sr-only">{step.done ? " (done)" : ""}</span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
