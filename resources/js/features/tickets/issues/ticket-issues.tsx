/*
 * The engineering work behind a ticket, for the ticket panel and the
 * tracker and incident pages: each linked issue or incident with its
 * tool, key, title and status as last seen, and any warning, such as a
 * Linear team the app can no longer see, so nothing drops silently.
 * Linking opens the two-step dialog; unlinking asks first and leaves the
 * issue in its tool. Below, the drafts that close the loop.
 */
import { RiAddLine, RiErrorWarningLine, RiExternalLinkLine, RiLinkUnlinkM } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { useAccount } from "@/features/settings/use-account";
import { integrationsQuery } from "@/lib/api/integrations";
import { refreshTicketKeys, ticketIssuesQuery, unlinkTicketIssue } from "@/lib/api/ticket-issues";
import { type TicketIssue, ticketQuery } from "@/lib/api/tickets";

import { CloseLoopDrafts } from "./close-loop-drafts";
import { LinkIssueDialog } from "./link-issue-dialog";
import { groupTint, isTool, toolLook } from "./tools";

function IssueRow({ issue, compact, onUnlink }: { issue: TicketIssue; compact: boolean; onUnlink: () => void }) {
  const look = toolLook(issue.provider);
  const problem = issue.warning ?? issue.sync_error;

  return (
    <li className={compact ? "flex flex-col gap-1 py-1" : "flex flex-col gap-1 px-4 py-3"}>
      <div className={compact ? "flex items-center gap-2" : "flex items-center gap-3"}>
        {compact ? null : <span className="shrink-0 text-caption-1-medium text-text-tertiary">{look.label}</span>}
        <a
          href={issue.url}
          target="_blank"
          rel="noopener noreferrer"
          className="flex min-w-0 flex-1 items-center gap-2 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <span className="shrink-0 text-body-2-medium text-text-primary">{issue.key}</span>
          <span className="truncate text-body-2-regular text-text-secondary">{issue.title}</span>
          <RiExternalLinkLine aria-hidden className="size-3.5 shrink-0 text-foreground-icon-tertiary" />
        </a>
        <Chip variant="caption" color={groupTint[issue.status_group] ?? "gray"}>
          {issue.status || issue.status_group}
        </Chip>
        <IconButton size="small" icon={RiLinkUnlinkM} aria-label={`Unlink ${issue.key}`} onClick={onUnlink} />
      </div>
      {problem ? (
        <p className="flex items-start gap-1.5 text-caption-1-regular text-text-error-primary">
          <RiErrorWarningLine aria-hidden className="mt-0.5 size-3.5 shrink-0" />
          {problem}
        </p>
      ) : null}
    </li>
  );
}

/*
 * LinkButton opens the dialog that links engineering work.
 */
function LinkButton({
  size,
  disabled = false,
  onPress,
}: {
  size: "small" | "xs";
  disabled?: boolean;
  onPress: () => void;
}) {
  return (
    <Button variant="secondary" size={size} leadingIcon={RiAddLine} disabled={disabled} onClick={onPress}>
      Link issue
    </Button>
  );
}

/*
 * Heading names the block and offers to link work.
 */
function Heading({ linkable, onLink }: { linkable: boolean; onLink: () => void }) {
  return (
    <div className="flex items-end justify-between gap-3">
      <div className="flex flex-col gap-0.5">
        <h3 className="text-headline-medium text-text-primary">Engineering work</h3>
        <p className="text-caption-1-regular text-text-tertiary">
          {linkable
            ? "Statuses come back from the tool and move this ticket for you."
            : "Connect Linear, Jira, GitHub or an incident tool in Settings, Integrations to link work here."}
        </p>
      </div>
      <LinkButton disabled={!linkable} size="small" onPress={onLink} />
    </div>
  );
}

/*
 * IssueList lists the linked issues, framed on a page and bare in the
 * panel.
 */
function IssueList({
  issues,
  compact,
  onUnlink,
}: {
  issues: TicketIssue[];
  compact: boolean;
  onUnlink: (issue: TicketIssue) => void;
}) {
  if (issues.length === 0) {
    return null;
  }

  return (
    <ul
      className={
        compact
          ? "flex flex-col"
          : "flex flex-col divide-y divide-separator-border rounded-xl border border-border-primary"
      }
    >
      {issues.map((issue) => (
        <IssueRow key={issue.id} issue={issue} compact={compact} onUnlink={() => onUnlink(issue)} />
      ))}
    </ul>
  );
}

/*
 * TicketIssues is the block itself; compact fits the conversation's
 * ticket panel, without the heading and the drafts.
 */
export function TicketIssues({ ticketId, compact = false }: { ticketId: string; compact?: boolean }) {
  const workspace = useAccount().workspace.id;
  const queryClient = useQueryClient();
  const { data: ticket } = useQuery(ticketQuery(workspace, ticketId));
  const { data: issues, isPending } = useQuery(ticketIssuesQuery(workspace, ticketId));
  const { data: installations } = useQuery(integrationsQuery(workspace));
  const [linking, setLinking] = useState(false);
  const [unlinking, setUnlinking] = useState<TicketIssue | null>(null);
  const tools = (installations ?? []).filter((i) => isTool(i.provider) && i.status === "active");
  const list = issues ?? [];

  const unlink = useMutation({
    mutationFn: (issue: TicketIssue) => unlinkTicketIssue(workspace, issue.id),
    onSuccess: async () => {
      await Promise.all(
        refreshTicketKeys(workspace, ticketId).map((queryKey) => queryClient.invalidateQueries({ queryKey })),
      );
      toast.success("Unlinked. The issue stays in its tool.");
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <section aria-label="Engineering work" className={compact ? "flex flex-col gap-1" : "flex flex-col gap-3"}>
      {compact ? null : <Heading linkable={tools.length > 0} onLink={() => setLinking(true)} />}
      {isPending ? null : <IssueList issues={list} compact={compact} onUnlink={(issue) => setUnlinking(issue)} />}
      {compact && tools.length > 0 ? (
        <div>
          <LinkButton size="xs" onPress={() => setLinking(true)} />
        </div>
      ) : null}
      {compact && tools.length === 0 && list.length === 0 ? (
        <span className="text-body-2-regular text-text-tertiary">No linked issues.</span>
      ) : null}
      {compact ? null : <CloseLoopDrafts workspace={workspace} ticket={ticketId} />}
      {linking ? (
        <LinkIssueDialog
          workspace={workspace}
          ticket={ticketId}
          title={ticket?.title ?? ""}
          tools={tools}
          onClose={() => setLinking(false)}
        />
      ) : null}
      <AlertDialog
        isOpen={unlinking !== null}
        onOpenChange={(open) => (open ? undefined : setUnlinking(null))}
        title={`Unlink ${unlinking?.key ?? "the issue"}?`}
        description="Its status stops moving this ticket. The issue itself stays in its tool."
        confirmLabel="Unlink"
        onConfirm={() => (unlinking ? unlink.mutateAsync(unlinking) : undefined)}
      />
    </section>
  );
}
