/*
 * One broadcast: its composer while it is a draft, its summary while it
 * waits for its time, and its report once it went out, filling in live
 * while it sends. The header holds the one or two things that make
 * sense now: unscheduling one that waits, pausing or stopping one that
 * is going out, resuming one that paused, and duplicating one that
 * finished. Stopping always asks first, since it cannot be undone.
 */
import { RiErrorWarningLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { useWorkspace } from "@/features/customers/names";
import { Row, Section } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { EmptyState, Panel } from "@/features/shell/page";
import { TableSkeleton } from "@/features/shell/table-skeleton";
import { type Broadcast, type BroadcastAction, broadcastEstimateQuery, broadcastQuery } from "@/lib/api/broadcasts";
import { formatDateTime } from "@/lib/i18n/format";

import { BroadcastReport } from "./broadcast-report";
import { draftOf } from "./draft";
import { DraftPage } from "./draft-page";
import { useLiveBroadcasts } from "./live";
import { channelLook, noteOf } from "./looks";
import { Frame } from "./page-frame";
import { ReviewStep } from "./review-step";
import { useNames, useWorkspaceZone } from "./sources";
import { useBroadcastActions } from "./use-actions";

function Details({ broadcast, audience, sender }: { broadcast: Broadcast; audience: string; sender: string }) {
  return (
    <Section title="Details">
      <Row label="Channel" description={channelLook[broadcast.channel].name} />
      <Row label="Audience" description={`${audience}, fixed when it was sent`} />
      <Row label="From" description={sender} />
      {broadcast.started_at ? <Row label="Started" description={formatDateTime(broadcast.started_at)} /> : null}
      {broadcast.finished_at ? <Row label="Finished" description={formatDateTime(broadcast.finished_at)} /> : null}
    </Section>
  );
}

function SentActions({
  broadcast,
  onAction,
}: {
  broadcast: Broadcast;
  onAction: (action: BroadcastAction) => Promise<unknown>;
}) {
  const [stopping, setStopping] = useState(false);
  const stop = (
    <>
      <AlertDialog
        isOpen={stopping}
        onOpenChange={setStopping}
        title="Stop this broadcast?"
        description="Nobody it hasn't reached yet will get it. This can't be undone."
        confirmLabel="Stop"
        onConfirm={() => onAction("stop")}
      />
      <Button variant="danger-secondary" size="small" onClick={() => setStopping(true)}>
        Stop
      </Button>
    </>
  );

  switch (broadcast.status) {
    case "scheduled":
      return (
        <Button variant="secondary" size="small" onClick={() => void onAction("cancel")}>
          Unschedule
        </Button>
      );
    case "sending":
      return (
        <>
          <Button variant="secondary" size="small" onClick={() => void onAction("pause")}>
            Pause
          </Button>
          {stop}
        </>
      );
    case "paused":
      return (
        <>
          {stop}
          <Button size="small" onClick={() => void onAction("resume")}>
            Resume
          </Button>
        </>
      );
    default:
      return (
        <Button variant="secondary" size="small" onClick={() => void onAction("duplicate")}>
          Duplicate
        </Button>
      );
  }
}

function OutPage({ workspace, broadcast }: { workspace: string; broadcast: Broadcast }) {
  const { act } = useBroadcastActions(workspace, broadcast.id);
  const names = useNames(workspace, broadcast.channel, broadcast);
  const zone = useWorkspaceZone(workspace);
  const brand = useAccount().workspace.name;
  const scheduled = broadcast.status === "scheduled";
  const { data: estimate } = useQuery({ ...broadcastEstimateQuery(workspace, broadcast.id), enabled: scheduled });
  const onAction = (action: BroadcastAction) =>
    act.mutateAsync(action).catch((error: Error) => void toast.error(error.message));

  return (
    <Frame
      name={broadcast.name}
      status={broadcast.status}
      note={noteOf(broadcast, zone)}
      actions={<SentActions broadcast={broadcast} onAction={onAction} />}
      message={{ ...broadcast, sender: names.sender, brand }}
      details={scheduled ? null : <Details broadcast={broadcast} {...names} />}
    >
      {scheduled ? (
        <div className="mx-auto flex w-full max-w-[680px] flex-col gap-8 px-4 pt-4 pb-16 sm:px-6">
          <p className="px-3 text-body-2-regular text-text-tertiary">Unschedule it to make changes.</p>
          <ReviewStep
            broadcast={broadcast}
            draft={draftOf(broadcast)}
            names={names}
            estimate={estimate}
            checks={[]}
            refusal={null}
          />
        </div>
      ) : (
        <BroadcastReport workspace={workspace} broadcast={broadcast} onAction={(action) => void onAction(action)} />
      )}
    </Frame>
  );
}

export function BroadcastPage({ id }: { id: string }) {
  const workspace = useWorkspace();
  useLiveBroadcasts(workspace);
  const { data: broadcast, isPending, isError } = useQuery(broadcastQuery(workspace, id));

  if (isPending) {
    return (
      <Panel label="Broadcast" ground className="h-full min-w-0 flex-1 p-6">
        <TableSkeleton rows={6} />
      </Panel>
    );
  }
  if (isError || !broadcast) {
    return (
      <Panel label="Broadcast" ground className="h-full min-w-0 flex-1">
        <EmptyState icon={RiErrorWarningLine} title="This broadcast isn't here" description="It may have been deleted.">
          <Link to="/d/broadcasts" className="text-body-2-medium text-text-secondary underline">
            Back to broadcasts
          </Link>
        </EmptyState>
      </Panel>
    );
  }
  if (broadcast.status === "draft") {
    return <DraftPage key={`${broadcast.id}:draft`} workspace={workspace} broadcast={broadcast} />;
  }

  return <OutPage workspace={workspace} broadcast={broadcast} />;
}
