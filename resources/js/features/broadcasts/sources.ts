/*
 * What the composer's pickers choose from: saved audiences, topics, the
 * teammates a broadcast can come from, the teams replies can go to, the
 * addresses email leaves from, events a goal can name, and the
 * customer Slack channels a Slack broadcast posts into. A Slack channel
 * counts when it is active and shared with a customer or part of a
 * community; the team's own channels never get broadcasts.
 */
import { useQueries, useQuery } from "@tanstack/react-query";

import { audiencesQuery } from "@/lib/api/audiences";
import { broadcastTopicsQuery, type Channel } from "@/lib/api/broadcasts";
import { emailAddressesQuery } from "@/lib/api/email";
import { eventsQuery } from "@/lib/api/events";
import { integrationsQuery, type SlackRoom, slackRoomsQuery } from "@/lib/api/integrations";
import { teamsQuery } from "@/lib/api/teams";
import { membersQuery, workspaceQuery } from "@/lib/api/workspace";

import type { Draft } from "./draft";
import type { Names } from "./review-step";

export function useAudiences(workspace: string) {
  return useQuery(audiencesQuery(workspace)).data ?? [];
}

export function useTopics(workspace: string) {
  return useQuery(broadcastTopicsQuery(workspace)).data ?? [];
}

export function useActiveMembers(workspace: string) {
  const members = useQuery(membersQuery(workspace)).data ?? [];

  return members.filter((member) => member.status === "active");
}

export function useTeams(workspace: string) {
  return useQuery(teamsQuery(workspace)).data ?? [];
}

export function useEmailAddresses(workspace: string, enabled = true) {
  return useQuery({ ...emailAddressesQuery(workspace), enabled }).data ?? [];
}

export function useEvents(workspace: string) {
  const events = useQuery(eventsQuery(workspace)).data ?? [];

  return events.filter((event) => event.status === "active");
}

/*
 * useCustomerRooms reads every connected Slack workspace's channels and
 * keeps those shared with customers. loading holds until all are read.
 */
export function useCustomerRooms(workspace: string, enabled = true): { rooms: SlackRoom[]; loading: boolean } {
  const integrations = useQuery({ ...integrationsQuery(workspace), enabled });
  const connections = (integrations.data ?? [])
    .filter((integration) => integration.provider === "slack" && integration.status === "active")
    .flatMap((integration) => integration.channel_ids);

  return useQueries({
    queries: connections.map((channel) => ({ ...slackRoomsQuery(workspace, channel), enabled })),
    combine: (results) => ({
      rooms: results
        .flatMap((result) => result.data ?? [])
        .filter((room) => room.status === "active" && (room.mode === "shared" || room.mode === "community")),
      loading: integrations.isPending || results.some((result) => result.isPending),
    }),
  });
}

/*
 * useNames names what the draft's ids point at: who it goes to and the
 * teammate it comes from.
 */
export function useNames(
  workspace: string,
  channel: Channel,
  draft: Pick<Draft, "audience_id" | "rules" | "room_ids" | "sender_id">,
): Names {
  const audiences = useAudiences(workspace);
  const members = useActiveMembers(workspace);
  const audience = audiences.find((one) => one.id === draft.audience_id)?.name;
  const sender = members.find((member) => member.id === draft.sender_id)?.user.name ?? "A teammate";

  return { audience: audienceName(channel, draft, audience), sender };
}

function audienceName(
  channel: Channel,
  draft: Pick<Draft, "audience_id" | "rules" | "room_ids">,
  audience: string | undefined,
): string {
  if (channel === "slack") {
    const rooms = draft.room_ids?.length ?? 0;
    const where = rooms ? `${rooms} ${rooms === 1 ? "channel" : "channels"}` : "every customer channel";

    return audience ? `${where} of ${audience}` : where;
  }
  if (audience) {
    return audience;
  }

  return draft.rules ? "people matching its own rule" : "no audience yet";
}

/*
 * useWorkspaceZone is the workspace's time zone, which a send in each
 * person's own zone is written in.
 */
export function useWorkspaceZone(workspace: string): string | undefined {
  return useQuery(workspaceQuery(workspace)).data?.timezone;
}
