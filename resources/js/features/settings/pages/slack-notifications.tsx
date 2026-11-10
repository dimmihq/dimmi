/*
 * The Slack channels an installation posts notifications in, each with
 * what it hears about, a test post, and editing or removing it. A
 * channel Slack refused shows the reason and a way to turn it back on
 * once the cause is fixed. Changing them takes
 * notification_destinations:manage; everyone else reads them.
 */
import { RiHashtag } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import {
  deleteDestination,
  destinationsQuery,
  type NotificationDestination,
} from "@/lib/api/notification-destinations";

import { destinationLine } from "../destinations";
import { Empty, Glyph, Item, Section } from "../layout";
import { useDestinationActions } from "../use-destination-actions";
import { SlackDestinationModal } from "./slack-destination-modal";

/*
 * DestinationRow is one channel destination with what can be done to
 * it.
 */
function DestinationRow({
  workspace,
  destination,
  canManage,
  onEdit,
  onDelete,
}: {
  workspace: string;
  destination: NotificationDestination;
  canManage: boolean;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const { test, enable } = useDestinationActions(workspace);
  const name = `#${destination.slack_channel_name ?? destination.name}`;
  const isStopped = destination.status !== "active";

  return (
    <Item
      mark={<Glyph icon={RiHashtag} />}
      title={
        <span className="flex items-center gap-2">
          {name}
          {destination.status === "error" ? (
            <Chip variant="caption" color="rose">
              Stopped
            </Chip>
          ) : null}
          {destination.status === "disabled" ? (
            <Chip variant="caption" color="neutral">
              Off
            </Chip>
          ) : null}
        </span>
      }
      detail={
        destination.status === "error" ? (
          <span className="text-text-error-primary">{destinationLine(destination)}</span>
        ) : (
          destinationLine(destination)
        )
      }
    >
      {canManage && isStopped ? (
        <Button
          variant="primary"
          size="xs"
          aria-label={`Turn ${name} back on`}
          loading={enable.isPending}
          onClick={() => enable.mutate(destination)}
        >
          Turn back on
        </Button>
      ) : null}
      {canManage ? (
        <>
          <Button
            variant="secondary"
            size="xs"
            aria-label={`Test ${name}`}
            loading={test.isPending}
            onClick={() => test.mutate(destination)}
          >
            Test
          </Button>
          <Button variant="secondary" size="xs" aria-label={`Edit ${name}`} onClick={onEdit}>
            Edit
          </Button>
          <Button variant="danger-secondary" size="xs" aria-label={`Remove ${name}`} onClick={onDelete}>
            Remove
          </Button>
        </>
      ) : null}
    </Item>
  );
}

export function SlackNotificationsSection({
  workspace,
  integration,
  canManage,
}: {
  workspace: string;
  integration: string;
  canManage: boolean;
}) {
  const { data = [], isPending } = useQuery(destinationsQuery(workspace));
  const { refresh } = useDestinationActions(workspace);
  const [editing, setEditing] = useState<NotificationDestination | "new" | null>(null);
  const [removing, setRemoving] = useState<NotificationDestination | null>(null);
  const channels = data.filter((d) => d.type === "slack_channel" && d.integration_id === integration);

  const remove = async () => {
    if (!removing) {
      return;
    }

    try {
      await deleteDestination(workspace, removing.id);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Something went wrong. Try again.");
      throw error;
    }

    await refresh();
  };

  return (
    <Section
      title="Notifications"
      description="Channels where the team hears about conversations and gets digests."
      loading={isPending}
      rows={1}
      action={
        canManage ? (
          <Button variant="secondary" size="small" onClick={() => setEditing("new")}>
            Add channel
          </Button>
        ) : undefined
      }
    >
      {channels.length === 0 ? <Empty>No channel gets notifications yet.</Empty> : null}
      {channels.map((destination) => (
        <DestinationRow
          key={destination.id}
          workspace={workspace}
          destination={destination}
          canManage={canManage}
          onEdit={() => setEditing(destination)}
          onDelete={() => setRemoving(destination)}
        />
      ))}
      {editing ? (
        <SlackDestinationModal
          workspace={workspace}
          integration={integration}
          destination={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
        />
      ) : null}
      <AlertDialog
        isOpen={removing !== null}
        onOpenChange={(open) => !open && setRemoving(null)}
        title={`Stop posting to #${removing?.slack_channel_name ?? removing?.name ?? "this channel"}?`}
        description="The channel stops getting notifications and digests. Nothing already posted is removed."
        confirmLabel="Remove"
        onConfirm={remove}
      />
    </Section>
  );
}
