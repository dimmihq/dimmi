/*
 * The Slack channels a connection watches, each with who counts as the
 * customer there and when a message becomes a conversation. Slack takes
 * the app out of every channel when it is uninstalled, so a removed
 * channel stays listed with its settings and asks for the app to be
 * invited back. A channel's settings open in a dialog; people who may
 * not manage channels read them without changing them.
 */
import { RiHashtag } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { type SlackRoom, slackRoomsQuery } from "@/lib/api/integrations";

import { roomLine } from "../integrations";
import { Empty, Glyph, Item, Section } from "../layout";
import { RoomSettingsModal } from "./slack-room-modal";

export function RoomsSection({
  workspace,
  channel,
  canManage,
}: {
  workspace: string;
  channel: string;
  canManage: boolean;
}) {
  const { data: rooms = [], isPending } = useQuery(slackRoomsQuery(workspace, channel));
  const [open, setOpen] = useState<SlackRoom | null>(null);

  return (
    <Section
      title="Channels"
      description="The Slack channels the app is in. Invite it to a channel to start watching it."
      loading={isPending}
      rows={2}
    >
      {rooms.length === 0 ? <Empty>No channels yet. Invite @dimmi to a Slack channel to start.</Empty> : null}
      {rooms.map((room) => (
        <Item
          key={room.id}
          mark={<Glyph icon={RiHashtag} />}
          title={
            <span className="flex items-center gap-2">
              {room.name}
              {room.status === "removed" ? (
                <Chip variant="caption" color="neutral">
                  Removed
                </Chip>
              ) : null}
            </span>
          }
          detail={roomLine(room)}
        >
          <Button
            variant="secondary"
            size="xs"
            aria-label={`${canManage ? "Settings" : "View"} for ${room.name}`}
            onClick={() => setOpen(room)}
          >
            {canManage ? "Settings" : "View"}
          </Button>
        </Item>
      ))}
      {open ? (
        <RoomSettingsModal
          workspace={workspace}
          room={open}
          rooms={rooms}
          canManage={canManage}
          onClose={() => setOpen(null)}
        />
      ) : null}
    </Section>
  );
}
