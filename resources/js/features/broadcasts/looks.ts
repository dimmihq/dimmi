/*
 * How each channel and each state of a broadcast looks: its name, its
 * icon and its BoardUI chip tint, so the list, the composer and the
 * report name them the same way. The reasons a person was left out or
 * a message failed read here in words; a code nobody named reads as
 * itself, made readable.
 */
import { RiChat3Line, RiFileTextLine, RiMailLine, RiSlackLine } from "@remixicon/react";
import type { ComponentType } from "react";

import type { ChipColor } from "@/components/base/badges/chip";
import type { Broadcast, BroadcastStatus, Channel, PauseReason } from "@/lib/api/broadcasts";
import { formatDateTime } from "@/lib/i18n/format";

/*
 * ChannelLook is a channel's name, icon and tint, and the line that
 * says where the message shows up.
 */
export interface ChannelLook {
  name: string;
  icon: ComponentType<{ className?: string }>;
  tint: ChipColor;
  where: string;
}

export const channelLook: Record<Channel, ChannelLook> = {
  email: { name: "Email", icon: RiMailLine, tint: "blue", where: "Lands in their inbox, replies come back here." },
  chat: { name: "Chat", icon: RiChat3Line, tint: "lime", where: "A message from a teammate in the messenger." },
  post: { name: "Post", icon: RiFileTextLine, tint: "purple", where: "An announcement card in the messenger." },
  slack: { name: "Slack", icon: RiSlackLine, tint: "orange", where: "A post in each customer's shared Slack channel." },
};

export const channels = Object.keys(channelLook) as Channel[];

export const statusLook: Record<BroadcastStatus, { name: string; tint: ChipColor }> = {
  draft: { name: "Draft", tint: "gray" },
  scheduled: { name: "Scheduled", tint: "blue" },
  sending: { name: "Sending", tint: "yellow" },
  paused: { name: "Paused", tint: "orange" },
  sent: { name: "Sent", tint: "lime" },
  stopped: { name: "Stopped", tint: "rose" },
};

export const statuses = Object.keys(statusLook) as BroadcastStatus[];

export function statusName(status: BroadcastStatus): string {
  return statusLook[status].name;
}

export const pauseReasons: Record<PauseReason, string> = {
  bounce_rate: "Too many emails bounced, so sending paused to protect your domain.",
  complaint_rate: "Too many people reported it as spam, so sending paused to protect your domain.",
  paused_by_teammate: "A teammate paused it.",
};

const reasonNames: Record<string, string> = {
  no_email: "No email address",
  address_not_found: "The address doesn't exist",
  mailbox_refused: "The mailbox refused it",
  spam_complaint: "Reported as spam",
  suppressed: "Bounced or reported spam before",
  unsubscribed: "Unsubscribed from everything",
  not_subscribed: "Not subscribed to the topic",
  frequency_cap: "Got another message recently",
  duplicate_address: "Same address as someone else",
  no_messenger: "Never used the messenger",
  blocked: "Blocked",
  room_inactive: "Channel no longer connected",
  room_no_company: "Channel has no account",
  room_unsubscribed: "Channel turned broadcasts off",
  not_in_audience: "No longer in the audience",
  broadcast_stopped: "Broadcast was stopped",
  no_sending_address: "No address to send from",
};

/*
 * reasonName names why a recipient was left out or failed, reading an
 * unknown code as words.
 */
export function reasonName(reason: string): string {
  const known = reasonNames[reason];
  if (known) {
    return known;
  }
  const words = reason.replaceAll(/[_.-]+/g, " ").trim();

  return words.charAt(0).toUpperCase() + words.slice(1);
}

/*
 * percent says what share of whole the part is, rounded to one place.
 */
export function percent(part: number, whole: number): string {
  if (whole === 0) {
    return "0%";
  }

  return `${Math.round((part / whole) * 1000) / 10}%`;
}

/*
 * noteOf says, in a few words, when the broadcast went or will go out.
 */
export function noteOf(broadcast: Broadcast): string {
  const at = (moment: string | null) => (moment ? formatDateTime(moment) : "");
  switch (broadcast.status) {
    case "scheduled":
      return broadcast.scheduled_at ? `Goes out ${at(broadcast.scheduled_at)}` : "Scheduled";
    case "sending":
      return `Started ${at(broadcast.started_at)}`;
    case "paused":
      return "Paused";
    case "sent":
      return `Sent ${at(broadcast.finished_at ?? broadcast.started_at)}`;
    case "stopped":
      return `Stopped ${at(broadcast.finished_at)}`;
    default:
      return `Edited ${at(broadcast.updated_at)}`;
  }
}

/*
 * reachNoun is what a broadcast reaches, singular or plural: Slack
 * channels, or people.
 */
export function reachNoun(channel: string, count: number): string {
  if (channel === "slack") {
    return count === 1 ? "channel" : "channels";
  }

  return count === 1 ? "person" : "people";
}

/*
 * roomName is a Slack channel's name with one leading #.
 */
export function roomName(name: string): string {
  return `#${name.replace(/^#+/, "")}`;
}

/*
 * counted writes a count with its noun, singular for one.
 */
export function counted(count: number, one: string, many: string): string {
  return `${count.toLocaleString()} ${count === 1 ? one : many}`;
}
