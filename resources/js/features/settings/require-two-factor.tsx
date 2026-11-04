/*
 * Requiring two-factor authentication from everyone in the workspace.
 * Members without it are kept out until they add an authenticator, so
 * the server only lets someone who has one switch it, and asks for the
 * password again when the last sign-in is a while ago. A refused change
 * puts the switch back where the server has it. Before it is on, the
 * section says how many members would be kept out and links to them.
 */
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import { ApiError } from "@/lib/api/client";
import { callerQuery, membersQuery, updateWorkspace, workspaceQuery } from "@/lib/api/workspace";

import { Toggle } from "./controls";
import { Row, Section } from "./layout";
import { unprotectedLine } from "./members/member-review";
import { useRecentAuth } from "./recent-auth";

const label = "Require two-factor authentication";

/*
 * refusal says why the server kept the requirement as it was.
 */
function refusal(error: unknown): string {
  if (error instanceof ApiError && error.code === "two_factor_not_enabled") {
    return "Turn on two-factor authentication for your own account first, under Security & access.";
  }

  return error instanceof Error ? error.message : "Something went wrong. Try again.";
}

export function RequireTwoFactorSection({
  workspace,
  hasPassword,
  title = "Security",
}: {
  workspace: string;
  hasPassword: boolean;
  title?: string;
}) {
  const queryClient = useQueryClient();
  const { data } = useQuery(workspaceQuery(workspace));
  const { data: caller } = useQuery(callerQuery(workspace));
  const manages = caller?.member?.permissions.includes("members:write") ?? false;
  const { data: unprotected = [] } = useQuery({
    ...membersQuery(workspace, { twoFactor: false }),
    enabled: manages && data !== undefined && !data.require_two_factor,
  });
  const waiting = unprotectedLine(unprotected.filter((member) => member.status === "active").length);
  const { run, prompt } = useRecentAuth(hasPassword);
  const [failure, setFailure] = useState<string | null>(null);
  const [reset, setReset] = useState(0);

  const change = async (on: boolean) => {
    setFailure(null);

    try {
      const done = await run(async () => {
        const saved = await updateWorkspace(workspace, { require_two_factor: on });
        queryClient.setQueryData(workspaceQuery(workspace).queryKey, saved);
      });

      if (!done) {
        setReset((n) => n + 1);
      }
    } catch (error) {
      setFailure(refusal(error));
      setReset((n) => n + 1);
    }
  };

  return (
    <Section title={title} description="Applies to everyone in the workspace, API tokens included.">
      <Row
        label={label}
        description="Members without it cannot open the workspace until they add an authenticator app."
      >
        {data ? (
          <Toggle
            key={`${data.require_two_factor}-${reset}`}
            label={label}
            on={data.require_two_factor}
            onChange={(on) => void change(on)}
          />
        ) : null}
      </Row>
      {waiting && data && !data.require_two_factor ? (
        <p className="flex flex-wrap gap-1 py-2.5 pe-3 text-body-2-regular text-text-secondary">
          <span>{waiting}</span>
          <Link
            to="/d/settings/$section"
            params={{ section: "members" }}
            search={{ two_factor: false }}
            className="text-text-primary underline outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            See who
          </Link>
        </p>
      ) : null}
      {failure ? (
        <p role="alert" className="py-2.5 pe-3 text-body-2-regular text-text-error-primary">
          {failure}
        </p>
      ) : null}
      {prompt}
    </Section>
  );
}
