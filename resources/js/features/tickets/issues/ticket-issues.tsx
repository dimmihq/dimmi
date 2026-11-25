/*
 * The engineering work behind a ticket, for the ticket panel and the
 * tracker page: each linked issue or incident with its
 * tool, key, title and status as last seen, and any warning, such as a
 * Linear team the app can no longer see, so nothing drops silently.
 * Linking opens the two-step dialog; unlinking asks first and leaves the
 * issue in its tool.
 */
import { RiAddLine, RiErrorWarningLine, RiExternalLinkLine, RiLinkUnlinkM, RiRefreshLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { LinkButton as LinkAction } from "@/components/base/buttons/link-button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { Section } from "@/features/inbox/customer/rows";
import { useAccount } from "@/features/settings/use-account";
import { type Integration, integrationsQuery } from "@/lib/api/integrations";
import {
  issueKeys,
  refreshTicketIssues,
  refreshTicketKeys,
  ticketIssuesQuery,
  unlinkTicketIssue,
} from "@/lib/api/ticket-issues";
import { type TicketIssue, ticketQuery } from "@/lib/api/tickets";

import { LinkIssueDialog } from "./link-issue-dialog";
import { ToolMark } from "./tool-logos";
import { groupTint, isTool, toolLook, writtenOn } from "./tools";

function IssueRow({ issue, compact, onUnlink }: { issue: TicketIssue; compact: boolean; onUnlink: () => void }) {
  const look = toolLook(issue.provider);
  const problem = issue.warning ?? issue.sync_error;

  return (
    <li
      className={compact ? "group/issue flex min-h-8 flex-col justify-center gap-1" : "flex flex-col gap-1 px-4 py-3"}
    >
      <div className={compact ? "flex items-center gap-2" : "flex items-center gap-3"}>
        <a
          href={issue.url}
          target="_blank"
          rel="noopener noreferrer"
          aria-label={`${issue.key} in ${look.label}: ${issue.title}`}
          className="flex min-w-0 flex-1 items-center gap-2 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <ToolMark provider={issue.provider} />
          <span className="shrink-0 text-body-2-medium text-text-primary">{issue.key}</span>
          <span className="truncate text-body-2-regular text-text-secondary">{issue.title}</span>
          <RiExternalLinkLine aria-hidden className="size-3.5 shrink-0 text-foreground-icon-tertiary" />
        </a>
        <Chip variant="caption" color={groupTint[issue.status_group] ?? "neutral"}>
          {issue.status || issue.status_group}
        </Chip>
        {compact ? (
          <Button
            variant="secondary"
            size="xs"
            iconOnly
            leadingIcon={RiLinkUnlinkM}
            aria-label={`Unlink ${issue.key}`}
            className="shrink-0 opacity-0 transition-opacity duration-150 focus-visible:opacity-100 group-hover/issue:opacity-100"
            onClick={onUnlink}
          />
        ) : (
          <IconButton size="small" icon={RiLinkUnlinkM} aria-label={`Unlink ${issue.key}`} onClick={onUnlink} />
        )}
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
function LinkButton({ disabled = false, onPress }: { disabled?: boolean; onPress: () => void }) {
  return (
    <Button variant="secondary" size="small" leadingIcon={RiAddLine} disabled={disabled} onClick={onPress}>
      Link issue
    </Button>
  );
}

/*
 * Refresh is how a ticket's issues are read from their tools now.
 */
interface Refresh {
  shown: boolean;
  busy: boolean;
  run: () => void;
}

/*
 * Heading names the block and offers to link work, and to read the
 * linked issues again.
 */
function Heading({ linkable, refresh, onLink }: { linkable: boolean; refresh: Refresh; onLink: () => void }) {
  return (
    <div className="flex items-end justify-between gap-3">
      <div className="flex flex-col gap-0.5">
        <h3 className="text-headline-medium text-text-primary">Linked issues</h3>
        <p className="text-caption-1-regular text-text-tertiary">
          {linkable
            ? "Statuses come back from the tool and move this ticket for you."
            : "Connect Linear, Jira, GitHub or an incident tool in Settings, Integrations to link work here."}
        </p>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        {refresh.shown ? (
          <Button
            variant="secondary"
            size="small"
            leadingIcon={RiRefreshLine}
            loading={refresh.busy}
            onClick={refresh.run}
          >
            Refresh
          </Button>
        ) : null}
        <LinkButton disabled={!linkable} onPress={onLink} />
      </div>
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
 * CompactFoot ends the panel's block: a quiet way to link work when a
 * tool is connected and to read the linked issues again, or a line
 * saying there is none.
 */
function CompactFoot({
  linkable,
  empty,
  refresh,
  onLink,
}: {
  linkable: boolean;
  empty: boolean;
  refresh: Refresh;
  onLink: () => void;
}) {
  if (linkable) {
    return (
      <div className="flex min-h-8 items-center gap-3">
        <LinkAction size="xs" variant="secondary" leadingIcon={RiAddLine} onClick={onLink}>
          Link issue
        </LinkAction>
        {refresh.shown ? (
          <LinkAction
            size="xs"
            variant="secondary"
            leadingIcon={RiRefreshLine}
            disabled={refresh.busy}
            onClick={refresh.run}
          >
            {refresh.busy ? "Refreshing…" : "Refresh"}
          </LinkAction>
        ) : null}
      </div>
    );
  }

  return empty ? <span className="text-body-2-regular text-text-tertiary">No linked issues.</span> : null;
}

/*
 * TicketIssues is the block itself; compact fits the conversation's
 * ticket panel, without the heading.
 */
/*
 * noun is what the tool holds: an incident or an issue.
 */
function noun(provider: string) {
  return toolLook(provider).kind === "incidents" ? "incident" : "issue";
}

export function TicketIssues({
  ticketId,
  compact = false,
  heading,
}: {
  ticketId: string;
  compact?: boolean;
  /* Names the compact block, which then shows nothing at all when it has no issue and no tool to link one in. */
  heading?: string;
}) {
  const workspace = useAccount().workspace.id;
  const queryClient = useQueryClient();
  const { data: ticket } = useQuery(ticketQuery(workspace, ticketId));
  const { data: issues, isPending } = useQuery(ticketIssuesQuery(workspace, ticketId));
  const { data: installations } = useQuery(integrationsQuery(workspace));
  const [linking, setLinking] = useState(false);
  const [unlinking, setUnlinking] = useState<TicketIssue | null>(null);
  const [removeFromTool, setRemoveFromTool] = useState(false);
  const tools = (installations ?? []).filter((i) => isTool(i.provider) && i.status === "active");
  const list = issues ?? [];

  const unlink = useMutation({
    mutationFn: ({ issue, remove }: { issue: TicketIssue; remove: boolean }) =>
      unlinkTicketIssue(workspace, issue.id, remove),
    onSuccess: async (_, { issue, remove }) => {
      await Promise.all(
        refreshTicketKeys(workspace, ticketId).map((queryKey) => queryClient.invalidateQueries({ queryKey })),
      );
      const label = toolLook(issue.provider).label;
      toast.success(
        remove
          ? `Unlinked, and dimmi's details removed from ${label}.`
          : `Unlinked. The ${noun(issue.provider)} stays in ${label} as it is.`,
      );
    },
    onError: (error) => toast.error(error.message),
  });
  const reread = useMutation({
    mutationFn: () => refreshTicketIssues(workspace, ticketId),
    onSuccess: async (fresh) => {
      queryClient.setQueryData(issueKeys.ticket(workspace, ticketId), fresh);
      await Promise.all(
        refreshTicketKeys(workspace, ticketId).map((queryKey) => queryClient.invalidateQueries({ queryKey })),
      );
    },
    onError: (error) => toast.error(error.message),
  });
  const refresh: Refresh = { shown: list.length > 0, busy: reread.isPending, run: () => reread.mutate() };

  if (heading && tools.length === 0 && list.length === 0) {
    return null;
  }

  const dialogs = (
    <IssueDialogs
      workspace={workspace}
      ticketId={ticketId}
      title={ticket?.title ?? ""}
      tools={tools}
      linking={linking}
      onCloseLinking={() => setLinking(false)}
      unlinking={unlinking}
      onCloseUnlinking={() => setUnlinking(null)}
      removeFromTool={removeFromTool}
      onRemoveFromTool={setRemoveFromTool}
      onUnlink={(issue) => unlink.mutateAsync({ issue, remove: removeFromTool })}
    />
  );

  if (heading) {
    return (
      <Section title={heading} count={list.length}>
        <div className="flex flex-col">
          <CompactFoot linkable={tools.length > 0} empty={false} refresh={refresh} onLink={() => setLinking(true)} />
          <IssueList
            issues={list}
            compact
            onUnlink={(issue) => {
              setRemoveFromTool(false);
              setUnlinking(issue);
            }}
          />
        </div>
        {dialogs}
      </Section>
    );
  }

  return (
    <section aria-label="Linked issues" className={compact ? "flex flex-col gap-1" : "flex flex-col gap-3"}>
      {compact ? null : <Heading linkable={tools.length > 0} refresh={refresh} onLink={() => setLinking(true)} />}
      {isPending ? null : (
        <IssueList
          issues={list}
          compact={compact}
          onUnlink={(issue) => {
            setRemoveFromTool(false);
            setUnlinking(issue);
          }}
        />
      )}
      {compact ? (
        <CompactFoot
          linkable={tools.length > 0}
          empty={list.length === 0}
          refresh={refresh}
          onLink={() => setLinking(true)}
        />
      ) : null}
      {dialogs}
    </section>
  );
}

/*
 * IssueDialogs are the block's dialogs: linking an issue, and confirming
 * an unlink, with the choice to remove what dimmi wrote in the tool.
 */
function IssueDialogs({
  workspace,
  ticketId,
  title,
  tools,
  linking,
  onCloseLinking,
  unlinking,
  onCloseUnlinking,
  removeFromTool,
  onRemoveFromTool,
  onUnlink,
}: {
  workspace: string;
  ticketId: string;
  title: string;
  tools: Integration[];
  linking: boolean;
  onCloseLinking: () => void;
  unlinking: TicketIssue | null;
  onCloseUnlinking: () => void;
  removeFromTool: boolean;
  onRemoveFromTool: (remove: boolean) => void;
  onUnlink: (issue: TicketIssue) => Promise<unknown>;
}) {
  const provider = unlinking?.provider ?? "";

  return (
    <>
      {linking ? (
        <LinkIssueDialog workspace={workspace} ticket={ticketId} title={title} tools={tools} onClose={onCloseLinking} />
      ) : null}
      <AlertDialog
        isOpen={unlinking !== null}
        onOpenChange={(open) => (open ? undefined : onCloseUnlinking())}
        title={`Unlink ${unlinking?.key ?? "the issue"}?`}
        description={`Its status stops moving this ticket. The ${noun(provider)} stays in ${toolLook(provider).label}.`}
        confirmLabel="Unlink"
        onConfirm={() => (unlinking ? onUnlink(unlinking) : undefined)}
      >
        {unlinking && writtenOn(unlinking.provider) ? (
          <Checkbox isSelected={removeFromTool} onChange={onRemoveFromTool}>
            <span className="flex flex-col">
              <span>Also remove what dimmi wrote in {toolLook(unlinking.provider).label}</span>
              <span className="text-body-2-regular text-text-secondary">{writtenOn(unlinking.provider)}</span>
            </span>
          </Checkbox>
        ) : null}
      </AlertDialog>
    </>
  );
}
