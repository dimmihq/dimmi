/*
 * ImportedTeammates lists the teammates brought over from another tool
 * who wait for access, with where each one stands, and lets an admin
 * enable them one by one or all at once. Enabling someone who signed up
 * lets them in straight away; anyone else gets an invitation by email.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { AlertDialog } from "@/components/base/modal/modal";
import { avatarColor } from "@/features/shell/avatar-color";
import { ApiError } from "@/lib/api/client";
import { teamsQuery } from "@/lib/api/teams";
import { type Member, membersQuery, updateMember } from "@/lib/api/workspace";
import { cx } from "@/utils/cx";

import { Item, Section } from "../layout";
import {
  actionLabel,
  type EnableText,
  enableAllText,
  enableText,
  importState,
  sectionLine,
  stateColors,
  stateLabels,
} from "./imported";

/*
 * Notice is the last thing enabling did, or why it failed.
 */
interface Notice {
  tone: "status" | "alert";
  text: string;
}

/*
 * Asked is what the confirmation dialog asks about: one teammate or
 * everyone, with its words.
 */
interface Asked {
  members: Member[];
  text: EnableText;
}

function problem(error: unknown): string {
  if (error instanceof ApiError) {
    return Object.values(error.fields)[0] ?? error.message;
  }

  return error instanceof Error ? error.message : "Something went wrong. Try again.";
}

/*
 * outcome says what enabling did, counting who got in and who was
 * invited.
 */
function outcome(enabled: Member[], failed: number): Notice {
  const now = enabled.filter((m) => importState(m) === "waiting").length;
  const invited = enabled.length - now;
  const single = enabled.length === 1 && failed === 0;
  const parts: string[] = [];
  if (now > 0) {
    parts.push(single ? `${enabled[0].user.name} is in` : `${now} enabled`);
  }
  if (invited > 0) {
    parts.push(single ? `Invitation sent to ${enabled[0].user.email}` : `${invited} invited`);
  }
  if (failed > 0) {
    parts.push(`${failed} couldn't be enabled, try again`);
  }

  return { tone: failed > 0 ? "alert" : "status", text: `${parts.join(". ")}.` };
}

export function ImportedTeammates({ workspace }: { workspace: string }) {
  const queryClient = useQueryClient();
  const { data: waiting = [] } = useQuery(membersQuery(workspace, { awaitingEnable: true }));
  const { data: teams = [] } = useQuery(teamsQuery(workspace));
  const [asked, setAsked] = useState<Asked | null>(null);
  const [notice, setNotice] = useState<Notice | null>(null);

  const teamNames = (member: Member) =>
    (member.enable_team_ids ?? []).flatMap((id) => teams.filter((t) => t.id === id).map((t) => t.name));

  const enable = useMutation({
    mutationFn: async (members: Member[]) => {
      const results = await Promise.allSettled(
        members.map((member) => updateMember(workspace, member.id, { status: "active" })),
      );
      const enabled = members.filter((_, i) => results[i].status === "fulfilled");
      const failure = results.find((r) => r.status === "rejected");
      if (enabled.length === 0 && failure) {
        throw failure.reason;
      }

      return { enabled, failed: members.length - enabled.length };
    },
    onSuccess: ({ enabled, failed }) => setNotice(outcome(enabled, failed)),
    onError: (error) => setNotice({ tone: "alert", text: problem(error) }),
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["workspace", workspace] }),
  });

  if (waiting.length === 0 && notice === null) {
    return null;
  }

  return (
    <>
      {waiting.length > 0 ? (
        <Section
          title="Imported"
          description={sectionLine(waiting)}
          action={
            waiting.length > 1 ? (
              <Button
                variant="secondary"
                size="small"
                onClick={() => setAsked({ members: waiting, text: enableAllText(waiting) })}
              >
                Enable all
              </Button>
            ) : null
          }
        >
          {waiting.map((member) => {
            const state = importState(member);

            return (
              <Item
                key={member.id}
                mark={<Avatar size="md" color={avatarColor(member.user.name)} initials={member.user.name.charAt(0)} />}
                title={member.user.name}
                detail={
                  <>
                    <span className="sm:hidden">{stateLabels[state]}</span>
                    <span className="hidden sm:inline">{member.user.email}</span>
                  </>
                }
              >
                <Chip variant="caption" color={stateColors[state]} className="hidden sm:inline-flex">
                  {stateLabels[state]}
                </Chip>
                <Chip variant="caption" color="neutral" className="hidden sm:inline-flex">
                  {member.role === "admin" ? "Admin" : "Member"}
                </Chip>
                <Button
                  variant="secondary"
                  size="small"
                  aria-label={`${actionLabel(member)} ${member.user.name}`}
                  onClick={() => setAsked({ members: [member], text: enableText(member, teamNames(member)) })}
                >
                  {actionLabel(member)}
                </Button>
              </Item>
            );
          })}
        </Section>
      ) : null}
      {notice ? (
        <p
          role={notice.tone}
          className={cx(
            "px-3 text-body-2-regular",
            notice.tone === "alert" ? "text-text-error-primary" : "text-text-secondary",
          )}
        >
          {notice.text}
        </p>
      ) : null}
      <AlertDialog
        isOpen={asked !== null}
        onOpenChange={(open) => (open ? null : setAsked(null))}
        variant="default"
        title={asked?.text.title ?? ""}
        description={asked?.text.description}
        confirmLabel={asked?.text.confirm}
        onConfirm={() => (asked ? enable.mutateAsync(asked.members).catch(() => undefined) : undefined)}
      />
    </>
  );
}
